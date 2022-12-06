package driver

import (
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt"
	"github.com/labstack/echo/v4"
	"github.com/lixiaofei123/nextlist/utils"
)

func checkSignHandler(key string) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(ctx echo.Context) error {

			// 检查连接的有效期
			expireTimeStr := utils.GetValue(ctx, "expireTime")
			if expireTimeStr == "" {
				return errors.New("必须包含一个expireTime参数")
			}

			now := time.Now()

			expireTime, err := time.ParseInLocation(timeLayout, expireTimeStr, now.Location())
			if err != nil {
				return err
			}

			if expireTime.Before(time.Now()) {
				return errors.New("链接已经失效")
			}

			path := utils.GetValue(ctx, "path")
			if path == "" {
				return errors.New("路径不能为空")
			}

			sign := utils.GetValue(ctx, "sign")
			if sign == "" {
				return errors.New("签名字符串不能为空")
			}

			method := jwt.GetSigningMethod("HS256")

			err = method.Verify(fmt.Sprintf("%s-%s", path, expireTimeStr), sign, []byte(key))
			if err != nil {
				return err
			}

			// 校验通过....

			return next(ctx)
		}
	}
}

const timeLayout string = "2006-01-02 15:04:05 "

func signUrl(url string, key string, path string, expireDuration time.Duration) (string, error) {

	expireTime := time.Now().Add(expireDuration)
	expireTimeStr := expireTime.Format(timeLayout)

	method := jwt.GetSigningMethod("HS256")

	sign, err := method.Sign(fmt.Sprintf("%s-%s", path, expireTimeStr), []byte(key))
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("%s?path=%s&expireTime=%s&sign=%s", url, path, expireTimeStr, sign), nil

}
