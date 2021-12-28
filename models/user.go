package models

import (
	"database/sql"
	"time"

	"github.com/dgrijalva/jwt-go"
)

type Role string

const (
	SuperAdminRole Role = "superadmin"
	AdminRole      Role = "admin"
	UserRole       Role = "user"
)

type User struct {
	ID        string       `gorm:"primaryKey,size:36" json:"id,omitempty"`
	UserName  string       `gorm:"size:20;uniqueIndex:idx_username" json:"userName,omitempty" validate:"required,min=5,max=20"`
	ShowName  string       `gorm:"size:40" json:"showName,omitempty" validate:"min=5,max=40"`
	Email     string       `gorm:"size:40;uniqueIndex:idx_email" json:"email,omitempty" validate:"required,email,max=40"`
	Tel       string       `gorm:"size:11;uniqueIndex:idx_tel" json:"tel,omitempty" validate:"required,len=11"`
	Password  string       `gorm:"size:32" json:"password,omitempty" validate:"required,min=10,max=20"`
	Role      Role         `gorm:"size:15" json:"role,omitempty"`
	Enable    sql.NullBool `json:"enable,omitempty"`
	CreatedAt time.Time    `json:"createAt,omitempty"`
}

type JWTClaims struct {
	Role     string `json:"role"`
	Email    string `json:"email"`
	ShowName string `json:"showname"`
	jwt.StandardClaims
}
