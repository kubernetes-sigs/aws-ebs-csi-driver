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
	"fmt"
	"math/rand"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/bertinatto/ebs-csi-driver/pkg/util"
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

func (c *FakeCloudProvider) GetMetadata() *Metadata {
	return &Metadata{"instanceID", "region", "az"}
}

func (c *FakeCloudProvider) CreateDisk(volumeName string, diskOptions *DiskOptions) (*Disk, error) {
	r1 := rand.New(rand.NewSource(time.Now().UnixNano()))
	d := &fakeDisk{
		Disk: &Disk{
			VolumeID:    fmt.Sprintf("vol-%d", r1.Uint64()),
			CapacityGiB: util.BytesToGiB(diskOptions.CapacityBytes),
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

func (c *FakeCloudProvider) AttachDisk(volumeID, nodeID string) error {
	return nil
}

func (c *FakeCloudProvider) DetachDisk(volumeID, nodeID string) error {
	return nil
}

func (c *FakeCloudProvider) GetDiskByNameAndSize(name string, capacityBytes int64) (*Disk, error) {
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

type fakeEC2 struct{}

func (f *fakeEC2) DescribeVolumes(input *ec2.DescribeVolumesInput) (*ec2.DescribeVolumesOutput, error) {
	return &ec2.DescribeVolumesOutput{}, nil
}

func (f *fakeEC2) CreateVolume(input *ec2.CreateVolumeInput) (*ec2.Volume, error) {
	return &ec2.Volume{
		VolumeId: aws.String("vol-test"),
		Size:     aws.Int64(1),
	}, nil
}

func (f *fakeEC2) DeleteVolume(input *ec2.DeleteVolumeInput) (*ec2.DeleteVolumeOutput, error) {
	return &ec2.DeleteVolumeOutput{}, nil
}

func (f *fakeEC2) AttachVolume(input *ec2.AttachVolumeInput) (*ec2.VolumeAttachment, error) {
	return &ec2.VolumeAttachment{}, nil
}

func (f *fakeEC2) DetachVolume(input *ec2.DetachVolumeInput) (*ec2.VolumeAttachment, error) {
	return &ec2.VolumeAttachment{}, nil
}
