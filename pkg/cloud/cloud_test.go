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
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/golang/mock/gomock"
	dm "github.com/kubernetes-sigs/aws-ebs-csi-driver/pkg/cloud/devicemanager"
	"github.com/kubernetes-sigs/aws-ebs-csi-driver/pkg/cloud/mocks"
	"github.com/kubernetes-sigs/aws-ebs-csi-driver/pkg/util"
)

func TestCreateDisk(t *testing.T) {
	testCases := []struct {
		name        string
		volumeName  string
		diskOptions *DiskOptions
		expDisk     *Disk
		expErr      error
	}{
		{
			name:       "success: normal",
			volumeName: "vol-test-name",
			diskOptions: &DiskOptions{
				CapacityBytes:    util.GiBToBytes(1),
				Tags:             map[string]string{VolumeNameTagKey: "vol-test"},
				AvailabilityZone: "",
			},
			expDisk: &Disk{
				VolumeID:    "vol-test",
				CapacityGiB: 1,
			},
			expErr: nil,
		},
		{
			name:       "success: normal with provided zone",
			volumeName: "vol-test-name",
			diskOptions: &DiskOptions{
				CapacityBytes:    util.GiBToBytes(1),
				Tags:             map[string]string{VolumeNameTagKey: "vol-test"},
				AvailabilityZone: "us-west-2",
			},
			expDisk: &Disk{
				VolumeID:    "vol-test",
				CapacityGiB: 1,
			},
			expErr: nil,
		},
		{
			name:       "fail: CreateVolume returned an error",
			volumeName: "vol-test-name-error",
			diskOptions: &DiskOptions{
				CapacityBytes:    util.GiBToBytes(1),
				Tags:             map[string]string{VolumeNameTagKey: "vol-test"},
				AvailabilityZone: "",
			},
			expErr: fmt.Errorf("CreateVolume generic error"),
		},
		{
			name:       "success: normal with encrypted volume",
			volumeName: "vol-test-name",
			diskOptions: &DiskOptions{
				CapacityBytes:    util.GiBToBytes(1),
				Tags:             map[string]string{VolumeNameTagKey: "vol-test"},
				AvailabilityZone: "us-west-2",
				Encrypted:        true,
				KmsKeyID:         "arn:aws:kms:us-east-1:012345678910:key/abcd1234-a123-456a-a12b-a123b4cd56ef",
			},
			expDisk: &Disk{
				VolumeID:    "vol-test",
				CapacityGiB: 1,
			},
			expErr: nil,
		},
	}

	for _, tc := range testCases {
		t.Logf("Test case: %s", tc.name)
		mockCtrl := gomock.NewController(t)
		mockEC2 := mocks.NewMockEC2(mockCtrl)
		c := newCloud(mockEC2)

		vol := &ec2.Volume{}
		if tc.expErr == nil {
			vol = &ec2.Volume{
				VolumeId: aws.String(tc.diskOptions.Tags[VolumeNameTagKey]),
				Size:     aws.Int64(util.BytesToGiB(tc.diskOptions.CapacityBytes)),
				State:    aws.String("available"),
			}
		}

		ctx := context.Background()
		mockEC2.EXPECT().CreateVolumeWithContext(gomock.Eq(ctx), gomock.Any()).Return(vol, tc.expErr)

		if tc.diskOptions.Encrypted {
			mockEC2.EXPECT().DescribeVolumesWithContext(gomock.Eq(ctx), gomock.Any()).Return(&ec2.DescribeVolumesOutput{Volumes: []*ec2.Volume{vol}}, nil)
		}

		disk, err := c.CreateDisk(ctx, tc.volumeName, tc.diskOptions)
		if err != nil {
			if tc.expErr == nil {
				t.Fatalf("CreateDisk() failed: expected no error, got: %v", err)
			}
		} else {
			if tc.expErr != nil {
				t.Fatal("CreateDisk() failed: expected error, got nothing")
			} else {
				if tc.expDisk.CapacityGiB != disk.CapacityGiB {
					t.Fatalf("CreateDisk() failed: expected capacity %d, got %v", tc.expDisk.CapacityGiB, disk.CapacityGiB)
				}

				if tc.expDisk.VolumeID != disk.VolumeID {
					t.Fatalf("CreateDisk() failed: expected capacity %d, got %v", tc.expDisk.CapacityGiB, disk.CapacityGiB)
				}
			}
		}

		mockCtrl.Finish()
	}
}

