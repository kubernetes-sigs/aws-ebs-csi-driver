//go:build linux
// +build linux

package sanity

import (
	"context"
	"fmt"
	"math/rand"
	"os"
	"path"
	"strconv"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	csisanity "github.com/kubernetes-csi/csi-test/v4/pkg/sanity"
	"github.com/kubernetes-sigs/aws-ebs-csi-driver/pkg/cloud"
	d "github.com/kubernetes-sigs/aws-ebs-csi-driver/pkg/driver"
	"github.com/kubernetes-sigs/aws-ebs-csi-driver/pkg/util"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

const (
	instanceId = "i-1234567890abcdef0"
	region     = "us-west-2"
)

var (
	disks            = make(map[string]*cloud.Disk)
	snapshots        = make(map[string]*cloud.Snapshot)
	snapshotNameToID = make(map[string]string)

	fakeMetaData = &cloud.Metadata{
		InstanceID: instanceId,
		Region:     region,
	}
)

func TestSanity(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("Test panicked: %v", r)
		}
	}()

	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	tmpDir, err := os.MkdirTemp("", "csi-sanity-")
	if err != nil {
		t.Fatalf("Failed to create sanity temp working dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	defer func() {
		if err = os.RemoveAll(tmpDir); err != nil {
			t.Fatalf("Failed to clean up sanity temp working dir %s: %v", tmpDir, err.Error())
		}
	}()

	endpoint := fmt.Sprintf("unix:%s/csi.sock", tmpDir)
	mountPath := path.Join(tmpDir, "mount")
	stagePath := path.Join(tmpDir, "stage")

	fakeMounter, fakeIdentifier, fakeResizeFs, fakeCloud := createMockObjects(mockCtrl)

	mockNodeService(fakeMounter, fakeIdentifier, fakeResizeFs, mountPath)
	mockControllerService(fakeCloud, mountPath)

	drv, err := d.NewFakeDriver(endpoint, fakeCloud, fakeMetaData, fakeMounter)
	if err != nil {
		t.Fatalf("Failed to create fake driver: %v", err.Error())
	}
	go func() {
		if err := drv.Run(); err != nil {
			panic(fmt.Sprintf("%v", err))
		}
	}()

	config := csisanity.TestConfig{
		TargetPath:           mountPath,
		StagingPath:          stagePath,
		Address:              endpoint,
		DialOptions:          []grpc.DialOption{grpc.WithTransportCredentials(insecure.NewCredentials())},
		IDGen:                csisanity.DefaultIDGenerator{},
		TestVolumeSize:       10 * util.GiB,
		TestVolumeAccessType: "mount",
	}
	csisanity.Test(t, config)
}

func createMockObjects(mockCtrl *gomock.Controller) (*d.MockMounter, *d.MockDeviceIdentifier, *d.MockResizefs, *cloud.MockCloud) {
	fakeMounter := d.NewMockMounter(mockCtrl)
	fakeIdentifier := d.NewMockDeviceIdentifier(mockCtrl)
	fakeResizeFs := d.NewMockResizefs(mockCtrl)
	fakeCloud := cloud.NewMockCloud(mockCtrl)

	return fakeMounter, fakeIdentifier, fakeResizeFs, fakeCloud
}

