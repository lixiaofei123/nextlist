package driver

import (
	"errors"
	"io"

	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
)

type File struct {
	Name         string
	IsDir        bool
	Childrens    []*File
	Size         int64
	AbsolutePath string
}

type Driver interface {
	Check() error

	InitConfig(config interface{}) error

	InitDriver(e *echo.Echo, db *gorm.DB) error

	PreUploadUrl(key string) (string, error)

	WalkDir(key string) (*File, error)

	PreDeleteUrl(key string) (string, error)

	DownloadUrl(configs DownloadConfigs, key string) ([]*DownloadUrl, error)
}

type DownloadUrl struct {
	Title       string `json:"title"`
	DownloadUrl string `json:"downloadUrl"`
}

var drivers map[string]Driver = map[string]Driver{}

func RegsiterDriver(name string, driver Driver) {
	drivers[name] = driver
}

func GetDriver(name string) (Driver, error) {

	if driver, ok := drivers[name]; ok {
		return driver, nil
	}
	return nil, errors.New("未找到合适的存储驱动")
}

type DownloadConfig struct {
	Title string `yaml:"title"`
	Url   string `yaml:"url"`
}

type DownloadConfigs []*DownloadConfig

type Json map[string]interface{}

type UploaderFileStream struct {
	Name       string
	parentPath string
	DataLength int
	reader     io.ReadCloser
}

func (r UploaderFileStream) Read(p []byte) (n int, err error) {
	return r.reader.Read(p)
}
