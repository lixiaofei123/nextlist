package driver

import (
	"testing"
)

func Test_FileWalkDir(t *testing.T) {
	fdriver := FileDriver{
		config: FileDriverConfig{
			Path: "/home/lixiaofei/Documents/workspace/nextlist",
		},
	}

	f, err := fdriver.WalkDir("/driver/")
	if err != nil {
		t.Fail()
	}

	if f != nil {
		t.Log("success")
	}
}
