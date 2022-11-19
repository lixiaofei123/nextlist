package driver

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"io/ioutil"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gabriel-vasile/mimetype"
	"github.com/golang-jwt/jwt"
	"github.com/labstack/echo/v4"
	"github.com/lixiaofei123/nextlist/utils"
	"gorm.io/gorm"
)

func init() {
	RegsiterDriver("file", "文件存储", &FileDriver{}, &FileDriverConfig{})
}

type FileDriverConfig struct {
	Path string `arg:"path;路径;文件存储路径;required" json:"path"`
	Key  string `arg:"key;签名key;部分接口所需要使用的签名key,随意填写;required" json:"key"`
	Host string `arg:"host;服务地址;Nextlist服务地址;required" json:"host"`
}

type FileDriver struct {
	config FileDriverConfig
}

func (d *FileDriver) initConfig(config interface{}) error {

	fileconfig := config.(*FileDriverConfig)
	d.config = *fileconfig

	return nil
}

const timeLayout string = "2006-01-02 15:04:05"

func (d *FileDriver) Check() error {
	tempPath := path.Join(d.config.Path, "test_temp")
	err := ioutil.WriteFile(tempPath, []byte("test!!!"), 0755)
	if err != nil {
		return err
	}
	return os.Remove(tempPath)

}

func (d *FileDriver) InitDriver(e *echo.Group, db *gorm.DB) error {

	checkSignMiddleware := func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(ctx echo.Context) error {

			// 检查连接的有效期
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

			// 校验通过....

			return next(ctx)
		}
	}

	e.PUT("/driver/file", func(ctx echo.Context) error {

		filepath := utils.GetValue(ctx, "path")

		body := ctx.Request().Body
		defer body.Close()

		absPath := path.Join(d.config.Path, filepath)
		dir := path.Dir(absPath)

		err := os.MkdirAll(dir, 0751)
		if err != nil {
			return err
		}

		dstFile, err := os.Create(absPath)
		if err != nil {
			return err
		}

		defer dstFile.Close()

		_, err = io.Copy(dstFile, body)
		if err != nil {
			return err
		}

		ctx.Response().Status = http.StatusCreated

		return nil
	}, checkSignMiddleware)

	e.DELETE("/driver/file", func(ctx echo.Context) error {

		filepath := utils.GetValue(ctx, "path")
		absPath := path.Join(d.config.Path, filepath)
		os.Remove(absPath)
		ctx.Response().Status = http.StatusOK
		return nil
	}, checkSignMiddleware)

	e.GET("/driver/file", func(ctx echo.Context) error {

		filepath := utils.GetValue(ctx, "path")
		absPath := path.Join(d.config.Path, filepath)

		data, err := ioutil.ReadFile(absPath)
		if err != nil {
			return err
		}

		ctx.Response().Status = http.StatusOK
		length := len(data)
		ctx.Response().Header().Add("Content-Length", strconv.Itoa(length))
		ctx.Response().Header().Add("Content-Disposition", fmt.Sprintf("attachment;filename=%s", path.Base(filepath)))

		mtype := mimetype.Detect(data)
		if mtype != nil {
			ctx.Response().Header().Add("Content-Type", mtype.String())
		}

		_, err = ctx.Response().Write(data)
		if err != nil {
			return err
		}

		return nil
	}, checkSignMiddleware)

	return nil
}

func (d *FileDriver) WalkDir(key string) (*File, error) {

	key = strings.TrimRight(key, "/")
	if key == "" {
		key = "/"
	}

	absPath := path.Join(d.config.Path, key)

	var temp map[string]*File = map[string]*File{}

	root := &File{
		Name:         filepath.Base(key),
		IsDir:        true,
		AbsolutePath: key,
		Childrens:    []*File{},
	}

	formatPath := func(path string) string {
		path = strings.TrimPrefix(path, d.config.Path)
		path = strings.TrimRight(path, "/")
		if path == "" || path == "." {
			return "/"
		}
		if !strings.HasPrefix(path, "/") {
			path = "/" + path
		}
		return path
	}

	temp[formatPath(key)] = root

	filepath.Walk(absPath, func(path string, info fs.FileInfo, err error) error {
		path = formatPath(path)
		if key != path {
			file := &File{
				Name:         filepath.Base(path),
				AbsolutePath: path,
			}
			if info.IsDir() {
				file.IsDir = true
				file.Childrens = []*File{}
				temp[path] = file
			} else {
				file.IsDir = false
				file.Size = info.Size()
			}

			cacheKey := formatPath(filepath.Dir(path))
			if dirFile, ok := temp[cacheKey]; ok {
				dirFile.Childrens = append(dirFile.Childrens, file)
			}

		}
		return nil
	})

	return root, nil

}

func (d *FileDriver) PreUploadUrl(path string) (string, error) {

	expireTime := time.Now().Add(time.Hour * 2)
	expireTimeStr := expireTime.Format(timeLayout)

	key := d.config.Key
	method := jwt.GetSigningMethod("HS256")

	sign, err := method.Sign(fmt.Sprintf("%s-%s", path, expireTimeStr), []byte(key))
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("%s/api/v1/driver/file?path=%s&expireTime=%s&sign=%s", d.config.Host, path, expireTimeStr, sign), nil
}

func (d *FileDriver) PreDeleteUrl(path string) (string, error) {

	expireTime := time.Now().Add(time.Hour * 2)
	expireTimeStr := expireTime.Format(timeLayout)

	key := d.config.Key
	method := jwt.GetSigningMethod("HS256")

	sign, err := method.Sign(fmt.Sprintf("%s-%s", path, expireTimeStr), []byte(key))
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("%s/api/v1/driver/file?path=%s&expireTime=%s&sign=%s", d.config.Host, path, expireTimeStr, sign), nil
}

func (d *FileDriver) DownloadUrl(path string) ([]*DownloadUrl, error) {

	downloadUrls := []*DownloadUrl{}

	expireTime := time.Now().Add(time.Hour * 2)
	expireTimeStr := expireTime.Format(timeLayout)

	key := d.config.Key
	method := jwt.GetSigningMethod("HS256")

	sign, err := method.Sign(fmt.Sprintf("%s-%s", path, expireTimeStr), []byte(key))
	if err == nil {
		downloadUrls = append(downloadUrls, &DownloadUrl{
			Title:       "原始链接",
			DownloadUrl: fmt.Sprintf("%s/api/v1/driver/file?path=%s&expireTime=%s&sign=%s", d.config.Host, path, expireTimeStr, sign),
		})
	}

	return downloadUrls, nil
}
