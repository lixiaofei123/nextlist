package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"strings"

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

func initApp() (driver.Driver, bool, error) {

	var err error
	var debug bool

	flag.BoolVar(&debug, "d", false, "debug mode")
	flag.Parse()

	err = configs.InitConfig()
	if err != nil {
		return nil, false, err
	}

	driverConfig := configs.GlobalConfig.DriverConfig
	driverName := driverConfig.Name

	sdriver, err := driver.GetDriver(driverName, driverConfig.Config)
	if err != nil {
		log.Panic(err)
	}

	return sdriver, debug, nil
}

func CustomHTTPErrorHandler(err error, c echo.Context) {
	c.Response().Status = http.StatusInternalServerError
	c.Response().Write([]byte(fmt.Sprintf(`{"code":%d,"data":"%s"}`, http.StatusInternalServerError, err.Error())))
}

func main() {

	// 初始化echo
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

	sdriver, debug, loaderr := initApp()
	if debug {
		e.Use(echo_middleware.Logger())
	}

	e.Use(echo_middleware.Recover())
	e.Use(echo_middleware.CORSWithConfig(echo_middleware.CORSConfig{
		AllowOrigins:     []string{"*"},
		AllowHeaders:     []string{"*"},
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "HEAD"},
		AllowCredentials: true,
	}))

	e.HTTPErrorHandler = CustomHTTPErrorHandler

	apiv1 := e.Group("/api/v1")

	apiv1.Add("POST", "/site/status", func(ctx echo.Context) error {
		if loaderr != nil {
			ctx.Response().Writer.Write([]byte(`{"ready":false}`))
		} else {
			ctx.Response().Writer.Write([]byte(`{"ready":true}`))
		}

		return nil
	})

	registerServiceRoutes := func() {
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

		driverConfig := configs.GlobalConfig.DriverConfig
		driverName := driverConfig.Name

		sdriver, err = driver.GetDriver(driverName, driverConfig.Config)
		if err != nil {
			log.Panic(err)
		}

		err = sdriver.InitDriver(apiv1, db)
		if err != nil {
			log.Panic(err)
		}

		userSrv := services.NewUserService(db)
		fileSrv := services.NewFileService(db, sdriver)

		user := apiv1.Group("/user")
		user.Use(middleware.NotMustAuthHandler)
		mvc.New(user).Handle(controller.NewUserController(userSrv))

		file := apiv1.Group("/file")
		file.Use(middleware.NotMustAuthHandler)
		mvc.New(file).Handle(controller.NewFileController(fileSrv))

		adminapi := apiv1.Group("/admin")
		adminapi.Use(middleware.AuthHandler)
		mvc.New(adminapi).Handle(controller.NewAdminFileController(fileSrv, sdriver))

		siteapi := apiv1.Group("/site")
		mvc.New(siteapi).Handle(controller.NewSiteController(userSrv))

		log.Println("程序已经运行......")
	}

	if loaderr != nil {
		// 站点未初始化，需要先进行初始化后再使用
		init := apiv1.Group("/init")
		init.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
			return func(ctx echo.Context) error {
				path := ctx.Request().URL.Path
				if strings.Contains(path, "/api/v1/init") && loaderr == nil {
					ctx.JSON(http.StatusNotFound, controller.DataResponse{
						Code: http.StatusNotFound,
						Data: "站点已经初始化完毕，请勿调用",
					})
					return nil
				}
				return next(ctx)
			}
		})

		mvc.New(init).Handle(controller.NewInitController())

		init.Add("POST", "/reload", func(ctx echo.Context) error {
			cerr := configs.InitConfig()
			if cerr != nil {
				return cerr
			}
			registerServiceRoutes()
			loaderr = nil
			return nil
		})

		log.Println("初始化程序已经运行......")
		e.Logger.Fatal(e.Start(":8081"))
	}

	registerServiceRoutes()
	e.Logger.Fatal(e.Start(":8081"))

}
