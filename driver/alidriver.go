// 阿里云盘的驱动代码主要复制来自 https://github.com/Xhofe/alist
// 复制的代码版权归原作者所有

package driver

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/eko/gocache/v2/cache"
	"github.com/eko/gocache/v2/store"
	"github.com/go-resty/resty/v2"
	"github.com/golang-jwt/jwt"
	"github.com/labstack/echo/v4"
	"github.com/labstack/gommon/log"
	fileerr "github.com/lixiaofei123/nextlist/errors"
	"github.com/lixiaofei123/nextlist/utils"
	goCache "github.com/patrickmn/go-cache"
	"gorm.io/gorm"
)

func init() {
	RegsiterDriver("alidriver", &AliDriver{})
}

type AliFile struct {
	DriveId       string     `json:"drive_id"`
	CreatedAt     *time.Time `json:"created_at"`
	FileExtension string     `json:"file_extension"`
	FileId        string     `json:"file_id"`
	Type          string     `json:"type"`
	Name          string     `json:"name"`
	Category      string     `json:"category"`
	ParentFileId  string     `json:"parent_file_id"`
	UpdatedAt     *time.Time `json:"updated_at"`
	Size          int64      `json:"size"`
	Thumbnail     string     `json:"thumbnail"`
	Url           string     `json:"url"`
}

type TokenResp struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
}

type AliRespError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type AliFiles struct {
	Items      []AliFile `json:"items"`
	NextMarker string    `json:"next_marker"`
}

type AliDriverConfig struct {
	RefreshToken string
	Status       string
	Key          string
	Host         string
	RootID       string
}

type AliDriver struct {
	client      *resty.Client
	config      AliDriverConfig
	ctx         context.Context
	alicache    *cache.Cache
	httpClient  *http.Client
	DriverID    string
	AccessToken string
}

type UploadResp struct {
	FileId       string `json:"file_id"`
	UploadId     string `json:"upload_id"`
	PartInfoList []struct {
		UploadUrl string `json:"upload_url"`
	} `json:"part_info_list"`
}

type UploaderFileStream struct {
	Name       string
	parentPath string
	DataLength int
	reader     io.ReadCloser
}

func (r UploaderFileStream) Read(p []byte) (n int, err error) {
	return r.reader.Read(p)
}

type Json map[string]interface{}

func (d *AliDriver) setCache(key string, value interface{}) error {
	key = strings.TrimRight(key, "/")
	return d.alicache.Set(d.ctx, key, value, nil)
}

func (d *AliDriver) getCache(key string) (interface{}, error) {
	key = strings.TrimRight(key, "/")
	return d.alicache.Get(d.ctx, key)
}

func (d *AliDriver) deleteCache(key string) error {
	key = strings.TrimRight(key, "/")
	return d.alicache.Delete(d.ctx, key)
}

func (d *AliDriver) InitConfig(config interface{}) error {

	data, err := json.Marshal(config)
	if err != nil {
		return err
	}

	alidriverconfig := new(AliDriverConfig)
	err = json.Unmarshal(data, alidriverconfig)
	if err != nil {
		return err
	}
	d.config = *alidriverconfig
	d.client = resty.New()
	d.client.
		SetRetryCount(3).
		SetHeader("user-agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/87.0.4280.88 Safari/537.36").
		SetHeader("content-type", "application/json").
		SetHeader("origin", "https://www.aliyundrive.com")

	d.ctx = context.TODO()
	goCacheClient := goCache.New(30*time.Second, 30*time.Second)
	goCacheStore := store.NewGoCache(goCacheClient, nil)
	d.alicache = cache.New(goCacheStore)

	d.httpClient = &http.Client{}

	d.GetAndSetDriverID()

	return nil
}

func (d *AliDriver) RefreshToken() error {
	url := "https://auth.aliyundrive.com/v2/account/token"
	var resp TokenResp
	var e AliRespError
	_, err := d.client.R().
		//ForceContentType("application/json").
		SetBody(Json{"refresh_token": d.config.RefreshToken, "grant_type": "refresh_token"}).
		SetResult(&resp).
		SetError(&e).
		Post(url)
	if err != nil {
		d.config.Status = err.Error()
		return err
	}
	if e.Code != "" {
		d.config.Status = e.Message
		return fmt.Errorf("failed to refresh token: %s", e.Message)
	} else {
		d.config.Status = "work"
		d.config.RefreshToken = resp.RefreshToken
		d.AccessToken = resp.AccessToken
	}

	return nil
}

