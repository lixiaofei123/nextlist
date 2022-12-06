package driver

import (
	"encoding/json"
	"testing"
)

func Test_S3alkDir(t *testing.T) {

	config := S3DriverConfig{
		SecretID:  "AKIDUqztZ2HLBxhJnAJFxWu7A9yZ0qW2WFyU",
		SecretKey: "RholV0aVFzYu3ye9UjpiNpFXgfO3jO5h",
		Region:    "ap-nanjing",
		Endpoint:  "nextlist-1303112930.cos.ap-nanjing.myqcloud.com",
		Bucket:    "nextlist-1303112930",
	}

	// config := S3DriverConfig{
	// 	SecretID:  "QUqIXEUWqJEgqJ3D",
	// 	SecretKey: "I0NWL3NU9VFgHrC8BgCkl3iNk7eioU",
	// 	Region:    "oss-cn-shenzhen",
	// 	Endpoint:  "oss-cn-shenzhen.aliyuncs.com",
	// 	Bucket:    "huiyuanai",
	// }

	sdriver := S3Driver{}
	sdriver.initConfig(&config)

	f, err := sdriver.WalkDir("软件/")
	if err != nil {
		t.Fail()
	}

	data, err := json.Marshal(f)
	if err != nil {
		t.Fail()
	}

	t.Log(string(data))
}
