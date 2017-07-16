package qcloud

import (
	"testing"
	"fmt"
	"k8s.io/kubernetes/pkg/cloudprovider"
)

func TestQCloud_CurrentNodeName(t *testing.T) {
	cloud, err := newQCloud()
	if err != nil {
		t.Error(err)
		return
	}
	nodeName, err := cloud.CurrentNodeName("")
	if err != nil {
		t.Error(err)
		return
	}
	fmt.Printf("currentNodeName: %s\n", nodeName)
}

func TestQCloud_ExternalID(t *testing.T) {
	cloud, err := newQCloud()
	if err != nil {
		t.Error(err)
		return
	}
	name := "10.3.2.15"
	_, err = cloud.ExternalID(name)
	if err != cloudprovider.InstanceNotFound {
		t.Error(err)
		return
	}
	fmt.Print(err)
}
