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
		Region:      "ap-nanjing",
		Endpoint:    "cos.ap-nanjing.myqcloud.com",
		Bucket:      "xxxx",
		ForceS3Path: true,
	}

	sdriver := S3Driver{}
	sdriver.initConfig(&config)

	f, err := sdriver.WalkDir("/")

	if err != nil {
		t.Fail()
	}

	data, err := json.Marshal(f)
	if err != nil {
		t.Fail()
	}

	fmt.Println(string(data))
}