func TestDeleteDisk(t *testing.T) {
	testCases := []struct {
		name     string
		volumeID string
		expResp  bool
		expErr   error
	}{
		{
			name:     "success: normal",
			volumeID: "vol-test-1234",
			expResp:  true,
			expErr:   nil,
		},
		{
			name:     "fail: DeleteVolume returned generic error",
			volumeID: "vol-test-1234",
			expResp:  false,
			expErr:   fmt.Errorf("DeleteVolume generic error"),
		},
		{
			name:     "fail: DeleteVolume returned not found error",
			volumeID: "vol-test-1234",
			expResp:  false,
			expErr:   awserr.New("InvalidVolume.NotFound", "", nil),
		},
	}

	for _, tc := range testCases {
		t.Logf("Test case: %s", tc.name)
		mockCtrl := gomock.NewController(t)
		mockEC2 := mocks.NewMockEC2(mockCtrl)
		c := newCloud(mockEC2)

		ctx := context.Background()
		mockEC2.EXPECT().DeleteVolumeWithContext(gomock.Eq(ctx), gomock.Any()).Return(&ec2.DeleteVolumeOutput{}, tc.expErr)

		ok, err := c.DeleteDisk(ctx, tc.volumeID)
		if err != nil && tc.expErr == nil {
			t.Fatalf("DeleteDisk() failed: expected no error, got: %v", err)
		}

		if err == nil && tc.expErr != nil {
			t.Fatal("DeleteDisk() failed: expected error, got nothing")
		}

		if tc.expResp != ok {
			t.Fatalf("DeleteDisk() failed: expected return %v, got %v", tc.expResp, ok)
		}

		mockCtrl.Finish()
	}
}

func TestAttachDisk(t *testing.T) {
	testCases := []struct {
		name     string
		volumeID string
		nodeID   string
		expErr   error
	}{
		{
			name:     "success: normal",
			volumeID: "vol-test-1234",
			nodeID:   "node-1234",
			expErr:   nil,
		},
		{
			name:     "fail: AttachVolume returned generic error",
			volumeID: "vol-test-1234",
			nodeID:   "node-1234",
			expErr:   fmt.Errorf(""),
		},
	}

	for _, tc := range testCases {
		t.Logf("Test case: %s", tc.name)
		mockCtrl := gomock.NewController(t)
		mockEC2 := mocks.NewMockEC2(mockCtrl)
		c := newCloud(mockEC2)

		vol := &ec2.Volume{
			VolumeId:    aws.String(tc.volumeID),
			Attachments: []*ec2.VolumeAttachment{{State: aws.String("attached")}},
		}

		ctx := context.Background()
		mockEC2.EXPECT().DescribeVolumesWithContext(gomock.Eq(ctx), gomock.Any()).Return(&ec2.DescribeVolumesOutput{Volumes: []*ec2.Volume{vol}}, nil).AnyTimes()
		mockEC2.EXPECT().DescribeInstancesWithContext(gomock.Eq(ctx), gomock.Any()).Return(newDescribeInstancesOutput(tc.nodeID), nil)
		mockEC2.EXPECT().AttachVolumeWithContext(gomock.Eq(ctx), gomock.Any()).Return(&ec2.VolumeAttachment{}, tc.expErr)

		devicePath, err := c.AttachDisk(ctx, tc.volumeID, tc.nodeID)
		if err != nil {
			if tc.expErr == nil {
				t.Fatalf("AttachDisk() failed: expected no error, got: %v", err)
			}
		} else {
			if tc.expErr != nil {
				t.Fatal("AttachDisk() failed: expected error, got nothing")
			}
			if !strings.HasPrefix(devicePath, "/dev/") {
				t.Fatal("AttachDisk() failed: expected valid device path, got emtpy string")
			}
		}

		mockCtrl.Finish()
	}
}

func TestDetachDisk(t *testing.T) {
	testCases := []struct {
		name     string
		volumeID string
		nodeID   string
		expErr   error
	}{
		{
			name:     "success: normal",
			volumeID: "vol-test-1234",
			nodeID:   "node-1234",
			expErr:   nil,
		},
		{
			name:     "fail: DetachVolume returned generic error",
			volumeID: "vol-test-1234",
			nodeID:   "node-1234",
			expErr:   fmt.Errorf("DetachVolume generic error"),
		},
	}

	for _, tc := range testCases {
		t.Logf("Test case: %s", tc.name)
		mockCtrl := gomock.NewController(t)
		mockEC2 := mocks.NewMockEC2(mockCtrl)
		c := newCloud(mockEC2)

		vol := &ec2.Volume{
			VolumeId:    aws.String(tc.volumeID),
			Attachments: nil,
		}

		ctx := context.Background()
		mockEC2.EXPECT().DescribeVolumesWithContext(gomock.Eq(ctx), gomock.Any()).Return(&ec2.DescribeVolumesOutput{Volumes: []*ec2.Volume{vol}}, nil).AnyTimes()
		mockEC2.EXPECT().DescribeInstancesWithContext(gomock.Eq(ctx), gomock.Any()).Return(newDescribeInstancesOutput(tc.nodeID), nil)
		mockEC2.EXPECT().DetachVolumeWithContext(gomock.Eq(ctx), gomock.Any()).Return(&ec2.VolumeAttachment{}, tc.expErr)

		err := c.DetachDisk(ctx, tc.volumeID, tc.nodeID)
		if err != nil {
			if tc.expErr == nil {
				t.Fatalf("DetachDisk() failed: expected no error, got: %v", err)
			}
		} else {
			if tc.expErr != nil {
				t.Fatal("DetachDisk() failed: expected error, got nothing")
			}
		}

		mockCtrl.Finish()
	}
}

