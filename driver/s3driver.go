package driver

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/labstack/echo/v4"
	fileerr "github.com/lixiaofei123/nextlist/errors"
	"gorm.io/gorm"
)

func init() {
	RegsiterDriver("s3", &S3Driver{})
}

type S3DriverConfig struct {
	SecretID  string
	SecretKey string
	Region    string
	Endpoint  string
	Bucket    string
}

type S3Driver struct {
	config *aws.Config
	Bucket string
}

func (d *S3Driver) InitConfig(config interface{}) error {

	data, err := json.Marshal(config)
	if err != nil {
		return err
	}

	s3config := new(S3DriverConfig)
	err = json.Unmarshal(data, s3config)
	if err != nil {
		return err
	}

	creds := credentials.NewStaticCredentials(s3config.SecretID, s3config.SecretKey, "")

	d.config = &aws.Config{
		Region:           aws.String(s3config.Region),
		Endpoint:         &s3config.Endpoint,
		S3ForcePathStyle: aws.Bool(true),
		Credentials:      creds,
	}

	d.Bucket = s3config.Bucket

	return nil

}

func (d *S3Driver) InitDriver(e *echo.Echo, db *gorm.DB) error {
	return nil
}

func (d *S3Driver) WalkDir(key string) (*File, error) {
	return nil, fileerr.ErrUnSupportOperation
}
func (d *S3Driver) PreUploadUrl(key string) (string, error) {

	sess, err := session.NewSession(d.config)

	if err != nil {
		return "", err
	}

	svc := s3.New(sess)

	req, _ := svc.PutObjectRequest(&s3.PutObjectInput{
		Bucket: aws.String(d.Bucket),
		Key:    aws.String(key),
	})

	return req.Presign(2 * time.Hour)
}

func (d *S3Driver) PreDeleteUrl(key string) (string, error) {

	sess, err := session.NewSession(d.config)

	if err != nil {
		return "", err
	}

	svc := s3.New(sess)

	req, _ := svc.DeleteObjectRequest(&s3.DeleteObjectInput{
		Bucket: aws.String(d.Bucket),
		Key:    aws.String(key),
	})

	return req.Presign(15 * time.Minute)
}

func (d *S3Driver) DownloadUrl(configs DownloadConfigs, key string) ([]*DownloadUrl, error) {

	var downloads []*DownloadUrl = []*DownloadUrl{}

	for _, config := range configs {
		downloads = append(downloads, &DownloadUrl{
			Title:       config.Title,
			DownloadUrl: fmt.Sprintf("%s%s", config.Url, key),
		})
	}

	return downloads, nil
}
