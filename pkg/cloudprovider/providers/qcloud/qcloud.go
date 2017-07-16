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
	"fmt"
	"github.com/golang/glog"
	"io"
	"k8s.io/kubernetes/pkg/api/v1"
	"k8s.io/kubernetes/pkg/cloudprovider"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/kubernetes/pkg/controller"
	norm "k8s.io/kubernetes/pkg/cloudprovider/providers/qcloud/component"

	"encoding/json"
)

const (
	ProviderName = "qcloud"
	AnnoServiceLBInternalSubnetID = "service.kubernetes.io/qcloud-loadbalancer-internal"

	AnnoServiceClusterId = "service.kubernetes.io/qcloud-loadbalancer-clusterid"
)

type QCloud struct {
	currentInstanceInfo *norm.NormGetNodeInfoRsp
}

type Disks interface {
	// AttachDisk attaches given disk to given node. Current node
	// is used when nodeName is empty string.
	AttachDisk(diskId string, nodeName string) (err error)

	// DetachDisk detaches given disk to given node. Current node
	// is used when nodeName is empty string.
	// Assumption: If node doesn't exist, disk is already detached from node.
	DetachDisk(diskId string, nodeName string) error

	// DiskIsAttached checks if a disk is attached to the given node.
	// Assumption: If node doesn't exist, disk is not attached to the node.
	DiskIsAttached(diskId, nodeName string) (bool, error)

	// DisksAreAttached checks if a list disks are attached to the given node.
	// Assumption: If node doesn't exist, disks are not attached to the node.
	DisksAreAttached(diskIds []string, nodeName string) (map[string]bool, error)

	// CreateDisk creates a new cbs disk with specified parameters.
	CreateDisk(name string, size int, region int, zone int, tags map[string]string) (diskId string, err error)

	// DeleteVolume deletes cbs disk.
	DeleteDisk(diskId string) error
}

type Config struct {
	Region string `json:"region"`
	Zone   string `json:"zone"`
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

	return nil
}

