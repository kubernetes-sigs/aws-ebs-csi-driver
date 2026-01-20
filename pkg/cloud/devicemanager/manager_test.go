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
	t.Helper()
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if d.IsAlreadyAssigned != assigned {
		t.Fatalf("Expected IsAlreadyAssigned to be %v, got %v", assigned, d.IsAlreadyAssigned)
	}
}

func TestNewDeviceWithCardIndex(t *testing.T) {
	testCases := []struct {
		name              string
		instance          *types.Instance
		volumeID          string
		expectedCardIndex *int32
		isAlreadyAssigned bool
	}{
		{
			name: "new volume on instance without cards",
			instance: &types.Instance{
				InstanceId:   aws.String("i-1234567890abcdef0"),
				InstanceType: "m5.large", // Instance type with 1 card
				BlockDeviceMappings: []types.InstanceBlockDeviceMapping{
					{
						DeviceName: aws.String("/dev/xvda"),
						Ebs: &types.EbsInstanceBlockDevice{
							VolumeId: aws.String("vol-1"),
						},
					},
				},
			},
			volumeID:          "vol-2",
			expectedCardIndex: nil,
			isAlreadyAssigned: false,
		},

		{
			name: "new volume on instance with cards",
			instance: &types.Instance{
				InstanceId:   aws.String("i-1234567890abcdef0"),
				InstanceType: "r8gb.48xlarge", // Instance type with 2 cards
				BlockDeviceMappings: []types.InstanceBlockDeviceMapping{
					{
						DeviceName: aws.String("/dev/xvda"),
						Ebs: &types.EbsInstanceBlockDevice{
							VolumeId:     aws.String("vol-1"),
							EbsCardIndex: aws.Int32(0),
						},
					},
				},
			},
			volumeID:          "vol-2",
			expectedCardIndex: aws.Int32(1), // Should pick card 1 (has fewer volumes)
			isAlreadyAssigned: false,
		},

		{
			name: "existing volume with card index",
			instance: &types.Instance{
				InstanceId:   aws.String("i-1234567890abcdef0"),
				InstanceType: "r8gb.48xlarge", // Instance type with 2 cards
				BlockDeviceMappings: []types.InstanceBlockDeviceMapping{
					{
						DeviceName: aws.String("/dev/xvda"),
						Ebs: &types.EbsInstanceBlockDevice{
							VolumeId:     aws.String("vol-1"),
							EbsCardIndex: aws.Int32(1),
						},
					},
				},
			},
			volumeID:          "vol-1",
			expectedCardIndex: aws.Int32(1),
			isAlreadyAssigned: true,
		},

		{
			name: "new volume chooses card with fewer volumes",
			instance: &types.Instance{
				InstanceId:   aws.String("i-1234567890abcdef0"),
				InstanceType: "r8gb.48xlarge", // Instance type with 2 cards
				BlockDeviceMappings: []types.InstanceBlockDeviceMapping{
					{
						DeviceName: aws.String("/dev/xvda"),
						Ebs: &types.EbsInstanceBlockDevice{
							VolumeId:     aws.String("vol-1"),
							EbsCardIndex: aws.Int32(0),
						},
					},
					{
						DeviceName: aws.String("/dev/xvdb"),
						Ebs: &types.EbsInstanceBlockDevice{
							VolumeId:     aws.String("vol-2"),
							EbsCardIndex: aws.Int32(0),
						},
					},
					{
						DeviceName: aws.String("/dev/xvdc"),
						Ebs: &types.EbsInstanceBlockDevice{
							VolumeId:     aws.String("vol-3"),
							EbsCardIndex: aws.Int32(1),
						},
					},
				},
			},
			volumeID:          "vol-4",
			expectedCardIndex: aws.Int32(1), // Should pick card 1 (has fewer volumes)
			isAlreadyAssigned: false,
		},
		{
			name: "new volume on multi-card instance with no existing card indexes",
			instance: &types.Instance{
				InstanceId:   aws.String("i-1234567890abcdef0"),
				InstanceType: "r8gb.48xlarge", // Instance type with 2 cards
				BlockDeviceMappings: []types.InstanceBlockDeviceMapping{
					{
						DeviceName: aws.String("/dev/xvda"),
						Ebs: &types.EbsInstanceBlockDevice{
							VolumeId: aws.String("vol-1"),
						},
					},
					{
						DeviceName: aws.String("/dev/xvdb"),
						Ebs: &types.EbsInstanceBlockDevice{
							VolumeId: aws.String("vol-2"),
						},
					},
				},
			},
			volumeID:          "vol-3",
			expectedCardIndex: aws.Int32(0), // Should pick card 0 (both have 0 volumes, picks lowest index)
			isAlreadyAssigned: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			dm := NewDeviceManager()
			device, err := dm.NewDevice(tc.instance, tc.volumeID, new(sync.Map))
			if err != nil {
				t.Fatalf("Expected no error, got %v", err)
			}

			if device.IsAlreadyAssigned != tc.isAlreadyAssigned {
				t.Fatalf("Expected IsAlreadyAssigned to be %v, got %v", tc.isAlreadyAssigned, device.IsAlreadyAssigned)
			}

			if tc.expectedCardIndex == nil {
				if device.CardIndex != nil {
					t.Fatalf("Expected nil card index, got %v", *device.CardIndex)
				}
			} else {
				if device.CardIndex == nil {
					t.Fatalf("Expected card index %v, got nil", *tc.expectedCardIndex)
				}
				if *device.CardIndex != *tc.expectedCardIndex {
					t.Fatalf("Expected card index %v, got %v", *tc.expectedCardIndex, *device.CardIndex)
				}
			}
		})
	}
}

