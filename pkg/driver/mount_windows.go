//go:build windows
// +build windows

/*
Copyright 2019 The Kubernetes Authors.

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

package driver

import (
	"fmt"
	"regexp"

	"github.com/kubernetes-sigs/aws-ebs-csi-driver/pkg/mounter"
)

func (m NodeMounter) FormatAndMount(source string, target string, fstype string, options []string) error {
	proxyMounter, ok := m.SafeFormatAndMount.Interface.(*mounter.CSIProxyMounter)
	if !ok {
		return fmt.Errorf("failed to cast mounter to csi proxy mounter")
	}
	return proxyMounter.FormatAndMount(source, target, fstype, options)
}

// GetDeviceNameFromMount returns the volume ID for a mount path.
// The ref count returned is always 1 or 0 because csi-proxy doesn't provide a
// way to determine the actual ref count (as opposed to Linux where the mount
// table gets read). In practice this shouldn't matter, as in the NodeStage
// case the ref count is ignored and in the NodeUnstage case, the ref count
// being >1 is just a warning.
// Command to determine ref count would be something like:
// Get-Volume -UniqueId "\\?\Volume{7c3da0c1-0000-0000-0000-010000000000}\" | Get-Partition | Select AccessPaths
func (m NodeMounter) GetDeviceNameFromMount(mountPath string) (string, int, error) {
	proxyMounter, ok := m.SafeFormatAndMount.Interface.(*mounter.CSIProxyMounter)
	if !ok {
		return "", 0, fmt.Errorf("failed to cast mounter to csi proxy mounter")
	}
	deviceName, err := proxyMounter.GetDeviceNameFromMount(mountPath, "")
	if err != nil {
		// HACK change csi-proxy behavior instead of relying on fragile internal
		// implementation details!
		// if err contains '"(Get-Item...).Target, output: , error: <nil>' then the
		// internal Get-Item cmdlet didn't fail but no item/device was found at the
		// path so we should return empty string and nil error just like the Linux
		// implementation would.
		pattern := `(Get-Item -Path \S+).Target, output: , error: <nil>`
		matched, matchErr := regexp.MatchString(pattern, err.Error())
		if matched {
			return "", 0, nil
		}
		err = fmt.Errorf("error getting device name from mount: %v", err)
		if matchErr != nil {
			err = fmt.Errorf("%v, and error matching pattern %q: %v", err, pattern, matchErr)
		}
		return "", 0, err
	}
	return deviceName, 1, nil
}

func (m *NodeMounter) MakeFile(path string) error {
	proxyMounter, ok := m.SafeFormatAndMount.Interface.(*mounter.CSIProxyMounter)
	if !ok {
		return fmt.Errorf("failed to cast mounter to csi proxy mounter")
	}
	return proxyMounter.MakeFile(path)
}

func (m *NodeMounter) MakeDir(path string) error {
	proxyMounter, ok := m.SafeFormatAndMount.Interface.(*mounter.CSIProxyMounter)
	if !ok {
		return fmt.Errorf("failed to cast mounter to csi proxy mounter")
	}
	return proxyMounter.MakeDir(path)
}

func (m *NodeMounter) PathExists(path string) (bool, error) {
	proxyMounter, ok := m.SafeFormatAndMount.Interface.(*mounter.CSIProxyMounter)
	if !ok {
		return false, fmt.Errorf("failed to cast mounter to csi proxy mounter")
	}
	return proxyMounter.ExistsPath(path)
}

func (m *NodeMounter) NeedResize(devicePath string, deviceMountPath string) (bool, error) {
	// TODO this is called at NodeStage to ensure file system is the correct size
	// Implement it to respect spec v1.4.0 https://github.com/container-storage-interface/spec/pull/452
	return false, nil
}
