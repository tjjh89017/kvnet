/*
Copyright 2026.

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

package helper

import (
	"fmt"
	"net"
	"os/exec"
	"strconv"
	"strings"

	kvnetv1alpha1 "github.com/tjjh89017/kvnet/api/v1alpha1"
)

// ParseDeviceName strips the node-name prefix from a CR name to get the device name.
func ParseDeviceName(crName, nodeName string) (string, error) {
	prefix := nodeName + "."
	if !strings.HasPrefix(crName, prefix) {
		return "", fmt.Errorf("CR name %q does not match node %q", crName, nodeName)
	}
	return strings.TrimPrefix(crName, prefix), nil
}

// ExecCmd runs a command and returns an error if it fails.
func ExecCmd(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("command %q failed: %w, output: %s", append([]string{name}, args...), err, string(output))
	}
	return nil
}

// IsBridgeSlave checks if a network device is attached to a bridge as a slave.
func IsBridgeSlave(devName string) bool {
	cmd := exec.Command("ip", "-d", "link", "show", "dev", devName)
	output, err := cmd.Output()
	if err != nil {
		return false
	}
	return strings.Contains(string(output), "bridge_slave")
}

// GetInterfaceIP returns the first IPv4 address of a network interface.
func GetInterfaceIP(devName string) (string, error) {
	iface, err := net.InterfaceByName(devName)
	if err != nil {
		return "", fmt.Errorf("interface %q not found: %w", devName, err)
	}
	addrs, err := iface.Addrs()
	if err != nil {
		return "", fmt.Errorf("get addrs for %q: %w", devName, err)
	}
	for _, addr := range addrs {
		if ipnet, ok := addr.(*net.IPNet); ok && ipnet.IP.To4() != nil {
			return ipnet.IP.String(), nil
		}
	}
	return "", fmt.Errorf("no IPv4 address found on %q", devName)
}

// ApplyPortVLANConfig configures bridge port VLAN settings for a device.
func ApplyPortVLANConfig(devName string, cfg *kvnetv1alpha1.PortVLANConfig) error {
	if cfg.Pvid != nil {
		if err := ExecCmd("bridge", "vlan", "add", "dev", devName, "vid", strconv.Itoa(*cfg.Pvid), "pvid", "untagged"); err != nil {
			return fmt.Errorf("set pvid %d on %s: %w", *cfg.Pvid, devName, err)
		}
	}

	for _, vid := range cfg.Vids {
		if err := ExecCmd("bridge", "vlan", "add", "dev", devName, "vid", strconv.Itoa(vid)); err != nil {
			return fmt.Errorf("add vid %d on %s: %w", vid, devName, err)
		}
	}

	for _, mapping := range cfg.TunnelInfo {
		vidEnd := mapping.Vid
		if mapping.VidEnd != nil {
			vidEnd = *mapping.VidEnd
		}
		for i := 0; i <= vidEnd-mapping.Vid; i++ {
			vid := mapping.Vid + i
			vni := mapping.Vni + i
			if err := ExecCmd("bridge", "vlan", "add", "dev", devName, "vid", strconv.Itoa(vid), "tunnel_info", "id", strconv.Itoa(vni)); err != nil {
				return fmt.Errorf("add tunnel_info vid=%d vni=%d on %s: %w", vid, vni, devName, err)
			}
		}
	}

	return nil
}