func (d *AliDriver) InitDriver(e *echo.Echo, db *gorm.DB) error {

	checkSignMiddleware := func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(ctx echo.Context) error {

			//检查连接的有效期
			expireTimeStr := utils.GetValue(ctx, "expireTime")
			if expireTimeStr == "" {
				return errors.New("必须包含一个expireTime参数")
			}

			expireTime, err := time.Parse(timeLayout, expireTimeStr)
			if err != nil {
				return err
			}

			if expireTime.Before(time.Now()) {
				return errors.New("链接已经失效")
			}

			path := utils.GetValue(ctx, "path")
			if path == "" {
				return errors.New("路径不能为空")
			}

			sign := utils.GetValue(ctx, "sign")
			if sign == "" {
				return errors.New("签名字符串不能为空")
			}

			key := d.config.Key
			method := jwt.GetSigningMethod("HS256")

			err = method.Verify(fmt.Sprintf("%s-%s", path, expireTimeStr), sign, []byte(key))
			if err != nil {
				return err
			}

			//校验通过....

			return next(ctx)
		}
	}

	e.PUT("/api/driver/alidriver", func(ctx echo.Context) error {

		filepath := utils.GetValue(ctx, "path")
		dataLength := utils.GetIntValueFromAnywhere(ctx, "Content-Length")

		body := ctx.Request().Body
		defer body.Close()

		dir := path.Dir(filepath)
		if dir == "." {
			dir = ""
		}
		name := path.Base(filepath)

		return d.Upload(UploaderFileStream{
			Name:       name,
			parentPath: dir,
			DataLength: dataLength,
			reader:     body,
		})

	}, checkSignMiddleware)

	e.DELETE("/api/driver/alidriver", func(ctx echo.Context) error {

		filepath := utils.GetValue(ctx, "path")
		return d.Delete(filepath)

	}, checkSignMiddleware)

	e.GET("/api/driver/alidriver", func(ctx echo.Context) error {

		filepath := utils.GetValue(ctx, "path")

		url, err := d.Link(filepath)
		if err != nil {
			return err
		}

		ctx.Response().Header().Add("Location", url)
		ctx.Response().WriteHeader(http.StatusMovedPermanently)

		return nil
	}, checkSignMiddleware)

	return nil
}

type AliWalkFunc func(path string, file *AliFile) error

func (d *AliDriver) Walk(path string, fn AliWalkFunc) error {
	file, err := d.File(path)
	if err != nil {
		return err
	}

	if file.Type == "folder" {
		alifiles, err := d.GetFiles(file.FileId)
		if err != nil {
			return err
		}
		for _, alifile := range alifiles {
			err = fn(filepath.Join(path, alifile.Name), &alifile)
			if err != nil {
				return err
			}
			if alifile.Type == "folder" {
				err = d.Walk(filepath.Join(path, alifile.Name), fn)
				if err != nil {
					return err
				}
			}
		}
	}

	return nil
}

func (d *AliDriver) WalkDir(key string) (*File, error) {

	formatPath := func(path string) string {
		path = strings.TrimRight(path, "/")
		if path == "" {
			return "/"
		}
		return path
	}

	key = formatPath(key)

	file, err := d.File(key)
	if err != nil {
		return nil, err
	}

	if file.Type != "folder" {
		return nil, fileerr.ErrNotDirectoy
	}

	var temp map[string]*File = map[string]*File{}

	root := &File{
		Name:         filepath.Base(key),
		IsDir:        true,
		AbsolutePath: key,
		Childrens:    []*File{},
	}

	temp[key] = root

	err = d.Walk(key, func(path string, alifile *AliFile) error {

		file := &File{
			Name:         filepath.Base(path),
			AbsolutePath: path,
		}
		if alifile.Type == "folder" {
			file.IsDir = true
			file.Childrens = []*File{}
			temp[path] = file
		} else {
			file.IsDir = false
			file.Size = alifile.Size
		}

		cacheKey := formatPath(filepath.Dir(path))
		if dirFile, ok := temp[cacheKey]; ok {
			dirFile.Childrens = append(dirFile.Childrens, file)
		}

		return nil
	})

	return root, err

}

