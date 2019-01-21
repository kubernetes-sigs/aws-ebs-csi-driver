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
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"k8s.io/kubernetes/pkg/util/mount"
)

func TestNodeStageVolume(t *testing.T) {
	stdVolCap := &csi.VolumeCapability{
		AccessType: &csi.VolumeCapability_Mount{
			Mount: &csi.VolumeCapability_MountVolume{},
		},
		AccessMode: &csi.VolumeCapability_AccessMode{
			Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER,
		},
	}
	testCases := []struct {
		name string
		req  *csi.NodeStageVolumeRequest
		// expected fake mount actions the test will make
		expActions []mount.FakeAction
		// expected test error code
		expErrCode codes.Code
		// expected mount points when test finishes
		expMountPoints []mount.MountPoint
		// setup this mount point before running the test
		fakeMountPoint *mount.MountPoint
	}{
		{
			name: "success normal",
			req: &csi.NodeStageVolumeRequest{
				PublishContext:    map[string]string{"devicePath": "/dev/fake"},
				StagingTargetPath: "/test/path",
				VolumeCapability:  stdVolCap,
				VolumeId:          "vol-test",
			},
			expActions: []mount.FakeAction{
				{
					Action: "mount",
					Target: "/test/path",
					Source: "/dev/fake",
					FSType: defaultFsType,
				},
			},
			expMountPoints: []mount.MountPoint{
				{
					Device: "/dev/fake",
					Opts:   []string{"defaults"},
					Path:   "/test/path",
					Type:   defaultFsType,
				},
			},
		},
		{
			name: "success mount options fsType ext3",
			req: &csi.NodeStageVolumeRequest{
				PublishContext:    map[string]string{"devicePath": "/dev/fake"},
				StagingTargetPath: "/test/path",
				VolumeCapability:  stdVolCap,
				VolumeContext:     map[string]string{"fsType": FSTypeExt3},
				VolumeId:          "vol-test",
			},
			expActions: []mount.FakeAction{
				{
					Action: "mount",
					Target: "/test/path",
					Source: "/dev/fake",
					FSType: FSTypeExt3,
				},
			},
			expMountPoints: []mount.MountPoint{
				{
					Device: "/dev/fake",
					Opts:   []string{"defaults"},
					Path:   "/test/path",
					Type:   FSTypeExt3,
				},
			},
		},
		{
			name: "fail no VolumeId",
			req: &csi.NodeStageVolumeRequest{
				PublishContext:    map[string]string{"devicePath": "/dev/fake"},
				StagingTargetPath: "/test/path",
				VolumeCapability:  stdVolCap,
			},
			expErrCode: codes.InvalidArgument,
		},
		{
			name: "fail no StagingTargetPath",
			req: &csi.NodeStageVolumeRequest{
				PublishContext:   map[string]string{"devicePath": "/dev/fake"},
				VolumeCapability: stdVolCap,
				VolumeId:         "vol-test",
			},
			expErrCode: codes.InvalidArgument,
		},
		{
			name: "fail no VolumeCapability",
			req: &csi.NodeStageVolumeRequest{
				PublishContext:    map[string]string{"devicePath": "/dev/fake"},
				StagingTargetPath: "/test/path",
				VolumeId:          "vol-test",
			},
			expErrCode: codes.InvalidArgument,
		},
		{
			name: "fail invalid VolumeCapability",
			req: &csi.NodeStageVolumeRequest{
				PublishContext:    map[string]string{"devicePath": "/dev/fake"},
				StagingTargetPath: "/test/path",
				VolumeCapability: &csi.VolumeCapability{
					AccessMode: &csi.VolumeCapability_AccessMode{
						Mode: csi.VolumeCapability_AccessMode_UNKNOWN,
					},
				},
				VolumeId: "vol-test",
			},
			expErrCode: codes.InvalidArgument,
		},
		{
			name: "fail no devicePath",
			req: &csi.NodeStageVolumeRequest{
				StagingTargetPath: "/test/path",
				VolumeCapability:  stdVolCap,
				VolumeId:          "vol-test",
			},
			expErrCode: codes.InvalidArgument,
		},
		{
			// To test idempotency we need to test the
			// volume corresponding to the volume_id is
			// already staged to the staging_target_path
			// and the Plugin replied with OK. To achieve
			// this we setup the fake mounter to return
			// that /dev/fake is mounted at /test/path.
			name: "success device already mounted at target",
			req: &csi.NodeStageVolumeRequest{
				PublishContext:    map[string]string{"devicePath": "/dev/fake"},
				StagingTargetPath: "/test/path",
				VolumeCapability:  stdVolCap,
				VolumeId:          "vol-test",
			},
			fakeMountPoint: &mount.MountPoint{
				Device: "/dev/fake",
				Path:   "/test/path",
			},
			// no actions means mount isn't called because
			// device is already mounted
			expActions: []mount.FakeAction{},
			// expMountPoints should contain only the
			// fakeMountPoint
			expMountPoints: []mount.MountPoint{
				{
					Device: "/dev/fake",
					Path:   "/test/path",
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			fakeMounter := NewFakeMounter()
			if tc.fakeMountPoint != nil {
				fakeMounter.MountPoints = append(fakeMounter.MountPoints, *tc.fakeMountPoint)
			}
			awsDriver := NewFakeDriver("", fakeMounter)

			_, err := awsDriver.NodeStageVolume(context.TODO(), tc.req)
			if err != nil {
				srvErr, ok := status.FromError(err)
				if !ok {
					t.Fatalf("Could not get error status code from error: %v", srvErr)
				}
				if srvErr.Code() != tc.expErrCode {
					t.Fatalf("Expected error code %d, got %d message %s", tc.expErrCode, srvErr.Code(), srvErr.Message())
				}
			} else if tc.expErrCode != codes.OK {
				t.Fatalf("Expected error %v, got no error", tc.expErrCode)
			}

			// if fake mounter did anything we should
			// check if it was expected
			if len(fakeMounter.Log) > 0 && !reflect.DeepEqual(fakeMounter.Log, tc.expActions) {
				t.Fatalf("Expected actions {%+v}, got {%+v}", tc.expActions, fakeMounter.Log)
			}
			if len(fakeMounter.MountPoints) > 0 && !reflect.DeepEqual(fakeMounter.MountPoints, tc.expMountPoints) {
				t.Fatalf("Expected mount points {%+v}, got {%+v}", tc.expMountPoints, fakeMounter.MountPoints)
			}
		})
	}
}

func TestNodeUnstageVolume(t *testing.T) {
	testCases := []struct {
		name            string
		req             *csi.NodeUnstageVolumeRequest
		expErrCode      codes.Code
		fakeMountPoints []mount.MountPoint
		// expected fake mount actions the test will make
		expActions []mount.FakeAction
		// expected mount points when test finishes
		expMountPoints []mount.MountPoint
	}{
		{
			name: "success normal",
			req: &csi.NodeUnstageVolumeRequest{
				StagingTargetPath: "/test/path",
				VolumeId:          "vol-test",
			},
			fakeMountPoints: []mount.MountPoint{
				{Device: "/dev/fake", Path: "/test/path"},
			},
			expActions: []mount.FakeAction{
				{Action: "unmount", Target: "/test/path"},
			},
		},
		{
			name: "success no device mounted at target",
			req: &csi.NodeUnstageVolumeRequest{
				StagingTargetPath: "/test/path",
				VolumeId:          "vol-test",
			},
			expActions: []mount.FakeAction{},
		},
		{
			name: "success device mounted at multiple targets",
			req: &csi.NodeUnstageVolumeRequest{
				StagingTargetPath: "/test/path",
				VolumeId:          "vol-test",
			},
			// mount a fake device in two locations
			fakeMountPoints: []mount.MountPoint{
				{Device: "/dev/fake", Path: "/test/path"},
				{Device: "/dev/fake", Path: "/foo/bar"},
			},
			// it should unmount from the original
			expActions: []mount.FakeAction{
				{Action: "unmount", Target: "/test/path"},
			},
			expMountPoints: []mount.MountPoint{
				{Device: "/dev/fake", Path: "/foo/bar"},
			},
		},
		{
			name: "fail no VolumeId",
			req: &csi.NodeUnstageVolumeRequest{
				StagingTargetPath: "/test/path",
			},
			expErrCode: codes.InvalidArgument,
		},
		{
			name: "fail no StagingTargetPath",
			req: &csi.NodeUnstageVolumeRequest{
				VolumeId: "vol-test",
			},
			expErrCode: codes.InvalidArgument,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			fakeMounter := NewFakeMounter()
			if len(tc.fakeMountPoints) > 0 {
				fakeMounter.MountPoints = tc.fakeMountPoints
			}
			awsDriver := NewFakeDriver("", fakeMounter)

			_, err := awsDriver.NodeUnstageVolume(context.TODO(), tc.req)
			if err != nil {
				srvErr, ok := status.FromError(err)
				if !ok {
					t.Fatalf("Could not get error status code from error: %v", srvErr)
				}
				if srvErr.Code() != tc.expErrCode {
					t.Fatalf("Expected error code %d, got %d message %s", tc.expErrCode, srvErr.Code(), srvErr.Message())
				}
			} else if tc.expErrCode != codes.OK {
				t.Fatalf("Expected error %v, got no error", tc.expErrCode)
			}
			// if fake mounter did anything we should
			// check if it was expected
			if len(fakeMounter.Log) > 0 && !reflect.DeepEqual(fakeMounter.Log, tc.expActions) {
				t.Fatalf("Expected actions {%+v}, got {%+v}", tc.expActions, fakeMounter.Log)
			}
			if len(fakeMounter.MountPoints) > 0 && !reflect.DeepEqual(fakeMounter.MountPoints, tc.expMountPoints) {
				t.Fatalf("Expected mount points {%+v}, got {%+v}", tc.expMountPoints, fakeMounter.MountPoints)
			}
		})
	}
}

