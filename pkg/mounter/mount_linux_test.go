//go:build linux
// +build linux

/*
Copyright 2020 The Kubernetes Authors.

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
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/mount-utils"

	utilexec "k8s.io/utils/exec"
	fakeexec "k8s.io/utils/exec/testing"
)

func TestNeedResize(t *testing.T) {
	testcases := []struct {
		name            string
		devicePath      string
		deviceMountPath string
		deviceSize      string
		cmdOutputFsType string
		expectError     bool
		expectResult    bool
	}{
		{
			name:            "False - Unsupported fs type",
			devicePath:      "/dev/test1",
			deviceMountPath: "/mnt/test1",
			deviceSize:      "2048",
			cmdOutputFsType: "TYPE=ntfs",
			expectError:     true,
			expectResult:    false,
		},
	}

	for _, test := range testcases {
		t.Run(test.name, func(t *testing.T) {
			fcmd := fakeexec.FakeCmd{
				CombinedOutputScript: []fakeexec.FakeAction{
					func() ([]byte, []byte, error) { return []byte(test.deviceSize), nil, nil },
					func() ([]byte, []byte, error) { return []byte(test.cmdOutputFsType), nil, nil },
				},
			}
			fexec := fakeexec.FakeExec{
				CommandScript: []fakeexec.FakeCommandAction{
					func(cmd string, args ...string) utilexec.Cmd { return fakeexec.InitFakeCmd(&fcmd, cmd, args...) },
					func(cmd string, args ...string) utilexec.Cmd { return fakeexec.InitFakeCmd(&fcmd, cmd, args...) },
				},
			}
			safe := mount.SafeFormatAndMount{
				Interface: mount.New(""),
				Exec:      &fexec,
			}
			fakeMounter := NodeMounter{&safe}

			needResize, err := fakeMounter.NeedResize(test.devicePath, test.deviceMountPath)
			if needResize != test.expectResult {
				t.Fatalf("Expect result is %v but got %v", test.expectResult, needResize)
			}
			if !test.expectError && err != nil {
				t.Fatalf("Expect no error but got %v", err)
			}
		})
	}
}

func TestMakeDir(t *testing.T) {
	// Setup the full driver and its environment
	dir, err := os.MkdirTemp("", "mount-ebs-csi")
	if err != nil {
		t.Fatalf("error creating directory %v", err)
	}
	defer os.RemoveAll(dir)

	targetPath := filepath.Join(dir, "targetdir")

	mountObj, err := NewNodeMounter(false)
	if err != nil {
		t.Fatalf("error creating mounter %v", err)
	}

	if mountObj.MakeDir(targetPath) != nil {
		t.Fatalf("Expect no error but got: %v", err)
	}

	if mountObj.MakeDir(targetPath) != nil {
		t.Fatalf("Expect no error but got: %v", err)
	}

	if exists, err := mountObj.PathExists(targetPath); !exists {
		t.Fatalf("Expect no error but got: %v", err)
	}
}

func TestMakeFile(t *testing.T) {
	// Setup the full driver and its environment
	dir, err := os.MkdirTemp("", "mount-ebs-csi")
	if err != nil {
		t.Fatalf("error creating directory %v", err)
	}
	defer os.RemoveAll(dir)

	targetPath := filepath.Join(dir, "targetfile")

	mountObj, err := NewNodeMounter(false)
	if err != nil {
		t.Fatalf("error creating mounter %v", err)
	}

	if mountObj.MakeFile(targetPath) != nil {
		t.Fatalf("Expect no error but got: %v", err)
	}

	if mountObj.MakeFile(targetPath) != nil {
		t.Fatalf("Expect no error but got: %v", err)
	}

	if exists, err := mountObj.PathExists(targetPath); !exists {
		t.Fatalf("Expect no error but got: %v", err)
	}

}

func TestPathExists(t *testing.T) {
	// Setup the full driver and its environment
	dir, err := os.MkdirTemp("", "mount-ebs-csi")
	if err != nil {
		t.Fatalf("error creating directory %v", err)
	}
	defer os.RemoveAll(dir)

	targetPath := filepath.Join(dir, "notafile")

	mountObj, err := NewNodeMounter(false)
	if err != nil {
		t.Fatalf("error creating mounter %v", err)
	}

	exists, err := mountObj.PathExists(targetPath)

	if err != nil {
		t.Fatalf("Expect no error but got: %v", err)
	}

	if exists {
		t.Fatalf("Expected file %s to not exist", targetPath)
	}

}

func TestGetDeviceName(t *testing.T) {
	// Setup the full driver and its environment
	dir, err := os.MkdirTemp("", "mount-ebs-csi")
	if err != nil {
		t.Fatalf("error creating directory %v", err)
	}
	defer os.RemoveAll(dir)

	targetPath := filepath.Join(dir, "notafile")

	mountObj, err := NewNodeMounter(false)
	if err != nil {
		t.Fatalf("error creating mounter %v", err)
	}

	if _, _, err := mountObj.GetDeviceNameFromMount(targetPath); err != nil {
		t.Fatalf("Expect no error but got: %v", err)
	}

}

func TestFindDevicePath(t *testing.T) {
	testCases := []struct {
		name            string
		volumeID        string
		partition       string
		region          string
		createTempDir   bool
		symlink         bool
		verifyErr       error
		deviceSize      string
		cmdOutputFsType string
		expectedErr     error
	}{
		{
			name:            "Device path exists and matches volume ID",
			volumeID:        "vol-1234567890abcdef0",
			partition:       "1",
			createTempDir:   true,
			symlink:         false,
			verifyErr:       nil,
			deviceSize:      "1024",
			cmdOutputFsType: "ext4",
			expectedErr:     nil,
		},
		{
			name:            "Device path doesn't exist",
			volumeID:        "vol-1234567890abcdef0",
			partition:       "1",
			createTempDir:   false,
			symlink:         false,
			verifyErr:       nil,
			deviceSize:      "1024",
			cmdOutputFsType: "ext4",
			expectedErr:     fmt.Errorf("no device path for device %q volume %q found", "/temp/vol-1234567890abcdef0", "vol-1234567890abcdef0"),
		},
		{
			name:            "SBE region fallback",
			volumeID:        "vol-1234567890abcdef0",
			partition:       "1",
			region:          "snow",
			createTempDir:   false,
			symlink:         false,
			verifyErr:       nil,
			deviceSize:      "1024",
			cmdOutputFsType: "ext4",
			expectedErr:     nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var tmpDir string
			var err error

			if tc.createTempDir {
				tmpDir, err = os.MkdirTemp("", "temp-test-device-path")
				if err != nil {
					t.Fatalf("Failed to create temporary directory: %v", err)
				}
				defer os.RemoveAll(tmpDir)
			} else {
				tmpDir = "/temp"
			}

			devicePath := filepath.Join(tmpDir, tc.volumeID)
			expectedResult := devicePath + tc.partition

			fcmd := fakeexec.FakeCmd{
				CombinedOutputScript: []fakeexec.FakeAction{
					func() ([]byte, []byte, error) { return []byte(tc.deviceSize), nil, nil },
					func() ([]byte, []byte, error) { return []byte(tc.cmdOutputFsType), nil, nil },
				},
			}
			fexec := fakeexec.FakeExec{
				CommandScript: []fakeexec.FakeCommandAction{
					func(cmd string, args ...string) utilexec.Cmd { return fakeexec.InitFakeCmd(&fcmd, cmd, args...) },
					func(cmd string, args ...string) utilexec.Cmd { return fakeexec.InitFakeCmd(&fcmd, cmd, args...) },
				},
			}

			safe := mount.SafeFormatAndMount{
				Interface: mount.New(""),
				Exec:      &fexec,
			}

			fakeMounter := NodeMounter{&safe}

			if tc.createTempDir {
				if tc.symlink {
					symlinkErr := os.Symlink(devicePath, devicePath)
					if symlinkErr != nil {
						t.Fatalf("Failed to create symlink: %v", err)
					}
				} else {
					_, osCreateErr := os.Create(devicePath)
					if osCreateErr != nil {
						t.Fatalf("Failed to create device path: %v", err)
					}
				}
			}

			result, err := fakeMounter.FindDevicePath(devicePath, tc.volumeID, tc.partition, tc.region)

			if tc.region == "snow" {
				expectedResult = "/dev/vd" + tc.volumeID[len(tc.volumeID)-1:] + tc.partition
			}

			if tc.expectedErr == nil {
				assert.Equal(t, expectedResult, result)
				assert.NoError(t, err)
			} else {
				assert.Empty(t, result)
				assert.EqualError(t, err, tc.expectedErr.Error())
			}
		})
	}
}
