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
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go/aws/arn"
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
				Mount: &csi.VolumeCapability_MountVolume{
					FsType: FSTypeExt4,
				},
			},
			AccessMode: &csi.VolumeCapability_AccessMode{
				Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER,
			},
		}
		stdVolContext           = map[string]string{VolumeAttributePartition: "1"}
		devicePathWithPartition = devicePath + "1"
	)
	testCases := []struct {
		name         string
		request      *csi.NodeStageVolumeRequest
		inFlightFunc func(*internal.InFlight) *internal.InFlight
		expectMock   func(mockMounter mocks.MockMounter)
		expectedCode codes.Code
	}{
		{
			name: "success normal",
			request: &csi.NodeStageVolumeRequest{
				PublishContext:    map[string]string{DevicePathKey: devicePath},
				StagingTargetPath: targetPath,
				VolumeCapability:  stdVolCap,
				VolumeId:          "vol-test",
			},
			expectMock: func(mockMounter mocks.MockMounter) {
				gomock.InOrder(
					mockMounter.EXPECT().PathExists(gomock.Eq(devicePath)).Return(true, nil),
					mockMounter.EXPECT().PathExists(gomock.Eq(targetPath)).Return(false, nil),
				)
				mockMounter.EXPECT().MakeDir(targetPath).Return(nil)
				mockMounter.EXPECT().GetDeviceNameFromMount(targetPath).Return("", 1, nil)
				mockMounter.EXPECT().FormatAndMount(gomock.Eq(devicePath), gomock.Eq(targetPath), gomock.Eq(defaultFsType), gomock.Any())
				mockMounter.EXPECT().NeedResize(gomock.Eq(devicePath), gomock.Eq(targetPath)).Return(false, nil)
			},
		},
		{
			name: "success normal [raw block]",
			request: &csi.NodeStageVolumeRequest{
				PublishContext:    map[string]string{DevicePathKey: devicePath},
				StagingTargetPath: targetPath,
				VolumeCapability: &csi.VolumeCapability{
					AccessType: &csi.VolumeCapability_Block{
						Block: &csi.VolumeCapability_BlockVolume{},
					},
					AccessMode: &csi.VolumeCapability_AccessMode{
						Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER,
					},
				},
				VolumeId: "vol-test",
			},
			expectMock: func(mockMounter mocks.MockMounter) {
				mockMounter.EXPECT().FormatAndMount(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Times(0)
			},
		},
		{
			name: "success with mount options",
			request: &csi.NodeStageVolumeRequest{
				PublishContext:    map[string]string{DevicePathKey: devicePath},
				StagingTargetPath: targetPath,
				VolumeCapability: &csi.VolumeCapability{
					AccessType: &csi.VolumeCapability_Mount{
						Mount: &csi.VolumeCapability_MountVolume{
							MountFlags: []string{"dirsync", "noexec"},
						},
					},
					AccessMode: &csi.VolumeCapability_AccessMode{
						Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER,
					},
				},
				VolumeId: "vol-test",
			},
			expectMock: func(mockMounter mocks.MockMounter) {
				gomock.InOrder(
					mockMounter.EXPECT().PathExists(gomock.Eq(devicePath)).Return(true, nil),
					mockMounter.EXPECT().PathExists(gomock.Eq(targetPath)).Return(false, nil),
				)

				mockMounter.EXPECT().MakeDir(targetPath).Return(nil)
				mockMounter.EXPECT().GetDeviceNameFromMount(targetPath).Return("", 1, nil)
				mockMounter.EXPECT().FormatAndMount(gomock.Eq(devicePath), gomock.Eq(targetPath), gomock.Eq(FSTypeExt4), gomock.Eq([]string{"dirsync", "noexec"}))
				mockMounter.EXPECT().NeedResize(gomock.Eq(devicePath), gomock.Eq(targetPath)).Return(false, nil)
			},
		},
		{
			name: "success fsType ext3",
			request: &csi.NodeStageVolumeRequest{
				PublishContext:    map[string]string{DevicePathKey: devicePath},
				StagingTargetPath: targetPath,
				VolumeCapability: &csi.VolumeCapability{
					AccessType: &csi.VolumeCapability_Mount{
						Mount: &csi.VolumeCapability_MountVolume{
							FsType: FSTypeExt3,
						},
					},
					AccessMode: &csi.VolumeCapability_AccessMode{
						Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER,
					},
				},
				VolumeId: "vol-test",
			},
			expectMock: func(mockMounter mocks.MockMounter) {
				gomock.InOrder(
					mockMounter.EXPECT().PathExists(gomock.Eq(devicePath)).Return(true, nil),
					mockMounter.EXPECT().PathExists(gomock.Eq(targetPath)).Return(false, nil),
				)

				mockMounter.EXPECT().MakeDir(targetPath).Return(nil)
				mockMounter.EXPECT().GetDeviceNameFromMount(targetPath).Return("", 1, nil)
				mockMounter.EXPECT().FormatAndMount(gomock.Eq(devicePath), gomock.Eq(targetPath), gomock.Eq(FSTypeExt3), gomock.Any())
				mockMounter.EXPECT().NeedResize(gomock.Eq(devicePath), gomock.Eq(targetPath)).Return(false, nil)
			},
		},
		{
			name: "success mount with default fsType ext4",
			request: &csi.NodeStageVolumeRequest{
				PublishContext:    map[string]string{DevicePathKey: devicePath},
				StagingTargetPath: targetPath,
				VolumeCapability: &csi.VolumeCapability{
					AccessType: &csi.VolumeCapability_Mount{
						Mount: &csi.VolumeCapability_MountVolume{
							FsType: "",
						},
					},
					AccessMode: &csi.VolumeCapability_AccessMode{
						Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER,
					},
				},
				VolumeId: "vol-test",
			},
			expectMock: func(mockMounter mocks.MockMounter) {
				gomock.InOrder(
					mockMounter.EXPECT().PathExists(gomock.Eq(devicePath)).Return(true, nil),
					mockMounter.EXPECT().PathExists(gomock.Eq(targetPath)).Return(false, nil),
				)

				mockMounter.EXPECT().MakeDir(targetPath).Return(nil)
				mockMounter.EXPECT().GetDeviceNameFromMount(targetPath).Return("", 1, nil)
				mockMounter.EXPECT().FormatAndMount(gomock.Eq(devicePath), gomock.Eq(targetPath), gomock.Eq(FSTypeExt4), gomock.Any())
				mockMounter.EXPECT().NeedResize(gomock.Eq(devicePath), gomock.Eq(targetPath)).Return(false, nil)
			},
		},
		{
			name: "fail no VolumeId",
			request: &csi.NodeStageVolumeRequest{
				PublishContext:    map[string]string{DevicePathKey: devicePath},
				StagingTargetPath: targetPath,
				VolumeCapability:  stdVolCap,
			},
			expectedCode: codes.InvalidArgument,
		},
		{
			name: "fail no mount",
			request: &csi.NodeStageVolumeRequest{
				PublishContext:    map[string]string{DevicePathKey: devicePath},
				StagingTargetPath: targetPath,
				VolumeCapability: &csi.VolumeCapability{
					AccessType: &csi.VolumeCapability_Mount{},
					AccessMode: &csi.VolumeCapability_AccessMode{
						Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER,
					},
				},
			},
			expectedCode: codes.InvalidArgument,
		},
		{
			name: "fail no StagingTargetPath",
			request: &csi.NodeStageVolumeRequest{
				PublishContext:   map[string]string{DevicePathKey: devicePath},
				VolumeCapability: stdVolCap,
				VolumeId:         "vol-test",
			},
			expectedCode: codes.InvalidArgument,
		},
		{
			name: "fail no VolumeCapability",
			request: &csi.NodeStageVolumeRequest{
				PublishContext:    map[string]string{DevicePathKey: devicePath},
				StagingTargetPath: targetPath,
				VolumeId:          "vol-test",
			},
			expectedCode: codes.InvalidArgument,
		},
		{
			name: "fail invalid VolumeCapability",
			request: &csi.NodeStageVolumeRequest{
				PublishContext:    map[string]string{DevicePathKey: devicePath},
				StagingTargetPath: targetPath,
				VolumeCapability: &csi.VolumeCapability{
					AccessMode: &csi.VolumeCapability_AccessMode{
						Mode: csi.VolumeCapability_AccessMode_UNKNOWN,
					},
				},
				VolumeId: "vol-test",
			},
			expectedCode: codes.InvalidArgument,
		},
		{
			name: "fail no devicePath",
			request: &csi.NodeStageVolumeRequest{
				StagingTargetPath: targetPath,
				VolumeCapability:  stdVolCap,
				VolumeId:          "vol-test",
			},
			expectedCode: codes.InvalidArgument,
		},
		{
			name: "fail invalid volumeContext",
			request: &csi.NodeStageVolumeRequest{
				PublishContext:    map[string]string{DevicePathKey: devicePath},
				StagingTargetPath: targetPath,
				VolumeCapability:  stdVolCap,
				VolumeContext:     map[string]string{VolumeAttributePartition: "partition1"},
				VolumeId:          "vol-test",
			},
			expectedCode: codes.InvalidArgument,
		},
		{
			name: "success device already mounted at target",
			request: &csi.NodeStageVolumeRequest{
				PublishContext:    map[string]string{DevicePathKey: devicePath},
				StagingTargetPath: targetPath,
				VolumeCapability:  stdVolCap,
				VolumeId:          "vol-test",
			},
			expectMock: func(mockMounter mocks.MockMounter) {
				gomock.InOrder(
					mockMounter.EXPECT().PathExists(gomock.Eq(devicePath)).Return(true, nil),
					mockMounter.EXPECT().PathExists(gomock.Eq(targetPath)).Return(false, nil),
				)

				mockMounter.EXPECT().MakeDir(targetPath).Return(nil)
				mockMounter.EXPECT().GetDeviceNameFromMount(targetPath).Return(devicePath, 1, nil)
				mockMounter.EXPECT().FormatAndMount(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Times(0)
			},
		},
		{
			name: "success with partition",
			request: &csi.NodeStageVolumeRequest{
				PublishContext:    map[string]string{DevicePathKey: devicePath},
				StagingTargetPath: targetPath,
				VolumeCapability:  stdVolCap,
				VolumeContext:     stdVolContext,
				VolumeId:          "vol-test",
			},
			expectMock: func(mockMounter mocks.MockMounter) {
				gomock.InOrder(
					mockMounter.EXPECT().PathExists(gomock.Eq(devicePath)).Return(true, nil),
					mockMounter.EXPECT().PathExists(gomock.Eq(targetPath)).Return(false, nil),
				)
				mockMounter.EXPECT().MakeDir(targetPath).Return(nil)
				mockMounter.EXPECT().GetDeviceNameFromMount(targetPath).Return("", 1, nil)
				mockMounter.EXPECT().FormatAndMount(gomock.Eq(devicePathWithPartition), gomock.Eq(targetPath), gomock.Eq(defaultFsType), gomock.Any())
				mockMounter.EXPECT().NeedResize(gomock.Eq(devicePathWithPartition), gomock.Eq(targetPath)).Return(false, nil)
			},
		},
		{
			name: "success with invalid partition config, will ignore partition",
			request: &csi.NodeStageVolumeRequest{
				PublishContext:    map[string]string{DevicePathKey: devicePath},
				StagingTargetPath: targetPath,
				VolumeCapability:  stdVolCap,
				VolumeContext:     map[string]string{VolumeAttributePartition: "0"},
				VolumeId:          "vol-test",
			},
			expectMock: func(mockMounter mocks.MockMounter) {
				gomock.InOrder(
					mockMounter.EXPECT().PathExists(gomock.Eq(devicePath)).Return(true, nil),
					mockMounter.EXPECT().PathExists(gomock.Eq(targetPath)).Return(false, nil),
				)
				mockMounter.EXPECT().MakeDir(targetPath).Return(nil)
				mockMounter.EXPECT().GetDeviceNameFromMount(targetPath).Return("", 1, nil)
				mockMounter.EXPECT().FormatAndMount(gomock.Eq(devicePath), gomock.Eq(targetPath), gomock.Eq(defaultFsType), gomock.Any())
				mockMounter.EXPECT().NeedResize(gomock.Eq(devicePath), gomock.Eq(targetPath)).Return(false, nil)
			},
		},
		{
			name: "fail with in-flight request",
			request: &csi.NodeStageVolumeRequest{
				PublishContext:    map[string]string{DevicePathKey: devicePath},
				StagingTargetPath: targetPath,
				VolumeCapability:  stdVolCap,
				VolumeId:          "vol-test",
			},
			inFlightFunc: func(inFlight *internal.InFlight) *internal.InFlight {
				inFlight.Insert("vol-test")
				return inFlight
			},
			expectedCode: codes.Aborted,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockCtl := gomock.NewController(t)
			defer mockCtl.Finish()

			mockMetadata := mocks.NewMockMetadataService(mockCtl)
			mockMounter := mocks.NewMockMounter(mockCtl)

			inFlight := internal.NewInFlight()
			if tc.inFlightFunc != nil {
				tc.inFlightFunc(inFlight)
			}

			awsDriver := &nodeService{
				metadata: mockMetadata,
				mounter:  mockMounter,
				inFlight: inFlight,
			}

			if tc.expectMock != nil {
				tc.expectMock(*mockMounter)
			}

			_, err := awsDriver.NodeStageVolume(context.TODO(), tc.request)
			if tc.expectedCode != codes.OK {
				expectErr(t, err, tc.expectedCode)
			} else if err != nil {
				t.Fatalf("Expect no error but got: %v", err)
			}
		})
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

				mockMounter.EXPECT().GetDeviceNameFromMount(gomock.Eq(targetPath)).Return(devicePath, 1, nil)
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

				mockMounter.EXPECT().GetDeviceNameFromMount(gomock.Eq(targetPath)).Return(devicePath, 0, nil)

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

				mockMounter.EXPECT().GetDeviceNameFromMount(gomock.Eq(targetPath)).Return(devicePath, 2, nil)
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

				mockMounter.EXPECT().GetDeviceNameFromMount(gomock.Eq(targetPath)).Return("", 0, errors.New("GetDeviceName faield"))

				req := &csi.NodeUnstageVolumeRequest{
					StagingTargetPath: targetPath,
					VolumeId:          "vol-test",
				}

				_, err := awsDriver.NodeUnstageVolume(context.TODO(), req)
				expectErr(t, err, codes.Internal)
			},
		},
		{
			name: "fail another operation in-flight on given volumeId",
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
					VolumeId:          "vol-test",
				}

				awsDriver.inFlight.Insert("vol-test")
				_, err := awsDriver.NodeUnstageVolume(context.TODO(), req)
				expectErr(t, err, codes.Aborted)
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
	stdVolContext := map[string]string{"partition": "1"}
	devicePathWithPartition := devicePath + "1"
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
				mockMounter.EXPECT().Mount(gomock.Eq(stagingTargetPath), gomock.Eq(targetPath), gomock.Eq(defaultFsType), gomock.Eq([]string{"bind"})).Return(nil)
				mockMounter.EXPECT().IsLikelyNotMountPoint(gomock.Eq(targetPath)).Return(true, nil)

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
			name: "success filesystem mounted already",
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
				mockMounter.EXPECT().IsLikelyNotMountPoint(gomock.Eq(targetPath)).Return(false, nil)

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
			name: "success filesystem mountpoint error",
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
					mockMounter.EXPECT().PathExists(gomock.Eq(targetPath)).Return(true, nil),
				)
				mockMounter.EXPECT().MakeDir(gomock.Eq(targetPath)).Return(nil)
				mockMounter.EXPECT().IsLikelyNotMountPoint(gomock.Eq(targetPath)).Return(true, errors.New("Internal system error"))

				req := &csi.NodePublishVolumeRequest{
					PublishContext:    map[string]string{DevicePathKey: devicePath},
					StagingTargetPath: stagingTargetPath,
					TargetPath:        targetPath,
					VolumeCapability:  stdVolCap,
					VolumeId:          "vol-test",
				}

				_, err := awsDriver.NodePublishVolume(context.TODO(), req)
				expectErr(t, err, codes.Internal)
			},
		},
		{
			name: "success filesystem corrupted mountpoint error",
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

				mockMounter.EXPECT().PathExists(gomock.Eq(targetPath)).Return(true, errors.New("CorruptedMntError"))
				mockMounter.EXPECT().IsCorruptedMnt(gomock.Eq(errors.New("CorruptedMntError"))).Return(true)

				mockMounter.EXPECT().MakeDir(gomock.Eq(targetPath)).Return(nil)
				mockMounter.EXPECT().IsLikelyNotMountPoint(gomock.Eq(targetPath)).Return(true, errors.New("internal system error"))
				mockMounter.EXPECT().Unmount(gomock.Eq(targetPath)).Return(nil)
				mockMounter.EXPECT().Mount(gomock.Eq(stagingTargetPath), gomock.Eq(targetPath), gomock.Eq(defaultFsType), gomock.Eq([]string{"bind"})).Return(nil)

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
			name: "success fstype",
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
				mockMounter.EXPECT().Mount(gomock.Eq(stagingTargetPath), gomock.Eq(targetPath), gomock.Eq(FSTypeXfs), gomock.Eq([]string{"bind"})).Return(nil)
				mockMounter.EXPECT().IsLikelyNotMountPoint(gomock.Eq(targetPath)).Return(true, nil)

				req := &csi.NodePublishVolumeRequest{
					PublishContext:    map[string]string{DevicePathKey: devicePath},
					StagingTargetPath: stagingTargetPath,
					TargetPath:        targetPath,
					VolumeCapability: &csi.VolumeCapability{
						AccessType: &csi.VolumeCapability_Mount{
							Mount: &csi.VolumeCapability_MountVolume{
								FsType: FSTypeXfs,
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
				mockMounter.EXPECT().Mount(gomock.Eq(stagingTargetPath), gomock.Eq(targetPath), gomock.Eq(defaultFsType), gomock.Eq([]string{"bind", "ro"})).Return(nil)
				mockMounter.EXPECT().IsLikelyNotMountPoint(gomock.Eq(targetPath)).Return(true, nil)

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
				mockMounter.EXPECT().Mount(gomock.Eq(stagingTargetPath), gomock.Eq(targetPath), gomock.Eq(defaultFsType), gomock.Eq([]string{"bind", "test-flag"})).Return(nil)
				mockMounter.EXPECT().IsLikelyNotMountPoint(gomock.Eq(targetPath)).Return(true, nil)

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
					mockMounter.EXPECT().PathExists(gomock.Eq(devicePath)).Return(true, nil),
					mockMounter.EXPECT().PathExists(gomock.Eq("/test")).Return(false, nil),
				)
				mockMounter.EXPECT().MakeDir(gomock.Eq("/test")).Return(nil)
				mockMounter.EXPECT().MakeFile(targetPath).Return(nil)
				mockMounter.EXPECT().Mount(gomock.Eq(devicePath), gomock.Eq(targetPath), gomock.Eq(""), gomock.Eq([]string{"bind"})).Return(nil)
				mockMounter.EXPECT().IsLikelyNotMountPoint(gomock.Eq(targetPath)).Return(true, nil)

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
			name: "success mounted already [raw block]",
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
					mockMounter.EXPECT().PathExists(gomock.Eq(devicePath)).Return(true, nil),
					mockMounter.EXPECT().PathExists(gomock.Eq("/test")).Return(false, nil),
				)
				mockMounter.EXPECT().MakeDir(gomock.Eq("/test")).Return(nil)
				mockMounter.EXPECT().MakeFile(targetPath).Return(nil)
				mockMounter.EXPECT().IsLikelyNotMountPoint(gomock.Eq(targetPath)).Return(false, nil)

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
			name: "success mountpoint error [raw block]",
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
					mockMounter.EXPECT().PathExists(gomock.Eq(devicePath)).Return(true, nil),
					mockMounter.EXPECT().PathExists(gomock.Eq("/test")).Return(false, nil),
					mockMounter.EXPECT().PathExists(gomock.Eq(targetPath)).Return(true, nil),
				)

				mockMounter.EXPECT().MakeDir(gomock.Eq("/test")).Return(nil)
				mockMounter.EXPECT().MakeFile(targetPath).Return(nil)
				mockMounter.EXPECT().IsLikelyNotMountPoint(gomock.Eq(targetPath)).Return(true, errors.New("Internal System Error"))

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
				expectErr(t, err, codes.Internal)
			},
		},
		{
			name: "success corrupted mountpoint error [raw block]",
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
					mockMounter.EXPECT().PathExists(gomock.Eq(devicePath)).Return(true, nil),
					mockMounter.EXPECT().PathExists(gomock.Eq("/test")).Return(false, nil),
					mockMounter.EXPECT().PathExists(gomock.Eq(targetPath)).Return(true, errors.New("CorruptedMntError")),
				)

				mockMounter.EXPECT().IsCorruptedMnt(errors.New("CorruptedMntError")).Return(true)

				mockMounter.EXPECT().MakeDir(gomock.Eq("/test")).Return(nil)
				mockMounter.EXPECT().MakeFile(targetPath).Return(nil)
				mockMounter.EXPECT().Unmount(gomock.Eq(targetPath)).Return(nil)
				mockMounter.EXPECT().IsLikelyNotMountPoint(gomock.Eq(targetPath)).Return(true, errors.New("Internal System Error"))
				mockMounter.EXPECT().Mount(gomock.Eq(devicePath), gomock.Eq(targetPath), gomock.Any(), gomock.Any()).Return(nil)

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
			name: "success normal with partition [raw block]",
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
					mockMounter.EXPECT().PathExists(gomock.Eq(devicePath)).Return(true, nil),
					mockMounter.EXPECT().PathExists(gomock.Eq("/test")).Return(false, nil),
				)
				mockMounter.EXPECT().MakeDir(gomock.Eq("/test")).Return(nil)
				mockMounter.EXPECT().MakeFile(targetPath).Return(nil)
				mockMounter.EXPECT().Mount(gomock.Eq(devicePathWithPartition), gomock.Eq(targetPath), gomock.Eq(""), gomock.Eq([]string{"bind"})).Return(nil)
				mockMounter.EXPECT().IsLikelyNotMountPoint(gomock.Eq(targetPath)).Return(true, nil)

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
					VolumeContext: stdVolContext,
					VolumeId:      "vol-test",
				}

				_, err := awsDriver.NodePublishVolume(context.TODO(), req)
				if err != nil {
					t.Fatalf("Expect no error but got: %v", err)
				}
			},
		},
		{
			name: "success normal with invalid partition config, will ignore the config [raw block]",
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
					mockMounter.EXPECT().PathExists(gomock.Eq(devicePath)).Return(true, nil),
					mockMounter.EXPECT().PathExists(gomock.Eq("/test")).Return(false, nil),
				)
				mockMounter.EXPECT().MakeDir(gomock.Eq("/test")).Return(nil)
				mockMounter.EXPECT().MakeFile(targetPath).Return(nil)
				mockMounter.EXPECT().Mount(gomock.Eq(devicePath), gomock.Eq(targetPath), gomock.Eq(""), gomock.Eq([]string{"bind"})).Return(nil)
				mockMounter.EXPECT().IsLikelyNotMountPoint(gomock.Eq(targetPath)).Return(true, nil)

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
					VolumeContext: map[string]string{VolumeAttributePartition: "0"},
					VolumeId:      "vol-test",
				}

				_, err := awsDriver.NodePublishVolume(context.TODO(), req)
				if err != nil {
					t.Fatalf("Expect no error but got: %v", err)
				}
			},
		},
		{
			name: "Fail invalid volumeContext config [raw block]",
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
					VolumeContext: map[string]string{VolumeAttributePartition: "partition1"},
					VolumeId:      "vol-test",
				}

				_, err := awsDriver.NodePublishVolume(context.TODO(), req)
				expectErr(t, err, codes.InvalidArgument)
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

				mockMounter.EXPECT().PathExists(gomock.Eq(devicePath)).Return(false, errors.New("findDevicePath failed"))

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
		{
			name: "fail another operation in-flight on given volumeId",
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
						AccessType: &csi.VolumeCapability_Block{
							Block: &csi.VolumeCapability_BlockVolume{},
						},
						AccessMode: &csi.VolumeCapability_AccessMode{
							Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER,
						},
					},
				}
				awsDriver.inFlight.Insert("vol-test")

				_, err := awsDriver.NodePublishVolume(context.TODO(), req)
				expectErr(t, err, codes.Aborted)

			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, tc.testFunc)
	}
}
func TestNodeExpandVolume(t *testing.T) {
	mockCtl := gomock.NewController(t)
	defer mockCtl.Finish()

	mockMetadata := mocks.NewMockMetadataService(mockCtl)
	mockMounter := mocks.NewMockMounter(mockCtl)

	awsDriver := &nodeService{
		metadata: mockMetadata,
		mounter:  mockMounter,
		inFlight: internal.NewInFlight(),
	}

	tests := []struct {
		name               string
		request            csi.NodeExpandVolumeRequest
		expectResponseCode codes.Code
	}{
		{
			name:               "fail missing volumeId",
			request:            csi.NodeExpandVolumeRequest{},
			expectResponseCode: codes.InvalidArgument,
		},
		{
			name: "fail missing volumePath",
			request: csi.NodeExpandVolumeRequest{
				StagingTargetPath: "/testDevice/Path",
				VolumeId:          "test-volume-id",
			},
			expectResponseCode: codes.InvalidArgument,
		},
		{
			name: "fail volume path not exist",
			request: csi.NodeExpandVolumeRequest{
				VolumePath: "./test",
				VolumeId:   "test-volume-id",
			},
			expectResponseCode: codes.Internal,
		},
		{
			name: "Fail validate VolumeCapability",
			request: csi.NodeExpandVolumeRequest{
				VolumePath: "./test",
				VolumeId:   "test-volume-id",
				VolumeCapability: &csi.VolumeCapability{
					AccessType: &csi.VolumeCapability_Block{
						Block: &csi.VolumeCapability_BlockVolume{},
					},
					AccessMode: &csi.VolumeCapability_AccessMode{
						Mode: csi.VolumeCapability_AccessMode_UNKNOWN,
					},
				},
			},
			expectResponseCode: codes.InvalidArgument,
		},
		{
			name: "Success [VolumeCapability is block]",
			request: csi.NodeExpandVolumeRequest{
				VolumePath: "./test",
				VolumeId:   "test-volume-id",
				VolumeCapability: &csi.VolumeCapability{
					AccessType: &csi.VolumeCapability_Block{
						Block: &csi.VolumeCapability_BlockVolume{},
					},
					AccessMode: &csi.VolumeCapability_AccessMode{
						Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER,
					},
				},
			},
			expectResponseCode: codes.OK,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := awsDriver.NodeExpandVolume(context.Background(), &test.request)
			if err != nil {
				if test.expectResponseCode != codes.OK {
					expectErr(t, err, test.expectResponseCode)
				} else {
					t.Fatalf("Expect no error but got: %v", err)
				}
			}
		})
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
		{
			name: "fail error on unmount",
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

				mockMounter.EXPECT().Unmount(gomock.Eq(targetPath)).Return(errors.New("test Unmount error"))
				_, err := awsDriver.NodeUnpublishVolume(context.TODO(), req)
				expectErr(t, err, codes.Internal)
			},
		},
		{
			name: "fail another operation in-flight on given volumeId",
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

				awsDriver.inFlight.Insert("vol-test")
				_, err := awsDriver.NodeUnpublishVolume(context.TODO(), req)
				expectErr(t, err, codes.Aborted)
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, tc.testFunc)
	}
}