func TestNodePublishVolume(t *testing.T) {
	stdVolCap := &csi.VolumeCapability{
		AccessType: &csi.VolumeCapability_Mount{
			Mount: &csi.VolumeCapability_MountVolume{},
		},
		AccessMode: &csi.VolumeCapability_AccessMode{
			Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER,
		},
	}
	testCases := []struct {
		name string
		req  *csi.NodePublishVolumeRequest
		// expect these actions to have occured
		expActions []mount.FakeAction
		// expected test error code
		expErrCode codes.Code
		// expect these mount points to be setup
		expMountPoints []mount.MountPoint
	}{
		{
			name: "success normal",
			req: &csi.NodePublishVolumeRequest{
				PublishContext:    map[string]string{"devicePath": "/dev/fake"},
				StagingTargetPath: "/test/staging/path",
				TargetPath:        "/test/target/path",
				VolumeCapability:  stdVolCap,
				VolumeId:          "vol-test",
			},
			expActions: []mount.FakeAction{
				{
					Action: "mount",
					FSType: defaultFsType,
					Source: "/test/staging/path",
					Target: "/test/target/path",
				},
			},
			expMountPoints: []mount.MountPoint{
				{
					Device: "/test/staging/path",
					Opts:   []string{"bind"},
					Path:   "/test/target/path",
					Type:   defaultFsType,
				},
			},
		},
		{
			name: "success readonly",
			req: &csi.NodePublishVolumeRequest{
				PublishContext:    map[string]string{"devicePath": "/dev/fake"},
				Readonly:          true,
				StagingTargetPath: "/test/staging/path",
				TargetPath:        "/test/target/path",
				VolumeCapability:  stdVolCap,
				VolumeId:          "vol-test",
			},
			expActions: []mount.FakeAction{
				{
					Action: "mount",
					FSType: defaultFsType,
					Source: "/test/staging/path",
					Target: "/test/target/path",
				},
			},
			expMountPoints: []mount.MountPoint{
				{
					Device: "/test/staging/path",
					Opts:   []string{"bind", "ro"},
					Path:   "/test/target/path",
					Type:   defaultFsType,
				},
			},
		},
		{
			name: "success mount options",
			req: &csi.NodePublishVolumeRequest{
				PublishContext:    map[string]string{"devicePath": "/dev/fake"},
				StagingTargetPath: "/test/staging/path",
				TargetPath:        "/test/target/path",
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
			},
			expActions: []mount.FakeAction{
				{
					Action: "mount",
					FSType: defaultFsType,
					Source: "/test/staging/path",
					Target: "/test/target/path",
				},
			},
			expMountPoints: []mount.MountPoint{
				{
					Device: "/test/staging/path",
					Opts:   []string{"bind", "test-flag"},
					Path:   "/test/target/path",
					Type:   defaultFsType,
				},
			},
		},
		{
			name: "fail no VolumeId",
			req: &csi.NodePublishVolumeRequest{
				PublishContext:    map[string]string{"devicePath": "/dev/fake"},
				StagingTargetPath: "/test/staging/path",
				TargetPath:        "/test/target/path",
				VolumeCapability:  stdVolCap,
			},
			expErrCode: codes.InvalidArgument,
		},
		{
			name: "fail no StagingTargetPath",
			req: &csi.NodePublishVolumeRequest{
				PublishContext:   map[string]string{"devicePath": "/dev/fake"},
				TargetPath:       "/test/target/path",
				VolumeCapability: stdVolCap,
				VolumeId:         "vol-test",
			},
			expErrCode: codes.InvalidArgument,
		},
		{
			name: "fail no TargetPath",
			req: &csi.NodePublishVolumeRequest{
				PublishContext:    map[string]string{"devicePath": "/dev/fake"},
				StagingTargetPath: "/test/staging/path",
				VolumeCapability:  stdVolCap,
				VolumeId:          "vol-test",
			},
			expErrCode: codes.InvalidArgument,
		},
		{
			name: "fail no VolumeCapability",
			req: &csi.NodePublishVolumeRequest{
				PublishContext:    map[string]string{"devicePath": "/dev/fake"},
				StagingTargetPath: "/test/staging/path",
				TargetPath:        "/test/target/path",
				VolumeId:          "vol-test",
			},
			expErrCode: codes.InvalidArgument,
		},
		{
			name: "fail invalid VolumeCapability",
			req: &csi.NodePublishVolumeRequest{
				PublishContext:    map[string]string{"devicePath": "/dev/fake"},
				StagingTargetPath: "/test/staging/path",
				TargetPath:        "/test/target/path",
				VolumeId:          "vol-test",
				VolumeCapability: &csi.VolumeCapability{
					AccessMode: &csi.VolumeCapability_AccessMode{
						Mode: csi.VolumeCapability_AccessMode_UNKNOWN,
					},
				},
			},
			expErrCode: codes.InvalidArgument,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			fakeMounter := NewFakeMounter()
			awsDriver := NewFakeDriver("", fakeMounter)

			_, err := awsDriver.NodePublishVolume(context.TODO(), tc.req)
			if err != nil {
				srvErr, ok := status.FromError(err)
				if !ok {
					t.Fatalf("Could not get error status code from error: %v", srvErr)
				}
				if srvErr.Code() != tc.expErrCode {
					t.Fatalf("Expected error code %d, got %d message %s", tc.expErrCode, srvErr.Code(), srvErr.Message())
				}
			} else if tc.expErrCode != codes.OK {
				t.Fatalf("Expected error %v and got no error", tc.expErrCode)
			}

			// if fake mounter did anything we should
			// check if it was expected
			if len(fakeMounter.Log) > 0 && !reflect.DeepEqual(fakeMounter.Log, tc.expActions) {
				t.Fatalf("Expected actions {%+v}, got {%+v}", tc.expActions, fakeMounter.Log)
			}
			if len(fakeMounter.MountPoints) > 0 && !reflect.DeepEqual(fakeMounter.MountPoints, tc.expMountPoints) {
				t.Fatalf("Expected mount points {%+v}, got {%+v}", tc.expMountPoints, fakeMounter.MountPoints)
			}
		})
	}
}

