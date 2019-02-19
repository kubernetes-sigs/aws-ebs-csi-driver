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

package driver

import (
	"context"
	"reflect"
	"testing"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/kubernetes-sigs/aws-ebs-csi-driver/pkg/cloud"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	expZone   = "us-west-2b"
	expFsType = "ext2"
)

func TestCreateVolume(t *testing.T) {
	stdVolCap := []*csi.VolumeCapability{
		{
			AccessType: &csi.VolumeCapability_Mount{
				Mount: &csi.VolumeCapability_MountVolume{},
			},
			AccessMode: &csi.VolumeCapability_AccessMode{
				Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER,
			},
		},
	}
	stdVolSize := int64(5 * 1024 * 1024 * 1024)
	stdCapRange := &csi.CapacityRange{RequiredBytes: stdVolSize}
	stdParams := map[string]string{}

	testCases := []struct {
		name       string
		req        *csi.CreateVolumeRequest
		extraReq   *csi.CreateVolumeRequest
		expVol     *csi.Volume
		expErrCode codes.Code
	}{
		{
			name: "success normal",
			req: &csi.CreateVolumeRequest{
				Name:               "random-vol-name",
				CapacityRange:      stdCapRange,
				VolumeCapabilities: stdVolCap,
				Parameters:         nil,
			},
			expVol: &csi.Volume{
				CapacityBytes: stdVolSize,
				VolumeId:      "vol-test",
				VolumeContext: map[string]string{FsTypeKey: ""},
			},
		},
		{
			name: "fail no name",
			req: &csi.CreateVolumeRequest{
				Name:               "",
				CapacityRange:      stdCapRange,
				VolumeCapabilities: stdVolCap,
				Parameters:         stdParams,
			},
			expErrCode: codes.InvalidArgument,
		},
		{
			name: "success same name and same capacity",
			req: &csi.CreateVolumeRequest{
				Name:               "test-vol",
				CapacityRange:      stdCapRange,
				VolumeCapabilities: stdVolCap,
				Parameters:         stdParams,
			},
			extraReq: &csi.CreateVolumeRequest{
				Name:               "test-vol",
				CapacityRange:      stdCapRange,
				VolumeCapabilities: stdVolCap,
				Parameters:         stdParams,
			},
			expVol: &csi.Volume{
				CapacityBytes: stdVolSize,
				VolumeId:      "vol-test",
				VolumeContext: map[string]string{FsTypeKey: ""},
			},
		},
		{
			name: "fail same name and different capacity",
			req: &csi.CreateVolumeRequest{
				Name:               "test-vol",
				CapacityRange:      stdCapRange,
				VolumeCapabilities: stdVolCap,
				Parameters:         stdParams,
			},
			extraReq: &csi.CreateVolumeRequest{
				Name:               "test-vol",
				CapacityRange:      &csi.CapacityRange{RequiredBytes: 10000},
				VolumeCapabilities: stdVolCap,
				Parameters:         stdParams,
			},
			expErrCode: codes.AlreadyExists,
		},
		{
			name: "success no capacity range",
			req: &csi.CreateVolumeRequest{
				Name:               "test-vol",
				VolumeCapabilities: stdVolCap,
				Parameters:         stdParams,
			},
			expVol: &csi.Volume{
				CapacityBytes: cloud.DefaultVolumeSize,
				VolumeId:      "vol-test",
				VolumeContext: map[string]string{FsTypeKey: ""},
			},
		},
		{
			name: "success with correct round up",
			req: &csi.CreateVolumeRequest{
				Name:               "vol-test",
				CapacityRange:      &csi.CapacityRange{RequiredBytes: 1073741825},
				VolumeCapabilities: stdVolCap,
				Parameters:         nil,
			},
			expVol: &csi.Volume{
				CapacityBytes: 2147483648, // 1 GiB + 1 byte = 2 GiB
				VolumeId:      "vol-test",
				VolumeContext: map[string]string{FsTypeKey: ""},
			},
		},
		{
			name: "success with fstype parameter",
			req: &csi.CreateVolumeRequest{
				Name:               "vol-test",
				CapacityRange:      stdCapRange,
				VolumeCapabilities: stdVolCap,
				Parameters:         map[string]string{FsTypeKey: defaultFsType},
			},
			expVol: &csi.Volume{
				CapacityBytes: stdVolSize,
				VolumeId:      "vol-test",
				VolumeContext: map[string]string{FsTypeKey: defaultFsType},
			},
		},
		{
			name: "success with volume type io1",
			req: &csi.CreateVolumeRequest{
				Name:               "vol-test",
				CapacityRange:      stdCapRange,
				VolumeCapabilities: stdVolCap,
				Parameters: map[string]string{
					VolumeTypeKey: cloud.VolumeTypeIO1,
					IopsPerGBKey:  "5",
				},
			},
			expVol: &csi.Volume{
				CapacityBytes: stdVolSize,
				VolumeId:      "vol-test",
				VolumeContext: map[string]string{FsTypeKey: ""},
			},
		},
		{
			name: "success with volume type sc1",
			req: &csi.CreateVolumeRequest{
				Name:               "vol-test",
				CapacityRange:      stdCapRange,
				VolumeCapabilities: stdVolCap,
				Parameters: map[string]string{
					VolumeTypeKey: cloud.VolumeTypeSC1,
				},
			},
			expVol: &csi.Volume{
				CapacityBytes: stdVolSize,
				VolumeId:      "vol-test",
				VolumeContext: map[string]string{FsTypeKey: ""},
			},
		},
		{
			name: "success with volume encryption",
			req: &csi.CreateVolumeRequest{
				Name:               "vol-test",
				CapacityRange:      stdCapRange,
				VolumeCapabilities: stdVolCap,
				Parameters: map[string]string{
					EncryptedKey: "true",
				},
			},
			expVol: &csi.Volume{
				CapacityBytes: stdVolSize,
				VolumeId:      "vol-test",
				VolumeContext: map[string]string{FsTypeKey: ""},
			},
		},
		{
			name: "success with volume encryption with KMS key",
			req: &csi.CreateVolumeRequest{
				Name:               "vol-test",
				CapacityRange:      stdCapRange,
				VolumeCapabilities: stdVolCap,
				Parameters: map[string]string{
					EncryptedKey: "true",
					KmsKeyIdKey:  "arn:aws:kms:us-east-1:012345678910:key/abcd1234-a123-456a-a12b-a123b4cd56ef",
				},
			},
			expVol: &csi.Volume{
				CapacityBytes: stdVolSize,
				VolumeId:      "vol-test",
				VolumeContext: map[string]string{FsTypeKey: ""},
			},
		},
		{
			name: "success when volume exists and contains VolumeContext and AccessibleTopology",
			req: &csi.CreateVolumeRequest{
				Name:               "test-vol",
				CapacityRange:      stdCapRange,
				VolumeCapabilities: stdVolCap,
				Parameters: map[string]string{
					FsTypeKey: expFsType,
				},
				AccessibilityRequirements: &csi.TopologyRequirement{
					Requisite: []*csi.Topology{
						{
							Segments: map[string]string{TopologyKey: expZone},
						},
					},
				},
			},
			extraReq: &csi.CreateVolumeRequest{
				Name:               "test-vol",
				CapacityRange:      stdCapRange,
				VolumeCapabilities: stdVolCap,
				Parameters: map[string]string{
					FsTypeKey: expFsType,
				},
				AccessibilityRequirements: &csi.TopologyRequirement{
					Requisite: []*csi.Topology{
						{
							Segments: map[string]string{TopologyKey: expZone},
						},
					},
				},
			},
			expVol: &csi.Volume{
				CapacityBytes: stdVolSize,
				VolumeId:      "vol-test",
				VolumeContext: map[string]string{FsTypeKey: expFsType},
				AccessibleTopology: []*csi.Topology{
					{
						Segments: map[string]string{TopologyKey: expZone},
					},
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			awsDriver := NewFakeDriver("", cloud.NewFakeCloudProvider(), NewFakeMounter())

			resp, err := awsDriver.CreateVolume(context.TODO(), tc.req)
			if err != nil {
				srvErr, ok := status.FromError(err)
				if !ok {
					t.Fatalf("Could not get error status code from error: %v", srvErr)
				}
				if srvErr.Code() != tc.expErrCode {
					t.Fatalf("Expected error code %d, got %d message %s", tc.expErrCode, srvErr.Code(), srvErr.Message())
				}
				return
			}

			// Repeat the same request and check they results of the second call
			if tc.extraReq != nil {
				resp, err = awsDriver.CreateVolume(context.TODO(), tc.extraReq)
				if err != nil {
					srvErr, ok := status.FromError(err)
					if !ok {
						t.Fatalf("Could not get error status code from error: %v", srvErr)
					}
					if srvErr.Code() != tc.expErrCode {
						t.Fatalf("Expected error code %d, got %d", tc.expErrCode, srvErr.Code())
					}
					return
				}
			}

			if tc.expErrCode != codes.OK {
				t.Fatalf("Expected error %v, got no error", tc.expErrCode)
			}

			vol := resp.GetVolume()
			if vol == nil && tc.expVol != nil {
				t.Fatalf("Expected volume %v, got nil", tc.expVol)
			}

			if vol.GetCapacityBytes() != tc.expVol.GetCapacityBytes() {
				t.Fatalf("Expected volume capacity bytes: %v, got: %v", tc.expVol.GetCapacityBytes(), vol.GetCapacityBytes())
			}

			for expKey, expVal := range tc.expVol.GetVolumeContext() {
				ctx := vol.GetVolumeContext()
				if gotVal, ok := ctx[expKey]; !ok || gotVal != expVal {
					t.Fatalf("Expected volume context for key %v: %v, got: %v", expKey, expVal, gotVal)
				}
			}
			if tc.expVol.GetVolumeContext() == nil && vol.GetVolumeContext() != nil {
				t.Fatalf("Expected volume context to be nil, got: %#v", vol.GetVolumeContext())
			}
			if tc.expVol.GetAccessibleTopology() != nil {
				if !reflect.DeepEqual(tc.expVol.GetAccessibleTopology(), vol.GetAccessibleTopology()) {
					t.Fatalf("Expected AccessibleTopology to be %+v, got: %+v", tc.expVol.GetAccessibleTopology(), vol.GetAccessibleTopology())
				}
			}
		})
	}
}

func TestDeleteVolume(t *testing.T) {
	testCases := []struct {
		name       string
		req        *csi.DeleteVolumeRequest
		expResp    *csi.DeleteVolumeResponse
		expErrCode codes.Code
	}{
		{
			name: "success normal",
			req: &csi.DeleteVolumeRequest{
				VolumeId: "vol-test",
			},
			expResp: &csi.DeleteVolumeResponse{},
		},
		{
			name: "success invalid volume id",
			req: &csi.DeleteVolumeRequest{
				VolumeId: "invalid-volume-name",
			},
			expResp: &csi.DeleteVolumeResponse{},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			awsDriver := NewFakeDriver("", cloud.NewFakeCloudProvider(), NewFakeMounter())
			_, err := awsDriver.DeleteVolume(context.TODO(), tc.req)
			if err != nil {
				srvErr, ok := status.FromError(err)
				if !ok {
					t.Fatalf("Could not get error status code from error: %v", srvErr)
				}
				if srvErr.Code() != tc.expErrCode {
					t.Fatalf("Expected error code %d, got %d", tc.expErrCode, srvErr.Code())
				}
				return
			}
			if tc.expErrCode != codes.OK {
				t.Fatalf("Expected error %v, got no error", tc.expErrCode)
			}
		})
	}
}

