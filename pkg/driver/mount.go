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
	"os"

	"github.com/kubernetes-sigs/aws-ebs-csi-driver/pkg/mounter"
	mountutils "k8s.io/mount-utils"
	utilexec "k8s.io/utils/exec"
)

type Mounter interface {
	// Implemented by NodeMounter.SafeFormatAndMount
	mountutils.Interface
	FormatAndMount(source string, target string, fstype string, options []string) error

	// Implemented by NodeMounter.SafeFormatAndMount.Exec
	// TODO this won't make sense on Windows with csi-proxy
	utilexec.Interface

	// Implemented by NodeMounter below
	GetDeviceNameFromMount(mountPath string) (string, int, error)
	// TODO this won't make sense on Windows with csi-proxy
	MakeFile(path string) error
	MakeDir(path string) error
	PathExists(path string) (bool, error)
}

type NodeMounter struct {
	mountutils.SafeFormatAndMount
	utilexec.Interface
}

func newNodeMounter() (Mounter, error) {
	safeMounter, err := mounter.NewSafeMounter()
	if err != nil {
		return nil, err
	}
	return &NodeMounter{*safeMounter, safeMounter.Exec}, nil
}

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
