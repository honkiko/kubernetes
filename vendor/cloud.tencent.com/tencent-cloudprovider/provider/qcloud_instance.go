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
	"errors"
	"github.com/dbdd4us/qcloudapi-sdk-go/cvm"
	"k8s.io/kubernetes/pkg/api/v1"
	"k8s.io/kubernetes/pkg/cloudprovider"
	"k8s.io/apimachinery/pkg/types"
	"fmt"
	"net/url"
	"strings"
)


//TODO 隔离，已退还，退还中
func (self *QCloud)getInstanceInfoByNodeName(lanIP string) (*cvm.InstanceInfo, error) {
	filter := cvm.NewFilter(cvm.FilterNamePrivateIpAddress, lanIP)

	args := cvm.DescribeInstancesArgs{
		Version: cvm.DefaultVersion,
		Filters: &[]cvm.Filter{filter, },
	}

	response, err := self.cvm.DescribeInstances(&args)
	if err != nil {
		return nil, err
	}
	instanceSet := response.InstanceSet
	for _, instance := range instanceSet {
		if instance.VirtualPrivateCloud.VpcID == self.Config.VpcId && stringIn(lanIP, instance.PrivateIPAddresses) {
			return &instance, nil
		}
	}
	return nil, cloudprovider.InstanceNotFound
}

func (self *QCloud)getInstanceInfoById(instanceId string) (*cvm.InstanceInfo, error) {
	filter := cvm.NewFilter(cvm.FilterNameInstanceId, instanceId)

	args := cvm.DescribeInstancesArgs{
		Version: cvm.DefaultVersion,
		Filters: &[]cvm.Filter{filter, },
	}

	response, err := self.cvm.DescribeInstances(&args)
	if err != nil {
		return nil, err
	}

	instanceSet := response.InstanceSet
	if len(instanceSet) == 0 {
		return nil, cloudprovider.InstanceNotFound
	}
	if len(instanceSet) > 1 {
		return nil, fmt.Errorf("multiple instances found for instance: %s", instanceId)
	}
	return &instanceSet[0], cloudprovider.InstanceNotFound
}

type kubernetesInstanceID string

// mapToAWSInstanceID extracts the awsInstanceID from the kubernetesInstanceID
func (name kubernetesInstanceID) mapToInstanceID() (string, error) {
	s := string(name)

	if !strings.HasPrefix(s, "aws://") {
		// Assume a bare aws volume id (vol-1234...)
		// Build a URL with an empty host (AZ)
		s = "aws://" + "/" + "/" + s
	}
	url, err := url.Parse(s)
	if err != nil {
		return "", fmt.Errorf("Invalid instance name (%s): %v", name, err)
	}
	if url.Scheme != "aws" {
		return "", fmt.Errorf("Invalid scheme for AWS instance (%s)", name)
	}

	instanceId := ""
	tokens := strings.Split(strings.Trim(url.Path, "/"), "/")
	if len(tokens) == 1 {
		// instanceId
		instanceId = tokens[0]
	} else if len(tokens) == 2 {
		// az/instanceId
		instanceId = tokens[1]
	}

	if instanceId == "" || strings.Contains(instanceId, "/") || !strings.HasPrefix(instanceId, "i-") {
		return "", fmt.Errorf("Invalid format for AWS instance (%s)", name)
	}

	return instanceId, nil
}



//TODO 如果NodeAddressesByProviderID失败，nodeController会调用此接口
func (self *QCloud) NodeAddresses(name types.NodeName) ([]v1.NodeAddress, error) {

	addresses := make([]v1.NodeAddress, 0)

	ip, err := self.metaData.PrivateIPv4()
	if err != nil {
		return nil, err
	}
	if ip == string(name) {
		addresses = append(addresses, v1.NodeAddress{
			Type:v1.NodeInternalIP, Address:ip,
		})
		return addresses, nil
	} else {
		info, err := self.getInstanceInfoByNodeName(string(name))
		if err != nil {
			return nil, err
		}
		addresses = append(addresses, v1.NodeAddress{
			Type:v1.NodeInternalIP, Address:info.PrivateIPAddresses[0],
		})
		return addresses, nil
	}

}

