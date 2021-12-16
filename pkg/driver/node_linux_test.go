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
	"io/fs"
	"os"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/kubernetes-sigs/aws-ebs-csi-driver/pkg/cloud"
	"github.com/kubernetes-sigs/aws-ebs-csi-driver/pkg/driver/internal"
	"github.com/stretchr/testify/assert"
)

func TestFindDevicePath(t *testing.T) {
	devicePath := "/dev/xvbda"
	nvmeDevicePath := "/dev/nvme1n1"
	volumeID := "vol-test"
	nvmeName := "/dev/disk/by-id/nvme-Amazon_Elastic_Block_Store_voltest"
	deviceFileInfo := fs.FileInfo(&fakeFileInfo{devicePath, os.ModeDevice})
	symlinkFileInfo := fs.FileInfo(&fakeFileInfo{nvmeName, os.ModeSymlink})
	nvmeDevicePathSymlinkFileInfo := fs.FileInfo(&fakeFileInfo{nvmeDevicePath, os.ModeSymlink})
	type testCase struct {
		name             string
		devicePath       string
		volumeID         string
		partition        string
		expectMock       func(mockMounter MockMounter, mockDeviceIdentifier MockDeviceIdentifier)
		expectDevicePath string
		expectError      string
	}
	testCases := []testCase{
		{
			name:       "11: device path exists and nvme device path exists",
			devicePath: devicePath,
			volumeID:   volumeID,
			partition:  "",
			expectMock: func(mockMounter MockMounter, mockDeviceIdentifier MockDeviceIdentifier) {
				gomock.InOrder(
					mockMounter.EXPECT().PathExists(gomock.Eq(devicePath)).Return(true, nil),
					mockDeviceIdentifier.EXPECT().Lstat(gomock.Eq(devicePath)).Return(nvmeDevicePathSymlinkFileInfo, nil),
					mockDeviceIdentifier.EXPECT().EvalSymlinks(gomock.Eq(devicePath)).Return(nvmeDevicePath, nil),
				)
			},
			expectDevicePath: nvmeDevicePath,
		},
		{
			name:       "10: device path exists and nvme device path doesn't exist",
			devicePath: devicePath,
			volumeID:   volumeID,
			partition:  "",
			expectMock: func(mockMounter MockMounter, mockDeviceIdentifier MockDeviceIdentifier) {
				gomock.InOrder(
					mockMounter.EXPECT().PathExists(gomock.Eq(devicePath)).Return(true, nil),
					mockDeviceIdentifier.EXPECT().Lstat(gomock.Eq(devicePath)).Return(deviceFileInfo, nil),
				)
			},
			expectDevicePath: devicePath,
		},
		{
			name:       "01: device path doesn't exist and nvme device path exists",
			devicePath: devicePath,
			volumeID:   volumeID,
			partition:  "",
			expectMock: func(mockMounter MockMounter, mockDeviceIdentifier MockDeviceIdentifier) {
				gomock.InOrder(
					mockMounter.EXPECT().PathExists(gomock.Eq(devicePath)).Return(false, nil),

					mockDeviceIdentifier.EXPECT().Lstat(gomock.Eq(nvmeName)).Return(symlinkFileInfo, nil),
					mockDeviceIdentifier.EXPECT().EvalSymlinks(gomock.Eq(symlinkFileInfo.Name())).Return(nvmeDevicePath, nil),
				)
			},
			expectDevicePath: nvmeDevicePath,
		},
		{
			name:       "00: device path doesn't exist and nvme device path doesn't exist",
			devicePath: devicePath,
			volumeID:   volumeID,
			partition:  "",
			expectMock: func(mockMounter MockMounter, mockDeviceIdentifier MockDeviceIdentifier) {
				gomock.InOrder(
					mockMounter.EXPECT().PathExists(gomock.Eq(devicePath)).Return(false, nil),

					mockDeviceIdentifier.EXPECT().Lstat(gomock.Eq(nvmeName)).Return(nil, os.ErrNotExist),
				)
			},
			expectError: errNoDevicePathFound(devicePath, volumeID).Error(),
		},
	}
	// The partition variant of each case should be the same except the partition
	// is expected to be appended to devicePath
	generatedTestCases := []testCase{}
	for _, tc := range testCases {
		tc.name += " (with partition)"
		tc.partition = "1"
		if tc.expectDevicePath == devicePath {
			tc.expectDevicePath += tc.partition
		} else if tc.expectDevicePath == nvmeDevicePath {
			tc.expectDevicePath += "p" + tc.partition
		}
		generatedTestCases = append(generatedTestCases, tc)
	}
	testCases = append(testCases, generatedTestCases...)
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockCtl := gomock.NewController(t)
			defer mockCtl.Finish()

			mockMounter := NewMockMounter(mockCtl)
			mockDeviceIdentifier := NewMockDeviceIdentifier(mockCtl)

			nodeDriver := nodeService{
				metadata:         &cloud.Metadata{},
				mounter:          mockMounter,
				deviceIdentifier: mockDeviceIdentifier,
				inFlight:         internal.NewInFlight(),
				driverOptions:    &DriverOptions{},
			}

			if tc.expectMock != nil {
				tc.expectMock(*mockMounter, *mockDeviceIdentifier)
			}

			devicePath, err := nodeDriver.findDevicePath(tc.devicePath, tc.volumeID, tc.partition)
			if tc.expectError != "" {
				assert.EqualError(t, err, tc.expectError)
			} else {
				assert.Equal(t, tc.expectDevicePath, devicePath)
				assert.NoError(t, err)
			}
		})
	}
}

type fakeFileInfo struct {
	name string
	mode os.FileMode
}

func (fi *fakeFileInfo) Name() string {
	return fi.name
}

func (fi *fakeFileInfo) Size() int64 {
	return 0
}

func (fi *fakeFileInfo) Mode() os.FileMode {
	return fi.mode
}

func (fi *fakeFileInfo) ModTime() time.Time {
	return time.Now()
}

func (fi *fakeFileInfo) IsDir() bool {
	return false
}

func (fi *fakeFileInfo) Sys() interface{} {
	return nil
}
