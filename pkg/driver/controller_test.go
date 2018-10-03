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
	"testing"

	csi "github.com/container-storage-interface/spec/lib/go/csi/v0"
	"github.com/kubernetes-sigs/ebs-csi-driver/pkg/cloud"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
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
				Id:            "vol-test",
				Attributes:    nil,
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
				Id:            "vol-test",
				Attributes:    nil,
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
				Id:            "vol-test",
				Attributes:    nil,
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
				Id:            "vol-test",
				Attributes:    nil,
			},
		},
	}

	for _, tc := range testCases {
		t.Logf("Test case: %s", tc.name)
		awsDriver := NewDriver(cloud.NewFakeCloudProvider(), NewFakeMounter(), "")

		resp, err := awsDriver.CreateVolume(context.TODO(), tc.req)
		if err != nil {
			srvErr, ok := status.FromError(err)
			if !ok {
				t.Fatalf("Could not get error status code from error: %v", srvErr)
			}
			if srvErr.Code() != tc.expErrCode {
				t.Fatalf("Expected error code %d, got %d", tc.expErrCode, srvErr.Code())
			}
			continue
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
				continue
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

		for expKey, expVal := range tc.expVol.GetAttributes() {
			attrs := vol.GetAttributes()
			if gotVal, ok := attrs[expKey]; !ok || gotVal != expVal {
				t.Fatalf("Expected volume attribute for key %v: %v, got: %v", expKey, expVal, gotVal)
			}
		}
		if tc.expVol.GetAttributes() == nil && vol.GetAttributes() != nil {
			t.Fatalf("Expected volume attributes to be nil, got: %#v", vol.GetAttributes())
		}
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
		t.Logf("Test case: %s", tc.name)
		awsDriver := NewDriver(cloud.NewFakeCloudProvider(), NewFakeMounter(), "")
		_, err := awsDriver.DeleteVolume(context.TODO(), tc.req)
		if err != nil {
			srvErr, ok := status.FromError(err)
			if !ok {
				t.Fatalf("Could not get error status code from error: %v", srvErr)
			}
			if srvErr.Code() != tc.expErrCode {
				t.Fatalf("Expected error code %d, got %d", tc.expErrCode, srvErr.Code())
			}
			continue
		}
		if tc.expErrCode != codes.OK {
			t.Fatalf("Expected error %v, got no error", tc.expErrCode)
		}
	}
}
