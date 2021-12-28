package errors

import "errors"

var (
	ErrFileNotFound        error = errors.New("文件不存在")
	ErrNotEnoughPermission error = errors.New("权限不足")
	ErrPasswordIsWrong     error = errors.New("密码错误")
	ErrNotDirectoy         error = errors.New("不是文件夹")
	ErrCreateDirConflict   error = errors.New("创建文件夹冲突")
	ErrNotEmptyDirectoy    error = errors.New("不是空文件夹")
	ErrFileExists          error = errors.New("文件已经存在")
	ErrRegisterIsDisabled  error = errors.New("站点关闭了注册功能")
	ErrNeedLogin           error = errors.New("需要先进行登录")
	ErrUnAllowUrl          error = errors.New("不允许的跳转链接")
	ErrUnSupportOperation  error = errors.New("不支持的操作")
)
