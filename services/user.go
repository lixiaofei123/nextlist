package services

import (
	"crypto/md5"
	"errors"
	"fmt"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"
	models "github.com/lixiaofei123/nextlist/models"
	"gorm.io/gorm"
)

var (
	validate *validator.Validate = validator.New()
)

type LoginType int

const (
	UserName LoginType = 0
	Email    LoginType = 1
	Tel      LoginType = 2
)

func MD5Password(user *models.User) string {
	data := []byte(fmt.Sprintf("(%s)(%s)", user.ID, user.Password))
	return fmt.Sprintf("%x", md5.Sum(data))
}

type UserService interface {
	Login(username string, password string, loginType LoginType) (*models.User, error)

	Register(user *models.User) (*models.User, error)

	UserCount() (int64, error)
}

func NewUserService(db *gorm.DB) UserService {
	return &userService{
		db: db,
	}
}

type userService struct {
	db *gorm.DB
}

func (u *userService) UserCount() (int64, error) {

	var count int64

	if err := u.db.Model(&models.User{}).Count(&count).Error; err != nil {
		return 0, err
	}

	return count, nil

}

func (u *userService) Login(username string, password string, loginType LoginType) (*models.User, error) {

	user := &models.User{}

	if loginType == Tel {
		user.Tel = username
	} else if loginType == Email {
		user.Email = username
	} else {
		user.UserName = username
	}

	if err := u.db.Where(user).First(user).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("用户不存在")
		}
		return nil, err
	}

	encryPassword := user.Password
	user.Password = password
	if MD5Password(user) != encryPassword {
		return nil, errors.New("密码错误，请重试")
	}

	user.Password = ""

	return user, nil
}

func (u *userService) Register(user *models.User) (*models.User, error) {

	err := validate.Struct(user)
	if err != nil {
		return nil, err
	}

	// 校验通过
	user.ID = uuid.NewString()
	user.Role = models.UserRole
	user.Password = MD5Password(user)
	user.CreatedAt = time.Now()

	if err := u.db.Create(user).Error; err != nil {
		return nil, err
	}

	user.Password = ""
	return user, nil
}
