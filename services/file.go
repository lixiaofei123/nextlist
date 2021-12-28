package services

import (
	"database/sql"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/lixiaofei123/nextlist/configs"
	"github.com/lixiaofei123/nextlist/driver"
	fileerr "github.com/lixiaofei123/nextlist/errors"
	models "github.com/lixiaofei123/nextlist/models"
	"github.com/lixiaofei123/nextlist/utils"
	"gorm.io/gorm"
)

type FileService interface {
	FindChildFiles(username string, fileId string, password string, page, count int) (*models.PageResult, error)

	ListFilesByPath(username string, path string, password string, page, count int) (*models.PageResult, error)

	FindById(username string, password, fileId string) (*models.File, error)

	FindByPath(path string) (*models.File, error)

	BaseInfo(fileId string) (*models.File, error)

	CreateDictory(username, parentId, name string, permission models.Permission, password string) (*models.File, error)

	PreSaveFile(username string, file *models.File) (*models.File, error)

	UpdateFileStatus(username string, fileId string, status models.FileStatus) (*models.File, error)

	DeleteFile(username, fileId string) (*models.File, error)

	SearchFile(username, keyword string, page, count int) (*models.PageResult, error)

	CountFiles() (map[string]int64, error)

	SyncFiles(username string, key string) error
}

type fileService struct {
	db     *gorm.DB
	driver driver.Driver
}

func NewFileService(db *gorm.DB, driver driver.Driver) FileService {
	return &fileService{
		db:     db,
		driver: driver,
	}
}

func (f *fileService) CountFiles() (map[string]int64, error) {

	rows, err := f.db.Raw("select file_type as fileType,count(*) as count from files where is_dict = false  group by file_type").Rows()
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var fileType string
	var count int64

	var results map[string]int64 = map[string]int64{}

	for rows.Next() {
		rows.Scan(&fileType, &count)
		results[fileType] = count
	}

	// 总量
	var totalSize int64
	err = f.db.Raw("select sum(file_size) from files").Scan(&totalSize).Error
	if err != nil {
		return nil, err
	}

	results["totalSize"] = totalSize

	return results, nil

}

func (f *fileService) ListFilesByPath(username string, path string, password string, page, count int) (*models.PageResult, error) {

	if path == "" || path == "/" {
		return f.FindChildFiles(username, "", password, page, count)
	} else {
		file := &models.File{
			AbsolutePath: path,
		}

		if err := f.db.Where(file).First(file).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil, fileerr.ErrFileNotFound
			}
			return nil, err
		}

		return f.FindChildFiles(username, file.ID, password, page, count)
	}

}

func (f *fileService) FindChildFiles(username string, fileId string, password string, page, count int) (*models.PageResult, error) {

	if page < 1 {
		page = 1
	}

	if count < 1 || count > 50 {
		count = 50
	}

	var file *models.File

	if fileId != "" {
		file = &models.File{
			ID: fileId,
		}

		if err := f.db.Where(file).First(file).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil, fileerr.ErrFileNotFound
			}
			return nil, err
		}
	} else {
		file = &models.File{
			Permission: models.PUBLICREAD,
			IsDict: sql.NullBool{
				Valid: true,
				Bool:  true,
			},
		}
	}

	if file.IsDict.Bool {

		if file.Permission == models.PASSWORD && password != file.Password {
			return nil, fileerr.ErrPasswordIsWrong
		}

		if file.Permission != models.PASSWORD && ((username == "" && file.Permission != models.PUBLICREAD) || (file.Permission == models.MEREAD && file.UserName != username)) {
			return nil, fileerr.ErrNotEnoughPermission
		}

		files := []*models.File{}
		if err := f.db.Model(&models.File{}).Where("parent_id = ?", fileId).Order("last_modify_time desc").Offset((page - 1) * count).Limit(count).Find(&files).Error; err != nil {
			if !errors.Is(err, gorm.ErrRecordNotFound) {
				return nil, err
			}
		}

		for index, file := range files {
			if !file.IsDict.Bool {
				files[index].DownloadUrls, _ = f.driver.DownloadUrl(configs.GlobalConfig.DriverConfig.Download, file.AbsolutePath)
			}

		}

		// 查询数量
		var total int64
		if err := f.db.Model(&models.File{}).Where("parent_id = ?", fileId).Count(&total).Error; err != nil {
			return nil, err
		}

		var extend map[string]string = map[string]string{}
		extend["fileid"] = fileId

		return &models.PageResult{
			Total:     int(total),
			Page:      page,
			PageCount: count,
			List:      files,
			Extend:    extend,
		}, nil
	}

	return nil, fileerr.ErrNotDirectoy

}