func TestGetDiskByName(t *testing.T) {
	testCases := []struct {
		name           string
		volumeName     string
		volumeCapacity int64
		expErr         error
	}{
		{
			name:           "success: normal",
			volumeName:     "vol-test-1234",
			volumeCapacity: util.GiBToBytes(1),
			expErr:         nil,
		},
		{
			name:           "fail: DescribeVolumes returned generic error",
			volumeName:     "vol-test-1234",
			volumeCapacity: util.GiBToBytes(1),
			expErr:         fmt.Errorf("DescribeVolumes generic error"),
		},
	}

	for _, tc := range testCases {
		t.Logf("Test case: %s", tc.name)
		mockCtrl := gomock.NewController(t)
		mockEC2 := mocks.NewMockEC2(mockCtrl)
		c := newCloud(mockEC2)

		vol := &ec2.Volume{
			VolumeId: aws.String(tc.volumeName),
			Size:     aws.Int64(util.BytesToGiB(tc.volumeCapacity)),
		}

		ctx := context.Background()
		mockEC2.EXPECT().DescribeVolumesWithContext(gomock.Eq(ctx), gomock.Any()).Return(&ec2.DescribeVolumesOutput{Volumes: []*ec2.Volume{vol}}, tc.expErr)

		disk, err := c.GetDiskByName(ctx, tc.volumeName, tc.volumeCapacity)
		if err != nil {
			if tc.expErr == nil {
				t.Fatalf("GetDiskByName() failed: expected no error, got: %v", err)
			}
		} else {
			if tc.expErr != nil {
				t.Fatal("GetDiskByName() failed: expected error, got nothing")
			}
			if disk.CapacityGiB != util.BytesToGiB(tc.volumeCapacity) {
				t.Fatalf("GetDiskByName() failed: expected capacity %d, got %d", util.BytesToGiB(tc.volumeCapacity), disk.CapacityGiB)
			}
		}

		mockCtrl.Finish()
	}
}

func TestGetDiskByID(t *testing.T) {
	testCases := []struct {
		name     string
		volumeID string
		expErr   error
	}{
		{
			name:     "success: normal",
			volumeID: "vol-test-1234",
			expErr:   nil,
		},
		{
			name:     "fail: DescribeVolumes returned generic error",
			volumeID: "vol-test-1234",
			expErr:   fmt.Errorf("DescribeVolumes generic error"),
		},
	}

	for _, tc := range testCases {
		t.Logf("Test case: %s", tc.name)
		mockCtrl := gomock.NewController(t)
		mockEC2 := mocks.NewMockEC2(mockCtrl)
		c := newCloud(mockEC2)

		ctx := context.Background()
		mockEC2.EXPECT().DescribeVolumesWithContext(gomock.Eq(ctx), gomock.Any()).Return(
			&ec2.DescribeVolumesOutput{
				Volumes: []*ec2.Volume{
					{VolumeId: aws.String(tc.volumeID)},
				},
			},
			tc.expErr,
		)

		disk, err := c.GetDiskByID(ctx, tc.volumeID)
		if err != nil {
			if tc.expErr == nil {
				t.Fatalf("GetDisk() failed: expected no error, got: %v", err)
			}
		} else {
			if tc.expErr != nil {
				t.Fatal("GetDisk() failed: expected error, got nothing")
			}
			if disk.VolumeID != tc.volumeID {
				t.Fatalf("GetDisk() failed: expected ID %q, got %q", tc.volumeID, disk.VolumeID)
			}
		}

		mockCtrl.Finish()
	}
}

func newCloud(mockEC2 EC2) Cloud {
	return &cloud{
		metadata: &metadata{
			instanceID:       "test-instance",
			region:           "test-region",
			availabilityZone: "test-az",
		},
		dm:  dm.NewDeviceManager(),
		ec2: mockEC2,
	}
}

func newDescribeInstancesOutput(nodeID string) *ec2.DescribeInstancesOutput {
	return &ec2.DescribeInstancesOutput{
		Reservations: []*ec2.Reservation{{
			Instances: []*ec2.Instance{
				{InstanceId: aws.String(nodeID)},
			},
		}},
	}
}
