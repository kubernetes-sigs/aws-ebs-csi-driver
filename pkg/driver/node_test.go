/*
Copyright 2024 The Kubernetes Authors.

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
	"os"
	"reflect"
	"runtime"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws/arn"
	csi "github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/golang/mock/gomock"
	"github.com/kubernetes-sigs/aws-ebs-csi-driver/pkg/cloud/metadata"
	"github.com/kubernetes-sigs/aws-ebs-csi-driver/pkg/driver/internal"
	"github.com/kubernetes-sigs/aws-ebs-csi-driver/pkg/mounter"
	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/storage/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
)

func TestNewNodeService(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockMetadataService := metadata.NewMockMetadataService(ctrl)
	mockMounter := mounter.NewMockMounter(ctrl)
	mockKubernetesClient := NewMockKubernetesClient(ctrl)

	os.Setenv("AWS_REGION", "us-west-2")
	defer os.Unsetenv("AWS_REGION")

	options := &Options{}

	nodeService := NewNodeService(options, mockMetadataService, mockMounter, mockKubernetesClient)

	if nodeService == nil {
		t.Fatal("Expected NewNodeService to return a non-nil NodeService")
	}

	if nodeService.metadata != mockMetadataService {
		t.Error("Expected NodeService.metadata to be set to the mock MetadataService")
	}

	if nodeService.mounter != mockMounter {
		t.Error("Expected NodeService.mounter to be set to the mock Mounter")
	}

	if nodeService.inFlight == nil {
		t.Error("Expected NodeService.inFlight to be initialized")
	}

	if nodeService.options != options {
		t.Error("Expected NodeService.options to be set to the provided options")
	}
}

func TestNodeStageVolume(t *testing.T) {
	testCases := []struct {
		name         string
		req          *csi.NodeStageVolumeRequest
		mounterMock  func(ctrl *gomock.Controller) *mounter.MockMounter
		metadataMock func(ctrl *gomock.Controller) *metadata.MockMetadataService
		expectedErr  error
		inflight     bool
	}{
		{
			name: "success",
			req: &csi.NodeStageVolumeRequest{
				VolumeId:          "vol-test",
				StagingTargetPath: "/staging/path",
				VolumeCapability: &csi.VolumeCapability{
					AccessType: &csi.VolumeCapability_Mount{
						Mount: &csi.VolumeCapability_MountVolume{
							FsType: "ext4",
						},
					},
					AccessMode: &csi.VolumeCapability_AccessMode{
						Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER,
					},
				},
				PublishContext: map[string]string{DevicePathKey: "/dev/xvdba"},
			},
			mounterMock: func(ctrl *gomock.Controller) *mounter.MockMounter {
				m := mounter.NewMockMounter(ctrl)
				m.EXPECT().FindDevicePath(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return("/dev/xvdba", nil)
				m.EXPECT().PathExists(gomock.Any()).Return(true, nil)
				m.EXPECT().GetDeviceNameFromMount(gomock.Any()).Return("", 1, nil)
				m.EXPECT().FormatAndMountSensitiveWithFormatOptions(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
				m.EXPECT().NeedResize(gomock.Any(), gomock.Any()).Return(false, nil)
				return m
			},
			metadataMock: func(ctrl *gomock.Controller) *metadata.MockMetadataService {
				m := metadata.NewMockMetadataService(ctrl)
				m.EXPECT().GetRegion().Return("us-west-2")
				return m
			},
			expectedErr: nil,
		},
		{
			name: "missing_volume_id",
			req: &csi.NodeStageVolumeRequest{
				StagingTargetPath: "/staging/path",
				VolumeCapability: &csi.VolumeCapability{
					AccessType: &csi.VolumeCapability_Mount{
						Mount: &csi.VolumeCapability_MountVolume{
							FsType: "ext4",
						},
					},
					AccessMode: &csi.VolumeCapability_AccessMode{
						Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER,
					},
				},
			},
			mounterMock:  nil,
			metadataMock: nil,
			expectedErr:  status.Error(codes.InvalidArgument, "Volume ID not provided"),
		},
		{
			name: "missing_staging_target",
			req: &csi.NodeStageVolumeRequest{
				VolumeId: "vol-test",
				VolumeCapability: &csi.VolumeCapability{
					AccessType: &csi.VolumeCapability_Mount{
						Mount: &csi.VolumeCapability_MountVolume{
							FsType: "ext4",
						},
					},
					AccessMode: &csi.VolumeCapability_AccessMode{
						Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER,
					},
				},
			},
			mounterMock:  nil,
			metadataMock: nil,
			expectedErr:  status.Error(codes.InvalidArgument, "Staging target not provided"),
		},
		{
			name: "missing_volume_capability",
			req: &csi.NodeStageVolumeRequest{
				VolumeId:          "vol-test",
				StagingTargetPath: "/staging/path",
			},
			mounterMock:  nil,
			metadataMock: nil,
			expectedErr:  status.Error(codes.InvalidArgument, "Volume capability not provided"),
		},
		{
			name: "invalid_volume_attribute",
			req: &csi.NodeStageVolumeRequest{
				VolumeId:          "vol-test",
				StagingTargetPath: "/staging/path",
				VolumeCapability: &csi.VolumeCapability{
					AccessType: &csi.VolumeCapability_Mount{
						Mount: &csi.VolumeCapability_MountVolume{
							FsType: "ext4",
						},
					},
					AccessMode: &csi.VolumeCapability_AccessMode{
						Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER,
					},
				},
				VolumeContext: map[string]string{
					VolumeAttributePartition: "invalid-partition",
				},
			},
			mounterMock:  nil,
			metadataMock: nil,
			expectedErr:  status.Error(codes.InvalidArgument, "Volume Attribute is not valid"),
		},
		{
			name: "unsupported_volume_capability",
			req: &csi.NodeStageVolumeRequest{
				VolumeId:          "vol-test",
				StagingTargetPath: "/staging/path",
				VolumeCapability: &csi.VolumeCapability{
					AccessType: &csi.VolumeCapability_Mount{
						Mount: &csi.VolumeCapability_MountVolume{
							FsType: "ext4",
						},
					},
					AccessMode: &csi.VolumeCapability_AccessMode{
						Mode: csi.VolumeCapability_AccessMode_UNKNOWN,
					},
				},
			},
			mounterMock:  nil,
			metadataMock: nil,
			expectedErr:  status.Error(codes.InvalidArgument, "Volume capability not supported"),
		},
		{
			name: "block_volume",
			req: &csi.NodeStageVolumeRequest{
				VolumeId:          "vol-test",
				StagingTargetPath: "/staging/path",
				VolumeCapability: &csi.VolumeCapability{
					AccessType: &csi.VolumeCapability_Block{
						Block: &csi.VolumeCapability_BlockVolume{},
					},
					AccessMode: &csi.VolumeCapability_AccessMode{
						Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER,
					},
				},
			},
			mounterMock:  nil,
			metadataMock: nil,
			expectedErr:  nil,
		},
		{
			name: "missing_mount_volume",
			req: &csi.NodeStageVolumeRequest{
				VolumeId:          "vol-test",
				StagingTargetPath: "/staging/path",
				VolumeCapability: &csi.VolumeCapability{
					AccessType: &csi.VolumeCapability_Mount{},
					AccessMode: &csi.VolumeCapability_AccessMode{
						Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER,
					},
				},
			},
			mounterMock:  nil,
			metadataMock: nil,
			expectedErr:  status.Error(codes.InvalidArgument, "NodeStageVolume: mount is nil within volume capability"),
		},
		{
			name: "default_fstype",
			req: &csi.NodeStageVolumeRequest{
				VolumeId:          "vol-test",
				StagingTargetPath: "/staging/path",
				VolumeCapability: &csi.VolumeCapability{
					AccessType: &csi.VolumeCapability_Mount{
						Mount: &csi.VolumeCapability_MountVolume{},
					},
					AccessMode: &csi.VolumeCapability_AccessMode{
						Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER,
					},
				},
				PublishContext: map[string]string{DevicePathKey: "/dev/xvdba"},
			},
			mounterMock: func(ctrl *gomock.Controller) *mounter.MockMounter {
				m := mounter.NewMockMounter(ctrl)
				m.EXPECT().FindDevicePath(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return("/dev/xvdba", nil)
				m.EXPECT().PathExists(gomock.Any()).Return(false, nil)
				m.EXPECT().MakeDir(gomock.Any()).Return(nil)
				m.EXPECT().GetDeviceNameFromMount(gomock.Any()).Return("", 0, nil)
				m.EXPECT().FormatAndMountSensitiveWithFormatOptions(gomock.Any(), gomock.Any(), defaultFsType, gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
				m.EXPECT().NeedResize(gomock.Any(), gomock.Any()).Return(false, nil)
				return m
			},
			metadataMock: func(ctrl *gomock.Controller) *metadata.MockMetadataService {
				m := metadata.NewMockMetadataService(ctrl)
				m.EXPECT().GetRegion().Return("us-west-2")
				return m
			},
			expectedErr: nil,
		},
		{
			name: "invalid_fstype",
			req: &csi.NodeStageVolumeRequest{
				VolumeId:          "vol-test",
				StagingTargetPath: "/staging/path",
				VolumeCapability: &csi.VolumeCapability{
					AccessType: &csi.VolumeCapability_Mount{
						Mount: &csi.VolumeCapability_MountVolume{
							FsType: "invalid",
						},
					},
					AccessMode: &csi.VolumeCapability_AccessMode{
						Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER,
					},
				},
			},
			mounterMock:  nil,
			metadataMock: nil,
			expectedErr:  status.Errorf(codes.InvalidArgument, "NodeStageVolume: invalid fstype invalid"),
		},
		{
			name: "invalid_block_size",
			req: &csi.NodeStageVolumeRequest{
				VolumeId:          "vol-test",
				StagingTargetPath: "/staging/path",
				VolumeCapability: &csi.VolumeCapability{
					AccessType: &csi.VolumeCapability_Mount{
						Mount: &csi.VolumeCapability_MountVolume{
							FsType: "ext4",
						},
					},
					AccessMode: &csi.VolumeCapability_AccessMode{
						Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER,
					},
				},
				VolumeContext: map[string]string{
					BlockSizeKey: "-",
				},
				PublishContext: map[string]string{
					DevicePathKey: "/dev/xvdba",
				},
			},
			mounterMock: func(ctrl *gomock.Controller) *mounter.MockMounter {
				m := mounter.NewMockMounter(ctrl)
				return m
			},
			metadataMock: func(ctrl *gomock.Controller) *metadata.MockMetadataService {
				return nil
			},
			expectedErr: status.Error(codes.InvalidArgument, "Invalid blocksize (aborting!): <nil>"),
		},
		{
			name: "invalid_inode_size",
			req: &csi.NodeStageVolumeRequest{
				VolumeId:          "vol-test",
				StagingTargetPath: "/staging/path",
				VolumeCapability: &csi.VolumeCapability{
					AccessType: &csi.VolumeCapability_Mount{
						Mount: &csi.VolumeCapability_MountVolume{
							FsType: "ext4",
						},
					},
					AccessMode: &csi.VolumeCapability_AccessMode{
						Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER,
					},
				},
				VolumeContext: map[string]string{
					InodeSizeKey: "-",
				},
				PublishContext: map[string]string{
					DevicePathKey: "/dev/xvdba",
				},
			},
			mounterMock: func(ctrl *gomock.Controller) *mounter.MockMounter {
				m := mounter.NewMockMounter(ctrl)
				return m
			},
			metadataMock: func(ctrl *gomock.Controller) *metadata.MockMetadataService {
				return nil
			},
			expectedErr: status.Error(codes.InvalidArgument, "Invalid inodesize (aborting!): <nil>"),
		},
		{
			name: "invalid_bytes_per_inode",
			req: &csi.NodeStageVolumeRequest{
				VolumeId:          "vol-test",
				StagingTargetPath: "/staging/path",
				VolumeCapability: &csi.VolumeCapability{
					AccessType: &csi.VolumeCapability_Mount{
						Mount: &csi.VolumeCapability_MountVolume{
							FsType: "ext4",
						},
					},
					AccessMode: &csi.VolumeCapability_AccessMode{
						Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER,
					},
				},
				VolumeContext: map[string]string{
					BytesPerInodeKey: "-",
				},
				PublishContext: map[string]string{
					DevicePathKey: "/dev/xvdba",
				},
			},
			mounterMock: func(ctrl *gomock.Controller) *mounter.MockMounter {
				m := mounter.NewMockMounter(ctrl)
				return m
			},
			metadataMock: func(ctrl *gomock.Controller) *metadata.MockMetadataService {
				return nil
			},
			expectedErr: status.Error(codes.InvalidArgument, "Invalid bytesperinode (aborting!): <nil>"),
		},
		{
			name: "invalid_number_of_inodes",
			req: &csi.NodeStageVolumeRequest{
				VolumeId:          "vol-test",
				StagingTargetPath: "/staging/path",
				VolumeCapability: &csi.VolumeCapability{
					AccessType: &csi.VolumeCapability_Mount{
						Mount: &csi.VolumeCapability_MountVolume{
							FsType: "ext4",
						},
					},
					AccessMode: &csi.VolumeCapability_AccessMode{
						Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER,
					},
				},
				VolumeContext: map[string]string{
					NumberOfInodesKey: "-",
				},
				PublishContext: map[string]string{
					DevicePathKey: "/dev/xvdba",
				},
			},
			mounterMock: func(ctrl *gomock.Controller) *mounter.MockMounter {
				m := mounter.NewMockMounter(ctrl)
				return m
			},
			metadataMock: func(ctrl *gomock.Controller) *metadata.MockMetadataService {
				return nil
			},
			expectedErr: status.Error(codes.InvalidArgument, "Invalid numberofinodes (aborting!): <nil>"),
		},
		{
			name: "invalid_ext4_bigalloc",
			req: &csi.NodeStageVolumeRequest{
				VolumeId:          "vol-test",
				StagingTargetPath: "/staging/path",
				VolumeCapability: &csi.VolumeCapability{
					AccessType: &csi.VolumeCapability_Mount{
						Mount: &csi.VolumeCapability_MountVolume{
							FsType: "ext4",
						},
					},
					AccessMode: &csi.VolumeCapability_AccessMode{
						Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER,
					},
				},
				VolumeContext: map[string]string{
					Ext4BigAllocKey: "-",
				},
				PublishContext: map[string]string{
					DevicePathKey: "/dev/xvdba",
				},
			},
			mounterMock: func(ctrl *gomock.Controller) *mounter.MockMounter {
				m := mounter.NewMockMounter(ctrl)
				return m
			},
			metadataMock: func(ctrl *gomock.Controller) *metadata.MockMetadataService {
				return nil
			},
			expectedErr: status.Error(codes.InvalidArgument, "Invalid ext4bigalloc (aborting!): <nil>"),
		},
		{
			name: "invalid_ext4_cluster_size",
			req: &csi.NodeStageVolumeRequest{
				VolumeId:          "vol-test",
				StagingTargetPath: "/staging/path",
				VolumeCapability: &csi.VolumeCapability{
					AccessType: &csi.VolumeCapability_Mount{
						Mount: &csi.VolumeCapability_MountVolume{
							FsType: "ext4",
						},
					},
					AccessMode: &csi.VolumeCapability_AccessMode{
						Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER,
					},
				},
				VolumeContext: map[string]string{
					Ext4ClusterSizeKey: "-",
				},
				PublishContext: map[string]string{
					DevicePathKey: "/dev/xvdba",
				},
			},
			mounterMock: func(ctrl *gomock.Controller) *mounter.MockMounter {
				m := mounter.NewMockMounter(ctrl)
				return m
			},
			metadataMock: func(ctrl *gomock.Controller) *metadata.MockMetadataService {
				return nil
			},
			expectedErr: status.Error(codes.InvalidArgument, "Invalid ext4clustersize (aborting!): <nil>"),
		},
		{
			name: "device_path_not_provided",
			req: &csi.NodeStageVolumeRequest{
				VolumeId:          "vol-test",
				StagingTargetPath: "/staging/path",
				VolumeCapability: &csi.VolumeCapability{
					AccessType: &csi.VolumeCapability_Mount{
						Mount: &csi.VolumeCapability_MountVolume{
							FsType: "ext4",
						},
					},
					AccessMode: &csi.VolumeCapability_AccessMode{
						Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER,
					},
				},
				VolumeContext: map[string]string{
					Ext4ClusterSizeKey: "51",
				},
				PublishContext: map[string]string{},
			},
			mounterMock: func(ctrl *gomock.Controller) *mounter.MockMounter {
				m := mounter.NewMockMounter(ctrl)
				return m
			},
			metadataMock: func(ctrl *gomock.Controller) *metadata.MockMetadataService {
				return nil
			},
			expectedErr: status.Error(codes.InvalidArgument, "Device path not provided"),
		},
		{
			name: "volume_operation_already_exists",
			req: &csi.NodeStageVolumeRequest{
				VolumeId:          "vol-test",
				StagingTargetPath: "/staging/path",
				VolumeCapability: &csi.VolumeCapability{
					AccessType: &csi.VolumeCapability_Mount{
						Mount: &csi.VolumeCapability_MountVolume{
							FsType: "ext4",
						},
					},
					AccessMode: &csi.VolumeCapability_AccessMode{
						Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER,
					},
				},
				PublishContext: map[string]string{
					DevicePathKey: "/dev/xvdba",
				},
			},
			mounterMock: func(ctrl *gomock.Controller) *mounter.MockMounter {
				m := mounter.NewMockMounter(ctrl)
				return m
			},
			metadataMock: func(ctrl *gomock.Controller) *metadata.MockMetadataService {
				return nil
			},
			expectedErr: status.Errorf(codes.Aborted, VolumeOperationAlreadyExists, "vol-test"),
			inflight:    true,
		},
		{
			name: "valid_partition",
			req: &csi.NodeStageVolumeRequest{
				VolumeId:          "vol-test",
				StagingTargetPath: "/staging/path",
				VolumeCapability: &csi.VolumeCapability{
					AccessType: &csi.VolumeCapability_Mount{
						Mount: &csi.VolumeCapability_MountVolume{
							FsType: "ext4",
						},
					},
					AccessMode: &csi.VolumeCapability_AccessMode{
						Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER,
					},
				},
				VolumeContext: map[string]string{
					VolumeAttributePartition: "1",
				},
				PublishContext: map[string]string{
					DevicePathKey: "/dev/xvdba",
				},
			},
			mounterMock: func(ctrl *gomock.Controller) *mounter.MockMounter {
				m := mounter.NewMockMounter(ctrl)
				m.EXPECT().FindDevicePath(gomock.Any(), gomock.Any(), "1", gomock.Any()).Return("/dev/xvdba1", nil)
				m.EXPECT().PathExists(gomock.Any()).Return(true, nil)
				m.EXPECT().GetDeviceNameFromMount(gomock.Any()).Return("", 1, nil)
				m.EXPECT().FormatAndMountSensitiveWithFormatOptions(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
				m.EXPECT().NeedResize(gomock.Any(), gomock.Any()).Return(false, nil)
				return m
			},
			metadataMock: func(ctrl *gomock.Controller) *metadata.MockMetadataService {
				m := metadata.NewMockMetadataService(ctrl)
				m.EXPECT().GetRegion().Return("us-west-2")
				return m
			},
			expectedErr: nil,
		},
		{
			name: "invalid_partition",
			req: &csi.NodeStageVolumeRequest{
				VolumeId:          "vol-test",
				StagingTargetPath: "/staging/path",
				VolumeCapability: &csi.VolumeCapability{
					AccessType: &csi.VolumeCapability_Mount{
						Mount: &csi.VolumeCapability_MountVolume{
							FsType: "ext4",
						},
					},
					AccessMode: &csi.VolumeCapability_AccessMode{
						Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER,
					},
				},
				VolumeContext: map[string]string{
					VolumeAttributePartition: "0",
				},
				PublishContext: map[string]string{
					DevicePathKey: "/dev/xvdba",
				},
			},
			mounterMock: func(ctrl *gomock.Controller) *mounter.MockMounter {
				m := mounter.NewMockMounter(ctrl)
				m.EXPECT().FindDevicePath(gomock.Any(), gomock.Any(), "", gomock.Any()).Return("/dev/xvdba", nil)
				m.EXPECT().PathExists(gomock.Any()).Return(true, nil)
				m.EXPECT().GetDeviceNameFromMount(gomock.Any()).Return("", 1, nil)
				m.EXPECT().FormatAndMountSensitiveWithFormatOptions(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
				m.EXPECT().NeedResize(gomock.Any(), gomock.Any()).Return(false, nil)
				return m
			},
			metadataMock: func(ctrl *gomock.Controller) *metadata.MockMetadataService {
				m := metadata.NewMockMetadataService(ctrl)
				m.EXPECT().GetRegion().Return("us-west-2")
				return m
			},
			expectedErr: nil,
		},
		{
			name: "find_device_path_error",
			req: &csi.NodeStageVolumeRequest{
				VolumeId:          "vol-test",
				StagingTargetPath: "/staging/path",
				VolumeCapability: &csi.VolumeCapability{
					AccessType: &csi.VolumeCapability_Mount{
						Mount: &csi.VolumeCapability_MountVolume{
							FsType: "ext4",
						},
					},
					AccessMode: &csi.VolumeCapability_AccessMode{
						Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER,
					},
				},
				PublishContext: map[string]string{
					DevicePathKey: "/dev/xvdba",
				},
			},
			mounterMock: func(ctrl *gomock.Controller) *mounter.MockMounter {
				m := mounter.NewMockMounter(ctrl)
				m.EXPECT().FindDevicePath(gomock.Eq("/dev/xvdba"), gomock.Eq("vol-test"), gomock.Eq(""), gomock.Eq("us-west-2")).Return("", errors.New("find device path error"))
				return m
			},
			metadataMock: func(ctrl *gomock.Controller) *metadata.MockMetadataService {
				m := metadata.NewMockMetadataService(ctrl)
				m.EXPECT().GetRegion().Return("us-west-2")
				return m
			},
			expectedErr: status.Errorf(codes.Internal, "Failed to find device path %s. %v", "/dev/xvdba", errors.New("find device path error")),
		},
		{
			name: "path_exists_error",
			req: &csi.NodeStageVolumeRequest{
				VolumeId:          "vol-test",
				StagingTargetPath: "/staging/path",
				VolumeCapability: &csi.VolumeCapability{
					AccessType: &csi.VolumeCapability_Mount{
						Mount: &csi.VolumeCapability_MountVolume{
							FsType: "ext4",
						},
					},
					AccessMode: &csi.VolumeCapability_AccessMode{
						Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER,
					},
				},
				PublishContext: map[string]string{
					DevicePathKey: "/dev/xvdba",
				},
			},
			mounterMock: func(ctrl *gomock.Controller) *mounter.MockMounter {
				m := mounter.NewMockMounter(ctrl)
				m.EXPECT().FindDevicePath(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return("/dev/xvdba", nil)
				m.EXPECT().PathExists(gomock.Eq("/staging/path")).Return(false, errors.New("path exists error"))
				return m
			},
			metadataMock: func(ctrl *gomock.Controller) *metadata.MockMetadataService {
				m := metadata.NewMockMetadataService(ctrl)
				m.EXPECT().GetRegion().Return("us-west-2")
				return m
			},
			expectedErr: status.Error(codes.Internal, "failed to check if target \"/staging/path\" exists: path exists error"),
		},
		{
			name: "create_target_dir_error",
			req: &csi.NodeStageVolumeRequest{
				VolumeId:          "vol-test",
				StagingTargetPath: "/staging/path",
				VolumeCapability: &csi.VolumeCapability{
					AccessType: &csi.VolumeCapability_Mount{
						Mount: &csi.VolumeCapability_MountVolume{
							FsType: "ext4",
						},
					},
					AccessMode: &csi.VolumeCapability_AccessMode{
						Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER,
					},
				},
				PublishContext: map[string]string{
					DevicePathKey: "/dev/xvdba",
				},
			},
			mounterMock: func(ctrl *gomock.Controller) *mounter.MockMounter {
				m := mounter.NewMockMounter(ctrl)
				m.EXPECT().FindDevicePath(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return("/dev/xvdba", nil)
				m.EXPECT().PathExists(gomock.Eq("/staging/path")).Return(false, nil)
				m.EXPECT().MakeDir(gomock.Eq("/staging/path")).Return(errors.New("make dir error"))
				return m
			},
			metadataMock: func(ctrl *gomock.Controller) *metadata.MockMetadataService {
				m := metadata.NewMockMetadataService(ctrl)
				m.EXPECT().GetRegion().Return("us-west-2")
				return m
			},
			expectedErr: status.Error(codes.Internal, "could not create target dir \"/staging/path\": make dir error"),
		},
		{
			name: "get_device_name_from_mount_error",
			req: &csi.NodeStageVolumeRequest{
				VolumeId:          "vol-test",
				StagingTargetPath: "/staging/path",
				VolumeCapability: &csi.VolumeCapability{
					AccessType: &csi.VolumeCapability_Mount{
						Mount: &csi.VolumeCapability_MountVolume{
							FsType: "ext4",
						},
					},
					AccessMode: &csi.VolumeCapability_AccessMode{
						Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER,
					},
				},
				PublishContext: map[string]string{
					DevicePathKey: "/dev/xvdba",
				},
			},
			mounterMock: func(ctrl *gomock.Controller) *mounter.MockMounter {
				m := mounter.NewMockMounter(ctrl)
				m.EXPECT().FindDevicePath(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return("/dev/xvdba", nil)
				m.EXPECT().PathExists(gomock.Eq("/staging/path")).Return(true, nil)
				m.EXPECT().GetDeviceNameFromMount(gomock.Eq("/staging/path")).Return("", 0, errors.New("get device name error"))
				return m
			},
			metadataMock: func(ctrl *gomock.Controller) *metadata.MockMetadataService {
				m := metadata.NewMockMetadataService(ctrl)
				m.EXPECT().GetRegion().Return("us-west-2")
				return m
			},
			expectedErr: status.Error(codes.Internal, "failed to check if volume is already mounted: get device name error"),
		},
		{
			name: "volume_already_staged",
			req: &csi.NodeStageVolumeRequest{
				VolumeId:          "vol-test",
				StagingTargetPath: "/staging/path",
				VolumeCapability: &csi.VolumeCapability{
					AccessType: &csi.VolumeCapability_Mount{
						Mount: &csi.VolumeCapability_MountVolume{
							FsType: "ext4",
						},
					},
					AccessMode: &csi.VolumeCapability_AccessMode{
						Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER,
					},
				},
				PublishContext: map[string]string{
					DevicePathKey: "/dev/xvdba",
				},
			},
			mounterMock: func(ctrl *gomock.Controller) *mounter.MockMounter {
				m := mounter.NewMockMounter(ctrl)
				m.EXPECT().FindDevicePath(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return("/dev/xvdba", nil)
				m.EXPECT().PathExists(gomock.Eq("/staging/path")).Return(true, nil)
				m.EXPECT().GetDeviceNameFromMount(gomock.Eq("/staging/path")).Return("/dev/xvdba", 1, nil)
				return m
			},
			metadataMock: func(ctrl *gomock.Controller) *metadata.MockMetadataService {
				m := metadata.NewMockMetadataService(ctrl)
				m.EXPECT().GetRegion().Return("us-west-2")
				return m
			},
			expectedErr: nil,
		},
		{
			name: "format_and_mount_error",
			req: &csi.NodeStageVolumeRequest{
				VolumeId:          "vol-test",
				StagingTargetPath: "/staging/path",
				VolumeCapability: &csi.VolumeCapability{
					AccessType: &csi.VolumeCapability_Mount{
						Mount: &csi.VolumeCapability_MountVolume{
							FsType: "ext4",
						},
					},
					AccessMode: &csi.VolumeCapability_AccessMode{
						Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER,
					},
				},
				PublishContext: map[string]string{
					DevicePathKey: "/dev/xvdba",
				},
			},
			mounterMock: func(ctrl *gomock.Controller) *mounter.MockMounter {
				m := mounter.NewMockMounter(ctrl)
				m.EXPECT().FindDevicePath(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return("/dev/xvdba", nil)
				m.EXPECT().PathExists(gomock.Eq("/staging/path")).Return(true, nil)
				m.EXPECT().GetDeviceNameFromMount(gomock.Eq("/staging/path")).Return("", 1, nil)
				m.EXPECT().FormatAndMountSensitiveWithFormatOptions(gomock.Eq("/dev/xvdba"), gomock.Eq("/staging/path"), gomock.Eq("ext4"), gomock.Any(), gomock.Any(), gomock.Any()).Return(errors.New("format and mount error"))
				return m
			},
			metadataMock: func(ctrl *gomock.Controller) *metadata.MockMetadataService {
				m := metadata.NewMockMetadataService(ctrl)
				m.EXPECT().GetRegion().Return("us-west-2")
				return m
			},
			expectedErr: status.Error(codes.Internal, "could not format \"/dev/xvdba\" and mount it at \"/staging/path\": format and mount error"),
		},
		{
			name: "need_resize_error",
			req: &csi.NodeStageVolumeRequest{
				VolumeId:          "vol-test",
				StagingTargetPath: "/staging/path",
				VolumeCapability: &csi.VolumeCapability{
					AccessType: &csi.VolumeCapability_Mount{
						Mount: &csi.VolumeCapability_MountVolume{
							FsType: "ext4",
						},
					},
					AccessMode: &csi.VolumeCapability_AccessMode{
						Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER,
					},
				},
				PublishContext: map[string]string{
					DevicePathKey: "/dev/xvdba",
				},
			},
			mounterMock: func(ctrl *gomock.Controller) *mounter.MockMounter {
				m := mounter.NewMockMounter(ctrl)
				m.EXPECT().FindDevicePath(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return("/dev/xvdba", nil)
				m.EXPECT().PathExists(gomock.Any()).Return(true, nil)
				m.EXPECT().GetDeviceNameFromMount(gomock.Any()).Return("", 1, nil)
				m.EXPECT().FormatAndMountSensitiveWithFormatOptions(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
				m.EXPECT().NeedResize(gomock.Eq("/dev/xvdba"), gomock.Eq("/staging/path")).Return(false, errors.New("need resize error"))
				return m
			},
			metadataMock: func(ctrl *gomock.Controller) *metadata.MockMetadataService {
				m := metadata.NewMockMetadataService(ctrl)
				m.EXPECT().GetRegion().Return("us-west-2")
				return m
			},
			expectedErr: status.Error(codes.Internal, "Could not determine if volume \"vol-test\" (\"/dev/xvdba\") need to be resized:  need resize error"),
		},
		{
			name: "resize_error",
			req: &csi.NodeStageVolumeRequest{
				VolumeId:          "vol-test",
				StagingTargetPath: "/staging/path",
				VolumeCapability: &csi.VolumeCapability{
					AccessType: &csi.VolumeCapability_Mount{
						Mount: &csi.VolumeCapability_MountVolume{
							FsType: "ext4",
						},
					},
					AccessMode: &csi.VolumeCapability_AccessMode{
						Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER,
					},
				},
				PublishContext: map[string]string{
					DevicePathKey: "/dev/xvdba",
				},
			},
			mounterMock: func(ctrl *gomock.Controller) *mounter.MockMounter {
				m := mounter.NewMockMounter(ctrl)
				m.EXPECT().FindDevicePath(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return("/dev/xvdba", nil)
				m.EXPECT().PathExists(gomock.Any()).Return(true, nil)
				m.EXPECT().GetDeviceNameFromMount(gomock.Any()).Return("", 1, nil)
				m.EXPECT().FormatAndMountSensitiveWithFormatOptions(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
				m.EXPECT().NeedResize(gomock.Eq("/dev/xvdba"), gomock.Eq("/staging/path")).Return(true, nil)
				m.EXPECT().Resize(gomock.Eq("/dev/xvdba"), gomock.Eq("/staging/path")).Return(false, errors.New("resize error"))
				return m
			},
			metadataMock: func(ctrl *gomock.Controller) *metadata.MockMetadataService {
				m := metadata.NewMockMetadataService(ctrl)
				m.EXPECT().GetRegion().Return("us-west-2")
				return m
			},
			expectedErr: status.Error(codes.Internal, "Could not resize volume \"vol-test\" (\"/dev/xvdba\"):  resize error"),
		},
		{
			name: "format_options_ext4",
			req: &csi.NodeStageVolumeRequest{
				VolumeId:          "vol-test",
				StagingTargetPath: "/staging/path",
				VolumeCapability: &csi.VolumeCapability{
					AccessType: &csi.VolumeCapability_Mount{
						Mount: &csi.VolumeCapability_MountVolume{
							FsType: "ext4",
						},
					},
					AccessMode: &csi.VolumeCapability_AccessMode{
						Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER,
					},
				},
				VolumeContext: map[string]string{
					BlockSizeKey:       "4096",
					InodeSizeKey:       "512",
					BytesPerInodeKey:   "16384",
					NumberOfInodesKey:  "1000000",
					Ext4BigAllocKey:    "true",
					Ext4ClusterSizeKey: "65536",
				},
				PublishContext: map[string]string{
					DevicePathKey: "/dev/xvdba",
				},
			},
			mounterMock: func(ctrl *gomock.Controller) *mounter.MockMounter {
				m := mounter.NewMockMounter(ctrl)
				m.EXPECT().FindDevicePath(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return("/dev/xvdba", nil)
				m.EXPECT().PathExists(gomock.Eq("/staging/path")).Return(true, nil)
				m.EXPECT().GetDeviceNameFromMount(gomock.Eq("/staging/path")).Return("", 1, nil)
				m.EXPECT().FormatAndMountSensitiveWithFormatOptions(gomock.Eq("/dev/xvdba"), gomock.Eq("/staging/path"), gomock.Eq("ext4"), gomock.Any(), gomock.Any(), gomock.Eq([]string{"-b", "4096", "-I", "512", "-i", "16384", "-N", "1000000", "-O", "bigalloc", "-C", "65536"})).Return(nil)
				m.EXPECT().NeedResize(gomock.Eq("/dev/xvdba"), gomock.Eq("/staging/path")).Return(false, nil)
				return m
			},
			metadataMock: func(ctrl *gomock.Controller) *metadata.MockMetadataService {
				m := metadata.NewMockMetadataService(ctrl)
				m.EXPECT().GetRegion().Return("us-west-2")
				return m
			},
			expectedErr: nil,
		},
		{
			name: "format_options_xfs",
			req: &csi.NodeStageVolumeRequest{
				VolumeId:          "vol-test",
				StagingTargetPath: "/staging/path",
				VolumeCapability: &csi.VolumeCapability{
					AccessType: &csi.VolumeCapability_Mount{
						Mount: &csi.VolumeCapability_MountVolume{
							FsType: "xfs",
						},
					},
					AccessMode: &csi.VolumeCapability_AccessMode{
						Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER,
					},
				},
				VolumeContext: map[string]string{
					BlockSizeKey: "4096",
					InodeSizeKey: "512",
				},
				PublishContext: map[string]string{
					DevicePathKey: "/dev/xvdba",
				},
			},
			mounterMock: func(ctrl *gomock.Controller) *mounter.MockMounter {
				m := mounter.NewMockMounter(ctrl)
				m.EXPECT().FindDevicePath(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return("/dev/xvdba", nil)
				m.EXPECT().PathExists(gomock.Eq("/staging/path")).Return(true, nil)
				m.EXPECT().GetDeviceNameFromMount(gomock.Eq("/staging/path")).Return("", 1, nil)
				m.EXPECT().FormatAndMountSensitiveWithFormatOptions(gomock.Eq("/dev/xvdba"), gomock.Eq("/staging/path"), gomock.Eq("xfs"), gomock.Any(), gomock.Any(), gomock.Eq([]string{"-b", "size=4096", "-i", "size=512"})).Return(nil)
				m.EXPECT().NeedResize(gomock.Eq("/dev/xvdba"), gomock.Eq("/staging/path")).Return(false, nil)
				return m
			},
			metadataMock: func(ctrl *gomock.Controller) *metadata.MockMetadataService {
				m := metadata.NewMockMetadataService(ctrl)
				m.EXPECT().GetRegion().Return("us-west-2")
				return m
			},
			expectedErr: nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			var mounter *mounter.MockMounter
			if tc.mounterMock != nil {
				mounter = tc.mounterMock(ctrl)
			}

			var metadata *metadata.MockMetadataService
			if tc.metadataMock != nil {
				metadata = tc.metadataMock(ctrl)
			}

			driver := &NodeService{
				metadata: metadata,
				mounter:  mounter,
				inFlight: internal.NewInFlight(),
			}

			if tc.inflight {
				driver.inFlight.Insert("vol-test")
			}

			_, err := driver.NodeStageVolume(context.Background(), tc.req)
			if !reflect.DeepEqual(err, tc.expectedErr) {
				t.Fatalf("Expected error '%v' but got '%v'", tc.expectedErr, err)
			}
		})
	}
}

func TestGetVolumesLimit(t *testing.T) {
	testCases := []struct {
		name         string
		expectedErr  error
		expectedVal  int64
		options      *Options
		metadataMock func(ctrl *gomock.Controller) *metadata.MockMetadataService
	}{
		{
			name: "VolumeAttachLimit_specified",
			options: &Options{
				VolumeAttachLimit:         10,
				ReservedVolumeAttachments: -1,
			},
			expectedVal: 10,
		},
		{
			name: "sbeDeviceVolumeAttachmentLimit",
			options: &Options{
				VolumeAttachLimit: -1,
			},
			expectedVal: sbeDeviceVolumeAttachmentLimit,
			metadataMock: func(ctrl *gomock.Controller) *metadata.MockMetadataService {
				m := metadata.NewMockMetadataService(ctrl)
				m.EXPECT().GetRegion().Return("snow")
				return m
			},
		},
		{
			name: "t2.medium_volume_attach_limit",
			options: &Options{
				VolumeAttachLimit:         -1,
				ReservedVolumeAttachments: -1,
			},
			expectedVal: 38,
			metadataMock: func(ctrl *gomock.Controller) *metadata.MockMetadataService {
				m := metadata.NewMockMetadataService(ctrl)
				m.EXPECT().GetRegion().Return("us-west-2")
				m.EXPECT().GetNumBlockDeviceMappings().Return(0)
				m.EXPECT().GetInstanceType().Return("t2.medium")
				return m
			},
		},
		{
			name: "ReservedVolumeAttachments_specified",
			options: &Options{
				VolumeAttachLimit:         -1,
				ReservedVolumeAttachments: 3,
			},
			expectedVal: 36,
			metadataMock: func(ctrl *gomock.Controller) *metadata.MockMetadataService {
				m := metadata.NewMockMetadataService(ctrl)
				m.EXPECT().GetRegion().Return("us-west-2")
				m.EXPECT().GetInstanceType().Return("t2.medium")
				return m
			},
		},
		{
			name: "m5d.large_volume_attach_limit",
			options: &Options{
				VolumeAttachLimit:         -1,
				ReservedVolumeAttachments: -1,
			},
			expectedVal: 23,
			metadataMock: func(ctrl *gomock.Controller) *metadata.MockMetadataService {
				m := metadata.NewMockMetadataService(ctrl)
				m.EXPECT().GetRegion().Return("us-west-2")
				m.EXPECT().GetInstanceType().Return("m5d.large")
				m.EXPECT().GetNumBlockDeviceMappings().Return(0)
				m.EXPECT().GetNumAttachedENIs().Return(3)
				return m
			},
		},
		{
			name: "d3en.12xlarge_volume_attach_limit",
			options: &Options{
				VolumeAttachLimit:         -1,
				ReservedVolumeAttachments: -1,
			},
			expectedVal: 1,
			metadataMock: func(ctrl *gomock.Controller) *metadata.MockMetadataService {
				m := metadata.NewMockMetadataService(ctrl)
				m.EXPECT().GetRegion().Return("us-west-2")
				m.EXPECT().GetInstanceType().Return("d3en.12xlarge")
				m.EXPECT().GetNumBlockDeviceMappings().Return(0)
				m.EXPECT().GetNumAttachedENIs().Return(1)
				return m
			},
		},
		{
			name: "d3.8xlarge_volume_attach_limit",
			options: &Options{
				VolumeAttachLimit:         -1,
				ReservedVolumeAttachments: -1,
			},
			expectedVal: 1,
			metadataMock: func(ctrl *gomock.Controller) *metadata.MockMetadataService {
				m := metadata.NewMockMetadataService(ctrl)
				m.EXPECT().GetRegion().Return("us-west-2")
				m.EXPECT().GetInstanceType().Return("d3.8xlarge")
				m.EXPECT().GetNumBlockDeviceMappings().Return(0)
				m.EXPECT().GetNumAttachedENIs().Return(1)
				return m
			},
		},
		{
			name: "nitro_volume_attach_limit",
			options: &Options{
				VolumeAttachLimit:         -1,
				ReservedVolumeAttachments: -1,
			},
			expectedVal: 127,
			metadataMock: func(ctrl *gomock.Controller) *metadata.MockMetadataService {
				m := metadata.NewMockMetadataService(ctrl)
				m.EXPECT().GetRegion().Return("us-west-2")
				m.EXPECT().GetInstanceType().Return("m7i.48xlarge")
				m.EXPECT().GetNumBlockDeviceMappings().Return(0)
				return m
			},
		},
		{
			name: "attached_max_enis",
			options: &Options{
				VolumeAttachLimit: -1,
			},
			expectedVal: 1,
			metadataMock: func(ctrl *gomock.Controller) *metadata.MockMetadataService {
				m := metadata.NewMockMetadataService(ctrl)
				m.EXPECT().GetRegion().Return("us-west-2")
				m.EXPECT().GetInstanceType().Return("t3.xlarge")
				m.EXPECT().GetNumAttachedENIs().Return(40)
				return m
			},
		},
		{
			name: "inf1.24xlarge_volume_attach_limit",
			options: &Options{
				VolumeAttachLimit:         -1,
				ReservedVolumeAttachments: -1,
			},
			expectedVal: 9,
			metadataMock: func(ctrl *gomock.Controller) *metadata.MockMetadataService {
				m := metadata.NewMockMetadataService(ctrl)
				m.EXPECT().GetRegion().Return("us-west-2")
				m.EXPECT().GetInstanceType().Return("inf1.24xlarge")
				m.EXPECT().GetNumAttachedENIs().Return(1)
				m.EXPECT().GetNumBlockDeviceMappings().Return(0)
				return m
			},
		},
		{
			name: "mac1.metal_volume_attach_limit",
			options: &Options{
				VolumeAttachLimit:         -1,
				ReservedVolumeAttachments: -1,
			},
			expectedVal: 14,
			metadataMock: func(ctrl *gomock.Controller) *metadata.MockMetadataService {
				m := metadata.NewMockMetadataService(ctrl)
				m.EXPECT().GetRegion().Return("us-west-2")
				m.EXPECT().GetInstanceType().Return("mac1.metal")
				m.EXPECT().GetNumBlockDeviceMappings().Return(0)
				m.EXPECT().GetNumAttachedENIs().Return(1)
				return m
			},
		},
		{
			name: "u-12tb1.metal_volume_attach_limit",
			options: &Options{
				VolumeAttachLimit:         -1,
				ReservedVolumeAttachments: -1,
			},
			expectedVal: 17,
			metadataMock: func(ctrl *gomock.Controller) *metadata.MockMetadataService {
				m := metadata.NewMockMetadataService(ctrl)
				m.EXPECT().GetRegion().Return("us-west-2")
				m.EXPECT().GetInstanceType().Return("u-12tb1.metal")
				m.EXPECT().GetNumBlockDeviceMappings().Return(0)
				m.EXPECT().GetNumAttachedENIs().Return(1)
				return m
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			var mounter *mounter.MockMounter

			var metadata *metadata.MockMetadataService
			if tc.metadataMock != nil {
				metadata = tc.metadataMock(ctrl)
			}

			driver := &NodeService{
				mounter:  mounter,
				inFlight: internal.NewInFlight(),
				options:  tc.options,
				metadata: metadata,
			}

			value := driver.getVolumesLimit()
			if value != tc.expectedVal {
				t.Fatalf("Expected value %v but got %v", tc.expectedVal, value)
			}
		})
	}
}

func TestNodePublishVolume(t *testing.T) {
	testCases := []struct {
		name         string
		req          *csi.NodePublishVolumeRequest
		mounterMock  func(ctrl *gomock.Controller) *mounter.MockMounter
		metadataMock func(ctrl *gomock.Controller) *metadata.MockMetadataService
		expectedErr  error
		inflight     bool
	}{
		{
			name: "success_block_device",
			req: &csi.NodePublishVolumeRequest{
				VolumeId:          "vol-test",
				StagingTargetPath: "/staging/path",
				TargetPath:        "/target/path",
				VolumeCapability: &csi.VolumeCapability{
					AccessType: &csi.VolumeCapability_Block{
						Block: &csi.VolumeCapability_BlockVolume{},
					},
					AccessMode: &csi.VolumeCapability_AccessMode{
						Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER,
					},
				},
				PublishContext: map[string]string{
					DevicePathKey: "/dev/xvdba",
				},
			},
			mounterMock: func(ctrl *gomock.Controller) *mounter.MockMounter {
				m := mounter.NewMockMounter(ctrl)

				m.EXPECT().FindDevicePath(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return("/dev/xvdba", nil)
				m.EXPECT().PathExists(gomock.Any()).Return(true, nil)
				m.EXPECT().MakeFile(gomock.Any()).Return(nil)
				m.EXPECT().IsLikelyNotMountPoint(gomock.Any()).Return(true, nil)
				m.EXPECT().Mount(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
				return m
			},
			metadataMock: func(ctrl *gomock.Controller) *metadata.MockMetadataService {
				m := metadata.NewMockMetadataService(ctrl)
				m.EXPECT().GetRegion().Return("us-west-2")
				return m
			},
		},
		{
			name: "success_fs",
			req: &csi.NodePublishVolumeRequest{
				VolumeId:          "vol-test",
				StagingTargetPath: "/staging/path",
				TargetPath:        "/target/path",
				VolumeCapability: &csi.VolumeCapability{
					AccessType: &csi.VolumeCapability_Mount{
						Mount: &csi.VolumeCapability_MountVolume{},
					},
					AccessMode: &csi.VolumeCapability_AccessMode{
						Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER,
					},
				},
				PublishContext: map[string]string{
					DevicePathKey: "/dev/xvdba",
				},
			},
			mounterMock: func(ctrl *gomock.Controller) *mounter.MockMounter {
				m := mounter.NewMockMounter(ctrl)
				m.EXPECT().PreparePublishTarget(gomock.Any()).Return(nil)
				m.EXPECT().IsLikelyNotMountPoint(gomock.Any()).Return(true, nil)
				m.EXPECT().Mount(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
				return m
			},
		},
		{
			name: "volume_id_not_provided",
			req: &csi.NodePublishVolumeRequest{
				StagingTargetPath: "/staging/path",
				TargetPath:        "/target/path",
				VolumeCapability: &csi.VolumeCapability{
					AccessType: &csi.VolumeCapability_Block{
						Block: &csi.VolumeCapability_BlockVolume{},
					},
					AccessMode: &csi.VolumeCapability_AccessMode{
						Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER,
					},
				},
				PublishContext: map[string]string{
					DevicePathKey: "/dev/xvdba",
				},
			},
			expectedErr: status.Error(codes.InvalidArgument, "Volume ID not provided"),
		},
		{
			name: "staging_target_path_not_provided",
			req: &csi.NodePublishVolumeRequest{
				VolumeId:   "vol-test",
				TargetPath: "/target/path",
				VolumeCapability: &csi.VolumeCapability{
					AccessType: &csi.VolumeCapability_Block{
						Block: &csi.VolumeCapability_BlockVolume{},
					},
					AccessMode: &csi.VolumeCapability_AccessMode{
						Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER,
					},
				},
				PublishContext: map[string]string{
					DevicePathKey: "/dev/xvdba",
				},
			},
			expectedErr: status.Error(codes.InvalidArgument, "Staging target not provided"),
		},
		{
			name: "target_path_not_provided",
			req: &csi.NodePublishVolumeRequest{
				VolumeId:          "vol-test",
				StagingTargetPath: "/staging/path",
				VolumeCapability: &csi.VolumeCapability{
					AccessType: &csi.VolumeCapability_Block{
						Block: &csi.VolumeCapability_BlockVolume{},
					},
					AccessMode: &csi.VolumeCapability_AccessMode{
						Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER,
					},
				},
				PublishContext: map[string]string{
					DevicePathKey: "/dev/xvdba",
				},
			},
			expectedErr: status.Error(codes.InvalidArgument, "Target path not provided"),
		},
		{
			name: "capability_not_provided",
			req: &csi.NodePublishVolumeRequest{
				VolumeId:          "vol-test",
				StagingTargetPath: "/staging/path",
				TargetPath:        "/target/path",
				PublishContext: map[string]string{
					DevicePathKey: "/dev/xvdba",
				},
			},
			expectedErr: status.Error(codes.InvalidArgument, "Volume capability not provided"),
		},
		{
			name: "success_block_device",
			req: &csi.NodePublishVolumeRequest{
				VolumeId:          "vol-test",
				StagingTargetPath: "/staging/path",
				TargetPath:        "/target/path",
				VolumeCapability: &csi.VolumeCapability{
					AccessType: &csi.VolumeCapability_Block{
						Block: &csi.VolumeCapability_BlockVolume{},
					},
					AccessMode: &csi.VolumeCapability_AccessMode{
						Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER,
					},
				},
				PublishContext: map[string]string{
					DevicePathKey: "/dev/xvdba",
				},
			},
			inflight:    true,
			expectedErr: status.Errorf(codes.Aborted, VolumeOperationAlreadyExists, "vol-test"),
		},
		{
			name: "success_block_device",
			req: &csi.NodePublishVolumeRequest{
				VolumeId:          "vol-test",
				StagingTargetPath: "/staging/path",
				TargetPath:        "/target/path",
				VolumeCapability: &csi.VolumeCapability{
					AccessType: &csi.VolumeCapability_Block{
						Block: &csi.VolumeCapability_BlockVolume{},
					},
					AccessMode: &csi.VolumeCapability_AccessMode{
						Mode: csi.VolumeCapability_AccessMode_Mode(csi.ControllerServiceCapability_RPC_UNKNOWN),
					},
				},
				PublishContext: map[string]string{
					DevicePathKey: "/dev/xvdba",
				},
			},
			expectedErr: status.Errorf(codes.InvalidArgument, "Volume capability not supported"),
		},
		{
			name: "read_only_enabled",
			req: &csi.NodePublishVolumeRequest{
				VolumeId:          "vol-test",
				StagingTargetPath: "/staging/path",
				TargetPath:        "/target/path",
				VolumeCapability: &csi.VolumeCapability{
					AccessType: &csi.VolumeCapability_Block{
						Block: &csi.VolumeCapability_BlockVolume{},
					},
					AccessMode: &csi.VolumeCapability_AccessMode{
						Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER,
					},
				},
				PublishContext: map[string]string{
					DevicePathKey: "/dev/xvdba",
				},
				Readonly: true,
			},
			mounterMock: func(ctrl *gomock.Controller) *mounter.MockMounter {
				m := mounter.NewMockMounter(ctrl)

				m.EXPECT().FindDevicePath(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return("/dev/xvdba", nil)
				m.EXPECT().PathExists(gomock.Any()).Return(true, nil)
				m.EXPECT().MakeFile(gomock.Any()).Return(nil)
				m.EXPECT().IsLikelyNotMountPoint(gomock.Any()).Return(true, nil)
				m.EXPECT().Mount(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
				return m
			},
			metadataMock: func(ctrl *gomock.Controller) *metadata.MockMetadataService {
				m := metadata.NewMockMetadataService(ctrl)
				m.EXPECT().GetRegion().Return("us-west-2")
				return m
			},
		},
		{
			name: "nodePublishVolumeForBlock_error",
			req: &csi.NodePublishVolumeRequest{
				VolumeId:          "vol-test",
				StagingTargetPath: "/staging/path",
				TargetPath:        "/target/path",
				VolumeCapability: &csi.VolumeCapability{
					AccessType: &csi.VolumeCapability_Block{
						Block: &csi.VolumeCapability_BlockVolume{},
					},
					AccessMode: &csi.VolumeCapability_AccessMode{
						Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER,
					},
				},
			},
			expectedErr: status.Errorf(codes.InvalidArgument, "Device path not provided"),
		},
		{
			name: "nodePublishVolumeForBlock_invalid_volume_attribute",
			req: &csi.NodePublishVolumeRequest{
				VolumeId:          "vol-test",
				StagingTargetPath: "/staging/path",
				TargetPath:        "/target/path",
				VolumeCapability: &csi.VolumeCapability{
					AccessType: &csi.VolumeCapability_Block{
						Block: &csi.VolumeCapability_BlockVolume{},
					},
					AccessMode: &csi.VolumeCapability_AccessMode{
						Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER,
					},
				},
				PublishContext: map[string]string{
					DevicePathKey: "/dev/xvdba",
				},
				VolumeContext: map[string]string{
					VolumeAttributePartition: "invalid-partition",
				},
			},
			expectedErr: status.Error(codes.InvalidArgument, "Volume Attribute is invalid"),
		},
		{
			name: "nodePublishVolumeForBlock_invalid_partition",
			req: &csi.NodePublishVolumeRequest{
				VolumeId:          "vol-test",
				StagingTargetPath: "/staging/path",
				TargetPath:        "/target/path",
				VolumeCapability: &csi.VolumeCapability{
					AccessType: &csi.VolumeCapability_Block{
						Block: &csi.VolumeCapability_BlockVolume{},
					},
					AccessMode: &csi.VolumeCapability_AccessMode{
						Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER,
					},
				},
				PublishContext: map[string]string{
					DevicePathKey: "/dev/xvdba",
				},
				VolumeContext: map[string]string{
					VolumeAttributePartition: "0",
				},
			},
			mounterMock: func(ctrl *gomock.Controller) *mounter.MockMounter {
				m := mounter.NewMockMounter(ctrl)

				m.EXPECT().FindDevicePath(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return("/dev/xvdba", nil)
				m.EXPECT().PathExists(gomock.Any()).Return(true, nil)
				m.EXPECT().MakeFile(gomock.Any()).Return(nil)
				m.EXPECT().IsLikelyNotMountPoint(gomock.Any()).Return(true, nil)
				m.EXPECT().Mount(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
				return m
			},
			metadataMock: func(ctrl *gomock.Controller) *metadata.MockMetadataService {
				m := metadata.NewMockMetadataService(ctrl)
				m.EXPECT().GetRegion().Return("us-west-2")
				return m
			},
		},
		{
			name: "nodePublishVolumeForBlock_valid_partition",
			req: &csi.NodePublishVolumeRequest{
				VolumeId:          "vol-test",
				StagingTargetPath: "/staging/path",
				TargetPath:        "/target/path",
				VolumeCapability: &csi.VolumeCapability{
					AccessType: &csi.VolumeCapability_Block{
						Block: &csi.VolumeCapability_BlockVolume{},
					},
					AccessMode: &csi.VolumeCapability_AccessMode{
						Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER,
					},
				},
				PublishContext: map[string]string{
					DevicePathKey: "/dev/xvdba",
				},
				VolumeContext: map[string]string{
					VolumeAttributePartition: "1",
				},
			},
			mounterMock: func(ctrl *gomock.Controller) *mounter.MockMounter {
				m := mounter.NewMockMounter(ctrl)

				m.EXPECT().FindDevicePath(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return("/dev/xvdba", nil)
				m.EXPECT().PathExists(gomock.Any()).Return(true, nil)
				m.EXPECT().MakeFile(gomock.Any()).Return(nil)
				m.EXPECT().IsLikelyNotMountPoint(gomock.Any()).Return(true, nil)
				m.EXPECT().Mount(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
				return m
			},
			metadataMock: func(ctrl *gomock.Controller) *metadata.MockMetadataService {
				m := metadata.NewMockMetadataService(ctrl)
				m.EXPECT().GetRegion().Return("us-west-2")
				return m
			},
		},
		{
			name: "nodePublishVolumeForBlock_device_path_failure",
			req: &csi.NodePublishVolumeRequest{
				VolumeId:          "vol-test",
				StagingTargetPath: "/staging/path",
				TargetPath:        "/target/path",
				VolumeCapability: &csi.VolumeCapability{
					AccessType: &csi.VolumeCapability_Block{
						Block: &csi.VolumeCapability_BlockVolume{},
					},
					AccessMode: &csi.VolumeCapability_AccessMode{
						Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER,
					},
				},
				PublishContext: map[string]string{
					DevicePathKey: "/dev/xvdba",
				},
			},
			mounterMock: func(ctrl *gomock.Controller) *mounter.MockMounter {
				m := mounter.NewMockMounter(ctrl)

				m.EXPECT().FindDevicePath(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return("", errors.New("device path error"))
				return m
			},
			metadataMock: func(ctrl *gomock.Controller) *metadata.MockMetadataService {
				m := metadata.NewMockMetadataService(ctrl)
				m.EXPECT().GetRegion().Return("us-west-2")
				return m
			},
			expectedErr: status.Error(codes.Internal, "Failed to find device path /dev/xvdba. device path error"),
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			var mounter *mounter.MockMounter
			if tc.mounterMock != nil {
				mounter = tc.mounterMock(ctrl)
			}

			var metadata *metadata.MockMetadataService
			if tc.metadataMock != nil {
				metadata = tc.metadataMock(ctrl)
			}

			driver := &NodeService{
				metadata: metadata,
				mounter:  mounter,
				inFlight: internal.NewInFlight(),
			}

			if tc.inflight {
				driver.inFlight.Insert("vol-test")
			}

			_, err := driver.NodePublishVolume(context.Background(), tc.req)
			if !reflect.DeepEqual(err, tc.expectedErr) {
				t.Fatalf("Expected error '%v' but got '%v'", tc.expectedErr, err)
			}
		})
	}
}

func TestNodeUnstageVolume(t *testing.T) {
	testCases := []struct {
		name        string
		req         *csi.NodeUnstageVolumeRequest
		mounterMock func(ctrl *gomock.Controller) *mounter.MockMounter
		expectedErr error
		inflight    bool
	}{
		{
			name: "success",
			req: &csi.NodeUnstageVolumeRequest{
				VolumeId:          "vol-test",
				StagingTargetPath: "/staging/path",
			},
			mounterMock: func(ctrl *gomock.Controller) *mounter.MockMounter {
				m := mounter.NewMockMounter(ctrl)
				m.EXPECT().GetDeviceNameFromMount(gomock.Any()).Return("dev-test", 1, nil)
				m.EXPECT().Unstage(gomock.Any()).Return(nil)
				return m
			},
		},
		{
			name: "missing_volume_id",
			req: &csi.NodeUnstageVolumeRequest{
				StagingTargetPath: "/staging/path",
			},
			expectedErr: status.Error(codes.InvalidArgument, "Volume ID not provided"),
		},
		{
			name: "missing_staging_target",
			req: &csi.NodeUnstageVolumeRequest{
				VolumeId: "vol-test",
			},
			expectedErr: status.Error(codes.InvalidArgument, "Staging target not provided"),
		},
		{
			name: "unstage_failed",
			req: &csi.NodeUnstageVolumeRequest{
				VolumeId:          "vol-test",
				StagingTargetPath: "/staging/path",
			},
			mounterMock: func(ctrl *gomock.Controller) *mounter.MockMounter {
				m := mounter.NewMockMounter(ctrl)
				m.EXPECT().GetDeviceNameFromMount(gomock.Any()).Return("", 1, nil)
				m.EXPECT().Unstage(gomock.Any()).Return(errors.New("unstage failed"))
				return m
			},
			expectedErr: status.Errorf(codes.Internal, "Could not unmount target %q: %v", "/staging/path", errors.New("unstage failed")),
		},
		{
			name: "target_not_mounted",
			req: &csi.NodeUnstageVolumeRequest{
				VolumeId:          "vol-test",
				StagingTargetPath: "/staging/path",
			},
			mounterMock: func(ctrl *gomock.Controller) *mounter.MockMounter {
				m := mounter.NewMockMounter(ctrl)
				m.EXPECT().GetDeviceNameFromMount(gomock.Any()).Return("", 0, nil)
				return m
			},
		},
		{
			name: "get_device_name_from_mount_failed",
			req: &csi.NodeUnstageVolumeRequest{
				VolumeId:          "vol-test",
				StagingTargetPath: "/staging/path",
			},
			mounterMock: func(ctrl *gomock.Controller) *mounter.MockMounter {
				m := mounter.NewMockMounter(ctrl)
				m.EXPECT().GetDeviceNameFromMount(gomock.Any()).Return("", 0, errors.New("failed to get device name"))
				return m
			},
			expectedErr: status.Error(codes.Internal, "failed to check if target \"/staging/path\" is a mount point: failed to get device name"),
		},
		{
			name: "multiple_references",
			req: &csi.NodeUnstageVolumeRequest{
				VolumeId:          "vol-test",
				StagingTargetPath: "/staging/path",
			},
			mounterMock: func(ctrl *gomock.Controller) *mounter.MockMounter {
				m := mounter.NewMockMounter(ctrl)
				m.EXPECT().GetDeviceNameFromMount(gomock.Any()).Return("dev-test", 2, nil)
				m.EXPECT().Unstage(gomock.Any()).Return(nil)
				return m
			},
		},
		{
			name: "operation_already_exists",
			req: &csi.NodeUnstageVolumeRequest{
				VolumeId:          "vol-test",
				StagingTargetPath: "/staging/path",
			},
			expectedErr: status.Error(codes.Aborted, "An operation with the given volume=\"vol-test\" is already in progress"),
			inflight:    true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			var mounter *mounter.MockMounter
			if tc.mounterMock != nil {
				mounter = tc.mounterMock(ctrl)
			}

			driver := &NodeService{
				mounter:  mounter,
				inFlight: internal.NewInFlight(),
			}

			if tc.inflight {
				driver.inFlight.Insert("vol-test")
			}

			_, err := driver.NodeUnstageVolume(context.Background(), tc.req)
			if !reflect.DeepEqual(err, tc.expectedErr) {
				t.Fatalf("Expected error '%v' but got '%v'", tc.expectedErr, err)
			}
		})
	}
}

func TestNodeGetCapabilities(t *testing.T) {
	req := &csi.NodeGetCapabilitiesRequest{}
	expectedCaps := []*csi.NodeServiceCapability{
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

	driver := &NodeService{}

	resp, err := driver.NodeGetCapabilities(context.Background(), req)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if len(resp.GetCapabilities()) != len(expectedCaps) {
		t.Fatalf("Expected %d capabilities, but got %d", len(expectedCaps), len(resp.GetCapabilities()))
	}

	for i, cap := range resp.GetCapabilities() {
		if cap.GetRpc().GetType() != expectedCaps[i].GetRpc().GetType() {
			t.Fatalf("Expected capability %v, but got %v", expectedCaps[i].GetRpc().GetType(), cap.GetRpc().GetType())
		}
	}
}

func TestNodeGetInfo(t *testing.T) {
	testCases := []struct {
		name         string
		metadataMock func(ctrl *gomock.Controller) *metadata.MockMetadataService
		expectedResp *csi.NodeGetInfoResponse
	}{
		{
			name: "without_outpost_arn",
			metadataMock: func(ctrl *gomock.Controller) *metadata.MockMetadataService {
				m := metadata.NewMockMetadataService(ctrl)
				m.EXPECT().GetInstanceID().Return("i-1234567890abcdef0")
				m.EXPECT().GetAvailabilityZone().Return("us-west-2a")
				m.EXPECT().GetOutpostArn().Return(arn.ARN{})
				return m
			},
			expectedResp: &csi.NodeGetInfoResponse{
				NodeId: "i-1234567890abcdef0",
				AccessibleTopology: &csi.Topology{
					Segments: map[string]string{
						ZoneTopologyKey:          "us-west-2a",
						WellKnownZoneTopologyKey: "us-west-2a",
						OSTopologyKey:            runtime.GOOS,
					},
				},
			},
		},
		{
			name: "with_outpost_arn",
			metadataMock: func(ctrl *gomock.Controller) *metadata.MockMetadataService {
				m := metadata.NewMockMetadataService(ctrl)
				m.EXPECT().GetInstanceID().Return("i-1234567890abcdef0")
				m.EXPECT().GetAvailabilityZone().Return("us-west-2a")
				m.EXPECT().GetOutpostArn().Return(arn.ARN{
					Partition: "aws",
					Service:   "outposts",
					Region:    "us-west-2",
					AccountID: "123456789012",
					Resource:  "op-1234567890abcdef0",
				})
				return m
			},
			expectedResp: &csi.NodeGetInfoResponse{
				NodeId: "i-1234567890abcdef0",
				AccessibleTopology: &csi.Topology{
					Segments: map[string]string{
						ZoneTopologyKey:          "us-west-2a",
						WellKnownZoneTopologyKey: "us-west-2a",
						OSTopologyKey:            runtime.GOOS,
						AwsRegionKey:             "us-west-2",
						AwsPartitionKey:          "aws",
						AwsAccountIDKey:          "123456789012",
						AwsOutpostIDKey:          "op-1234567890abcdef0",
					},
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			metadataService := tc.metadataMock(ctrl)
			mounter := mounter.NewMockMounter(ctrl)

			driver := &NodeService{
				metadata: metadataService,
				mounter:  mounter,
				inFlight: internal.NewInFlight(),
				options:  &Options{},
			}

			resp, err := driver.NodeGetInfo(context.Background(), &csi.NodeGetInfoRequest{})
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			if !reflect.DeepEqual(resp, tc.expectedResp) {
				t.Fatalf("Expected response %+v, but got %+v", tc.expectedResp, resp)
			}
		})
	}
}

func TestNodeUnpublishVolume(t *testing.T) {
	testCases := []struct {
		name        string
		req         *csi.NodeUnpublishVolumeRequest
		mounterMock func(ctrl *gomock.Controller) *mounter.MockMounter
		expectedErr error
		inflight    bool
	}{
		{
			name: "success",
			req: &csi.NodeUnpublishVolumeRequest{
				VolumeId:   "vol-test",
				TargetPath: "/target/path",
			},
			mounterMock: func(ctrl *gomock.Controller) *mounter.MockMounter {
				m := mounter.NewMockMounter(ctrl)
				m.EXPECT().Unpublish(gomock.Eq("/target/path")).Return(nil)
				return m
			},
		},
		{
			name: "missing_volume_id",
			req: &csi.NodeUnpublishVolumeRequest{
				TargetPath: "/target/path",
			},
			expectedErr: status.Error(codes.InvalidArgument, "Volume ID not provided"),
		},
		{
			name: "missing_target_path",
			req: &csi.NodeUnpublishVolumeRequest{
				VolumeId: "vol-test",
			},
			expectedErr: status.Error(codes.InvalidArgument, "Target path not provided"),
		},
		{
			name: "unpublish_failed",
			req: &csi.NodeUnpublishVolumeRequest{
				VolumeId:   "vol-test",
				TargetPath: "/target/path",
			},
			mounterMock: func(ctrl *gomock.Controller) *mounter.MockMounter {
				m := mounter.NewMockMounter(ctrl)
				m.EXPECT().Unpublish(gomock.Eq("/target/path")).Return(errors.New("unpublish failed"))
				return m
			},
			expectedErr: status.Errorf(codes.Internal, "Could not unmount %q: %v", "/target/path", errors.New("unpublish failed")),
		},
		{
			name: "operation_already_exists",
			req: &csi.NodeUnpublishVolumeRequest{
				VolumeId:   "vol-test",
				TargetPath: "/target/path",
			},
			expectedErr: status.Error(codes.Aborted, "An operation with the given volume=\"vol-test\" is already in progress"),
			inflight:    true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			var mounter *mounter.MockMounter
			if tc.mounterMock != nil {
				mounter = tc.mounterMock(ctrl)
			}

			driver := &NodeService{
				mounter:  mounter,
				inFlight: internal.NewInFlight(),
			}

			if tc.inflight {
				driver.inFlight.Insert("vol-test")
			}

			_, err := driver.NodeUnpublishVolume(context.Background(), tc.req)
			if !reflect.DeepEqual(err, tc.expectedErr) {
				t.Fatalf("Expected error '%v' but got '%v'", tc.expectedErr, err)
			}
		})
	}
}

func TestNodeExpandVolume(t *testing.T) {
	testCases := []struct {
		name         string
		req          *csi.NodeExpandVolumeRequest
		mounterMock  func(ctrl *gomock.Controller) *mounter.MockMounter
		metadataMock func(ctrl *gomock.Controller) *metadata.MockMetadataService
		expectedResp *csi.NodeExpandVolumeResponse
		expectedErr  error
	}{
		{
			name: "success",
			req: &csi.NodeExpandVolumeRequest{
				VolumeId:   "vol-test",
				VolumePath: "/volume/path",
			},
			mounterMock: func(ctrl *gomock.Controller) *mounter.MockMounter {
				m := mounter.NewMockMounter(ctrl)
				m.EXPECT().IsBlockDevice(gomock.Eq("/volume/path")).Return(false, nil)
				m.EXPECT().GetDeviceNameFromMount(gomock.Eq("/volume/path")).Return("device-name", 1, nil)
				m.EXPECT().FindDevicePath(gomock.Eq("device-name"), gomock.Eq("vol-test"), gomock.Eq(""), gomock.Eq("us-west-2")).Return("/dev/xvdba", nil)
				m.EXPECT().Resize(gomock.Eq("/dev/xvdba"), gomock.Eq("/volume/path")).Return(true, nil)
				m.EXPECT().GetBlockSizeBytes(gomock.Eq("/dev/xvdba")).Return(int64(1000), nil)
				return m
			},
			metadataMock: func(ctrl *gomock.Controller) *metadata.MockMetadataService {
				m := metadata.NewMockMetadataService(ctrl)
				m.EXPECT().GetRegion().Return("us-west-2")
				return m
			},
			expectedResp: &csi.NodeExpandVolumeResponse{CapacityBytes: int64(1000)},
		},
		{
			name: "missing_volume_id",
			req: &csi.NodeExpandVolumeRequest{
				VolumePath: "/volume/path",
			},
			expectedErr: status.Error(codes.InvalidArgument, "Volume ID not provided"),
		},
		{
			name: "missing_volume_path",
			req: &csi.NodeExpandVolumeRequest{
				VolumeId: "vol-test",
			},
			expectedErr: status.Error(codes.InvalidArgument, "volume path must be provided"),
		},
		{
			name: "invalid_volume_capability",
			req: &csi.NodeExpandVolumeRequest{
				VolumeId:   "vol-test",
				VolumePath: "/volume/path",
				VolumeCapability: &csi.VolumeCapability{
					AccessType: &csi.VolumeCapability_Block{
						Block: &csi.VolumeCapability_BlockVolume{},
					},
					AccessMode: &csi.VolumeCapability_AccessMode{
						Mode: csi.VolumeCapability_AccessMode_UNKNOWN,
					},
				},
			},
			expectedErr: status.Error(codes.InvalidArgument, "VolumeCapability is invalid: block:{} access_mode:{}"),
		},
		{
			name: "block_device",
			req: &csi.NodeExpandVolumeRequest{
				VolumeId:   "vol-test",
				VolumePath: "/volume/path",
				VolumeCapability: &csi.VolumeCapability{
					AccessType: &csi.VolumeCapability_Block{
						Block: &csi.VolumeCapability_BlockVolume{},
					},
					AccessMode: &csi.VolumeCapability_AccessMode{
						Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER,
					},
				},
			},
			expectedResp: &csi.NodeExpandVolumeResponse{},
		},
		{
			name: "is_block_device_error",
			req: &csi.NodeExpandVolumeRequest{
				VolumeId:   "vol-test",
				VolumePath: "/volume/path",
			},
			mounterMock: func(ctrl *gomock.Controller) *mounter.MockMounter {
				m := mounter.NewMockMounter(ctrl)
				m.EXPECT().IsBlockDevice(gomock.Eq("/volume/path")).Return(false, errors.New("failed to determine if block device"))
				return m
			},
			expectedErr: status.Error(codes.Internal, "failed to determine if volumePath [/volume/path] is a block device: failed to determine if block device"),
		},
		{
			name: "get_block_size_bytes_error",
			req: &csi.NodeExpandVolumeRequest{
				VolumeId:   "vol-test",
				VolumePath: "/volume/path",
			},
			mounterMock: func(ctrl *gomock.Controller) *mounter.MockMounter {
				m := mounter.NewMockMounter(ctrl)
				m.EXPECT().IsBlockDevice(gomock.Eq("/volume/path")).Return(true, nil)
				m.EXPECT().GetBlockSizeBytes(gomock.Eq("/volume/path")).Return(int64(0), errors.New("failed to get block size"))
				return m
			},
			expectedErr: status.Error(codes.Internal, "failed to get block capacity on path /volume/path: failed to get block size"),
		},
		{
			name: "block_device_success",
			req: &csi.NodeExpandVolumeRequest{
				VolumeId:   "vol-test",
				VolumePath: "/volume/path",
			},
			mounterMock: func(ctrl *gomock.Controller) *mounter.MockMounter {
				m := mounter.NewMockMounter(ctrl)
				m.EXPECT().IsBlockDevice(gomock.Eq("/volume/path")).Return(true, nil)
				m.EXPECT().GetBlockSizeBytes(gomock.Eq("/volume/path")).Return(int64(1000), nil)
				return m
			},
			expectedResp: &csi.NodeExpandVolumeResponse{CapacityBytes: int64(1000)},
		},
		{
			name: "get_device_name_error",
			req: &csi.NodeExpandVolumeRequest{
				VolumeId:   "vol-test",
				VolumePath: "/volume/path",
			},
			mounterMock: func(ctrl *gomock.Controller) *mounter.MockMounter {
				m := mounter.NewMockMounter(ctrl)
				m.EXPECT().IsBlockDevice(gomock.Eq("/volume/path")).Return(false, nil)
				m.EXPECT().GetDeviceNameFromMount(gomock.Eq("/volume/path")).Return("", 0, errors.New("failed to get device name"))
				return m
			},
			metadataMock: nil,
			expectedResp: nil,
			expectedErr:  status.Error(codes.Internal, "failed to get device name from mount /volume/path: failed to get device name"),
		},
		{
			name: "find_device_path_error",
			req: &csi.NodeExpandVolumeRequest{
				VolumeId:   "vol-test",
				VolumePath: "/volume/path",
			},
			mounterMock: func(ctrl *gomock.Controller) *mounter.MockMounter {
				m := mounter.NewMockMounter(ctrl)
				m.EXPECT().IsBlockDevice(gomock.Eq("/volume/path")).Return(false, nil)
				m.EXPECT().GetDeviceNameFromMount(gomock.Eq("/volume/path")).Return("device-name", 1, nil)
				m.EXPECT().FindDevicePath(gomock.Eq("device-name"), gomock.Eq("vol-test"), gomock.Eq(""), gomock.Eq("us-west-2")).Return("", errors.New("failed to find device path"))
				return m
			},
			metadataMock: func(ctrl *gomock.Controller) *metadata.MockMetadataService {
				m := metadata.NewMockMetadataService(ctrl)
				m.EXPECT().GetRegion().Return("us-west-2")
				return m
			},
			expectedResp: nil,
			expectedErr:  status.Error(codes.Internal, "failed to find device path for device name device-name for mount /volume/path: failed to find device path"),
		},
		{
			name: "resize_error",
			req: &csi.NodeExpandVolumeRequest{
				VolumeId:   "vol-test",
				VolumePath: "/volume/path",
			},
			mounterMock: func(ctrl *gomock.Controller) *mounter.MockMounter {
				m := mounter.NewMockMounter(ctrl)
				m.EXPECT().IsBlockDevice(gomock.Eq("/volume/path")).Return(false, nil)
				m.EXPECT().GetDeviceNameFromMount(gomock.Eq("/volume/path")).Return("device-name", 1, nil)
				m.EXPECT().FindDevicePath(gomock.Eq("device-name"), gomock.Eq("vol-test"), gomock.Eq(""), gomock.Eq("us-west-2")).Return("/dev/xvdba", nil)
				m.EXPECT().Resize(gomock.Eq("/dev/xvdba"), gomock.Eq("/volume/path")).Return(false, errors.New("failed to resize volume"))
				return m
			},
			metadataMock: func(ctrl *gomock.Controller) *metadata.MockMetadataService {
				m := metadata.NewMockMetadataService(ctrl)
				m.EXPECT().GetRegion().Return("us-west-2")
				return m
			},
			expectedResp: nil,
			expectedErr:  status.Error(codes.Internal, "Could not resize volume \"vol-test\" (\"/dev/xvdba\"): failed to resize volume"),
		},
		{
			name: "get_block_size_bytes_error_after_resize",
			req: &csi.NodeExpandVolumeRequest{
				VolumeId:   "vol-test",
				VolumePath: "/volume/path",
			},
			mounterMock: func(ctrl *gomock.Controller) *mounter.MockMounter {
				m := mounter.NewMockMounter(ctrl)
				m.EXPECT().IsBlockDevice(gomock.Eq("/volume/path")).Return(false, nil)
				m.EXPECT().GetDeviceNameFromMount(gomock.Eq("/volume/path")).Return("device-name", 1, nil)
				m.EXPECT().FindDevicePath(gomock.Eq("device-name"), gomock.Eq("vol-test"), gomock.Eq(""), gomock.Eq("us-west-2")).Return("/dev/xvdba", nil)
				m.EXPECT().Resize(gomock.Eq("/dev/xvdba"), gomock.Eq("/volume/path")).Return(true, nil)
				m.EXPECT().GetBlockSizeBytes(gomock.Eq("/dev/xvdba")).Return(int64(0), errors.New("failed to get block size"))
				return m
			},
			metadataMock: func(ctrl *gomock.Controller) *metadata.MockMetadataService {
				m := metadata.NewMockMetadataService(ctrl)
				m.EXPECT().GetRegion().Return("us-west-2")
				return m
			},
			expectedResp: nil,
			expectedErr:  status.Error(codes.Internal, "failed to get block capacity on path /volume/path: failed to get block size"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			var mounter *mounter.MockMounter
			if tc.mounterMock != nil {
				mounter = tc.mounterMock(ctrl)
			}

			var metadata *metadata.MockMetadataService
			if tc.metadataMock != nil {
				metadata = tc.metadataMock(ctrl)
			}

			driver := &NodeService{
				mounter:  mounter,
				metadata: metadata,
			}

			resp, err := driver.NodeExpandVolume(context.Background(), tc.req)
			if !reflect.DeepEqual(err, tc.expectedErr) {
				t.Fatalf("Expected error '%v' but got '%v'", tc.expectedErr, err)
			}

			if !reflect.DeepEqual(resp, tc.expectedResp) {
				t.Fatalf("Expected response '%v' but got '%v'", tc.expectedResp, resp)
			}
		})
	}
}

func TestNodeGetVolumeStats(t *testing.T) {
	testCases := []struct {
		name           string
		validVolId     bool
		validPath      bool
		metricsStatErr bool
		mounterMock    func(mockCtl *gomock.Controller, dir string) *mounter.MockMounter
		expectedErr    func(dir string) error
	}{
		{
			name:       "success normal",
			validVolId: true,
			validPath:  true,
			mounterMock: func(ctrl *gomock.Controller, dir string) *mounter.MockMounter {
				m := mounter.NewMockMounter(ctrl)
				m.EXPECT().PathExists(dir).Return(true, nil)
				m.EXPECT().IsBlockDevice(gomock.Eq(dir)).Return(false, nil)
				return m
			},
			expectedErr: func(dir string) error {
				return nil
			},
		},
		{
			name:       "invalid_volume_id",
			validVolId: false,
			expectedErr: func(dir string) error {
				return status.Error(codes.InvalidArgument, "NodeGetVolumeStats volume ID was empty")
			},
		},
		{
			name:       "invalid_volume_path",
			validVolId: true,
			validPath:  false,
			expectedErr: func(dir string) error {
				return status.Error(codes.InvalidArgument, "NodeGetVolumeStats volume path was empty")
			},
		},
		{
			name:       "path_exists_error",
			validVolId: true,
			validPath:  true,
			mounterMock: func(ctrl *gomock.Controller, dir string) *mounter.MockMounter {
				m := mounter.NewMockMounter(ctrl)
				m.EXPECT().PathExists(dir).Return(false, errors.New("path exists error"))
				return m
			},
			expectedErr: func(dir string) error {
				return status.Errorf(codes.Internal, "unknown error when stat on %s: path exists error", dir)
			},
		},
		{
			name:       "path_does_not_exist",
			validVolId: true,
			validPath:  true,
			mounterMock: func(ctrl *gomock.Controller, dir string) *mounter.MockMounter {
				m := mounter.NewMockMounter(ctrl)
				m.EXPECT().PathExists(dir).Return(false, nil)
				return m
			},
			expectedErr: func(dir string) error {
				return status.Errorf(codes.NotFound, "path %s does not exist", dir)
			},
		},
		{
			name:       "is_block_device_error",
			validVolId: true,
			validPath:  true,
			mounterMock: func(ctrl *gomock.Controller, dir string) *mounter.MockMounter {
				m := mounter.NewMockMounter(ctrl)
				m.EXPECT().PathExists(dir).Return(true, nil)
				m.EXPECT().IsBlockDevice(gomock.Eq(dir)).Return(false, errors.New("is block device error"))
				return m
			},
			expectedErr: func(dir string) error {
				return status.Errorf(codes.Internal, "failed to determine whether %s is block device: is block device error", dir)
			},
		},
		{
			name:       "get_block_size_bytes_error",
			validVolId: true,
			validPath:  true,
			mounterMock: func(ctrl *gomock.Controller, dir string) *mounter.MockMounter {
				m := mounter.NewMockMounter(ctrl)
				m.EXPECT().PathExists(dir).Return(true, nil)
				m.EXPECT().IsBlockDevice(gomock.Eq(dir)).Return(true, nil)
				m.EXPECT().GetBlockSizeBytes(dir).Return(int64(0), errors.New("get block size bytes error"))
				return m
			},
			expectedErr: func(dir string) error {
				return status.Errorf(codes.Internal, "failed to get block capacity on path %s: %v", dir, "get block size bytes error")
			},
		},
		{
			name:       "success block device",
			validVolId: true,
			validPath:  true,
			mounterMock: func(ctrl *gomock.Controller, dir string) *mounter.MockMounter {
				m := mounter.NewMockMounter(ctrl)
				m.EXPECT().PathExists(dir).Return(true, nil)
				m.EXPECT().IsBlockDevice(gomock.Eq(dir)).Return(true, nil)
				m.EXPECT().GetBlockSizeBytes(dir).Return(int64(1024), nil)
				return m
			},
			expectedErr: func(dir string) error {
				return nil
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			dir := t.TempDir()

			var mounter *mounter.MockMounter
			if tc.mounterMock != nil {
				mounter = tc.mounterMock(ctrl, dir)
			}

			var metadata *metadata.MockMetadataService
			driver := &NodeService{
				mounter:  mounter,
				metadata: metadata,
			}

			req := &csi.NodeGetVolumeStatsRequest{}
			if tc.validVolId {
				req.VolumeId = "vol-test"
			}
			if tc.validPath {
				req.VolumePath = dir
			}
			if tc.metricsStatErr {
				req.VolumePath = "fake-path"
			}

			_, err := driver.NodeGetVolumeStats(context.TODO(), req)

			if !reflect.DeepEqual(err, tc.expectedErr(dir)) {
				t.Fatalf("Expected error '%v' but got '%v'", tc.expectedErr(dir), err)
			}
		})
	}
}

func TestRemoveNotReadyTaint(t *testing.T) {
	nodeName := "test-node-123"
	testCases := []struct {
		name      string
		setup     func(t *testing.T, mockCtl *gomock.Controller) func() (kubernetes.Interface, error)
		expResult error
	}{
		{
			name: "failed to get node",
			setup: func(t *testing.T, mockCtl *gomock.Controller) func() (kubernetes.Interface, error) {
				t.Setenv("CSI_NODE_NAME", nodeName)
				getNodeMock, _ := getNodeMock(mockCtl, nodeName, nil, fmt.Errorf("Failed to get node!"))

				return func() (kubernetes.Interface, error) {
					return getNodeMock, nil
				}
			},
			expResult: fmt.Errorf("Failed to get node!"),
		},
		{
			name: "no taints to remove",
			setup: func(t *testing.T, mockCtl *gomock.Controller) func() (kubernetes.Interface, error) {
				t.Setenv("CSI_NODE_NAME", nodeName)
				getNodeMock, _ := getNodeMock(mockCtl, nodeName, &corev1.Node{}, nil)

				storageV1Mock := NewMockStorageV1Interface(mockCtl)
				getNodeMock.(*MockKubernetesClient).EXPECT().StorageV1().Return(storageV1Mock).AnyTimes()

				csiNodesMock := NewMockCSINodeInterface(mockCtl)
				storageV1Mock.EXPECT().CSINodes().Return(csiNodesMock).Times(1)

				count := int32(1)

				mockCSINode := &v1.CSINode{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-node-123",
					},
					Spec: v1.CSINodeSpec{
						Drivers: []v1.CSINodeDriver{
							{
								Name: DriverName,
								Allocatable: &v1.VolumeNodeResources{
									Count: &count,
								},
							},
						},
					},
				}

				csiNodesMock.EXPECT().
					Get(gomock.Any(), gomock.Eq("test-node-123"), gomock.Any()).
					Return(mockCSINode, nil).
					Times(1)

				return func() (kubernetes.Interface, error) {
					return getNodeMock, nil
				}
			},
			expResult: nil,
		},
		{
			name: "failed to patch node",
			setup: func(t *testing.T, mockCtl *gomock.Controller) func() (kubernetes.Interface, error) {
				t.Setenv("CSI_NODE_NAME", nodeName)
				getNodeMock, mockNode := getNodeMock(mockCtl, nodeName, &corev1.Node{
					ObjectMeta: metav1.ObjectMeta{
						Name: nodeName,
					},
					Spec: corev1.NodeSpec{
						Taints: []corev1.Taint{
							{
								Key:    AgentNotReadyNodeTaintKey,
								Effect: corev1.TaintEffectNoExecute,
							},
						},
					},
				}, nil)

				storageV1Mock := NewMockStorageV1Interface(mockCtl)
				getNodeMock.(*MockKubernetesClient).EXPECT().StorageV1().Return(storageV1Mock).AnyTimes()

				csiNodesMock := NewMockCSINodeInterface(mockCtl)
				storageV1Mock.EXPECT().CSINodes().Return(csiNodesMock).Times(1)

				count := int32(1)
				mockCSINode := &v1.CSINode{
					ObjectMeta: metav1.ObjectMeta{
						Name: nodeName,
					},
					Spec: v1.CSINodeSpec{
						Drivers: []v1.CSINodeDriver{
							{
								Name:   DriverName,
								NodeID: nodeName,
								Allocatable: &v1.VolumeNodeResources{
									Count: &count,
								},
							},
						},
					},
				}

				csiNodesMock.EXPECT().
					Get(gomock.Any(), gomock.Eq(nodeName), gomock.Any()).
					Return(mockCSINode, nil).
					Times(1)

				mockNode.EXPECT().
					Patch(gomock.Any(), gomock.Eq(nodeName), gomock.Any(), gomock.Any(), gomock.Any()).
					Return(nil, fmt.Errorf("Failed to patch node!")).
					Times(1)

				return func() (kubernetes.Interface, error) {
					return getNodeMock, nil
				}
			},
			expResult: fmt.Errorf("Failed to patch node!"),
		},
		{
			name: "success",
			setup: func(t *testing.T, mockCtl *gomock.Controller) func() (kubernetes.Interface, error) {
				t.Setenv("CSI_NODE_NAME", nodeName)
				getNodeMock, mockNode := getNodeMock(mockCtl, nodeName, &corev1.Node{
					ObjectMeta: metav1.ObjectMeta{
						Name: nodeName,
					},
					Spec: corev1.NodeSpec{
						Taints: []corev1.Taint{
							{
								Key:    AgentNotReadyNodeTaintKey,
								Effect: corev1.TaintEffectNoSchedule,
							},
						},
					},
				}, nil)

				storageV1Mock := NewMockStorageV1Interface(mockCtl)
				getNodeMock.(*MockKubernetesClient).EXPECT().StorageV1().Return(storageV1Mock).AnyTimes()

				csiNodesMock := NewMockCSINodeInterface(mockCtl)
				storageV1Mock.EXPECT().CSINodes().Return(csiNodesMock).Times(1)

				count := int32(1)
				mockCSINode := &v1.CSINode{
					ObjectMeta: metav1.ObjectMeta{
						Name: nodeName,
					},
					Spec: v1.CSINodeSpec{
						Drivers: []v1.CSINodeDriver{
							{
								Name:   DriverName,
								NodeID: nodeName,
								Allocatable: &v1.VolumeNodeResources{
									Count: &count,
								},
							},
						},
					},
				}

				csiNodesMock.EXPECT().
					Get(gomock.Any(), gomock.Eq(nodeName), gomock.Any()).
					Return(mockCSINode, nil).
					Times(1)

				mockNode.EXPECT().
					Patch(gomock.Any(), gomock.Eq(nodeName), gomock.Any(), gomock.Any(), gomock.Any()).
					Return(nil, nil).
					Times(1)

				return func() (kubernetes.Interface, error) {
					return getNodeMock, nil
				}
			},
			expResult: nil,
		},
		{
			name: "failed to get CSINode",
			setup: func(t *testing.T, mockCtl *gomock.Controller) func() (kubernetes.Interface, error) {
				t.Setenv("CSI_NODE_NAME", nodeName)
				getNodeMock, _ := getNodeMock(mockCtl, nodeName, &corev1.Node{
					ObjectMeta: metav1.ObjectMeta{
						Name: nodeName,
					},
					Spec: corev1.NodeSpec{
						Taints: []corev1.Taint{
							{
								Key:    AgentNotReadyNodeTaintKey,
								Effect: corev1.TaintEffectNoSchedule,
							},
						},
					},
				}, nil)

				storageV1Mock := NewMockStorageV1Interface(mockCtl)
				getNodeMock.(*MockKubernetesClient).EXPECT().StorageV1().Return(storageV1Mock).AnyTimes()

				csiNodesMock := NewMockCSINodeInterface(mockCtl)
				storageV1Mock.EXPECT().CSINodes().Return(csiNodesMock).Times(1)

				csiNodesMock.EXPECT().
					Get(gomock.Any(), gomock.Eq(nodeName), gomock.Any()).
					Return(nil, fmt.Errorf("Failed to get CSINode")).
					Times(1)

				return func() (kubernetes.Interface, error) {
					return getNodeMock, nil
				}
			},
			expResult: fmt.Errorf("isAllocatableSet: failed to get CSINode for %s: Failed to get CSINode", nodeName),
		},
		{
			name: "allocatable value not set for driver on node",
			setup: func(t *testing.T, mockCtl *gomock.Controller) func() (kubernetes.Interface, error) {
				t.Setenv("CSI_NODE_NAME", nodeName)
				getNodeMock, _ := getNodeMock(mockCtl, nodeName, &corev1.Node{
					ObjectMeta: metav1.ObjectMeta{
						Name: nodeName,
					},
					Spec: corev1.NodeSpec{
						Taints: []corev1.Taint{
							{
								Key:    AgentNotReadyNodeTaintKey,
								Effect: corev1.TaintEffectNoSchedule,
							},
						},
					},
				}, nil)

				storageV1Mock := NewMockStorageV1Interface(mockCtl)
				getNodeMock.(*MockKubernetesClient).EXPECT().StorageV1().Return(storageV1Mock).AnyTimes()

				csiNodesMock := NewMockCSINodeInterface(mockCtl)
				storageV1Mock.EXPECT().CSINodes().Return(csiNodesMock).Times(1)

				mockCSINode := &v1.CSINode{
					ObjectMeta: metav1.ObjectMeta{
						Name: nodeName,
					},
					Spec: v1.CSINodeSpec{
						Drivers: []v1.CSINodeDriver{
							{
								Name:   DriverName,
								NodeID: nodeName,
							},
						},
					},
				}

				csiNodesMock.EXPECT().
					Get(gomock.Any(), gomock.Eq(nodeName), gomock.Any()).
					Return(mockCSINode, nil).
					Times(1)

				return func() (kubernetes.Interface, error) {
					return getNodeMock, nil
				}
			},
			expResult: fmt.Errorf("isAllocatableSet: allocatable value not set for driver on node %s", nodeName),
		},
		{
			name: "driver not found on node",
			setup: func(t *testing.T, mockCtl *gomock.Controller) func() (kubernetes.Interface, error) {
				t.Setenv("CSI_NODE_NAME", nodeName)
				getNodeMock, _ := getNodeMock(mockCtl, nodeName, &corev1.Node{
					ObjectMeta: metav1.ObjectMeta{
						Name: nodeName,
					},
					Spec: corev1.NodeSpec{
						Taints: []corev1.Taint{
							{
								Key:    AgentNotReadyNodeTaintKey,
								Effect: corev1.TaintEffectNoSchedule,
							},
						},
					},
				}, nil)

				storageV1Mock := NewMockStorageV1Interface(mockCtl)
				getNodeMock.(*MockKubernetesClient).EXPECT().StorageV1().Return(storageV1Mock).AnyTimes()

				csiNodesMock := NewMockCSINodeInterface(mockCtl)
				storageV1Mock.EXPECT().CSINodes().Return(csiNodesMock).Times(1)

				mockCSINode := &v1.CSINode{
					ObjectMeta: metav1.ObjectMeta{
						Name: nodeName,
					},
					Spec: v1.CSINodeSpec{},
				}

				csiNodesMock.EXPECT().
					Get(gomock.Any(), gomock.Eq(nodeName), gomock.Any()).
					Return(mockCSINode, nil).
					Times(1)

				return func() (kubernetes.Interface, error) {
					return getNodeMock, nil
				}
			},
			expResult: fmt.Errorf("isAllocatableSet: driver not found on node %s", nodeName),
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockCtl := gomock.NewController(t)
			defer mockCtl.Finish()

			k8sClientGetter := tc.setup(t, mockCtl)
			client, err := k8sClientGetter()
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}
			result := removeNotReadyTaint(client)

			if (result == nil) != (tc.expResult == nil) {
				t.Fatalf("expected %v, got %v", tc.expResult, result)
			}
			if result != nil && tc.expResult != nil {
				if result.Error() != tc.expResult.Error() {
					t.Fatalf("Expected error message `%v`, got `%v`", tc.expResult.Error(), result.Error())
				}
			}
		})
	}
}

func TestRemoveTaintInBackground(t *testing.T) {
	t.Run("Successful taint removal", func(t *testing.T) {
		mockRemovalCount := 0
		mockRemovalFunc := func(_ kubernetes.Interface) error {
			mockRemovalCount += 1
			if mockRemovalCount == 3 {
				return nil
			} else {
				return fmt.Errorf("Taint removal failed!")
			}
		}
		removeTaintInBackground(nil, taintRemovalBackoff, mockRemovalFunc)
		assert.Equal(t, 3, mockRemovalCount)
	})

	t.Run("Retries exhausted", func(t *testing.T) {
		mockRemovalCount := 0
		mockRemovalFunc := func(_ kubernetes.Interface) error {
			mockRemovalCount += 1
			return fmt.Errorf("Taint removal failed!")
		}
		removeTaintInBackground(nil, wait.Backoff{
			Steps:    5,
			Duration: 1 * time.Millisecond,
		}, mockRemovalFunc)
		assert.Equal(t, 5, mockRemovalCount)
	})
}

func getNodeMock(mockCtl *gomock.Controller, nodeName string, returnNode *corev1.Node, returnError error) (kubernetes.Interface, *MockNodeInterface) {
	mockClient := NewMockKubernetesClient(mockCtl)
	mockCoreV1 := NewMockCoreV1Interface(mockCtl)
	mockNode := NewMockNodeInterface(mockCtl)

	mockClient.EXPECT().CoreV1().Return(mockCoreV1).MinTimes(1)
	mockCoreV1.EXPECT().Nodes().Return(mockNode).MinTimes(1)
	mockNode.EXPECT().Get(gomock.Any(), gomock.Eq(nodeName), gomock.Any()).Return(returnNode, returnError).MinTimes(1)

	return mockClient, mockNode
}
