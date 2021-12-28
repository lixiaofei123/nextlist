package controller

import (
	"time"

	"github.com/dgrijalva/jwt-go"
	"github.com/go-playground/validator"
	"github.com/labstack/echo/v4"

	"github.com/lixiaofei123/nextlist/configs"
	fileerr "github.com/lixiaofei123/nextlist/errors"
	models "github.com/lixiaofei123/nextlist/models"
	services "github.com/lixiaofei123/nextlist/services"
	utils "github.com/lixiaofei123/nextlist/utils"
	mvc "github.com/lixiaofei123/nextlist/web/mvc"
)

type UserController struct {
	userSrv services.UserService
}

func NewUserController(userSrv services.UserService) *UserController {
	return &UserController{
		userSrv: userSrv,
	}
}

func (u *UserController) PostLogin(ctx echo.Context) mvc.Result {

	username := utils.GetValue(ctx, "username")
	password := utils.GetValue(ctx, "password")
	loginType := utils.GetIntValueWithDefault(ctx, "loginType", 0)

	user, err := u.userSrv.Login(username, password, services.LoginType(loginType))
	if err != nil {
		return HandleData(nil, err)
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, models.JWTClaims{
		Role:     string(user.Role),
		Email:    user.Email,
		ShowName: user.ShowName,
		StandardClaims: jwt.StandardClaims{
			ExpiresAt: time.Now().Add(7 * 24 * time.Hour).Unix(),
			Issuer:    user.UserName,
		},
	})

	authorization, _ := token.SignedString([]byte(configs.GlobalConfig.Auth.Secret))

	return HandleData(authorization, err)
}

func (u *UserController) PostRegister(ctx echo.Context, user models.User) mvc.Result {

	userCount, err := u.userSrv.UserCount()
	if err != nil {
		return HandleData(nil, err)
	}

	if !configs.GlobalConfig.SiteConfig.AllowRegister && userCount != 0 {
		return HandleData(nil, fileerr.ErrRegisterIsDisabled)
	}

	updateUser, err := u.userSrv.Register(&user)
	if err != nil {
		if errs, ok := err.(validator.ValidationErrors); ok {
			return HandleData(nil, errs)
		}
		return HandleData(nil, err)
	}

	return HandleData(updateUser, nil)
}

func (u *UserController) GetInfo(ctx echo.Context) mvc.Result {

	email := ctx.Request().Header.Get("email")
	role := ctx.Request().Header.Get("role")
	username := ctx.Request().Header.Get("username")
	showname := ctx.Request().Header.Get("showname")

	if username == "" {
		return HandleData(nil, fileerr.ErrNeedLogin)
	}

	return HandleData(&models.User{
		UserName: username,
		ShowName: showname,
		Role:     models.Role(role),
		Email:    email,
	}, nil)
}
