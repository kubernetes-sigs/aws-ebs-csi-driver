package internal

import (
	"testing"

	csi "github.com/container-storage-interface/spec/lib/go/csi"
)

func TestInFlight(t *testing.T) {
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
	stdParams := map[string]string{
		"fsType":     "ext3",
		"volumeType": "gp2",
	}

	testCases := []struct {
		delete  bool
		expResp bool
		name    string
		req     *csi.CreateVolumeRequest
	}{
		{
			name:    "success normal",
			delete:  false,
			expResp: true,
			req: &csi.CreateVolumeRequest{
				Name:               "random-vol-name",
				CapacityRange:      stdCapRange,
				VolumeCapabilities: stdVolCap,
				Parameters:         stdParams,
			},
		},
		{
			name:    "success adding copy of request",
			delete:  false,
			expResp: false,
			req: &csi.CreateVolumeRequest{
				Name:               "random-vol-name",
				CapacityRange:      stdCapRange,
				VolumeCapabilities: stdVolCap,
				Parameters:         stdParams,
			},
		},
		{
			name:    "success adding request with different name",
			delete:  false,
			expResp: true,
			req: &csi.CreateVolumeRequest{
				Name:               "random-vol-name-foobar",
				CapacityRange:      stdCapRange,
				VolumeCapabilities: stdVolCap,
				Parameters:         stdParams,
			},
		},
		{
			name:    "success adding request with different parameters",
			delete:  false,
			expResp: true,
			req: &csi.CreateVolumeRequest{
				Name:               "random-vol-name-foobar",
				CapacityRange:      stdCapRange,
				VolumeCapabilities: stdVolCap,
				Parameters:         map[string]string{"foo": "bar"},
			},
		},
		{
			name:    "success after deleting request",
			delete:  true,
			expResp: true,
			req: &csi.CreateVolumeRequest{
				Name:               "random-vol-name",
				CapacityRange:      stdCapRange,
				VolumeCapabilities: stdVolCap,
				Parameters:         stdParams,
			},
		},
	}

	db := NewInFlight()

	for _, tc := range testCases {
		t.Logf("Test case: %s", tc.name)

		if tc.delete {
			db.Delete(tc.req)
		}

		resp := db.Upsert(tc.req)
		if tc.expResp != resp {
			t.Fatalf("expected upsert to be %+v, got %+v", tc.expResp, resp)
		}
	}
}
