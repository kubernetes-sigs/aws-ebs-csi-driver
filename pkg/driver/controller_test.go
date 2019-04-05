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
	"fmt"
	"math/rand"
	"reflect"
	"testing"
	"time"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/golang/mock/gomock"
	"github.com/kubernetes-sigs/aws-ebs-csi-driver/pkg/cloud"
	"github.com/kubernetes-sigs/aws-ebs-csi-driver/pkg/driver/mocks"
	"github.com/kubernetes-sigs/aws-ebs-csi-driver/pkg/util"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	expZone       = "us-west-2b"
	expFsType     = "ext2"
	expInstanceId = "i-123456789abcdef01"
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
		name     string
		testFunc func(t *testing.T)
	}{
		{
			name: "success normal",
			testFunc: func(t *testing.T) {
				req := &csi.CreateVolumeRequest{
					Name:               "random-vol-name",
					CapacityRange:      stdCapRange,
					VolumeCapabilities: stdVolCap,
					Parameters:         nil,
				}
				expVol := &csi.Volume{
					CapacityBytes: stdVolSize,
					VolumeId:      "vol-test",
					VolumeContext: map[string]string{FsTypeKey: ""},
				}

				ctx := context.Background()

				mockDisk := &cloud.Disk{
					VolumeID:         req.Name,
					AvailabilityZone: expZone,
					FsType:           expVol.VolumeContext[FsTypeKey],
					CapacityGiB:      util.BytesToGiB(stdVolSize),
				}

				mockCtl := gomock.NewController(t)
				defer mockCtl.Finish()

				mockCloud := mocks.NewMockCloud(mockCtl)
				mockCloud.EXPECT().GetDiskByName(gomock.Eq(ctx), gomock.Eq(req.Name), gomock.Eq(stdVolSize)).Return(nil, cloud.ErrNotFound)
				mockCloud.EXPECT().CreateDisk(gomock.Eq(ctx), gomock.Eq(req.Name), gomock.Any()).Return(mockDisk, nil)

				awsDriver := controllerService{cloud: mockCloud}

				if _, err := awsDriver.CreateVolume(ctx, req); err != nil {
					srvErr, ok := status.FromError(err)
					if !ok {
						t.Fatalf("Could not get error status code from error: %v", srvErr)
					}
					t.Fatalf("Unexpected error: %v", srvErr.Code())
				}
			},
		},
		{
			name: "fail no name",
			testFunc: func(t *testing.T) {
				req := &csi.CreateVolumeRequest{
					Name:               "",
					CapacityRange:      stdCapRange,
					VolumeCapabilities: stdVolCap,
					Parameters:         stdParams,
				}
				expErr := codes.InvalidArgument

				ctx := context.Background()

				mockCtl := gomock.NewController(t)
				defer mockCtl.Finish()

				mockCloud := mocks.NewMockCloud(mockCtl)

				awsDriver := controllerService{cloud: mockCloud}

				if _, err := awsDriver.CreateVolume(ctx, req); err != nil {
					srvErr, ok := status.FromError(err)
					if !ok {
						t.Fatalf("Could not get error status code from error: %v", srvErr)
					}
					if srvErr.Code() != expErr {
						t.Fatalf("Expected error code %d, got %d message %s", expErr, srvErr.Code(), srvErr.Message())
					}
				} else {
					t.Fatalf("Expected error got nil")
				}
			},
		},
		{
			name: "success same name and same capacity",
			testFunc: func(t *testing.T) {
				req := &csi.CreateVolumeRequest{
					Name:               "test-vol",
					CapacityRange:      stdCapRange,
					VolumeCapabilities: stdVolCap,
					Parameters:         stdParams,
				}
				extraReq := &csi.CreateVolumeRequest{
					Name:               "test-vol",
					CapacityRange:      stdCapRange,
					VolumeCapabilities: stdVolCap,
					Parameters:         stdParams,
				}
				expVol := &csi.Volume{
					CapacityBytes: stdVolSize,
					VolumeId:      "vol-test",
					VolumeContext: map[string]string{FsTypeKey: ""},
				}

				ctx := context.Background()

				mockDisk := &cloud.Disk{
					VolumeID:         req.Name,
					AvailabilityZone: expZone,
					FsType:           expVol.VolumeContext[FsTypeKey],
					CapacityGiB:      util.BytesToGiB(stdVolSize),
				}

				mockCtl := gomock.NewController(t)
				defer mockCtl.Finish()

				mockCloud := mocks.NewMockCloud(mockCtl)
				mockCloud.EXPECT().GetDiskByName(gomock.Eq(ctx), gomock.Eq(req.Name), gomock.Eq(stdVolSize)).Return(nil, cloud.ErrNotFound)
				mockCloud.EXPECT().CreateDisk(gomock.Eq(ctx), gomock.Eq(req.Name), gomock.Any()).Return(mockDisk, nil)

				awsDriver := controllerService{cloud: mockCloud}

				if _, err := awsDriver.CreateVolume(ctx, req); err != nil {
					srvErr, ok := status.FromError(err)
					if !ok {
						t.Fatalf("Could not get error status code from error: %v", srvErr)
					}
					t.Fatalf("Unexpected error: %v", srvErr.Code())
				}

				// Subsequent call returns the created disk
				mockCloud.EXPECT().GetDiskByName(gomock.Eq(ctx), gomock.Eq(req.Name), gomock.Eq(stdVolSize)).Return(mockDisk, nil)
				resp, err := awsDriver.CreateVolume(ctx, extraReq)
				if err != nil {
					srvErr, ok := status.FromError(err)
					if !ok {
						t.Fatalf("Could not get error status code from error: %v", srvErr)
					}
					t.Fatalf("Unexpected error: %v", srvErr.Code())
				}

				vol := resp.GetVolume()
				if vol == nil && expVol != nil {
					t.Fatalf("Expected volume %v, got nil", expVol)
				}

				if vol.GetCapacityBytes() != expVol.GetCapacityBytes() {
					t.Fatalf("Expected volume capacity bytes: %v, got: %v", expVol.GetCapacityBytes(), vol.GetCapacityBytes())
				}

				for expKey, expVal := range expVol.GetVolumeContext() {
					ctx := vol.GetVolumeContext()
					if gotVal, ok := ctx[expKey]; !ok || gotVal != expVal {
						t.Fatalf("Expected volume context for key %v: %v, got: %v", expKey, expVal, gotVal)
					}
				}

				if expVol.GetVolumeContext() == nil && vol.GetVolumeContext() != nil {
					t.Fatalf("Expected volume context to be nil, got: %#v", vol.GetVolumeContext())
				}
			},
		},
		{
			name: "fail same name and different capacity",
			testFunc: func(t *testing.T) {
				req := &csi.CreateVolumeRequest{
					Name:               "test-vol",
					CapacityRange:      stdCapRange,
					VolumeCapabilities: stdVolCap,
					Parameters:         stdParams,
				}
				extraReq := &csi.CreateVolumeRequest{
					Name:               "test-vol",
					CapacityRange:      &csi.CapacityRange{RequiredBytes: 10000},
					VolumeCapabilities: stdVolCap,
					Parameters:         stdParams,
				}

				ctx := context.Background()

				mockDisk := &cloud.Disk{
					VolumeID:         req.Name,
					AvailabilityZone: expZone,
					FsType:           expFsType,
				}
				volSizeBytes, err := getVolSizeBytes(req)
				if err != nil {
					t.Fatalf("Unable to get volume size bytes for req: %s", err)
				}
				mockDisk.CapacityGiB = util.BytesToGiB(volSizeBytes)

				mockCtl := gomock.NewController(t)
				defer mockCtl.Finish()

				mockCloud := mocks.NewMockCloud(mockCtl)
				mockCloud.EXPECT().GetDiskByName(gomock.Eq(ctx), gomock.Eq(req.Name), gomock.Eq(volSizeBytes)).Return(nil, cloud.ErrNotFound)
				mockCloud.EXPECT().CreateDisk(gomock.Eq(ctx), gomock.Eq(req.Name), gomock.Any()).Return(mockDisk, nil)

				awsDriver := controllerService{cloud: mockCloud}

				_, err = awsDriver.CreateVolume(ctx, req)
				if err != nil {
					srvErr, ok := status.FromError(err)
					if !ok {
						t.Fatalf("Could not get error status code from error: %v", srvErr)
					}
					t.Fatalf("Unexpected error: %v", srvErr.Code())
				}

				extraVolSizeBytes, err := getVolSizeBytes(extraReq)
				if err != nil {
					t.Fatalf("Unable to get volume size bytes for req: %s", err)
				}

				// Subsequent failure
				mockCloud.EXPECT().GetDiskByName(gomock.Eq(ctx), gomock.Eq(extraReq.Name), gomock.Eq(extraVolSizeBytes)).Return(nil, cloud.ErrDiskExistsDiffSize)
				if _, err := awsDriver.CreateVolume(ctx, extraReq); err != nil {
					srvErr, ok := status.FromError(err)
					if !ok {
						t.Fatalf("Could not get error status code from error: %v", srvErr)
					}
					if srvErr.Code() != codes.AlreadyExists {
						t.Fatalf("Expected error code %d, got %d", codes.AlreadyExists, srvErr.Code())
					}
				} else {
					t.Fatalf("Expected error code %d, got nil", codes.AlreadyExists)
				}
			},
		},
		{
			name: "success no capacity range",
			testFunc: func(t *testing.T) {
				req := &csi.CreateVolumeRequest{
					Name:               "test-vol",
					VolumeCapabilities: stdVolCap,
					Parameters:         stdParams,
				}
				expVol := &csi.Volume{
					CapacityBytes: cloud.DefaultVolumeSize,
					VolumeId:      "vol-test",
					VolumeContext: map[string]string{FsTypeKey: ""},
				}

				ctx := context.Background()

				mockDisk := &cloud.Disk{
					VolumeID:         req.Name,
					AvailabilityZone: expZone,
					FsType:           expVol.VolumeContext[FsTypeKey],
				}
				volSizeBytes, err := getVolSizeBytes(req)
				if err != nil {
					t.Fatalf("Unable to get volume size bytes for req: %s", err)
				}
				mockDisk.CapacityGiB = util.BytesToGiB(volSizeBytes)

				mockCtl := gomock.NewController(t)
				defer mockCtl.Finish()

				mockCloud := mocks.NewMockCloud(mockCtl)
				mockCloud.EXPECT().GetDiskByName(gomock.Eq(ctx), gomock.Eq(req.Name), gomock.Eq(volSizeBytes)).Return(nil, cloud.ErrNotFound)
				mockCloud.EXPECT().CreateDisk(gomock.Eq(ctx), gomock.Eq(req.Name), gomock.Any()).Return(mockDisk, nil)

				awsDriver := controllerService{cloud: mockCloud}

				resp, err := awsDriver.CreateVolume(ctx, req)
				if err != nil {
					srvErr, ok := status.FromError(err)
					if !ok {
						t.Fatalf("Could not get error status code from error: %v", srvErr)
					}
					t.Fatalf("Unexpected error: %v", srvErr.Code())
				}

				vol := resp.GetVolume()
				if vol == nil && expVol != nil {
					t.Fatalf("Expected volume %v, got nil", expVol)
				}

				if vol.GetCapacityBytes() != expVol.GetCapacityBytes() {
					t.Fatalf("Expected volume capacity bytes: %v, got: %v", expVol.GetCapacityBytes(), vol.GetCapacityBytes())
				}

				for expKey, expVal := range expVol.GetVolumeContext() {
					ctx := vol.GetVolumeContext()
					if gotVal, ok := ctx[expKey]; !ok || gotVal != expVal {
						t.Fatalf("Expected volume context for key %v: %v, got: %v", expKey, expVal, gotVal)
					}
				}

				if expVol.GetVolumeContext() == nil && vol.GetVolumeContext() != nil {
					t.Fatalf("Expected volume context to be nil, got: %#v", vol.GetVolumeContext())
				}
			},
		},
		{
			name: "success with correct round up",
			testFunc: func(t *testing.T) {
				req := &csi.CreateVolumeRequest{
					Name:               "vol-test",
					CapacityRange:      &csi.CapacityRange{RequiredBytes: 1073741825},
					VolumeCapabilities: stdVolCap,
					Parameters:         nil,
				}
				expVol := &csi.Volume{
					CapacityBytes: 2147483648, // 1 GiB + 1 byte = 2 GiB
					VolumeId:      "vol-test",
					VolumeContext: map[string]string{FsTypeKey: ""},
				}

				ctx := context.Background()

				mockDisk := &cloud.Disk{
					VolumeID:         req.Name,
					AvailabilityZone: expZone,
					FsType:           expVol.VolumeContext[FsTypeKey],
				}
				volSizeBytes, err := getVolSizeBytes(req)
				if err != nil {
					t.Fatalf("Unable to get volume size bytes for req: %s", err)
				}
				mockDisk.CapacityGiB = util.BytesToGiB(volSizeBytes)

				mockCtl := gomock.NewController(t)
				defer mockCtl.Finish()

				mockCloud := mocks.NewMockCloud(mockCtl)
				mockCloud.EXPECT().GetDiskByName(gomock.Eq(ctx), gomock.Eq(req.Name), gomock.Eq(volSizeBytes)).Return(nil, cloud.ErrNotFound)
				mockCloud.EXPECT().CreateDisk(gomock.Eq(ctx), gomock.Eq(req.Name), gomock.Any()).Return(mockDisk, nil)

				awsDriver := controllerService{cloud: mockCloud}

				resp, err := awsDriver.CreateVolume(ctx, req)
				if err != nil {
					srvErr, ok := status.FromError(err)
					if !ok {
						t.Fatalf("Could not get error status code from error: %v", srvErr)
					}
					t.Fatalf("Unexpected error: %v", srvErr.Code())
				}

				vol := resp.GetVolume()
				if vol == nil && expVol != nil {
					t.Fatalf("Expected volume %v, got nil", expVol)
				}

				if vol.GetCapacityBytes() != expVol.GetCapacityBytes() {
					t.Fatalf("Expected volume capacity bytes: %v, got: %v", expVol.GetCapacityBytes(), vol.GetCapacityBytes())
				}
			},
		},
		{
			name: "success with fstype parameter",
			testFunc: func(t *testing.T) {
				req := &csi.CreateVolumeRequest{
					Name:               "vol-test",
					CapacityRange:      stdCapRange,
					VolumeCapabilities: stdVolCap,
					Parameters:         map[string]string{FsTypeKey: defaultFsType},
				}
				expVol := &csi.Volume{
					CapacityBytes: stdVolSize,
					VolumeId:      "vol-test",
					VolumeContext: map[string]string{FsTypeKey: defaultFsType},
				}

				ctx := context.Background()

				mockDisk := &cloud.Disk{
					VolumeID:         req.Name,
					AvailabilityZone: expZone,
					FsType:           expVol.VolumeContext[FsTypeKey],
					CapacityGiB:      util.BytesToGiB(stdVolSize),
				}

				mockCtl := gomock.NewController(t)
				defer mockCtl.Finish()

				mockCloud := mocks.NewMockCloud(mockCtl)
				mockCloud.EXPECT().GetDiskByName(gomock.Eq(ctx), gomock.Eq(req.Name), gomock.Eq(stdVolSize)).Return(nil, cloud.ErrNotFound)
				mockCloud.EXPECT().CreateDisk(gomock.Eq(ctx), gomock.Eq(req.Name), gomock.Any()).Return(mockDisk, nil)

				awsDriver := controllerService{cloud: mockCloud}

				resp, err := awsDriver.CreateVolume(ctx, req)
				if err != nil {
					srvErr, ok := status.FromError(err)
					if !ok {
						t.Fatalf("Could not get error status code from error: %v", srvErr)
					}
					t.Fatalf("Unexpected error: %v", srvErr.Code())
				}

				vol := resp.GetVolume()
				if vol == nil && expVol != nil {
					t.Fatalf("Expected volume %v, got nil", expVol)
				}

				if vol.GetCapacityBytes() != expVol.GetCapacityBytes() {
					t.Fatalf("Expected volume capacity bytes: %v, got: %v", expVol.GetCapacityBytes(), vol.GetCapacityBytes())
				}

				for expKey, expVal := range expVol.GetVolumeContext() {
					ctx := vol.GetVolumeContext()
					if gotVal, ok := ctx[expKey]; !ok || gotVal != expVal {
						t.Fatalf("Expected volume context for key %v: %v, got: %v", expKey, expVal, gotVal)
					}
				}

				if expVol.GetVolumeContext() == nil && vol.GetVolumeContext() != nil {
					t.Fatalf("Expected volume context to be nil, got: %#v", vol.GetVolumeContext())
				}
			},
		},
		{
			name: "success with volume type io1",
			testFunc: func(t *testing.T) {
				req := &csi.CreateVolumeRequest{
					Name:               "vol-test",
					CapacityRange:      stdCapRange,
					VolumeCapabilities: stdVolCap,
					Parameters: map[string]string{
						VolumeTypeKey: cloud.VolumeTypeIO1,
						IopsPerGBKey:  "5",
					},
				}
				expVol := &csi.Volume{
					CapacityBytes: stdVolSize,
					VolumeId:      "vol-test",
					VolumeContext: map[string]string{FsTypeKey: ""},
				}

				ctx := context.Background()

				mockDisk := &cloud.Disk{
					VolumeID:         req.Name,
					AvailabilityZone: expZone,
					FsType:           expVol.VolumeContext[FsTypeKey],
					CapacityGiB:      util.BytesToGiB(stdVolSize),
				}

				mockCtl := gomock.NewController(t)
				defer mockCtl.Finish()

				mockCloud := mocks.NewMockCloud(mockCtl)
				mockCloud.EXPECT().GetDiskByName(gomock.Eq(ctx), gomock.Eq(req.Name), gomock.Eq(stdVolSize)).Return(nil, cloud.ErrNotFound)
				mockCloud.EXPECT().CreateDisk(gomock.Eq(ctx), gomock.Eq(req.Name), gomock.Any()).Return(mockDisk, nil)

				awsDriver := controllerService{cloud: mockCloud}

				if _, err := awsDriver.CreateVolume(ctx, req); err != nil {
					srvErr, ok := status.FromError(err)
					if !ok {
						t.Fatalf("Could not get error status code from error: %v", srvErr)
					}
					t.Fatalf("Unexpected error: %v", srvErr.Code())
				}
			},
		},
		{
			name: "success with volume type sc1",
			testFunc: func(t *testing.T) {
				req := &csi.CreateVolumeRequest{
					Name:               "vol-test",
					CapacityRange:      stdCapRange,
					VolumeCapabilities: stdVolCap,
					Parameters: map[string]string{
						VolumeTypeKey: cloud.VolumeTypeSC1,
					},
				}
				expVol := &csi.Volume{
					CapacityBytes: stdVolSize,
					VolumeId:      "vol-test",
					VolumeContext: map[string]string{FsTypeKey: ""},
				}

				ctx := context.Background()

				mockDisk := &cloud.Disk{
					VolumeID:         req.Name,
					AvailabilityZone: expZone,
					FsType:           expVol.VolumeContext[FsTypeKey],
					CapacityGiB:      util.BytesToGiB(stdVolSize),
				}

				mockCtl := gomock.NewController(t)
				defer mockCtl.Finish()

				mockCloud := mocks.NewMockCloud(mockCtl)
				mockCloud.EXPECT().GetDiskByName(gomock.Eq(ctx), gomock.Eq(req.Name), gomock.Eq(stdVolSize)).Return(nil, cloud.ErrNotFound)
				mockCloud.EXPECT().CreateDisk(gomock.Eq(ctx), gomock.Eq(req.Name), gomock.Any()).Return(mockDisk, nil)

				awsDriver := controllerService{cloud: mockCloud}

				if _, err := awsDriver.CreateVolume(ctx, req); err != nil {
					srvErr, ok := status.FromError(err)
					if !ok {
						t.Fatalf("Could not get error status code from error: %v", srvErr)
					}
					t.Fatalf("Unexpected error: %v", srvErr.Code())
				}
			},
		},
		{
			name: "success with volume encryption",
			testFunc: func(t *testing.T) {
				req := &csi.CreateVolumeRequest{
					Name:               "vol-test",
					CapacityRange:      stdCapRange,
					VolumeCapabilities: stdVolCap,
					Parameters: map[string]string{
						EncryptedKey: "true",
					},
				}
				expVol := &csi.Volume{
					CapacityBytes: stdVolSize,
					VolumeId:      "vol-test",
					VolumeContext: map[string]string{FsTypeKey: ""},
				}

				ctx := context.Background()

				mockDisk := &cloud.Disk{
					VolumeID:         req.Name,
					AvailabilityZone: expZone,
					FsType:           expVol.VolumeContext[FsTypeKey],
					CapacityGiB:      util.BytesToGiB(stdVolSize),
				}

				mockCtl := gomock.NewController(t)
				defer mockCtl.Finish()

				mockCloud := mocks.NewMockCloud(mockCtl)
				mockCloud.EXPECT().GetDiskByName(gomock.Eq(ctx), gomock.Eq(req.Name), gomock.Eq(stdVolSize)).Return(nil, cloud.ErrNotFound)
				mockCloud.EXPECT().CreateDisk(gomock.Eq(ctx), gomock.Eq(req.Name), gomock.Any()).Return(mockDisk, nil)

				awsDriver := controllerService{cloud: mockCloud}

				if _, err := awsDriver.CreateVolume(ctx, req); err != nil {
					srvErr, ok := status.FromError(err)
					if !ok {
						t.Fatalf("Could not get error status code from error: %v", srvErr)
					}
					t.Fatalf("Unexpected error: %v", srvErr.Code())
				}
			},
		},
		{
			name: "success with volume encryption with KMS key",
			testFunc: func(t *testing.T) {
				req := &csi.CreateVolumeRequest{
					Name:               "vol-test",
					CapacityRange:      stdCapRange,
					VolumeCapabilities: stdVolCap,
					Parameters: map[string]string{
						EncryptedKey: "true",
						KmsKeyIdKey:  "arn:aws:kms:us-east-1:012345678910:key/abcd1234-a123-456a-a12b-a123b4cd56ef",
					},
				}
				expVol := &csi.Volume{
					CapacityBytes: stdVolSize,
					VolumeId:      "vol-test",
					VolumeContext: map[string]string{FsTypeKey: ""},
				}

				ctx := context.Background()

				mockDisk := &cloud.Disk{
					VolumeID:         req.Name,
					AvailabilityZone: expZone,
					FsType:           expVol.VolumeContext[FsTypeKey],
					CapacityGiB:      util.BytesToGiB(stdVolSize),
				}

				mockCtl := gomock.NewController(t)
				defer mockCtl.Finish()

				mockCloud := mocks.NewMockCloud(mockCtl)
				mockCloud.EXPECT().GetDiskByName(gomock.Eq(ctx), gomock.Eq(req.Name), gomock.Eq(stdVolSize)).Return(nil, cloud.ErrNotFound)
				mockCloud.EXPECT().CreateDisk(gomock.Eq(ctx), gomock.Eq(req.Name), gomock.Any()).Return(mockDisk, nil)

				awsDriver := controllerService{cloud: mockCloud}

				if _, err := awsDriver.CreateVolume(ctx, req); err != nil {
					srvErr, ok := status.FromError(err)
					if !ok {
						t.Fatalf("Could not get error status code from error: %v", srvErr)
					}
					t.Fatalf("Unexpected error: %v", srvErr.Code())
				}
			},
		},
		{
			name: "success when volume exists and contains VolumeContext and AccessibleTopology",
			testFunc: func(t *testing.T) {
				req := &csi.CreateVolumeRequest{
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
				}
				extraReq := &csi.CreateVolumeRequest{
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
				}
				expVol := &csi.Volume{
					CapacityBytes: stdVolSize,
					VolumeId:      "vol-test",
					VolumeContext: map[string]string{FsTypeKey: expFsType},
					AccessibleTopology: []*csi.Topology{
						{
							Segments: map[string]string{TopologyKey: expZone},
						},
					},
				}

				ctx := context.Background()

				mockDisk := &cloud.Disk{
					VolumeID:         req.Name,
					AvailabilityZone: expZone,
					FsType:           expVol.VolumeContext[FsTypeKey],
					CapacityGiB:      util.BytesToGiB(stdVolSize),
				}

				mockCtl := gomock.NewController(t)
				defer mockCtl.Finish()

				mockCloud := mocks.NewMockCloud(mockCtl)
				mockCloud.EXPECT().GetDiskByName(gomock.Eq(ctx), gomock.Eq(req.Name), gomock.Eq(stdVolSize)).Return(nil, cloud.ErrNotFound)
				mockCloud.EXPECT().CreateDisk(gomock.Eq(ctx), gomock.Eq(req.Name), gomock.Any()).Return(mockDisk, nil)

				awsDriver := controllerService{cloud: mockCloud}

				if _, err := awsDriver.CreateVolume(ctx, req); err != nil {
					srvErr, ok := status.FromError(err)
					if !ok {
						t.Fatalf("Could not get error status code from error: %v", srvErr)
					}
					t.Fatalf("Unexpected error: %v", srvErr.Code())
				}

				mockCloud.EXPECT().GetDiskByName(gomock.Eq(ctx), gomock.Eq(req.Name), gomock.Eq(stdVolSize)).Return(mockDisk, nil)
				resp, err := awsDriver.CreateVolume(ctx, extraReq)
				if err != nil {
					srvErr, ok := status.FromError(err)
					if !ok {
						t.Fatalf("Could not get error status code from error: %v", srvErr)
					}
					t.Fatalf("Unexpected error: %v", srvErr.Code())
				}

				vol := resp.GetVolume()
				if vol == nil && expVol != nil {
					t.Fatalf("Expected volume %v, got nil", expVol)
				}

				for expKey, expVal := range expVol.GetVolumeContext() {
					ctx := vol.GetVolumeContext()
					if gotVal, ok := ctx[expKey]; !ok || gotVal != expVal {
						t.Fatalf("Expected volume context for key %v: %v, got: %v", expKey, expVal, gotVal)
					}
				}

				if expVol.GetVolumeContext() == nil && vol.GetVolumeContext() != nil {
					t.Fatalf("Expected volume context to be nil, got: %#v", vol.GetVolumeContext())
				}

				if expVol.GetAccessibleTopology() != nil {
					if !reflect.DeepEqual(expVol.GetAccessibleTopology(), vol.GetAccessibleTopology()) {
						t.Fatalf("Expected AccessibleTopology to be %+v, got: %+v", expVol.GetAccessibleTopology(), vol.GetAccessibleTopology())
					}
				}
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, tc.testFunc)
	}
}

