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
	norm "k8s.io/kubernetes/pkg/cloudprovider/providers/qcloud/component"
)

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