func (d *AliDriver) PreUploadUrl(path string) (string, error) {
	expireTime := time.Now().Add(time.Hour * 2)
	expireTimeStr := expireTime.Format(timeLayout)

	key := d.config.Key
	method := jwt.GetSigningMethod("HS256")

	sign, err := method.Sign(fmt.Sprintf("%s-%s", path, expireTimeStr), []byte(key))
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("%s/api/driver/alidriver?path=%s&expireTime=%s&sign=%s", d.config.Host, path, expireTimeStr, sign), nil
}

func (d *AliDriver) MakeDir(path string) error {
	path = strings.TrimRight(path, "/")
	dir, name := filepath.Split(path)
	parentFile, err := d.File(dir)
	if err != nil {
		if err == fileerr.ErrFileNotFound {
			// 父文件夹不存在，就创建父文件夹
			err = d.MakeDir(dir)
			if err == nil {
				parentFile, err = d.File(dir)
			}
		}
		if err != nil {
			return err
		}
	}
	if parentFile.Type != "folder" {
		return fileerr.ErrCreateDirConflict
	}
	var resp Json
	var e AliRespError
	_, err = d.client.R().SetResult(&resp).SetError(&e).
		SetHeader("authorization", "Bearer\t"+d.AccessToken).
		SetBody(Json{
			"check_name_mode": "refuse",
			"drive_id":        d.DriverID,
			"name":            name,
			"parent_file_id":  parentFile.FileId,
			"type":            "folder",
		}).Post("https://api.aliyundrive.com/adrive/v2/file/createWithFolders")
	if err != nil {
		return err
	}
	if e.Code != "" {
		if e.Code == "AccessTokenInvalid" {
			err = d.RefreshToken()
			if err != nil {
				return err
			} else {
				return d.MakeDir(path)
			}
		}
		return fmt.Errorf("%s", e.Message)
	}
	if resp["file_name"] == name {
		_ = d.deleteCache(dir)
		return nil
	}
	return fmt.Errorf("%+v", resp)
}

func (d *AliDriver) Delete(filepath string) error {
	file, err := d.File(filepath)
	if err != nil {
		return err
	}
	var e AliRespError
	res, err := d.client.R().SetError(&e).
		SetHeader("authorization", "Bearer\t"+d.AccessToken).
		SetBody(Json{
			"drive_id": d.DriverID,
			"file_id":  file.FileId,
		}).Post("https://api.aliyundrive.com/v2/recyclebin/trash")
	if err != nil {
		return err
	}
	if e.Code != "" {
		if e.Code == "AccessTokenInvalid" {
			err = d.RefreshToken()
			if err != nil {
				return err
			} else {
				return d.Delete(filepath)
			}
		}
		return fmt.Errorf("%s", e.Message)
	}
	if res.StatusCode() == 204 {
		d.deleteCache(path.Dir(filepath))
		return nil
	}
	return errors.New(res.String())
}

func (d *AliDriver) Link(path string) (string, error) {
	file, err := d.File(path)
	if err != nil {
		return "", err
	}
	var resp Json
	var e AliRespError
	_, err = d.client.R().SetResult(&resp).
		SetError(&e).
		SetHeader("authorization", "Bearer\t"+d.AccessToken).
		SetBody(Json{
			"drive_id":   d.DriverID,
			"file_id":    file.FileId,
			"expire_sec": 14400,
		}).Post("https://api.aliyundrive.com/v2/file/get_download_url")
	if err != nil {
		return "", err
	}
	if e.Code != "" {
		if e.Code == "AccessTokenInvalid" {
			err = d.RefreshToken()
			if err != nil {
				return "", err
			} else {
				return d.Link(path)
			}
		}
		return "", fmt.Errorf("%s", e.Message)
	}
	return resp["url"].(string), nil
}

