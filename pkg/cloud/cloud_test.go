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
	"errors"
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

const (
	defaultZone = "test-az"
	expZone     = "us-west-2b"
)

func TestCreateDisk(t *testing.T) {
	testCases := []struct {
		name     string
		testFunc func(t *testing.T)
	}{
		{
			name: "success: normal",
			testFunc: func(t *testing.T) {
				diskOptions := &DiskOptions{
					CapacityBytes: util.GiBToBytes(1),
					Tags:          map[string]string{VolumeNameTagKey: "vol-test"},
				}
				expDisk := &Disk{
					VolumeID:         "vol-test",
					CapacityGiB:      1,
					AvailabilityZone: defaultZone,
				}

				vol := &ec2.Volume{
					VolumeId:         aws.String(diskOptions.Tags[VolumeNameTagKey]),
					Size:             aws.Int64(util.BytesToGiB(diskOptions.CapacityBytes)),
					State:            aws.String("available"),
					AvailabilityZone: aws.String(diskOptions.AvailabilityZone),
				}

				mockCtrl := gomock.NewController(t)
				defer mockCtrl.Finish()
				mockEC2 := mocks.NewMockEC2(mockCtrl)
				c := newCloud(mockEC2)
				ctx := context.Background()
				mockEC2.EXPECT().CreateVolumeWithContext(gomock.Eq(ctx), gomock.Any()).Return(vol, nil)
				mockEC2.EXPECT().WaitUntilVolumeAvailableWithContext(gomock.Eq(ctx), gomock.Any()).Return(nil).AnyTimes()

				disk, err := c.CreateDisk(ctx, "vol-test-name", diskOptions)
				if err != nil {
					t.Fatalf("CreateDisk() failed: expected no error, got: %v", err)
				} else {
					if expDisk.CapacityGiB != disk.CapacityGiB {
						t.Fatalf("CreateDisk() failed: expected capacity %d, got %d", expDisk.CapacityGiB, disk.CapacityGiB)
					}
					if expDisk.VolumeID != disk.VolumeID {
						t.Fatalf("CreateDisk() failed: expected capacity %q, got %q", expDisk.VolumeID, disk.VolumeID)
					}
					if expDisk.AvailabilityZone != disk.AvailabilityZone {
						t.Fatalf("CreateDisk() failed: expected availability zone %q, got %q", expDisk.AvailabilityZone, disk.AvailabilityZone)
					}
				}
			},
		},
		{
			name: "success: normal with provided zone",
			testFunc: func(t *testing.T) {
				diskOptions := &DiskOptions{
					CapacityBytes:    util.GiBToBytes(1),
					Tags:             map[string]string{VolumeNameTagKey: "vol-test"},
					AvailabilityZone: expZone,
				}
				expDisk := &Disk{
					VolumeID:         "vol-test",
					CapacityGiB:      1,
					AvailabilityZone: expZone,
				}

				vol := &ec2.Volume{
					VolumeId:         aws.String(diskOptions.Tags[VolumeNameTagKey]),
					Size:             aws.Int64(util.BytesToGiB(diskOptions.CapacityBytes)),
					State:            aws.String("available"),
					AvailabilityZone: aws.String(expZone),
				}

				mockCtrl := gomock.NewController(t)
				defer mockCtrl.Finish()
				mockEC2 := mocks.NewMockEC2(mockCtrl)
				c := newCloud(mockEC2)
				ctx := context.Background()
				mockEC2.EXPECT().CreateVolumeWithContext(gomock.Eq(ctx), gomock.Any()).Return(vol, nil)
				mockEC2.EXPECT().WaitUntilVolumeAvailableWithContext(gomock.Eq(ctx), gomock.Any()).Return(nil).AnyTimes()

				disk, err := c.CreateDisk(ctx, "vol-test-name", diskOptions)
				if err != nil {
					t.Fatalf("CreateDisk() failed: expected no error, got: %v", err)
				} else {
					if expDisk.CapacityGiB != disk.CapacityGiB {
						t.Fatalf("CreateDisk() failed: expected capacity %d, got %d", expDisk.CapacityGiB, disk.CapacityGiB)
					}
					if expDisk.VolumeID != disk.VolumeID {
						t.Fatalf("CreateDisk() failed: expected capacity %q, got %q", expDisk.VolumeID, disk.VolumeID)
					}
					if expDisk.AvailabilityZone != disk.AvailabilityZone {
						t.Fatalf("CreateDisk() failed: expected availabilityZone %q, got %q", expDisk.AvailabilityZone, disk.AvailabilityZone)
					}
				}
			},
		},
		{
			name: "success: normal with encrypted volume",
			testFunc: func(t *testing.T) {
				diskOptions := &DiskOptions{
					CapacityBytes:    util.GiBToBytes(1),
					Tags:             map[string]string{VolumeNameTagKey: "vol-test"},
					AvailabilityZone: expZone,
					Encrypted:        true,
					KmsKeyID:         "arn:aws:kms:us-east-1:012345678910:key/abcd1234-a123-456a-a12b-a123b4cd56ef",
				}
				expDisk := &Disk{
					VolumeID:         "vol-test",
					CapacityGiB:      1,
					AvailabilityZone: expZone,
				}

				vol := &ec2.Volume{
					VolumeId:         aws.String(diskOptions.Tags[VolumeNameTagKey]),
					Size:             aws.Int64(util.BytesToGiB(diskOptions.CapacityBytes)),
					State:            aws.String("available"),
					AvailabilityZone: aws.String(expZone),
				}

				mockCtrl := gomock.NewController(t)
				defer mockCtrl.Finish()
				mockEC2 := mocks.NewMockEC2(mockCtrl)
				c := newCloud(mockEC2)
				ctx := context.Background()
				mockEC2.EXPECT().CreateVolumeWithContext(gomock.Eq(ctx), gomock.Any()).Return(vol, nil)
				mockEC2.EXPECT().WaitUntilVolumeAvailableWithContext(gomock.Eq(ctx), gomock.Any()).Return(nil).AnyTimes()

				disk, err := c.CreateDisk(ctx, "vol-test-name", diskOptions)
				if err != nil {
					t.Fatalf("CreateDisk() failed: expected no error, got: %v", err)
				} else {
					if expDisk.CapacityGiB != disk.CapacityGiB {
						t.Fatalf("CreateDisk() failed: expected capacity %d, got %d", expDisk.CapacityGiB, disk.CapacityGiB)
					}
					if expDisk.VolumeID != disk.VolumeID {
						t.Fatalf("CreateDisk() failed: expected capacity %q, got %q", expDisk.VolumeID, disk.VolumeID)
					}
					if expDisk.AvailabilityZone != disk.AvailabilityZone {
						t.Fatalf("CreateDisk() failed: expected availabilityZone %q, got %q", expDisk.AvailabilityZone, disk.AvailabilityZone)
					}
				}
			},
		},
		{
			name: "success: normal from snapshot",
			testFunc: func(t *testing.T) {
				diskOptions := &DiskOptions{
					CapacityBytes:    util.GiBToBytes(1),
					Tags:             map[string]string{VolumeNameTagKey: "vol-test"},
					AvailabilityZone: expZone,
					SnapshotID:       "snapshot-test",
				}
				expDisk := &Disk{
					VolumeID:         "vol-test",
					CapacityGiB:      1,
					AvailabilityZone: expZone,
				}

				vol := &ec2.Volume{
					VolumeId:         aws.String(diskOptions.Tags[VolumeNameTagKey]),
					Size:             aws.Int64(util.BytesToGiB(diskOptions.CapacityBytes)),
					State:            aws.String("available"),
					AvailabilityZone: aws.String(diskOptions.AvailabilityZone),
				}

				snapshot := &ec2.Snapshot{
					SnapshotId: aws.String(diskOptions.SnapshotID),
					VolumeId:   aws.String("snap-test-volume"),
					State:      aws.String("completed"),
				}

				mockCtrl := gomock.NewController(t)
				defer mockCtrl.Finish()
				mockEC2 := mocks.NewMockEC2(mockCtrl)
				c := newCloud(mockEC2)
				ctx := context.Background()
				mockEC2.EXPECT().CreateVolumeWithContext(gomock.Eq(ctx), gomock.Any()).Return(vol, nil)
				mockEC2.EXPECT().WaitUntilVolumeAvailableWithContext(gomock.Eq(ctx), gomock.Any()).Return(nil).AnyTimes()
				mockEC2.EXPECT().DescribeSnapshotsWithContext(gomock.Eq(ctx), gomock.Any()).Return(&ec2.DescribeSnapshotsOutput{Snapshots: []*ec2.Snapshot{snapshot}}, nil).AnyTimes()

				disk, err := c.CreateDisk(ctx, "vol-test-name", diskOptions)
				if err != nil {
					t.Fatalf("CreateDisk() failed: expected no error, got: %v", err)
				} else {
					if expDisk.CapacityGiB != disk.CapacityGiB {
						t.Fatalf("CreateDisk() failed: expected capacity %d, got %d", expDisk.CapacityGiB, disk.CapacityGiB)
					}
					if expDisk.VolumeID != disk.VolumeID {
						t.Fatalf("CreateDisk() failed: expected capacity %q, got %q", expDisk.VolumeID, disk.VolumeID)
					}
					if expDisk.AvailabilityZone != disk.AvailabilityZone {
						t.Fatalf("CreateDisk() failed: expected availability zone %q, got %q", expDisk.AvailabilityZone, disk.AvailabilityZone)
					}
				}
			},
		},
		{
			name: "fail: CreateVolume returned CreateVolume error",
			testFunc: func(t *testing.T) {
				diskOptions := &DiskOptions{
					CapacityBytes:    util.GiBToBytes(1),
					Tags:             map[string]string{VolumeNameTagKey: "vol-test"},
					AvailabilityZone: expZone,
				}

				mockCtrl := gomock.NewController(t)
				defer mockCtrl.Finish()
				mockEC2 := mocks.NewMockEC2(mockCtrl)
				c := newCloud(mockEC2)
				ctx := context.Background()
				mockEC2.EXPECT().CreateVolumeWithContext(gomock.Eq(ctx), gomock.Any()).Return(nil, fmt.Errorf("CreateVolume generic error"))

				if _, err := c.CreateDisk(ctx, "vol-test-name", diskOptions); err == nil {
					t.Fatalf("CreateDisk() succeeded, expected error")
				} else {
					expErr := fmt.Errorf("could not create volume in EC2: CreateVolume generic error")
					if err.Error() != expErr.Error() {
						t.Fatalf("CreateDisk() failed: expected error: %q, got: %q", expErr, err)
					}
				}
			},
		},
		{
			name: "fail: CreateVolume returned DescribeVolumes error",
			testFunc: func(t *testing.T) {
				diskOptions := &DiskOptions{
					CapacityBytes:    util.GiBToBytes(1),
					Tags:             map[string]string{VolumeNameTagKey: "vol-test"},
					AvailabilityZone: "",
				}
				vol := &ec2.Volume{
					VolumeId:         aws.String(diskOptions.Tags[VolumeNameTagKey]),
					Size:             aws.Int64(util.BytesToGiB(diskOptions.CapacityBytes)),
					State:            aws.String("creating"),
					AvailabilityZone: aws.String(expZone),
				}

				mockCtrl := gomock.NewController(t)
				defer mockCtrl.Finish()
				mockEC2 := mocks.NewMockEC2(mockCtrl)
				c := newCloud(mockEC2)
				ctx := context.Background()
				mockEC2.EXPECT().CreateVolumeWithContext(gomock.Eq(ctx), gomock.Any()).Return(vol, nil)
				mockEC2.EXPECT().WaitUntilVolumeAvailableWithContext(gomock.Eq(ctx), gomock.Any()).Return(fmt.Errorf("DescribeVolumes generic error")).AnyTimes()

				if _, err := c.CreateDisk(ctx, "vol-test-name", diskOptions); err == nil {
					t.Fatalf("CreateDisk() succeeded, expected error")
				} else {
					expErr := fmt.Errorf("failed to get an available volume in EC2: DescribeVolumes generic error")
					if err.Error() != expErr.Error() {
						t.Fatalf("CreateDisk() failed: expected error: %q, got: %q", expErr, err)
					}
				}
			},
		},
		{
			name: "fail: CreateVolume returned a volume with wrong state causing wait timeout",
			testFunc: func(t *testing.T) {
				diskOptions := &DiskOptions{
					CapacityBytes:    util.GiBToBytes(1),
					Tags:             map[string]string{VolumeNameTagKey: "vol-test"},
					AvailabilityZone: "",
				}
				vol := &ec2.Volume{
					VolumeId:         aws.String(diskOptions.Tags[VolumeNameTagKey]),
					Size:             aws.Int64(util.BytesToGiB(diskOptions.CapacityBytes)),
					State:            aws.String("creating"),
					AvailabilityZone: aws.String(expZone),
				}

				mockCtrl := gomock.NewController(t)
				defer mockCtrl.Finish()
				mockEC2 := mocks.NewMockEC2(mockCtrl)
				c := newCloud(mockEC2)
				ctx := context.Background()
				mockEC2.EXPECT().CreateVolumeWithContext(gomock.Eq(ctx), gomock.Any()).Return(vol, nil)
				mockEC2.EXPECT().WaitUntilVolumeAvailableWithContext(gomock.Eq(ctx), gomock.Any()).Return(fmt.Errorf("timed out waiting for the condition")).AnyTimes()

				if _, err := c.CreateDisk(ctx, "vol-test-name", diskOptions); err == nil {
					t.Fatalf("CreateDisk() succeeded, expected error")
				} else {
					expErr := fmt.Errorf("failed to get an available volume in EC2: timed out waiting for the condition")
					if err.Error() != expErr.Error() {
						t.Fatalf("CreateDisk() failed: expected error: %q, got: %q", expErr, err)
					}
				}
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, tc.testFunc)
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
		t.Run(tc.name, func(t *testing.T) {
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
		})
	}
}

func TestAttachDisk(t *testing.T) {
	testCases := []struct {
		name     string
		testFunc func(t *testing.T)
	}{
		{
			name: "success: normal",
			testFunc: func(t *testing.T) {

				volumeID := "vol-test-1234"
				nodeID := "node-1234"

				mockCtrl := gomock.NewController(t)
				mockEC2 := mocks.NewMockEC2(mockCtrl)
				c := newCloud(mockEC2)

				vol := &ec2.Volume{
					VolumeId:    aws.String(volumeID),
					Attachments: []*ec2.VolumeAttachment{{State: aws.String("attached")}},
				}

				ctx := context.Background()
				mockEC2.EXPECT().DescribeVolumesWithContext(gomock.Eq(ctx), gomock.Any()).Return(&ec2.DescribeVolumesOutput{Volumes: []*ec2.Volume{vol}}, nil).AnyTimes()
				mockEC2.EXPECT().DescribeInstancesWithContext(gomock.Eq(ctx), gomock.Any()).Return(newDescribeInstancesOutput(nodeID), nil)
				mockEC2.EXPECT().AttachVolumeWithContext(gomock.Eq(ctx), gomock.Any()).Return(&ec2.VolumeAttachment{}, nil)
				mockEC2.EXPECT().WaitUntilVolumeInUseWithContext(gomock.Eq(ctx), gomock.Any()).Return(nil).AnyTimes()

				devicePath, err := c.AttachDisk(ctx, volumeID, nodeID)
				if err != nil {
					t.Fatalf("AttachDisk() failed: expected no error, got: %v", err)
				} else {
					if !strings.HasPrefix(devicePath, "/dev/") {
						t.Fatal("AttachDisk() failed: expected valid device path, got empty string")
					}
				}

				mockCtrl.Finish()
			},
		},
		{
			name: "fail: AttachVolume generic error",
			testFunc: func(t *testing.T) {

				volumeID := "vol-test-1234"
				nodeID := "node-1234"

				mockCtrl := gomock.NewController(t)
				defer mockCtrl.Finish()
				mockEC2 := mocks.NewMockEC2(mockCtrl)
				c := newCloud(mockEC2)

				vol := &ec2.Volume{
					VolumeId:    aws.String(volumeID),
					Attachments: []*ec2.VolumeAttachment{{State: aws.String("attached")}},
				}

				attachErr := fmt.Errorf("generic error")

				ctx := context.Background()
				mockEC2.EXPECT().DescribeVolumesWithContext(gomock.Eq(ctx), gomock.Any()).Return(&ec2.DescribeVolumesOutput{Volumes: []*ec2.Volume{vol}}, nil).AnyTimes()
				mockEC2.EXPECT().DescribeInstancesWithContext(gomock.Eq(ctx), gomock.Any()).Return(newDescribeInstancesOutput(nodeID), nil)
				mockEC2.EXPECT().AttachVolumeWithContext(gomock.Eq(ctx), gomock.Any()).Return(nil, attachErr)

				expErr := fmt.Errorf("could not attach volume \"%s\" to node \"%s\": %s", volumeID, nodeID, attachErr.Error())
				if _, err := c.AttachDisk(ctx, volumeID, nodeID); err == nil {
					t.Fatalf("AttachDisk() succeeded: expected error %v", expErr)
				} else {
					if err.Error() != expErr.Error() {
						t.Fatalf("AttachDisk() failed: expected error: %q, got: %q", expErr, err)
					}
				}
			},
		},
		{
			name: "fail: wait error",
			testFunc: func(t *testing.T) {

				volumeID := "vol-test-1234"
				nodeID := "node-1234"

				mockCtrl := gomock.NewController(t)
				defer mockCtrl.Finish()
				mockEC2 := mocks.NewMockEC2(mockCtrl)
				c := newCloud(mockEC2)

				vol := &ec2.Volume{
					VolumeId:    aws.String(volumeID),
					Attachments: []*ec2.VolumeAttachment{{State: aws.String("attached")}},
				}

				ctx := context.Background()
				mockEC2.EXPECT().DescribeVolumesWithContext(gomock.Eq(ctx), gomock.Any()).Return(&ec2.DescribeVolumesOutput{Volumes: []*ec2.Volume{vol}}, nil).AnyTimes()
				mockEC2.EXPECT().DescribeInstancesWithContext(gomock.Eq(ctx), gomock.Any()).Return(newDescribeInstancesOutput(nodeID), nil)
				mockEC2.EXPECT().AttachVolumeWithContext(gomock.Eq(ctx), gomock.Any()).Return(&ec2.VolumeAttachment{}, nil)
				mockEC2.EXPECT().WaitUntilVolumeInUseWithContext(gomock.Eq(ctx), gomock.Any()).Return(fmt.Errorf("timed out waiting for the condition")).AnyTimes()

				expErr := fmt.Errorf("timed out waiting for the condition")
				if _, err := c.AttachDisk(ctx, volumeID, nodeID); err == nil {
					t.Fatalf("AttachDisk() succeeded: expected error %v", expErr)
				} else {
					if err.Error() != expErr.Error() {
						t.Fatalf("AttachDisk() failed: expected error: %q, got: %q", expErr, err)
					}
				}
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, tc.testFunc)
	}
}

func TestDetachDisk(t *testing.T) {
	testCases := []struct {
		name     string
		volumeID string
		nodeID   string
		expErr   error
		testFunc func(t *testing.T)
	}{
		{
			name: "success: normal",
			testFunc: func(t *testing.T) {
				volumeID := "vol-test-1234"
				nodeID := "node-1234"

				mockCtrl := gomock.NewController(t)
				defer mockCtrl.Finish()
				mockEC2 := mocks.NewMockEC2(mockCtrl)
				c := newCloud(mockEC2)

				vol := &ec2.Volume{
					VolumeId:    aws.String(volumeID),
					Attachments: nil,
				}

				ctx := context.Background()
				mockEC2.EXPECT().DescribeVolumesWithContext(gomock.Eq(ctx), gomock.Any()).Return(&ec2.DescribeVolumesOutput{Volumes: []*ec2.Volume{vol}}, nil).AnyTimes()
				mockEC2.EXPECT().DescribeInstancesWithContext(gomock.Eq(ctx), gomock.Any()).Return(newDescribeInstancesOutput(nodeID), nil)
				mockEC2.EXPECT().DetachVolumeWithContext(gomock.Eq(ctx), gomock.Any()).Return(&ec2.VolumeAttachment{}, nil)
				mockEC2.EXPECT().WaitUntilVolumeAvailableWithContext(gomock.Eq(ctx), gomock.Any()).Return(nil).AnyTimes()

				if err := c.DetachDisk(ctx, volumeID, nodeID); err != nil {
					t.Fatalf("DetachDisk() failed: expected no error, got: %v", err)
				}
			},
		},
		{
			name:     "fail: DetachVolume returned generic error",
			volumeID: "vol-test-1234",
			nodeID:   "node-1234",
			expErr:   fmt.Errorf("DetachVolume generic error"),
			testFunc: func(t *testing.T) {
				volumeID := "vol-test-1234"
				nodeID := "node-1234"

				mockCtrl := gomock.NewController(t)
				defer mockCtrl.Finish()
				mockEC2 := mocks.NewMockEC2(mockCtrl)
				c := newCloud(mockEC2)

				vol := &ec2.Volume{
					VolumeId:    aws.String(volumeID),
					Attachments: nil,
				}

				ctx := context.Background()
				mockEC2.EXPECT().DescribeVolumesWithContext(gomock.Eq(ctx), gomock.Any()).Return(&ec2.DescribeVolumesOutput{Volumes: []*ec2.Volume{vol}}, nil).AnyTimes()
				mockEC2.EXPECT().DescribeInstancesWithContext(gomock.Eq(ctx), gomock.Any()).Return(newDescribeInstancesOutput(nodeID), nil)
				mockEC2.EXPECT().DetachVolumeWithContext(gomock.Eq(ctx), gomock.Any()).Return(nil, fmt.Errorf("generic error"))

				expErr := fmt.Errorf("could not detach volume \"%s\" from node \"%s\": generic error", volumeID, nodeID)
				if err := c.DetachDisk(ctx, volumeID, nodeID); err == nil {
					t.Fatalf("DetachDisk() failed: expected error, %v", expErr)
				} else {
					if err.Error() != expErr.Error() {
						t.Fatalf("DetachDisk() failed: expected error: %q, got: %q", expErr, err)
					}
				}
			},
		},
		{
			name:     "fail: wait error",
			volumeID: "vol-test-1234",
			nodeID:   "node-1234",
			expErr:   fmt.Errorf("DetachVolume generic error"),
			testFunc: func(t *testing.T) {
				volumeID := "vol-test-1234"
				nodeID := "node-1234"

				mockCtrl := gomock.NewController(t)
				defer mockCtrl.Finish()
				mockEC2 := mocks.NewMockEC2(mockCtrl)
				c := newCloud(mockEC2)

				vol := &ec2.Volume{
					VolumeId:    aws.String(volumeID),
					Attachments: nil,
				}

				expErr := fmt.Errorf("timed out waiting for the condition")

				ctx := context.Background()
				mockEC2.EXPECT().DescribeVolumesWithContext(gomock.Eq(ctx), gomock.Any()).Return(&ec2.DescribeVolumesOutput{Volumes: []*ec2.Volume{vol}}, nil).AnyTimes()
				mockEC2.EXPECT().DescribeInstancesWithContext(gomock.Eq(ctx), gomock.Any()).Return(newDescribeInstancesOutput(nodeID), nil)
				mockEC2.EXPECT().DetachVolumeWithContext(gomock.Eq(ctx), gomock.Any()).Return(&ec2.VolumeAttachment{}, nil)
				mockEC2.EXPECT().WaitUntilVolumeAvailableWithContext(gomock.Eq(ctx), gomock.Any()).Return(expErr).AnyTimes()

				if err := c.DetachDisk(ctx, volumeID, nodeID); err == nil {
					t.Fatalf("DetachDisk() failed: expected error, %v", expErr)
				} else {
					if err.Error() != expErr.Error() {
						t.Fatalf("DetachDisk() failed: expected error: %q, got: %q", expErr, err)
					}
				}
			},
		},
	}

	for _, tc := range testCases {
		if tc.testFunc != nil {
			t.Run(tc.name, tc.testFunc)
		}
	}
}

func TestGetDiskByName(t *testing.T) {
	testCases := []struct {
		name             string
		volumeName       string
		volumeCapacity   int64
		availabilityZone string
		expErr           error
	}{
		{
			name:             "success: normal",
			volumeName:       "vol-test-1234",
			volumeCapacity:   util.GiBToBytes(1),
			availabilityZone: expZone,
			expErr:           nil,
		},
		{
			name:           "fail: DescribeVolumes returned generic error",
			volumeName:     "vol-test-1234",
			volumeCapacity: util.GiBToBytes(1),
			expErr:         fmt.Errorf("DescribeVolumes generic error"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockCtrl := gomock.NewController(t)
			mockEC2 := mocks.NewMockEC2(mockCtrl)
			c := newCloud(mockEC2)

			vol := &ec2.Volume{
				VolumeId:         aws.String(tc.volumeName),
				Size:             aws.Int64(util.BytesToGiB(tc.volumeCapacity)),
				AvailabilityZone: aws.String(tc.availabilityZone),
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
				if tc.availabilityZone != disk.AvailabilityZone {
					t.Fatalf("GetDiskByName() failed: expected availabilityZone %q, got %q", tc.availabilityZone, disk.AvailabilityZone)
				}
			}

			mockCtrl.Finish()
		})
	}
}

func TestGetDiskByID(t *testing.T) {
	testCases := []struct {
		name             string
		volumeID         string
		availabilityZone string
		expErr           error
	}{
		{
			name:             "success: normal",
			volumeID:         "vol-test-1234",
			availabilityZone: expZone,
			expErr:           nil,
		},
		{
			name:     "fail: DescribeVolumes returned generic error",
			volumeID: "vol-test-1234",
			expErr:   fmt.Errorf("DescribeVolumes generic error"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockCtrl := gomock.NewController(t)
			mockEC2 := mocks.NewMockEC2(mockCtrl)
			c := newCloud(mockEC2)

			ctx := context.Background()
			mockEC2.EXPECT().DescribeVolumesWithContext(gomock.Eq(ctx), gomock.Any()).Return(
				&ec2.DescribeVolumesOutput{
					Volumes: []*ec2.Volume{
						{
							VolumeId:         aws.String(tc.volumeID),
							AvailabilityZone: aws.String(tc.availabilityZone),
						},
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
				if tc.availabilityZone != disk.AvailabilityZone {
					t.Fatalf("GetDiskByName() failed: expected availabilityZone %q, got %q", tc.availabilityZone, disk.AvailabilityZone)
				}
			}

			mockCtrl.Finish()
		})
	}
}

func TestCreateSnapshot(t *testing.T) {
	testCases := []struct {
		name            string
		snapshotName    string
		snapshotOptions *SnapshotOptions
		expSnapshot     *Snapshot
		expErr          error
	}{
		{
			name:         "success: normal",
			snapshotName: "snap-test-name",
			snapshotOptions: &SnapshotOptions{
				Tags: map[string]string{
					SnapshotNameTagKey: "snap-test-name",
				},
			},
			expSnapshot: &Snapshot{
				SourceVolumeID: "snap-test-volume",
			},
			expErr: nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockCtrl := gomock.NewController(t)
			mockEC2 := mocks.NewMockEC2(mockCtrl)
			c := newCloud(mockEC2)

			ec2snapshot := &ec2.Snapshot{
				SnapshotId: aws.String(tc.snapshotOptions.Tags[SnapshotNameTagKey]),
				VolumeId:   aws.String("snap-test-volume"),
				State:      aws.String("completed"),
			}

			ctx := context.Background()
			mockEC2.EXPECT().CreateSnapshotWithContext(gomock.Eq(ctx), gomock.Any()).Return(ec2snapshot, tc.expErr)
			mockEC2.EXPECT().DescribeSnapshotsWithContext(gomock.Eq(ctx), gomock.Any()).Return(&ec2.DescribeSnapshotsOutput{Snapshots: []*ec2.Snapshot{ec2snapshot}}, nil).AnyTimes()

			snapshot, err := c.CreateSnapshot(ctx, tc.expSnapshot.SourceVolumeID, tc.snapshotOptions)
			if err != nil {
				if tc.expErr == nil {
					t.Fatalf("CreateSnapshot() failed: expected no error, got: %v", err)
				}
			} else {
				if tc.expErr != nil {
					t.Fatal("CreateSnapshot() failed: expected error, got nothing")
				} else {
					if snapshot.SourceVolumeID != tc.expSnapshot.SourceVolumeID {
						t.Fatalf("CreateSnapshot() failed: expected source volume ID %s, got %v", tc.expSnapshot.SourceVolumeID, snapshot.SourceVolumeID)
					}
				}
			}

			mockCtrl.Finish()
		})
	}
}

func TestDeleteSnapshot(t *testing.T) {
	testCases := []struct {
		name         string
		snapshotName string
		expErr       error
	}{
		{
			name:         "success: normal",
			snapshotName: "snap-test-name",
			expErr:       nil,
		},
		{
			name:         "fail: delete snapshot return generic error",
			snapshotName: "snap-test-name",
			expErr:       fmt.Errorf("DeleteSnapshot generic error"),
		},
		{
			name:         "fail: delete snapshot return not found error",
			snapshotName: "snap-test-name",
			expErr:       awserr.New("InvalidSnapshot.NotFound", "", nil),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockCtrl := gomock.NewController(t)
			mockEC2 := mocks.NewMockEC2(mockCtrl)
			c := newCloud(mockEC2)

			ctx := context.Background()
			mockEC2.EXPECT().DeleteSnapshotWithContext(gomock.Eq(ctx), gomock.Any()).Return(&ec2.DeleteSnapshotOutput{}, tc.expErr)

			_, err := c.DeleteSnapshot(ctx, tc.snapshotName)
			if err != nil {
				if tc.expErr == nil {
					t.Fatalf("DeleteSnapshot() failed: expected no error, got: %v", err)
				}
			} else {
				if tc.expErr != nil {
					t.Fatal("DeleteSnapshot() failed: expected error, got nothing")
				}
			}

			mockCtrl.Finish()
		})
	}
}

func TestGetSnapshotByName(t *testing.T) {
	testCases := []struct {
		name            string
		snapshotName    string
		snapshotOptions *SnapshotOptions
		expSnapshot     *Snapshot
		expErr          error
	}{
		{
			name:         "success: normal",
			snapshotName: "snap-test-name",
			snapshotOptions: &SnapshotOptions{
				Tags: map[string]string{
					SnapshotNameTagKey: "snap-test-name",
				},
			},
			expSnapshot: &Snapshot{
				SourceVolumeID: "snap-test-volume",
			},
			expErr: nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockCtrl := gomock.NewController(t)
			mockEC2 := mocks.NewMockEC2(mockCtrl)
			c := newCloud(mockEC2)

			ec2snapshot := &ec2.Snapshot{
				SnapshotId: aws.String(tc.snapshotOptions.Tags[SnapshotNameTagKey]),
				VolumeId:   aws.String("snap-test-volume"),
				State:      aws.String("completed"),
			}

			ctx := context.Background()
			mockEC2.EXPECT().DescribeSnapshotsWithContext(gomock.Eq(ctx), gomock.Any()).Return(&ec2.DescribeSnapshotsOutput{Snapshots: []*ec2.Snapshot{ec2snapshot}}, nil)

			_, err := c.GetSnapshotByName(ctx, tc.snapshotOptions.Tags[SnapshotNameTagKey])
			if err != nil {
				if tc.expErr == nil {
					t.Fatalf("GetSnapshotByName() failed: expected no error, got: %v", err)
				}
			} else {
				if tc.expErr != nil {
					t.Fatal("GetSnapshotByName() failed: expected error, got nothing")
				}
			}

			mockCtrl.Finish()
		})
	}
}

func TestWaitForAttachmentState(t *testing.T) {
	testCases := []struct {
		name     string
		testFunc func(t *testing.T)
	}{
		{
			name: "success: in use",
			testFunc: func(t *testing.T) {
				mockCtrl := gomock.NewController(t)
				mockEC2 := mocks.NewMockEC2(mockCtrl)
				c := newCloud(mockEC2)
				ctx := context.Background()
				mockEC2.EXPECT().WaitUntilVolumeInUseWithContext(gomock.Eq(ctx), gomock.Any()).Return(nil)
				if err := c.WaitForAttachmentState(ctx, "vol-test", "attached"); err != nil {
					t.Fatalf("WaitForAttachmentState() unexpected error: %v", err)
				}
			},
		},
		{
			name: "success: available",
			testFunc: func(t *testing.T) {
				mockCtrl := gomock.NewController(t)
				mockEC2 := mocks.NewMockEC2(mockCtrl)
				c := newCloud(mockEC2)
				ctx := context.Background()
				mockEC2.EXPECT().WaitUntilVolumeAvailableWithContext(gomock.Eq(ctx), gomock.Any()).Return(nil)
				if err := c.WaitForAttachmentState(ctx, "vol-test", "detached"); err != nil {
					t.Fatalf("WaitForAttachmentState() unexpected error: %v", err)
				}
			},
		},
		{
			name: "fail: in use",
			testFunc: func(t *testing.T) {
				mockCtrl := gomock.NewController(t)
				mockEC2 := mocks.NewMockEC2(mockCtrl)
				c := newCloud(mockEC2)
				ctx := context.Background()
				expErr := fmt.Errorf("generic error")
				mockEC2.EXPECT().WaitUntilVolumeAvailableWithContext(gomock.Eq(ctx), gomock.Any()).Return(fmt.Errorf("generic error"))
				if err := c.WaitForAttachmentState(ctx, "vol-test", "detached"); err == nil {
					t.Fatalf("WaitForAttachmentState() expected error: %v", expErr)
				} else {
					if err.Error() != expErr.Error() {
						t.Fatalf("WaitForAttachmentState() expected error %v, got %v", expErr, err)
					}
				}
			},
		},
		{
			name: "fail: detached",
			testFunc: func(t *testing.T) {
				mockCtrl := gomock.NewController(t)
				mockEC2 := mocks.NewMockEC2(mockCtrl)
				c := newCloud(mockEC2)
				ctx := context.Background()
				expErr := fmt.Errorf("generic error")
				mockEC2.EXPECT().WaitUntilVolumeInUseWithContext(gomock.Eq(ctx), gomock.Any()).Return(fmt.Errorf("generic error"))
				if err := c.WaitForAttachmentState(ctx, "vol-test", "attached"); err == nil {
					t.Fatalf("WaitForAttachmentState() expected error: %v", expErr)
				} else {
					if err.Error() != expErr.Error() {
						t.Fatalf("WaitForAttachmentState() expected error %v, got %v", expErr, err)
					}
				}
			},
		},
		{
			name: "fail: invalid state",
			testFunc: func(t *testing.T) {
				mockCtrl := gomock.NewController(t)
				mockEC2 := mocks.NewMockEC2(mockCtrl)
				c := newCloud(mockEC2)
				ctx := context.Background()
				state := "foo"
				expErr := fmt.Errorf("invalid state name %s", state)
				if err := c.WaitForAttachmentState(ctx, "vol-test", state); err == nil {
					t.Fatalf("WaitForAttachmentState() expected error: %v", expErr)
				} else {
					if err.Error() != expErr.Error() {
						t.Fatalf("WaitForAttachmentState() expected error %v, got %v", expErr, err)
					}
				}
			},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, tc.testFunc)
	}
}

func TestListSnapshots(t *testing.T) {
	testCases := []struct {
		name     string
		testFunc func(t *testing.T)
	}{
		{
			name: "success: normal",
			testFunc: func(t *testing.T) {
				expSnapshots := []*Snapshot{
					{
						SourceVolumeID: "snap-test-volume1",
						SnapshotID:     "snap-test-name1",
					},
					{
						SourceVolumeID: "snap-test-volume2",
						SnapshotID:     "snap-test-name2",
					},
				}
				ec2Snapshots := []*ec2.Snapshot{
					{
						SnapshotId: aws.String(expSnapshots[0].SnapshotID),
						VolumeId:   aws.String("snap-test-volume1"),
						State:      aws.String("completed"),
					},
					{
						SnapshotId: aws.String(expSnapshots[1].SnapshotID),
						VolumeId:   aws.String("snap-test-volume2"),
						State:      aws.String("completed"),
					},
				}

				mockCtl := gomock.NewController(t)
				defer mockCtl.Finish()
				mockEC2 := mocks.NewMockEC2(mockCtl)
				c := newCloud(mockEC2)

				ctx := context.Background()

				mockEC2.EXPECT().DescribeSnapshotsWithContext(gomock.Eq(ctx), gomock.Any()).Return(&ec2.DescribeSnapshotsOutput{Snapshots: ec2Snapshots}, nil)

				_, err := c.ListSnapshots(ctx, "", 0, "")
				if err != nil {
					t.Fatalf("ListSnapshots() failed: expected no error, got: %v", err)
				}
			},
		},
		{
			name: "success: with volume ID",
			testFunc: func(t *testing.T) {
				sourceVolumeID := "snap-test-volume"
				expSnapshots := []*Snapshot{
					{
						SourceVolumeID: sourceVolumeID,
						SnapshotID:     "snap-test-name1",
					},
					{
						SourceVolumeID: sourceVolumeID,
						SnapshotID:     "snap-test-name2",
					},
				}
				ec2Snapshots := []*ec2.Snapshot{
					{
						SnapshotId: aws.String(expSnapshots[0].SnapshotID),
						VolumeId:   aws.String(sourceVolumeID),
						State:      aws.String("completed"),
					},
					{
						SnapshotId: aws.String(expSnapshots[1].SnapshotID),
						VolumeId:   aws.String(sourceVolumeID),
						State:      aws.String("completed"),
					},
				}

				mockCtl := gomock.NewController(t)
				defer mockCtl.Finish()
				mockEC2 := mocks.NewMockEC2(mockCtl)
				c := newCloud(mockEC2)

				ctx := context.Background()

				mockEC2.EXPECT().DescribeSnapshotsWithContext(gomock.Eq(ctx), gomock.Any()).Return(&ec2.DescribeSnapshotsOutput{Snapshots: ec2Snapshots}, nil)

				resp, err := c.ListSnapshots(ctx, sourceVolumeID, 0, "")
				if err != nil {
					t.Fatalf("ListSnapshots() failed: expected no error, got: %v", err)
				}

				if len(resp.Snapshots) != len(expSnapshots) {
					t.Fatalf("Expected %d snapshots, got %d", len(expSnapshots), len(resp.Snapshots))
				}

				for _, snap := range resp.Snapshots {
					if snap.SourceVolumeID != sourceVolumeID {
						t.Fatalf("Unexpected source volume.  Expected %s, got %s", sourceVolumeID, snap.SourceVolumeID)
					}
				}
			},
		},
		{
			name: "success: max results, next token",
			testFunc: func(t *testing.T) {
				maxResults := 5
				nextTokenValue := "nextTokenValue"
				var expSnapshots []*Snapshot
				for i := 0; i < maxResults*2; i++ {
					expSnapshots = append(expSnapshots, &Snapshot{
						SourceVolumeID: "snap-test-volume1",
						SnapshotID:     fmt.Sprintf("snap-test-name%d", i),
					})
				}

				var ec2Snapshots []*ec2.Snapshot
				for i := 0; i < maxResults*2; i++ {
					ec2Snapshots = append(ec2Snapshots, &ec2.Snapshot{
						SnapshotId: aws.String(expSnapshots[i].SnapshotID),
						VolumeId:   aws.String(fmt.Sprintf("snap-test-volume%d", i)),
						State:      aws.String("completed"),
					})
				}

				mockCtl := gomock.NewController(t)
				defer mockCtl.Finish()
				mockEC2 := mocks.NewMockEC2(mockCtl)
				c := newCloud(mockEC2)

				ctx := context.Background()

				firstCall := mockEC2.EXPECT().DescribeSnapshotsWithContext(gomock.Eq(ctx), gomock.Any()).Return(&ec2.DescribeSnapshotsOutput{
					Snapshots: ec2Snapshots[:maxResults],
					NextToken: aws.String(nextTokenValue),
				}, nil)
				secondCall := mockEC2.EXPECT().DescribeSnapshotsWithContext(gomock.Eq(ctx), gomock.Any()).Return(&ec2.DescribeSnapshotsOutput{
					Snapshots: ec2Snapshots[maxResults:],
				}, nil)
				gomock.InOrder(
					firstCall,
					secondCall,
				)

				firstSnapshotsResponse, err := c.ListSnapshots(ctx, "", 5, "")
				if err != nil {
					t.Fatalf("ListSnapshots() failed: expected no error, got: %v", err)
				}

				if len(firstSnapshotsResponse.Snapshots) != maxResults {
					t.Fatalf("Expected %d snapshots, got %d", maxResults, len(firstSnapshotsResponse.Snapshots))
				}

				if firstSnapshotsResponse.NextToken != nextTokenValue {
					t.Fatalf("Expected next token value '%s' got '%s'", nextTokenValue, firstSnapshotsResponse.NextToken)
				}

				secondSnapshotsResponse, err := c.ListSnapshots(ctx, "", 0, firstSnapshotsResponse.NextToken)
				if err != nil {
					t.Fatalf("CreateSnapshot() failed: expected no error, got: %v", err)
				}

				if len(secondSnapshotsResponse.Snapshots) != maxResults {
					t.Fatalf("Expected %d snapshots, got %d", maxResults, len(secondSnapshotsResponse.Snapshots))
				}

				if secondSnapshotsResponse.NextToken != "" {
					t.Fatalf("Expected next token value to be empty got %s", secondSnapshotsResponse.NextToken)
				}
			},
		},
		{
			name: "fail: AWS DescribeSnapshotsWithContext error",
			testFunc: func(t *testing.T) {
				mockCtl := gomock.NewController(t)
				defer mockCtl.Finish()
				mockEC2 := mocks.NewMockEC2(mockCtl)
				c := newCloud(mockEC2)

				ctx := context.Background()

				mockEC2.EXPECT().DescribeSnapshotsWithContext(gomock.Eq(ctx), gomock.Any()).Return(nil, errors.New("test error"))

				if _, err := c.ListSnapshots(ctx, "", 0, ""); err == nil {
					t.Fatalf("ListSnapshots() failed: expected an error, got none")
				}
			},
		},
		{
			name: "fail: no snapshots ErrNotFound",
			testFunc: func(t *testing.T) {
				mockCtl := gomock.NewController(t)
				defer mockCtl.Finish()
				mockEC2 := mocks.NewMockEC2(mockCtl)
				c := newCloud(mockEC2)

				ctx := context.Background()

				mockEC2.EXPECT().DescribeSnapshotsWithContext(gomock.Eq(ctx), gomock.Any()).Return(&ec2.DescribeSnapshotsOutput{}, nil)

				if _, err := c.ListSnapshots(ctx, "", 0, ""); err != nil {
					if err != ErrNotFound {
						t.Fatalf("Expected error %v, got %v", ErrNotFound, err)
					}
				} else {
					t.Fatalf("Expected error, got none")
				}
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, tc.testFunc)
	}
}

func newCloud(mockEC2 EC2) Cloud {
	return &cloud{
		metadata: &Metadata{
			InstanceID:       "test-instance",
			Region:           "test-region",
			AvailabilityZone: defaultZone,
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
