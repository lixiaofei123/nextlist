package controller

import (
	"github.com/labstack/echo/v4"

	"github.com/lixiaofei123/nextlist/configs"
	services "github.com/lixiaofei123/nextlist/services"
	mvc "github.com/lixiaofei123/nextlist/web/mvc"
	"github.com/mohae/deepcopy"
)

type SiteController struct {
	userSrv services.UserService
}

func NewSiteController(userSrv services.UserService) *SiteController {
	return &SiteController{
		userSrv: userSrv,
	}
}

func (u *SiteController) GetConfig(ctx echo.Context) mvc.Result {

	config := deepcopy.Copy(&configs.GlobalConfig.SiteConfig).(*configs.SiteConfig)

	if !config.AllowRegister {
		count, err := u.userSrv.UserCount()
		if err != nil {
			return HandleData(config, err)
		}
		if count == 0 {
			config.AllowRegister = true
		} else {
			config.AllowRegister = false
		}
	}

	return HandleData(config, nil)
}
