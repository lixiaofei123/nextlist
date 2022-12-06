package driver

import (
	"bytes"
	"fmt"
	"net/http"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/labstack/echo/v4"
	fileerr "github.com/lixiaofei123/nextlist/errors"
	"github.com/lixiaofei123/nextlist/utils"
	"gorm.io/gorm"
)

func init() {
	RegsiterDriver("s3", "对象存储", &S3Driver{}, &S3DriverConfig{})
}

type S3DriverConfig struct {
	SecretID  string `arg:"secretID;SecretID;对象存储的密钥ID;required" json:"secretID"`
	SecretKey string `arg:"secretKey;SecretKey;对象存储的密钥Key;required" json:"secretKey"`
	Region    string `arg:"region;区域;对象存储所在区域;required" json:"region"`
	Endpoint  string `arg:"endpoint;Endpoint;endpoint地址;required" json:"endpoint"`
	Bucket    string `arg:"bucket;Bucket;bucket名称;required" json:"bucket"`
	Key       string `arg:"key;签名key;部分接口所需要使用的签名key,随意填写;required" json:"key"`
	Host      string `arg:"host;服务地址;Nextlist服务地址;required" json:"host"`
}

type S3Driver struct {
	config *aws.Config
	Bucket string
	s3     *s3.S3
	key    string
	host   string
}

func (d *S3Driver) Check() error {

	testdata := bytes.NewReader([]byte("data"))
	key := fmt.Sprintf("%d", time.Now().Unix())
	_, err := d.s3.PutObject(&s3.PutObjectInput{
		Bucket: &d.Bucket,
		Body:   testdata,
		Key:    &key,
	})
	if err != nil {
		return err
	}

	_, err = d.s3.DeleteObject(&s3.DeleteObjectInput{
		Bucket: &d.Bucket,
		Key:    &key,
	})

	return err
}

func (d *S3Driver) initConfig(config interface{}) error {

	s3config := config.(*S3DriverConfig)
	creds := credentials.NewStaticCredentials(s3config.SecretID, s3config.SecretKey, "")

	d.config = &aws.Config{
		Region:           aws.String(s3config.Region),
		Endpoint:         &s3config.Endpoint,
		S3ForcePathStyle: aws.Bool(true),
		Credentials:      creds,
	}

	d.Bucket = s3config.Bucket
	d.key = s3config.Key
	d.host = s3config.Host

	sess, err := session.NewSession(d.config)
	if err != nil {
		return err
	}
	d.s3 = s3.New(sess)

	return nil

}

func (d *S3Driver) InitDriver(e *echo.Group, db *gorm.DB) error {

	e.GET("/driver/s3", func(ctx echo.Context) error {

		filepath := utils.GetValue(ctx, "path")

		req, _ := d.s3.GetObjectRequest(&s3.GetObjectInput{
			Bucket: aws.String(d.Bucket),
			Key:    aws.String(filepath),
		})

		downloadUrl, err := req.Presign(15 * time.Minute)
		if err != nil {
			return err
		}

		ctx.Response().Header().Add("Location", downloadUrl)
		ctx.Response().WriteHeader(http.StatusMovedPermanently)

		return nil
	}, checkSignHandler(d.key))

	return nil
}

func (d *S3Driver) WalkDir(key string) (*File, error) {
	return nil, fileerr.ErrUnSupportOperation
}
func (d *S3Driver) PreUploadUrl(key string) (string, error) {

	req, _ := d.s3.PutObjectRequest(&s3.PutObjectInput{
		Bucket: aws.String(d.Bucket),
		Key:    aws.String(key),
	})

	return req.Presign(2 * time.Hour)
}

func (d *S3Driver) PreDeleteUrl(key string) (string, error) {

	req, _ := d.s3.DeleteObjectRequest(&s3.DeleteObjectInput{
		Bucket: aws.String(d.Bucket),
		Key:    aws.String(key),
	})

	return req.Presign(15 * time.Minute)
}

func (d *S3Driver) DownloadUrl(path string) ([]*DownloadUrl, error) {

	var downloads []*DownloadUrl = []*DownloadUrl{}

	downloadUrl, err := signUrl(fmt.Sprintf("%s/api/v1/driver/s3", d.host), d.key, path, time.Minute*10)
	if err == nil {
		downloads = append(downloads, &DownloadUrl{
			Title:       "下载地址",
			DownloadUrl: downloadUrl,
		})
	}

	return downloads, nil
}
