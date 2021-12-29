package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	echo "github.com/labstack/echo/v4"
	echo_middleware "github.com/labstack/echo/v4/middleware"

	"github.com/lixiaofei123/nextlist/configs"
	"github.com/lixiaofei123/nextlist/driver"
	"github.com/lixiaofei123/nextlist/models"
	"github.com/lixiaofei123/nextlist/services"
	"github.com/lixiaofei123/nextlist/web/controller"
	"github.com/lixiaofei123/nextlist/web/middleware"
	"github.com/lixiaofei123/nextlist/web/mvc"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

var debug bool
var sdriver driver.Driver

func init() {

	var err error

	// 时区
	timezone := os.Getenv("TZ")
	if timezone == "" {
		timezone = "Asia/Shanghai"
	}

	loc, err := time.LoadLocation(timezone)
	if err != nil {
		log.Panic(err)
	}

	time.Local = loc

	var configPath string
	flag.StringVar(&configPath, "c", "./config.yaml", "配置文件地址")
	flag.BoolVar(&debug, "d", false, "debug mode")
	flag.Parse()

	err = configs.InitConfig(configPath)
	if err != nil {
		log.Panic(err)
	}

	driverName := configs.GlobalConfig.DriverConfig.Name

	sdriver, err = driver.GetDriver(driverName)
	if err != nil {
		log.Panic(err)
	}

	err = sdriver.InitConfig(configs.GlobalConfig.DriverConfig.Config)
	if err != nil {
		log.Panic(err)
	}
}

func CustomHTTPErrorHandler(err error, c echo.Context) {
	c.Response().Status = http.StatusInternalServerError
	c.Response().Write([]byte(fmt.Sprintf(`{"code":%d,"data":"%s"}`, http.StatusInternalServerError, err.Error())))
}

func main() {

	gormDebugLevel := gormlogger.Error
	if debug {
		gormDebugLevel = gormlogger.Info
	}

	db, err := gorm.Open(mysql.Open(fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?parseTime=true",
		configs.GlobalConfig.DataBase.Mysql.Username,
		configs.GlobalConfig.DataBase.Mysql.Password,
		configs.GlobalConfig.DataBase.Mysql.Url,
		configs.GlobalConfig.DataBase.Mysql.Port,
		configs.GlobalConfig.DataBase.Mysql.Database,
	)), &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormDebugLevel),
	})

	if err != nil {
		log.Panic(err)
	}

	err = db.AutoMigrate(&models.User{})
	if err != nil {
		log.Panic(err)
	}

	err = db.AutoMigrate(&models.File{})
	if err != nil {
		log.Panic(err)
	}

	e := echo.New()
	e.IPExtractor = func(r *http.Request) string {
		IPAddress := r.Header.Get("X-Real-Ip")
		if IPAddress == "" {
			IPAddress = r.Header.Get("X-Forwarded-For")
		}
		if IPAddress == "" {
			IPAddress = r.RemoteAddr
		}
		return IPAddress
	}
	if debug {
		e.Use(echo_middleware.Logger())
	}

	err = sdriver.InitDriver(e, db)
	if err != nil {
		log.Panic(err)
	}

	userSrv := services.NewUserService(db)
	fileSrv := services.NewFileService(db, sdriver)

	e.Use(echo_middleware.Recover())
	e.Use(echo_middleware.CORSWithConfig(echo_middleware.CORSConfig{
		AllowOrigins:     []string{"*"},
		AllowHeaders:     []string{"*"},
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "HEAD"},
		AllowCredentials: true,
	}))

	e.HTTPErrorHandler = CustomHTTPErrorHandler

	user := e.Group("/user")
	user.Use(middleware.NotMustAuthHandler)
	mvc.New(user).Handle(controller.NewUserController(userSrv))

	api := e.Group("/api")
	api.Use(middleware.NotMustAuthHandler)
	mvc.New(api).Handle(controller.NewFileController(fileSrv))

	adminapi := e.Group("/admin/api")
	adminapi.Use(middleware.AuthHandler)
	mvc.New(adminapi).Handle(controller.NewAdminFileController(fileSrv, sdriver))

	siteapi := e.Group("/site")
	mvc.New(siteapi).Handle(controller.NewSiteController(userSrv))

	e.Logger.Fatal(e.Start(fmt.Sprintf(":%d", configs.GlobalConfig.Port)))
}
