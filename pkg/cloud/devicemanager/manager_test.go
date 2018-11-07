/*
Copyright 2018 The Kubernetes Authors.

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
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
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
		tc := tc // capture tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			// Should fail if instance is nil
			dev1, err := dm.NewDevice(nil, tc.volumeID)
			if err == nil {
				t.Fatalf("Expected error when nil instance is passed in, got nothing")
			}
			if dev1 != nil {
				t.Fatalf("Expected nil device, got %v", dev1)
			}

			fakeInstance := newFakeInstance(tc.instanceID, tc.existingVolumeID, tc.existingDevicePath)

			// Should create valid Device with valid path
			dev1, err = dm.NewDevice(fakeInstance, tc.volumeID)
			assertDevice(t, dev1, false, err)

			// Devices with same instance and volume should have same paths
			dev2, err := dm.NewDevice(fakeInstance, tc.volumeID)
			assertDevice(t, dev2, true /*IsAlreadyAssigned*/, err)
			if dev1.Path != dev2.Path {
				t.Fatalf("Expected equal paths, got %v and %v", dev1.Path, dev2.Path)
			}

			// Should create new Device with a different path after releasing
			dev2.Release(false)
			dev3, err := dm.NewDevice(fakeInstance, tc.volumeID)
			assertDevice(t, dev3, false, err)
			if dev3.Path == dev1.Path {
				t.Fatalf("Expected equal paths, got %v and %v", dev1.Path, dev2.Path)
			}
			dev3.Release(false)
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
			dev1, err := dm.NewDevice(fakeInstance, tc.volumeID)
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
			dev, err := dm.NewDevice(fakeInstance, tc.volumeID)
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

func TestExaustDevices(t *testing.T) {
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
			existingDevicePath: "",
			existingVolumeID:   "",
			volumeID:           "vol-2",
		},
	}

	dm := NewDeviceManager()
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			fakeInstance := newFakeInstance(tc.instanceID, tc.existingVolumeID, tc.existingDevicePath)

			// Create one device and save it for later
			dev, err := dm.NewDevice(fakeInstance, tc.volumeID)
			assertDevice(t, dev, false /*IsAlreadyAssigned*/, err)
			dev.Release(true)

			// The maximum number of the ring is 52, so create enough devices
			// to circle back to the first device gotten, i.e., dev
			for i := 0; i < 51; i++ {
				d, err := dm.NewDevice(fakeInstance, tc.volumeID)
				assertDevice(t, d, false, err)
				// Make sure none of them have the same path as the first device created
				if d.Path == dev.Path {
					t.Fatalf("Expected different device paths, got equals %q", d.Path)
				}
				d.Release(true)
			}

			dev2, err := dm.NewDevice(fakeInstance, tc.volumeID)
			assertDevice(t, dev2, false /*IsAlreadyAssigned*/, err)

			//Should be equal to the first device created
			if dev2.Path != dev.Path {
				t.Fatalf("Expected %q, got %q", dev2.Path, dev.Path)
			}
		})
	}
}

func newFakeInstance(instanceID, volumeID, devicePath string) *ec2.Instance {
	return &ec2.Instance{
		InstanceId: aws.String(instanceID),
		BlockDeviceMappings: []*ec2.InstanceBlockDeviceMapping{
			{
				DeviceName: aws.String(devicePath),
				Ebs:        &ec2.EbsInstanceBlockDevice{VolumeId: aws.String(volumeID)},
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