func TestPickAvailabilityZone(t *testing.T) {
	testCases := []struct {
		name        string
		requirement *csi.TopologyRequirement
		expZone     string
	}{
		{
			name: "Pick from preferred",
			requirement: &csi.TopologyRequirement{
				Requisite: []*csi.Topology{
					{
						Segments: map[string]string{TopologyKey: expZone},
					},
				},
				Preferred: []*csi.Topology{
					{
						Segments: map[string]string{TopologyKey: expZone},
					},
				},
			},
			expZone: expZone,
		},
		{
			name: "Pick from requisite",
			requirement: &csi.TopologyRequirement{
				Requisite: []*csi.Topology{
					{
						Segments: map[string]string{TopologyKey: expZone},
					},
				},
			},
			expZone: expZone,
		},
		{
			name: "Pick from empty topology",
			requirement: &csi.TopologyRequirement{
				Preferred: []*csi.Topology{{}},
				Requisite: []*csi.Topology{{}},
			},
			expZone: "",
		},
		{
			name:        "Topology Requirement is nil",
			requirement: nil,
			expZone:     "",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actual := pickAvailabilityZone(tc.requirement)
			if actual != tc.expZone {
				t.Fatalf("Expected zone %v, got zone: %v", tc.expZone, actual)
			}
		})
	}
}

