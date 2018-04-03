package kubenet

import (
	"encoding/json"
)

type QcloudCniConf struct {
	UseBridge bool    `json:"useBridge"`
}

func GetQcloudCniSepc(annotations string) (*QcloudCniConf, error) {
	var cniConf QcloudCniConf
	err := json.Unmarshal([]byte(annotations), &cniConf)
	if err != nil {
		return nil, err
	}
	return &cniConf, nil
}