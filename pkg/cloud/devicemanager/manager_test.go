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

package devicemanager

import (
	"sync"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
)

func TestNewDevice(t *testing.T) {
	testCases := []struct {
		name               string
		instanceID         string
		existingDevicePath string
		existingVolumeID   string
		volumeID           string
	}{
		{
			name:               "success: normal",
			instanceID:         "instance-1",
			existingDevicePath: "/dev/xvdbc",
			existingVolumeID:   "vol-1",
			volumeID:           "vol-2",
		},
		{
			name:               "success: parallel same instance but different volumes",
			instanceID:         "instance-1",
			existingDevicePath: "/dev/xvdbc",
			existingVolumeID:   "vol-1",
			volumeID:           "vol-4",
		},
		{
			name:               "success: parallel different instances but same volume",
			instanceID:         "instance-2",
			existingDevicePath: "/dev/xvdbc",
			existingVolumeID:   "vol-1",
			volumeID:           "vol-4",
		},
	}
	// Use a shared DeviceManager to make sure that there are no race conditions
	dm := NewDeviceManager()

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Should fail if instance is nil
			dev1, err := dm.NewDevice(nil, tc.volumeID, new(sync.Map))
			if err == nil {
				t.Fatalf("Expected error when nil instance is passed in, got nothing")
			}
			if dev1 != nil {
				t.Fatalf("Expected nil device, got %v", dev1)
			}

			fakeInstance := newFakeInstance(tc.instanceID, tc.existingVolumeID, tc.existingDevicePath)

			// Should create valid Device with valid path
			dev1, err = dm.NewDevice(fakeInstance, tc.volumeID, new(sync.Map))
			assertDevice(t, dev1, false, err)

			// Devices with same instance and volume should have same paths
			dev2, err := dm.NewDevice(fakeInstance, tc.volumeID, new(sync.Map))
			assertDevice(t, dev2, true /*IsAlreadyAssigned*/, err)
			if dev1.Path != dev2.Path {
				t.Fatalf("Expected equal paths, got %v and %v", dev1.Path, dev2.Path)
			}

			// Should create new Device with the same path after releasing
			dev2.Release(false)
			dev3, err := dm.NewDevice(fakeInstance, tc.volumeID, new(sync.Map))
			assertDevice(t, dev3, false, err)
			if dev3.Path != dev1.Path {
				t.Fatalf("Expected equal paths, got %v and %v", dev1.Path, dev3.Path)
			}
			dev3.Release(false)
		})
	}
}

func TestNewDeviceWithExistingDevice(t *testing.T) {
	testCases := []struct {
		name         string
		existingID   string
		existingPath string
		volumeID     string
		expectedPath string
	}{
		{
			name:         "success: different volumes",
			existingID:   "vol-1",
			existingPath: deviceNames[0],
			volumeID:     "vol-2",
			expectedPath: deviceNames[1],
		},
		{
			name:         "success: same volumes",
			existingID:   "vol-1",
			existingPath: "/dev/xvdcc",
			volumeID:     "vol-1",
			expectedPath: "/dev/xvdcc",
		},
		{
			name:         "success: same volumes with /dev/sdX path",
			existingID:   "vol-3",
			existingPath: "/dev/sdf",
			volumeID:     "vol-3",
			expectedPath: "/dev/sdf",
		},
		{
			name:         "success: same volumes with weird path",
			existingID:   "vol-42",
			existingPath: "/weird/path",
			volumeID:     "vol-42",
			expectedPath: "/weird/path",
		},
	}
	// Use a shared DeviceManager to make sure that there are no race conditions
	dm := NewDeviceManager()

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			fakeInstance := newFakeInstance("fake-instance", tc.existingID, tc.existingPath)

			dev, err := dm.NewDevice(fakeInstance, tc.volumeID, new(sync.Map))
			assertDevice(t, dev, tc.existingID == tc.volumeID, err)

			if dev.Path != tc.expectedPath {
				t.Fatalf("Expected path %v got %v", tc.expectedPath, dev.Path)
			}
		})
	}
}

func TestGetDevice(t *testing.T) {
	testCases := []struct {
		name               string
		instanceID         string
		existingDevicePath string
		existingVolumeID   string
		volumeID           string
	}{
		{
			name:               "success: normal",
			instanceID:         "instance-1",
			existingDevicePath: "/dev/xvdbc",
			existingVolumeID:   "vol-1",
			volumeID:           "vol-2",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			dm := NewDeviceManager()
			fakeInstance := newFakeInstance(tc.instanceID, tc.existingVolumeID, tc.existingDevicePath)

			// Should create valid Device with valid path
			dev1, err := dm.NewDevice(fakeInstance, tc.volumeID, new(sync.Map))
			assertDevice(t, dev1, false /*IsAlreadyAssigned*/, err)

			// Devices with same instance and volume should have same paths
			dev2, err := dm.GetDevice(fakeInstance, tc.volumeID)
			assertDevice(t, dev2, true /*IsAlreadyAssigned*/, err)
			if dev1.Path != dev2.Path {
				t.Fatalf("Expected equal paths, got %v and %v", dev1.Path, dev2.Path)
			}
		})
	}
}

func TestReleaseDevice(t *testing.T) {
	testCases := []struct {
		name               string
		instanceID         string
		existingDevicePath string
		existingVolumeID   string
		volumeID           string
	}{
		{
			name:               "success: normal",
			instanceID:         "instance-1",
			existingDevicePath: "/dev/xvdbc",
			existingVolumeID:   "vol-1",
			volumeID:           "vol-2",
		},
	}

	dm := NewDeviceManager()
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			fakeInstance := newFakeInstance(tc.instanceID, tc.existingVolumeID, tc.existingDevicePath)

			// Should get assigned Device after releasing tainted device
			dev, err := dm.NewDevice(fakeInstance, tc.volumeID, new(sync.Map))
			assertDevice(t, dev, false /*IsAlreadyAssigned*/, err)
			dev.Taint()
			dev.Release(false)
			dev2, err := dm.GetDevice(fakeInstance, tc.volumeID)
			assertDevice(t, dev2, true /*IsAlreadyAssigned*/, err)
			if dev.Path != dev2.Path {
				t.Fatalf("Expected device to be already assigned, got unassigned")
			}

			// Should release tainted device if force=true is passed in
			dev2.Release(true)
			dev3, err := dm.GetDevice(fakeInstance, tc.volumeID)
			assertDevice(t, dev3, false /*IsAlreadyAssigned*/, err)
		})
	}
}

func newFakeInstance(instanceID, volumeID, devicePath string) *types.Instance {
	return &types.Instance{
		InstanceId: aws.String(instanceID),
		BlockDeviceMappings: []types.InstanceBlockDeviceMapping{
			{
				DeviceName: aws.String(devicePath),
				Ebs: &types.EbsInstanceBlockDevice{
					VolumeId: aws.String(volumeID),
				},
			},
		},
	}
}

func assertDevice(t *testing.T, d *Device, assigned bool, err error) {
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if d == nil {
		t.Fatalf("Expected valid device, got nil")
	}

	if d.IsAlreadyAssigned != assigned {
		t.Fatalf("Expected IsAlreadyAssigned to be %v, got %v", assigned, d.IsAlreadyAssigned)
	}
}
