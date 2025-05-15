//go:build darwin

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
	"errors"

	mountutils "k8s.io/mount-utils"
)

const (
	stubMessage = "nodeService is unsupported for this platform"
)

/*
NOTE: This is stub implementation of nodeService so that maintainers without access to a Linux/Windows workstation can
run driver e2e tests.
*/

func NewSafeMounter() (*mountutils.SafeFormatAndMount, error) {
	return nil, errors.New("NewSafeMounter is not supported on this platform")
}

func NewSafeMounterV2() (*mountutils.SafeFormatAndMount, error) {
	return nil, errors.New("NewSafeMounterV2 is not supported on this platform")
}

func (m *NodeMounter) FindDevicePath(devicePath, volumeID, partition, region string) (string, error) {
	return stubMessage, errors.New(stubMessage)
}

func (m *NodeMounter) PreparePublishTarget(target string) error {
	return errors.New(stubMessage)
}

func (m *NodeMounter) IsBlockDevice(fullPath string) (bool, error) {
	return false, errors.New(stubMessage)
}

func (m *NodeMounter) GetBlockSizeBytes(devicePath string) (int64, error) {
	return 1, errors.New(stubMessage)
}

func (m NodeMounter) GetDeviceNameFromMount(mountPath string) (string, int, error) {
	return stubMessage, 0, errors.New(stubMessage)
}

func (m NodeMounter) IsCorruptedMnt(err error) bool {
	return false
}

func (m *NodeMounter) MakeFile(path string) error {
	return errors.New(stubMessage)
}

func (m *NodeMounter) MakeDir(path string) error {
	return errors.New(stubMessage)
}

func (m *NodeMounter) PathExists(path string) (bool, error) {
	return false, errors.New(stubMessage)
}

func (m *NodeMounter) Resize(devicePath, deviceMountPath string) (bool, error) {
	return false, errors.New(stubMessage)
}

func (m *NodeMounter) NeedResize(devicePath string, deviceMountPath string) (bool, error) {
	return false, errors.New(stubMessage)
}

func (m *NodeMounter) Unpublish(path string) error {
	return errors.New(stubMessage)
}

func (m *NodeMounter) Unstage(path string) error {
	return errors.New(stubMessage)
}

func (m *NodeMounter) GetVolumeStats(volumePath string) (VolumeStats, error) {
	return VolumeStats{}, errors.New(stubMessage)
}
