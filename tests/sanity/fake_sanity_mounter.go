// Copyright 2024 The Kubernetes Authors.
//
// Licensed under the Apache License, Version 2.0 (the 'License');
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//    http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an 'AS IS' BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package sanity

import (
	"fmt"
	"os"

	"github.com/kubernetes-sigs/aws-ebs-csi-driver/pkg/cloud"
	"k8s.io/mount-utils"
)

var mounts = make(map[string]string)

type FakeMounter struct{}

func NewFakeMounter() *FakeMounter {
	return &FakeMounter{}
}

func (m *FakeMounter) FindDevicePath(devicePath, volumeID, partition, region string) (string, error) {
	if len(devicePath) == 0 {
		return devicePath, cloud.ErrNotFound
	}
	return devicePath, nil
}

func (m *FakeMounter) PreparePublishTarget(target string) error {
	if err := m.MakeDir(target); err != nil {
		return fmt.Errorf("could not create dir %q: %w", target, err)
	}
	return nil
}

func (m *FakeMounter) IsBlockDevice(fullPath string) (bool, error) {
	return false, nil
}

func (m *FakeMounter) GetBlockSizeBytes(devicePath string) (int64, error) {
	return 0, nil
}

func (m *FakeMounter) GetDeviceNameFromMount(mountPath string) (string, int, error) {
	return mounts[mountPath], 0, nil
}

func (m *FakeMounter) IsCorruptedMnt(err error) bool {
	return false
}

func (m *FakeMounter) MakeFile(path string) error {
	return nil
}

func (m *FakeMounter) MakeDir(path string) error {
	err := os.MkdirAll(path, os.FileMode(0755))
	if err != nil {
		if !os.IsExist(err) {
			return err
		}
	}
	return nil
}

func (m *FakeMounter) PathExists(path string) (bool, error) {
	_, exists := mounts[path]
	if !exists {
		return false, nil
	}
	return true, nil
}

func (m *FakeMounter) Resize(devicePath, deviceMountPath string) (bool, error) {
	return false, nil
}

func (m *FakeMounter) NeedResize(devicePath string, deviceMountPath string) (bool, error) {
	return false, nil
}

func (m *FakeMounter) Unpublish(path string) error {
	return m.Unstage(path)
}

func (m *FakeMounter) Unstage(path string) error {
	err := os.RemoveAll(path)
	return err
}

func (m *FakeMounter) Mount(source string, target string, fstype string, options []string) error {
	mounts[target] = source
	return nil
}

func (m *FakeMounter) CanSafelySkipMountPointCheck() bool {
	return false
}

func (m *FakeMounter) FormatAndMountSensitiveWithFormatOptions(source, target, fstype string, options, sensitiveOptions, formatOptions []string) error {
	return nil
}

func (m *FakeMounter) GetMountRefs(pathname string) ([]string, error) {
	return nil, nil
}

func (m *FakeMounter) IsLikelyNotMountPoint(file string) (bool, error) {
	return true, nil
}

func (m *FakeMounter) IsMountPoint(file string) (bool, error) {
	return false, nil
}

func (m *FakeMounter) List() ([]mount.MountPoint, error) {
	return nil, nil
}

func (m *FakeMounter) MountSensitive(source, target, fstype string, options, sensitiveOptions []string) error {
	return nil
}

func (m *FakeMounter) MountSensitiveWithoutSystemd(source, target, fstype string, options, sensitiveOptions []string) error {
	return nil
}

func (m *FakeMounter) MountSensitiveWithoutSystemdWithMountFlags(source, target, fstype string, options, sensitiveOptions, mountFlags []string) error {
	return nil
}

func (m *FakeMounter) Unmount(target string) error {
	return nil
}
