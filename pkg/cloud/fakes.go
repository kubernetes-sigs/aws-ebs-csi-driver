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

package cloud

import (
	"context"
	"fmt"
	"math/rand"
	"time"

	"github.com/kubernetes-sigs/aws-ebs-csi-driver/pkg/util"
)

type FakeCloudProvider struct {
	disks     map[string]*fakeDisk
	snapshots map[string]*fakeSnapshot
	m         *metadata
	pub       map[string]string
}

type fakeDisk struct {
	*Disk
	tags map[string]string
}

type fakeSnapshot struct {
	*Snapshot
	tags map[string]string
}

func NewFakeCloudProvider() *FakeCloudProvider {
	return &FakeCloudProvider{
		disks:     make(map[string]*fakeDisk),
		snapshots: make(map[string]*fakeSnapshot),
		pub:       make(map[string]string),
		m:         &metadata{"instanceID", "region", "az"},
	}
}

func (c *FakeCloudProvider) GetMetadata() MetadataService {
	return c.m
}

func (c *FakeCloudProvider) CreateDisk(ctx context.Context, volumeName string, diskOptions *DiskOptions) (*Disk, error) {
	r1 := rand.New(rand.NewSource(time.Now().UnixNano()))
	d := &fakeDisk{
		Disk: &Disk{
			VolumeID:         fmt.Sprintf("vol-%d", r1.Uint64()),
			CapacityGiB:      util.BytesToGiB(diskOptions.CapacityBytes),
			AvailabilityZone: diskOptions.AvailabilityZone,
		},
		tags: diskOptions.Tags,
	}
	c.disks[volumeName] = d
	return d.Disk, nil
}

func (c *FakeCloudProvider) DeleteDisk(ctx context.Context, volumeID string) (bool, error) {
	for volName, f := range c.disks {
		if f.Disk.VolumeID == volumeID {
			delete(c.disks, volName)
		}
	}
	return true, nil
}

func (c *FakeCloudProvider) AttachDisk(ctx context.Context, volumeID, nodeID string) (string, error) {
	if _, ok := c.pub[volumeID]; ok {
		return "", ErrAlreadyExists
	}
	c.pub[volumeID] = nodeID
	return "/dev/xvdbc", nil
}

func (c *FakeCloudProvider) DetachDisk(ctx context.Context, volumeID, nodeID string) error {
	return nil
}

func (c *FakeCloudProvider) WaitForAttachmentState(ctx context.Context, volumeID, state string) error {
	return nil
}

func (c *FakeCloudProvider) GetDiskByName(ctx context.Context, name string, capacityBytes int64) (*Disk, error) {
	var disks []*fakeDisk
	for _, d := range c.disks {
		for key, value := range d.tags {
			if key == VolumeNameTagKey && value == name {
				disks = append(disks, d)
			}
		}
	}
	if len(disks) > 1 {
		return nil, ErrMultiDisks
	} else if len(disks) == 1 {
		if capacityBytes != disks[0].Disk.CapacityGiB*1024*1024*1024 {
			return nil, ErrDiskExistsDiffSize
		}
		return disks[0].Disk, nil
	}
	return nil, nil
}

func (c *FakeCloudProvider) GetDiskByID(ctx context.Context, volumeID string) (*Disk, error) {
	for _, f := range c.disks {
		if f.Disk.VolumeID == volumeID {
			return f.Disk, nil
		}
	}
	return nil, ErrNotFound
}

func (c *FakeCloudProvider) IsExistInstance(ctx context.Context, nodeID string) bool {
	return nodeID == c.m.GetInstanceID()
}

func (c *FakeCloudProvider) CreateSnapshot(ctx context.Context, volumeID string, snapshotOptions *SnapshotOptions) (snapshot *Snapshot, err error) {
	r1 := rand.New(rand.NewSource(time.Now().UnixNano()))
	snapshotID := fmt.Sprintf("snapshot-%d", r1.Uint64())
	if len(snapshotOptions.Tags[SnapshotNameTagKey]) == 0 {
		// for simplicity: let's have the Name and ID identical
		snapshotOptions.Tags[SnapshotNameTagKey] = snapshotID
	}
	s := &fakeSnapshot{
		Snapshot: &Snapshot{
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

func (c *FakeCloudProvider) DeleteSnapshot(ctx context.Context, snapshotID string) (success bool, err error) {
	delete(c.snapshots, snapshotID)
	return true, nil

}

func (c *FakeCloudProvider) GetSnapshotByName(ctx context.Context, name string) (snapshot *Snapshot, err error) {
	var snapshots []*fakeSnapshot
	for _, s := range c.snapshots {
		for key, value := range s.tags {
			if key == SnapshotNameTagKey && value == name {
				snapshots = append(snapshots, s)
			}
		}
	}
	if len(snapshots) == 0 {
		return nil, nil
	}
	return snapshots[0].Snapshot, nil
}
