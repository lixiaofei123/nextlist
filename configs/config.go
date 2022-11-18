package configs

import (
	"io/ioutil"
	"os"
	"path"
	"sync"

	"github.com/go-yaml/yaml"
	"github.com/lixiaofei123/nextlist/driver"
)

type Mysql struct {
	Url      string `yaml:"url" json:"url"`
	Port     int    `yaml:"port" json:"port"`
	Database string `yaml:"database" json:"database"`
	Username string `yaml:"username" json:"username"`
	Password string `ysml:"password" json:"password"`
}

type DataBase struct {
	Mysql Mysql `ysml:"mysql" json:"mysql"`
}

type Auth struct {
	Secret string `yaml:"secret" json:"secret"`
}

type SiteConfig struct {
	AllowRegister bool   `yaml:"allowRegister" json:"allowRegister"`
	Title         string `yaml:"title" json:"title"`
	CopyRight     string `yaml:"copyright" json:"copyright"`
}

type DriverConfig struct {
	Name     string                 `yaml:"name" json:"name"`
	Config   map[string]interface{} `yaml:"config" json:"config"`
	Download driver.DownloadConfigs `yaml:"download" json:"download"`
}

type Config struct {
	DataBase     DataBase     `yaml:"database" json:"database"`
	Auth         Auth         `yaml:"auth" json:"auth"`
	DriverConfig DriverConfig `yaml:"driver" json:"driver"`
	SiteConfig   SiteConfig   `yaml:"site" json:"site"`
}

var GlobalConfig *Config

var configLock sync.RWMutex = sync.RWMutex{}

const configFilePath string = "/app/config/config.yaml"

func InitConfig() error {
	var err error
	GlobalConfig, err = readConfig(configFilePath)
	return err
}

func readConfig(configPath string) (*Config, error) {
	configLock.RLock()
	data, err := ioutil.ReadFile(configPath)
	configLock.RLocker().Unlock()
	if err != nil {
		return nil, err
	}
	config := Config{}
	err = yaml.Unmarshal(data, &config)
	if err != nil {
		return nil, err
	}
	return &config, nil
}

func ReadConfig() Config {
	return *GlobalConfig
}

func writeConfig(config *Config) error {
	data, err := yaml.Marshal(config)
	if err != nil {
		return err
	}

	dir := path.Dir(configFilePath)
	err = os.MkdirAll(dir, 0755)
	if err != nil {
		return err
	}

	configLock.Lock()
	err = ioutil.WriteFile(configFilePath, data, 0660)
	configLock.Unlock()
	if err != nil {
		return err
	}
	return err
}

func WriteConfig(config *Config) error {
	return writeConfig(config)
}
