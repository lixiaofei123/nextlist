package driver

import (
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gabriel-vasile/mimetype"
	"github.com/labstack/echo/v4"
	"github.com/lixiaofei123/nextlist/utils"
	"gorm.io/gorm"
)

func init() {
	RegsiterDriver("file", "文件存储", &FileDriver{}, &FileDriverConfig{})
}

var nginx_path string = "/usr/share/nginx/html/upload/"

type FileDriverConfig struct {
	Path     string `arg:"path;路径;文件存储路径;required" json:"path"`
	Key      string `arg:"key;签名key;部分接口所需要使用的签名key,随意填写;required" json:"key"`
	Host     string `arg:"host;服务地址;NextList服务地址,需要外网能够访问;required" json:"host"`
	UseNginx bool   `arg:"useNginx;使用Nginx;此模式下文件存储路径将被固定为/usr/share/nginx/html/upload/;required" josn:"useNginx"`
}

type FileDriver struct {
	config FileDriverConfig
	path   string
}

func (d *FileDriver) initConfig(config interface{}) error {

	fileconfig := config.(*FileDriverConfig)
	d.config = *fileconfig
	d.path = d.config.Path
	if d.config.UseNginx {
		d.path = nginx_path
		os.Mkdir(nginx_path, 0755)
	}

	return nil
}

func (d *FileDriver) Check() error {

	tempPath := path.Join(d.path, "test_temp")
	err := os.WriteFile(tempPath, []byte("test!!!"), 0755)
	if err != nil {
		return err
	}
	return os.Remove(tempPath)

}

func (d *FileDriver) InitDriver(e *echo.Group, db *gorm.DB) error {

	e.PUT("/driver/file", func(ctx echo.Context) error {

		filepath := utils.GetValue(ctx, "path")

		body := ctx.Request().Body
		defer body.Close()

		absPath := path.Join(d.path, filepath)
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
	}, checkSignHandler(d.config.Key))

	e.DELETE("/driver/file", func(ctx echo.Context) error {

		filepath := utils.GetValue(ctx, "path")
		absPath := path.Join(d.path, filepath)
		os.Remove(absPath)
		ctx.Response().Status = http.StatusOK
		return nil
	}, checkSignHandler(d.config.Key))

	e.GET("/driver/file", func(ctx echo.Context) error {

		filepath := utils.GetValue(ctx, "path")
		absPath := path.Join(d.path, filepath)

		data, err := os.ReadFile(absPath)
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
	}, checkSignHandler(d.config.Key))

	return nil
}

func (d *FileDriver) WalkDir(key string) (*File, error) {

	key = strings.TrimRight(key, "/")
	if key == "" {
		key = "/"
	}

	absPath := path.Join(d.path, key)

	var temp map[string]*File = map[string]*File{}

	root := &File{
		Name:         filepath.Base(key),
		IsDir:        true,
		AbsolutePath: key,
		Childrens:    []*File{},
	}

	formatPath := func(path string) string {
		path = strings.TrimPrefix(path, d.path)
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

	return signUrl(fmt.Sprintf("%s/api/v1/driver/file", d.config.Host), d.config.Key, path, time.Hour*2)
}

func (d *FileDriver) PreDeleteUrl(path string) (string, error) {
	return signUrl(fmt.Sprintf("%s/api/v1/driver/file", d.config.Host), d.config.Key, path, time.Hour*2)
}

func (d *FileDriver) DownloadUrl(path string) ([]*DownloadUrl, error) {

	downloadUrls := []*DownloadUrl{}

	if d.config.UseNginx {
		downloadUrls = append(downloadUrls, &DownloadUrl{
			Title:       "下载链接",
			DownloadUrl: fmt.Sprintf("%s/upload/%s", d.config.Host, path),
		})
	} else {
		downloadUrl, err := signUrl(fmt.Sprintf("%s/api/v1/driver/file", d.config.Host), d.config.Key, path, time.Hour*2)

		if err == nil {
			downloadUrls = append(downloadUrls, &DownloadUrl{
				Title:       "下载链接",
				DownloadUrl: downloadUrl,
			})
		}
	}

	return downloadUrls, nil
}