func TestGetDeviceWithCardIndex(t *testing.T) {
	testCases := []struct {
		name              string
		instance          *types.Instance
		volumeID          string
		expectedCardIndex *int32
		isAlreadyAssigned bool
	}{
		{
			name: "existing volume with card index",
			instance: &types.Instance{
				InstanceId: aws.String("i-1234567890abcdef0"),
				BlockDeviceMappings: []types.InstanceBlockDeviceMapping{
					{
						DeviceName: aws.String("/dev/xvda"),
						Ebs: &types.EbsInstanceBlockDevice{
							VolumeId:     aws.String("vol-1"),
							EbsCardIndex: aws.Int32(2),
						},
					},
				},
			},
			volumeID:          "vol-1",
			expectedCardIndex: aws.Int32(2),
			isAlreadyAssigned: true,
		},
		{
			name: "existing volume without card index",
			instance: &types.Instance{
				InstanceId: aws.String("i-1234567890abcdef0"),
				BlockDeviceMappings: []types.InstanceBlockDeviceMapping{
					{
						DeviceName: aws.String("/dev/xvda"),
						Ebs: &types.EbsInstanceBlockDevice{
							VolumeId: aws.String("vol-1"),
						},
					},
				},
			},
			volumeID:          "vol-1",
			expectedCardIndex: nil,
			isAlreadyAssigned: true,
		},
		{
			name: "non-existing volume",
			instance: &types.Instance{
				InstanceId: aws.String("i-1234567890abcdef0"),
				BlockDeviceMappings: []types.InstanceBlockDeviceMapping{
					{
						DeviceName: aws.String("/dev/xvda"),
						Ebs: &types.EbsInstanceBlockDevice{
							VolumeId:     aws.String("vol-1"),
							EbsCardIndex: aws.Int32(0),
						},
					},
				},
			},
			volumeID:          "vol-2",
			expectedCardIndex: nil,
			isAlreadyAssigned: false,
		},
	}

	dm := NewDeviceManager()
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			device, err := dm.GetDevice(tc.instance, tc.volumeID)
			if err != nil {
				t.Fatalf("Expected no error, got %v", err)
			}

			if device.IsAlreadyAssigned != tc.isAlreadyAssigned {
				t.Fatalf("Expected IsAlreadyAssigned to be %v, got %v", tc.isAlreadyAssigned, device.IsAlreadyAssigned)
			}

			if tc.expectedCardIndex == nil {
				if device.CardIndex != nil {
					t.Fatalf("Expected nil card index, got %v", *device.CardIndex)
				}
			} else {
				if device.CardIndex == nil {
					t.Fatalf("Expected card index %v, got nil", *tc.expectedCardIndex)
				}
				if *device.CardIndex != *tc.expectedCardIndex {
					t.Fatalf("Expected card index %v, got %v", *tc.expectedCardIndex, *device.CardIndex)
				}
			}
		})
	}
}