func (d *AliDriver) Upload(file UploaderFileStream) error {

	const DEFAULT uint64 = 10485760
	var count = int64(math.Ceil(float64(file.DataLength) / float64(DEFAULT)))
	var finish uint64 = 0
	parentFile, err := d.File(file.parentPath)
	if err != nil {
		if err == fileerr.ErrFileNotFound {
			err = d.MakeDir(file.parentPath)
			if err != nil {
				return err
			}
			parentFile, err = d.File(file.parentPath)
			if err != nil {
				return err
			}
		}
		if err != nil {
			return err
		}

	}
	if parentFile.Type != "folder" {
		return fileerr.ErrNotDirectoy
	}
	var resp UploadResp
	var e AliRespError

	partInfoList := make([]Json, 0)
	var i int64
	for i = 0; i < count; i++ {
		partInfoList = append(partInfoList, Json{
			"part_number": i + 1,
		})
	}
	_, err = d.client.R().SetResult(&resp).SetError(&e).
		SetHeader("authorization", "Bearer\t"+d.AccessToken).
		SetBody(Json{
			"check_name_mode": "auto_rename",
			// content_hash
			"content_hash_name": "none",
			"drive_id":          d.DriverID,
			"name":              file.Name,
			"parent_file_id":    parentFile.FileId,
			"part_info_list":    partInfoList,
			//proof_code
			"proof_version": "v1",
			"size":          file.DataLength,
			"type":          "file",
		}).Post("https://api.aliyundrive.com/adrive/v2/file/createWithFolders") // /v2/file/create_with_proof
	if err != nil {
		return err
	}
	if e.Code != "" {
		if e.Code == "AccessTokenInvalid" {
			err = d.RefreshToken()
			if err != nil {
				return err
			} else {
				return d.Upload(file)
			}
		}
		return fmt.Errorf("%s", e.Message)
	}
	var byteSize uint64
	for i = 0; i < count; i++ {
		byteSize = uint64(file.DataLength) - finish
		if DEFAULT < byteSize {
			byteSize = DEFAULT
		}
		log.Debugf("%d,%d", byteSize, finish)
		byteData := make([]byte, byteSize)
		n, err := io.ReadFull(file, byteData)
		//n, err := file.Read(byteData)
		//byteData, err := io.ReadAll(file)
		//n := len(byteData)
		log.Debug(err, n)
		if err != nil {
			return err
		}

		finish += uint64(n)

		req, err := http.NewRequest("PUT", resp.PartInfoList[i].UploadUrl, bytes.NewBuffer(byteData))
		if err != nil {
			return err
		}
		res, err := d.httpClient.Do(req)
		if err != nil {
			return err
		}
		log.Debugf("%+v", res)
	}
	var resp2 Json
	_, err = d.client.R().SetResult(&resp2).SetError(&e).
		SetHeader("authorization", "Bearer\t"+d.AccessToken).
		SetBody(Json{
			"drive_id":  d.DriverID,
			"file_id":   resp.FileId,
			"upload_id": resp.UploadId,
		}).Post("https://api.aliyundrive.com/v2/file/complete")
	if err != nil {
		return err
	}
	if e.Code != "" {
		//if e.Code == "AccessTokenInvalid" {
		//	err = driver.RefreshToken(account)
		//	if err != nil {
		//		return err
		//	} else {
		//		_ = model.SaveAccount(account)
		//		return driver.Upload(file, account)
		//	}
		//}
		return fmt.Errorf("%s", e.Message)
	}
	if resp2["file_id"] == resp.FileId {
		_ = d.deleteCache(file.parentPath)
		return nil
	}
	return fmt.Errorf("%+v", resp2)
}

func (d *AliDriver) File(path string) (*AliFile, error) {

	path = utils.ParsePath(path)
	if path == "/" {
		return &AliFile{
			Type:   "folder",
			FileId: d.config.RootID,
		}, nil
	}
	dir, name := filepath.Split(path)
	files, err := d.Files(dir)
	if err != nil {
		return nil, err
	}
	for _, file := range files {
		if file.Name == name {
			return &file, nil
		}
	}
	return nil, fileerr.ErrFileNotFound
}

