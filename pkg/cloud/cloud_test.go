package cloud

import (
	"testing"

	"github.com/bertinatto/ebs-csi-driver/pkg/util"
)

func TestCreateDisk(t *testing.T) {
	testCases := []struct {
		name        string
		volumeName  string
		diskOptions *DiskOptions
		expDisk     *Disk
		expErr      error
	}{
		{
			name:       "success: normal",
			volumeName: "vol-test",
			diskOptions: &DiskOptions{
				CapacityBytes: util.GiBToBytes(1),
				Tags:          map[string]string{VolumeNameTagKey: "vol-test"},
			},
			expDisk: &Disk{
				VolumeID:    "vol-test",
				CapacityGiB: 1,
			},
			expErr: nil,
		},
	}

	for _, tc := range testCases {
		t.Logf("Test case: %s", tc.name)
		c := &Cloud{
			metadata: &Metadata{
				InstanceID:       "test-instance",
				Region:           "test-region",
				AvailabilityZone: "test-az",
			},
			ec2: &fakeEC2{},
		}

		disk, err := c.CreateDisk(tc.volumeName, tc.diskOptions)
		if err != nil {
			if tc.expErr == nil {
				t.Fatalf("Expected no error, got: %v", err)
			}
		}

		if disk == nil && tc.expDisk != nil {
			t.Fatalf("Expected valid disk, got nil")
		}

		if disk.VolumeID != tc.expDisk.VolumeID {
			t.Fatalf("Expected volume ID %q, got %q", tc.expDisk.VolumeID, disk.VolumeID)
		}

		if disk.CapacityGiB != tc.expDisk.CapacityGiB {
			t.Fatalf("Expected capacity %q, got %q", tc.expDisk.CapacityGiB, disk.CapacityGiB)
		}
	}
}
