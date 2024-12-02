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

type fakeCloud struct {
	fakeMetadata     metadata.Metadata
	mountPath        string
	disks            map[string]*cloud.Disk
	snapshots        map[string]*cloud.Snapshot
	snapshotNameToID map[string]string
}

func newFakeCloud(fmd metadata.Metadata, mp string) *fakeCloud {
	return &fakeCloud{
		fakeMetadata:     fmd,
		mountPath:        mp,
		disks:            make(map[string]*cloud.Disk),
		snapshots:        make(map[string]*cloud.Snapshot),
		snapshotNameToID: make(map[string]string),
	}
}

func (d *fakeCloud) CreateDisk(ctx context.Context, volumeID string, diskOptions *cloud.DiskOptions) (*cloud.Disk, error) {
	for _, existingDisk := range d.disks {
		if existingDisk.VolumeID == volumeID && existingDisk.CapacityGiB != util.BytesToGiB(diskOptions.CapacityBytes) {
			return nil, cloud.ErrAlreadyExists
		}
	}

	if diskOptions.SnapshotID != "" {
		if _, exists := d.snapshots[diskOptions.SnapshotID]; !exists {
			return nil, cloud.ErrNotFound
		}
		newDisk := &cloud.Disk{
			SnapshotID:       diskOptions.SnapshotID,
			VolumeID:         volumeID,
			AvailabilityZone: diskOptions.AvailabilityZone,
			CapacityGiB:      util.BytesToGiB(diskOptions.CapacityBytes),
		}
		d.disks[volumeID] = newDisk
		return newDisk, nil
	}

	newDisk := &cloud.Disk{
		VolumeID:         volumeID,
		AvailabilityZone: diskOptions.AvailabilityZone,
		CapacityGiB:      util.BytesToGiB(diskOptions.CapacityBytes),
	}
	d.disks[volumeID] = newDisk
	return newDisk, nil
}
func (d *fakeCloud) DeleteDisk(ctx context.Context, volumeID string) (bool, error) {
	_, exists := d.disks[volumeID]
	if !exists {
		return false, cloud.ErrNotFound
	}
	delete(d.disks, volumeID)
	return true, nil
}

func (d *fakeCloud) GetDiskByID(ctx context.Context, volumeID string) (*cloud.Disk, error) {
	disk, exists := d.disks[volumeID]
	if !exists {
		return nil, cloud.ErrNotFound
	}
	return disk, nil
}

func (d *fakeCloud) CreateSnapshot(ctx context.Context, volumeID string, opts *cloud.SnapshotOptions) (*cloud.Snapshot, error) {
	snapshotID := fmt.Sprintf("snap-%d", rand.New(rand.NewSource(time.Now().UnixNano())).Uint64())

	_, exists := d.snapshots[snapshotID]
	if exists {
		return nil, cloud.ErrAlreadyExists
	}
	newSnapshot := &cloud.Snapshot{
		SnapshotID:     snapshotID,
		SourceVolumeID: volumeID,
		CreationTime:   time.Now(),
		ReadyToUse:     true,
	}
	d.snapshots[snapshotID] = newSnapshot
	d.snapshotNameToID[opts.Tags["CSIVolumeSnapshotName"]] = snapshotID
	return newSnapshot, nil
}

func (d *fakeCloud) DeleteSnapshot(ctx context.Context, snapshotID string) (bool, error) {
	if _, exists := d.snapshots[snapshotID]; !exists {
		return false, cloud.ErrNotFound
	}
	for name, id := range d.snapshotNameToID {
		if id == snapshotID {
			delete(d.snapshotNameToID, name)
			break
		}
	}
	delete(d.snapshots, snapshotID)
	return true, nil
}
func (d *fakeCloud) GetSnapshotByID(ctx context.Context, snapshotID string) (*cloud.Snapshot, error) {
	snapshot, exists := d.snapshots[snapshotID]
	if !exists {
		return nil, cloud.ErrNotFound
	}
	return snapshot, nil
}

func (d *fakeCloud) GetSnapshotByName(ctx context.Context, name string) (*cloud.Snapshot, error) {
	if snapshotID, exists := d.snapshotNameToID[name]; exists {
		return d.snapshots[snapshotID], nil
	}
	return nil, cloud.ErrNotFound
}

func (d *fakeCloud) ListSnapshots(ctx context.Context, sourceVolumeID string, maxResults int32, nextToken string) (*cloud.ListSnapshotsResponse, error) {
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
	for _, snap := range d.snapshots {
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

func (d *fakeCloud) AttachDisk(ctx context.Context, volumeID string, instanceID string) (string, error) {
	_, diskExists := d.disks[volumeID]
	if !diskExists || instanceID != d.fakeMetadata.InstanceID {
		return "", cloud.ErrNotFound
	}
	return d.mountPath, nil
}

func (d *fakeCloud) DetachDisk(ctx context.Context, volumeID string, instanceID string) error {
	_, diskExists := d.disks[volumeID]
	if !diskExists || instanceID != d.fakeMetadata.InstanceID {
		return cloud.ErrNotFound
	}
	return nil
}

func (d *fakeCloud) ResizeOrModifyDisk(ctx context.Context, volumeID string, newSizeBytes int64, modifyOptions *cloud.ModifyDiskOptions) (int32, error) {
	disk, exists := d.disks[volumeID]
	if !exists {
		return 0, cloud.ErrNotFound
	}
	newSizeGiB := util.BytesToGiB(newSizeBytes)
	disk.CapacityGiB = newSizeGiB
	d.disks[volumeID] = disk
	realSizeGiB := newSizeGiB
	return realSizeGiB, nil
}

func (d *fakeCloud) AvailabilityZones(ctx context.Context) (map[string]struct{}, error) {
	return map[string]struct{}{}, nil
}

func (d *fakeCloud) EnableFastSnapshotRestores(ctx context.Context, availabilityZones []string, snapshotID string) (*ec2.EnableFastSnapshotRestoresOutput, error) {
	return &ec2.EnableFastSnapshotRestoresOutput{}, nil
}

func (d *fakeCloud) GetDiskByName(ctx context.Context, name string, capacityBytes int64) (*cloud.Disk, error) {
	return &cloud.Disk{}, nil
}

func (d *fakeCloud) ModifyTags(ctx context.Context, volumeID string, tagOptions cloud.ModifyTagsOptions) error {
	return nil
}

func (d *fakeCloud) WaitForAttachmentState(ctx context.Context, volumeID, expectedState, expectedInstance, expectedDevice string, alreadyAssigned bool) (*types.VolumeAttachment, error) {
	return &types.VolumeAttachment{}, nil
}
