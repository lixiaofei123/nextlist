package controller

import (
	"errors"
	"net/http"

	"github.com/labstack/echo/v4"
	fileerr "github.com/lixiaofei123/nextlist/errors"
	mvc "github.com/lixiaofei123/nextlist/web/mvc"
)

type RespMessage struct {
	Code int
	Text string
	Data interface{}
	Err  error
}

type PageRespMessage struct {
	PageIndex  int         `json:"pageIndex"`
	PageCount  int         `json:"pageCount"`
	TotalCount int         `json:"totalCount"`
	List       interface{} `json:"list"`
}

type TextResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type DataResponse struct {
	Code int         `json:"code"`
	Data interface{} `json:"data"`
}

type EmptyResponse struct {
	Code int `json:"code"`
}

// implementsResult.
func (e RespMessage) Dispatch(ctx echo.Context) {
	if e.Code == 0 {
		e.Code = 200
	}

	//if ctx.GetStatusCode() ==

	if ctx.Response().Status == http.StatusNotModified {
		ctx.JSON(http.StatusNotModified, EmptyResponse{
			Code: e.Code,
		})
		return
	}

	if ctx.Response().Status == http.StatusMovedPermanently {
		ctx.JSON(http.StatusMovedPermanently, EmptyResponse{
			Code: e.Code,
		})
		return
	}

	if e.Text != "" {
		ctx.JSON(e.Code, TextResponse{
			Code:    e.Code,
			Message: e.Text,
		})
	} else if e.Data != nil {
		ctx.JSON(e.Code, DataResponse{
			Code: e.Code,
			Data: e.Data,
		})
	} else if e.Err.Error() != "" {
		ctx.JSON(e.Code, DataResponse{
			Code: e.Code,
			Data: e.Err.Error(),
		})
	} else {
		ctx.JSON(e.Code, EmptyResponse{
			Code: e.Code,
		})
	}
}

func HandleData(data interface{}, err error) mvc.Result {

	if err != nil {
		if errors.Is(err, fileerr.ErrNeedLogin) || errors.Is(err, fileerr.ErrNotEnoughPermission) {
			return RespMessage{
				Code: http.StatusUnauthorized,
				Err:  err,
			}
		}
		return RespMessage{
			Code: http.StatusInternalServerError,
			Err:  err,
		}
	}
	return RespMessage{
		Code: 200,
		Data: data,
	}
}