//返回instanceID or cloudprovider.InstanceNotFound
func (self *QCloud) ExternalID(name types.NodeName) (string, error) {

	ip, err := self.metaData.PrivateIPv4()
	if err != nil {
		return "", err
	}
	if ip == string(name) {
		instanceId, err := self.metaData.InstanceID()
		if err != nil {
			return "", err
		}
		return instanceId, nil
	} else {
		info, err := self.getInstanceInfoByNodeName(string(name))
		if err != nil {
			return "", err
		}
		return info.InstanceID, nil
	}

}



// /zone/instanceId
func (self *QCloud) InstanceID(name types.NodeName) (string, error) {

	ip, err := self.metaData.PrivateIPv4()
	if err != nil {
		return "", err
	}

	if ip == string(name) {
		instanceId, err := self.metaData.InstanceID()
		if err != nil {
			return "", err
		}

		zone, err := self.GetZone()
		if err != nil {
			return "", err
		}
		return "/" + zone.FailureDomain + "/" + instanceId, nil
	} else {
		info, err := self.getInstanceInfoByNodeName(string(name))
		if err != nil {
			return "", err
		}
		return "/" + info.Placement.Zone + "/" + info.InstanceID, nil
	}

}

func (self *QCloud) NodeAddressesByProviderID(providerID string) ([]v1.NodeAddress, error) {
	instanceId, err := kubernetesInstanceID(providerID).mapToInstanceID()
	if err != nil {
		return nil, err
	}
	info, err := self.getInstanceInfoById(instanceId)
	if err != nil {
		return nil, err
	}
	addresses := make([]v1.NodeAddress, 0)
	addresses = append(addresses, v1.NodeAddress{
		Type:v1.NodeInternalIP, Address:info.PrivateIPAddresses[0],
	})
	return addresses, nil
}

//TODO
func (self *QCloud) InstanceTypeByProviderID(providerID string) (string, error) {
	return "QCLOUD", nil
}

func (self *QCloud) InstanceType(name types.NodeName) (string, error) {
	// TODO: 返回更有意义的类型，以便可以根据机器类型进行pod调度
	return "QCLOUD", nil
}

func (self *QCloud) AddSSHKeyToAllInstances(user string, keyData []byte) error {
	return errors.New("AddSSHKeyToAllInstances not implemented")
}

func (self *QCloud) CurrentNodeName(hostName string) (types.NodeName, error) {

	ip, err := self.metaData.PrivateIPv4()
	if err != nil {
		return types.NodeName(""), err
	}
	return types.NodeName(ip), nil

}

func (self *QCloud) GetZone() (cloudprovider.Zone, error) {
	//zone, err := self.metaData.Zone()
	//if err != nil {
	//	return cloudprovider.Zone{}, err
	//}
	//region, err := self.metaData.Region()
	//if err != nil {
	//	return cloudprovider.Zone{}, err
	//}

	return cloudprovider.Zone{Region: self.Config.Region, FailureDomain: self.Config.Zone}, nil
}


//TODO dont dirct call
func (c *QCloud) buildSelfInstance() (*cvm.InstanceInfo, error) {
	instanceID, err := c.metaData.InstanceID()
	if err != nil {
		return nil, fmt.Errorf("error fetching instance-id from metadata: %v", err)
	}

	instance, err := c.getInstanceInfoById(instanceID)
	if err != nil {
		return nil, fmt.Errorf("error finding instance %s: %v", instanceID, err)
	}
	return instance, nil
}

func (c *QCloud) buildSelfNodeName() (types.NodeName, error) {
	ip, err := c.metaData.PrivateIPv4()
	if err != nil {
		return types.NodeName(""), fmt.Errorf("error fetching PrivateIPv4 from metadata: %v", err)
	}
	return types.NodeName(ip), nil
}