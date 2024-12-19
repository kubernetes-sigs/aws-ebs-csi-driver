// Copyright 2024 The Kubernetes Authors.
//
// Licensed under the Apache License, Version 2.0 (the 'License');
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//    http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an 'AS IS' BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package sanity

import (
	"context"
	"fmt"
	"math/rand"
	"strconv"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/kubernetes-sigs/aws-ebs-csi-driver/pkg/cloud"
	"github.com/kubernetes-sigs/aws-ebs-csi-driver/pkg/cloud/metadata"
	"github.com/kubernetes-sigs/aws-ebs-csi-driver/pkg/util"
)

var (
	disks            = make(map[string]*cloud.Disk)
	snapshots        = make(map[string]*cloud.Snapshot)
	snapshotNameToID = make(map[string]string)
)

type FakeCloud struct {
	fakeMetaData metadata.Metadata
	mountPath    string
}

func newFakeCloud(fmd metadata.Metadata, mp string) *FakeCloud {
	return &FakeCloud{
		fakeMetaData: fmd,
		mountPath:    mp,
	}
}

func (d *FakeCloud) CreateDisk(ctx context.Context, volumeID string, diskOptions *cloud.DiskOptions) (*cloud.Disk, error) {
	for _, existingDisk := range disks {
		if existingDisk.VolumeID == volumeID && existingDisk.CapacityGiB != util.BytesToGiB(diskOptions.CapacityBytes) {
			return nil, cloud.ErrAlreadyExists
		}
	}

	if diskOptions.SnapshotID != "" {
		if _, exists := snapshots[diskOptions.SnapshotID]; !exists {
			return nil, cloud.ErrNotFound
		}
		newDisk := &cloud.Disk{
			SnapshotID:       diskOptions.SnapshotID,
			VolumeID:         volumeID,
			AvailabilityZone: diskOptions.AvailabilityZone,
			CapacityGiB:      util.BytesToGiB(diskOptions.CapacityBytes),
		}
		disks[volumeID] = newDisk
		return newDisk, nil
	}

	newDisk := &cloud.Disk{
		VolumeID:         volumeID,
		AvailabilityZone: diskOptions.AvailabilityZone,
		CapacityGiB:      util.BytesToGiB(diskOptions.CapacityBytes),
	}
	disks[volumeID] = newDisk
	return newDisk, nil
}
func (d *FakeCloud) DeleteDisk(ctx context.Context, volumeID string) (bool, error) {
	_, exists := disks[volumeID]
	if !exists {
		return false, cloud.ErrNotFound
	}
	delete(disks, volumeID)
	return true, nil
}

func (d *FakeCloud) GetDiskByID(ctx context.Context, volumeID string) (*cloud.Disk, error) {
	disk, exists := disks[volumeID]
	if !exists {
		return nil, cloud.ErrNotFound
	}
	return disk, nil
}

func (d *FakeCloud) CreateSnapshot(ctx context.Context, volumeID string, opts *cloud.SnapshotOptions) (*cloud.Snapshot, error) {
	snapshotID := fmt.Sprintf("snapshot-%d", rand.New(rand.NewSource(time.Now().UnixNano())).Uint64())

	_, exists := snapshots[snapshotID]
	if exists {
		return nil, cloud.ErrAlreadyExists
	}
	newSnapshot := &cloud.Snapshot{
		SnapshotID:     snapshotID,
		SourceVolumeID: volumeID,
		CreationTime:   time.Now(),
		ReadyToUse:     true,
	}
	snapshots[snapshotID] = newSnapshot
	snapshotNameToID[opts.Tags["CSIVolumeSnapshotName"]] = snapshotID
	return newSnapshot, nil
}

func (d *FakeCloud) DeleteSnapshot(ctx context.Context, snapshotID string) (bool, error) {
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
}
func (d *FakeCloud) GetSnapshotByID(ctx context.Context, snapshotID string) (*cloud.Snapshot, error) {
	snapshot, exists := snapshots[snapshotID]
	if !exists {
		return nil, cloud.ErrNotFound
	}
	return snapshot, nil
}

func (d *FakeCloud) GetSnapshotByName(ctx context.Context, name string) (*cloud.Snapshot, error) {
	if snapshotID, exists := snapshotNameToID[name]; exists {
		return snapshots[snapshotID], nil
	}
	return nil, cloud.ErrNotFound
}

func (d *FakeCloud) ListSnapshots(ctx context.Context, sourceVolumeID string, maxResults int32, nextToken string) (*cloud.ListSnapshotsResponse, error) {
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
}

func (d *FakeCloud) AttachDisk(ctx context.Context, volumeID string, instanceID string) (string, error) {
	_, diskExists := disks[volumeID]
	if !diskExists || instanceID != d.fakeMetaData.InstanceID {
		return "", cloud.ErrNotFound
	}
	return d.mountPath, nil
}

func (d *FakeCloud) DetachDisk(ctx context.Context, volumeID string, instanceID string) error {
	_, diskExists := disks[volumeID]
	if !diskExists || instanceID != d.fakeMetaData.InstanceID {
		return cloud.ErrNotFound
	}
	return nil
}

func (d *FakeCloud) ResizeOrModifyDisk(ctx context.Context, volumeID string, newSizeBytes int64, modifyOptions *cloud.ModifyDiskOptions) (int32, error) {
	disk, exists := disks[volumeID]
	if !exists {
		return 0, cloud.ErrNotFound
	}
	newSizeGiB := util.BytesToGiB(newSizeBytes)
	disk.CapacityGiB = newSizeGiB
	disks[volumeID] = disk
	realSizeGiB := newSizeGiB
	return realSizeGiB, nil
}

func (d *FakeCloud) AvailabilityZones(ctx context.Context) (map[string]struct{}, error) {
	return map[string]struct{}{}, nil
}

func (d *FakeCloud) EnableFastSnapshotRestores(ctx context.Context, availabilityZones []string, snapshotID string) (*ec2.EnableFastSnapshotRestoresOutput, error) {
	return &ec2.EnableFastSnapshotRestoresOutput{}, nil
}

func (d *FakeCloud) GetDiskByName(ctx context.Context, name string, capacityBytes int64) (*cloud.Disk, error) {
	return &cloud.Disk{}, nil
}

func (d *FakeCloud) ModifyTags(ctx context.Context, volumeID string, tagOptions cloud.ModifyTagsOptions) error {
	return nil
}

func (d *FakeCloud) WaitForAttachmentState(ctx context.Context, volumeID, expectedState, expectedInstance, expectedDevice string, alreadyAssigned bool) (*types.VolumeAttachment, error) {
	return &types.VolumeAttachment{}, nil
}
