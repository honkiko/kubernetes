package qcloud

import (
	"testing"
	"fmt"
	"github.com/dbdd4us/qcloudapi-sdk-go/cvm"
)

func TestGetInstanceInfoByLanIP(t *testing.T) {
	client, err := cvm.NewClientFromEnv()
	if err != nil {
		t.Fatal(err)
	}
	info, err := getInstanceInfoByLanIP(client, "vpc-b2h3xykt", "192.168.0.109")
	if err != nil {
		t.Fatal(err)
	}

	fmt.Printf("info: %v\n", info)
}
