// +build linux

/*
Copyright 2017 The Kubernetes Authors.

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

package resizefs

import (
	"fmt"
	"strconv"
	"strings"

	"k8s.io/klog"
	"k8s.io/utils/mount"
)

// ResizeFs Provides support for resizing file systems
type ResizeFs struct {
	mounter *mount.SafeFormatAndMount
}

// NewResizeFs returns new instance of resizer
func NewResizeFs(mounter *mount.SafeFormatAndMount) *ResizeFs {
	return &ResizeFs{mounter: mounter}
}

// Resize perform resize of file system
func (resizefs *ResizeFs) Resize(devicePath string, deviceMountPath string) (bool, error) {
	format, err := resizefs.mounter.GetDiskFormat(devicePath)

	if err != nil {
		formatErr := fmt.Errorf("ResizeFS.Resize - error checking format for device %s: %v", devicePath, err)
		return false, formatErr
	}

	// If disk has no format, there is no need to resize the disk because mkfs.*
	// by default will use whole disk anyways.
	if format == "" {
		return false, nil
	}

	klog.V(3).Infof("ResizeFS.Resize - Expanding mounted volume %s", devicePath)
	switch format {
	case "ext3", "ext4":
		return resizefs.extResize(devicePath)
	case "xfs":
		return resizefs.xfsResize(deviceMountPath)
	}
	return false, fmt.Errorf("ResizeFS.Resize - resize of format %s is not supported for device %s mounted at %s", format, devicePath, deviceMountPath)
}

func (resizefs *ResizeFs) ResizeIfNecessary(devicePath string, deviceMountPath string) (bool, error) {
	resize, err := resizefs.needResize(devicePath, deviceMountPath)
	if err != nil {
		return false, err
	}
	if resize {
		klog.V(2).Infof("Volume %s needs resizing", devicePath)
		return resizefs.Resize(devicePath, deviceMountPath)
	}
	return false, nil
}

func (resizefs *ResizeFs) getDeviceSize(devicePath string) (uint64, error) {
	output, err := resizefs.mounter.Exec.Command("blockdev", "--getsize64", devicePath).CombinedOutput()
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

func (resizefs *ResizeFs) getExtSize(devicePath string) (uint64, uint64, error) {
	output, err := resizefs.mounter.Exec.Command("dumpe2fs", "-h", devicePath).CombinedOutput()
	if err != nil {
		return 0, 0, fmt.Errorf("failed to read size of filesystem on %s: %s: %s", devicePath, err, string(output))
	}

	lines := strings.Split(string(output), "\n")
	var blockSize, blockCount uint64

	for _, line := range lines {
		tokens := strings.Split(line, ":")
		if len(tokens) != 2 {
			continue
		}
		key, value := strings.TrimSpace(tokens[0]), strings.TrimSpace(tokens[1])
		if key == "Block count" {
			blockCount, err = strconv.ParseUint(value, 10, 64)
			if err != nil {
				return 0, 0, fmt.Errorf("failed to parse block count %s: %s", value, err)
			}
		}
		if key == "Block size" {
			blockSize, err = strconv.ParseUint(value, 10, 64)
			if err != nil {
				return 0, 0, fmt.Errorf("failed to parse block size %s: %s", value, err)
			}
		}
	}
	if blockSize == 0 {
		return 0, 0, fmt.Errorf("could not find block size of device %s", devicePath)
	}
	if blockCount == 0 {
		return 0, 0, fmt.Errorf("could not find block count of device %s", devicePath)
	}
	return blockSize, blockSize * blockCount, nil
}

func (resizefs *ResizeFs) getXFSSize(devicePath string) (uint64, uint64, error) {
	// TODO: implement parsing of xfs_info <path>
	// meta-data=/dev/loop0             isize=512    agcount=4, agsize=655424 blks
	//          =                       sectsz=512   attr=2, projid32bit=1
	//          =                       crc=1        finobt=0 spinodes=0
	// data     =                       bsize=4096   blocks=2621696, imaxpct=25
	//          =                       sunit=0      swidth=0 blks
	// naming   =version 2              bsize=4096   ascii-ci=0 ftype=1
	// log      =internal               bsize=4096   blocks=2560, version=2
	//          =                       sectsz=512   sunit=0 blks, lazy-count=1
	// realtime =none                   extsz=4096   blocks=0, rtextents=0
	//
	// Need to parse "bsize=<xxx>" and "blocks=<yyy>" in "data =" segment.
	// There is no machine-friendly output.

	return 0, 0, fmt.Errorf("unimplemented")
}

func (resizefs *ResizeFs) needResize(devicePath string, deviceMountPath string) (bool, error) {
	deviceSize, err := resizefs.getDeviceSize(devicePath)
	if err != nil {
		return false, err
	}
	var fsSize, blockSize uint64
	format, err := resizefs.mounter.GetDiskFormat(devicePath)
	if err != nil {
		formatErr := fmt.Errorf("ResizeFS.Resize - error checking format for device %s: %v", devicePath, err)
		return false, formatErr
	}

	// If disk has no format, there is no need to resize the disk because mkfs.*
	// by default will use whole disk anyways.
	if format == "" {
		return false, nil
	}

	klog.V(3).Infof("ResizeFS.needResize - checking mounted volume %s", devicePath)
	switch format {
	case "ext3", "ext4":
		blockSize, fsSize, err = resizefs.getExtSize(devicePath)
	case "xfs":
		blockSize, fsSize, err = resizefs.getXFSSize(deviceMountPath)
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

func (resizefs *ResizeFs) extResize(devicePath string) (bool, error) {
	output, err := resizefs.mounter.Exec.Command("resize2fs", devicePath).CombinedOutput()
	if err == nil {
		klog.V(2).Infof("Device %s resized successfully", devicePath)
		return true, nil
	}

	resizeError := fmt.Errorf("resize of device %s failed: %v. resize2fs output: %s", devicePath, err, string(output))
	return false, resizeError

}

func (resizefs *ResizeFs) xfsResize(deviceMountPath string) (bool, error) {
	args := []string{"-d", deviceMountPath}
	output, err := resizefs.mounter.Exec.Command("xfs_growfs", args...).CombinedOutput()

	if err == nil {
		klog.V(2).Infof("Device %s resized successfully", deviceMountPath)
		return true, nil
	}

	resizeError := fmt.Errorf("resize of device %s failed: %v. xfs_growfs output: %s", deviceMountPath, err, string(output))
	return false, resizeError
}
