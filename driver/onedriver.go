package driver

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/lixiaofei123/nextlist/utils"
	"github.com/robfig/cron/v3"
	"gorm.io/gorm"
)

var refreshTokenLock sync.Locker

func init() {
	refreshTokenLock = &sync.Mutex{}
	RegsiterDriver("onedriver", "OneDriver", &OneDriver{}, &OneDriverConfig{})
}

type OneDriverConfig struct {
	RefreshToken string `arg:"refreshToken;刷新Token;OneDriver所使用的刷新Token;required" json:"refreshToken"`
	ClientID     string `arg:"clientID;应用ID;注册的应用的ID;required" json:"clientID"`
	ClientSecret string `arg:"clientSecret;应用密钥;应用密钥;required" json:"clientSecret"`
	RedirectUrl  string `arg:"refirectUrl;跳转地址;暂时固定为https://tool.nn.ci/onedrive/callback;required" json:"refirectUrl"`
	Path         string `arg:"path;目录;要作为列表的onedriver的目录;required" json:"path"`
	Key          string `arg:"key;签名key;部分接口所需要使用的签名key，随意填写;required" json:"key"`
	Host         string `arg:"host;服务地址;请修改为Nextlist的外网地址;required" json:"host"`
}

type OneDriver struct {
	config      OneDriverConfig
	AccessToken string
}

func (d *OneDriver) Check() error {
	_, err := d.listDir("")
	return err

}

func (d *OneDriver) initConfig(config interface{}) error {

	onedriveConfig := config.(*OneDriverConfig)
	d.config = *onedriveConfig

	if d.config.Path == "" {
		d.config.Path = "/"
	}

	err := d.refreshToken()
	if err != nil {
		return err
	}

	cron.New().AddFunc("@midnight", func() {
		fmt.Println("定时刷新token")
		d.refreshToken()
	})

	return nil

}

func (d *OneDriver) InitDriver(e *echo.Group, db *gorm.DB) error {

	e.PUT("/driver/onedriver", func(ctx echo.Context) error {

		filepath := utils.GetValue(ctx, "path")
		dataLength := utils.GetIntValueFromAnywhere(ctx, "Content-Length")

		body := ctx.Request().Body
		defer body.Close()

		dir := path.Dir(filepath)
		name := path.Base(filepath)

		return d.Upload(UploaderFileStream{
			Name:       name,
			parentPath: dir,
			DataLength: dataLength,
			reader:     body,
		})

	}, checkSignHandler(d.config.Key))

	e.DELETE("/driver/onedriver", func(ctx echo.Context) error {

		filepath := utils.GetValue(ctx, "path")
		return d.Delete(filepath)

	}, checkSignHandler(d.config.Key))

	e.GET("/driver/onedriver", func(ctx echo.Context) error {

		filepath := utils.GetValue(ctx, "path")

		url, err := d.Link(filepath)
		if err != nil {
			return err
		}

		ctx.Response().Header().Add("Location", url)
		ctx.Response().WriteHeader(http.StatusMovedPermanently)

		return nil
	}, checkSignHandler(d.config.Key))

	return nil
}

type ODWalkFunc func(path string, file *ODFile) error