func TestCreateSnapshot(t *testing.T) {
	testCases := []struct {
		name            string
		req             *csi.CreateSnapshotRequest
		extraReq        *csi.CreateSnapshotRequest
		expSnapshot     *csi.Snapshot
		expErrCode      codes.Code
		extraExpErrCode codes.Code
	}{
		{
			name: "success normal",
			req: &csi.CreateSnapshotRequest{
				Name:           "test-snapshot",
				Parameters:     nil,
				SourceVolumeId: "vol-test",
			},
			expSnapshot: &csi.Snapshot{
				ReadyToUse: true,
			},
			expErrCode: codes.OK,
		},
		{
			name: "fail no name",
			req: &csi.CreateSnapshotRequest{
				Parameters:     nil,
				SourceVolumeId: "vol-test",
			},
			expSnapshot: nil,
			expErrCode:  codes.InvalidArgument,
		},
		{
			name: "fail same name different volume ID",
			req: &csi.CreateSnapshotRequest{
				Name:           "test-snapshot",
				Parameters:     nil,
				SourceVolumeId: "vol-test",
			},
			extraReq: &csi.CreateSnapshotRequest{
				Name:           "test-snapshot",
				Parameters:     nil,
				SourceVolumeId: "vol-xxx",
			},
			expSnapshot: &csi.Snapshot{
				ReadyToUse: true,
			},
			expErrCode:      codes.OK,
			extraExpErrCode: codes.AlreadyExists,
		},
		{
			name: "success same name same volume ID",
			req: &csi.CreateSnapshotRequest{
				Name:           "test-snapshot",
				Parameters:     nil,
				SourceVolumeId: "vol-test",
			},
			extraReq: &csi.CreateSnapshotRequest{
				Name:           "test-snapshot",
				Parameters:     nil,
				SourceVolumeId: "vol-test",
			},
			expSnapshot: &csi.Snapshot{
				ReadyToUse: true,
			},
			expErrCode:      codes.OK,
			extraExpErrCode: codes.OK,
		},
	}
	for _, tc := range testCases {
		t.Logf("Test case: %s", tc.name)
		awsDriver := NewFakeDriver("", cloud.NewFakeCloudProvider(), NewFakeMounter())
		resp, err := awsDriver.CreateSnapshot(context.TODO(), tc.req)
		if err != nil {
			srvErr, ok := status.FromError(err)
			if !ok {
				t.Fatalf("Could not get error status code from error: %v", srvErr)
			}
			if srvErr.Code() != tc.expErrCode {
				t.Fatalf("Expected error code %d, got %d message %s", tc.expErrCode, srvErr.Code(), srvErr.Message())
			}
			continue
		}
		if tc.expErrCode != codes.OK {
			t.Fatalf("Expected error %v, got no error", tc.expErrCode)
		}
		snap := resp.GetSnapshot()
		if snap == nil && tc.expSnapshot != nil {
			t.Fatalf("Expected snapshot %v, got nil", tc.expSnapshot)
		}
		if tc.extraReq != nil {
			// extraReq is never used in a situation when a new snapshot
			// should be really created: checking the return code is enough
			_, err = awsDriver.CreateSnapshot(context.TODO(), tc.extraReq)
			if err != nil {
				srvErr, ok := status.FromError(err)
				if !ok {
					t.Fatalf("Could not get error status code from error: %v", srvErr)
				}
				if srvErr.Code() != tc.extraExpErrCode {
					t.Fatalf("Expected error code %d, got %d message %s", tc.expErrCode, srvErr.Code(), srvErr.Message())
				}
				continue
			}
			if tc.extraExpErrCode != codes.OK {
				t.Fatalf("Expected error %v, got no error", tc.extraExpErrCode)
			}
		}
	}
}

