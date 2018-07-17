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
	realVolumeID string
	options      *DiskOptions
}

func NewFakeCloudProvider() *FakeCloudProvider {
	return &FakeCloudProvider{
		disks: make(map[string]*fakeDisk),
	}
}

func (f *FakeCloudProvider) CreateDisk(volumeName string, diskOptions *DiskOptions) (string, error) {
	if d, ok := f.disks[volumeName]; ok {
		if d.options.CapacityGB == diskOptions.CapacityGB {
			return d.realVolumeID, nil
		}
		return "", fmt.Errorf("volume already exist with different capacity")
	}
	r1 := rand.New(rand.NewSource(time.Now().UnixNano()))
	realVolumeID := fmt.Sprintf("vol-%d", r1.Uint64())
	f.disks[volumeName] = &fakeDisk{realVolumeID, diskOptions}
	return realVolumeID, nil
}

func (f *FakeCloudProvider) DeleteDisk(volumeID string) (bool, error) {
	for volName, disk := range f.disks {
		if disk.realVolumeID == volumeID {
			delete(f.disks, volName)
		}
	}
	return true, nil
}

func (f *FakeCloudProvider) GetVolumeByNameAndSize(name string, size int) ([]string, error) {
	var disks []*fakeDisk
	for _, disk := range f.disks {
		for key, value := range disk.options.Tags {
			if key == VolumeNameTagKey && value == name {
				disks = append(disks, disk)
			}
		}
	}
	if len(disks) == 1 {
		if disks[0].options.CapacityGB != size {
			return nil, ErrDiskExistsDiffSize
		}
	}
	var results []string
	for _, disk := range disks {
		results = append(results, string(disk.realVolumeID))
	}
	return results, nil
}