func (f *fileService) SearchFile(username, keyword string, page, count int) (*models.PageResult, error) {

	allFiles := []*models.File{}
	if err := f.db.Model(&models.File{}).Where("name like ?", fmt.Sprintf("%%%s%%", keyword)).Offset((page - 1) * count).Limit(count).Find(&allFiles).Error; err != nil {
		return nil, err
	}

	files := []*models.File{}
	for _, file := range allFiles {
		if (file.Permission == models.PUBLICREAD || (file.Permission == models.USERREAD && username != "") || (file.UserName == username)) && file.Permission != models.PASSWORD {
			file.DownloadUrls, _ = f.driver.DownloadUrl(configs.GlobalConfig.DriverConfig.Download, file.AbsolutePath)
			files = append(files, file)
		}
	}

	var total int64
	if err := f.db.Model(&models.File{}).Where("name like ?", fmt.Sprintf("%%%s%%", keyword)).Count(&total).Error; err != nil {
		return nil, err
	}

	return &models.PageResult{
		Total:     int(total),
		Page:      page,
		PageCount: count,
		List:      files,
	}, nil
}

func (f *fileService) FindById(username string, password, fileId string) (*models.File, error) {

	file := &models.File{
		ID: fileId,
	}

	if err := f.db.Where(file).First(file).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fileerr.ErrFileNotFound
		}
		return nil, err
	}

	if file.Permission == models.PASSWORD && password != file.Password {
		return nil, fileerr.ErrPasswordIsWrong
	}

	if file.Permission != models.PASSWORD && ((username == "" && file.Permission != models.PUBLICREAD) || (file.Permission == models.MEREAD && file.UserName != username)) {
		return nil, fileerr.ErrNotEnoughPermission
	}

	if !file.IsDict.Bool {
		file.DownloadUrls, _ = f.driver.DownloadUrl(configs.GlobalConfig.DriverConfig.Download, file.AbsolutePath)
	}

	return file, nil
}

func (f *fileService) BaseInfo(fileId string) (*models.File, error) {

	file := &models.File{
		ID: fileId,
	}

	if err := f.db.Where(file).First(file).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fileerr.ErrFileNotFound
		}
		return nil, err
	}

	return &models.File{
		AbsolutePath: file.AbsolutePath,
	}, nil
}

func (f *fileService) FindByPath(path string) (*models.File, error) {

	file := &models.File{
		AbsolutePath: path,
	}

	if err := f.db.Where(file).First(file).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fileerr.ErrFileNotFound
		}
		return nil, err
	}

	return file, nil
}

func (f *fileService) UpdateFileStatus(username string, fileId string, status models.FileStatus) (*models.File, error) {

	file := &models.File{
		ID: fileId,
	}

	if err := f.db.Transaction(func(tx *gorm.DB) error {

		if err := tx.Where(file).First(file).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return fileerr.ErrFileNotFound
			}
			return err
		}

		if file.UserName != username {
			return fileerr.ErrNotEnoughPermission
		}

		file.FileStatus = status
		return tx.Updates(file).Error

	}); err != nil {
		return nil, err
	}

	return file, nil

}

func (f *fileService) DeleteFile(username, fileId string) (*models.File, error) {

	if username == "" {
		return nil, fileerr.ErrNotEnoughPermission
	}

	var file *models.File = &models.File{ID: fileId}

	if err := f.db.Transaction(func(tx *gorm.DB) error {

		if err := tx.Where(file).First(file).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return fileerr.ErrFileNotFound
			}
			return err
		}

		// 只能删除自己的
		if file.UserName != username && file.UserName != "" {
			return fileerr.ErrNotEnoughPermission
		}

		// 如果是目录的话，需要检查目录下是否还有文件
		if file.IsDict.Bool {
			var count int64
			if err := tx.Model(file).Where(&models.File{ParentId: fileId}).Count(&count).Error; err != nil {
				return err
			}
			if count > 0 {
				return fileerr.ErrNotEmptyDirectoy
			}
		}

		// All is OK
		return tx.Delete(file).Error

	}); err != nil {
		return nil, err
	}

	return file, nil

}

func (f *fileService) PreSaveFile(username string, file *models.File) (*models.File, error) {
	return f.createFile(username, file, false)
}

func (f *fileService) CreateDictory(username, parentId, name string, permission models.Permission, password string) (*models.File, error) {

	return f.createFile(username, &models.File{
		ParentId:   parentId,
		Name:       name,
		Permission: permission,
		Password:   password,
	}, true)

}

