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

package sanity

import (
	"context"
	"fmt"
	"math/rand"
	"time"

	"github.com/kubernetes-sigs/aws-ebs-csi-driver/pkg/cloud"
	"github.com/kubernetes-sigs/aws-ebs-csi-driver/pkg/util"
)

type fakeCloudProvider struct {
	disks     map[string]*fakeDisk
	snapshots map[string]*fakeSnapshot
	m         *cloud.Metadata
	pub       map[string]string
}

type fakeDisk struct {
	*cloud.Disk
	tags map[string]string
}

type fakeSnapshot struct {
	*cloud.Snapshot
	tags map[string]string
}

func newFakeCloudProvider() *fakeCloudProvider {
	return &fakeCloudProvider{
		disks:     make(map[string]*fakeDisk),
		snapshots: make(map[string]*fakeSnapshot),
		pub:       make(map[string]string),
		m: &cloud.Metadata{
			InstanceID:       "instanceID",
			Region:           "region",
			AvailabilityZone: "az",
		},
	}
}

func (c *fakeCloudProvider) GetMetadata() cloud.MetadataService {
	return c.m
}

func (c *fakeCloudProvider) CreateDisk(ctx context.Context, volumeName string, diskOptions *cloud.DiskOptions) (*cloud.Disk, error) {
	r1 := rand.New(rand.NewSource(time.Now().UnixNano()))
	d := &fakeDisk{
		Disk: &cloud.Disk{
			VolumeID:         fmt.Sprintf("vol-%d", r1.Uint64()),
			CapacityGiB:      util.BytesToGiB(diskOptions.CapacityBytes),
			AvailabilityZone: diskOptions.AvailabilityZone,
		},
		tags: diskOptions.Tags,
	}
	c.disks[volumeName] = d
	return d.Disk, nil
}

func (c *fakeCloudProvider) DeleteDisk(ctx context.Context, volumeID string) (bool, error) {
	for volName, f := range c.disks {
		if f.Disk.VolumeID == volumeID {
			delete(c.disks, volName)
		}
	}
	return true, nil
}

func (c *fakeCloudProvider) AttachDisk(ctx context.Context, volumeID, nodeID string) (string, error) {
	if _, ok := c.pub[volumeID]; ok {
		return "", cloud.ErrAlreadyExists
	}
	c.pub[volumeID] = nodeID
	return "/dev/xvdbc", nil
}

func (c *fakeCloudProvider) DetachDisk(ctx context.Context, volumeID, nodeID string) error {
	return nil
}

func (c *fakeCloudProvider) WaitForAttachmentState(ctx context.Context, volumeID, state string) error {
	return nil
}

func (c *fakeCloudProvider) GetDiskByName(ctx context.Context, name string, capacityBytes int64) (*cloud.Disk, error) {
	var disks []*fakeDisk
	for _, d := range c.disks {
		for key, value := range d.tags {
			if key == cloud.VolumeNameTagKey && value == name {
				disks = append(disks, d)
			}
		}
	}
	if len(disks) > 1 {
		return nil, cloud.ErrMultiDisks
	} else if len(disks) == 1 {
		if capacityBytes != disks[0].Disk.CapacityGiB*util.GiB {
			return nil, cloud.ErrDiskExistsDiffSize
		}
		return disks[0].Disk, nil
	}
	return nil, nil
}

func (c *fakeCloudProvider) GetDiskByID(ctx context.Context, volumeID string) (*cloud.Disk, error) {
	for _, f := range c.disks {
		if f.Disk.VolumeID == volumeID {
			return f.Disk, nil
		}
	}
	return nil, cloud.ErrNotFound
}

func (c *fakeCloudProvider) IsExistInstance(ctx context.Context, nodeID string) bool {
	return nodeID == c.m.GetInstanceID()
}

func (c *fakeCloudProvider) CreateSnapshot(ctx context.Context, volumeID string, snapshotOptions *cloud.SnapshotOptions) (snapshot *cloud.Snapshot, err error) {
	r1 := rand.New(rand.NewSource(time.Now().UnixNano()))
	snapshotID := fmt.Sprintf("snapshot-%d", r1.Uint64())
	if len(snapshotOptions.Tags[cloud.SnapshotNameTagKey]) == 0 {
		// for simplicity: let's have the Name and ID identical
		snapshotOptions.Tags[cloud.SnapshotNameTagKey] = snapshotID
	}
	s := &fakeSnapshot{
		Snapshot: &cloud.Snapshot{
			SnapshotID:     snapshotID,
			SourceVolumeID: volumeID,
			Size:           1,
			CreationTime:   time.Now(),
		},
		tags: snapshotOptions.Tags,
	}
	c.snapshots[snapshotID] = s
	return s.Snapshot, nil

}

func (c *fakeCloudProvider) DeleteSnapshot(ctx context.Context, snapshotID string) (success bool, err error) {
	delete(c.snapshots, snapshotID)
	return true, nil

}

func (c *fakeCloudProvider) GetSnapshotByName(ctx context.Context, name string) (snapshot *cloud.Snapshot, err error) {
	var snapshots []*fakeSnapshot
	for _, s := range c.snapshots {
		for key, value := range s.tags {
			if key == cloud.SnapshotNameTagKey && value == name {
				snapshots = append(snapshots, s)
			}
		}
	}
	if len(snapshots) == 0 {
		return nil, nil
	}
	return snapshots[0].Snapshot, nil
}

func (c *fakeCloudProvider) ListSnapshots(ctx context.Context, volumeID string, maxResults int64, nextToken string) (listSnapshotsResponse *cloud.ListSnapshotsResponse, err error) {
	var snapshots []*cloud.Snapshot
	for _, fakeSnapshot := range c.snapshots {
		snapshots = append(snapshots, fakeSnapshot.Snapshot)
	}
	return &cloud.ListSnapshotsResponse{
		Snapshots: snapshots,
	}, nil
}