func TestGetNextCardIndex(t *testing.T) {
	testCases := []struct {
		name              string
		instanceType      string
		cardCounts        map[int32]int
		expectedCardIndex *int32
	}{
		{
			name:              "single card instance returns nil",
			instanceType:      "m5.large",
			cardCounts:        map[int32]int{},
			expectedCardIndex: nil,
		},
		{
			name:              "multi-card instance with empty counts picks card 0",
			instanceType:      "r8gb.48xlarge",
			cardCounts:        map[int32]int{},
			expectedCardIndex: aws.Int32(0),
		},
		{
			name:         "multi-card instance picks card with fewer volumes",
			instanceType: "r8gb.48xlarge",
			cardCounts: map[int32]int{
				0: 3,
				1: 1,
			},
			expectedCardIndex: aws.Int32(1),
		},
		{
			name:         "multi-card instance with equal counts picks lowest index",
			instanceType: "r8gb.48xlarge",
			cardCounts: map[int32]int{
				0: 2,
				1: 2,
			},
			expectedCardIndex: aws.Int32(0),
		},
		{
			name:         "multi-card instance with only one card populated",
			instanceType: "r8gb.48xlarge",
			cardCounts: map[int32]int{
				0: 5,
			},
			expectedCardIndex: aws.Int32(1),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := getNextCardIndex(tc.instanceType, tc.cardCounts)

			if tc.expectedCardIndex == nil {
				if result != nil {
					t.Fatalf("Expected nil card index, got %v", *result)
				}
			} else {
				if result == nil {
					t.Fatalf("Expected card index %v, got nil", *tc.expectedCardIndex)
				}
				if *result != *tc.expectedCardIndex {
					t.Fatalf("Expected card index %v, got %v", *tc.expectedCardIndex, *result)
				}
			}
		})
	}
}

