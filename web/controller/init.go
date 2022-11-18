package controller

import (
	"fmt"
	"os"

	"github.com/labstack/echo/v4"
	"github.com/lixiaofei123/nextlist/configs"
	"github.com/lixiaofei123/nextlist/driver"
	mvc "github.com/lixiaofei123/nextlist/web/mvc"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

type InitController struct {
}

func NewInitController() *InitController {
	return &InitController{}
}

func (c *InitController) PostConfig(ctx echo.Context, config configs.Config) mvc.Result {

	// 先检查配置

	err := checkDb(&config.DataBase.Mysql)
	if err != nil {
		return HandleData(nil, err)
	}

	err = checkDriver(&config.DriverConfig)
	if err != nil {
		return HandleData(nil, err)
	}

	// 最后写入配置文件

	err = configs.WriteConfig(&config)
	if err != nil {
		return HandleData(nil, err)
	}
	return HandleData("ok", nil)
}

func (c *InitController) PostCheckDb(ctx echo.Context, dbconfig configs.Mysql) mvc.Result {

	err := checkDb(&dbconfig)
	if err != nil {
		return HandleData(nil, err)
	}

	return HandleData("ok", nil)
}

func (c *InitController) PostCheckDriver(ctx echo.Context, driverConfig configs.DriverConfig) mvc.Result {

	err := checkDriver(&driverConfig)
	if err != nil {
		return HandleData(nil, err)
	}

	return HandleData("ok", nil)
}

func (c *InitController) GetDriverprops(ctx echo.Context) mvc.Result {

	props := driver.GetDriverProps()
	return HandleData(props, nil)
}

func (c *InitController) PostRestart(ctx echo.Context) mvc.Result {

	// 退出程序
	os.Exit(0)

	return HandleData("ok", nil)
}

func checkDriver(driverConfig *configs.DriverConfig) error {
	sdriver, err := driver.GetDriver(driverConfig.Name, driverConfig.Config)
	if err != nil {
		return err
	}

	err = sdriver.Check()
	if err != nil {
		return err
	}

	return nil
}

func checkDb(dbconfig *configs.Mysql) error {
	db, err := gorm.Open(mysql.Open(fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?parseTime=true",
		dbconfig.Username,
		dbconfig.Password,
		dbconfig.Url,
		dbconfig.Port,
		dbconfig.Database,
	)))

	if err != nil {
		return err
	}

	err = db.Exec("select 1").Error
	if err != nil {
		return err
	}
	return nil
}
