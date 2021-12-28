package middleware

import (
	"net/http"

	"github.com/golang-jwt/jwt"
	"github.com/labstack/echo/v4"
	"github.com/lixiaofei123/nextlist/configs"
	fileerr "github.com/lixiaofei123/nextlist/errors"
	"github.com/lixiaofei123/nextlist/models"
	"github.com/lixiaofei123/nextlist/web/controller"
)

func AuthHandler(next echo.HandlerFunc) echo.HandlerFunc {

	return func(ctx echo.Context) error {

		authorization := ctx.Request().Header.Get("authorization")
		token, err := jwt.ParseWithClaims(authorization, &models.JWTClaims{}, func(token *jwt.Token) (interface{}, error) {
			return []byte(configs.GlobalConfig.Auth.Secret), nil
		})

		if err == nil && token.Valid {
			if claims, ok := token.Claims.(*models.JWTClaims); ok {
				ctx.Request().Header.Set("email", claims.Email)
				ctx.Request().Header.Set("role", claims.Role)
				ctx.Request().Header.Set("username", claims.Issuer)
				ctx.Request().Header.Set("showname", claims.ShowName)

				return next(ctx)
			}
		}

		ctx.JSON(http.StatusUnauthorized, controller.DataResponse{
			Code: http.StatusUnauthorized,
			Data: fileerr.ErrNeedLogin.Error(),
		})
		return nil
	}

}

func NotMustAuthHandler(next echo.HandlerFunc) echo.HandlerFunc {

	return func(ctx echo.Context) error {

		authorization := ctx.Request().Header.Get("authorization")
		token, err := jwt.ParseWithClaims(authorization, &models.JWTClaims{}, func(token *jwt.Token) (interface{}, error) {
			return []byte(configs.GlobalConfig.Auth.Secret), nil
		})
		if err != nil {
			return next(ctx)
		}
		if !token.Valid {
			return next(ctx)
		}
		if claims, ok := token.Claims.(*models.JWTClaims); ok {

			ctx.Request().Header.Set("email", claims.Email)
			ctx.Request().Header.Set("role", claims.Role)
			ctx.Request().Header.Set("username", claims.Issuer)
			ctx.Request().Header.Set("showname", claims.ShowName)

			return next(ctx)
		}

		return next(ctx)
	}

}
