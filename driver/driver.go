package driver

import (
	"encoding/json"
	"errors"
	"io"
	"reflect"
	"strings"

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

	initConfig(config interface{}) error

	InitDriver(e *echo.Group, db *gorm.DB) error

	PreUploadUrl(key string) (string, error)

	WalkDir(key string) (*File, error)

	PreDeleteUrl(key string) (string, error)

	DownloadUrl(key string) ([]*DownloadUrl, error)
}

type DriveConfig interface {
}

type DownloadUrl struct {
	Title       string `json:"title"`
	DownloadUrl string `json:"downloadUrl"`
}

type PropType string

const (
	Int     PropType = "int"
	String  PropType = "string"
	Boolean PropType = "boolean"
)

type Property struct {
	Name     string   `json:"name"`
	Label    string   `json:"label"`
	PropType PropType `json:"type"`
	Required bool     `json:"required"`
	Usage    string   `json:"usage"`
}

type DriveConfigProp struct {
	Name       string     `json:"name"`
	ShowName   string     `json:"showname"`
	Properties []Property `json:"properties"`
}

var drivers map[string]Driver = map[string]Driver{}
var driveConfigs map[string]DriveConfig = map[string]DriveConfig{}
var driveProps map[string]DriveConfigProp = map[string]DriveConfigProp{}

func RegsiterDriver(name string, showName string, driver Driver, driveConfig DriveConfig) {
	drivers[name] = driver
	driveConfigs[name] = driveConfig

	//解析driveConfig属性
	confType := reflect.TypeOf(driveConfig).Elem()
	properties := []Property{}
	for i := 0; i < confType.NumField(); i++ {
		prop := GetNameAndUsages(confType, i)
		properties = append(properties, prop)
	}
	driveProps[name] = DriveConfigProp{
		Name:       name,
		ShowName:   showName,
		Properties: properties,
	}

}

func GetDriverProps() map[string]DriveConfigProp {
	return driveProps
}

func GetDriver(name string, config map[string]interface{}) (Driver, error) {

	data, err := json.Marshal(config)
	if err != nil {
		return nil, err
	}

	driverconfig, ok := driveConfigs[name]
	if !ok {
		return nil, errors.New("未找到合适的存储驱动")
	}

	err = json.Unmarshal(data, driverconfig)
	if err != nil {
		return nil, errors.New("存储配置错误")
	}

	if driver, ok := drivers[name]; ok {
		driver.initConfig(driverconfig)
		return driver, nil
	}
	return nil, errors.New("未找到合适的存储驱动")
}

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

func GetNameAndUsages(otype reflect.Type, i int) Property {
	field := otype.Field(i)
	name := field.Name
	var argname string = strings.ToLower(name)

	tag := field.Tag
	tagvalue := tag.Get("arg")
	arr := strings.Split(tagvalue, ";")
	if len(arr) >= 1 {
		argname = arr[0]
	}

	var label string
	if len(arr) >= 2 {
		label = arr[1]
	}

	var argusage string
	if len(arr) >= 3 {
		argusage = arr[2]
	}

	required := false
	if len(arr) >= 4 && arr[3] == "required" {
		required = true
	}

	proptype := String

	switch field.Type.Kind() {
	case reflect.Int:
		fallthrough
	case reflect.Int64:
		proptype = Int
	case reflect.Bool:
		proptype = Boolean
	case reflect.String:
		proptype = String
	default:
		panic("不支持的数据类型")
	}

	return Property{
		Name:     argname,
		Label:    label,
		PropType: proptype,
		Required: required,
		Usage:    argusage,
	}
}
