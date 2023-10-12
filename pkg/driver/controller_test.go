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
	"context"
	"errors"
	"fmt"
	"math/rand"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/arn"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/golang/mock/gomock"
	"github.com/kubernetes-sigs/aws-ebs-csi-driver/pkg/cloud"
	"github.com/kubernetes-sigs/aws-ebs-csi-driver/pkg/driver/internal"
	"github.com/kubernetes-sigs/aws-ebs-csi-driver/pkg/util"
	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	expZone       = "us-west-2b"
	expInstanceID = "i-123456789abcdef01"
	expDevicePath = "/dev/xvda"
)

func TestNewControllerService(t *testing.T) {

	var (
		cloudObj   cloud.Cloud
		testErr    = errors.New("test error")
		testRegion = "test-region"

		getNewCloudFunc = func(expectedRegion string, _ bool) func(region string, awsSdkDebugLog bool, userAgentExtra string) (cloud.Cloud, error) {
			return func(region string, awsSdkDebugLog bool, userAgentExtra string) (cloud.Cloud, error) {
				if region != expectedRegion {
					t.Fatalf("expected region %q but got %q", expectedRegion, region)
				}
				return cloudObj, nil
			}
		}
	)

	testCases := []struct {
		name                  string
		region                string
		newCloudFunc          func(string, bool, string) (cloud.Cloud, error)
		newMetadataFuncErrors bool
		expectPanic           bool
	}{
		{
			name:         "AWS_REGION variable set, newCloud does not error",
			region:       "foo",
			newCloudFunc: getNewCloudFunc("foo", false),
		},
		{
			name:   "AWS_REGION variable set, newCloud errors",
			region: "foo",
			newCloudFunc: func(region string, awsSdkDebugLog bool, userAgentExtra string) (cloud.Cloud, error) {
				return nil, testErr
			},
			expectPanic: true,
		},
		{
			name:         "AWS_REGION variable not set, newMetadata does not error",
			newCloudFunc: getNewCloudFunc(testRegion, false),
		},
		{
			name:                  "AWS_REGION variable not set, newMetadata errors",
			newCloudFunc:          getNewCloudFunc(testRegion, false),
			newMetadataFuncErrors: true,
			expectPanic:           true,
		},
	}

	driverOptions := &DriverOptions{
		endpoint: "test",
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			oldNewCloudFunc := NewCloudFunc
			defer func() { NewCloudFunc = oldNewCloudFunc }()
			NewCloudFunc = tc.newCloudFunc

			if tc.region == "" {
				mockCtl := gomock.NewController(t)
				defer mockCtl.Finish()
				mockMetadataService := cloud.NewMockMetadataService(mockCtl)

				oldNewMetadataFunc := NewMetadataFunc
				defer func() { NewMetadataFunc = oldNewMetadataFunc }()
				NewMetadataFunc = func(cloud.EC2MetadataClient, cloud.KubernetesAPIClient, string) (cloud.MetadataService, error) {
					if tc.newMetadataFuncErrors {
						return nil, testErr
					}
					return mockMetadataService, nil
				}

				if !tc.newMetadataFuncErrors {
					mockMetadataService.EXPECT().GetRegion().Return(testRegion)
				}
			} else {
				os.Setenv("AWS_REGION", tc.region)
				defer os.Unsetenv("AWS_REGION")
			}

			if tc.expectPanic {
				defer func() {
					if r := recover(); r == nil {
						t.Errorf("The code did not panic")
					}
				}()
			}

			controllerSvc := newControllerService(driverOptions)

			if controllerSvc.cloud != cloudObj {
				t.Fatalf("expected cloud attribute to be equal to instantiated cloud object")
			}
			if !reflect.DeepEqual(controllerSvc.driverOptions, driverOptions) {
				t.Fatalf("expected driverOptions attribute to be equal to input")
			}
		})
	}
}

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
	invalidVolCap := []*csi.VolumeCapability{
		{
			AccessMode: &csi.VolumeCapability_AccessMode{
				Mode: csi.VolumeCapability_AccessMode_MULTI_NODE_SINGLE_WRITER,
			},
		},
	}
	stdVolSize := int64(5 * 1024 * 1024 * 1024)
	stdCapRange := &csi.CapacityRange{RequiredBytes: stdVolSize}
	stdParams := map[string]string{}
	rawOutpostArn := "arn:aws:outposts:us-west-2:111111111111:outpost/op-0aaa000a0aaaa00a0"
	strippedOutpostArn, _ := arn.Parse(strings.ReplaceAll(rawOutpostArn, "outpost/", ""))

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

				ctx := context.Background()

				mockDisk := &cloud.Disk{
					VolumeID:         req.Name,
					AvailabilityZone: expZone,
					CapacityGiB:      util.BytesToGiB(stdVolSize),
				}

				mockCtl := gomock.NewController(t)
				defer mockCtl.Finish()

				mockCloud := cloud.NewMockCloud(mockCtl)
				mockCloud.EXPECT().CreateDisk(gomock.Eq(ctx), gomock.Eq(req.Name), gomock.Any()).Return(mockDisk, nil)

				awsDriver := controllerService{
					cloud:         mockCloud,
					inFlight:      internal.NewInFlight(),
					driverOptions: &DriverOptions{},
				}

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
			name: "success outposts",
			testFunc: func(t *testing.T) {
				outpostArn := strippedOutpostArn
				req := &csi.CreateVolumeRequest{
					Name:               "test-vol",
					CapacityRange:      stdCapRange,
					VolumeCapabilities: stdVolCap,
					Parameters:         map[string]string{},
					AccessibilityRequirements: &csi.TopologyRequirement{
						Requisite: []*csi.Topology{
							{
								Segments: map[string]string{
									TopologyKey:     expZone,
									AwsAccountIDKey: outpostArn.AccountID,
									AwsOutpostIDKey: outpostArn.Resource,
									AwsRegionKey:    outpostArn.Region,
									AwsPartitionKey: outpostArn.Partition,
								},
							},
						},
					},
				}
				expVol := &csi.Volume{
					CapacityBytes: stdVolSize,
					VolumeId:      "vol-test",
					VolumeContext: map[string]string{},
					AccessibleTopology: []*csi.Topology{
						{
							Segments: map[string]string{
								TopologyKey:     expZone,
								AwsAccountIDKey: outpostArn.AccountID,
								AwsOutpostIDKey: outpostArn.Resource,
								AwsRegionKey:    outpostArn.Region,
								AwsPartitionKey: outpostArn.Partition,
							},
						},
					},
				}

				ctx := context.Background()

				mockDisk := &cloud.Disk{
					VolumeID:         req.Name,
					AvailabilityZone: expZone,
					CapacityGiB:      util.BytesToGiB(stdVolSize),
					OutpostArn:       outpostArn.String(),
				}

				mockCtl := gomock.NewController(t)
				defer mockCtl.Finish()

				mockCloud := cloud.NewMockCloud(mockCtl)
				mockCloud.EXPECT().CreateDisk(gomock.Eq(ctx), gomock.Eq(req.Name), gomock.Any()).Return(mockDisk, nil)

				awsDriver := controllerService{
					cloud:         mockCloud,
					inFlight:      internal.NewInFlight(),
					driverOptions: &DriverOptions{},
				}

				resp, err := awsDriver.CreateVolume(ctx, req)
				if err != nil {
					srvErr, ok := status.FromError(err)
					if !ok {
						t.Fatalf("Could not get error status code from error: %v", srvErr)
					}
					t.Fatalf("Unexpected error: %v", srvErr.Code())
				}

				// mockCloud.EXPECT().GetDiskByName(gomock.Eq(ctx), gomock.Eq(req.Name), gomock.Eq(stdVolSize)).Return(mockDisk, nil)
				vol := resp.GetVolume()
				if vol == nil {
					t.Fatalf("Expected volume %v, got nil", expVol)
				}

				for expKey, expVal := range expVol.GetVolumeContext() {
					ctx := vol.GetVolumeContext()
					if gotVal, ok := ctx[expKey]; !ok || gotVal != expVal {
						t.Fatalf("Expected volume context for key %v: %v, got: %v", expKey, expVal, gotVal)
					}
				}

				if expVol.GetAccessibleTopology() != nil {
					if !reflect.DeepEqual(expVol.GetAccessibleTopology(), vol.GetAccessibleTopology()) {
						t.Fatalf("Expected AccessibleTopology to be %+v, got: %+v", expVol.GetAccessibleTopology(), vol.GetAccessibleTopology())
					}
				}
			},
		},
		{
			name: "restore snapshot",
			testFunc: func(t *testing.T) {
				req := &csi.CreateVolumeRequest{
					Name:               "random-vol-name",
					CapacityRange:      stdCapRange,
					VolumeCapabilities: stdVolCap,
					Parameters:         nil,
					VolumeContentSource: &csi.VolumeContentSource{
						Type: &csi.VolumeContentSource_Snapshot{
							Snapshot: &csi.VolumeContentSource_SnapshotSource{
								SnapshotId: "snapshot-id",
							},
						},
					},
				}

				ctx := context.Background()

				mockDisk := &cloud.Disk{
					VolumeID:         req.Name,
					AvailabilityZone: expZone,
					CapacityGiB:      util.BytesToGiB(stdVolSize),
					SnapshotID:       "snapshot-id",
				}

				mockCtl := gomock.NewController(t)
				defer mockCtl.Finish()

				mockCloud := cloud.NewMockCloud(mockCtl)
				mockCloud.EXPECT().CreateDisk(gomock.Eq(ctx), gomock.Eq(req.Name), gomock.Any()).Return(mockDisk, nil)

				awsDriver := controllerService{
					cloud:         mockCloud,
					inFlight:      internal.NewInFlight(),
					driverOptions: &DriverOptions{},
				}

				rsp, err := awsDriver.CreateVolume(ctx, req)
				if err != nil {
					srvErr, ok := status.FromError(err)
					if !ok {
						t.Fatalf("Could not get error status code from error: %v", srvErr)
					}
					t.Fatalf("Unexpected error: %v", srvErr.Code())
				}

				snapshotID := ""
				if rsp.Volume != nil && rsp.Volume.ContentSource != nil && rsp.Volume.ContentSource.GetSnapshot() != nil {
					snapshotID = rsp.Volume.ContentSource.GetSnapshot().SnapshotId
				}
				if rsp.Volume.ContentSource.GetSnapshot().SnapshotId != "snapshot-id" {
					t.Errorf("Unexpected snapshot ID: %q", snapshotID)
				}
			},
		},
		{
			name: "restore snapshot, volume already exists",
			testFunc: func(t *testing.T) {
				req := &csi.CreateVolumeRequest{
					Name:               "random-vol-name",
					CapacityRange:      stdCapRange,
					VolumeCapabilities: stdVolCap,
					Parameters:         nil,
					VolumeContentSource: &csi.VolumeContentSource{
						Type: &csi.VolumeContentSource_Snapshot{
							Snapshot: &csi.VolumeContentSource_SnapshotSource{
								SnapshotId: "snapshot-id",
							},
						},
					},
				}

				ctx := context.Background()

				mockDisk := &cloud.Disk{
					VolumeID:         req.Name,
					AvailabilityZone: expZone,
					CapacityGiB:      util.BytesToGiB(stdVolSize),
					SnapshotID:       "snapshot-id",
				}

				mockCtl := gomock.NewController(t)
				defer mockCtl.Finish()

				mockCloud := cloud.NewMockCloud(mockCtl)
				mockCloud.EXPECT().CreateDisk(gomock.Eq(ctx), gomock.Eq(req.Name), gomock.Any()).Return(mockDisk, nil)

				awsDriver := controllerService{
					cloud:         mockCloud,
					inFlight:      internal.NewInFlight(),
					driverOptions: &DriverOptions{},
				}

				rsp, err := awsDriver.CreateVolume(ctx, req)
				if err != nil {
					srvErr, ok := status.FromError(err)
					if !ok {
						t.Fatalf("Could not get error status code from error: %v", srvErr)
					}
					t.Fatalf("Unexpected error: %v", srvErr.Code())
				}

				snapshotID := ""
				if rsp.Volume != nil && rsp.Volume.ContentSource != nil && rsp.Volume.ContentSource.GetSnapshot() != nil {
					snapshotID = rsp.Volume.ContentSource.GetSnapshot().SnapshotId
				}
				if rsp.Volume.ContentSource.GetSnapshot().SnapshotId != "snapshot-id" {
					t.Errorf("Unexpected snapshot ID: %q", snapshotID)
				}
			},
		},
		{
			name: "restore snapshot, volume already exists with different snapshot ID",
			testFunc: func(t *testing.T) {
				req := &csi.CreateVolumeRequest{
					Name:               "random-vol-name",
					CapacityRange:      stdCapRange,
					VolumeCapabilities: stdVolCap,
					Parameters:         nil,
					VolumeContentSource: &csi.VolumeContentSource{
						Type: &csi.VolumeContentSource_Snapshot{
							Snapshot: &csi.VolumeContentSource_SnapshotSource{
								SnapshotId: "snapshot-id",
							},
						},
					},
				}

				ctx := context.Background()

				mockCtl := gomock.NewController(t)
				defer mockCtl.Finish()

				mockCloud := cloud.NewMockCloud(mockCtl)
				mockCloud.EXPECT().CreateDisk(gomock.Eq(ctx), gomock.Eq(req.Name), gomock.Any()).Return(nil, cloud.ErrIdempotentParameterMismatch)

				awsDriver := controllerService{
					cloud:         mockCloud,
					inFlight:      internal.NewInFlight(),
					driverOptions: &DriverOptions{},
				}

				_, err := awsDriver.CreateVolume(ctx, req)
				checkExpectedErrorCode(t, err, codes.AlreadyExists)
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

				ctx := context.Background()

				mockCtl := gomock.NewController(t)
				defer mockCtl.Finish()

				mockCloud := cloud.NewMockCloud(mockCtl)

				awsDriver := controllerService{
					cloud:         mockCloud,
					inFlight:      internal.NewInFlight(),
					driverOptions: &DriverOptions{},
				}

				if _, err := awsDriver.CreateVolume(ctx, req); err != nil {
					srvErr, ok := status.FromError(err)
					if !ok {
						t.Fatalf("Could not get error status code from error: %v", srvErr)
					}
					if srvErr.Code() != codes.InvalidArgument {
						t.Fatalf("Expected error code %d, got %d message %s", codes.InvalidArgument, srvErr.Code(), srvErr.Message())
					}
				} else {
					t.Fatalf("Expected error %v, got no error", codes.InvalidArgument)
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
					VolumeId:      "test-vol",
					VolumeContext: map[string]string{},
				}

				ctx := context.Background()

				mockDisk := &cloud.Disk{
					VolumeID:         req.Name,
					AvailabilityZone: expZone,
					CapacityGiB:      util.BytesToGiB(stdVolSize),
				}

				mockCtl := gomock.NewController(t)
				defer mockCtl.Finish()

				mockCloud := cloud.NewMockCloud(mockCtl)
				mockCloud.EXPECT().CreateDisk(gomock.Eq(ctx), gomock.Eq(req.Name), gomock.Any()).Return(mockDisk, nil)

				awsDriver := controllerService{
					cloud:         mockCloud,
					inFlight:      internal.NewInFlight(),
					driverOptions: &DriverOptions{},
				}

				if _, err := awsDriver.CreateVolume(ctx, req); err != nil {
					srvErr, ok := status.FromError(err)
					if !ok {
						t.Fatalf("Could not get error status code from error: %v", srvErr)
					}
					t.Fatalf("Unexpected error: %v", srvErr.Code())
				}

				// Subsequent call returns the created disk
				mockCloud.EXPECT().CreateDisk(gomock.Eq(ctx), gomock.Eq(req.Name), gomock.Any()).Return(mockDisk, nil)
				resp, err := awsDriver.CreateVolume(ctx, extraReq)
				if err != nil {
					srvErr, ok := status.FromError(err)
					if !ok {
						t.Fatalf("Could not get error status code from error: %v", srvErr)
					}
					t.Fatalf("Unexpected error: %v", srvErr.Code())
				}

				vol := resp.GetVolume()
				if vol == nil {
					t.Fatalf("Expected volume %v, got nil", expVol)
				}

				if vol.GetCapacityBytes() != expVol.GetCapacityBytes() {
					t.Fatalf("Expected volume capacity bytes: %v, got: %v", expVol.GetCapacityBytes(), vol.GetCapacityBytes())
				}

				if vol.GetVolumeId() != expVol.GetVolumeId() {
					t.Fatalf("Expected volume id: %v, got: %v", expVol.GetVolumeId(), vol.GetVolumeId())
				}

				if expVol.GetAccessibleTopology() != nil {
					if !reflect.DeepEqual(expVol.GetAccessibleTopology(), vol.GetAccessibleTopology()) {
						t.Fatalf("Expected AccessibleTopology to be %+v, got: %+v", expVol.GetAccessibleTopology(), vol.GetAccessibleTopology())
					}
				}

				for expKey, expVal := range expVol.GetVolumeContext() {
					ctx := vol.GetVolumeContext()
					if gotVal, ok := ctx[expKey]; !ok || gotVal != expVal {
						t.Fatalf("Expected volume context for key %v: %v, got: %v", expKey, expVal, gotVal)
					}
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
				}
				volSizeBytes, err := getVolSizeBytes(req)
				if err != nil {
					t.Fatalf("Unable to get volume size bytes for req: %s", err)
				}
				mockDisk.CapacityGiB = util.BytesToGiB(volSizeBytes)

				mockCtl := gomock.NewController(t)
				defer mockCtl.Finish()

				mockCloud := cloud.NewMockCloud(mockCtl)
				mockCloud.EXPECT().CreateDisk(gomock.Eq(ctx), gomock.Eq(req.Name), gomock.Any()).Return(mockDisk, nil)

				awsDriver := controllerService{
					cloud:         mockCloud,
					inFlight:      internal.NewInFlight(),
					driverOptions: &DriverOptions{},
				}

				_, err = awsDriver.CreateVolume(ctx, req)
				if err != nil {
					srvErr, ok := status.FromError(err)
					if !ok {
						t.Fatalf("Could not get error status code from error: %v", srvErr)
					}
					t.Fatalf("Unexpected error: %v", srvErr.Code())
				}

				// Subsequent failure
				mockCloud.EXPECT().CreateDisk(gomock.Eq(ctx), gomock.Eq(extraReq.Name), gomock.Any()).Return(nil, cloud.ErrIdempotentParameterMismatch)
				if _, err := awsDriver.CreateVolume(ctx, extraReq); err != nil {
					srvErr, ok := status.FromError(err)
					if !ok {
						t.Fatalf("Could not get error status code from error: %v", srvErr)
					}
					if srvErr.Code() != codes.AlreadyExists {
						t.Fatalf("Expected error code %d, got %d", codes.AlreadyExists, srvErr.Code())
					}
				} else {
					t.Fatalf("Expected error %v, got no error", codes.AlreadyExists)
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
					VolumeContext: map[string]string{},
				}

				ctx := context.Background()

				mockDisk := &cloud.Disk{
					VolumeID:         req.Name,
					AvailabilityZone: expZone,
					CapacityGiB:      util.BytesToGiB(cloud.DefaultVolumeSize),
				}

				mockCtl := gomock.NewController(t)
				defer mockCtl.Finish()

				mockCloud := cloud.NewMockCloud(mockCtl)
				mockCloud.EXPECT().CreateDisk(gomock.Eq(ctx), gomock.Eq(req.Name), gomock.Any()).Return(mockDisk, nil)

				awsDriver := controllerService{
					cloud:         mockCloud,
					inFlight:      internal.NewInFlight(),
					driverOptions: &DriverOptions{},
				}

				resp, err := awsDriver.CreateVolume(ctx, req)
				if err != nil {
					srvErr, ok := status.FromError(err)
					if !ok {
						t.Fatalf("Could not get error status code from error: %v", srvErr)
					}
					t.Fatalf("Unexpected error: %v", srvErr.Code())
				}

				vol := resp.GetVolume()
				if vol == nil {
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
					VolumeContext: map[string]string{},
				}

				ctx := context.Background()

				mockDisk := &cloud.Disk{
					VolumeID:         req.Name,
					AvailabilityZone: expZone,
					CapacityGiB:      util.BytesToGiB(expVol.CapacityBytes),
				}

				mockCtl := gomock.NewController(t)
				defer mockCtl.Finish()

				mockCloud := cloud.NewMockCloud(mockCtl)
				mockCloud.EXPECT().CreateDisk(gomock.Eq(ctx), gomock.Eq(req.Name), gomock.Any()).Return(mockDisk, nil)

				awsDriver := controllerService{
					cloud:         mockCloud,
					inFlight:      internal.NewInFlight(),
					driverOptions: &DriverOptions{},
				}

				resp, err := awsDriver.CreateVolume(ctx, req)
				if err != nil {
					srvErr, ok := status.FromError(err)
					if !ok {
						t.Fatalf("Could not get error status code from error: %v", srvErr)
					}
					t.Fatalf("Unexpected error: %v", srvErr.Code())
				}

				vol := resp.GetVolume()
				if vol == nil {
					t.Fatalf("Expected volume %v, got nil", expVol)
				}

				if vol.GetCapacityBytes() != expVol.GetCapacityBytes() {
					t.Fatalf("Expected volume capacity bytes: %v, got: %v", expVol.GetCapacityBytes(), vol.GetCapacityBytes())
				}
			},
		},
		{
			name: "success with volume type gp3",
			testFunc: func(t *testing.T) {
				// iops 5000 requires at least 10GB
				volSize := int64(20 * 1024 * 1024 * 1024)
				capRange := &csi.CapacityRange{RequiredBytes: volSize}
				req := &csi.CreateVolumeRequest{
					Name:               "vol-test",
					CapacityRange:      capRange,
					VolumeCapabilities: stdVolCap,
					Parameters: map[string]string{
						VolumeTypeKey: cloud.VolumeTypeGP3,
						IopsKey:       "5000",
						ThroughputKey: "250",
					},
				}

				ctx := context.Background()

				mockDisk := &cloud.Disk{
					VolumeID:         req.Name,
					AvailabilityZone: expZone,
					CapacityGiB:      util.BytesToGiB(volSize),
				}

				mockCtl := gomock.NewController(t)
				defer mockCtl.Finish()

				mockCloud := cloud.NewMockCloud(mockCtl)
				mockCloud.EXPECT().CreateDisk(gomock.Eq(ctx), gomock.Eq(req.Name), gomock.Any()).Return(mockDisk, nil)

				awsDriver := controllerService{
					cloud:         mockCloud,
					inFlight:      internal.NewInFlight(),
					driverOptions: &DriverOptions{},
				}

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
			name: "success with volume type io1 using iopsPerGB",
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

				ctx := context.Background()

				mockDisk := &cloud.Disk{
					VolumeID:         req.Name,
					AvailabilityZone: expZone,
					CapacityGiB:      util.BytesToGiB(stdVolSize),
				}

				mockCtl := gomock.NewController(t)
				defer mockCtl.Finish()

				mockCloud := cloud.NewMockCloud(mockCtl)
				mockCloud.EXPECT().CreateDisk(gomock.Eq(ctx), gomock.Eq(req.Name), gomock.Any()).Return(mockDisk, nil)

				awsDriver := controllerService{
					cloud:         mockCloud,
					inFlight:      internal.NewInFlight(),
					driverOptions: &DriverOptions{},
				}

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
			name: "success with volume type io1 using iops",
			testFunc: func(t *testing.T) {
				req := &csi.CreateVolumeRequest{
					Name:               "vol-test",
					CapacityRange:      stdCapRange,
					VolumeCapabilities: stdVolCap,
					Parameters: map[string]string{
						VolumeTypeKey: cloud.VolumeTypeIO1,
						IopsKey:       "5",
					},
				}

				ctx := context.Background()

				mockDisk := &cloud.Disk{
					VolumeID:         req.Name,
					AvailabilityZone: expZone,
					CapacityGiB:      util.BytesToGiB(stdVolSize),
				}

				mockCtl := gomock.NewController(t)
				defer mockCtl.Finish()

				mockCloud := cloud.NewMockCloud(mockCtl)
				mockCloud.EXPECT().CreateDisk(gomock.Eq(ctx), gomock.Eq(req.Name), gomock.Any()).Return(mockDisk, nil)

				awsDriver := controllerService{
					cloud:         mockCloud,
					inFlight:      internal.NewInFlight(),
					driverOptions: &DriverOptions{},
				}

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
			name: "success with volume type io2 using iopsPerGB",
			testFunc: func(t *testing.T) {
				req := &csi.CreateVolumeRequest{
					Name:               "vol-test",
					CapacityRange:      stdCapRange,
					VolumeCapabilities: stdVolCap,
					Parameters: map[string]string{
						VolumeTypeKey: cloud.VolumeTypeIO2,
						IopsPerGBKey:  "5",
					},
				}

				ctx := context.Background()

				mockDisk := &cloud.Disk{
					VolumeID:         req.Name,
					AvailabilityZone: expZone,
					CapacityGiB:      util.BytesToGiB(stdVolSize),
				}

				mockCtl := gomock.NewController(t)
				defer mockCtl.Finish()

				mockCloud := cloud.NewMockCloud(mockCtl)
				mockCloud.EXPECT().CreateDisk(gomock.Eq(ctx), gomock.Eq(req.Name), gomock.Any()).Return(mockDisk, nil)

				awsDriver := controllerService{
					cloud:         mockCloud,
					inFlight:      internal.NewInFlight(),
					driverOptions: &DriverOptions{},
				}

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
			name: "success with volume type io2 using iops",
			testFunc: func(t *testing.T) {
				req := &csi.CreateVolumeRequest{
					Name:               "vol-test",
					CapacityRange:      stdCapRange,
					VolumeCapabilities: stdVolCap,
					Parameters: map[string]string{
						VolumeTypeKey: cloud.VolumeTypeIO2,
						IopsKey:       "5",
					},
				}

				ctx := context.Background()

				mockDisk := &cloud.Disk{
					VolumeID:         req.Name,
					AvailabilityZone: expZone,
					CapacityGiB:      util.BytesToGiB(stdVolSize),
				}

				mockCtl := gomock.NewController(t)
				defer mockCtl.Finish()

				mockCloud := cloud.NewMockCloud(mockCtl)
				mockCloud.EXPECT().CreateDisk(gomock.Eq(ctx), gomock.Eq(req.Name), gomock.Any()).Return(mockDisk, nil)

				awsDriver := controllerService{
					cloud:         mockCloud,
					inFlight:      internal.NewInFlight(),
					driverOptions: &DriverOptions{},
				}

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

				ctx := context.Background()

				mockDisk := &cloud.Disk{
					VolumeID:         req.Name,
					AvailabilityZone: expZone,
					CapacityGiB:      util.BytesToGiB(stdVolSize),
				}

				mockCtl := gomock.NewController(t)
				defer mockCtl.Finish()

				mockCloud := cloud.NewMockCloud(mockCtl)
				mockCloud.EXPECT().CreateDisk(gomock.Eq(ctx), gomock.Eq(req.Name), gomock.Any()).Return(mockDisk, nil)

				awsDriver := controllerService{
					cloud:         mockCloud,
					inFlight:      internal.NewInFlight(),
					driverOptions: &DriverOptions{},
				}

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
			name: "success with volume type standard",
			testFunc: func(t *testing.T) {
				req := &csi.CreateVolumeRequest{
					Name:               "vol-test",
					CapacityRange:      stdCapRange,
					VolumeCapabilities: stdVolCap,
					Parameters: map[string]string{
						VolumeTypeKey: cloud.VolumeTypeStandard,
					},
				}

				ctx := context.Background()

				mockDisk := &cloud.Disk{
					VolumeID:         req.Name,
					AvailabilityZone: expZone,
					CapacityGiB:      util.BytesToGiB(stdVolSize),
				}

				mockCtl := gomock.NewController(t)
				defer mockCtl.Finish()

				mockCloud := cloud.NewMockCloud(mockCtl)
				mockCloud.EXPECT().CreateDisk(gomock.Eq(ctx), gomock.Eq(req.Name), gomock.Any()).Return(mockDisk, nil)

				awsDriver := controllerService{
					cloud:         mockCloud,
					inFlight:      internal.NewInFlight(),
					driverOptions: &DriverOptions{},
				}

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

				ctx := context.Background()

				mockDisk := &cloud.Disk{
					VolumeID:         req.Name,
					AvailabilityZone: expZone,
					CapacityGiB:      util.BytesToGiB(stdVolSize),
				}

				mockCtl := gomock.NewController(t)
				defer mockCtl.Finish()

				mockCloud := cloud.NewMockCloud(mockCtl)
				mockCloud.EXPECT().CreateDisk(gomock.Eq(ctx), gomock.Eq(req.Name), gomock.Any()).Return(mockDisk, nil)

				awsDriver := controllerService{
					cloud:         mockCloud,
					inFlight:      internal.NewInFlight(),
					driverOptions: &DriverOptions{},
				}

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
						KmsKeyIDKey:  "arn:aws:kms:us-east-1:012345678910:key/abcd1234-a123-456a-a12b-a123b4cd56ef",
					},
				}

				ctx := context.Background()

				mockDisk := &cloud.Disk{
					VolumeID:         req.Name,
					AvailabilityZone: expZone,
					CapacityGiB:      util.BytesToGiB(stdVolSize),
				}

				mockCtl := gomock.NewController(t)
				defer mockCtl.Finish()

				mockCloud := cloud.NewMockCloud(mockCtl)
				mockCloud.EXPECT().CreateDisk(gomock.Eq(ctx), gomock.Eq(req.Name), gomock.Any()).Return(mockDisk, nil)

				awsDriver := controllerService{
					cloud:         mockCloud,
					inFlight:      internal.NewInFlight(),
					driverOptions: &DriverOptions{},
				}

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
			name: "fail with invalid volume parameter",
			testFunc: func(t *testing.T) {
				req := &csi.CreateVolumeRequest{
					Name:               "vol-test",
					CapacityRange:      stdCapRange,
					VolumeCapabilities: stdVolCap,
					Parameters: map[string]string{
						VolumeTypeKey: cloud.VolumeTypeIO1,
						IopsPerGBKey:  "5",
						"unknownKey":  "unknownValue",
					},
				}

				ctx := context.Background()

				mockCtl := gomock.NewController(t)
				defer mockCtl.Finish()

				mockCloud := cloud.NewMockCloud(mockCtl)

				awsDriver := controllerService{
					cloud:         mockCloud,
					inFlight:      internal.NewInFlight(),
					driverOptions: &DriverOptions{},
				}

				_, err := awsDriver.CreateVolume(ctx, req)
				if err == nil {
					t.Fatalf("Expected CreateVolume to fail but got no error")
				}

				srvErr, ok := status.FromError(err)
				if !ok {
					t.Fatalf("Could not get error status code from error: %v", srvErr)
				}
				if srvErr.Code() != codes.InvalidArgument {
					t.Fatalf("Expect InvalidArgument but got: %s", srvErr.Code())
				}
			},
		},
		{
			name: "fail with invalid iops parameter",
			testFunc: func(t *testing.T) {
				req := &csi.CreateVolumeRequest{
					Name:               "vol-test",
					CapacityRange:      stdCapRange,
					VolumeCapabilities: stdVolCap,
					Parameters: map[string]string{
						VolumeTypeKey: cloud.VolumeTypeGP3,
						IopsKey:       "aaa",
					},
				}

				ctx := context.Background()

				mockCtl := gomock.NewController(t)
				defer mockCtl.Finish()

				mockCloud := cloud.NewMockCloud(mockCtl)

				awsDriver := controllerService{
					cloud:         mockCloud,
					inFlight:      internal.NewInFlight(),
					driverOptions: &DriverOptions{},
				}

				_, err := awsDriver.CreateVolume(ctx, req)
				if err == nil {
					t.Fatalf("Expected CreateVolume to fail but got no error")
				}

				srvErr, ok := status.FromError(err)
				if !ok {
					t.Fatalf("Could not get error status code from error: %v", srvErr)
				}
				if srvErr.Code() != codes.InvalidArgument {
					t.Fatalf("Expect InvalidArgument but got: %s", srvErr.Code())
				}
			},
		},
		{
			name: "fail with invalid throughput parameter",
			testFunc: func(t *testing.T) {
				req := &csi.CreateVolumeRequest{
					Name:               "vol-test",
					CapacityRange:      stdCapRange,
					VolumeCapabilities: stdVolCap,
					Parameters: map[string]string{
						VolumeTypeKey: cloud.VolumeTypeGP3,
						ThroughputKey: "aaa",
					},
				}

				ctx := context.Background()

				mockCtl := gomock.NewController(t)
				defer mockCtl.Finish()

				mockCloud := cloud.NewMockCloud(mockCtl)

				awsDriver := controllerService{
					cloud:         mockCloud,
					inFlight:      internal.NewInFlight(),
					driverOptions: &DriverOptions{},
				}

				_, err := awsDriver.CreateVolume(ctx, req)
				if err == nil {
					t.Fatalf("Expected CreateVolume to fail but got no error")
				}

				srvErr, ok := status.FromError(err)
				if !ok {
					t.Fatalf("Could not get error status code from error: %v", srvErr)
				}
				if srvErr.Code() != codes.InvalidArgument {
					t.Fatalf("Expect InvalidArgument but got: %s", srvErr.Code())
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
					Parameters:         map[string]string{},
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
					Parameters:         map[string]string{},
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
					VolumeContext: map[string]string{},
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
					CapacityGiB:      util.BytesToGiB(stdVolSize),
				}

				mockCtl := gomock.NewController(t)
				defer mockCtl.Finish()

				mockCloud := cloud.NewMockCloud(mockCtl)
				mockCloud.EXPECT().CreateDisk(gomock.Eq(ctx), gomock.Eq(req.Name), gomock.Any()).Return(mockDisk, nil)

				awsDriver := controllerService{
					cloud:         mockCloud,
					inFlight:      internal.NewInFlight(),
					driverOptions: &DriverOptions{},
				}

				if _, err := awsDriver.CreateVolume(ctx, req); err != nil {
					srvErr, ok := status.FromError(err)
					if !ok {
						t.Fatalf("Could not get error status code from error: %v", srvErr)
					}
					t.Fatalf("Unexpected error: %v", srvErr.Code())
				}

				mockCloud.EXPECT().CreateDisk(gomock.Eq(ctx), gomock.Eq(req.Name), gomock.Any()).Return(mockDisk, nil)
				resp, err := awsDriver.CreateVolume(ctx, extraReq)
				if err != nil {
					srvErr, ok := status.FromError(err)
					if !ok {
						t.Fatalf("Could not get error status code from error: %v", srvErr)
					}
					t.Fatalf("Unexpected error: %v", srvErr.Code())
				}

				vol := resp.GetVolume()
				if vol == nil {
					t.Fatalf("Expected volume %v, got nil", expVol)
				}

				for expKey, expVal := range expVol.GetVolumeContext() {
					ctx := vol.GetVolumeContext()
					if gotVal, ok := ctx[expKey]; !ok || gotVal != expVal {
						t.Fatalf("Expected volume context for key %v: %v, got: %v", expKey, expVal, gotVal)
					}
				}

				if expVol.GetAccessibleTopology() != nil {
					if !reflect.DeepEqual(expVol.GetAccessibleTopology(), vol.GetAccessibleTopology()) {
						t.Fatalf("Expected AccessibleTopology to be %+v, got: %+v", expVol.GetAccessibleTopology(), vol.GetAccessibleTopology())
					}
				}
			},
		},
		{
			name: "success with extra tags",
			testFunc: func(t *testing.T) {
				const (
					volumeName          = "random-vol-name"
					extraVolumeTagKey   = "extra-tag-key"
					extraVolumeTagValue = "extra-tag-value"
				)
				req := &csi.CreateVolumeRequest{
					Name:               volumeName,
					CapacityRange:      stdCapRange,
					VolumeCapabilities: stdVolCap,
					Parameters:         nil,
				}

				ctx := context.Background()

				mockDisk := &cloud.Disk{
					VolumeID:         req.Name,
					AvailabilityZone: expZone,
					CapacityGiB:      util.BytesToGiB(stdVolSize),
				}

				diskOptions := &cloud.DiskOptions{
					CapacityBytes: stdVolSize,
					Tags: map[string]string{
						cloud.VolumeNameTagKey:   volumeName,
						cloud.AwsEbsDriverTagKey: "true",
						extraVolumeTagKey:        extraVolumeTagValue,
					},
				}

				mockCtl := gomock.NewController(t)
				defer mockCtl.Finish()

				mockCloud := cloud.NewMockCloud(mockCtl)
				mockCloud.EXPECT().CreateDisk(gomock.Eq(ctx), gomock.Eq(req.Name), gomock.Eq(diskOptions)).Return(mockDisk, nil)

				awsDriver := controllerService{
					cloud:    mockCloud,
					inFlight: internal.NewInFlight(),
					driverOptions: &DriverOptions{
						extraTags: map[string]string{
							extraVolumeTagKey: extraVolumeTagValue,
						},
					},
				}

				_, err := awsDriver.CreateVolume(ctx, req)
				if err != nil {
					srvErr, ok := status.FromError(err)
					if !ok {
						t.Fatalf("Could not get error status code from error: %v", srvErr)
					}
					t.Fatalf("Unexpected error: %v", srvErr.Code())
				}
			},
		},
		{
			name: "success with cluster-id",
			testFunc: func(t *testing.T) {
				const (
					volumeName                        = "random-vol-name"
					clusterID                         = "test-cluster-id"
					expectedOwnerTag                  = "kubernetes.io/cluster/test-cluster-id"
					expectedOwnerTagValue             = "owned"
					expectedNameTag                   = "Name"
					expectedNameTagValue              = "test-cluster-id-dynamic-random-vol-name"
					expectedKubernetesClusterTag      = "KubernetesCluster"
					expectedKubernetesClusterTagValue = "test-cluster-id"
				)
				req := &csi.CreateVolumeRequest{
					Name:               volumeName,
					CapacityRange:      stdCapRange,
					VolumeCapabilities: stdVolCap,
					Parameters:         nil,
				}

				ctx := context.Background()

				mockDisk := &cloud.Disk{
					VolumeID:         req.Name,
					AvailabilityZone: expZone,
					CapacityGiB:      util.BytesToGiB(stdVolSize),
				}

				diskOptions := &cloud.DiskOptions{
					CapacityBytes: stdVolSize,
					Tags: map[string]string{
						cloud.VolumeNameTagKey:       volumeName,
						cloud.AwsEbsDriverTagKey:     "true",
						expectedOwnerTag:             expectedOwnerTagValue,
						expectedNameTag:              expectedNameTagValue,
						expectedKubernetesClusterTag: expectedKubernetesClusterTagValue,
					},
				}

				mockCtl := gomock.NewController(t)
				defer mockCtl.Finish()

				mockCloud := cloud.NewMockCloud(mockCtl)
				mockCloud.EXPECT().CreateDisk(gomock.Eq(ctx), gomock.Eq(req.Name), gomock.Eq(diskOptions)).Return(mockDisk, nil)

				awsDriver := controllerService{
					cloud:    mockCloud,
					inFlight: internal.NewInFlight(),
					driverOptions: &DriverOptions{
						kubernetesClusterID: clusterID,
					},
				}

				_, err := awsDriver.CreateVolume(ctx, req)
				if err != nil {
					srvErr, ok := status.FromError(err)
					if !ok {
						t.Fatalf("Could not get error status code from error: %v", srvErr)
					}
					t.Fatalf("Unexpected error: %v", srvErr.Code())
				}
			},
		},
		{
			name: "success with legacy tags",
			testFunc: func(t *testing.T) {
				const (
					volumeName              = "random-vol-name"
					expectedPVCNameTag      = "kubernetes.io/created-for/pvc/name"
					expectedPVCNamespaceTag = "kubernetes.io/created-for/pvc/namespace"
					expectedPVNameTag       = "kubernetes.io/created-for/pv/name"
					pvcNamespace            = "default"
					pvcName                 = "my-pvc"
					pvName                  = volumeName
				)
				req := &csi.CreateVolumeRequest{
					Name:               volumeName,
					CapacityRange:      stdCapRange,
					VolumeCapabilities: stdVolCap,
					Parameters: map[string]string{
						"csi.storage.k8s.io/pvc/name":      pvcName,
						"csi.storage.k8s.io/pvc/namespace": pvcNamespace,
						"csi.storage.k8s.io/pv/name":       pvName,
					},
				}

				ctx := context.Background()

				mockDisk := &cloud.Disk{
					VolumeID:         req.Name,
					AvailabilityZone: expZone,
					CapacityGiB:      util.BytesToGiB(stdVolSize),
				}

				diskOptions := &cloud.DiskOptions{
					CapacityBytes: stdVolSize,
					Tags: map[string]string{
						cloud.VolumeNameTagKey:   volumeName,
						cloud.AwsEbsDriverTagKey: "true",
						expectedPVCNameTag:       pvcName,
						expectedPVCNamespaceTag:  pvcNamespace,
						expectedPVNameTag:        pvName,
					},
				}

				mockCtl := gomock.NewController(t)
				defer mockCtl.Finish()

				mockCloud := cloud.NewMockCloud(mockCtl)
				mockCloud.EXPECT().CreateDisk(gomock.Eq(ctx), gomock.Eq(req.Name), gomock.Eq(diskOptions)).Return(mockDisk, nil)

				awsDriver := controllerService{
					cloud:         mockCloud,
					inFlight:      internal.NewInFlight(),
					driverOptions: &DriverOptions{},
				}

				_, err := awsDriver.CreateVolume(ctx, req)
				if err != nil {
					srvErr, ok := status.FromError(err)
					if !ok {
						t.Fatalf("Could not get error status code from error: %v", srvErr)
					}
					t.Fatalf("Unexpected error: %v", srvErr.Code())
				}
			},
		},
		{
			name: "fail with invalid volume access modes",
			testFunc: func(t *testing.T) {
				req := &csi.CreateVolumeRequest{
					Name:               "vol-test",
					CapacityRange:      stdCapRange,
					VolumeCapabilities: invalidVolCap,
					Parameters: map[string]string{
						VolumeTypeKey: cloud.VolumeTypeIO1,
						IopsPerGBKey:  "5",
						"unknownKey":  "unknownValue",
					},
				}

				ctx := context.Background()

				mockCtl := gomock.NewController(t)
				defer mockCtl.Finish()

				mockCloud := cloud.NewMockCloud(mockCtl)

				awsDriver := controllerService{
					cloud:         mockCloud,
					inFlight:      internal.NewInFlight(),
					driverOptions: &DriverOptions{},
				}

				_, err := awsDriver.CreateVolume(ctx, req)
				if err == nil {
					t.Fatalf("Expected CreateVolume to fail but got no error")
				}

				srvErr, ok := status.FromError(err)
				if !ok {
					t.Fatalf("Could not get error status code from error: %v", srvErr)
				}
				if srvErr.Code() != codes.InvalidArgument {
					t.Fatalf("Expect InvalidArgument but got: %s", srvErr.Code())
				}
			},
		},
		{
			name: "fail with in-flight request",
			testFunc: func(t *testing.T) {
				req := &csi.CreateVolumeRequest{
					Name:               "random-vol-name",
					CapacityRange:      stdCapRange,
					VolumeCapabilities: stdVolCap,
					Parameters:         nil,
				}

				ctx := context.Background()

				mockCtl := gomock.NewController(t)
				defer mockCtl.Finish()

				mockCloud := cloud.NewMockCloud(mockCtl)

				inFlight := internal.NewInFlight()
				inFlight.Insert(req.GetName())
				defer inFlight.Delete(req.GetName())

				awsDriver := controllerService{
					cloud:         mockCloud,
					inFlight:      inFlight,
					driverOptions: &DriverOptions{},
				}

				_, err := awsDriver.CreateVolume(ctx, req)

				checkExpectedErrorCode(t, err, codes.Aborted)
			},
		},
		{
			name: "Fail with IdempotentParameterMismatch error",
			testFunc: func(t *testing.T) {
				req := &csi.CreateVolumeRequest{
					Name:               "vol-test",
					CapacityRange:      stdCapRange,
					VolumeCapabilities: stdVolCap,
				}

				ctx := context.Background()

				mockCtl := gomock.NewController(t)
				defer mockCtl.Finish()

				mockCloud := cloud.NewMockCloud(mockCtl)
				mockCloud.EXPECT().CreateDisk(gomock.Eq(ctx), gomock.Eq(req.Name), gomock.Any()).Return(nil, cloud.ErrIdempotentParameterMismatch)

				awsDriver := controllerService{
					cloud:         mockCloud,
					inFlight:      internal.NewInFlight(),
					driverOptions: &DriverOptions{},
				}

				_, err := awsDriver.CreateVolume(ctx, req)
				checkExpectedErrorCode(t, err, codes.AlreadyExists)
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, tc.testFunc)
	}
}

func TestCreateVolumeWithFormattingParameters(t *testing.T) {
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

	testCases := []struct {
		name                       string
		formattingOptionParameters map[string]string
		errExpected                bool
	}{
		{
			name: "success with block size",
			formattingOptionParameters: map[string]string{
				BlockSizeKey: "4096",
			},
			errExpected: false,
		},
		{
			name: "success with inode size",
			formattingOptionParameters: map[string]string{
				InodeSizeKey: "256",
			},
			errExpected: false,
		},
		{
			name: "success with bytes-per-inode",
			formattingOptionParameters: map[string]string{
				BytesPerInodeKey: "8192",
			},
			errExpected: false,
		},
		{
			name: "success with number-of-inodes",
			formattingOptionParameters: map[string]string{
				NumberOfInodesKey: "13107200",
			},
			errExpected: false,
		},
		{
			name: "success with ext4 big alloc option",
			formattingOptionParameters: map[string]string{
				Ext4BigAllocKey: "true",
			},
			errExpected: false,
		},
		{
			name: "success with ext4 bigalloc option and custom cluster size",
			formattingOptionParameters: map[string]string{
				Ext4BigAllocKey:    "true",
				Ext4ClusterSizeKey: "16384",
			},
			errExpected: false,
		},
		{
			name: "failure with block size",
			formattingOptionParameters: map[string]string{
				BlockSizeKey: "wrong_value",
			},
			errExpected: true,
		},
		{
			name: "failure with inode size",
			formattingOptionParameters: map[string]string{
				InodeSizeKey: "wrong_value",
			},
			errExpected: true,
		},
		{
			name: "failure with bytes-per-inode",
			formattingOptionParameters: map[string]string{
				BytesPerInodeKey: "wrong_value",
			},
			errExpected: true,
		},
		{
			name: "failure with number-of-inodes",
			formattingOptionParameters: map[string]string{
				NumberOfInodesKey: "wrong_value",
			},
			errExpected: true,
		},
		{
			name: "failure with ext4 custom cluster size",
			formattingOptionParameters: map[string]string{
				Ext4BigAllocKey:    "true",
				Ext4ClusterSizeKey: "wrong_value",
			},
			errExpected: true,
		},
		{
			name: "failure with ext4 bigalloc option and cluster size mismatch",
			formattingOptionParameters: map[string]string{
				Ext4BigAllocKey:    "false",
				Ext4ClusterSizeKey: "16384",
			},
			errExpected: true,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assert := assert.New(t)

			req := &csi.CreateVolumeRequest{
				Name:               "random-vol-name",
				CapacityRange:      stdCapRange,
				VolumeCapabilities: stdVolCap,
				Parameters:         tc.formattingOptionParameters,
			}

			ctx := context.Background()

			mockDisk := &cloud.Disk{
				VolumeID:         req.Name,
				AvailabilityZone: expZone,
				CapacityGiB:      util.BytesToGiB(stdVolSize),
			}

			mockCtl := gomock.NewController(t)

			mockCloud := cloud.NewMockCloud(mockCtl)

			// CreateDisk not called on Unhappy Case
			if !tc.errExpected {
				mockCloud.EXPECT().CreateDisk(gomock.Eq(ctx), gomock.Eq(req.Name), gomock.Any()).Return(mockDisk, nil)
				defer mockCtl.Finish()
			}

			awsDriver := controllerService{
				cloud:         mockCloud,
				inFlight:      internal.NewInFlight(),
				driverOptions: &DriverOptions{},
			}

			response, err := awsDriver.CreateVolume(ctx, req)

			// Splits happy case tests from unhappy case tests
			if !tc.errExpected {
				assert.Nilf(err, "Unexpected error: %w", err)

				volCtx := response.Volume.VolumeContext

				for formattingParamKey, formattingParamValue := range tc.formattingOptionParameters {
					createdFormattingParamValue, ok := volCtx[formattingParamKey]
					assert.Truef(ok, "Missing key %s in VolumeContext", formattingParamKey)

					assert.Equalf(createdFormattingParamValue, formattingParamValue, "Invalid %s in VolumeContext", formattingParamKey)
				}
			} else {
				assert.NotNilf(err, "CreateVolume did not return an error")

				checkExpectedErrorCode(t, err, codes.InvalidArgument)
			}
		})
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

				mockCloud := cloud.NewMockCloud(mockCtl)
				mockCloud.EXPECT().DeleteDisk(gomock.Eq(ctx), gomock.Eq(req.VolumeId)).Return(true, nil)
				awsDriver := controllerService{
					cloud:         mockCloud,
					inFlight:      internal.NewInFlight(),
					driverOptions: &DriverOptions{},
				}
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

				mockCloud := cloud.NewMockCloud(mockCtl)
				mockCloud.EXPECT().DeleteDisk(gomock.Eq(ctx), gomock.Eq(req.VolumeId)).Return(false, cloud.ErrNotFound)
				awsDriver := controllerService{
					cloud:         mockCloud,
					inFlight:      internal.NewInFlight(),
					driverOptions: &DriverOptions{},
				}
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
			name: "fail delete disk",
			testFunc: func(t *testing.T) {
				req := &csi.DeleteVolumeRequest{
					VolumeId: "test-vol",
				}

				ctx := context.Background()
				mockCtl := gomock.NewController(t)
				defer mockCtl.Finish()

				mockCloud := cloud.NewMockCloud(mockCtl)
				mockCloud.EXPECT().DeleteDisk(gomock.Eq(ctx), gomock.Eq(req.VolumeId)).Return(false, fmt.Errorf("DeleteDisk could not delete volume"))
				awsDriver := controllerService{
					cloud:         mockCloud,
					inFlight:      internal.NewInFlight(),
					driverOptions: &DriverOptions{},
				}
				resp, err := awsDriver.DeleteVolume(ctx, req)
				if err != nil {
					srvErr, ok := status.FromError(err)
					if !ok {
						t.Fatalf("Could not get error status code from error: %v", srvErr)
					}
					if srvErr.Code() != codes.Internal {
						t.Fatalf("Unexpected error: %v", srvErr.Code())
					}
				} else {
					t.Fatalf("Expected error, got nil")
				}

				if resp != nil {
					t.Fatalf("Expected resp to be nil, got: %+v", resp)
				}
			},
		},
		{
			name: "fail another request already in-flight",
			testFunc: func(t *testing.T) {
				req := &csi.DeleteVolumeRequest{
					VolumeId: "vol-test",
				}

				ctx := context.Background()
				mockCtl := gomock.NewController(t)
				defer mockCtl.Finish()

				mockCloud := cloud.NewMockCloud(mockCtl)
				inFlight := internal.NewInFlight()
				inFlight.Insert(req.GetVolumeId())
				defer inFlight.Delete(req.GetVolumeId())
				awsDriver := controllerService{
					cloud:         mockCloud,
					inFlight:      inFlight,
					driverOptions: &DriverOptions{},
				}
				_, err := awsDriver.DeleteVolume(ctx, req)

				checkExpectedErrorCode(t, err, codes.Aborted)
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
			name: "Return WellKnownTopologyKey if present from preferred",
			requirement: &csi.TopologyRequirement{
				Requisite: []*csi.Topology{
					{
						Segments: map[string]string{TopologyKey: ""},
					},
				},
				Preferred: []*csi.Topology{
					{
						Segments: map[string]string{TopologyKey: expZone, WellKnownTopologyKey: "foobar"},
					},
				},
			},
			expZone: "foobar",
		},
		{
			name: "Return WellKnownTopologyKey if present from requisite",
			requirement: &csi.TopologyRequirement{
				Requisite: []*csi.Topology{
					{
						Segments: map[string]string{TopologyKey: expZone, WellKnownTopologyKey: "foobar"},
					},
				},
			},
			expZone: "foobar",
		},
		{
			name: "Pick from preferred",
			requirement: &csi.TopologyRequirement{
				Requisite: []*csi.Topology{
					{
						Segments: map[string]string{TopologyKey: ""},
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

func TestGetOutpostArn(t *testing.T) {
	expRawOutpostArn := "arn:aws:outposts:us-west-2:111111111111:outpost/op-0aaa000a0aaaa00a0"
	outpostArn, _ := arn.Parse(strings.ReplaceAll(expRawOutpostArn, "outpost/", ""))
	testCases := []struct {
		name          string
		requirement   *csi.TopologyRequirement
		expZone       string
		expOutpostArn string
	}{
		{
			name: "Get from preferred",
			requirement: &csi.TopologyRequirement{
				Requisite: []*csi.Topology{
					{
						Segments: map[string]string{TopologyKey: expZone},
					},
				},
				Preferred: []*csi.Topology{
					{
						Segments: map[string]string{
							TopologyKey:     expZone,
							AwsAccountIDKey: outpostArn.AccountID,
							AwsOutpostIDKey: outpostArn.Resource,
							AwsRegionKey:    outpostArn.Region,
							AwsPartitionKey: outpostArn.Partition,
						},
					},
				},
			},
			expZone:       expZone,
			expOutpostArn: expRawOutpostArn,
		},
		{
			name: "Get from requisite",
			requirement: &csi.TopologyRequirement{
				Requisite: []*csi.Topology{
					{
						Segments: map[string]string{
							TopologyKey:     expZone,
							AwsAccountIDKey: outpostArn.AccountID,
							AwsOutpostIDKey: outpostArn.Resource,
							AwsRegionKey:    outpostArn.Region,
							AwsPartitionKey: outpostArn.Partition,
						},
					},
				},
			},
			expZone:       expZone,
			expOutpostArn: expRawOutpostArn,
		},
		{
			name: "Get from empty topology",
			requirement: &csi.TopologyRequirement{
				Preferred: []*csi.Topology{{}},
				Requisite: []*csi.Topology{{}},
			},
			expZone:       "",
			expOutpostArn: "",
		},
		{
			name:          "Topology Requirement is nil",
			requirement:   nil,
			expZone:       "",
			expOutpostArn: "",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actual := getOutpostArn(tc.requirement)
			if actual != tc.expOutpostArn {
				t.Fatalf("Expected %v, got outpostArn: %v", tc.expOutpostArn, actual)
			}
		})
	}
}

func TestBuildOutpostArn(t *testing.T) {
	expRawOutpostArn := "arn:aws:outposts:us-west-2:111111111111:outpost/op-0aaa000a0aaaa00a0"
	testCases := []struct {
		name         string
		awsPartition string
		awsRegion    string
		awsAccountID string
		awsOutpostID string
		expectedArn  string
	}{
		{
			name:         "all fields are present",
			awsPartition: "aws",
			awsRegion:    "us-west-2",
			awsOutpostID: "op-0aaa000a0aaaa00a0",
			awsAccountID: "111111111111",
			expectedArn:  expRawOutpostArn,
		},
		{
			name:         "partition is missing",
			awsRegion:    "us-west-2",
			awsOutpostID: "op-0aaa000a0aaaa00a0",
			awsAccountID: "111111111111",
			expectedArn:  "",
		},
		{
			name:         "region is missing",
			awsPartition: "aws",
			awsOutpostID: "op-0aaa000a0aaaa00a0",
			awsAccountID: "111111111111",
			expectedArn:  "",
		},
		{
			name:         "account id is missing",
			awsPartition: "aws",
			awsRegion:    "us-west-2",
			awsOutpostID: "op-0aaa000a0aaaa00a0",
			expectedArn:  "",
		},
		{
			name:         "outpost id is missing",
			awsPartition: "aws",
			awsRegion:    "us-west-2",
			awsAccountID: "111111111111",
			expectedArn:  "",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			segment := map[string]string{
				AwsRegionKey:    tc.awsRegion,
				AwsPartitionKey: tc.awsPartition,
				AwsAccountIDKey: tc.awsAccountID,
				AwsOutpostIDKey: tc.awsOutpostID,
			}
			actual := BuildOutpostArn(segment)
			if actual != tc.expectedArn {
				t.Fatalf("Expected %v, got outpostArn: %v", tc.expectedArn, actual)
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

				ctx := context.Background()
				mockSnapshot := &cloud.Snapshot{
					SnapshotID:     fmt.Sprintf("snapshot-%d", rand.New(rand.NewSource(time.Now().UnixNano())).Uint64()),
					SourceVolumeID: req.SourceVolumeId,
					Size:           1,
					CreationTime:   time.Now(),
				}
				mockCtl := gomock.NewController(t)
				defer mockCtl.Finish()

				mockCloud := cloud.NewMockCloud(mockCtl)
				mockCloud.EXPECT().CreateSnapshot(gomock.Eq(ctx), gomock.Eq(req.SourceVolumeId), gomock.Any()).Return(mockSnapshot, nil)
				mockCloud.EXPECT().GetSnapshotByName(gomock.Eq(ctx), gomock.Eq(req.GetName())).Return(nil, cloud.ErrNotFound)

				awsDriver := controllerService{
					cloud:         mockCloud,
					inFlight:      internal.NewInFlight(),
					driverOptions: &DriverOptions{},
				}
				resp, err := awsDriver.CreateSnapshot(context.Background(), req)
				if err != nil {
					t.Fatalf("Unexpected error: %v", err)
				}

				if snap := resp.GetSnapshot(); snap == nil {
					t.Fatalf("Expected snapshot %v, got nil", expSnapshot)
				}
			},
		},
		{
			name: "success with cluster-id",
			testFunc: func(t *testing.T) {
				const (
					snapshotName          = "test-snapshot"
					clusterID             = "test-cluster-id"
					expectedOwnerTag      = "kubernetes.io/cluster/test-cluster-id"
					expectedOwnerTagValue = "owned"
					expectedNameTag       = "Name"
					expectedNameTagValue  = "test-cluster-id-dynamic-test-snapshot"
				)
				req := &csi.CreateSnapshotRequest{
					Name:           snapshotName,
					Parameters:     nil,
					SourceVolumeId: "vol-test",
				}
				expSnapshot := &csi.Snapshot{
					ReadyToUse: true,
				}

				ctx := context.Background()
				mockSnapshot := &cloud.Snapshot{
					SnapshotID:     fmt.Sprintf("snapshot-%d", rand.New(rand.NewSource(time.Now().UnixNano())).Uint64()),
					SourceVolumeID: req.SourceVolumeId,
					Size:           1,
					CreationTime:   time.Now(),
				}
				snapshotOptions := &cloud.SnapshotOptions{
					Tags: map[string]string{
						cloud.SnapshotNameTagKey: snapshotName,
						cloud.AwsEbsDriverTagKey: "true",
						expectedOwnerTag:         expectedOwnerTagValue,
						expectedNameTag:          expectedNameTagValue,
					},
				}
				mockCtl := gomock.NewController(t)
				defer mockCtl.Finish()

				mockCloud := cloud.NewMockCloud(mockCtl)
				mockCloud.EXPECT().CreateSnapshot(gomock.Eq(ctx), gomock.Eq(req.SourceVolumeId), gomock.Eq(snapshotOptions)).Return(mockSnapshot, nil)
				mockCloud.EXPECT().GetSnapshotByName(gomock.Eq(ctx), gomock.Eq(req.GetName())).Return(nil, cloud.ErrNotFound)

				awsDriver := controllerService{
					cloud:    mockCloud,
					inFlight: internal.NewInFlight(),
					driverOptions: &DriverOptions{
						kubernetesClusterID: clusterID,
					},
				}
				resp, err := awsDriver.CreateSnapshot(context.Background(), req)
				if err != nil {
					t.Fatalf("Unexpected error: %v", err)
				}

				if snap := resp.GetSnapshot(); snap == nil {
					t.Fatalf("Expected snapshot %v, got nil", expSnapshot)
				}
			},
		},
		{
			name: "success with extra tags",
			testFunc: func(t *testing.T) {
				const (
					snapshotName        = "test-snapshot"
					extraVolumeTagKey   = "extra-tag-key"
					extraVolumeTagValue = "extra-tag-value"
				)
				req := &csi.CreateSnapshotRequest{
					Name:           snapshotName,
					Parameters:     nil,
					SourceVolumeId: "vol-test",
				}
				expSnapshot := &csi.Snapshot{
					ReadyToUse: true,
				}

				ctx := context.Background()
				mockSnapshot := &cloud.Snapshot{
					SnapshotID:     fmt.Sprintf("snapshot-%d", rand.New(rand.NewSource(time.Now().UnixNano())).Uint64()),
					SourceVolumeID: req.SourceVolumeId,
					Size:           1,
					CreationTime:   time.Now(),
				}
				snapshotOptions := &cloud.SnapshotOptions{
					Tags: map[string]string{
						cloud.SnapshotNameTagKey: snapshotName,
						cloud.AwsEbsDriverTagKey: "true",
						extraVolumeTagKey:        extraVolumeTagValue,
					},
				}
				mockCtl := gomock.NewController(t)
				defer mockCtl.Finish()

				mockCloud := cloud.NewMockCloud(mockCtl)
				mockCloud.EXPECT().CreateSnapshot(gomock.Eq(ctx), gomock.Eq(req.SourceVolumeId), gomock.Eq(snapshotOptions)).Return(mockSnapshot, nil)
				mockCloud.EXPECT().GetSnapshotByName(gomock.Eq(ctx), gomock.Eq(req.GetName())).Return(nil, cloud.ErrNotFound)

				awsDriver := controllerService{
					cloud:    mockCloud,
					inFlight: internal.NewInFlight(),
					driverOptions: &DriverOptions{
						extraTags: map[string]string{
							extraVolumeTagKey: extraVolumeTagValue,
						},
					},
				}
				resp, err := awsDriver.CreateSnapshot(context.Background(), req)
				if err != nil {
					t.Fatalf("Unexpected error: %v", err)
				}

				if snap := resp.GetSnapshot(); snap == nil {
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

				mockCtl := gomock.NewController(t)
				defer mockCtl.Finish()

				mockCloud := cloud.NewMockCloud(mockCtl)

				awsDriver := controllerService{
					cloud:         mockCloud,
					inFlight:      internal.NewInFlight(),
					driverOptions: &DriverOptions{},
				}
				if _, err := awsDriver.CreateSnapshot(context.Background(), req); err != nil {
					srvErr, ok := status.FromError(err)
					if !ok {
						t.Fatalf("Could not get error status code from error: %v", srvErr)
					}
					if srvErr.Code() != codes.InvalidArgument {
						t.Fatalf("Expected error code %d, got %d message %s", codes.InvalidArgument, srvErr.Code(), srvErr.Message())
					}
				} else {
					t.Fatalf("Expected error %v, got no error", codes.InvalidArgument)
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

				ctx := context.Background()
				mockSnapshot := &cloud.Snapshot{
					SnapshotID:     fmt.Sprintf("snapshot-%d", rand.New(rand.NewSource(time.Now().UnixNano())).Uint64()),
					SourceVolumeID: req.SourceVolumeId,
					Size:           1,
					CreationTime:   time.Now(),
				}
				mockCtl := gomock.NewController(t)
				defer mockCtl.Finish()

				mockCloud := cloud.NewMockCloud(mockCtl)
				mockCloud.EXPECT().GetSnapshotByName(gomock.Eq(ctx), gomock.Eq(req.GetName())).Return(nil, cloud.ErrNotFound)
				mockCloud.EXPECT().CreateSnapshot(gomock.Eq(ctx), gomock.Eq(req.SourceVolumeId), gomock.Any()).Return(mockSnapshot, nil)

				awsDriver := controllerService{
					cloud:         mockCloud,
					inFlight:      internal.NewInFlight(),
					driverOptions: &DriverOptions{},
				}
				resp, err := awsDriver.CreateSnapshot(context.Background(), req)
				if err != nil {
					srvErr, ok := status.FromError(err)
					if !ok {
						t.Fatalf("Could not get error status code from error: %v", srvErr)
					}
					if srvErr.Code() != codes.OK {
						t.Fatalf("Expected error code %d, got %d message %s", codes.OK, srvErr.Code(), srvErr.Message())
					}
					t.Fatalf("Unexpected error: %v", err)
				}
				snap := resp.GetSnapshot()
				if snap == nil {
					t.Fatalf("Expected snapshot %v, got nil", expSnapshot)
				}

				mockCloud.EXPECT().GetSnapshotByName(gomock.Eq(ctx), gomock.Eq(extraReq.GetName())).Return(mockSnapshot, nil)
				_, err = awsDriver.CreateSnapshot(ctx, extraReq)
				if err != nil {
					srvErr, ok := status.FromError(err)
					if !ok {
						t.Fatalf("Could not get error status code from error: %v", srvErr)
					}
					if srvErr.Code() != codes.AlreadyExists {
						t.Fatalf("Expected error code %d, got %d message %s", codes.AlreadyExists, srvErr.Code(), srvErr.Message())
					}
				} else {
					t.Fatalf("Expected error %v, got no error", codes.AlreadyExists)
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

				ctx := context.Background()
				mockSnapshot := &cloud.Snapshot{
					SnapshotID:     fmt.Sprintf("snapshot-%d", rand.New(rand.NewSource(time.Now().UnixNano())).Uint64()),
					SourceVolumeID: req.SourceVolumeId,
					Size:           1,
					CreationTime:   time.Now(),
				}
				mockCtl := gomock.NewController(t)
				defer mockCtl.Finish()

				mockCloud := cloud.NewMockCloud(mockCtl)
				mockCloud.EXPECT().GetSnapshotByName(gomock.Eq(ctx), gomock.Eq(req.GetName())).Return(nil, cloud.ErrNotFound)
				mockCloud.EXPECT().CreateSnapshot(gomock.Eq(ctx), gomock.Eq(req.SourceVolumeId), gomock.Any()).Return(mockSnapshot, nil)

				awsDriver := controllerService{
					cloud:         mockCloud,
					inFlight:      internal.NewInFlight(),
					driverOptions: &DriverOptions{},
				}
				resp, err := awsDriver.CreateSnapshot(context.Background(), req)
				if err != nil {
					t.Fatalf("Unexpected error: %v", err)
				}
				snap := resp.GetSnapshot()
				if snap == nil {
					t.Fatalf("Expected snapshot %v, got nil", expSnapshot)
				}

				mockCloud.EXPECT().GetSnapshotByName(gomock.Eq(ctx), gomock.Eq(extraReq.GetName())).Return(mockSnapshot, nil)
				_, err = awsDriver.CreateSnapshot(ctx, extraReq)
				if err != nil {
					t.Fatalf("Unexpected error: %v", err)
				}
			},
		},
		{
			name: "fail with another request in-flight",
			testFunc: func(t *testing.T) {
				req := &csi.CreateSnapshotRequest{
					Name:           "test-snapshot",
					Parameters:     nil,
					SourceVolumeId: "vol-test",
				}

				mockCtl := gomock.NewController(t)
				defer mockCtl.Finish()

				mockCloud := cloud.NewMockCloud(mockCtl)

				inFlight := internal.NewInFlight()
				inFlight.Insert(req.GetName())
				defer inFlight.Delete(req.GetName())

				awsDriver := controllerService{
					cloud:         mockCloud,
					inFlight:      inFlight,
					driverOptions: &DriverOptions{},
				}
				_, err := awsDriver.CreateSnapshot(context.Background(), req)

				checkExpectedErrorCode(t, err, codes.Aborted)
			},
		},
		{
			name: "success with VolumeSnapshotClass tags",
			testFunc: func(t *testing.T) {
				const (
					snapshotName  = "test-snapshot"
					extraTagKey   = "test-key"
					extraTagValue = "test-value"
				)

				req := &csi.CreateSnapshotRequest{
					Name: snapshotName,
					Parameters: map[string]string{
						"tagSpecification_1": fmt.Sprintf("%s=%s", extraTagKey, extraTagValue),
					},
					SourceVolumeId: "vol-test",
				}
				expSnapshot := &csi.Snapshot{
					ReadyToUse: true,
				}

				ctx := context.Background()
				mockSnapshot := &cloud.Snapshot{
					SnapshotID:     fmt.Sprintf("snapshot-%d", rand.New(rand.NewSource(time.Now().UnixNano())).Uint64()),
					SourceVolumeID: req.SourceVolumeId,
					Size:           1,
					CreationTime:   time.Now(),
				}
				mockCtl := gomock.NewController(t)
				defer mockCtl.Finish()

				snapshotOptions := &cloud.SnapshotOptions{
					Tags: map[string]string{
						cloud.SnapshotNameTagKey: snapshotName,
						cloud.AwsEbsDriverTagKey: isManagedByDriver,
						extraTagKey:              extraTagValue,
					},
				}

				mockCloud := cloud.NewMockCloud(mockCtl)
				mockCloud.EXPECT().CreateSnapshot(gomock.Eq(ctx), gomock.Eq(req.SourceVolumeId), gomock.Eq(snapshotOptions)).Return(mockSnapshot, nil)
				mockCloud.EXPECT().GetSnapshotByName(gomock.Eq(ctx), gomock.Eq(req.GetName())).Return(nil, cloud.ErrNotFound)

				awsDriver := controllerService{
					cloud:         mockCloud,
					inFlight:      internal.NewInFlight(),
					driverOptions: &DriverOptions{},
				}
				resp, err := awsDriver.CreateSnapshot(context.Background(), req)
				if err != nil {
					t.Fatalf("Unexpected error: %v", err)
				}

				if snap := resp.GetSnapshot(); snap == nil {
					t.Fatalf("Expected snapshot %v, got nil", expSnapshot)
				}
			},
		},
		{
			name: "success with VolumeSnapshotClass with Name tag and cluster id",
			testFunc: func(t *testing.T) {
				const (
					snapshotName = "test-snapshot"
					nameTagValue = "test-name-tag-value"
					clusterId    = "test-cluster-id"
				)

				req := &csi.CreateSnapshotRequest{
					Name: snapshotName,
					Parameters: map[string]string{
						"tagSpecification_1": NameTag + "=" + nameTagValue,
					},
					SourceVolumeId: "vol-test",
				}
				expSnapshot := &csi.Snapshot{
					ReadyToUse: true,
				}

				ctx := context.Background()
				mockSnapshot := &cloud.Snapshot{
					SnapshotID:     fmt.Sprintf("snapshot-%d", rand.New(rand.NewSource(time.Now().UnixNano())).Uint64()),
					SourceVolumeID: req.SourceVolumeId,
					Size:           1,
					CreationTime:   time.Now(),
				}
				mockCtl := gomock.NewController(t)
				defer mockCtl.Finish()

				snapshotOptions := &cloud.SnapshotOptions{
					Tags: map[string]string{
						cloud.SnapshotNameTagKey:               snapshotName,
						cloud.AwsEbsDriverTagKey:               isManagedByDriver,
						NameTag:                                nameTagValue,
						ResourceLifecycleTagPrefix + clusterId: ResourceLifecycleOwned,
					},
				}

				mockCloud := cloud.NewMockCloud(mockCtl)
				mockCloud.EXPECT().CreateSnapshot(gomock.Eq(ctx), gomock.Eq(req.SourceVolumeId), gomock.Eq(snapshotOptions)).Return(mockSnapshot, nil)
				mockCloud.EXPECT().GetSnapshotByName(gomock.Eq(ctx), gomock.Eq(req.GetName())).Return(nil, cloud.ErrNotFound)

				awsDriver := controllerService{
					cloud:         mockCloud,
					inFlight:      internal.NewInFlight(),
					driverOptions: &DriverOptions{kubernetesClusterID: clusterId},
				}
				resp, err := awsDriver.CreateSnapshot(context.Background(), req)
				if err != nil {
					t.Fatalf("Unexpected error: %v", err)
				}

				if snap := resp.GetSnapshot(); snap == nil {
					t.Fatalf("Expected snapshot %v, got nil", expSnapshot)
				}
			},
		},
		{
			name: "success with EnableFastSnapshotRestore - normal",
			testFunc: func(t *testing.T) {
				const (
					snapshotName = "test-snapshot"
				)

				req := &csi.CreateSnapshotRequest{
					Name: snapshotName,
					Parameters: map[string]string{
						"fastSnapshotRestoreAvailabilityZones": "us-east-1a, us-east-1f",
					},
					SourceVolumeId: "vol-test",
				}
				expSnapshot := &csi.Snapshot{
					ReadyToUse: true,
				}

				ctx := context.Background()
				mockSnapshot := &cloud.Snapshot{
					SnapshotID:     fmt.Sprintf("snapshot-%d", rand.New(rand.NewSource(time.Now().UnixNano())).Uint64()),
					SourceVolumeID: req.SourceVolumeId,
					Size:           1,
					CreationTime:   time.Now(),
				}
				mockCtl := gomock.NewController(t)
				defer mockCtl.Finish()

				snapshotOptions := &cloud.SnapshotOptions{
					Tags: map[string]string{
						cloud.SnapshotNameTagKey: snapshotName,
						cloud.AwsEbsDriverTagKey: isManagedByDriver,
					},
				}

				expOutput := &ec2.EnableFastSnapshotRestoresOutput{
					Successful: []*ec2.EnableFastSnapshotRestoreSuccessItem{{
						AvailabilityZone: aws.String("us-east-1a,us-east-1f"),
						SnapshotId:       aws.String("snap-test-id")}},
					Unsuccessful: []*ec2.EnableFastSnapshotRestoreErrorItem{},
				}

				mockCloud := cloud.NewMockCloud(mockCtl)
				mockCloud.EXPECT().GetSnapshotByName(gomock.Eq(ctx), gomock.Eq(req.GetName())).Return(nil, cloud.ErrNotFound).AnyTimes()
				mockCloud.EXPECT().AvailabilityZones(gomock.Eq(ctx)).Return(map[string]struct{}{
					"us-east-1a": {}, "us-east-1f": {}}, nil).AnyTimes()
				mockCloud.EXPECT().CreateSnapshot(gomock.Eq(ctx), gomock.Eq(req.SourceVolumeId), gomock.Eq(snapshotOptions)).Return(mockSnapshot, nil).AnyTimes()
				mockCloud.EXPECT().EnableFastSnapshotRestores(gomock.Eq(ctx), gomock.Eq([]string{"us-east-1a", "us-east-1f"}), gomock.Eq(mockSnapshot.SnapshotID)).Return(expOutput, nil).AnyTimes()

				awsDriver := controllerService{
					cloud:         mockCloud,
					inFlight:      internal.NewInFlight(),
					driverOptions: &DriverOptions{},
				}

				resp, err := awsDriver.CreateSnapshot(context.Background(), req)
				if err != nil {
					t.Fatalf("Unexpected error: %v", err)
				}

				if snap := resp.GetSnapshot(); snap == nil {
					t.Fatalf("Expected snapshot %v, got nil", expSnapshot)
				}
			},
		},
		{
			name: "success with EnableFastSnapshotRestore - failed to get availability zones",
			testFunc: func(t *testing.T) {
				const (
					snapshotName = "test-snapshot"
				)

				req := &csi.CreateSnapshotRequest{
					Name: snapshotName,
					Parameters: map[string]string{
						"fastSnapshotRestoreAvailabilityZones": "us-east-1a, us-east-1f",
					},
					SourceVolumeId: "vol-test",
				}
				expSnapshot := &csi.Snapshot{
					ReadyToUse: true,
				}

				ctx := context.Background()
				mockSnapshot := &cloud.Snapshot{
					SnapshotID:     fmt.Sprintf("snapshot-%d", rand.New(rand.NewSource(time.Now().UnixNano())).Uint64()),
					SourceVolumeID: req.SourceVolumeId,
					Size:           1,
					CreationTime:   time.Now(),
				}
				mockCtl := gomock.NewController(t)
				defer mockCtl.Finish()

				snapshotOptions := &cloud.SnapshotOptions{
					Tags: map[string]string{
						cloud.SnapshotNameTagKey: snapshotName,
						cloud.AwsEbsDriverTagKey: isManagedByDriver,
					},
				}

				expOutput := &ec2.EnableFastSnapshotRestoresOutput{
					Successful: []*ec2.EnableFastSnapshotRestoreSuccessItem{{
						AvailabilityZone: aws.String("us-east-1a,us-east-1f"),
						SnapshotId:       aws.String("snap-test-id")}},
					Unsuccessful: []*ec2.EnableFastSnapshotRestoreErrorItem{},
				}

				mockCloud := cloud.NewMockCloud(mockCtl)
				mockCloud.EXPECT().GetSnapshotByName(gomock.Eq(ctx), gomock.Eq(req.GetName())).Return(nil, cloud.ErrNotFound).AnyTimes()
				mockCloud.EXPECT().AvailabilityZones(gomock.Eq(ctx)).Return(nil, fmt.Errorf("error describing availability zones")).AnyTimes()
				mockCloud.EXPECT().CreateSnapshot(gomock.Eq(ctx), gomock.Eq(req.SourceVolumeId), gomock.Eq(snapshotOptions)).Return(mockSnapshot, nil).AnyTimes()
				mockCloud.EXPECT().EnableFastSnapshotRestores(gomock.Eq(ctx), gomock.Eq([]string{"us-east-1a", "us-east-1f"}), gomock.Eq(mockSnapshot.SnapshotID)).Return(expOutput, nil).AnyTimes()

				awsDriver := controllerService{
					cloud:         mockCloud,
					inFlight:      internal.NewInFlight(),
					driverOptions: &DriverOptions{},
				}

				resp, err := awsDriver.CreateSnapshot(context.Background(), req)
				if err != nil {
					t.Fatalf("Unexpected error: %v", err)
				}

				if snap := resp.GetSnapshot(); snap == nil {
					t.Fatalf("Expected snapshot %v, got nil", expSnapshot)
				}
			},
		},
		{
			name: "fail with EnableFastSnapshotRestore - call to enable FSR failed",
			testFunc: func(t *testing.T) {
				const (
					snapshotName = "test-snapshot"
				)

				req := &csi.CreateSnapshotRequest{
					Name: snapshotName,
					Parameters: map[string]string{
						"fastSnapshotRestoreAvailabilityZones": "us-west-1a, us-east-1f",
					},
					SourceVolumeId: "vol-test",
				}

				ctx := context.Background()
				mockSnapshot := &cloud.Snapshot{
					SnapshotID:     fmt.Sprintf("snapshot-%d", rand.New(rand.NewSource(time.Now().UnixNano())).Uint64()),
					SourceVolumeID: req.SourceVolumeId,
					Size:           1,
					CreationTime:   time.Now(),
				}
				mockCtl := gomock.NewController(t)
				defer mockCtl.Finish()

				snapshotOptions := &cloud.SnapshotOptions{
					Tags: map[string]string{
						cloud.SnapshotNameTagKey: snapshotName,
						cloud.AwsEbsDriverTagKey: isManagedByDriver,
					},
				}
				expOutput := &ec2.EnableFastSnapshotRestoresOutput{
					Successful: []*ec2.EnableFastSnapshotRestoreSuccessItem{},
					Unsuccessful: []*ec2.EnableFastSnapshotRestoreErrorItem{{
						SnapshotId: aws.String("snap-test-id"),
						FastSnapshotRestoreStateErrors: []*ec2.EnableFastSnapshotRestoreStateErrorItem{
							{
								AvailabilityZone: aws.String("us-west-1a,us-east-1f"),
								Error: &ec2.EnableFastSnapshotRestoreStateError{
									Message: aws.String("failed to create fast snapshot restore"),
								}},
						},
					}},
				}

				mockCloud := cloud.NewMockCloud(mockCtl)
				mockCloud.EXPECT().GetSnapshotByName(gomock.Eq(ctx), gomock.Eq(req.GetName())).Return(nil, cloud.ErrNotFound).AnyTimes()
				mockCloud.EXPECT().AvailabilityZones(gomock.Eq(ctx)).Return(nil, fmt.Errorf("error describing availability zones")).AnyTimes()
				mockCloud.EXPECT().CreateSnapshot(gomock.Eq(ctx), gomock.Eq(req.SourceVolumeId), gomock.Eq(snapshotOptions)).Return(mockSnapshot, nil).AnyTimes()
				mockCloud.EXPECT().EnableFastSnapshotRestores(gomock.Eq(ctx), gomock.Eq([]string{"us-west-1a", "us-east-1f"}), gomock.Eq(mockSnapshot.SnapshotID)).
					Return(expOutput, fmt.Errorf("Failed to create Fast Snapshot Restores")).AnyTimes()
				mockCloud.EXPECT().DeleteSnapshot(gomock.Eq(ctx), gomock.Eq(mockSnapshot.SnapshotID)).Return(true, nil).AnyTimes()

				awsDriver := controllerService{
					cloud:         mockCloud,
					inFlight:      internal.NewInFlight(),
					driverOptions: &DriverOptions{},
				}

				_, err := awsDriver.CreateSnapshot(context.Background(), req)
				if err == nil {
					t.Fatalf("Expected error, got nil")
				}
			},
		},
		{
			name: "fail with EnableFastSnapshotRestore - invalid availability zones",
			testFunc: func(t *testing.T) {
				const (
					snapshotName = "test-snapshot"
				)

				req := &csi.CreateSnapshotRequest{
					Name: snapshotName,
					Parameters: map[string]string{
						"fastSnapshotRestoreAvailabilityZones": "invalid-az, us-east-1b",
					},
					SourceVolumeId: "vol-test",
				}

				ctx := context.Background()
				mockCtl := gomock.NewController(t)
				defer mockCtl.Finish()

				mockCloud := cloud.NewMockCloud(mockCtl)
				mockCloud.EXPECT().GetSnapshotByName(gomock.Eq(ctx), gomock.Eq(req.GetName())).Return(nil, cloud.ErrNotFound).AnyTimes()
				mockCloud.EXPECT().AvailabilityZones(gomock.Eq(ctx)).Return(map[string]struct{}{
					"us-east-1a": {}, "us-east-1b": {}}, nil).AnyTimes()

				awsDriver := controllerService{
					cloud:         mockCloud,
					inFlight:      internal.NewInFlight(),
					driverOptions: &DriverOptions{},
				}

				_, err := awsDriver.CreateSnapshot(context.Background(), req)
				if err == nil {
					t.Fatalf("Expected error, got nil")
				}
			},
		},
		{
			name: "fail with EnableFastSnapshotRestore",
			testFunc: func(t *testing.T) {
				const (
					snapshotName = "test-snapshot"
				)

				req := &csi.CreateSnapshotRequest{
					Name: snapshotName,
					Parameters: map[string]string{
						"fastSnapshotRestoreAvailabilityZones": "us-east-1a, us-east-1f",
					},
					SourceVolumeId: "vol-test",
				}

				ctx := context.Background()
				mockSnapshot := &cloud.Snapshot{
					SnapshotID:     fmt.Sprintf("snapshot-%d", rand.New(rand.NewSource(time.Now().UnixNano())).Uint64()),
					SourceVolumeID: req.SourceVolumeId,
					Size:           1,
					CreationTime:   time.Now(),
				}
				mockCtl := gomock.NewController(t)
				defer mockCtl.Finish()

				snapshotOptions := &cloud.SnapshotOptions{
					Tags: map[string]string{
						cloud.SnapshotNameTagKey: snapshotName,
						cloud.AwsEbsDriverTagKey: isManagedByDriver,
					},
				}

				mockCloud := cloud.NewMockCloud(mockCtl)
				mockCloud.EXPECT().GetSnapshotByName(gomock.Eq(ctx), gomock.Eq(req.GetName())).Return(nil, cloud.ErrNotFound).AnyTimes()
				mockCloud.EXPECT().AvailabilityZones(gomock.Eq(ctx)).Return(map[string]struct{}{
					"us-east-1a": {}, "us-east-1f": {}}, nil).AnyTimes()
				mockCloud.EXPECT().CreateSnapshot(gomock.Eq(ctx), gomock.Eq(req.SourceVolumeId), gomock.Eq(snapshotOptions)).Return(mockSnapshot, nil).AnyTimes()
				mockCloud.EXPECT().EnableFastSnapshotRestores(gomock.Eq(ctx), gomock.Eq([]string{"us-east-1a", "us-east-1f"}),
					gomock.Eq(mockSnapshot.SnapshotID)).Return(nil, fmt.Errorf("error")).AnyTimes()
				mockCloud.EXPECT().DeleteSnapshot(gomock.Eq(ctx), gomock.Eq(mockSnapshot.SnapshotID)).Return(true, nil).AnyTimes()

				awsDriver := controllerService{
					cloud:         mockCloud,
					inFlight:      internal.NewInFlight(),
					driverOptions: &DriverOptions{},
				}

				_, err := awsDriver.CreateSnapshot(context.Background(), req)
				if err == nil {
					t.Fatalf("Expected error, got nil")
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
				ctx := context.Background()

				mockCtl := gomock.NewController(t)
				defer mockCtl.Finish()
				mockCloud := cloud.NewMockCloud(mockCtl)

				awsDriver := controllerService{
					cloud:         mockCloud,
					inFlight:      internal.NewInFlight(),
					driverOptions: &DriverOptions{},
				}

				req := &csi.DeleteSnapshotRequest{
					SnapshotId: "xxx",
				}

				mockCloud.EXPECT().DeleteSnapshot(gomock.Eq(ctx), gomock.Eq("xxx")).Return(true, nil)
				if _, err := awsDriver.DeleteSnapshot(ctx, req); err != nil {
					t.Fatalf("Unexpected error: %v", err)
				}
			},
		},
		{
			name: "success not found",
			testFunc: func(t *testing.T) {
				ctx := context.Background()

				mockCtl := gomock.NewController(t)
				defer mockCtl.Finish()
				mockCloud := cloud.NewMockCloud(mockCtl)

				awsDriver := controllerService{
					cloud:         mockCloud,
					inFlight:      internal.NewInFlight(),
					driverOptions: &DriverOptions{},
				}

				req := &csi.DeleteSnapshotRequest{
					SnapshotId: "xxx",
				}

				mockCloud.EXPECT().DeleteSnapshot(gomock.Eq(ctx), gomock.Eq("xxx")).Return(false, cloud.ErrNotFound)
				if _, err := awsDriver.DeleteSnapshot(ctx, req); err != nil {
					t.Fatalf("Unexpected error: %v", err)
				}
			},
		},
		{
			name: "fail with another request in-flight",
			testFunc: func(t *testing.T) {
				ctx := context.Background()

				mockCtl := gomock.NewController(t)
				defer mockCtl.Finish()

				mockCloud := cloud.NewMockCloud(mockCtl)

				req := &csi.DeleteSnapshotRequest{
					SnapshotId: "test-snapshotID",
				}
				inFlight := internal.NewInFlight()
				inFlight.Insert(req.GetSnapshotId())
				defer inFlight.Delete(req.GetSnapshotId())

				awsDriver := controllerService{
					cloud:         mockCloud,
					inFlight:      inFlight,
					driverOptions: &DriverOptions{},
				}

				_, err := awsDriver.DeleteSnapshot(ctx, req)

				checkExpectedErrorCode(t, err, codes.Aborted)
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
			name: "success normal",
			testFunc: func(t *testing.T) {
				req := &csi.ListSnapshotsRequest{}
				mockCloudSnapshotsResponse := &cloud.ListSnapshotsResponse{
					Snapshots: []*cloud.Snapshot{
						{
							SnapshotID:     "snapshot-1",
							SourceVolumeID: "test-vol",
							Size:           1,
							CreationTime:   time.Now(),
						},
						{
							SnapshotID:     "snapshot-2",
							SourceVolumeID: "test-vol",
							Size:           1,
							CreationTime:   time.Now(),
						},
					},
					NextToken: "",
				}

				ctx := context.Background()
				mockCtl := gomock.NewController(t)
				defer mockCtl.Finish()

				mockCloud := cloud.NewMockCloud(mockCtl)
				mockCloud.EXPECT().ListSnapshots(gomock.Eq(ctx), gomock.Eq(""), gomock.Eq(int64(0)), gomock.Eq("")).Return(mockCloudSnapshotsResponse, nil)

				awsDriver := controllerService{
					cloud:         mockCloud,
					inFlight:      internal.NewInFlight(),
					driverOptions: &DriverOptions{},
				}

				resp, err := awsDriver.ListSnapshots(context.Background(), req)
				if err != nil {
					t.Fatalf("Unexpected error: %v", err)
				}

				if len(resp.GetEntries()) != len(mockCloudSnapshotsResponse.Snapshots) {
					t.Fatalf("Expected %d entries, got %d", len(mockCloudSnapshotsResponse.Snapshots), len(resp.GetEntries()))
				}
			},
		},
		{
			name: "success no snapshots",
			testFunc: func(t *testing.T) {
				req := &csi.ListSnapshotsRequest{}
				ctx := context.Background()
				mockCtl := gomock.NewController(t)
				defer mockCtl.Finish()

				mockCloud := cloud.NewMockCloud(mockCtl)
				mockCloud.EXPECT().ListSnapshots(gomock.Eq(ctx), gomock.Eq(""), gomock.Eq(int64(0)), gomock.Eq("")).Return(nil, cloud.ErrNotFound)

				awsDriver := controllerService{
					cloud:         mockCloud,
					inFlight:      internal.NewInFlight(),
					driverOptions: &DriverOptions{},
				}

				resp, err := awsDriver.ListSnapshots(context.Background(), req)
				if err != nil {
					t.Fatalf("Unexpected error: %v", err)
				}

				if !reflect.DeepEqual(resp, &csi.ListSnapshotsResponse{}) {
					t.Fatalf("Expected empty response, got %+v", resp)
				}
			},
		},
		{
			name: "success snapshot ID",
			testFunc: func(t *testing.T) {
				req := &csi.ListSnapshotsRequest{
					SnapshotId: "snapshot-1",
				}
				mockCloudSnapshotsResponse := &cloud.Snapshot{
					SnapshotID:     "snapshot-1",
					SourceVolumeID: "test-vol",
					Size:           1,
					CreationTime:   time.Now(),
				}

				ctx := context.Background()
				mockCtl := gomock.NewController(t)
				defer mockCtl.Finish()

				mockCloud := cloud.NewMockCloud(mockCtl)
				mockCloud.EXPECT().GetSnapshotByID(gomock.Eq(ctx), gomock.Eq("snapshot-1")).Return(mockCloudSnapshotsResponse, nil)

				awsDriver := controllerService{
					cloud:         mockCloud,
					inFlight:      internal.NewInFlight(),
					driverOptions: &DriverOptions{},
				}

				resp, err := awsDriver.ListSnapshots(context.Background(), req)
				if err != nil {
					t.Fatalf("Unexpected error: %v", err)
				}

				if len(resp.GetEntries()) != 1 {
					t.Fatalf("Expected %d entry, got %d", 1, len(resp.GetEntries()))
				}
			},
		},
		{
			name: "success snapshot ID not found",
			testFunc: func(t *testing.T) {
				req := &csi.ListSnapshotsRequest{
					SnapshotId: "snapshot-1",
				}

				ctx := context.Background()
				mockCtl := gomock.NewController(t)
				defer mockCtl.Finish()

				mockCloud := cloud.NewMockCloud(mockCtl)
				mockCloud.EXPECT().GetSnapshotByID(gomock.Eq(ctx), gomock.Eq("snapshot-1")).Return(nil, cloud.ErrNotFound)

				awsDriver := controllerService{
					cloud:         mockCloud,
					inFlight:      internal.NewInFlight(),
					driverOptions: &DriverOptions{},
				}

				resp, err := awsDriver.ListSnapshots(context.Background(), req)
				if err != nil {
					t.Fatalf("Unexpected error: %v", err)
				}

				if !reflect.DeepEqual(resp, &csi.ListSnapshotsResponse{}) {
					t.Fatalf("Expected empty response, got %+v", resp)
				}
			},
		},
		{
			name: "fail snapshot ID multiple found",
			testFunc: func(t *testing.T) {
				req := &csi.ListSnapshotsRequest{
					SnapshotId: "snapshot-1",
				}

				ctx := context.Background()
				mockCtl := gomock.NewController(t)
				defer mockCtl.Finish()

				mockCloud := cloud.NewMockCloud(mockCtl)
				mockCloud.EXPECT().GetSnapshotByID(gomock.Eq(ctx), gomock.Eq("snapshot-1")).Return(nil, cloud.ErrMultiSnapshots)

				awsDriver := controllerService{
					cloud:         mockCloud,
					inFlight:      internal.NewInFlight(),
					driverOptions: &DriverOptions{},
				}

				if _, err := awsDriver.ListSnapshots(context.Background(), req); err != nil {
					srvErr, ok := status.FromError(err)
					if !ok {
						t.Fatalf("Could not get error status code from error: %v", srvErr)
					}
					if srvErr.Code() != codes.Internal {
						t.Fatalf("Expected error code %d, got %d message %s", codes.Internal, srvErr.Code(), srvErr.Message())
					}
				} else {
					t.Fatalf("Expected error code %d, got no error", codes.Internal)
				}
			},
		},
		{
			name: "fail 0 < MaxEntries < 5",
			testFunc: func(t *testing.T) {
				req := &csi.ListSnapshotsRequest{
					MaxEntries: 4,
				}

				ctx := context.Background()
				mockCtl := gomock.NewController(t)
				defer mockCtl.Finish()
				mockCloud := cloud.NewMockCloud(mockCtl)
				mockCloud.EXPECT().ListSnapshots(gomock.Eq(ctx), gomock.Eq(""), gomock.Eq(int64(4)), gomock.Eq("")).Return(nil, cloud.ErrInvalidMaxResults)

				awsDriver := controllerService{
					cloud:         mockCloud,
					inFlight:      internal.NewInFlight(),
					driverOptions: &DriverOptions{},
				}

				if _, err := awsDriver.ListSnapshots(context.Background(), req); err != nil {
					srvErr, ok := status.FromError(err)
					if !ok {
						t.Fatalf("Could not get error status code from error: %v", srvErr)
					}
					if srvErr.Code() != codes.InvalidArgument {
						t.Fatalf("Expected error code %d, got %d message %s", codes.InvalidArgument, srvErr.Code(), srvErr.Message())
					}
				} else {
					t.Fatalf("Expected error code %d, got no error", codes.InvalidArgument)
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

	testCases := []struct {
		name             string
		volumeId         string
		nodeId           string
		volumeCapability *csi.VolumeCapability
		mockAttach       func(mockCloud *cloud.MockCloud, ctx context.Context, volumeId string, nodeId string)
		expResp          *csi.ControllerPublishVolumeResponse
		errorCode        codes.Code
		setupFunc        func(controllerService *controllerService)
	}{
		{
			name:             "AttachDisk successfully with valid volume ID, node ID, and volume capability",
			volumeId:         "vol-test",
			nodeId:           expInstanceID,
			volumeCapability: stdVolCap,
			mockAttach: func(mockCloud *cloud.MockCloud, ctx context.Context, volumeId string, nodeId string) {
				mockCloud.EXPECT().AttachDisk(gomock.Eq(ctx), volumeId, gomock.Eq(nodeId)).Return(expDevicePath, nil)
			},
			expResp: &csi.ControllerPublishVolumeResponse{
				PublishContext: map[string]string{DevicePathKey: expDevicePath},
			},
			errorCode: codes.OK,
		},
		{
			name:             "AttachDisk when volume is already attached to the node",
			volumeId:         "vol-test",
			nodeId:           expInstanceID,
			volumeCapability: stdVolCap,
			mockAttach: func(mockCloud *cloud.MockCloud, ctx context.Context, volumeId string, nodeId string) {
				mockCloud.EXPECT().AttachDisk(gomock.Eq(ctx), gomock.Eq(volumeId), gomock.Eq(expInstanceID)).Return(expDevicePath, nil)
			},
			expResp: &csi.ControllerPublishVolumeResponse{
				PublishContext: map[string]string{DevicePathKey: expDevicePath},
			},
			errorCode: codes.OK,
		},

		{
			name:             "Invalid argument error when no VolumeId provided",
			volumeId:         "",
			nodeId:           expInstanceID,
			volumeCapability: stdVolCap,
			errorCode:        codes.InvalidArgument,
		},
		{
			name:             "Invalid argument error when no NodeId provided",
			volumeId:         "vol-test",
			nodeId:           "",
			volumeCapability: stdVolCap,
			errorCode:        codes.InvalidArgument,
		},
		{
			name:      "Invalid argument error when no VolumeCapability provided",
			volumeId:  "vol-test",
			nodeId:    expInstanceID,
			errorCode: codes.InvalidArgument,
		},
		{
			name:     "Invalid argument error when invalid VolumeCapability provided",
			volumeId: "vol-test",
			nodeId:   expInstanceID,
			volumeCapability: &csi.VolumeCapability{
				AccessMode: &csi.VolumeCapability_AccessMode{
					Mode: csi.VolumeCapability_AccessMode_UNKNOWN,
				},
			},
			errorCode: codes.InvalidArgument,
		},
		{
			name:             "Internal error when AttachDisk fails",
			volumeId:         "vol-test",
			nodeId:           expInstanceID,
			volumeCapability: stdVolCap,
			mockAttach: func(mockCloud *cloud.MockCloud, ctx context.Context, volumeId string, nodeId string) {
				mockCloud.EXPECT().AttachDisk(gomock.Eq(ctx), gomock.Eq(volumeId), gomock.Eq(expInstanceID)).Return("", status.Error(codes.Internal, "test error"))
			},
			errorCode: codes.Internal,
		},
		{
			name:             "Fail when node does not exist",
			volumeId:         "vol-test",
			nodeId:           expInstanceID,
			volumeCapability: stdVolCap,
			mockAttach: func(mockCloud *cloud.MockCloud, ctx context.Context, volumeId string, nodeId string) {
				mockCloud.EXPECT().AttachDisk(gomock.Eq(ctx), gomock.Eq(volumeId), gomock.Eq(nodeId)).Return("", status.Error(codes.Internal, "test error"))
			},
			errorCode: codes.Internal,
		},
		{
			name:             "Fail when volume does not exist",
			volumeId:         "vol-test",
			nodeId:           expInstanceID,
			volumeCapability: stdVolCap,
			mockAttach: func(mockCloud *cloud.MockCloud, ctx context.Context, volumeId string, nodeId string) {
				mockCloud.EXPECT().AttachDisk(gomock.Eq(ctx), gomock.Eq(volumeId), gomock.Eq(expInstanceID)).Return("", status.Error(codes.Internal, "volume not found"))
			},
			errorCode: codes.Internal,
		},
		{
			name:             "Fail when volume is already attached to another node",
			volumeId:         "vol-test",
			nodeId:           expInstanceID,
			volumeCapability: stdVolCap,
			mockAttach: func(mockCloud *cloud.MockCloud, ctx context.Context, volumeId string, nodeId string) {
				mockCloud.EXPECT().AttachDisk(gomock.Eq(ctx), gomock.Eq(volumeId), gomock.Eq(expInstanceID)).Return("", (cloud.ErrVolumeInUse))
			},
			errorCode: codes.FailedPrecondition,
		},
		{
			name:             "Aborted error when AttachDisk operation already in-flight",
			volumeId:         "vol-test",
			nodeId:           expInstanceID,
			volumeCapability: stdVolCap,
			mockAttach: func(mockCloud *cloud.MockCloud, ctx context.Context, volumeId string, nodeId string) {
			},
			errorCode: codes.Aborted,
			setupFunc: func(controllerService *controllerService) {
				controllerService.inFlight.Insert("vol-test")
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req := &csi.ControllerPublishVolumeRequest{
				NodeId:           tc.nodeId,
				VolumeCapability: tc.volumeCapability,
				VolumeId:         tc.volumeId,
			}
			ctx := context.Background()

			awsDriver, mockCtl, mockCloud := createControllerService(t)
			defer mockCtl.Finish()

			if tc.setupFunc != nil {
				tc.setupFunc(&awsDriver)
			}

			if tc.mockAttach != nil {
				tc.mockAttach(mockCloud, ctx, req.VolumeId, req.NodeId)
			}

			resp, err := awsDriver.ControllerPublishVolume(ctx, req)
			if tc.errorCode != codes.OK {
				assert.Equal(t, tc.errorCode, status.Code(err))
				assert.Nil(t, resp)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, resp)
				assert.Equal(t, tc.expResp, resp)
			}
		})
	}
}

func TestControllerUnpublishVolume(t *testing.T) {
	testCases := []struct {
		name       string
		volumeId   string
		nodeId     string
		errorCode  codes.Code
		mockDetach func(mockCloud *cloud.MockCloud, ctx context.Context, volumeId string, nodeId string)
		expResp    *csi.ControllerUnpublishVolumeResponse
		setupFunc  func(driver *controllerService)
	}{
		{
			name:      "DetachDisk successfully with valid volume ID and node ID",
			volumeId:  "vol-test",
			nodeId:    expInstanceID,
			errorCode: codes.OK,
			mockDetach: func(mockCloud *cloud.MockCloud, ctx context.Context, volumeId string, nodeId string) {
				mockCloud.EXPECT().DetachDisk(gomock.Eq(ctx), volumeId, nodeId).Return(nil)

			},
			expResp: &csi.ControllerUnpublishVolumeResponse{},
		},
		{
			name:      "Return success when volume not found during DetachDisk operation",
			volumeId:  "vol-not-found",
			nodeId:    expInstanceID,
			errorCode: codes.OK,
			mockDetach: func(mockCloud *cloud.MockCloud, ctx context.Context, volumeId string, nodeId string) {
				mockCloud.EXPECT().DetachDisk(gomock.Eq(ctx), volumeId, nodeId).Return(cloud.ErrNotFound)
			},
			expResp: &csi.ControllerUnpublishVolumeResponse{},
		},
		{
			name:      "Invalid argument error when no VolumeId provided",
			volumeId:  "",
			nodeId:    expInstanceID,
			errorCode: codes.InvalidArgument,
		},
		{
			name:      "Invalid argument error when no NodeId provided",
			volumeId:  "vol-test",
			nodeId:    "",
			errorCode: codes.InvalidArgument,
		},
		{
			name:      "Internal error when DetachDisk operation fails",
			volumeId:  "vol-test",
			nodeId:    expInstanceID,
			errorCode: codes.Internal,
			mockDetach: func(mockCloud *cloud.MockCloud, ctx context.Context, volumeId string, nodeId string) {
				mockCloud.EXPECT().DetachDisk(gomock.Eq(ctx), volumeId, nodeId).Return(errors.New("test error"))
			},
		},
		{
			name:      "Aborted error when operation already in-flight",
			volumeId:  "vol-test",
			nodeId:    expInstanceID,
			errorCode: codes.Aborted,
			setupFunc: func(driver *controllerService) {
				driver.inFlight.Insert("vol-test")
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req := &csi.ControllerUnpublishVolumeRequest{
				NodeId:   tc.nodeId,
				VolumeId: tc.volumeId,
			}

			ctx := context.Background()

			awsDriver, mockCtl, mockCloud := createControllerService(t)
			defer mockCtl.Finish()

			if tc.setupFunc != nil {
				tc.setupFunc(&awsDriver)
			}

			if tc.mockDetach != nil {
				tc.mockDetach(mockCloud, ctx, req.VolumeId, req.NodeId)
			}

			resp, err := awsDriver.ControllerUnpublishVolume(ctx, req)
			if tc.errorCode != codes.OK {
				assert.Equal(t, tc.errorCode, status.Code(err))
				assert.Nil(t, resp)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, resp)
				assert.Equal(t, tc.expResp, resp)
			}
		})
	}
}

func TestControllerExpandVolume(t *testing.T) {
	testCases := []struct {
		name     string
		req      *csi.ControllerExpandVolumeRequest
		newSize  int64
		expResp  *csi.ControllerExpandVolumeResponse
		expError bool
	}{
		{
			name: "success normal",
			req: &csi.ControllerExpandVolumeRequest{
				VolumeId: "vol-test",
				CapacityRange: &csi.CapacityRange{
					RequiredBytes: 5 * util.GiB,
				},
			},
			expResp: &csi.ControllerExpandVolumeResponse{
				CapacityBytes: 5 * util.GiB,
			},
		},
		{
			name:     "fail empty request",
			req:      &csi.ControllerExpandVolumeRequest{},
			expError: true,
		},
		{
			name: "fail exceeds limit after round up",
			req: &csi.ControllerExpandVolumeRequest{
				VolumeId: "vol-test",
				CapacityRange: &csi.CapacityRange{
					RequiredBytes: 5*util.GiB + 1, // should round up to 6 GiB
					LimitBytes:    5 * util.GiB,
				},
			},
			expError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			mockCtl := gomock.NewController(t)
			defer mockCtl.Finish()

			var retSizeGiB int64
			if tc.newSize != 0 {
				retSizeGiB = tc.newSize
			} else {
				retSizeGiB = util.BytesToGiB(tc.req.CapacityRange.GetRequiredBytes())
			}

			mockCloud := cloud.NewMockCloud(mockCtl)
			mockCloud.EXPECT().ResizeOrModifyDisk(gomock.Any(), gomock.Eq(tc.req.VolumeId), gomock.Any(), gomock.Any()).Return(retSizeGiB, nil).AnyTimes()

			awsDriver := controllerService{
				cloud:               mockCloud,
				inFlight:            internal.NewInFlight(),
				driverOptions:       &DriverOptions{},
				modifyVolumeManager: newModifyVolumeManager(),
			}

			resp, err := awsDriver.ControllerExpandVolume(ctx, tc.req)
			if err != nil {
				srvErr, ok := status.FromError(err)
				if !ok {
					t.Fatalf("Could not get error status code from error: %v", srvErr)
				}
				if !tc.expError {
					t.Fatalf("Unexpected error: %v", err)
				}
			} else {
				if tc.expError {
					t.Fatalf("Expected error from ControllerExpandVolume, got nothing")
				}
			}

			sizeGiB := util.BytesToGiB(resp.GetCapacityBytes())
			expSizeGiB := util.BytesToGiB(tc.expResp.GetCapacityBytes())
			if sizeGiB != expSizeGiB {
				t.Fatalf("Expected size %d GiB, got %d GiB", expSizeGiB, sizeGiB)
			}
		})
	}
}

func checkExpectedErrorCode(t *testing.T, err error, expectedCode codes.Code) {
	if err == nil {
		t.Fatalf("Expected operation to fail but got no error")
	}

	srvErr, ok := status.FromError(err)
	if !ok {
		t.Fatalf("Could not get error status code from error: %v", srvErr)
	}
	if srvErr.Code() != expectedCode {
		t.Fatalf("Expected Aborted but got: %s", srvErr.Code())
	}
}

func createControllerService(t *testing.T) (controllerService, *gomock.Controller, *cloud.MockCloud) {
	t.Helper()
	mockCtl := gomock.NewController(t)
	mockCloud := cloud.NewMockCloud(mockCtl)
	awsDriver := controllerService{
		cloud:         mockCloud,
		inFlight:      internal.NewInFlight(),
		driverOptions: &DriverOptions{},
	}
	return awsDriver, mockCtl, mockCloud
}
