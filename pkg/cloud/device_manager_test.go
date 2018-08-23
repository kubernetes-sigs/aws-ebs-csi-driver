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

package cloud

import (
	"testing"

	"github.com/aws/aws-sdk-go/service/ec2"
)

func newFakeInstance(deviceName, volumeID string) *ec2.Instance {
	return &ec2.Instance{
		BlockDeviceMappings: []*ec2.InstanceBlockDeviceMapping{
			//&ec2.InstanceBlockDeviceMapping{
			//DeviceName: aws.String(deviceName),
			//Ebs:        &ec2.EbsInstanceBlockDevice{VolumeId: aws.String(volumeID)},
			//},
		},
	}
}

func TestGetDevice(t *testing.T) {
	testCases := []struct {
		name               string
		instanceDeviceName string
		instanceVolumeID   string
		volumeID           string
	}{
		{
			name:               "should get a device",
			instanceDeviceName: "instance-test-1234",
			instanceVolumeID:   "instance-vol-test-1234",
			volumeID:           "vol-test-1234",
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			dm := NewDeviceManager()

			device, err := dm.NewDevice(nil, tc.volumeID)
			if err == nil {
				t.Fatalf("Expected error if nil instance is passed in, got nil error")
			}

			if device != nil {
				t.Fatalf("Expected nil device, got %v", device)
			}

			fakeInstance := newFakeInstance(tc.instanceDeviceName, tc.instanceVolumeID)

			device, err = dm.NewDevice(fakeInstance, tc.volumeID)
			if err != nil {
				t.Fatalf("Expected no error, got %v", err)
			}

			if device == nil {
				t.Fatalf("Expected valid device, got nil")
			}

			if device.Path == "" {
				t.Fatalf("Expected valid path, got empty string")
			}

			// TODO testar NewDevice antes e ver se ele retorna o mesmo path

			device2, err2 := dm.GetDevice(fakeInstance, tc.volumeID)
			if err2 != nil {
				t.Fatalf("Expected no error, got %v", err2)
			}

			if device2 == nil {
				t.Fatalf("Expected valid device, got nil")
			}

			if device.Path != device2.Path {
				t.Fatalf("Expected same device path, got %q and %q", device.Path, device2.Path)
			}

			//device.Release(false)
			//device2.Release(false)

			//t.Fatalf("----- %v", device2)

			for i := 0; i < 10; i++ {
				d, _ := dm.NewDevice(fakeInstance, tc.volumeID)

				if d.Path == device.Path {
					t.Fatalf("Expected different device paths, got equals %q", device.Path)
				}

				d.Release(false)

			}

			device3, _ := dm.NewDevice(fakeInstance, tc.volumeID)

			//t.Fatalf("Expected device path, got %q and %q", device.Path, device3.Path)
			if device.Path != device3.Path {
				t.Fatalf("Expected same device path, got %q and %q", device.Path, device3.Path)
			}

			device3.Taint()
			device3.Release(false)

			device4, _ := dm.GetDevice(fakeInstance, tc.volumeID)
			if device4.Path == "" {
				t.Fatalf("Expected valid device path, got nothing")
			}

			if device3.Path != device4.Path {
				t.Fatalf("Expected same device path, got %q and %q", device3.Path, device4.Path)
			}

			device3.Release(true)

			device5, _ := dm.GetDevice(fakeInstance, tc.volumeID)
			if device5.Path != "" {
				t.Fatalf("Expected empty path, got %v", device5.Path)
			}
			//if err3 != nil {
			//t.Fatalf("Expected no error, got %v", err3)
			//}

			//if device2.Path == device3.Path {
			//t.Fatalf("----- %s", device2.Path)
			//}

			// item :=

			//device2, _ := dm.NewDevice(fakeInstance, tc.volumeID)
			// if device.Path == "" {
			//t.Fatalf("Expected valid device path, got empty string %v %v", device.Path, device2.Path)
			// }

		})
	}
}