func newQCloud() (*QCloud, error) {
	cloud := &QCloud{}
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

// Instance interface

func (self *QCloud) retrieveCurrentNodeInfo() error {
	if self.currentInstanceInfo == nil {
		req := norm.NormGetNodeInfoReq{}
		currentInstanceInfo, err := norm.NormGetNodeInfo(req)
		if err != nil {
			return fmt.Errorf("retrieveCurrentNodeInfo: %s", err)
		}
		self.currentInstanceInfo = currentInstanceInfo
	}
	return nil
}

func (self *QCloud) NodeAddresses(name types.NodeName) ([]v1.NodeAddress, error) {
	// TODO: 仅能获取当前信息
	addresses := make([]v1.NodeAddress, 0)
	if err := self.retrieveCurrentNodeInfo(); err != nil {
		return addresses, err
	}
	addresses = append(addresses, v1.NodeAddress{
		Type:v1.NodeInternalIP, Address:self.currentInstanceInfo.InternalIp,
	})

	return addresses, nil
}

func (self *QCloud) ExternalID(name types.NodeName) (string, error) {
	if rsp, err := norm.NormGetNodeInfoByName(string(name)); err != nil {
		code, _ := retrieveErrorCodeAndMessage(err)
		glog.Info(code)
		if code == -8017 {
			return "", cloudprovider.InstanceNotFound
		}
		return "", err
	} else {
		return rsp.UInstanceId, nil
	}
}

func (self *QCloud) InstanceID(name types.NodeName) (string, error) {
	if rsp, err := norm.NormGetNodeInfoByName(string(name)); err != nil {
		return "", err
	} else {
		return rsp.UInstanceId, nil
	}
}

//TODO get nodeIP from instanceName

func (self *QCloud) NodeAddressesByProviderID(providerID string) ([]v1.NodeAddress, error) {
	return []v1.NodeAddress{}, errors.New("unimplemented")
}

//TODO
func (self *QCloud) InstanceTypeByProviderID(providerID string) (string, error) {
	return "QCLOUD", nil
}

func (self *QCloud) instanceUuid(lanIp string) (string, error) {
	if rsp, err := norm.NormGetNodeInfoByName(lanIp); err != nil {
		code, _ := retrieveErrorCodeAndMessage(err)
		glog.Info(code)
		if code == -8017 {
			return "", cloudprovider.InstanceNotFound
		}
		return "", err
	} else {
		return rsp.Uuid, nil
	}
}

func (self *QCloud) nodeNamesToUuid(nodeName []string) ([]string, error) {
	var uuids []string
	for _, lanIp := range nodeName {
		uuid, err := self.instanceUuid(lanIp)
		if err != nil {
			if err == cloudprovider.InstanceNotFound {
				glog.Infof("nodeNamesToUuid:not found node(%s),continue", lanIp)
				continue
			}
			return nil, err
		}
		uuids = append(uuids, uuid)
	}
	return uuids, nil
}

func (self *QCloud) InstanceType(name types.NodeName) (string, error) {
	if _, err := norm.NormGetNodeInfoByName(string(name)); err != nil {
		return "", err
	} else {
		// TODO: 返回更有意义的类型，以便可以根据机器类型进行pod调度
		return "QCLOUD", nil
	}
}

func (self *QCloud) List(filter string) ([]string, error) {
	result := make([]string, 0)
	return result, nil
}

func (self *QCloud) AddSSHKeyToAllInstances(user string, keyData []byte) error {
	return errors.New("AddSSHKeyToAllInstances not implemented")
}

func (self *QCloud) CurrentNodeName(hostName string) (types.NodeName, error) {
	glog.Infof("qcloud CurrentNodeName of %s\n", hostName)
	if err := self.retrieveCurrentNodeInfo(); err != nil {
		return "", err
	}
	return types.NodeName(self.currentInstanceInfo.Name), nil
}

func (self *QCloud) ListRoutes(clusterName string) ([]*cloudprovider.Route, error) {
	routes := make([]*cloudprovider.Route, 0)
	req := norm.NormListRoutesReq{ClusterName: clusterName}
	rsp, err := norm.NormListRoutes(req)
	if err != nil {
		return nil, err
	}
	for _, item := range rsp.Routes {
		if item.Subnet == "" {
			continue
		}
		routes = append(routes, &cloudprovider.Route{
			Name:            "", // TODO: what's this?
			TargetNode:  types.NodeName(item.Name),
			DestinationCIDR: item.Subnet,
		})
	}
	return routes, nil
}

func (self *QCloud) CreateRoute(clusterName string, nameHint string, route *cloudprovider.Route) error {
	glog.Infof("qcloud create route: %s %s %#v\n", clusterName, nameHint, route)
	req := []norm.NormRouteInfo{
		{Name: string(route.TargetNode), Subnet: route.DestinationCIDR},
	}
	_, err := norm.NormAddRoute(req)
	return err
}

// DeleteRoute deletes the specified managed route
// Route should be as returned by ListRoutes
func (self *QCloud) DeleteRoute(clusterName string, route *cloudprovider.Route) error {

	req := []norm.NormRouteInfo{
		{Name: string(route.TargetNode), Subnet: route.DestinationCIDR},
	}
	_, err := norm.NormDelRoute(req)
	return err
}

//TODO
func (self *QCloud) GetZone() (cloudprovider.Zone, error) {

	return cloudprovider.Zone{Region: config.Region, FailureDomain: config.Zone}, nil
}

func (qcloud *QCloud) CreateDisk(diskType string, sizeGb int, region int, zone int, tag map[string]string) (string, error) {

	glog.Infof("CreateDisk, diskType:%s, size:%d, region:%d, zone:%d",
		diskType, sizeGb, region, zone)

	var cbsType string = "cbs"
	if diskType == "fast" {
		cbsType = "cbsPrimium"
	}

	return norm.CreateCbsDisk(int(sizeGb), cbsType, region, zone)

}

func (qcloud *QCloud) DeleteDisk(diskId string) error {

	glog.Infof("DeleteDisk, disk:%s", diskId)

	err := norm.SetCbsRenewFlag(diskId, 0)
	if err != nil {
		glog.Errorf("DeleteDisk failed, disk:%s, error:%v", diskId, err)
		return err
	}

	return nil
}

func (qcloud *QCloud) AttachDisk(diskName, nodeName string) error {

	glog.Infof("AttachDisk, diskId:%s, node:%s", diskName, nodeName)

	getNodeRsp, err := norm.NormGetNodeInfoByName(nodeName)
	if err != nil || getNodeRsp == nil {
		return fmt.Errorf("AttachCbsDisk failed, disk:%s, get instance:%s, error:%v", diskName, nodeName, err)
	}

	instanceId := getNodeRsp.UInstanceId
	disk, err := norm.GetCbsDisk(diskName)
	if err != nil {
		return fmt.Errorf("AttachCbsDisk failed, can not get disk:%s, node:%s, error:%v", diskName, instanceId, err)
	}

	if disk == nil {
		return fmt.Errorf("AttachDisk, disk not found. diskId:%s, node:%s", diskName, instanceId)
	}

	if disk.InstanceIdAttached == instanceId {
		glog.Infof("AttachDisk, disk:%s is already attached to node:%s", diskName, instanceId)
		return nil
	}

	if disk.InstanceIdAttached != "" {
		glog.Errorf("AttachDisk to node:(%s:%s) failed, disk:%s has been attached to another node:%s.",
			nodeName, instanceId, diskName, disk.InstanceIdAttached)
		return fmt.Errorf("AttachDisk to node:(%s:%s) failed, disk:%s has been attached to another node:%s.",
			nodeName, instanceId, diskName, disk.InstanceIdAttached)
	}

	_, err = norm.AttachCbsDisk(diskName, instanceId)
	if err != nil {
		return fmt.Errorf("AttachCbsDisk failed, disk:%s, instance:%s, error:%v", diskName, instanceId, err)
	}

	err = norm.SetCbsRenewFlag(diskName, 1)
	if err != nil {
		glog.Errorf("SetCbsRenewFlag failed, disk:%s, error:%v", diskName, err)
	}

	return nil
}

func (qcloud *QCloud) DiskIsAttached(diskId, nodeName string) (bool, error) {

	glog.Infof("DiskIsAttached, diskId:%s, nodeName:%s", diskId, nodeName)

	getNodeRsp, err := norm.NormGetNodeInfoByName(nodeName)
	if err != nil {
		code, _ := retrieveErrorCodeAndMessage(err)
		if code == -8017 {
			glog.Warningf("DiskIsAttached, node is not found, assume disk:%s is not attched to the node:%s",
				diskId, nodeName)
			return false, nil
		}
		return false, fmt.Errorf("Check DiskIsAttached, get node info failed, disk:%s, get instance:%s, error:%v",
			diskId, nodeName, err)
	}

	instanceId := getNodeRsp.UInstanceId
	disk, err := norm.GetCbsDisk(diskId)
	if err != nil {
		glog.Errorf("DiskIsAttached, getCbsDisk failed, diskId:%s, node:%s, error:%v", diskId, nodeName, err)
		return false, err
	}

	if disk == nil {
		glog.Infof("DiskIsAttached, disk is not found, diskId:%s, node:%s", diskId, nodeName)
		return false, nil
	}

	if disk.InstanceIdAttached == instanceId {
		glog.Infof("DiskIsAttached, disk is attached to node, diskId:%s, node:%s", diskId, nodeName)
		return true, nil
	} else {
		glog.Infof("DiskIsAttached, disk:%s is not attached to node:%s, but attached to:(%s)",
			diskId, nodeName, disk.InstanceIdAttached)
		return false, nil
	}
}

func (qcloud *QCloud) DisksAreAttached(diskIds []string, nodeName string) (map[string]bool, error) {
	attached := make(map[string]bool)
	for _, diskId := range diskIds {
		attached[diskId] = false
	}

	getNodeRsp, err := norm.NormGetNodeInfoByName(nodeName)
	if err != nil {
		code, _ := retrieveErrorCodeAndMessage(err)
		if code == -8017 {
			glog.Warningf("DiskAreAttached, node is not found, assume disks are not attched to the node:%s",
				nodeName)
			return attached, nil
		}
		return attached, fmt.Errorf("Check DisksAreAttached, get node info failed, disks:%v, get instance:%s, error:%v",
			diskIds, nodeName, err)
	}

	instanceId := getNodeRsp.UInstanceId

	for _, diskId := range diskIds {
		disk, err := norm.GetCbsDisk(diskId)
		if err != nil {
			glog.Errorf("DiskIsAttached, getCbsDisk failed, diskId:%s, node:%s, error:%v", diskId, nodeName, err)
			return attached, err
		}

		if disk == nil {
			glog.Infof("DiskIsAttached, disk not found, diskId:%s, node:%s", diskId, nodeName)
		}

		if disk.InstanceIdAttached == instanceId {
			glog.Infof("DiskIsAttached, disk:%s is attached to node:%s", diskId, nodeName)
			attached[diskId] = true
		} else {
			glog.Infof("DiskAreAttached, disk:%s is not attached to node:%s, but attached to(%s)",
				diskId, nodeName, disk.InstanceIdAttached)
		}
	}

	return attached, nil
}

func (qcloud *QCloud) DetachDisk(diskId, nodeName string) error {

	glog.Infof("DetachDisk, disk:%s, instance:%s", diskId, nodeName)

	disk, err := norm.GetCbsDisk(diskId)
	getNodeRsp, err := norm.NormGetNodeInfoByName(nodeName)
	if err != nil || getNodeRsp == nil {
		code, _ := retrieveErrorCodeAndMessage(err)
		if code == -8017 {
			glog.Warningf("DetachDisk, node is not found, assume disk:%s is not attched to the node:%s",
				diskId, nodeName)
			return nil
		}

		return fmt.Errorf("DetachCbsDisk failed, disk:%s, get instance:%s, error:%v", diskId, nodeName, err)
	}

	if disk.InstanceIdAttached != getNodeRsp.UInstanceId {
		glog.Errorf("DetachDisk failed, disk:%s is not attached to node(%s:%s), but attached to: %s",
			diskId, nodeName, getNodeRsp.UInstanceId, disk.InstanceIdAttached)
		return nil
	}

	err = norm.DetachCbsDisk(diskId)
	if err != nil {
		return fmt.Errorf("DetachCbsDisk failed, disk:%s, instance:%s, error:%s", diskId, nodeName, err)
	}

	return nil
}