func TestDeleteVolume(t *testing.T) {
	testCases := []struct {
		name     string
		testFunc func(t *testing.T)
	}{
		{
			name: "success normal",
			testFunc: func(t *testing.T) {
				req := &csi.DeleteVolumeRequest{
					VolumeId: "vol-test",
				}
				expResp := &csi.DeleteVolumeResponse{}

				ctx := context.Background()
				mockCtl := gomock.NewController(t)
				defer mockCtl.Finish()

				mockCloud := mocks.NewMockCloud(mockCtl)
				mockCloud.EXPECT().DeleteDisk(gomock.Eq(ctx), gomock.Eq(req.VolumeId)).Return(true, nil)
				awsDriver := controllerService{cloud: mockCloud}
				resp, err := awsDriver.DeleteVolume(ctx, req)
				if err != nil {
					srvErr, ok := status.FromError(err)
					if !ok {
						t.Fatalf("Could not get error status code from error: %v", srvErr)
					}
					t.Fatalf("Unexpected error: %v", srvErr.Code())
				}
				if !reflect.DeepEqual(resp, expResp) {
					t.Fatalf("Expected resp to be %+v, got: %+v", expResp, resp)
				}
			},
		},
		{
			name: "success invalid volume id",
			testFunc: func(t *testing.T) {
				req := &csi.DeleteVolumeRequest{
					VolumeId: "invalid-volume-name",
				}
				expResp := &csi.DeleteVolumeResponse{}

				ctx := context.Background()
				mockCtl := gomock.NewController(t)
				defer mockCtl.Finish()

				mockCloud := mocks.NewMockCloud(mockCtl)
				mockCloud.EXPECT().DeleteDisk(gomock.Eq(ctx), gomock.Eq(req.VolumeId)).Return(true, nil)
				awsDriver := controllerService{cloud: mockCloud}
				resp, err := awsDriver.DeleteVolume(ctx, req)
				if err != nil {
					srvErr, ok := status.FromError(err)
					if !ok {
						t.Fatalf("Could not get error status code from error: %v", srvErr)
					}
					t.Fatalf("Unexpected error: %v", srvErr.Code())
				}
				if !reflect.DeepEqual(resp, expResp) {
					t.Fatalf("Expected resp to be %+v, got: %+v", expResp, resp)
				}
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, tc.testFunc)
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
		name     string
		testFunc func(t *testing.T)
	}{
		{
			name: "success normal",
			testFunc: func(t *testing.T) {
				req := &csi.CreateSnapshotRequest{
					Name:           "test-snapshot",
					Parameters:     nil,
					SourceVolumeId: "vol-test",
				}
				expSnapshot := &csi.Snapshot{
					ReadyToUse: true,
				}
				expErrCode := codes.OK

				ctx := context.Background()
				mockSnapshot := &cloud.Snapshot{
					SnapshotID:     fmt.Sprintf("snapshot-%d", rand.New(rand.NewSource(time.Now().UnixNano())).Uint64()),
					SourceVolumeID: req.SourceVolumeId,
					Size:           1,
					CreationTime:   time.Now(),
				}
				mockCtl := gomock.NewController(t)
				defer mockCtl.Finish()

				mockCloud := mocks.NewMockCloud(mockCtl)
				mockCloud.EXPECT().CreateSnapshot(gomock.Eq(ctx), gomock.Eq(req.SourceVolumeId), gomock.Any()).Return(mockSnapshot, nil)
				mockCloud.EXPECT().GetSnapshotByName(gomock.Eq(ctx), gomock.Eq(req.GetName())).Return(nil, cloud.ErrNotFound)

				awsDriver := controllerService{cloud: mockCloud}
				resp, err := awsDriver.CreateSnapshot(context.Background(), req)
				if err != nil {
					srvErr, ok := status.FromError(err)
					if !ok {
						t.Fatalf("Could not get error status code from error: %v", srvErr)
					}
					if srvErr.Code() != expErrCode {
						t.Fatalf("Expected error code %d, got %d message %s", expErrCode, srvErr.Code(), srvErr.Message())
					}
					return
				}
				if expErrCode != codes.OK {
					t.Fatalf("Expected error %v, got no error", expErrCode)
				}
				snap := resp.GetSnapshot()
				if snap == nil && expSnapshot != nil {
					t.Fatalf("Expected snapshot %v, got nil", expSnapshot)
				}
			},
		},
		{
			name: "fail no name",
			testFunc: func(t *testing.T) {
				req := &csi.CreateSnapshotRequest{
					Parameters:     nil,
					SourceVolumeId: "vol-test",
				}
				expErrCode := codes.InvalidArgument

				mockCtl := gomock.NewController(t)
				defer mockCtl.Finish()

				mockCloud := mocks.NewMockCloud(mockCtl)

				awsDriver := controllerService{cloud: mockCloud}
				if _, err := awsDriver.CreateSnapshot(context.Background(), req); err != nil {
					srvErr, ok := status.FromError(err)
					if !ok {
						t.Fatalf("Could not get error status code from error: %v", srvErr)
					}
					if srvErr.Code() != expErrCode {
						t.Fatalf("Expected error code %d, got %d message %s", expErrCode, srvErr.Code(), srvErr.Message())
					}
					return
				}
			},
		},
		{
			name: "fail same name different volume ID",
			testFunc: func(t *testing.T) {
				req := &csi.CreateSnapshotRequest{
					Name:           "test-snapshot",
					Parameters:     nil,
					SourceVolumeId: "vol-test",
				}
				extraReq := &csi.CreateSnapshotRequest{
					Name:           "test-snapshot",
					Parameters:     nil,
					SourceVolumeId: "vol-xxx",
				}
				expSnapshot := &csi.Snapshot{
					ReadyToUse: true,
				}
				expErrCode := codes.OK
				extraExpErrCode := codes.AlreadyExists

				ctx := context.Background()
				mockSnapshot := &cloud.Snapshot{
					SnapshotID:     fmt.Sprintf("snapshot-%d", rand.New(rand.NewSource(time.Now().UnixNano())).Uint64()),
					SourceVolumeID: req.SourceVolumeId,
					Size:           1,
					CreationTime:   time.Now(),
				}
				mockCtl := gomock.NewController(t)
				defer mockCtl.Finish()

				mockCloud := mocks.NewMockCloud(mockCtl)
				mockCloud.EXPECT().GetSnapshotByName(gomock.Eq(ctx), gomock.Eq(req.GetName())).Return(nil, cloud.ErrNotFound)
				mockCloud.EXPECT().CreateSnapshot(gomock.Eq(ctx), gomock.Eq(req.SourceVolumeId), gomock.Any()).Return(mockSnapshot, nil)

				awsDriver := controllerService{cloud: mockCloud}
				resp, err := awsDriver.CreateSnapshot(context.Background(), req)
				if err != nil {
					srvErr, ok := status.FromError(err)
					if !ok {
						t.Fatalf("Could not get error status code from error: %v", srvErr)
					}
				}
				if expErrCode != codes.OK {
					t.Fatalf("Expected error %v, got no error", expErrCode)
				}
				snap := resp.GetSnapshot()
				if snap == nil && expSnapshot != nil {
					t.Fatalf("Expected snapshot %v, got nil", expSnapshot)
				}

				mockCloud.EXPECT().GetSnapshotByName(gomock.Eq(ctx), gomock.Eq(extraReq.GetName())).Return(mockSnapshot, nil)
				_, err = awsDriver.CreateSnapshot(ctx, extraReq)
				if err != nil {
					srvErr, ok := status.FromError(err)
					if !ok {
						t.Fatalf("Could not get error status code from error: %v", srvErr)
					}
					if srvErr.Code() != extraExpErrCode {
						t.Fatalf("Expected error code %d, got %d message %s", expErrCode, srvErr.Code(), srvErr.Message())
					}
					return
				}
				if extraExpErrCode != codes.OK {
					t.Fatalf("Expected error %v, got no error", extraExpErrCode)
				}
			},
		},
		{
			name: "success same name same volume ID",
			testFunc: func(t *testing.T) {
				req := &csi.CreateSnapshotRequest{
					Name:           "test-snapshot",
					Parameters:     nil,
					SourceVolumeId: "vol-test",
				}
				extraReq := &csi.CreateSnapshotRequest{
					Name:           "test-snapshot",
					Parameters:     nil,
					SourceVolumeId: "vol-test",
				}
				expSnapshot := &csi.Snapshot{
					ReadyToUse: true,
				}
				expErrCode := codes.OK
				extraExpErrCode := codes.OK

				ctx := context.Background()
				mockSnapshot := &cloud.Snapshot{
					SnapshotID:     fmt.Sprintf("snapshot-%d", rand.New(rand.NewSource(time.Now().UnixNano())).Uint64()),
					SourceVolumeID: req.SourceVolumeId,
					Size:           1,
					CreationTime:   time.Now(),
				}
				mockCtl := gomock.NewController(t)
				defer mockCtl.Finish()

				mockCloud := mocks.NewMockCloud(mockCtl)
				mockCloud.EXPECT().GetSnapshotByName(gomock.Eq(ctx), gomock.Eq(req.GetName())).Return(nil, cloud.ErrNotFound)
				mockCloud.EXPECT().CreateSnapshot(gomock.Eq(ctx), gomock.Eq(req.SourceVolumeId), gomock.Any()).Return(mockSnapshot, nil)

				awsDriver := controllerService{cloud: mockCloud}
				resp, err := awsDriver.CreateSnapshot(context.Background(), req)
				if err != nil {
					srvErr, ok := status.FromError(err)
					if !ok {
						t.Fatalf("Could not get error status code from error: %v", srvErr)
					}
				}
				if expErrCode != codes.OK {
					t.Fatalf("Expected error %v, got no error", expErrCode)
				}
				snap := resp.GetSnapshot()
				if snap == nil && expSnapshot != nil {
					t.Fatalf("Expected snapshot %v, got nil", expSnapshot)
				}

				mockCloud.EXPECT().GetSnapshotByName(gomock.Eq(ctx), gomock.Eq(extraReq.GetName())).Return(mockSnapshot, nil)
				_, err = awsDriver.CreateSnapshot(ctx, extraReq)
				if err != nil {
					srvErr, ok := status.FromError(err)
					if !ok {
						t.Fatalf("Could not get error status code from error: %v", srvErr)
					}
					if srvErr.Code() != extraExpErrCode {
						t.Fatalf("Expected error code %d, got %d message %s", expErrCode, srvErr.Code(), srvErr.Message())
					}
					return
				}
				if extraExpErrCode != codes.OK {
					t.Fatalf("Expected error %v, got no error", extraExpErrCode)
				}
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, tc.testFunc)
	}
}

func TestDeleteSnapshot(t *testing.T) {
	testCases := []struct {
		name     string
		testFunc func(t *testing.T)
	}{
		{
			name: "success normal",
			testFunc: func(t *testing.T) {
				expErrCode := codes.OK
				ctx := context.Background()

				mockCtl := gomock.NewController(t)
				defer mockCtl.Finish()
				mockCloud := mocks.NewMockCloud(mockCtl)

				awsDriver := controllerService{cloud: mockCloud}

				req := &csi.DeleteSnapshotRequest{
					SnapshotId: "xxx",
				}

				mockCloud.EXPECT().DeleteSnapshot(gomock.Eq(ctx), gomock.Eq("xxx")).Return(true, nil)
				if _, err := awsDriver.DeleteSnapshot(ctx, req); err != nil {
					srvErr, ok := status.FromError(err)
					if !ok {
						t.Fatalf("Could not get error status code from error: %v", srvErr)
					}
					if srvErr.Code() != expErrCode {
						t.Fatalf("Expected error code %d, got %d message %s", expErrCode, srvErr.Code(), srvErr.Message())
					}
					return
				}
				if expErrCode != codes.OK {
					t.Fatalf("Expected error %v, got no error", expErrCode)
				}
			},
		},
		{
			name: "success not found",
			testFunc: func(t *testing.T) {
				expErrCode := codes.OK
				ctx := context.Background()

				mockCtl := gomock.NewController(t)
				defer mockCtl.Finish()
				mockCloud := mocks.NewMockCloud(mockCtl)

				awsDriver := controllerService{cloud: mockCloud}

				req := &csi.DeleteSnapshotRequest{
					SnapshotId: "xxx",
				}

				mockCloud.EXPECT().DeleteSnapshot(gomock.Eq(ctx), gomock.Eq("xxx")).Return(false, cloud.ErrNotFound)
				if _, err := awsDriver.DeleteSnapshot(ctx, req); err != nil {
					srvErr, ok := status.FromError(err)
					if !ok {
						t.Fatalf("Could not get error status code from error: %v", srvErr)
					}
					if srvErr.Code() != expErrCode {
						t.Fatalf("Expected error code %d, got %d message %s", expErrCode, srvErr.Code(), srvErr.Message())
					}
					return
				}
				if expErrCode != codes.OK {
					t.Fatalf("Expected error %v, got no error", expErrCode)
				}
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, tc.testFunc)
	}
}

func TestControllerPublishVolume(t *testing.T) {
	stdVolCap := &csi.VolumeCapability{
		AccessType: &csi.VolumeCapability_Mount{
			Mount: &csi.VolumeCapability_MountVolume{},
		},
		AccessMode: &csi.VolumeCapability_AccessMode{
			Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER,
		},
	}
	expDevicePath := "/dev/xvda"

	testCases := []struct {
		name     string
		testFunc func(t *testing.T)
	}{
		{
			name: "success normal",
			testFunc: func(t *testing.T) {
				req := &csi.ControllerPublishVolumeRequest{
					NodeId:           expInstanceId,
					VolumeCapability: stdVolCap,
					VolumeId:         "vol-test",
				}
				expResp := &csi.ControllerPublishVolumeResponse{
					PublishContext: map[string]string{DevicePathKey: expDevicePath},
				}

				ctx := context.Background()

				mockCtl := gomock.NewController(t)
				defer mockCtl.Finish()

				mockCloud := mocks.NewMockCloud(mockCtl)
				mockCloud.EXPECT().IsExistInstance(gomock.Eq(ctx), gomock.Eq(req.NodeId)).Return(true)
				mockCloud.EXPECT().GetDiskByID(gomock.Eq(ctx), gomock.Any()).Return(&cloud.Disk{}, nil)
				mockCloud.EXPECT().AttachDisk(gomock.Eq(ctx), gomock.Any(), gomock.Eq(req.NodeId)).Return(expDevicePath, nil)

				awsDriver := controllerService{cloud: mockCloud}
				resp, err := awsDriver.ControllerPublishVolume(ctx, req)
				if err != nil {
					srvErr, ok := status.FromError(err)
					if !ok {
						t.Fatalf("Could not get error status code from error: %v", srvErr)
					}
					t.Fatalf("Unexpected error: %v", srvErr.Code())
				}

				if !reflect.DeepEqual(resp, expResp) {
					t.Fatalf("Expected resp to be %+v, got: %+v", expResp, resp)
				}
			},
		},
		{
			name: "fail no VolumeId",
			testFunc: func(t *testing.T) {
				req := &csi.ControllerPublishVolumeRequest{}
				expErrCode := codes.InvalidArgument

				ctx := context.Background()

				mockCtl := gomock.NewController(t)
				defer mockCtl.Finish()

				mockCloud := mocks.NewMockCloud(mockCtl)

				awsDriver := controllerService{cloud: mockCloud}
				if _, err := awsDriver.ControllerPublishVolume(ctx, req); err != nil {
					srvErr, ok := status.FromError(err)
					if !ok {
						t.Fatalf("Could not get error status code from error: %v", srvErr)
					}
					if srvErr.Code() != expErrCode {
						t.Fatalf("Expected error code %d, got %d message %s", expErrCode, srvErr.Code(), srvErr.Message())
					}
					return
				}
				if expErrCode != codes.OK {
					t.Fatalf("Expected error %v, got no error", expErrCode)
				}
			},
		},
		{
			name: "fail no NodeId",
			testFunc: func(t *testing.T) {
				req := &csi.ControllerPublishVolumeRequest{
					VolumeId: "vol-test",
				}
				expErrCode := codes.InvalidArgument

				ctx := context.Background()

				mockCtl := gomock.NewController(t)
				defer mockCtl.Finish()

				mockCloud := mocks.NewMockCloud(mockCtl)

				awsDriver := controllerService{cloud: mockCloud}
				if _, err := awsDriver.ControllerPublishVolume(ctx, req); err != nil {
					srvErr, ok := status.FromError(err)
					if !ok {
						t.Fatalf("Could not get error status code from error: %v", srvErr)
					}
					if srvErr.Code() != expErrCode {
						t.Fatalf("Expected error code %d, got %d message %s", expErrCode, srvErr.Code(), srvErr.Message())
					}
					return
				}
				if expErrCode != codes.OK {
					t.Fatalf("Expected error %v, got no error", expErrCode)
				}
			},
		},
		{
			name: "fail no VolumeCapability",
			testFunc: func(t *testing.T) {
				req := &csi.ControllerPublishVolumeRequest{
					NodeId:   expInstanceId,
					VolumeId: "vol-test",
				}
				expErrCode := codes.InvalidArgument

				ctx := context.Background()

				mockCtl := gomock.NewController(t)
				defer mockCtl.Finish()

				mockCloud := mocks.NewMockCloud(mockCtl)

				awsDriver := controllerService{cloud: mockCloud}
				if _, err := awsDriver.ControllerPublishVolume(ctx, req); err != nil {
					srvErr, ok := status.FromError(err)
					if !ok {
						t.Fatalf("Could not get error status code from error: %v", srvErr)
					}
					if srvErr.Code() != expErrCode {
						t.Fatalf("Expected error code %d, got %d message %s", expErrCode, srvErr.Code(), srvErr.Message())
					}
					return
				}
				if expErrCode != codes.OK {
					t.Fatalf("Expected error %v, got no error", expErrCode)
				}
			},
		},
		{
			name: "fail invalid VolumeCapability",
			testFunc: func(t *testing.T) {
				req := &csi.ControllerPublishVolumeRequest{
					NodeId: expInstanceId,
					VolumeCapability: &csi.VolumeCapability{
						AccessMode: &csi.VolumeCapability_AccessMode{
							Mode: csi.VolumeCapability_AccessMode_UNKNOWN,
						},
					},
					VolumeId: "vol-test",
				}
				expErrCode := codes.InvalidArgument

				ctx := context.Background()

				mockCtl := gomock.NewController(t)
				defer mockCtl.Finish()

				mockCloud := mocks.NewMockCloud(mockCtl)

				awsDriver := controllerService{cloud: mockCloud}
				if _, err := awsDriver.ControllerPublishVolume(ctx, req); err != nil {
					srvErr, ok := status.FromError(err)
					if !ok {
						t.Fatalf("Could not get error status code from error: %v", srvErr)
					}
					if srvErr.Code() != expErrCode {
						t.Fatalf("Expected error code %d, got %d message %s", expErrCode, srvErr.Code(), srvErr.Message())
					}
					return
				}
				if expErrCode != codes.OK {
					t.Fatalf("Expected error %v, got no error", expErrCode)
				}
			},
		},
		{
			name: "fail instance not found",
			testFunc: func(t *testing.T) {
				req := &csi.ControllerPublishVolumeRequest{
					NodeId:           "does-not-exist",
					VolumeId:         "vol-test",
					VolumeCapability: stdVolCap,
				}
				expErrCode := codes.NotFound

				ctx := context.Background()

				mockCtl := gomock.NewController(t)
				defer mockCtl.Finish()

				mockCloud := mocks.NewMockCloud(mockCtl)
				mockCloud.EXPECT().IsExistInstance(gomock.Eq(ctx), gomock.Eq(req.NodeId)).Return(false)

				awsDriver := controllerService{cloud: mockCloud}
				if _, err := awsDriver.ControllerPublishVolume(ctx, req); err != nil {
					srvErr, ok := status.FromError(err)
					if !ok {
						t.Fatalf("Could not get error status code from error: %v", srvErr)
					}
					if srvErr.Code() != expErrCode {
						t.Fatalf("Expected error code %d, got %d message %s", expErrCode, srvErr.Code(), srvErr.Message())
					}
					return
				}
				if expErrCode != codes.OK {
					t.Fatalf("Expected error %v, got no error", expErrCode)
				}
			},
		},
		{
			name: "fail volume not found",
			testFunc: func(t *testing.T) {
				req := &csi.ControllerPublishVolumeRequest{
					VolumeId:         "does-not-exist",
					NodeId:           expInstanceId,
					VolumeCapability: stdVolCap,
				}
				expErrCode := codes.NotFound

				ctx := context.Background()

				mockCtl := gomock.NewController(t)
				defer mockCtl.Finish()

				mockCloud := mocks.NewMockCloud(mockCtl)
				mockCloud.EXPECT().IsExistInstance(gomock.Eq(ctx), gomock.Eq(req.NodeId)).Return(true)
				mockCloud.EXPECT().GetDiskByID(gomock.Eq(ctx), gomock.Any()).Return(nil, cloud.ErrNotFound)

				awsDriver := controllerService{cloud: mockCloud}
				if _, err := awsDriver.ControllerPublishVolume(ctx, req); err != nil {
					srvErr, ok := status.FromError(err)
					if !ok {
						t.Fatalf("Could not get error status code from error: %v", srvErr)
					}
					if srvErr.Code() != expErrCode {
						t.Fatalf("Expected error code %d, got %d message %s", expErrCode, srvErr.Code(), srvErr.Message())
					}
					return
				}
				if expErrCode != codes.OK {
					t.Fatalf("Expected error %v, got no error", expErrCode)
				}
			},
		},
		{
			name: "fail attach disk with already exists error",
			testFunc: func(t *testing.T) {
				req := &csi.ControllerPublishVolumeRequest{
					VolumeId:         "does-not-exist",
					NodeId:           expInstanceId,
					VolumeCapability: stdVolCap,
				}
				expErrCode := codes.AlreadyExists

				ctx := context.Background()

				mockCtl := gomock.NewController(t)
				defer mockCtl.Finish()

				mockCloud := mocks.NewMockCloud(mockCtl)
				mockCloud.EXPECT().IsExistInstance(gomock.Eq(ctx), gomock.Eq(req.NodeId)).Return(true)
				mockCloud.EXPECT().GetDiskByID(gomock.Eq(ctx), gomock.Any()).Return(&cloud.Disk{}, nil)
				mockCloud.EXPECT().AttachDisk(gomock.Eq(ctx), gomock.Any(), gomock.Eq(req.NodeId)).Return("", cloud.ErrAlreadyExists)

				awsDriver := controllerService{cloud: mockCloud}
				if _, err := awsDriver.ControllerPublishVolume(ctx, req); err != nil {
					srvErr, ok := status.FromError(err)
					if !ok {
						t.Fatalf("Could not get error status code from error: %v", srvErr)
					}
					if srvErr.Code() != expErrCode {
						t.Fatalf("Expected error code %d, got %d message %s", expErrCode, srvErr.Code(), srvErr.Message())
					}
					return
				}
				if expErrCode != codes.OK {
					t.Fatalf("Expected error %v, got no error", expErrCode)
				}
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, tc.testFunc)
	}
}

func TestControllerUnpublishVolume(t *testing.T) {
	testCases := []struct {
		name     string
		setup    func(req *csi.ControllerUnpublishVolumeRequest)
		testFunc func(t *testing.T)
	}{
		{
			name: "success normal",
			testFunc: func(t *testing.T) {
				req := &csi.ControllerUnpublishVolumeRequest{
					NodeId:   expInstanceId,
					VolumeId: "vol-test",
				}
				expResp := &csi.ControllerUnpublishVolumeResponse{}

				ctx := context.Background()

				mockCtl := gomock.NewController(t)
				defer mockCtl.Finish()

				mockCloud := mocks.NewMockCloud(mockCtl)
				mockCloud.EXPECT().DetachDisk(gomock.Eq(ctx), req.VolumeId, req.NodeId).Return(nil)

				awsDriver := controllerService{cloud: mockCloud}
				resp, err := awsDriver.ControllerUnpublishVolume(ctx, req)
				if err != nil {
					srvErr, ok := status.FromError(err)
					if !ok {
						t.Fatalf("Could not get error status code from error: %v", srvErr)
					}
					t.Fatalf("Unexpected error: %v", srvErr.Code())
				}

				if !reflect.DeepEqual(resp, expResp) {
					t.Fatalf("Expected resp to be %+v, got: %+v", expResp, resp)
				}
			},
		},
		{
			name: "fail no VolumeId",
			testFunc: func(t *testing.T) {
				req := &csi.ControllerUnpublishVolumeRequest{}
				expErrCode := codes.InvalidArgument

				ctx := context.Background()

				mockCtl := gomock.NewController(t)
				defer mockCtl.Finish()

				mockCloud := mocks.NewMockCloud(mockCtl)

				awsDriver := controllerService{cloud: mockCloud}
				if _, err := awsDriver.ControllerUnpublishVolume(ctx, req); err != nil {
					srvErr, ok := status.FromError(err)
					if !ok {
						t.Fatalf("Could not get error status code from error: %v", srvErr)
					}
					if srvErr.Code() != expErrCode {
						t.Fatalf("Expected error code %d, got %d message %s", expErrCode, srvErr.Code(), srvErr.Message())
					}
					return
				}
				if expErrCode != codes.OK {
					t.Fatalf("Expected error %v, got no error", expErrCode)
				}
			},
		},
		{
			name: "fail no NodeId",
			testFunc: func(t *testing.T) {
				req := &csi.ControllerUnpublishVolumeRequest{
					VolumeId: "vol-test",
				}
				expErrCode := codes.InvalidArgument

				ctx := context.Background()

				mockCtl := gomock.NewController(t)
				defer mockCtl.Finish()

				mockCloud := mocks.NewMockCloud(mockCtl)

				awsDriver := controllerService{cloud: mockCloud}
				if _, err := awsDriver.ControllerUnpublishVolume(ctx, req); err != nil {
					srvErr, ok := status.FromError(err)
					if !ok {
						t.Fatalf("Could not get error status code from error: %v", srvErr)
					}
					if srvErr.Code() != expErrCode {
						t.Fatalf("Expected error code %d, got %d message %s", expErrCode, srvErr.Code(), srvErr.Message())
					}
					return
				}
				if expErrCode != codes.OK {
					t.Fatalf("Expected error %v, got no error", expErrCode)
				}
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, tc.testFunc)
	}
}