func TestDeleteSnapshot(t *testing.T) {
	snapReq := &csi.CreateSnapshotRequest{
		Name:           "test-snapshot",
		Parameters:     nil,
		SourceVolumeId: "vol-test",
	}
	testCases := []struct {
		name       string
		req        *csi.DeleteSnapshotRequest
		expErrCode codes.Code
	}{
		{
			name:       "success normal",
			req:        &csi.DeleteSnapshotRequest{},
			expErrCode: codes.OK,
		},
		{
			name: "success not found",
			req: &csi.DeleteSnapshotRequest{
				SnapshotId: "xxx",
			},
			expErrCode: codes.OK,
		},
	}
	for _, tc := range testCases {
		t.Logf("Test case: %s", tc.name)
		awsDriver := NewFakeDriver("", cloud.NewFakeCloudProvider(), NewFakeMounter())
		snapResp, err := awsDriver.CreateSnapshot(context.TODO(), snapReq)
		if err != nil {
			t.Fatalf("Error creating testing snapshot: %v", err)
		}
		if len(tc.req.SnapshotId) == 0 {
			tc.req.SnapshotId = snapResp.Snapshot.SnapshotId
		}
		_, err = awsDriver.DeleteSnapshot(context.TODO(), tc.req)
		if err != nil {
			srvErr, ok := status.FromError(err)
			if !ok {
				t.Fatalf("Could not get error status code from error: %v", srvErr)
			}
			if srvErr.Code() != tc.expErrCode {
				t.Fatalf("Expected error code %d, got %d message %s", tc.expErrCode, srvErr.Code(), srvErr.Message())
			}
			continue
		}
		if tc.expErrCode != codes.OK {
			t.Fatalf("Expected error %v, got no error", tc.expErrCode)
		}
	}
}

