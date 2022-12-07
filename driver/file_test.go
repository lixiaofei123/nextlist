package driver

import (
	"encoding/json"
	"fmt"
	"testing"
)

func Test_FileWalkDir(t *testing.T) {
	fdriver := FileDriver{
		config: FileDriverConfig{
			Path: "/home/lixiaofei/Documents/workspace/nextlist/driver/",
		},
	}

	f, err := fdriver.WalkDir("/")
	if err != nil {
		t.Fail()
	}

	data, _ := json.Marshal(f)
	fmt.Println(string(data))
}
