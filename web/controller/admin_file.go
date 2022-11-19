package controller

import (
	"github.com/labstack/echo/v4"
	"github.com/lixiaofei123/nextlist/driver"
	"github.com/lixiaofei123/nextlist/models"
	services "github.com/lixiaofei123/nextlist/services"
	"github.com/lixiaofei123/nextlist/utils"
	mvc "github.com/lixiaofei123/nextlist/web/mvc"

	fileerr "github.com/lixiaofei123/nextlist/errors"
)

type AdminFileController struct {
	fileSrv services.FileService
	driver  driver.Driver
}

func NewAdminFileController(fileSrv services.FileService, driver driver.Driver) *AdminFileController {
	return &AdminFileController{
		fileSrv: fileSrv,
		driver:  driver,
	}
}

func (f *AdminFileController) PostDriverSignUpload(ctx echo.Context) mvc.Result {

	key := utils.GetValue(ctx, "key")

	urlStr, err := f.driver.PreUploadUrl(key)

	if err != nil {
		return HandleData(nil, err)
	}

	return HandleData(urlStr, nil)
}

func (f *AdminFileController) PostDriverSignDelete(ctx echo.Context) mvc.Result {

	key := utils.GetValue(ctx, "key")
	username := ctx.Request().Header.Get("username")

	file, err := f.fileSrv.FindByPath(key)
	if err != nil {
		return HandleData(nil, err)
	}

	//没有权限删除
	if file.UserName != username && file.UserName != "" {
		return HandleData(nil, fileerr.ErrNotEnoughPermission)
	}

	//然后才是删除
	urlStr, err := f.driver.PreDeleteUrl(key)

	if err != nil {
		return HandleData(nil, err)
	}

	return HandleData(urlStr, nil)
}

func (f *AdminFileController) PostSync(ctx echo.Context) mvc.Result {

	path := utils.GetValueWithDefault(ctx, "path", "/")
	username := ctx.Request().Header.Get("username")

	err := f.fileSrv.SyncFiles(username, path)
	if err != nil {
		return HandleData(nil, err)
	}

	return HandleData("OK", nil)
}

func (f *AdminFileController) PutDirBy(ctx echo.Context, parentid string) mvc.Result {

	name := utils.GetValueWithDefault(ctx, "name", "empty")
	permission := utils.GetIntValueWithDefault(ctx, "permission", 0)

	username := ctx.Request().Header.Get("username")
	password := utils.GetValueWithDefault(ctx, "password", "")

	file, err := f.fileSrv.CreateDictory(username, parentid, name, models.Permission(permission), password)
	if err != nil {
		return HandleData(nil, err)
	}

	return HandleData(file, nil)
}

func (f *AdminFileController) PutDir(ctx echo.Context) mvc.Result {
	return f.PutDirBy(ctx, "")
}

func (f *AdminFileController) PostFileBy(ctx echo.Context, parentid string) mvc.Result {

	name := utils.GetValueWithDefault(ctx, "name", "empty")
	permission := utils.GetIntValueWithDefault(ctx, "permission", 0)
	fileSize := utils.GetIntValueWithDefault(ctx, "fileSize", 0)
	fileType := utils.GetValueWithDefault(ctx, "fileType", "")
	username := ctx.Request().Header.Get("username")

	file, err := f.fileSrv.PreSaveFile(username, &models.File{
		ParentId:   parentid,
		Name:       name,
		Permission: models.Permission(permission),
		FileSize:   int64(fileSize),
		FileType:   fileType,
	})
	if err != nil {
		return HandleData(nil, err)
	}

	return HandleData(file, nil)
}

func (f *AdminFileController) Post(ctx echo.Context, parentid string) mvc.Result {

	return f.PostFileBy(ctx, "")
}

func (f *AdminFileController) DeleteBy(ctx echo.Context, fileid string) mvc.Result {

	username := ctx.Request().Header.Get("username")
	file, err := f.fileSrv.DeleteFile(username, fileid)
	if err != nil {
		return HandleData(nil, err)
	}

	return HandleData(file, nil)
}

// 确认执行成功
func (f *AdminFileController) PostConfirmFileBy(ctx echo.Context, fileid string) mvc.Result {

	username := ctx.Request().Header.Get("username")

	file, err := f.fileSrv.UpdateFileStatus(username, fileid, models.SUCCESS)
	if err != nil {
		return HandleData(nil, err)
	}
	return HandleData(file, nil)
}