func TestControllerPublishVolume(t *testing.T) {
	fakeCloud := cloud.NewFakeCloudProvider()
	stdVolCap := &csi.VolumeCapability{
		AccessType: &csi.VolumeCapability_Mount{
			Mount: &csi.VolumeCapability_MountVolume{},
		},
		AccessMode: &csi.VolumeCapability_AccessMode{
			Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER,
		},
	}
	testCases := []struct {
		name       string
		req        *csi.ControllerPublishVolumeRequest
		expResp    *csi.ControllerPublishVolumeResponse
		expErrCode codes.Code
		setup      func(req *csi.ControllerPublishVolumeRequest)
	}{
		{
			name:    "success normal",
			expResp: &csi.ControllerPublishVolumeResponse{},
			req: &csi.ControllerPublishVolumeRequest{
				NodeId:           fakeCloud.GetMetadata().GetInstanceID(),
				VolumeCapability: stdVolCap,
			},
			// create a fake disk and setup the request
			// parameters appropriately
			setup: func(req *csi.ControllerPublishVolumeRequest) {
				fakeDiskOpts := &cloud.DiskOptions{
					CapacityBytes:    1,
					AvailabilityZone: "az",
				}
				fakeDisk, _ := fakeCloud.CreateDisk(context.TODO(), "vol-test", fakeDiskOpts)
				req.VolumeId = fakeDisk.VolumeID
			},
		},
		{
			name:       "fail no VolumeId",
			req:        &csi.ControllerPublishVolumeRequest{},
			expErrCode: codes.InvalidArgument,
			setup:      func(req *csi.ControllerPublishVolumeRequest) {},
		},
		{
			name:       "fail no NodeId",
			expErrCode: codes.InvalidArgument,
			req: &csi.ControllerPublishVolumeRequest{
				VolumeId: "vol-test",
			},
			setup: func(req *csi.ControllerPublishVolumeRequest) {},
		},
		{
			name:       "fail no VolumeCapability",
			expErrCode: codes.InvalidArgument,
			req: &csi.ControllerPublishVolumeRequest{
				NodeId:   fakeCloud.GetMetadata().GetInstanceID(),
				VolumeId: "vol-test",
			},
			setup: func(req *csi.ControllerPublishVolumeRequest) {},
		},
		{
			name:       "fail invalid VolumeCapability",
			expErrCode: codes.InvalidArgument,
			req: &csi.ControllerPublishVolumeRequest{
				NodeId: fakeCloud.GetMetadata().GetInstanceID(),
				VolumeCapability: &csi.VolumeCapability{
					AccessMode: &csi.VolumeCapability_AccessMode{
						Mode: csi.VolumeCapability_AccessMode_UNKNOWN,
					},
				},
				VolumeId: "vol-test",
			},
			setup: func(req *csi.ControllerPublishVolumeRequest) {},
		},
		{
			name:       "fail instance not found",
			expErrCode: codes.NotFound,
			req: &csi.ControllerPublishVolumeRequest{
				NodeId:           "does-not-exist",
				VolumeId:         "vol-test",
				VolumeCapability: stdVolCap,
			},
			setup: func(req *csi.ControllerPublishVolumeRequest) {},
		},
		{
			name:       "fail volume not found",
			expErrCode: codes.NotFound,
			req: &csi.ControllerPublishVolumeRequest{
				VolumeId:         "does-not-exist",
				NodeId:           fakeCloud.GetMetadata().GetInstanceID(),
				VolumeCapability: stdVolCap,
			},
			setup: func(req *csi.ControllerPublishVolumeRequest) {},
		},
		{
			name:       "fail attach disk with already exists error",
			expErrCode: codes.AlreadyExists,
			req: &csi.ControllerPublishVolumeRequest{
				VolumeId:         "does-not-exist",
				NodeId:           fakeCloud.GetMetadata().GetInstanceID(),
				VolumeCapability: stdVolCap,
			},
			// create a fake disk, attach it and setup the
			// request appropriately
			setup: func(req *csi.ControllerPublishVolumeRequest) {
				fakeDiskOpts := &cloud.DiskOptions{
					CapacityBytes:    1,
					AvailabilityZone: "az",
				}
				fakeDisk, _ := fakeCloud.CreateDisk(context.TODO(), "vol-test", fakeDiskOpts)
				req.VolumeId = fakeDisk.VolumeID
				_, _ = fakeCloud.AttachDisk(context.TODO(), fakeDisk.VolumeID, fakeCloud.GetMetadata().GetInstanceID())
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tc.setup(tc.req)
			awsDriver := NewFakeDriver("", fakeCloud, NewFakeMounter())
			_, err := awsDriver.ControllerPublishVolume(context.TODO(), tc.req)
			if err != nil {
				srvErr, ok := status.FromError(err)
				if !ok {
					t.Fatalf("Could not get error status code from error: %v", srvErr)
				}
				if srvErr.Code() != tc.expErrCode {
					t.Fatalf("Expected error code '%d', got '%d': %s", tc.expErrCode, srvErr.Code(), srvErr.Message())
				}
				return
			}
			if tc.expErrCode != codes.OK {
				t.Fatalf("Expected error %v, got no error", tc.expErrCode)
			}
		})
	}
}

