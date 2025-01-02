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

type fakeMounter struct {
	mounts map[string]string
}

func newFakeMounter() *fakeMounter {
	return &fakeMounter{
		mounts: make(map[string]string),
	}
}

func (m *fakeMounter) FindDevicePath(devicePath, volumeID, partition, region string) (string, error) {
	if len(devicePath) == 0 {
		return devicePath, cloud.ErrNotFound
	}
	return devicePath, nil
}

func (m *fakeMounter) PreparePublishTarget(target string) error {
	if err := m.MakeDir(target); err != nil {
		return fmt.Errorf("could not create dir %q: %w", target, err)
	}
	return nil
}

func (m *fakeMounter) IsBlockDevice(fullPath string) (bool, error) {
	return false, nil
}

func (m *fakeMounter) GetBlockSizeBytes(devicePath string) (int64, error) {
	return 0, nil
}

func (m *fakeMounter) GetDeviceNameFromMount(mountPath string) (string, int, error) {
	return m.mounts[mountPath], 0, nil
}

func (m *fakeMounter) IsCorruptedMnt(err error) bool {
	return false
}

func (m *fakeMounter) MakeFile(path string) error {
	return nil
}

func (m *fakeMounter) MakeDir(path string) error {
	err := os.MkdirAll(path, os.FileMode(0755))
	if err != nil {
		if !os.IsExist(err) {
			return err
		}
	}
	return nil
}

func (m *fakeMounter) PathExists(path string) (bool, error) {
	_, exists := m.mounts[path]
	if !exists {
		return false, nil
	}
	return true, nil
}

func (m *fakeMounter) Resize(devicePath, deviceMountPath string) (bool, error) {
	return false, nil
}

func (m *fakeMounter) NeedResize(devicePath string, deviceMountPath string) (bool, error) {
	return false, nil
}

func (m *fakeMounter) Unpublish(path string) error {
	return m.Unstage(path)
}

func (m *fakeMounter) Unstage(path string) error {
	err := os.RemoveAll(path)
	return err
}

func (m *fakeMounter) Mount(source string, target string, fstype string, options []string) error {
	m.mounts[target] = source
	return nil
}

func (m *fakeMounter) CanSafelySkipMountPointCheck() bool {
	return false
}

func (m *fakeMounter) FormatAndMountSensitiveWithFormatOptions(source, target, fstype string, options, sensitiveOptions, formatOptions []string) error {
	return nil
}

func (m *fakeMounter) GetMountRefs(pathname string) ([]string, error) {
	return nil, nil
}

func (m *fakeMounter) IsLikelyNotMountPoint(file string) (bool, error) {
	return true, nil
}

func (m *fakeMounter) IsMountPoint(file string) (bool, error) {
	return false, nil
}

func (m *fakeMounter) List() ([]mount.MountPoint, error) {
	return nil, nil
}

func (m *fakeMounter) MountSensitive(source, target, fstype string, options, sensitiveOptions []string) error {
	return nil
}

func (m *fakeMounter) MountSensitiveWithoutSystemd(source, target, fstype string, options, sensitiveOptions []string) error {
	return nil
}

func (m *fakeMounter) MountSensitiveWithoutSystemdWithMountFlags(source, target, fstype string, options, sensitiveOptions, mountFlags []string) error {
	return nil
}

func (m *fakeMounter) Unmount(target string) error {
	return nil
}