func (f *fileService) createFile(username string, saveFile *models.File, isDir bool) (*models.File, error) {

	var file *models.File

	if err := f.db.Transaction(func(tx *gorm.DB) error {

		parentDir := ""

		// 先检查是否存在同名文件夹
		existFile := &models.File{
			ParentId: saveFile.ParentId,
			Name:     saveFile.Name,
		}

		var count int64
		if err := tx.Model(existFile).Where(existFile).Count(&count).Error; err != nil {
			return err
		}

		if count > 0 {
			return fileerr.ErrFileExists
		}

		if saveFile.ParentId != "" {
			// 检查父目录存不存在
			parentFile := &models.File{
				ID: saveFile.ParentId,
			}

			if err := tx.Where(parentFile).First(parentFile).Error; err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					return fileerr.ErrFileNotFound
				}
				return err
			}

			// 是不是文件夹
			if !parentFile.IsDict.Bool {
				return fileerr.ErrNotDirectoy
			}

			// 是否有权限
			if (username == "" && parentFile.Permission != models.PUBLICREAD) || (parentFile.Permission == models.MEREAD && parentFile.UserName != username) {
				return fileerr.ErrNotEnoughPermission
			}

			// 如果设置的权限小于父目录的权限，需要提升权限

			if saveFile.Permission < parentFile.Permission {
				saveFile.Permission = parentFile.Permission
			}
			if saveFile.Permission == models.PASSWORD && saveFile.Password == "" {
				saveFile.Password = parentFile.Password
			}

			parentDir = parentFile.AbsolutePath
		}

		fileStatus := models.SUCCESS
		if !isDir {
			fileStatus = models.READY
		}
		file = &models.File{
			ID:             uuid.NewString(),
			UserName:       username,
			Name:           saveFile.Name,
			ParentId:       saveFile.ParentId,
			AbsolutePath:   fmt.Sprintf("%s/%s", parentDir, saveFile.Name),
			IsDict:         sql.NullBool{Valid: true, Bool: isDir},
			Permission:     saveFile.Permission,
			LastModifyTime: time.Now(),
			FileStatus:     fileStatus,
			FileSize:       saveFile.FileSize,
			FileType:       saveFile.FileType,
			Password:       saveFile.Password,
		}

		return tx.Create(file).Error

	}); err != nil {
		return nil, err
	}

	return file, nil
}

func recursiveCreateFile(tx *gorm.DB, username string, parentFile *models.File, file *driver.File) error {
	if file.IsDir && len(file.Childrens) > 0 {

		for _, subfile := range file.Childrens {

			absolutePath := strings.TrimRight(subfile.AbsolutePath, "/")
			// 先检查是否已经存在这个文件

			existfile := &models.File{
				AbsolutePath: absolutePath,
			}

			err := tx.Where(existfile).First(existfile).Error

			if err != nil {

				if !errors.Is(err, gorm.ErrRecordNotFound) {
					return err
				}

				// 需要自行创建此文件
				saveFile := &models.File{
					ID:             uuid.NewString(),
					UserName:       username,
					Name:           subfile.Name,
					ParentId:       parentFile.ID,
					AbsolutePath:   absolutePath,
					IsDict:         sql.NullBool{Valid: true, Bool: subfile.IsDir},
					LastModifyTime: time.Now(),
					FileStatus:     models.SUCCESS,
					Permission:     parentFile.Permission,
					FileSize:       subfile.Size,
					FileType:       utils.FindMimetypeByExt(filepath.Ext(subfile.Name)),
					Password:       parentFile.Password,
				}

				// if !subfile.IsDir {
				// 	// 不是文件夹，需要从文件名来推断文件类型
				// }

				err := tx.Save(saveFile).Error
				if err != nil {
					return err
				}

				existfile = saveFile

			}

			// 不允许往别人的私人目录以及加密目录中塞文件
			if subfile.IsDir && (username == existfile.UserName || existfile.Permission == models.PUBLICREAD) {
				err = recursiveCreateFile(tx, username, existfile, subfile)
				if err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func (f *fileService) SyncFiles(username string, key string) error {
	file, err := f.driver.WalkDir(key)
	if err != nil {
		return err
	}

	err = f.db.Transaction(func(tx *gorm.DB) error {

		// 开始数据导入
		absolutePath := strings.TrimRight(file.AbsolutePath, "/")

		existfile := &models.File{
			AbsolutePath: absolutePath,
		}

		if absolutePath != "" {
			err := tx.Where(existfile).First(existfile).Error
			if err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					return fileerr.ErrFileNotFound
				}
				return err
			}
		} else {
			existfile.Permission = models.PUBLICREAD
			existfile.ID = ""
			existfile.Password = ""
		}

		return recursiveCreateFile(tx, username, existfile, file)
	})

	return err

}