func (d *AliDriver) GetAndSetDriverID() error {

	err := d.RefreshToken()
	if err != nil {
		return err
	}
	var resp Json
	_, _ = d.client.R().SetResult(&resp).
		SetBody("{}").
		SetHeader("authorization", "Bearer\t"+d.AccessToken).
		Post("https://api.aliyundrive.com/v2/user/get")
	log.Debugf("user info: %+v", resp)
	d.DriverID = resp["default_drive_id"].(string)
	return nil
}

func (d *AliDriver) Files(path string) ([]AliFile, error) {
	path = utils.ParsePath(path)
	var rawFiles []AliFile

	cache, err := d.getCache(path)
	if err == nil {
		rawFiles, _ = cache.([]AliFile)
	} else {
		file, err := d.File(path)
		if err != nil {
			return nil, err
		}
		rawFiles, err = d.GetFiles(file.FileId)
		if err != nil {
			return nil, err
		}
		if len(rawFiles) > 0 {
			_ = d.alicache.Set(d.ctx, path, rawFiles, nil)
			_ = d.setCache(path, rawFiles)
		}
	}

	return rawFiles, nil
}

func (d *AliDriver) GetFiles(fileId string) ([]AliFile, error) {
	marker := "first"
	res := make([]AliFile, 0)
	for marker != "" {
		if marker == "first" {
			marker = ""
		}
		var resp AliFiles
		var e AliRespError
		_, err := d.client.R().
			SetResult(&resp).
			SetError(&e).
			SetHeader("authorization", "Bearer\t"+d.AccessToken).
			SetBody(Json{
				"drive_id":                d.DriverID,
				"fields":                  "*",
				"image_thumbnail_process": "image/resize,w_400/format,jpeg",
				"image_url_process":       "image/resize,w_1920/format,jpeg",
				"limit":                   200,
				"marker":                  marker,
				"order_by":                "updated_at",
				"order_direction":         "DESC",
				"parent_file_id":          fileId,
				"video_thumbnail_process": "video/snapshot,t_0,f_jpg,ar_auto,w_300",
				"url_expire_sec":          14400,
			}).Post("https://api.aliyundrive.com/v2/file/list")
		if err != nil {
			return nil, err
		}
		if e.Code != "" {
			if e.Code == "AccessTokenInvalid" {
				err = d.RefreshToken()
				if err != nil {
					return nil, err
				} else {
					return d.GetFiles(fileId)
				}
			}
			return nil, fmt.Errorf("%s", e.Message)
		}
		marker = resp.NextMarker
		res = append(res, resp.Items...)
	}
	return res, nil
}

func (d *AliDriver) DownloadUrl(configs DownloadConfigs, path string) ([]*DownloadUrl, error) {

	downloadUrls := []*DownloadUrl{}

	expireTime := time.Now().Add(time.Hour * 2)
	expireTimeStr := expireTime.Format(timeLayout)

	key := d.config.Key
	method := jwt.GetSigningMethod("HS256")

	sign, err := method.Sign(fmt.Sprintf("%s-%s", path, expireTimeStr), []byte(key))
	if err == nil {
		downloadUrls = append(downloadUrls, &DownloadUrl{
			Title:       "阿里云高速下载线路",
			DownloadUrl: fmt.Sprintf("%s/api/driver/alidriver?path=%s&expireTime=%s&sign=%s", d.config.Host, path, expireTimeStr, sign),
		})
	}

	return downloadUrls, nil
}

func (d *AliDriver) PreDeleteUrl(path string) (string, error) {
	expireTime := time.Now().Add(time.Hour * 2)
	expireTimeStr := expireTime.Format(timeLayout)

	key := d.config.Key
	method := jwt.GetSigningMethod("HS256")

	sign, err := method.Sign(fmt.Sprintf("%s-%s", path, expireTimeStr), []byte(key))
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("%s/api/driver/alidriver?path=%s&expireTime=%s&sign=%s", d.config.Host, path, expireTimeStr, sign), nil
}
