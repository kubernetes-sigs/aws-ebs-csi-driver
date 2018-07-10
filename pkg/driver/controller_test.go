package driver

import (
	"context"
	"testing"

	"github.com/bertinatto/ebs-csi-driver/pkg/cloud"
	csi "github.com/container-storage-interface/spec/lib/go/csi/v0"
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
	stdVolSize := int64(5000000000)
	stdCapRange := &csi.CapacityRange{RequiredBytes: stdVolSize}
	stdParams := map[string]string{}

	testCases := []struct {
		name       string
		req        *csi.CreateVolumeRequest
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
			name: "success no capacity range",
			req: &csi.CreateVolumeRequest{
				Name:               "test-vol",
				VolumeCapabilities: stdVolCap,
				Parameters:         stdParams,
			},
			expVol: &csi.Volume{
				CapacityBytes: DefaultVolumeSize,
				Id:            "vol-test",
				Attributes:    nil,
			},
		},
	}

	for _, tc := range testCases {
		t.Logf("Test case: %s", tc.name)
		awsDriver := NewDriver(cloud.NewFakeCloudProvider(), "", "")

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

		//if vol.GetId() != tc.expVol.GetId() {
		//t.Fatalf("Expected volume id: %v, got: %v", tc.expVol.GetId(), vol.GetId())
		//}

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
		expVol     *csi.Volume
		expErrCode codes.Code
	}{
		{
			name: "fail volume not found",
			req: &csi.DeleteVolumeRequest{
				VolumeId: "vol-not-found",
			},
			expErrCode: codes.NotFound,
		},
	}

	for _, tc := range testCases {
		t.Logf("Test case: %s", tc.name)
		awsDriver := NewDriver(cloud.NewFakeCloudProvider(), "", "")
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