func TestControllerUnpublishVolume(t *testing.T) {
	fakeCloud := cloud.NewFakeCloudProvider()
	testCases := []struct {
		name       string
		req        *csi.ControllerUnpublishVolumeRequest
		expResp    *csi.ControllerUnpublishVolumeResponse
		expErrCode codes.Code
		setup      func(req *csi.ControllerUnpublishVolumeRequest)
	}{
		{
			name:    "success normal",
			expResp: &csi.ControllerUnpublishVolumeResponse{},
			req: &csi.ControllerUnpublishVolumeRequest{
				NodeId: fakeCloud.GetMetadata().GetInstanceID(),
			},
			// create a fake disk, attach it and setup the request
			// parameters appropriately
			setup: func(req *csi.ControllerUnpublishVolumeRequest) {
				fakeDiskOpts := &cloud.DiskOptions{
					CapacityBytes:    1,
					AvailabilityZone: "az",
				}
				fakeDisk, _ := fakeCloud.CreateDisk(context.TODO(), "vol-test", fakeDiskOpts)
				req.VolumeId = fakeDisk.VolumeID
				_, _ = fakeCloud.AttachDisk(context.TODO(), fakeDisk.VolumeID, fakeCloud.GetMetadata().GetInstanceID())
			},
		},
		{
			name:       "fail no VolumeId",
			req:        &csi.ControllerUnpublishVolumeRequest{},
			expErrCode: codes.InvalidArgument,
			setup:      func(req *csi.ControllerUnpublishVolumeRequest) {},
		},
		{
			name:       "fail no NodeId",
			expErrCode: codes.InvalidArgument,
			req: &csi.ControllerUnpublishVolumeRequest{
				VolumeId: "vol-test",
			},
			setup: func(req *csi.ControllerUnpublishVolumeRequest) {},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tc.setup(tc.req)
			awsDriver := NewFakeDriver("", fakeCloud, NewFakeMounter())
			_, err := awsDriver.ControllerUnpublishVolume(context.TODO(), tc.req)
			if err != nil {
				srvErr, ok := status.FromError(err)
				if !ok {
					t.Fatalf("Could not get error status code from error: %v", srvErr)
				}
				if srvErr.Code() != tc.expErrCode {
					t.Fatalf("Expected error code '%d', got '%d': %s", tc.expErrCode, srvErr.Code(), srvErr.Message())
				}
				return
			}
			if tc.expErrCode != codes.OK {
				t.Fatalf("Expected error %v, got no error", tc.expErrCode)
			}
		})
	}
}
