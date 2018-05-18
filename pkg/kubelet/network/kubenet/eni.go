package kubenet

import (
	"encoding/json"
)

// var (
// 	CNI_CONFIG_INFO = []byte(`{"eni":{"spec":{"peerVpcAppId":1251707795,"peerVpcId":"vpc-53n3fcml","name":"test","cvmOwnerAppId":1251707795,"cvmOwnerUin":"3321337994"},"status":{"mac":"20:90:6F:2B:7D:1F","pip":"169.254.131.218","eniId":"eni-d5kzo2zt","mask":17},"ClusterDeploy":true},"bridge":{"isDefaultRoute":false,"routeList":null}}`)
// )

type EniSpec struct {
	PeerVpcAppId  uint64 `json:"peerVpcAppId"`
	PeerVpcId     string `json:"peerVpcId"`
	PeerSubnetId  string `json:"peerSubnetId,omitempty"`
	Name          string `json:"name"`
	Description   string `json:"description,omitempty"`
	CvmOwnerAppID uint64 `json:"cvmOwnerAppId"`
	CvmOwnerUin   string `json:"cvmOwnerUin"`
}

type EniStatus struct {
	Mac   string `json:"mac"`
	Pip   string `json:"pip"`
	EniId string `json:"eniId"`
	Mask  int    `json:"mask"`
}

type EniConf struct {
	Spec   EniSpec    `json:"spec"`
	Status *EniStatus `json:"status,omitempty"`
}

type BridgeConf struct {
	IsDefaultRoute bool     `json:"isDefaultRoute"`
	RouteList      []string `json:"routeList"`
}

type QcloudCniConf struct {
	Eni       EniConf `json:"eni"`
	UseBridge bool    `json:"useBridge"`
}

type PodYAML struct {
	Metadata struct {
		Annotations struct {
			QcloudCNICONFIGINFO string `json:"qcloud.CNI_CONFIG_INFO"`
		} `json:"annotations"`
	} `json:"metadata"`
}

func GetQcloudCniInfo(podYAML []byte) (*QcloudCniConf, error) {
	var a PodYAML
	err := json.Unmarshal(podYAML, &a)
	if err != nil {
		return nil, err
	}

	var cniConf QcloudCniConf
	err = json.Unmarshal([]byte(a.Metadata.Annotations.QcloudCNICONFIGINFO), &cniConf)
	if err != nil {
		return nil, err
	}
	return &cniConf, nil
}
