//go:build linux
// +build linux

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
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"golang.org/x/sys/unix"
	"k8s.io/klog"
)

// findDevicePath finds path of device and verifies its existence
// if the device is not nvme, return the path directly
// if the device is nvme, finds and returns the nvme device path eg. /dev/nvme1n1
func (d *nodeService) findDevicePath(devicePath, volumeID string, partition string) (string, error) {
	exists, err := d.mounter.PathExists(devicePath)
	if err != nil {
		return "", err
	}

	// If the path exists, assume it is not nvme device
	if exists {
		if partition != "" {
			devicePath = devicePath + diskPartitionSuffix + partition
		}
		return devicePath, nil
	}

	// Else find the nvme device path using volume ID
	// This is the magic name on which AWS presents NVME devices under /dev/disk/by-id/
	// For example, vol-0fab1d5e3f72a5e23 creates a symlink at
	// /dev/disk/by-id/nvme-Amazon_Elastic_Block_Store_vol0fab1d5e3f72a5e23
	nvmeName := "nvme-Amazon_Elastic_Block_Store_" + strings.Replace(volumeID, "-", "", -1)

	nvmeDevicePath, err := findNvmeVolume(nvmeName)
	if err != nil {
		return "", err
	}
	if partition != "" {
		nvmeDevicePath = nvmeDevicePath + nvmeDiskPartitionSuffix + partition
	}
	return nvmeDevicePath, nil
}

// findNvmeVolume looks for the nvme volume with the specified name
// It follows the symlink (if it exists) and returns the absolute path to the device
func findNvmeVolume(findName string) (device string, err error) {
	p := filepath.Join("/dev/disk/by-id/", findName)
	stat, err := os.Lstat(p)
	if err != nil {
		if os.IsNotExist(err) {
			klog.V(5).Infof("[Debug] nvme path %q not found", p)
			return "", fmt.Errorf("nvme path %q not found", p)
		}
		return "", fmt.Errorf("error getting stat of %q: %v", p, err)
	}

	if stat.Mode()&os.ModeSymlink != os.ModeSymlink {
		klog.Warningf("nvme file %q found, but was not a symlink", p)
		return "", fmt.Errorf("nvme file %q found, but was not a symlink", p)
	}
	// Find the target, resolving to an absolute path
	// For example, /dev/disk/by-id/nvme-Amazon_Elastic_Block_Store_vol0fab1d5e3f72a5e23 -> ../../nvme2n1
	resolved, err := filepath.EvalSymlinks(p)
	if err != nil {
		return "", fmt.Errorf("error reading target of symlink %q: %v", p, err)
	}

	if !strings.HasPrefix(resolved, "/dev") {
		return "", fmt.Errorf("resolved symlink for %q was unexpected: %q", p, resolved)
	}

	return resolved, nil
}

// IsBlock checks if the given path is a block device
func (d *nodeService) IsBlockDevice(fullPath string) (bool, error) {
	var st unix.Stat_t
	err := unix.Stat(fullPath, &st)
	if err != nil {
		return false, err
	}

	return (st.Mode & unix.S_IFMT) == unix.S_IFBLK, nil
}

func (d *nodeService) getBlockSizeBytes(devicePath string) (int64, error) {
	cmd := d.mounter.Command("blockdev", "--getsize64", devicePath)
	output, err := cmd.Output()
	if err != nil {
		return -1, fmt.Errorf("error when getting size of block volume at path %s: output: %s, err: %v", devicePath, string(output), err)
	}
	strOut := strings.TrimSpace(string(output))
	gotSizeBytes, err := strconv.ParseInt(strOut, 10, 64)
	if err != nil {
		return -1, fmt.Errorf("failed to parse size %s as int", strOut)
	}
	return gotSizeBytes, nil
}