func mockNodeService(m *d.MockMounter, i *d.MockDeviceIdentifier, r *d.MockResizefs, mountPath string) {
	m.EXPECT().Unpublish(gomock.Any()).Return(nil).AnyTimes()
	m.EXPECT().GetDeviceNameFromMount(gomock.Any()).Return("", 0, nil).AnyTimes()
	m.EXPECT().Unstage(gomock.Any()).Return(nil).AnyTimes()
	m.EXPECT().FormatAndMountSensitiveWithFormatOptions(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	m.EXPECT().NeedResize(gomock.Any(), gomock.Any()).Return(false, nil).AnyTimes()
	m.EXPECT().MakeDir(gomock.Any()).Return(nil).AnyTimes()
	m.EXPECT().IsLikelyNotMountPoint(gomock.Any()).Return(true, nil).AnyTimes()
	m.EXPECT().Mount(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()

	m.EXPECT().PathExists(gomock.Any()).DoAndReturn(
		func(targetPath string) (bool, error) {
			if targetPath == mountPath {
				return true, nil
			}
			return false, nil
		},
	).AnyTimes()

	m.EXPECT().NewResizeFs().Return(r, nil).AnyTimes()
	i.EXPECT().Lstat(gomock.Any()).Return(nil, nil).AnyTimes()
	r.EXPECT().Resize(gomock.Any(), gomock.Any()).Return(true, nil).AnyTimes()
}

func mockControllerService(c *cloud.MockCloud, mountPath string) {
	// CreateDisk
	c.EXPECT().CreateDisk(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, volumeID string, diskOptions *cloud.DiskOptions) (*cloud.Disk, error) {
			for _, existingDisk := range disks {
				if existingDisk.VolumeID == volumeID && existingDisk.CapacityGiB != util.BytesToGiB(diskOptions.CapacityBytes) {
					return nil, cloud.ErrAlreadyExists
				}
			}

			if diskOptions.SnapshotID != "" {
				if _, exists := snapshots[diskOptions.SnapshotID]; !exists {
					return nil, cloud.ErrNotFound
				}
			}

			newDisk := &cloud.Disk{
				VolumeID:         volumeID,
				AvailabilityZone: diskOptions.AvailabilityZone,
				CapacityGiB:      util.BytesToGiB(diskOptions.CapacityBytes),
			}
			disks[volumeID] = newDisk
			return newDisk, nil
		},
	).AnyTimes()

	// DeleteDisk
	c.EXPECT().DeleteDisk(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, volumeID string) (bool, error) {
			_, exists := disks[volumeID]
			if !exists {
				return false, cloud.ErrNotFound
			}
			delete(disks, volumeID)
			return true, nil
		},
	).AnyTimes()

	// GetDiskByID
	c.EXPECT().GetDiskByID(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, volumeID string) (*cloud.Disk, error) {
			disk, exists := disks[volumeID]
			if !exists {
				return nil, cloud.ErrNotFound
			}
			return disk, nil
		},
	).AnyTimes()

	// CreateSnapshot
	c.EXPECT().
		CreateSnapshot(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, volumeID string, opts *cloud.SnapshotOptions) (*cloud.Snapshot, error) {
			snapshotID := fmt.Sprintf("snapshot-%d", rand.New(rand.NewSource(time.Now().UnixNano())).Uint64())

			_, exists := snapshots[snapshotID]
			if exists {
				return nil, cloud.ErrAlreadyExists
			}
			newSnapshot := &cloud.Snapshot{
				SnapshotID:     snapshotID,
				SourceVolumeID: volumeID,
				CreationTime:   time.Now(),
			}
			snapshots[snapshotID] = newSnapshot
			snapshotNameToID[opts.Tags["CSIVolumeSnapshotName"]] = snapshotID
			return newSnapshot, nil
		}).AnyTimes()

	// DeleteSnapshot
	c.EXPECT().DeleteSnapshot(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, snapshotID string) (bool, error) {
			if _, exists := snapshots[snapshotID]; !exists {
				return false, cloud.ErrNotFound
			}
			for name, id := range snapshotNameToID {
				if id == snapshotID {
					delete(snapshotNameToID, name)
					break
				}
			}
			delete(snapshots, snapshotID)
			return true, nil
		},
	).AnyTimes()

	// GetSnapshotByID
	c.EXPECT().GetSnapshotByID(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, snapshotID string) (*cloud.Snapshot, error) {
			snapshot, exists := snapshots[snapshotID]
			if !exists {
				return nil, cloud.ErrNotFound
			}
			return snapshot, nil
		},
	).AnyTimes()

	// GetSnapshotByName
	c.EXPECT().GetSnapshotByName(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, name string) (*cloud.Snapshot, error) {
			if snapshotID, exists := snapshotNameToID[name]; exists {
				return snapshots[snapshotID], nil
			}
			return nil, cloud.ErrNotFound
		},
	).AnyTimes()

	// ListSnapshots
	c.EXPECT().
		ListSnapshots(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(ctx context.Context, sourceVolumeID string, maxResults int32, nextToken string) (*cloud.ListSnapshotsResponse, error) {
			var s []*cloud.Snapshot
			startIndex := 0
			var err error

			if nextToken != "" {
				startIndex, err = strconv.Atoi(nextToken)
				if err != nil {
					return nil, fmt.Errorf("invalid next token %s", nextToken)
				}
			}

			var nextTokenStr string
			count := 0
			for _, snap := range snapshots {
				if snap.SourceVolumeID == sourceVolumeID || sourceVolumeID == "" {
					if startIndex <= count {
						s = append(s, snap)
						if maxResults > 0 && int32(len(s)) >= maxResults {
							nextTokenStr = strconv.Itoa(startIndex + int(maxResults))
							break
						}
					}
					count++
				}
			}

			return &cloud.ListSnapshotsResponse{
				Snapshots: s,
				NextToken: nextTokenStr,
			}, nil
		}).
		AnyTimes()

	// AttachDisk
	c.EXPECT().AttachDisk(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, volumeID string, instanceID string) (string, error) {
			_, diskExists := disks[volumeID]
			if !diskExists || instanceID != fakeMetaData.InstanceID {
				return "", cloud.ErrNotFound
			}
			return mountPath, nil
		},
	).AnyTimes()

	// DetachDisk
	c.EXPECT().DetachDisk(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, volumeID string, instanceID string) (bool, error) {
			_, diskExists := disks[volumeID]
			if !diskExists || instanceID != fakeMetaData.InstanceID {
				return false, cloud.ErrNotFound
			}
			return true, nil
		},
	).AnyTimes()

	// ResizeOrModifyDisk
	c.EXPECT().ResizeOrModifyDisk(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, volumeID string, newSizeBytes int64, modifyOptions *cloud.ModifyDiskOptions) (int32, error) {
			disk, exists := disks[volumeID]
			if !exists {
				return 0, cloud.ErrNotFound
			}
			newSizeGiB := util.BytesToGiB(newSizeBytes)
			disk.CapacityGiB = newSizeGiB
			disks[volumeID] = disk
			return newSizeGiB, nil
		},
	).AnyTimes()
}
