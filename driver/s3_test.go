package driver

import (
	"encoding/json"
	"fmt"
	"testing"
)

func Test_S3alkDir(t *testing.T) {

	config := S3DriverConfig{
		SecretID:    "xxxx",
		SecretKey:   "yyyy",
		Region:      "oss-cn-shenzhen",
		Endpoint:    "oss-cn-shenzhen.aliyuncs.com",
		Bucket:      "huiyuanai",
		ForceS3Path: false,
	}

	sdriver := S3Driver{}
	sdriver.initConfig(&config)

	f, err := sdriver.WalkDir("/test")

	if err != nil {
		t.Fail()
	}

	data, err := json.Marshal(f)
	if err != nil {
		t.Fail()
	}

	fmt.Println(string(data))
}
