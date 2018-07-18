package cloud

import (
	"fmt"
	"math/rand"
	"time"
)

type FakeCloudProvider struct {
	disks map[string]*fakeDisk
}

type fakeDisk struct {
	*Disk
	tags map[string]string
}

func NewFakeCloudProvider() *FakeCloudProvider {
	return &FakeCloudProvider{
		disks: make(map[string]*fakeDisk),
	}
}

func (c *FakeCloudProvider) CreateDisk(volumeName string, diskOptions *DiskOptions) (*Disk, error) {
	r1 := rand.New(rand.NewSource(time.Now().UnixNano()))
	d := &fakeDisk{
		Disk: &Disk{
			VolumeID:    fmt.Sprintf("vol-%d", r1.Uint64()),
			CapacityGiB: bytesToGiB(diskOptions.CapacityBytes),
		},
		tags: diskOptions.Tags,
	}
	c.disks[volumeName] = d
	return d.Disk, nil
}

func (c *FakeCloudProvider) DeleteDisk(volumeID string) (bool, error) {
	for volName, f := range c.disks {
		if f.Disk.VolumeID == volumeID {
			delete(c.disks, volName)
		}
	}
	return true, nil
}

func (c *FakeCloudProvider) GetVolumeByNameAndSize(name string, capacityBytes int64) (*Disk, error) {
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
