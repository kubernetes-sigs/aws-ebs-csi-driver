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

package mounter

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	diskapi "github.com/kubernetes-csi/csi-proxy/client/api/disk/v1"
	diskclient "github.com/kubernetes-csi/csi-proxy/client/groups/disk/v1"
	"k8s.io/klog/v2"
	mountutils "k8s.io/mount-utils"
)

const (
	DefaultBlockSize = 4096
)

func (m NodeMounter) FindDevicePath(devicePath, volumeID, _, _ string) (string, error) {
	diskClient, err := diskclient.NewClient()
	if err != nil {
		return "", fmt.Errorf("error creating csi-proxy disk client: %q", err)
	}
	defer diskClient.Close()

	response, err := diskClient.ListDiskIDs(context.TODO(), &diskapi.ListDiskIDsRequest{})
	if err != nil {
		return "", fmt.Errorf("error listing disk ids: %q", err)
	}

	diskIDs := response.GetDiskIDs()

	foundDiskNumber := ""
	for diskNumber, diskID := range diskIDs {
		serialNumber := diskID.GetSerialNumber()
		cleanVolumeID := strings.ReplaceAll(volumeID, "-", "")
		if strings.Contains(serialNumber, cleanVolumeID) {
			foundDiskNumber = strconv.Itoa(int(diskNumber))
			break
		}
	}

	if foundDiskNumber == "" {
		return "", fmt.Errorf("disk number for device path %q volume id %q not found", devicePath, volumeID)
	}

	return foundDiskNumber, nil
}

func (m NodeMounter) PreparePublishTarget(target string) error {
	// On Windows, Mount will create the parent of target and mklink (create a symbolic link) at target later, so don't create a
	// directory at target now. Otherwise mklink will error: "Cannot create a file when that file already exists".
	// Instead, delete the target if it already exists (like if it was created by kubelet <1.20)
	// https://github.com/kubernetes/kubernetes/pull/88759
	klog.V(4).InfoS("NodePublishVolume: removing dir", "target", target)
	exists, err := m.PathExists(target)
	if err != nil {
		return fmt.Errorf("error checking path %q exists: %v", target, err)
	}

	proxyMounter, ok := m.SafeFormatAndMount.Interface.(*CSIProxyMounter)
	if !ok {
		return fmt.Errorf("failed to cast mounter to csi proxy mounter")
	}

	if exists {
		if err := proxyMounter.Rmdir(target); err != nil {
			return fmt.Errorf("error Rmdir target %q: %v", target, err)
		}
	}
	return nil
}

// IsBlockDevice checks if the given path is a block device
func (m NodeMounter) IsBlockDevice(fullPath string) (bool, error) {
	return false, nil
}

// getBlockSizeBytes gets the size of the disk in bytes
func (m NodeMounter) GetBlockSizeBytes(devicePath string) (int64, error) {
	proxyMounter, ok := m.SafeFormatAndMount.Interface.(*CSIProxyMounter)
	if !ok {
		return -1, fmt.Errorf("failed to cast mounter to csi proxy mounter")
	}

	sizeInBytes, err := proxyMounter.GetDeviceSize(devicePath)
	if err != nil {
		return -1, err
	}

	return sizeInBytes, nil
}

func (m NodeMounter) FormatAndMountSensitiveWithFormatOptions(source string, target string, fstype string, options []string, sensitiveOptions []string, formatOptions []string) error {
	proxyMounter, ok := m.SafeFormatAndMount.Interface.(*CSIProxyMounter)
	if !ok {
		return fmt.Errorf("failed to cast mounter to csi proxy mounter")
	}
	return proxyMounter.FormatAndMountSensitiveWithFormatOptions(source, target, fstype, options, sensitiveOptions, formatOptions)
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
	proxyMounter, ok := m.SafeFormatAndMount.Interface.(*CSIProxyMounter)
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
		pattern := `(Get-Item -Path \S+).Target, output: , error: <nil>|because it does not exist`
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

// IsCorruptedMnt return true if err is about corrupted mount point
func (m NodeMounter) IsCorruptedMnt(err error) bool {
	return mountutils.IsCorruptedMnt(err)
}

func (m *NodeMounter) MakeFile(path string) error {
	proxyMounter, ok := m.SafeFormatAndMount.Interface.(*CSIProxyMounter)
	if !ok {
		return fmt.Errorf("failed to cast mounter to csi proxy mounter")
	}
	return proxyMounter.MakeFile(path)
}

func (m *NodeMounter) MakeDir(path string) error {
	proxyMounter, ok := m.SafeFormatAndMount.Interface.(*CSIProxyMounter)
	if !ok {
		return fmt.Errorf("failed to cast mounter to csi proxy mounter")
	}
	return proxyMounter.MakeDir(path)
}

func (m *NodeMounter) PathExists(path string) (bool, error) {
	proxyMounter, ok := m.SafeFormatAndMount.Interface.(*CSIProxyMounter)
	if !ok {
		return false, fmt.Errorf("failed to cast mounter to csi proxy mounter")
	}
	return proxyMounter.ExistsPath(path)
}

func (m *NodeMounter) Resize(devicePath, deviceMountPath string) (bool, error) {
	proxyMounter, ok := m.SafeFormatAndMount.Interface.(*CSIProxyMounter)
	if !ok {
		return false, fmt.Errorf("failed to cast mounter to csi proxy mounter")
	}
	return proxyMounter.ResizeVolume(deviceMountPath)
}

// NeedResize called at NodeStage to ensure file system is the correct size
func (m *NodeMounter) NeedResize(devicePath, deviceMountPath string) (bool, error) {
	proxyMounter, ok := m.SafeFormatAndMount.Interface.(*CSIProxyMounter)
	if !ok {
		return false, fmt.Errorf("failed to cast mounter to csi proxy mounter")
	}

	deviceSize, err := proxyMounter.GetDeviceSize(devicePath)
	if err != nil {
		return false, err
	}

	fsSize, err := proxyMounter.GetVolumeSizeInBytes(deviceMountPath)
	if err != nil {
		return false, err
	}
	// Tolerate one block difference (4096 bytes)
	if deviceSize <= DefaultBlockSize+fsSize {
		return true, nil
	}
	return false, nil
}

// Unmount volume from target path
func (m *NodeMounter) Unpublish(target string) error {
	proxyMounter, ok := m.SafeFormatAndMount.Interface.(*CSIProxyMounter)
	if !ok {
		return fmt.Errorf("failed to cast mounter to csi proxy mounter")
	}
	// Remove symlink
	err := proxyMounter.Rmdir(target)
	if err != nil {
		return err
	}
	return nil
}

// Unmount volume from staging path
// usually this staging path is a global directory on the node
func (m *NodeMounter) Unstage(target string) error {
	proxyMounter, ok := m.SafeFormatAndMount.Interface.(*CSIProxyMounter)
	if !ok {
		return fmt.Errorf("failed to cast mounter to csi proxy mounter")
	}
	// Unmounts and offlines the disk via the CSI Proxy API
	err := proxyMounter.Unmount(target)
	if err != nil {
		return err
	}
	return nil
}
