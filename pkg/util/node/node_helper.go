package node

import (
	"time"
	"net"
	"strconv"
	"k8s.io/api/core/v1"
	"os"
)

const (
	ENABLE_CHECK_KUBELET  = "ENABLE_CHECK_KUBELET"
	CHECK_KUBELET_TIMEOUT = "CHECK_KUBELET_TIMEOUT"
)

func IsNodeUnknown(node *v1.Node) bool {
	enableCheckKubelet := true
	timeout := 3 * time.Second
	if os.Getenv(ENABLE_CHECK_KUBELET) == "0" {
		enableCheckKubelet = false
	}
	if enableCheckKubelet {
		if timeoutStr := os.Getenv(CHECK_KUBELET_TIMEOUT); timeoutStr != "" {
			if timeoutEnv, err := time.ParseDuration(timeoutStr); err == nil && timeoutEnv > 0 {
				timeout = timeoutEnv
			}
		}
		if IsKubeletPortAlive(node, timeout) {
			return false
		}
	}
	return true
}

func IsKubeletPortAlive(node *v1.Node, timeout time.Duration) bool {
	internalIP, _ := GetPreferredNodeAddress(node, []v1.NodeAddressType{v1.NodeInternalIP})
	return IsPortAlive(internalIP, int64(node.Status.DaemonEndpoints.KubeletEndpoint.Port), timeout)
}

func IsPortAlive(ip string, port int64, timeout time.Duration) bool {
	if net.ParseIP(ip) == nil || port < 0 || port > 65535 {
		return false
	}
	addr := ip + ":" + strconv.FormatInt(port, 10)
	conn, err := net.DialTimeout("tcp", addr, timeout)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}
