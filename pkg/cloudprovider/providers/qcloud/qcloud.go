/*
Copyright 2014 The Kubernetes Authors All rights reserved.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package qcloud

import (
	"fmt"
	"github.com/golang/glog"
	"io"
	"k8s.io/kubernetes/pkg/cloudprovider"
	"k8s.io/kubernetes/pkg/controller"
	"k8s.io/apimachinery/pkg/types"

	norm "k8s.io/kubernetes/pkg/cloudprovider/providers/qcloud/component"

	"github.com/dbdd4us/qcloudapi-sdk-go/metadata"
	"github.com/dbdd4us/qcloudapi-sdk-go/cvm"
	"github.com/dbdd4us/qcloudapi-sdk-go/common"

	"encoding/json"
	"os"
)

const (
	ProviderName = "qcloud"
	AnnoServiceLBInternalSubnetID = "service.kubernetes.io/qcloud-loadbalancer-internal"

	AnnoServiceClusterId = "service.kubernetes.io/qcloud-loadbalancer-clusterid"
)

//TODO instance cache
type QCloud struct {
	currentInstanceInfo *norm.NormGetNodeInfoRsp
	metaData            *metadata.MetaData
	cvm                 *cvm.Client
	config              Config
	slefNodeName        types.NodeName
	selfInstanceInfo    *cvm.InstanceInfo
}

type Config struct {
	Region          string `json:"region"`
	Zone            string `json:"zone"`
	VpcId           string `json:"vpcId"`

	QCloudSecretId  string `json:"QCloudSecretId"`
	QCloudSecretKey string `json:"QCloudSecretKey"`
}

var config Config

func readConfig(cfg io.Reader) error {
	if cfg == nil {
		err := fmt.Errorf("No cloud provider config given")
		return err
	}

	if err := json.NewDecoder(cfg).Decode(&config); err != nil {
		glog.Errorf("Couldn't parse config: %v", err)
		return err
	}

	//TODO reflect from env
	QCloudSecretId := os.Getenv("QCloudSecretId")
	if QCloudSecretId != "" {
		config.QCloudSecretId = QCloudSecretId
	}
	QCloudSecretKey := os.Getenv("QCloudSecretKey")
	if QCloudSecretId != "" {
		config.QCloudSecretKey = QCloudSecretKey
	}

	return nil
}

//TODO 优化newClient
func newQCloud() (*QCloud, error) {
	cvmClient, err := cvm.NewClient(
		common.Credential{
			SecretId:config.QCloudSecretId,
			SecretKey:config.QCloudSecretKey,
		},
		common.Opts{
			Region: config.Region,
		}, )
	if err != nil {
		return nil, err
	}

	cloud := &QCloud{
		config:config,
		metaData:metadata.NewMetaData(nil),
		cvm:cvmClient,
	}

	info, err := cloud.buildSelfInstance()
	if err != nil {
		return nil, err
	}
	cloud.selfInstanceInfo = info

	nodeName, err := cloud.buildSelfNodeName()
	if err != nil {
		return nil, err
	}
	cloud.slefNodeName = nodeName

	return cloud, nil
}

func retrieveErrorCodeAndMessage(err error) (int, string) {
	if derr, ok := err.(*norm.RequestResultError); ok {
		return derr.Code, derr.Msg
	}
	return -999999, err.Error()
}

func init() {
	cloudprovider.RegisterCloudProvider(ProviderName, func(cfg io.Reader) (cloudprovider.Interface, error) {
		err := readConfig(cfg)
		if err != nil {
			return nil, err
		}
		return newQCloud()
	})
}

func (cloud *QCloud) Initialize(clientBuilder controller.ControllerClientBuilder) {}

func (cloud *QCloud) LoadBalancer() (cloudprovider.LoadBalancer, bool) {
	return cloud, true
}

func (cloud *QCloud) Instances() (cloudprovider.Instances, bool) {
	return cloud, true
}

func (cloud *QCloud) Zones() (cloudprovider.Zones, bool) {
	return cloud, true
}

func (cloud *QCloud) Clusters() (cloudprovider.Clusters, bool) {
	return nil, false
}

func (cloud *QCloud) Routes() (cloudprovider.Routes, bool) {
	return cloud, true
}

func (cloud *QCloud) ProviderName() string {
	return ProviderName
}

func (cloud *QCloud) ScrubDNS(nameservers, seraches []string) ([]string, []string) {
	return nameservers, seraches
}