func (d *OneDriver) Walk(path string, fn ODWalkFunc) error {

	odfiles, err := d.listDir(path)
	if err != nil {
		return err
	}
	for _, odfile := range odfiles {
		err = fn(filepath.Join(path, odfile.Name), odfile)
		if err != nil {
			return err
		}
		if odfile.IsDir {
			err = d.Walk(filepath.Join(path, odfile.Name), fn)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func (d *OneDriver) WalkDir(key string) (*File, error) {

	formatPath := func(path string) string {
		path = strings.TrimRight(path, "/")
		if path == "" {
			return "/"
		}
		return path
	}

	key = formatPath(key)

	var temp map[string]*File = map[string]*File{}
	root := &File{
		Name:         filepath.Base(key),
		IsDir:        true,
		AbsolutePath: key,
		Childrens:    []*File{},
	}

	temp[key] = root

	err := d.Walk(key, func(path string, odfile *ODFile) error {

		file := &File{
			Name:         filepath.Base(path),
			AbsolutePath: path,
		}

		if odfile.IsDir {
			file.IsDir = true
			file.Childrens = []*File{}
			temp[path] = file
		} else {
			file.IsDir = false
			file.Size = int64(odfile.Size)
		}

		cacheKey := formatPath(filepath.Dir(path))
		if dirFile, ok := temp[cacheKey]; ok {
			dirFile.Childrens = append(dirFile.Childrens, file)
		}

		return nil
	})

	return root, err

}

func (d *OneDriver) PreUploadUrl(path string) (string, error) {
	return signUrl(fmt.Sprintf("%s/api/v1/driver/onedriver", d.config.Host), d.config.Key, path, time.Hour*2)
}

func (d *OneDriver) PreDeleteUrl(path string) (string, error) {
	return signUrl(fmt.Sprintf("%s/api/v1/driver/onedriver", d.config.Host), d.config.Key, path, time.Hour*2)
}

func (d *OneDriver) DownloadUrl(path string) ([]*DownloadUrl, error) {

	downloadUrls := []*DownloadUrl{}

	downloadUrl, err := signUrl(fmt.Sprintf("%s/api/v1/driver/onedriver", d.config.Host), d.config.Key, path, time.Hour*2)
	if err == nil {
		downloadUrls = append(downloadUrls, &DownloadUrl{
			Title:       "OneDriver高速下载线路",
			DownloadUrl: downloadUrl,
		})
	}

	return downloadUrls, nil
}

func (d *OneDriver) Delete(path string) error {

	filePath := filepath.Join(d.config.Path, path)

	client := http.Client{}

	request, err := http.NewRequest("DELETE", fmt.Sprintf("https://graph.microsoft.com/v1.0/me/drive/items/root:%s", filePath), nil)
	if err != nil {
		return err
	}
	request.Header.Set("Authorization", fmt.Sprintf("bearer %s", d.AccessToken))
	resp, err := client.Do(request)
	if err != nil {
		return err
	}

	if resp.StatusCode == 401 {
		err = d.refreshToken()
		if err != nil {
			return err
		} else {
			return d.Delete(path)
		}
	}

	if resp.StatusCode != 204 {
		return errors.New("删除文件失败")
	}

	return nil
}

type OnedriveErrorResp struct {
	Error Json `json:"error"`
}

func (d *OneDriver) Link(path string) (string, error) {
	filePath := filepath.Join(d.config.Path, path)

	client := http.Client{}

	request, err := http.NewRequest("GET", fmt.Sprintf("https://graph.microsoft.com/v1.0/me/drive/items/root:%s?$select=@microsoft.graph.downloadUrl", filePath), nil)
	if err != nil {
		return "", err
	}
	request.Header.Set("Authorization", fmt.Sprintf("bearer %s", d.AccessToken))
	resp, err := client.Do(request)
	if err != nil {
		return "", err
	}

	if resp.StatusCode == 401 {
		err = d.refreshToken()
		if err != nil {
			return "", err
		} else {
			return d.Link(path)
		}
	}

	if resp.StatusCode != 200 {
		return "", errors.New("获取直链失败")
	}

	defer resp.Body.Close()
	data, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", nil
	}

	downloadResp := Json{}
	err = json.Unmarshal(data, &downloadResp)
	if err != nil {
		return "", nil
	}

	return downloadResp["@microsoft.graph.downloadUrl"].(string), nil
}

func (d *OneDriver) Upload(file UploaderFileStream) error {
	if file.DataLength <= 1024*1024*3 {
		return d.uploadFile(file, true)
	} else {
		return d.uploadLargeFile(file, true)
	}
}

type ConflictBehaviorType string

const (
	Rename  ConflictBehaviorType = "rename"
	Fail    ConflictBehaviorType = "fail"
	Replace ConflictBehaviorType = "replace"
)

type UploadSessionItem struct {
	ConflictBehavior ConflictBehaviorType `json:"@microsoft.graph.conflictBehavior"`
}

type UploadSessionOption struct {
	Item UploadSessionItem `json:"item"`
}

type UploadSessionResp struct {
	UploadUrl          string `json:"uploadUrl"`
	ExpirationDateTime string `json:"expirationDateTime"`
}

func (d *OneDriver) uploadLargeFile(file UploaderFileStream, forceWrite bool) error {

	filePath := filepath.Join(d.config.Path, file.parentPath, file.Name)

	uploadSessionOption, _ := json.Marshal(&UploadSessionOption{
		Item: UploadSessionItem{
			ConflictBehavior: Replace,
		},
	})
	client := http.Client{}
	request, err := http.NewRequest("PUT", fmt.Sprintf("https://graph.microsoft.com/v1.0/me/drive/root:%s:/createUploadSession", filePath), bytes.NewBuffer(uploadSessionOption))
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", fmt.Sprintf("bearer %s", d.AccessToken))
	resp, err := client.Do(request)
	if err != nil {
		return err
	}

	if resp.StatusCode == 401 {
		err = d.refreshToken()
		if err != nil {
			return err
		} else {
			return d.uploadLargeFile(file, forceWrite)
		}
	}

	if resp.StatusCode != 200 {
		return fmt.Errorf("创建上传会话返回错误返回码%d", resp.StatusCode)
	}

	defer resp.Body.Close()

	var uploadSessionResp UploadSessionResp = UploadSessionResp{}
	respData, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	err = json.Unmarshal(respData, &uploadSessionResp)
	if err != nil {
		return err
	}

	uploadUrl := uploadSessionResp.UploadUrl
	chunk := 10 * 1024 * 1024

	retry := 0
	maxTryTimes := 5

	dataLength := file.DataLength
	var byteSize uint64

	for i := 0; i <= dataLength/chunk; i++ {

		startPos := i * chunk
		endPos := (i+1)*chunk - 1
		if endPos > dataLength-1 {
			endPos = dataLength - 1
		}

		byteSize = uint64(endPos - startPos + 1)

		putData := make([]byte, byteSize)
		_, err := io.ReadFull(file, putData)
		if err != nil {
			return err
		}
		request, err := http.NewRequest("PUT", uploadUrl, bytes.NewBuffer(putData))
		if err != nil {
			return err
		}
		request.Header.Set("Content-Type", "application/octet-stream")
		request.Header.Set("Authorization", fmt.Sprintf("bearer %s", d.AccessToken))
		request.Header.Set("Content-Length", strconv.Itoa(endPos-startPos+1))
		contentRange := fmt.Sprintf("bytes %d-%d/%d", startPos, endPos, dataLength)
		request.Header.Set("Content-Range", contentRange)

		resp, err := client.Do(request)
		if err != nil {
			return err
		}

		if resp.StatusCode == 401 {
			err = d.refreshToken()
			if err != nil {
				return err
			} else {
				return d.uploadLargeFile(file, forceWrite)
			}
		}

		if resp.StatusCode/100 != 2 {
			retry++
			if retry > maxTryTimes {
				return errors.New("上传失败，且超过最大重试次数")
			}
			i = i - 1
		} else {
			retry = 0
			if resp.StatusCode == 200 || resp.StatusCode == 201 {
				break
			}
		}

	}

	return nil
}

func (d *OneDriver) uploadFile(file UploaderFileStream, forceWrite bool) error {

	filePath := filepath.Join(d.config.Path, file.parentPath, file.Name)

	client := http.Client{}

	data, err := io.ReadAll(file)
	if err != nil {
		return err
	}

	request, err := http.NewRequest("PUT", fmt.Sprintf("https://graph.microsoft.com/v1.0/me/drive/root:%s:/content", filePath), bytes.NewBuffer(data))
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", "application/octet-stream")
	request.Header.Set("Authorization", fmt.Sprintf("bearer %s", d.AccessToken))
	resp, err := client.Do(request)
	if err != nil {
		return err
	}

	if resp.StatusCode == 401 {
		err = d.refreshToken()
		if err != nil {
			return err
		} else {
			return d.uploadFile(file, forceWrite)
		}
	}

	if resp.StatusCode != 201 {
		return errors.New("创建文件失败")
	}

	return nil
}

type ODListFileResp struct {
	Value []*Json `json:""`
}

type ODFile struct {
	Name  string
	Size  int
	IsDir bool
}

func (d *OneDriver) listDir(path string) ([]*ODFile, error) {

	filePath := filepath.Join(d.config.Path, path)

	client := http.Client{}

	request, err := http.NewRequest("GET", fmt.Sprintf("https://graph.microsoft.com/v1.0/me/drive/items/root:%s:/children", filePath), nil)
	if err != nil {
		return nil, err
	}
	request.Header.Set("Authorization", fmt.Sprintf("bearer %s", d.AccessToken))
	resp, err := client.Do(request)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode == 401 {
		err = d.refreshToken()
		if err != nil {
			return nil, err
		} else {
			return d.listDir(path)
		}
	}

	if resp.StatusCode != 200 {
		return nil, errors.New("获取目录列表失败")
	}

	defer resp.Body.Close()
	data, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, nil
	}

	fileResp := ODListFileResp{}
	err = json.Unmarshal(data, &fileResp)
	if err != nil {
		return nil, nil
	}

	odfiles := []*ODFile{}
	for _, json := range fileResp.Value {
		odfile := &ODFile{
			Name: (*json)["name"].(string),
		}
		if _, ok := (*json)["folder"]; ok {
			odfile.IsDir = true
		} else {
			odfile.Size = int((*json)["size"].(float64))
			odfile.IsDir = false
		}

		odfiles = append(odfiles, odfile)
	}

	return odfiles, nil
}

type OnedriverTokenResp struct {
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
	Scope        string `json:"scope"`
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
}

func (d *OneDriver) refreshToken() error {

	refreshTokenLock.Lock()
	defer refreshTokenLock.Unlock()

	formData := url.Values{
		"client_id":     {d.config.ClientID},
		"redirect_uri":  {d.config.RedirectUrl},
		"refresh_token": {d.config.RefreshToken},
		"client_secret": {d.config.ClientSecret},
		"grant_type":    {"refresh_token"},
	}
	client := &http.Client{}
	req, err := http.NewRequest("POST", "https://login.microsoftonline.com/common/oauth2/v2.0/token", strings.NewReader(formData.Encode()))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	var tokenResp OnedriverTokenResp = OnedriverTokenResp{}
	err = json.Unmarshal(body, &tokenResp)
	if err != nil {
		return err
	}

	d.AccessToken = tokenResp.AccessToken

	return nil

}
