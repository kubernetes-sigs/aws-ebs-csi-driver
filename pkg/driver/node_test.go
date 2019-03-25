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
	"errors"
	"reflect"
	"testing"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/golang/mock/gomock"
	"github.com/kubernetes-sigs/aws-ebs-csi-driver/pkg/driver/internal"
	"github.com/kubernetes-sigs/aws-ebs-csi-driver/pkg/driver/mocks"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestNodeStageVolume(t *testing.T) {

	var (
		targetPath = "/test/path"
		devicePath = "/dev/fake"
		stdVolCap  = &csi.VolumeCapability{
			AccessType: &csi.VolumeCapability_Mount{
				Mount: &csi.VolumeCapability_MountVolume{},
			},
			AccessMode: &csi.VolumeCapability_AccessMode{
				Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER,
			},
		}
	)
	testCases := []struct {
		name     string
		testFunc func(t *testing.T)
	}{
		{
			name: "success normal",
			testFunc: func(t *testing.T) {
				mockCtl := gomock.NewController(t)
				defer mockCtl.Finish()

				mockMetadata := mocks.NewMockMetadataService(mockCtl)
				mockMounter := mocks.NewMockMounter(mockCtl)

				awsDriver := &nodeService{
					metadata: mockMetadata,
					mounter:  mockMounter,
					inFlight: internal.NewInFlight(),
				}

				req := &csi.NodeStageVolumeRequest{
					PublishContext:    map[string]string{DevicePathKey: devicePath},
					StagingTargetPath: targetPath,
					VolumeCapability:  stdVolCap,
					VolumeId:          "vol-test",
				}

				gomock.InOrder(
					mockMounter.EXPECT().ExistsPath(gomock.Eq(devicePath)).Return(true, nil),
					mockMounter.EXPECT().ExistsPath(gomock.Eq(targetPath)).Return(false, nil),
				)

				mockMounter.EXPECT().MakeDir(targetPath).Return(nil)
				mockMounter.EXPECT().GetDeviceName(targetPath).Return("", 1, nil)
				mockMounter.EXPECT().FormatAndMount(gomock.Eq(devicePath), gomock.Eq(targetPath), gomock.Eq(defaultFsType), gomock.Any())
				_, err := awsDriver.NodeStageVolume(context.TODO(), req)
				if err != nil {
					t.Fatalf("Expect no error but got: %v", err)
				}
			},
		},
		{
			name: "success normal [raw block]",
			testFunc: func(t *testing.T) {
				mockCtl := gomock.NewController(t)
				defer mockCtl.Finish()

				mockMetadata := mocks.NewMockMetadataService(mockCtl)
				mockMounter := mocks.NewMockMounter(mockCtl)

				awsDriver := nodeService{
					metadata: mockMetadata,
					mounter:  mockMounter,
					inFlight: internal.NewInFlight(),
				}

				req := &csi.NodeStageVolumeRequest{
					PublishContext:    map[string]string{DevicePathKey: "/dev/fake"},
					StagingTargetPath: "/test/path",
					VolumeCapability: &csi.VolumeCapability{
						AccessType: &csi.VolumeCapability_Block{
							Block: &csi.VolumeCapability_BlockVolume{},
						},
						AccessMode: &csi.VolumeCapability_AccessMode{
							Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER,
						},
					},
					VolumeId: "vol-test",
				}

				_, err := awsDriver.NodeStageVolume(context.TODO(), req)
				if err != nil {
					t.Fatalf("Expect no error but got: %v", err)
				}
			},
		},
		{
			name: "success mount options fsType ext3",
			testFunc: func(t *testing.T) {
				mockCtl := gomock.NewController(t)
				defer mockCtl.Finish()

				mockMetadata := mocks.NewMockMetadataService(mockCtl)
				mockMounter := mocks.NewMockMounter(mockCtl)

				awsDriver := nodeService{
					metadata: mockMetadata,
					mounter:  mockMounter,
					inFlight: internal.NewInFlight(),
				}

				req := &csi.NodeStageVolumeRequest{
					PublishContext:    map[string]string{DevicePathKey: devicePath},
					StagingTargetPath: targetPath,
					VolumeCapability:  stdVolCap,
					VolumeContext:     map[string]string{FsTypeKey: FSTypeExt3},
					VolumeId:          "vol-test",
				}

				gomock.InOrder(
					mockMounter.EXPECT().ExistsPath(gomock.Eq(devicePath)).Return(true, nil),
					mockMounter.EXPECT().ExistsPath(gomock.Eq(targetPath)).Return(false, nil),
				)

				mockMounter.EXPECT().MakeDir(targetPath).Return(nil)
				mockMounter.EXPECT().GetDeviceName(targetPath).Return("", 1, nil)
				mockMounter.EXPECT().FormatAndMount(gomock.Eq(devicePath), gomock.Eq(targetPath), gomock.Eq(FSTypeExt3), gomock.Any())
				_, err := awsDriver.NodeStageVolume(context.TODO(), req)
				if err != nil {
					t.Fatalf("Expect no error but got: %v", err)
				}
			},
		},
		{
			name: "fail no VolumeId",
			testFunc: func(t *testing.T) {
				mockCtl := gomock.NewController(t)
				defer mockCtl.Finish()

				mockMetadata := mocks.NewMockMetadataService(mockCtl)
				mockMounter := mocks.NewMockMounter(mockCtl)

				awsDriver := nodeService{
					metadata: mockMetadata,
					mounter:  mockMounter,
					inFlight: internal.NewInFlight(),
				}

				req := &csi.NodeStageVolumeRequest{
					PublishContext:    map[string]string{DevicePathKey: devicePath},
					StagingTargetPath: targetPath,
					VolumeCapability:  stdVolCap,
				}

				_, err := awsDriver.NodeStageVolume(context.TODO(), req)
				if err == nil {
					t.Fatalf("Expect error but got no error")
				}

				expectErr(t, err, codes.InvalidArgument)
			},
		},
		{
			name: "fail no StagingTargetPath",
			testFunc: func(t *testing.T) {
				mockCtl := gomock.NewController(t)
				defer mockCtl.Finish()

				mockMetadata := mocks.NewMockMetadataService(mockCtl)
				mockMounter := mocks.NewMockMounter(mockCtl)

				awsDriver := nodeService{
					metadata: mockMetadata,
					mounter:  mockMounter,
					inFlight: internal.NewInFlight(),
				}

				devicePath := "/dev/fake"
				req := &csi.NodeStageVolumeRequest{
					PublishContext:   map[string]string{DevicePathKey: devicePath},
					VolumeCapability: stdVolCap,
					VolumeId:         "vol-test",
				}

				_, err := awsDriver.NodeStageVolume(context.TODO(), req)
				if err == nil {
					t.Fatalf("Expect error but got no error")
				}

				expectErr(t, err, codes.InvalidArgument)
			},
		},
		{
			name: "fail no VolumeCapability",
			testFunc: func(t *testing.T) {
				mockCtl := gomock.NewController(t)
				defer mockCtl.Finish()

				mockMetadata := mocks.NewMockMetadataService(mockCtl)
				mockMounter := mocks.NewMockMounter(mockCtl)

				awsDriver := nodeService{
					metadata: mockMetadata,
					mounter:  mockMounter,
					inFlight: internal.NewInFlight(),
				}

				req := &csi.NodeStageVolumeRequest{
					PublishContext:    map[string]string{DevicePathKey: devicePath},
					StagingTargetPath: "/test/path",
					VolumeId:          "vol-test",
				}
				_, err := awsDriver.NodeStageVolume(context.TODO(), req)
				if err == nil {
					t.Fatalf("Expect error but got no error")
				}

				expectErr(t, err, codes.InvalidArgument)
			},
		},
		{
			name: "fail invalid VolumeCapability",
			testFunc: func(t *testing.T) {
				mockCtl := gomock.NewController(t)
				defer mockCtl.Finish()

				mockMetadata := mocks.NewMockMetadataService(mockCtl)
				mockMounter := mocks.NewMockMounter(mockCtl)

				awsDriver := nodeService{
					metadata: mockMetadata,
					mounter:  mockMounter,
					inFlight: internal.NewInFlight(),
				}

				req := &csi.NodeStageVolumeRequest{
					PublishContext:    map[string]string{DevicePathKey: "/dev/fake"},
					StagingTargetPath: "/test/path",
					VolumeCapability: &csi.VolumeCapability{
						AccessMode: &csi.VolumeCapability_AccessMode{
							Mode: csi.VolumeCapability_AccessMode_UNKNOWN,
						},
					},
					VolumeId: "vol-test",
				}
				_, err := awsDriver.NodeStageVolume(context.TODO(), req)
				if err == nil {
					t.Fatalf("Expect error but got no error")
				}

				expectErr(t, err, codes.InvalidArgument)
			},
		},
		{
			name: "fail no devicePath",
			testFunc: func(t *testing.T) {
				mockCtl := gomock.NewController(t)
				defer mockCtl.Finish()

				mockMetadata := mocks.NewMockMetadataService(mockCtl)
				mockMounter := mocks.NewMockMounter(mockCtl)

				awsDriver := nodeService{
					metadata: mockMetadata,
					mounter:  mockMounter,
					inFlight: internal.NewInFlight(),
				}

				req := &csi.NodeStageVolumeRequest{
					StagingTargetPath: targetPath,
					VolumeCapability:  stdVolCap,
					VolumeId:          "vol-test",
				}
				_, err := awsDriver.NodeStageVolume(context.TODO(), req)
				if err == nil {
					t.Fatalf("Expect error but got no error")
				}

				expectErr(t, err, codes.InvalidArgument)

			},
		},
		{
			name: "success device already mounted at target",
			testFunc: func(t *testing.T) {
				mockCtl := gomock.NewController(t)
				defer mockCtl.Finish()

				mockMetadata := mocks.NewMockMetadataService(mockCtl)
				mockMounter := mocks.NewMockMounter(mockCtl)

				awsDriver := &nodeService{
					metadata: mockMetadata,
					mounter:  mockMounter,
					inFlight: internal.NewInFlight(),
				}

				req := &csi.NodeStageVolumeRequest{
					PublishContext:    map[string]string{DevicePathKey: devicePath},
					StagingTargetPath: targetPath,
					VolumeCapability:  stdVolCap,
					VolumeId:          "vol-test",
				}

				gomock.InOrder(
					mockMounter.EXPECT().ExistsPath(gomock.Eq(devicePath)).Return(true, nil),
					mockMounter.EXPECT().ExistsPath(gomock.Eq(targetPath)).Return(false, nil),
				)

				mockMounter.EXPECT().MakeDir(targetPath).Return(nil)
				mockMounter.EXPECT().GetDeviceName(targetPath).Return(devicePath, 1, nil)
				_, err := awsDriver.NodeStageVolume(context.TODO(), req)
				if err != nil {
					t.Fatalf("Expect no error but got: %v", err)
				}
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, tc.testFunc)
	}
}

func TestNodeUnstageVolume(t *testing.T) {
	targetPath := "/test/path"
	devicePath := "/dev/fake"

	testCases := []struct {
		name     string
		testFunc func(t *testing.T)
	}{
		{
			name: "success normal",
			testFunc: func(t *testing.T) {
				mockCtl := gomock.NewController(t)
				defer mockCtl.Finish()

				mockMetadata := mocks.NewMockMetadataService(mockCtl)
				mockMounter := mocks.NewMockMounter(mockCtl)

				awsDriver := &nodeService{
					metadata: mockMetadata,
					mounter:  mockMounter,
					inFlight: internal.NewInFlight(),
				}

				mockMounter.EXPECT().GetDeviceName(gomock.Eq(targetPath)).Return(devicePath, 1, nil)
				mockMounter.EXPECT().Unmount(gomock.Eq(targetPath)).Return(nil)

				req := &csi.NodeUnstageVolumeRequest{
					StagingTargetPath: targetPath,
					VolumeId:          "vol-test",
				}

				_, err := awsDriver.NodeUnstageVolume(context.TODO(), req)
				if err != nil {
					t.Fatalf("Expect no error but got: %v", err)
				}
			},
		},
		{
			name: "success no device mounted at target",
			testFunc: func(t *testing.T) {
				mockCtl := gomock.NewController(t)
				defer mockCtl.Finish()

				mockMetadata := mocks.NewMockMetadataService(mockCtl)
				mockMounter := mocks.NewMockMounter(mockCtl)

				awsDriver := &nodeService{
					metadata: mockMetadata,
					mounter:  mockMounter,
					inFlight: internal.NewInFlight(),
				}

				mockMounter.EXPECT().GetDeviceName(gomock.Eq(targetPath)).Return(devicePath, 0, nil)

				req := &csi.NodeUnstageVolumeRequest{
					StagingTargetPath: targetPath,
					VolumeId:          "vol-test",
				}
				_, err := awsDriver.NodeUnstageVolume(context.TODO(), req)
				if err != nil {
					t.Fatalf("Expect no error but got: %v", err)
				}
			},
		},
		{
			name: "success device mounted at multiple targets",
			testFunc: func(t *testing.T) {
				mockCtl := gomock.NewController(t)
				defer mockCtl.Finish()

				mockMetadata := mocks.NewMockMetadataService(mockCtl)
				mockMounter := mocks.NewMockMounter(mockCtl)

				awsDriver := &nodeService{
					metadata: mockMetadata,
					mounter:  mockMounter,
					inFlight: internal.NewInFlight(),
				}

				mockMounter.EXPECT().GetDeviceName(gomock.Eq(targetPath)).Return(devicePath, 2, nil)
				mockMounter.EXPECT().Unmount(gomock.Eq(targetPath)).Return(nil)

				req := &csi.NodeUnstageVolumeRequest{
					StagingTargetPath: targetPath,
					VolumeId:          "vol-test",
				}

				_, err := awsDriver.NodeUnstageVolume(context.TODO(), req)
				if err != nil {
					t.Fatalf("Expect no error but got: %v", err)
				}
			},
		},
		{
			name: "fail no VolumeId",
			testFunc: func(t *testing.T) {
				mockCtl := gomock.NewController(t)
				defer mockCtl.Finish()

				mockMetadata := mocks.NewMockMetadataService(mockCtl)
				mockMounter := mocks.NewMockMounter(mockCtl)

				awsDriver := &nodeService{
					metadata: mockMetadata,
					mounter:  mockMounter,
					inFlight: internal.NewInFlight(),
				}

				req := &csi.NodeUnstageVolumeRequest{
					StagingTargetPath: targetPath,
				}

				_, err := awsDriver.NodeUnstageVolume(context.TODO(), req)
				expectErr(t, err, codes.InvalidArgument)
			},
		},
		{
			name: "fail no StagingTargetPath",
			testFunc: func(t *testing.T) {
				mockCtl := gomock.NewController(t)
				defer mockCtl.Finish()

				mockMetadata := mocks.NewMockMetadataService(mockCtl)
				mockMounter := mocks.NewMockMounter(mockCtl)

				awsDriver := &nodeService{
					metadata: mockMetadata,
					mounter:  mockMounter,
					inFlight: internal.NewInFlight(),
				}

				req := &csi.NodeUnstageVolumeRequest{
					VolumeId: "vol-test",
				}
				_, err := awsDriver.NodeUnstageVolume(context.TODO(), req)
				expectErr(t, err, codes.InvalidArgument)
			},
		},
		{
			name: "fail GetDeviceName returns error",
			testFunc: func(t *testing.T) {
				mockCtl := gomock.NewController(t)
				defer mockCtl.Finish()

				mockMetadata := mocks.NewMockMetadataService(mockCtl)
				mockMounter := mocks.NewMockMounter(mockCtl)

				awsDriver := &nodeService{
					metadata: mockMetadata,
					mounter:  mockMounter,
					inFlight: internal.NewInFlight(),
				}

				mockMounter.EXPECT().GetDeviceName(gomock.Eq(targetPath)).Return("", 0, errors.New("GetDeviceName faield"))

				req := &csi.NodeUnstageVolumeRequest{
					StagingTargetPath: targetPath,
					VolumeId:          "vol-test",
				}

				_, err := awsDriver.NodeUnstageVolume(context.TODO(), req)
				expectErr(t, err, codes.Internal)
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, tc.testFunc)
	}
}

func TestNodePublishVolume(t *testing.T) {
	targetPath := "/test/path"
	stagingTargetPath := "/test/staging/path"
	devicePath := "/dev/fake"
	stdVolCap := &csi.VolumeCapability{
		AccessType: &csi.VolumeCapability_Mount{
			Mount: &csi.VolumeCapability_MountVolume{},
		},
		AccessMode: &csi.VolumeCapability_AccessMode{
			Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER,
		},
	}
	testCases := []struct {
		name     string
		testFunc func(t *testing.T)
	}{
		{
			name: "success normal",
			testFunc: func(t *testing.T) {
				mockCtl := gomock.NewController(t)
				defer mockCtl.Finish()

				mockMetadata := mocks.NewMockMetadataService(mockCtl)
				mockMounter := mocks.NewMockMounter(mockCtl)

				awsDriver := &nodeService{
					metadata: mockMetadata,
					mounter:  mockMounter,
					inFlight: internal.NewInFlight(),
				}

				mockMounter.EXPECT().MakeDir(gomock.Eq(targetPath)).Return(nil)
				mockMounter.EXPECT().Mount(gomock.Eq(stagingTargetPath), gomock.Eq(targetPath), gomock.Eq(""), gomock.Eq([]string{"bind"})).Return(nil)

				req := &csi.NodePublishVolumeRequest{
					PublishContext:    map[string]string{DevicePathKey: devicePath},
					StagingTargetPath: stagingTargetPath,
					TargetPath:        targetPath,
					VolumeCapability:  stdVolCap,
					VolumeId:          "vol-test",
				}

				_, err := awsDriver.NodePublishVolume(context.TODO(), req)
				if err != nil {
					t.Fatalf("Expect no error but got: %v", err)
				}
			},
		},
		{
			name: "success readonly",
			testFunc: func(t *testing.T) {
				mockCtl := gomock.NewController(t)
				defer mockCtl.Finish()

				mockMetadata := mocks.NewMockMetadataService(mockCtl)
				mockMounter := mocks.NewMockMounter(mockCtl)

				awsDriver := &nodeService{
					metadata: mockMetadata,
					mounter:  mockMounter,
					inFlight: internal.NewInFlight(),
				}

				mockMounter.EXPECT().MakeDir(gomock.Eq(targetPath)).Return(nil)
				mockMounter.EXPECT().Mount(gomock.Eq(stagingTargetPath), gomock.Eq(targetPath), gomock.Eq(""), gomock.Eq([]string{"bind", "ro"})).Return(nil)

				req := &csi.NodePublishVolumeRequest{
					PublishContext:    map[string]string{DevicePathKey: devicePath},
					Readonly:          true,
					StagingTargetPath: stagingTargetPath,
					TargetPath:        targetPath,
					VolumeCapability:  stdVolCap,
					VolumeId:          "vol-test",
				}

				_, err := awsDriver.NodePublishVolume(context.TODO(), req)
				if err != nil {
					t.Fatalf("Expect no error but got: %v", err)
				}
			},
		},
		{
			name: "success mount options",
			testFunc: func(t *testing.T) {
				mockCtl := gomock.NewController(t)
				defer mockCtl.Finish()

				mockMetadata := mocks.NewMockMetadataService(mockCtl)
				mockMounter := mocks.NewMockMounter(mockCtl)

				awsDriver := &nodeService{
					metadata: mockMetadata,
					mounter:  mockMounter,
					inFlight: internal.NewInFlight(),
				}

				mockMounter.EXPECT().MakeDir(gomock.Eq(targetPath)).Return(nil)
				mockMounter.EXPECT().Mount(gomock.Eq(stagingTargetPath), gomock.Eq(targetPath), gomock.Eq(""), gomock.Eq([]string{"bind", "test-flag"})).Return(nil)

				req := &csi.NodePublishVolumeRequest{
					PublishContext:    map[string]string{DevicePathKey: "/dev/fake"},
					StagingTargetPath: stagingTargetPath,
					TargetPath:        targetPath,
					VolumeCapability: &csi.VolumeCapability{
						AccessType: &csi.VolumeCapability_Mount{
							Mount: &csi.VolumeCapability_MountVolume{
								// this request will call mount with the bind option,
								// adding "bind" here we test that we don't add the
								// same option twice. "test-flag" is a canary to check
								// that the driver calls mount with that flag
								MountFlags: []string{"bind", "test-flag"},
							},
						},
						AccessMode: &csi.VolumeCapability_AccessMode{
							Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER,
						},
					},
					VolumeId: "vol-test",
				}

				_, err := awsDriver.NodePublishVolume(context.TODO(), req)
				if err != nil {
					t.Fatalf("Expect no error but got: %v", err)
				}
			},
		},
		{
			name: "success normal [raw block]",
			testFunc: func(t *testing.T) {
				mockCtl := gomock.NewController(t)
				defer mockCtl.Finish()

				mockMetadata := mocks.NewMockMetadataService(mockCtl)
				mockMounter := mocks.NewMockMounter(mockCtl)

				awsDriver := &nodeService{
					metadata: mockMetadata,
					mounter:  mockMounter,
					inFlight: internal.NewInFlight(),
				}

				gomock.InOrder(
					mockMounter.EXPECT().ExistsPath(gomock.Eq(devicePath)).Return(true, nil),
					mockMounter.EXPECT().ExistsPath(gomock.Eq("/test")).Return(false, nil),
				)
				mockMounter.EXPECT().MakeDir(gomock.Eq("/test")).Return(nil)
				mockMounter.EXPECT().MakeFile(targetPath).Return(nil)
				mockMounter.EXPECT().Mount(gomock.Eq(devicePath), gomock.Eq(targetPath), gomock.Eq(""), gomock.Eq([]string{"bind"})).Return(nil)

				req := &csi.NodePublishVolumeRequest{
					PublishContext:    map[string]string{DevicePathKey: "/dev/fake"},
					StagingTargetPath: stagingTargetPath,
					TargetPath:        targetPath,
					VolumeCapability: &csi.VolumeCapability{
						AccessType: &csi.VolumeCapability_Block{
							Block: &csi.VolumeCapability_BlockVolume{},
						},
						AccessMode: &csi.VolumeCapability_AccessMode{
							Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER,
						},
					},
					VolumeId: "vol-test",
				}

				_, err := awsDriver.NodePublishVolume(context.TODO(), req)
				if err != nil {
					t.Fatalf("Expect no error but got: %v", err)
				}
			},
		},
		{
			name: "fail no device path [raw block]",
			testFunc: func(t *testing.T) {
				mockCtl := gomock.NewController(t)
				defer mockCtl.Finish()

				mockMetadata := mocks.NewMockMetadataService(mockCtl)
				mockMounter := mocks.NewMockMounter(mockCtl)

				awsDriver := &nodeService{
					metadata: mockMetadata,
					mounter:  mockMounter,
					inFlight: internal.NewInFlight(),
				}

				req := &csi.NodePublishVolumeRequest{
					StagingTargetPath: stagingTargetPath,
					TargetPath:        targetPath,
					VolumeCapability: &csi.VolumeCapability{
						AccessType: &csi.VolumeCapability_Block{
							Block: &csi.VolumeCapability_BlockVolume{},
						},
						AccessMode: &csi.VolumeCapability_AccessMode{
							Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER,
						},
					},
					VolumeId: "vol-test",
				}

				_, err := awsDriver.NodePublishVolume(context.TODO(), req)
				expectErr(t, err, codes.InvalidArgument)

			},
		},
		{
			name: "fail to find deivce path [raw block]",
			testFunc: func(t *testing.T) {
				mockCtl := gomock.NewController(t)
				defer mockCtl.Finish()

				mockMetadata := mocks.NewMockMetadataService(mockCtl)
				mockMounter := mocks.NewMockMounter(mockCtl)

				awsDriver := &nodeService{
					metadata: mockMetadata,
					mounter:  mockMounter,
					inFlight: internal.NewInFlight(),
				}

				req := &csi.NodePublishVolumeRequest{
					PublishContext:    map[string]string{DevicePathKey: "/dev/fake"},
					StagingTargetPath: stagingTargetPath,
					TargetPath:        targetPath,
					VolumeCapability: &csi.VolumeCapability{
						AccessType: &csi.VolumeCapability_Block{
							Block: &csi.VolumeCapability_BlockVolume{},
						},
						AccessMode: &csi.VolumeCapability_AccessMode{
							Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER,
						},
					},
					VolumeId: "vol-test",
				}

				mockMounter.EXPECT().ExistsPath(gomock.Eq(devicePath)).Return(false, errors.New("findDevicePath failed"))

				_, err := awsDriver.NodePublishVolume(context.TODO(), req)
				expectErr(t, err, codes.Internal)

			},
		},
		{
			name: "fail no VolumeId",
			testFunc: func(t *testing.T) {
				mockCtl := gomock.NewController(t)
				defer mockCtl.Finish()

				mockMetadata := mocks.NewMockMetadataService(mockCtl)
				mockMounter := mocks.NewMockMounter(mockCtl)

				awsDriver := &nodeService{
					metadata: mockMetadata,
					mounter:  mockMounter,
					inFlight: internal.NewInFlight(),
				}

				req := &csi.NodePublishVolumeRequest{
					PublishContext:    map[string]string{DevicePathKey: devicePath},
					StagingTargetPath: stagingTargetPath,
					TargetPath:        targetPath,
					VolumeCapability:  stdVolCap,
				}

				_, err := awsDriver.NodePublishVolume(context.TODO(), req)
				expectErr(t, err, codes.InvalidArgument)

			},
		},
		{
			name: "fail no StagingTargetPath",
			testFunc: func(t *testing.T) {
				mockCtl := gomock.NewController(t)
				defer mockCtl.Finish()

				mockMetadata := mocks.NewMockMetadataService(mockCtl)
				mockMounter := mocks.NewMockMounter(mockCtl)

				awsDriver := &nodeService{
					metadata: mockMetadata,
					mounter:  mockMounter,
					inFlight: internal.NewInFlight(),
				}

				req := &csi.NodePublishVolumeRequest{
					PublishContext:   map[string]string{DevicePathKey: devicePath},
					TargetPath:       targetPath,
					VolumeCapability: stdVolCap,
					VolumeId:         "vol-test",
				}

				_, err := awsDriver.NodePublishVolume(context.TODO(), req)
				expectErr(t, err, codes.InvalidArgument)

			},
		},
		{
			name: "fail no TargetPath",
			testFunc: func(t *testing.T) {
				mockCtl := gomock.NewController(t)
				defer mockCtl.Finish()

				mockMetadata := mocks.NewMockMetadataService(mockCtl)
				mockMounter := mocks.NewMockMounter(mockCtl)

				awsDriver := &nodeService{
					metadata: mockMetadata,
					mounter:  mockMounter,
					inFlight: internal.NewInFlight(),
				}

				req := &csi.NodePublishVolumeRequest{
					PublishContext:    map[string]string{DevicePathKey: devicePath},
					StagingTargetPath: stagingTargetPath,
					VolumeCapability:  stdVolCap,
					VolumeId:          "vol-test",
				}

				_, err := awsDriver.NodePublishVolume(context.TODO(), req)
				expectErr(t, err, codes.InvalidArgument)

			},
		},
		{
			name: "fail no VolumeCapability",
			testFunc: func(t *testing.T) {
				mockCtl := gomock.NewController(t)
				defer mockCtl.Finish()

				mockMetadata := mocks.NewMockMetadataService(mockCtl)
				mockMounter := mocks.NewMockMounter(mockCtl)

				awsDriver := &nodeService{
					metadata: mockMetadata,
					mounter:  mockMounter,
					inFlight: internal.NewInFlight(),
				}

				req := &csi.NodePublishVolumeRequest{
					PublishContext:    map[string]string{DevicePathKey: devicePath},
					StagingTargetPath: stagingTargetPath,
					TargetPath:        targetPath,
					VolumeId:          "vol-test",
				}
				_, err := awsDriver.NodePublishVolume(context.TODO(), req)
				expectErr(t, err, codes.InvalidArgument)

			},
		},
		{
			name: "fail invalid VolumeCapability",
			testFunc: func(t *testing.T) {
				mockCtl := gomock.NewController(t)
				defer mockCtl.Finish()

				mockMetadata := mocks.NewMockMetadataService(mockCtl)
				mockMounter := mocks.NewMockMounter(mockCtl)

				awsDriver := &nodeService{
					metadata: mockMetadata,
					mounter:  mockMounter,
					inFlight: internal.NewInFlight(),
				}

				req := &csi.NodePublishVolumeRequest{
					PublishContext:    map[string]string{DevicePathKey: "/dev/fake"},
					StagingTargetPath: "/test/staging/path",
					TargetPath:        "/test/target/path",
					VolumeId:          "vol-test",
					VolumeCapability: &csi.VolumeCapability{
						AccessMode: &csi.VolumeCapability_AccessMode{
							Mode: csi.VolumeCapability_AccessMode_UNKNOWN,
						},
					},
				}

				_, err := awsDriver.NodePublishVolume(context.TODO(), req)
				expectErr(t, err, codes.InvalidArgument)

			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, tc.testFunc)
	}
}

func TestNodeUnpublishVolume(t *testing.T) {
	targetPath := "/test/path"

	testCases := []struct {
		name     string
		testFunc func(t *testing.T)
	}{
		{
			name: "success normal",
			testFunc: func(t *testing.T) {
				mockCtl := gomock.NewController(t)
				defer mockCtl.Finish()

				mockMetadata := mocks.NewMockMetadataService(mockCtl)
				mockMounter := mocks.NewMockMounter(mockCtl)

				awsDriver := &nodeService{
					metadata: mockMetadata,
					mounter:  mockMounter,
					inFlight: internal.NewInFlight(),
				}

				req := &csi.NodeUnpublishVolumeRequest{
					TargetPath: targetPath,
					VolumeId:   "vol-test",
				}

				mockMounter.EXPECT().Unmount(gomock.Eq(targetPath)).Return(nil)
				_, err := awsDriver.NodeUnpublishVolume(context.TODO(), req)
				if err != nil {
					t.Fatalf("Expect no error but got: %v", err)
				}
			},
		},
		{
			name: "fail no VolumeId",
			testFunc: func(t *testing.T) {
				mockCtl := gomock.NewController(t)
				defer mockCtl.Finish()

				mockMetadata := mocks.NewMockMetadataService(mockCtl)
				mockMounter := mocks.NewMockMounter(mockCtl)

				awsDriver := &nodeService{
					metadata: mockMetadata,
					mounter:  mockMounter,
					inFlight: internal.NewInFlight(),
				}

				req := &csi.NodeUnpublishVolumeRequest{
					TargetPath: targetPath,
				}

				_, err := awsDriver.NodeUnpublishVolume(context.TODO(), req)
				expectErr(t, err, codes.InvalidArgument)
			},
		},
		{
			name: "fail no TargetPath",
			testFunc: func(t *testing.T) {
				mockCtl := gomock.NewController(t)
				defer mockCtl.Finish()

				mockMetadata := mocks.NewMockMetadataService(mockCtl)
				mockMounter := mocks.NewMockMounter(mockCtl)

				awsDriver := &nodeService{
					metadata: mockMetadata,
					mounter:  mockMounter,
					inFlight: internal.NewInFlight(),
				}

				req := &csi.NodeUnpublishVolumeRequest{
					VolumeId: "vol-test",
				}

				_, err := awsDriver.NodeUnpublishVolume(context.TODO(), req)
				expectErr(t, err, codes.InvalidArgument)
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, tc.testFunc)
	}
}

func TestNodeGetVolumeStats(t *testing.T) {
	mockCtl := gomock.NewController(t)
	defer mockCtl.Finish()

	mockMetadata := mocks.NewMockMetadataService(mockCtl)
	mockMounter := mocks.NewMockMounter(mockCtl)

	awsDriver := nodeService{
		metadata: mockMetadata,
		mounter:  mockMounter,
		inFlight: internal.NewInFlight(),
	}

	expErrCode := codes.Unimplemented

	req := &csi.NodeGetVolumeStatsRequest{}
	_, err := awsDriver.NodeGetVolumeStats(context.TODO(), req)
	if err == nil {
		t.Fatalf("Expected error code %d, got nil", expErrCode)
	}
	srvErr, ok := status.FromError(err)
	if !ok {
		t.Fatalf("Could not get error status code from error: %v", srvErr)
	}
	if srvErr.Code() != expErrCode {
		t.Fatalf("Expected error code %d, got %d message %s", expErrCode, srvErr.Code(), srvErr.Message())
	}
}

func TestNodeGetCapabilities(t *testing.T) {
	mockCtl := gomock.NewController(t)
	defer mockCtl.Finish()

	mockMetadata := mocks.NewMockMetadataService(mockCtl)
	mockMounter := mocks.NewMockMounter(mockCtl)

	awsDriver := nodeService{
		metadata: mockMetadata,
		mounter:  mockMounter,
		inFlight: internal.NewInFlight(),
	}

	caps := []*csi.NodeServiceCapability{
		{
			Type: &csi.NodeServiceCapability_Rpc{
				Rpc: &csi.NodeServiceCapability_RPC{
					Type: csi.NodeServiceCapability_RPC_STAGE_UNSTAGE_VOLUME,
				},
			},
		},
	}
	expResp := &csi.NodeGetCapabilitiesResponse{Capabilities: caps}

	req := &csi.NodeGetCapabilitiesRequest{}
	resp, err := awsDriver.NodeGetCapabilities(context.TODO(), req)
	if err != nil {
		srvErr, ok := status.FromError(err)
		if !ok {
			t.Fatalf("Could not get error status code from error: %v", srvErr)
		}
		t.Fatalf("Expected nil error, got %d message %s", srvErr.Code(), srvErr.Message())
	}
	if !reflect.DeepEqual(expResp, resp) {
		t.Fatalf("Expected response {%+v}, got {%+v}", expResp, resp)
	}
}

func TestNodeGetInfo(t *testing.T) {
	testCases := []struct {
		name             string
		instanceID       string
		instanceType     string
		availabilityZone string
		expMaxVolumes    int64
	}{
		{
			name:             "success normal",
			instanceID:       "i-123456789abcdef01",
			instanceType:     "t2.medium",
			availabilityZone: "us-west-2b",
			expMaxVolumes:    39,
		},
		{
			name:             "success normal with NVMe",
			instanceID:       "i-123456789abcdef01",
			instanceType:     "m5d.large",
			availabilityZone: "us-west-2b",
			expMaxVolumes:    25,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockCtl := gomock.NewController(t)
			defer mockCtl.Finish()

			mockMetadata := mocks.NewMockMetadataService(mockCtl)
			mockMetadata.EXPECT().GetInstanceID().Return(tc.instanceID)
			mockMetadata.EXPECT().GetInstanceType().Return(tc.instanceType)
			mockMetadata.EXPECT().GetAvailabilityZone().Return(tc.availabilityZone)

			mockMounter := mocks.NewMockMounter(mockCtl)

			awsDriver := &nodeService{
				metadata: mockMetadata,
				mounter:  mockMounter,
				inFlight: internal.NewInFlight(),
			}

			resp, err := awsDriver.NodeGetInfo(context.TODO(), &csi.NodeGetInfoRequest{})
			if err != nil {
				srvErr, ok := status.FromError(err)
				if !ok {
					t.Fatalf("Could not get error status code from error: %v", srvErr)
				}
				t.Fatalf("Expected nil error, got %d message %s", srvErr.Code(), srvErr.Message())
			}

			if resp.GetNodeId() != tc.instanceID {
				t.Fatalf("Expected node ID %q, got %q", tc.instanceID, resp.GetNodeId())
			}

			at := resp.GetAccessibleTopology()
			if at.Segments[TopologyKey] != tc.availabilityZone {
				t.Fatalf("Expected topology %q, got %q", tc.availabilityZone, at.Segments[TopologyKey])
			}

			if resp.GetMaxVolumesPerNode() != tc.expMaxVolumes {
				t.Fatalf("Expected %d max volumes per node, got %d", tc.expMaxVolumes, resp.GetMaxVolumesPerNode())
			}
		})
	}
}

func expectErr(t *testing.T, actualErr error, expectedCode codes.Code) {
	if actualErr == nil {
		t.Fatalf("Expect error but got no error")
	}

	status, ok := status.FromError(actualErr)
	if !ok {
		t.Fatalf("Failed to get error status code from error: %v", actualErr)
	}

	if status.Code() != expectedCode {
		t.Fatalf("Expected error code %d, got %d message %s", codes.InvalidArgument, status.Code(), status.Message())
	}
}
