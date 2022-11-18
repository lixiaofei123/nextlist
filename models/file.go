package models

import (
	"database/sql"
	"time"

	"github.com/lixiaofei123/nextlist/driver"
)

type Permission int

const (
	PUBLICREAD Permission = 0
	USERREAD   Permission = 1
	MEREAD     Permission = 2
	PASSWORD   Permission = 3
)

type FileStatus int

const (
	READY   FileStatus = 0
	SUCCESS FileStatus = 1
)

type File struct {
	ID             string                `gorm:"primaryKey,size:36" json:"id,omitempty"`
	UserName       string                `gorm:"size:20" json:"userName,omitempty"`
	Name           string                `gorm:"size:200;not null;uniqueIndex:idx_parent_name" json:"name"`
	ParentId       string                `gorm:"size:36;default:'';uniqueIndex:idx_parent_name" json:"parentId"`
	AbsolutePath   string                `gorm:"size:300;not null;" json:"absolutePath"`
	IsDict         sql.NullBool          `gorm:"not null;default:false" json:"isDict"`
	Children       []*File               `gorm:"-" json:"children"`
	FileType       string                `gorm:"size:100;not null;default:''" json:"fileType"`
	FileSize       int64                 `gorm:"not null;default:0" json:"fileSize"`
	Permission     Permission            `grom:"not null;default:0" json:"permission,omitempty"`
	FileStatus     FileStatus            `grom:"not null;default:1" json:"fileStatus,omitempty"`
	LastModifyTime time.Time             `gorm:"not null;" json:"createAt,omitempty"`
	DownloadUrls   []*driver.DownloadUrl `gorm:"-" json:"downloadUrls"`
	Password       string                `gorm:"size:30" json:"-"`
}

type PageResult struct {
	Total     int                    `json:"total"`
	Page      int                    `json:"page"`
	PageCount int                    `json:"pageCount"`
	List      interface{}            `json:"list"`
	Extend    map[string]interface{} `json:"extend,omitempty"`
}