func TestGetCardCounts(t *testing.T) {
	testCases := []struct {
		name           string
		instance       *types.Instance
		inflightSetup  func(dm *deviceManager)
		expectedCounts map[int32]int
	}{
		{
			name: "counts from block device mappings only",
			instance: &types.Instance{
				InstanceId:   aws.String("i-1234567890abcdef0"),
				InstanceType: "r8gb.48xlarge",
				BlockDeviceMappings: []types.InstanceBlockDeviceMapping{
					{
						DeviceName: aws.String("/dev/xvda"),
						Ebs: &types.EbsInstanceBlockDevice{
							VolumeId:     aws.String("vol-1"),
							EbsCardIndex: aws.Int32(0),
						},
					},
					{
						DeviceName: aws.String("/dev/xvdb"),
						Ebs: &types.EbsInstanceBlockDevice{
							VolumeId:     aws.String("vol-2"),
							EbsCardIndex: aws.Int32(1),
						},
					},
				},
			},
			inflightSetup:  nil,
			expectedCounts: map[int32]int{0: 1, 1: 1},
		},
		{
			name: "counts from inflight only",
			instance: &types.Instance{
				InstanceId:          aws.String("i-1234567890abcdef0"),
				InstanceType:        "r8gb.48xlarge",
				BlockDeviceMappings: []types.InstanceBlockDeviceMapping{},
			},
			inflightSetup: func(dm *deviceManager) {
				dm.inFlight.Add("i-1234567890abcdef0", "vol-1", "/dev/xvda", aws.Int32(0))
				dm.inFlight.Add("i-1234567890abcdef0", "vol-2", "/dev/xvdb", aws.Int32(1))
			},
			expectedCounts: map[int32]int{0: 1, 1: 1},
		},
		{
			name: "combined counts without double counting",
			instance: &types.Instance{
				InstanceId:   aws.String("i-1234567890abcdef0"),
				InstanceType: "r8gb.48xlarge",
				BlockDeviceMappings: []types.InstanceBlockDeviceMapping{
					{
						DeviceName: aws.String("/dev/xvda"),
						Ebs: &types.EbsInstanceBlockDevice{
							VolumeId:     aws.String("vol-1"),
							EbsCardIndex: aws.Int32(0),
						},
					},
				},
			},
			inflightSetup: func(dm *deviceManager) {
				// vol-1 is already in block device mappings, should not be double counted
				dm.inFlight.Add("i-1234567890abcdef0", "vol-1", "/dev/xvda", aws.Int32(0))
				// vol-2 is only in inflight, should be counted
				dm.inFlight.Add("i-1234567890abcdef0", "vol-2", "/dev/xvdb", aws.Int32(1))
			},
			expectedCounts: map[int32]int{0: 1, 1: 1},
		},
		{
			name: "inflight without card index not counted",
			instance: &types.Instance{
				InstanceId:          aws.String("i-1234567890abcdef0"),
				InstanceType:        "r8gb.48xlarge",
				BlockDeviceMappings: []types.InstanceBlockDeviceMapping{},
			},
			inflightSetup: func(dm *deviceManager) {
				dm.inFlight.Add("i-1234567890abcdef0", "vol-1", "/dev/xvda", nil)
			},
			expectedCounts: map[int32]int{},
		},
		{
			name: "block device without card index not counted",
			instance: &types.Instance{
				InstanceId:   aws.String("i-1234567890abcdef0"),
				InstanceType: "r8gb.48xlarge",
				BlockDeviceMappings: []types.InstanceBlockDeviceMapping{
					{
						DeviceName: aws.String("/dev/xvda"),
						Ebs: &types.EbsInstanceBlockDevice{
							VolumeId: aws.String("vol-1"),
							// No EbsCardIndex
						},
					},
				},
			},
			inflightSetup:  nil,
			expectedCounts: map[int32]int{},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			dm := &deviceManager{
				nameAllocator: &nameAllocator{},
				inFlight:      make(inFlightAttaching),
			}

			if tc.inflightSetup != nil {
				tc.inflightSetup(dm)
			}

			result := dm.getCardCounts(tc.instance)

			if len(result) != len(tc.expectedCounts) {
				t.Fatalf("Expected %d card counts, got %d: %v", len(tc.expectedCounts), len(result), result)
			}

			for cardIndex, expectedCount := range tc.expectedCounts {
				if result[cardIndex] != expectedCount {
					t.Fatalf("Expected count %d for card %d, got %d", expectedCount, cardIndex, result[cardIndex])
				}
			}
		})
	}
}

