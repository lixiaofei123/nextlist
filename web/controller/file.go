package controller

import (
	"github.com/labstack/echo/v4"
	services "github.com/lixiaofei123/nextlist/services"
	"github.com/lixiaofei123/nextlist/utils"
	mvc "github.com/lixiaofei123/nextlist/web/mvc"
)

type FileController struct {
	fileSrv services.FileService
}

func NewFileController(fileSrv services.FileService) *FileController {
	return &FileController{
		fileSrv: fileSrv,
	}
}

func (f *FileController) GetBy(ctx echo.Context, fileid string) mvc.Result {

	username := ctx.Request().Header.Get("username")
	password := utils.GetValueWithDefault(ctx, "password", "")

	file, err := f.fileSrv.FindById(username, password, fileid)
	if err != nil {
		return HandleData(nil, err)
	}

	return HandleData(file, nil)

}

func (f *FileController) GetBaseinfoBy(ctx echo.Context, fileid string) mvc.Result {

	file, err := f.fileSrv.BaseInfo(fileid)
	if err != nil {
		return HandleData(nil, err)
	}

	return HandleData(file, nil)

}

func (f *FileController) GetDirBy(ctx echo.Context, fileid string) mvc.Result {

	username := ctx.Request().Header.Get("username")
	password := utils.GetValueWithDefault(ctx, "password", "")
	page := utils.GetIntValueWithDefault(ctx, "page", 1)
	count := utils.GetIntValueWithDefault(ctx, "count", 50)

	result, err := f.fileSrv.FindChildFiles(username, fileid, password, page, count)
	if err != nil {
		return HandleData(nil, err)
	}

	return HandleData(result, nil)
}

func (f *FileController) GetDir(ctx echo.Context) mvc.Result {

	path := utils.GetValueWithDefault(ctx, "path", "/")
	username := ctx.Request().Header.Get("username")
	password := utils.GetValueWithDefault(ctx, "password", "")
	page := utils.GetIntValueWithDefault(ctx, "page", 1)
	count := utils.GetIntValueWithDefault(ctx, "count", 50)

	result, err := f.fileSrv.ListFilesByPath(username, path, password, page, count)
	if err != nil {
		return HandleData(nil, err)
	}

	return HandleData(result, nil)
}

func (f *FileController) PostCount(ctx echo.Context) mvc.Result {

	results, err := f.fileSrv.CountFiles()
	if err != nil {
		return HandleData(nil, err)
	}

	return HandleData(results, nil)
}

func (f *FileController) PostSearch(ctx echo.Context) mvc.Result {

	keyword := utils.GetValueWithDefault(ctx, "keyword", "")
	username := ctx.Request().Header.Get("username")
	page := utils.GetIntValueWithDefault(ctx, "page", 1)
	count := utils.GetIntValueWithDefault(ctx, "count", 50)

	result, err := f.fileSrv.SearchFile(username, keyword, page, count)
	if err != nil {
		return HandleData(nil, err)
	}

	return HandleData(result, nil)
}