func TestNodeUnpublishVolume(t *testing.T) {
	testCases := []struct {
		name string
		req  *csi.NodeUnpublishVolumeRequest
		// expected fake mount actions the test will make
		expActions []mount.FakeAction
		// expected test error code
		expErrCode codes.Code
		// setup this mount point before running the test
		fakeMountPoint *mount.MountPoint
	}{
		{
			name: "success normal",
			req: &csi.NodeUnpublishVolumeRequest{
				TargetPath: "/test/path",
				VolumeId:   "vol-test",
			},
			fakeMountPoint: &mount.MountPoint{
				Device: "/dev/fake",
				Path:   "/test/path",
			},
			expActions: []mount.FakeAction{
				{
					Action: "unmount",
					Target: "/test/path",
				},
			},
		},
		{
			name: "fail no VolumeId",
			req: &csi.NodeUnpublishVolumeRequest{
				TargetPath: "/test/path",
			},
			expErrCode: codes.InvalidArgument,
		},
		{
			name: "fail no TargetPath",
			req: &csi.NodeUnpublishVolumeRequest{
				VolumeId: "vol-test",
			},
			expErrCode: codes.InvalidArgument,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			fakeMounter := NewFakeMounter()
			if tc.fakeMountPoint != nil {
				fakeMounter.MountPoints = append(fakeMounter.MountPoints, *tc.fakeMountPoint)
			}
			awsDriver := NewFakeDriver("", fakeMounter)

			_, err := awsDriver.NodeUnpublishVolume(context.TODO(), tc.req)
			if err != nil {
				srvErr, ok := status.FromError(err)
				if !ok {
					t.Fatalf("Could not get error status code from error: %v", srvErr)
				}
				if srvErr.Code() != tc.expErrCode {
					t.Fatalf("Expected error code %d, got %d message %s", tc.expErrCode, srvErr.Code(), srvErr.Message())
				}
			} else if tc.expErrCode != codes.OK {
				t.Fatalf("Expected error %v, got no error", tc.expErrCode)
			}

			// if fake mounter did anything we should
			// check if it was expected
			if len(fakeMounter.Log) > 0 && !reflect.DeepEqual(fakeMounter.Log, tc.expActions) {
				t.Fatalf("Expected actions {%+v}, got {%+v}", tc.expActions, fakeMounter.Log)
			}
		})
	}
}

func TestNodeGetVolumeStats(t *testing.T) {
	req := &csi.NodeGetVolumeStatsRequest{}
	awsDriver := NewFakeDriver("", NewFakeMounter())
	expErrCode := codes.Unimplemented

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
	req := &csi.NodeGetCapabilitiesRequest{}
	awsDriver := NewFakeDriver("", NewFakeMounter())
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
	req := &csi.NodeGetInfoRequest{}
	awsDriver := NewFakeDriver("", NewFakeMounter())
	m := awsDriver.cloud.GetMetadata()
	expResp := &csi.NodeGetInfoResponse{
		NodeId: "instanceID",
		AccessibleTopology: &csi.Topology{
			Segments: map[string]string{topologyKey: m.GetAvailabilityZone()},
		},
	}

	resp, err := awsDriver.NodeGetInfo(context.TODO(), req)
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