func TestNewDeviceWithInflightCardIndex(t *testing.T) {
	testCases := []struct {
		name              string
		instance          *types.Instance
		volumeIDs         []string
		expectedCardIndex []*int32
	}{
		{
			name: "sequential volumes distributed across cards",
			instance: &types.Instance{
				InstanceId:          aws.String("i-1234567890abcdef0"),
				InstanceType:        "r8gb.48xlarge", // 2 cards
				BlockDeviceMappings: []types.InstanceBlockDeviceMapping{},
			},
			volumeIDs: []string{"vol-1", "vol-2", "vol-3", "vol-4"},
			// First goes to card 0, second to card 1, third to card 0, fourth to card 1
			expectedCardIndex: []*int32{aws.Int32(0), aws.Int32(1), aws.Int32(0), aws.Int32(1)},
		},
		{
			name: "volumes distributed considering existing block devices",
			instance: &types.Instance{
				InstanceId:   aws.String("i-1234567890abcdef0"),
				InstanceType: "r8gb.48xlarge", // 2 cards
				BlockDeviceMappings: []types.InstanceBlockDeviceMapping{
					{
						DeviceName: aws.String("/dev/xvda"),
						Ebs: &types.EbsInstanceBlockDevice{
							VolumeId:     aws.String("vol-existing"),
							EbsCardIndex: aws.Int32(0),
						},
					},
				},
			},
			volumeIDs: []string{"vol-1", "vol-2"},
			// Card 0 has 1 volume, so first new volume goes to card 1, then card 0
			expectedCardIndex: []*int32{aws.Int32(1), aws.Int32(0)},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			dm := NewDeviceManager()

			for i, volumeID := range tc.volumeIDs {
				device, err := dm.NewDevice(tc.instance, volumeID, new(sync.Map))
				if err != nil {
					t.Fatalf("Expected no error for volume %s, got %v", volumeID, err)
				}

				expectedCard := tc.expectedCardIndex[i]
				if expectedCard == nil {
					if device.CardIndex != nil {
						t.Fatalf("Volume %s: expected nil card index, got %v", volumeID, *device.CardIndex)
					}
				} else {
					if device.CardIndex == nil {
						t.Fatalf("Volume %s: expected card index %v, got nil", volumeID, *expectedCard)
					}
					if *device.CardIndex != *expectedCard {
						t.Fatalf("Volume %s: expected card index %v, got %v", volumeID, *expectedCard, *device.CardIndex)
					}
				}
			}
		})
	}
}

func TestInFlightAttachingWithCardIndex(t *testing.T) {
	inFlight := make(inFlightAttaching)

	const testVol1 = "vol-1"

	// Test Add with card index
	inFlight.Add("node-1", testVol1, "/dev/xvda", aws.Int32(0))
	inFlight.Add("node-1", "vol-2", "/dev/xvdb", aws.Int32(1))
	inFlight.Add("node-1", "vol-3", "/dev/xvdc", nil)

	// Test GetEntries - now keyed by volumeID
	entries := inFlight.GetEntries("node-1")
	if len(entries) != 3 {
		t.Fatalf("Expected 3 entries, got %d", len(entries))
	}

	// Verify entry with card index (keyed by volumeID)
	entry1 := entries[testVol1]
	if entry1.DeviceName != "/dev/xvda" {
		t.Fatalf("Expected device name /dev/xvda, got %s", entry1.DeviceName)
	}
	if entry1.CardIndex == nil || *entry1.CardIndex != 0 {
		t.Fatalf("Expected card index 0, got %v", entry1.CardIndex)
	}

	// Verify entry without card index
	entry3 := entries["vol-3"]
	if entry3.DeviceName != "/dev/xvdc" {
		t.Fatalf("Expected device name /dev/xvdc, got %s", entry3.DeviceName)
	}
	if entry3.CardIndex != nil {
		t.Fatalf("Expected nil card index, got %v", *entry3.CardIndex)
	}

	// Test GetNames still works (returns deviceName -> volumeID)
	names := inFlight.GetNames("node-1")
	if len(names) != 3 {
		t.Fatalf("Expected 3 names, got %d", len(names))
	}
	if names["/dev/xvda"] != testVol1 {
		t.Fatalf("Expected %s, got %s", testVol1, names["/dev/xvda"])
	}

	// Test GetEntry
	entry, exists := inFlight.GetEntry("node-1", testVol1)
	if !exists {
		t.Fatalf("Expected entry to exist for %s", testVol1)
	}
	if entry.DeviceName != "/dev/xvda" {
		t.Fatalf("Expected /dev/xvda, got %s", entry.DeviceName)
	}

	// Test Del (now by volumeID)
	inFlight.Del("node-1", testVol1)
	entries = inFlight.GetEntries("node-1")
	if len(entries) != 2 {
		t.Fatalf("Expected 2 entries after delete, got %d", len(entries))
	}
}