func TestNodeGetVolumeStats(t *testing.T) {
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
				VolumePath := "./test"
				err := os.MkdirAll(VolumePath, 0644)
				if err != nil {
					t.Fatalf("fail to create dir: %v", err)
				}
				defer os.RemoveAll(VolumePath)

				mockMounter.EXPECT().PathExists(VolumePath).Return(true, nil)

				awsDriver := nodeService{
					metadata: mockMetadata,
					mounter:  mockMounter,
					inFlight: internal.NewInFlight(),
				}

				req := &csi.NodeGetVolumeStatsRequest{
					VolumeId:   "vol-test",
					VolumePath: VolumePath,
				}
				_, err = awsDriver.NodeGetVolumeStats(context.TODO(), req)
				if err != nil {
					t.Fatalf("Expect no error but got: %v", err)
				}
			},
		},
		{
			name: "fail path not exist",
			testFunc: func(t *testing.T) {
				mockCtl := gomock.NewController(t)
				defer mockCtl.Finish()

				mockMetadata := mocks.NewMockMetadataService(mockCtl)
				mockMounter := mocks.NewMockMounter(mockCtl)
				VolumePath := "/test"

				mockMounter.EXPECT().PathExists(VolumePath).Return(false, nil)

				awsDriver := nodeService{
					metadata: mockMetadata,
					mounter:  mockMounter,
					inFlight: internal.NewInFlight(),
				}

				req := &csi.NodeGetVolumeStatsRequest{
					VolumeId:   "vol-test",
					VolumePath: VolumePath,
				}
				_, err := awsDriver.NodeGetVolumeStats(context.TODO(), req)
				expectErr(t, err, codes.NotFound)
			},
		},
		{
			name: "fail can't determine block device",
			testFunc: func(t *testing.T) {
				mockCtl := gomock.NewController(t)
				defer mockCtl.Finish()

				mockMetadata := mocks.NewMockMetadataService(mockCtl)
				mockMounter := mocks.NewMockMounter(mockCtl)
				VolumePath := "/test"

				mockMounter.EXPECT().PathExists(VolumePath).Return(true, nil)

				awsDriver := nodeService{
					metadata: mockMetadata,
					mounter:  mockMounter,
					inFlight: internal.NewInFlight(),
				}

				req := &csi.NodeGetVolumeStatsRequest{
					VolumeId:   "vol-test",
					VolumePath: VolumePath,
				}
				_, err := awsDriver.NodeGetVolumeStats(context.TODO(), req)
				expectErr(t, err, codes.Internal)
			},
		},
		{
			name: "fail error calling existsPath",
			testFunc: func(t *testing.T) {
				mockCtl := gomock.NewController(t)
				defer mockCtl.Finish()

				mockMetadata := mocks.NewMockMetadataService(mockCtl)
				mockMounter := mocks.NewMockMounter(mockCtl)
				VolumePath := "/test"

				mockMounter.EXPECT().PathExists(VolumePath).Return(false, errors.New("get existsPath call fail"))

				awsDriver := nodeService{
					metadata: mockMetadata,
					mounter:  mockMounter,
					inFlight: internal.NewInFlight(),
				}

				req := &csi.NodeGetVolumeStatsRequest{
					VolumeId:   "vol-test",
					VolumePath: VolumePath,
				}
				_, err := awsDriver.NodeGetVolumeStats(context.TODO(), req)
				expectErr(t, err, codes.Internal)
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, tc.testFunc)
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
		{
			Type: &csi.NodeServiceCapability_Rpc{
				Rpc: &csi.NodeServiceCapability_RPC{
					Type: csi.NodeServiceCapability_RPC_EXPAND_VOLUME,
				},
			},
		},
		{
			Type: &csi.NodeServiceCapability_Rpc{
				Rpc: &csi.NodeServiceCapability_RPC{
					Type: csi.NodeServiceCapability_RPC_GET_VOLUME_STATS,
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
	validOutpostArn, _ := arn.Parse(strings.ReplaceAll("arn:aws:outposts:us-west-2:111111111111:outpost/op-0aaa000a0aaaa00a0", "outpost/", ""))
	emptyOutpostArn := arn.ARN{}
	testCases := []struct {
		name              string
		instanceID        string
		instanceType      string
		availabilityZone  string
		volumeAttachLimit int64
		expMaxVolumes     int64
		outpostArn        arn.ARN
	}{
		{
			name:              "success normal",
			instanceID:        "i-123456789abcdef01",
			instanceType:      "t2.medium",
			availabilityZone:  "us-west-2b",
			volumeAttachLimit: -1,
			expMaxVolumes:     39,
			outpostArn:        emptyOutpostArn,
		},
		{
			name:              "success normal with overwrite",
			instanceID:        "i-123456789abcdef01",
			instanceType:      "t2.medium",
			availabilityZone:  "us-west-2b",
			volumeAttachLimit: 42,
			expMaxVolumes:     42,
			outpostArn:        emptyOutpostArn,
		},
		{
			name:              "success normal with NVMe",
			instanceID:        "i-123456789abcdef01",
			instanceType:      "m5d.large",
			availabilityZone:  "us-west-2b",
			volumeAttachLimit: -1,
			expMaxVolumes:     25,
			outpostArn:        emptyOutpostArn,
		},
		{
			name:              "success normal with NVMe and overwrite",
			instanceID:        "i-123456789abcdef01",
			instanceType:      "m5d.large",
			availabilityZone:  "us-west-2b",
			volumeAttachLimit: 30,
			expMaxVolumes:     30,
			outpostArn:        emptyOutpostArn,
		},
		{
			name:              "success normal outposts",
			instanceID:        "i-123456789abcdef01",
			instanceType:      "m5d.large",
			availabilityZone:  "us-west-2b",
			volumeAttachLimit: 30,
			expMaxVolumes:     30,
			outpostArn:        validOutpostArn,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockCtl := gomock.NewController(t)
			defer mockCtl.Finish()

			driverOptions := &DriverOptions{
				volumeAttachLimit: tc.volumeAttachLimit,
			}

			mockMounter := mocks.NewMockMounter(mockCtl)

			mockMetadata := mocks.NewMockMetadataService(mockCtl)
			mockMetadata.EXPECT().GetInstanceID().Return(tc.instanceID)
			mockMetadata.EXPECT().GetAvailabilityZone().Return(tc.availabilityZone)
			mockMetadata.EXPECT().GetOutpostArn().Return(tc.outpostArn)

			if tc.volumeAttachLimit < 0 {
				mockMetadata.EXPECT().GetInstanceType().Return(tc.instanceType)
			}

			awsDriver := &nodeService{
				metadata:      mockMetadata,
				mounter:       mockMounter,
				inFlight:      internal.NewInFlight(),
				driverOptions: driverOptions,
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

			if at.Segments[AwsAccountIDKey] != tc.outpostArn.AccountID {
				t.Fatalf("Expected AwsAccountId %q, got %q", tc.outpostArn.AccountID, at.Segments[AwsAccountIDKey])
			}

			if at.Segments[AwsRegionKey] != tc.outpostArn.Region {
				t.Fatalf("Expected AwsRegion %q, got %q", tc.outpostArn.Region, at.Segments[AwsRegionKey])
			}

			if at.Segments[AwsOutpostIDKey] != tc.outpostArn.Resource {
				t.Fatalf("Expected AwsOutpostID %q, got %q", tc.outpostArn.Resource, at.Segments[AwsOutpostIDKey])
			}

			if at.Segments[AwsPartitionKey] != tc.outpostArn.Partition {
				t.Fatalf("Expected AwsPartition %q, got %q", tc.outpostArn.Partition, at.Segments[AwsPartitionKey])
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
