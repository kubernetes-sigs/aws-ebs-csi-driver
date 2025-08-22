//go:build linux

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
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

const fakeVolumeName = "vol11111111111111111"
const fakeIncorrectVolumeName = "vol21111111111111111"

func TestVerifyVolumeSerialMatch(t *testing.T) {
	type testCase struct {
		name        string
		execOutput  string
		path        string
		execError   error
		expectError bool
	}
	testCases := []testCase{
		{
			name:       "success: empty",
			execOutput: "",
		},
		{
			name:       "success: single",
			execOutput: fakeVolumeName,
		},
		{
			name:       "success: multiple",
			execOutput: fakeVolumeName + "\n" + fakeVolumeName + "\n" + fakeVolumeName,
		},
		{
			name:       "success: whitespace",
			execOutput: "\t     " + fakeVolumeName + "         \n    \t    ",
		},
		{
			name:       "success: extra output",
			execOutput: "extra output without name in it\n" + fakeVolumeName,
		},
		{
			name:      "success: failed command",
			execError: errors.New("Exec failed"),
		},
		{
			name:        "failure: wrong volume",
			execOutput:  fakeIncorrectVolumeName,
			expectError: true,
		},
		{
			name:        "failure: mixed",
			execOutput:  fakeVolumeName + "\n" + fakeIncorrectVolumeName,
			expectError: true,
		},
		{
			name:        "failure: bad path (malicious argument)",
			path:        "--fake-malicious-path=do_evil",
			expectError: true,
		},
		{
			name:        "failure: bad path (directory traversal)",
			path:        "/dev/../nvme1n1",
			expectError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockExecRunner := func(_ string, _ ...string) ([]byte, error) {
				return []byte(tc.execOutput), tc.execError
			}

			path := "/dev/nvme1n1"
			if tc.path != "" {
				path = tc.path
			}
			result := verifyVolumeSerialMatch(path, fakeVolumeName, mockExecRunner)
			if tc.expectError {
				assert.Error(t, result)
			} else {
				require.NoError(t, result)
			}
		})
	}
}
