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
	"k8s.io/klog"
	"os"
	"strconv"
	"strings"

	mountutils "k8s.io/mount-utils"
)

// GetDeviceNameFromMount returns the volume ID for a mount path.
func (m NodeMounter) GetDeviceNameFromMount(mountPath string) (string, int, error) {
	return mountutils.GetDeviceNameFromMount(m, mountPath)
}

// This function is mirrored in ./sanity_test.go to make sure sanity test covered this block of code
// Please mirror the change to func MakeFile in ./sanity_test.go
func (m *NodeMounter) MakeFile(path string) error {
	f, err := os.OpenFile(path, os.O_CREATE, os.FileMode(0644))
	if err != nil {
		if !os.IsExist(err) {
			return err
		}
	}
	if err = f.Close(); err != nil {
		return err
	}
	return nil
}

// This function is mirrored in ./sanity_test.go to make sure sanity test covered this block of code
// Please mirror the change to func MakeFile in ./sanity_test.go
func (m *NodeMounter) MakeDir(path string) error {
	err := os.MkdirAll(path, os.FileMode(0755))
	if err != nil {
		if !os.IsExist(err) {
			return err
		}
	}
	return nil
}

// This function is mirrored in ./sanity_test.go to make sure sanity test covered this block of code
// Please mirror the change to func MakeFile in ./sanity_test.go
func (m *NodeMounter) PathExists(path string) (bool, error) {
	return mountutils.PathExists(path)
}

//TODO: use common util from vendor kubernetes/mount-util
func (m *NodeMounter) NeedResize(devicePath string, deviceMountPath string) (bool, error) {
	// TODO(xiangLi) resize fs size on formatted file system following this PR https://github.com/kubernetes/kubernetes/pull/99223
	// Port the in-tree un-released change first, need to remove after in-tree release
	deviceSize, err := m.getDeviceSize(devicePath)
	if err != nil {
		return false, err
	}
	var fsSize, blockSize uint64
	format, err := m.SafeFormatAndMount.GetDiskFormat(devicePath)
	if err != nil {
		formatErr := fmt.Errorf("ResizeFS.Resize - error checking format for device %s: %v", devicePath, err)
		return false, formatErr
	}

	// If disk has no format, there is no need to resize the disk because mkfs.*
	// by default will use whole disk anyways.
	if format == "" {
		return false, nil
	}

	klog.V(3).Infof("ResizeFs.needResize - checking mounted volume %s", devicePath)
	switch format {
	case "ext3", "ext4":
		blockSize, fsSize, err = m.getExtSize(devicePath)
		klog.V(5).Infof("Ext size: filesystem size=%d, block size=%d", fsSize, blockSize)
	case "xfs":
		blockSize, fsSize, err = m.getXFSSize(deviceMountPath)
		klog.V(5).Infof("Xfs size: filesystem size=%d, block size=%d, err=%v", fsSize, blockSize, err)
	default:
		klog.Errorf("Not able to parse given filesystem info. fsType: %s, will not resize", format)
		return false, fmt.Errorf("Could not parse fs info on given filesystem format: %s. Supported fs types are: xfs, ext3, ext4", format)
	}
	if err != nil {
		return false, err
	}
	// Tolerate one block difference, just in case of rounding errors somewhere.
	klog.V(5).Infof("Volume %s: device size=%d, filesystem size=%d, block size=%d", devicePath, deviceSize, fsSize, blockSize)
	if deviceSize <= fsSize+blockSize {
		return false, nil
	}
	return true, nil
}
func (m *NodeMounter) getDeviceSize(devicePath string) (uint64, error) {
	output, err := m.SafeFormatAndMount.Exec.Command("blockdev", "--getsize64", devicePath).CombinedOutput()
	outStr := strings.TrimSpace(string(output))
	if err != nil {
		return 0, fmt.Errorf("failed to read size of device %s: %s: %s", devicePath, err, outStr)
	}
	size, err := strconv.ParseUint(outStr, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("failed to parse size of device %s %s: %s", devicePath, outStr, err)
	}
	return size, nil
}

func (m *NodeMounter) getExtSize(devicePath string) (uint64, uint64, error) {
	output, err := m.SafeFormatAndMount.Exec.Command("dumpe2fs", "-h", devicePath).CombinedOutput()
	if err != nil {
		return 0, 0, fmt.Errorf("failed to read size of filesystem on %s: %s: %s", devicePath, err, string(output))
	}

	blockSize, blockCount, _ := m.parseFsInfoOutput(string(output), ":", "block size", "block count")

	if blockSize == 0 {
		return 0, 0, fmt.Errorf("could not find block size of device %s", devicePath)
	}
	if blockCount == 0 {
		return 0, 0, fmt.Errorf("could not find block count of device %s", devicePath)
	}
	return blockSize, blockSize * blockCount, nil
}

func (m *NodeMounter) getXFSSize(devicePath string) (uint64, uint64, error) {
	output, err := m.SafeFormatAndMount.Exec.Command("xfs_io", "-c", "statfs", devicePath).CombinedOutput()
	if err != nil {
		return 0, 0, fmt.Errorf("failed to read size of filesystem on %s: %s: %s", devicePath, err, string(output))
	}

	blockSize, blockCount, _ := m.parseFsInfoOutput(string(output), "=", "geom.bsize", "geom.datablocks")

	if blockSize == 0 {
		return 0, 0, fmt.Errorf("could not find block size of device %s", devicePath)
	}
	if blockCount == 0 {
		return 0, 0, fmt.Errorf("could not find block count of device %s", devicePath)
	}
	return blockSize, blockSize * blockCount, nil
}

func (m *NodeMounter) parseFsInfoOutput(cmdOutput string, spliter string, blockSizeKey string, blockCountKey string) (uint64, uint64, error) {
	lines := strings.Split(cmdOutput, "\n")
	var blockSize, blockCount uint64
	var err error

	for _, line := range lines {
		tokens := strings.Split(line, spliter)
		if len(tokens) != 2 {
			continue
		}
		key, value := strings.ToLower(strings.TrimSpace(tokens[0])), strings.ToLower(strings.TrimSpace(tokens[1]))
		if key == blockSizeKey {
			blockSize, err = strconv.ParseUint(value, 10, 64)
			if err != nil {
				return 0, 0, fmt.Errorf("failed to parse block size %s: %s", value, err)
			}
		}
		if key == blockCountKey {
			blockCount, err = strconv.ParseUint(value, 10, 64)
			if err != nil {
				return 0, 0, fmt.Errorf("failed to parse block count %s: %s", value, err)
			}
		}
	}
	return blockSize, blockCount, err
}
