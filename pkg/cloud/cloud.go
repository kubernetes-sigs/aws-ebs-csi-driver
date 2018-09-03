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
	"errors"
	"fmt"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/credentials/ec2rolecreds"
	"github.com/aws/aws-sdk-go/aws/ec2metadata"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"
	dm "github.com/bertinatto/ebs-csi-driver/pkg/cloud/devicemanager"
	"github.com/bertinatto/ebs-csi-driver/pkg/util"
	"github.com/golang/glog"
)

const (
	// DefaultVolumeSize represents the default volume size.
	// TODO: what should be the default size?
	DefaultVolumeSize int64 = 1 * 1024 * 1024 * 1024

	// VolumeNameTagKey is the key value that refers to the volume's name.
	VolumeNameTagKey = "com.amazon.aws.csi.volume"

	// VolumeTypeIO1 represents a provisioned IOPS SSD type of volume.
	VolumeTypeIO1 = "io1"

	// VolumeTypeGP2 represents a general purpose SSD type of volume.
	VolumeTypeGP2 = "gp2"

	// VolumeTypeSC1 represents a cold HDD (sc1) type of volume.
	VolumeTypeSC1 = "sc1"

	// VolumeTypeST1 represents a throughput-optimized HDD type of volume.
	VolumeTypeST1 = "st1"

	// MinTotalIOPS represents the minimum Input Output per second.
	MinTotalIOPS int64 = 100

	// MaxTotalIOPS represents the maximum Input Output per second.
	MaxTotalIOPS int64 = 20000

	// DefaultVolumeType specifies which storage to use for newly created Volumes.
	DefaultVolumeType = VolumeTypeGP2
)

var (
	// ErrMultiDisks is an error that is returned when multiple
	// disks are found with the same volume name.
	ErrMultiDisks = errors.New("Multiple disks with same name")

	// ErrDiskExistsDiffSize is an error that is returned if a disk with a given
	// name, but different size, is found.
	ErrDiskExistsDiffSize = errors.New("There is already a disk with same name and different size")

	// ErrVolumeNotFound is returned when a volume with a given ID is not found.
	ErrVolumeNotFound = errors.New("Volume was not found")
)

type Disk struct {
	VolumeID    string
	CapacityGiB int64
}

type DiskOptions struct {
	CapacityBytes int64
	Tags          map[string]string
	VolumeType    string
	IOPSPerGB     int64
}

// EC2 abstracts aws.EC2 to facilitate its mocking.
type EC2 interface {
	DescribeVolumes(input *ec2.DescribeVolumesInput) (*ec2.DescribeVolumesOutput, error)
	CreateVolume(input *ec2.CreateVolumeInput) (*ec2.Volume, error)
	DeleteVolume(input *ec2.DeleteVolumeInput) (*ec2.DeleteVolumeOutput, error)
	DetachVolume(input *ec2.DetachVolumeInput) (*ec2.VolumeAttachment, error)
	AttachVolume(input *ec2.AttachVolumeInput) (*ec2.VolumeAttachment, error)
	DescribeInstances(input *ec2.DescribeInstancesInput) (*ec2.DescribeInstancesOutput, error)
}

type Cloud interface {
	GetMetadata() MetadataService
	CreateDisk(string, *DiskOptions) (*Disk, error)
	DeleteDisk(string) (bool, error)
	AttachDisk(string, string) (string, error)
	DetachDisk(string, string) error
	GetDisk(string, int64) (*Disk, error)
}

type cloud struct {
	metadata MetadataService
	ec2      EC2
	dm       dm.BlockDeviceManager
}

var _ Cloud = &cloud{}

func NewCloud() (Cloud, error) {
	sess, err := session.NewSession(&aws.Config{})
	if err != nil {
		return nil, fmt.Errorf("unable to initialize AWS session: %v", err)
	}

	svc := ec2metadata.New(sess)

	metadata, err := NewMetadataService(svc)
	if err != nil {
		return nil, fmt.Errorf("could not get metadata from AWS: %v", err)
	}

	provider := []credentials.Provider{
		&credentials.EnvProvider{},
		&ec2rolecreds.EC2RoleProvider{Client: svc},
		&credentials.SharedCredentialsProvider{},
	}

	awsConfig := &aws.Config{
		Region:      aws.String(metadata.GetRegion()),
		Credentials: credentials.NewChainCredentials(provider),
	}
	awsConfig = awsConfig.WithCredentialsChainVerboseErrors(true)

	return &cloud{
		metadata: metadata,
		dm:       dm.NewBlockDeviceManager(),
		ec2:      ec2.New(session.New(awsConfig)),
	}, nil
}

func (c *cloud) GetMetadata() MetadataService {
	return c.metadata
}

func (c *cloud) CreateDisk(volumeName string, diskOptions *DiskOptions) (*Disk, error) {
	var createType string
	var iops int64
	capacityGiB := util.BytesToGiB(diskOptions.CapacityBytes)

	switch diskOptions.VolumeType {
	case VolumeTypeGP2, VolumeTypeSC1, VolumeTypeST1:
		createType = diskOptions.VolumeType
	case VolumeTypeIO1:
		createType = diskOptions.VolumeType
		iops = capacityGiB * diskOptions.IOPSPerGB
		if iops < MinTotalIOPS {
			iops = MinTotalIOPS
		}
		if iops > MaxTotalIOPS {
			iops = MaxTotalIOPS
		}
	case "":
		createType = DefaultVolumeType
	default:
		return nil, fmt.Errorf("invalid AWS VolumeType %q", diskOptions.VolumeType)
	}

	var tags []*ec2.Tag
	for key, value := range diskOptions.Tags {
		tags = append(tags, &ec2.Tag{Key: &key, Value: &value})
	}
	tagSpec := ec2.TagSpecification{
		ResourceType: aws.String("volume"),
		Tags:         tags,
	}

	m := c.GetMetadata()
	request := &ec2.CreateVolumeInput{
		AvailabilityZone:  aws.String(m.GetAvailabilityZone()),
		Size:              aws.Int64(capacityGiB),
		VolumeType:        aws.String(createType),
		TagSpecifications: []*ec2.TagSpecification{&tagSpec},
	}
	if iops > 0 {
		request.Iops = aws.Int64(iops)
	}

	response, err := c.ec2.CreateVolume(request)
	if err != nil {
		return nil, fmt.Errorf("could not create volume in EC2: %v", err)
	}

	volumeID := aws.StringValue(response.VolumeId)
	if len(volumeID) == 0 {
		return nil, fmt.Errorf("volume ID was not returned by CreateVolume")
	}

	size := aws.Int64Value(response.Size)
	if size == 0 {
		return nil, fmt.Errorf("disk size was not returned by CreateVolume")
	}

	return &Disk{CapacityGiB: size, VolumeID: volumeID}, nil
}

func (c *cloud) DeleteDisk(volumeID string) (bool, error) {
	request := &ec2.DeleteVolumeInput{VolumeId: &volumeID}
	if _, err := c.ec2.DeleteVolume(request); err != nil {
		if awsErr, ok := err.(awserr.Error); ok {
			if awsErr.Code() == "InvalidVolume.NotFound" {
				return false, ErrVolumeNotFound
			}
		}
		return false, fmt.Errorf("DeleteDisk could not delete volume: %v", err)
	}
	return true, nil
}

func (c *cloud) AttachDisk(volumeID, nodeID string) (string, error) {
	instance, err := c.getInstance(nodeID)
	if err != nil {
		return "", fmt.Errorf("could not get instance %q", nodeID)
	}

	device, err := c.dm.NewBlockDevice(instance, volumeID)
	if err != nil {
		return "", err
	}
	defer device.Release(false)

	if !device.IsAlreadyAssigned {
		request := &ec2.AttachVolumeInput{
			Device:     aws.String(device.Path),
			InstanceId: aws.String(nodeID),
			VolumeId:   aws.String(volumeID),
		}

		resp, err := c.ec2.AttachVolume(request)
		if err != nil {
			return "", fmt.Errorf("could not attach volume %q to node %q: %v", volumeID, nodeID, err)
		}
		glog.V(2).Infof("AttachVolume volume=%q instance=%q request returned %v", volumeID, nodeID, resp)
	}

	// TODO: wait attaching
	// TODO: this is the only situation where this method returns and the device is not released
	//attachment, err := disk.waitForAttachmentStatus("attached")
	//if err != nil {
	// device.Taint()
	//if err == wait.ErrWaitTimeout {
	//c.applyUnSchedulableTaint(nodeName, "Volume stuck in attaching state - node needs reboot to fix impaired state.")
	//}
	//return "", err
	//}

	// Manually release the device so it can be picked again.

	// Double check the attachment to be 100% sure we attached the correct volume at the correct mountpoint
	// It could happen otherwise that we see the volume attached from a previous/separate AttachVolume call,
	// which could theoretically be against a different device (or even instance).
	//if attachment == nil {
	//// Impossible?
	//return "", fmt.Errorf("unexpected state: attachment nil after attached %q to %q", diskName, nodeName)
	//}
	//if ec2Device != aws.StringValue(attachment.Device) {
	//return "", fmt.Errorf("disk attachment of %q to %q failed: requested device %q but found %q", diskName, nodeName, ec2Device, aws.StringValue(attachment.Device))
	//}
	//if awsInstance.awsID != aws.StringValue(attachment.InstanceId) {
	//return "", fmt.Errorf("disk attachment of %q to %q failed: requested instance %q but found %q", diskName, nodeName, awsInstance.awsID, aws.StringValue(attachment.InstanceId))
	//}
	//return hostDevice, nil

	return device.Path, nil
}

func (c *cloud) DetachDisk(volumeID, nodeID string) error {
	instance, err := c.getInstance(nodeID)
	if err != nil {
		return fmt.Errorf("could not get instance %q", nodeID)
	}

	// TODO: check if attached
	device, err := c.dm.GetBlockDevice(instance, volumeID)
	if err != nil {
		return err
	}
	defer device.Release(true)

	if !device.IsAlreadyAssigned {
		// There is no device attached for this volume in this node
		glog.Warningf("DetachDisk called on non-attached volume: %s", volumeID)
	}

	request := &ec2.DetachVolumeInput{
		InstanceId: aws.String(nodeID),
		VolumeId:   aws.String(volumeID),
	}

	_, err = c.ec2.DetachVolume(request)
	if err != nil {
		return fmt.Errorf("could not detach volume %q from node %q: %v", volumeID, nodeID, err)
	}

	return nil
}

func (c *cloud) GetDisk(name string, capacityBytes int64) (*Disk, error) {
	volume, err := c.getVolume(name)
	if err != nil {
		return nil, err
	}

	volSizeBytes := aws.Int64Value(volume.Size)
	if volSizeBytes != util.BytesToGiB(capacityBytes) {
		return nil, ErrDiskExistsDiffSize
	}

	return &Disk{
		VolumeID:    aws.StringValue(volume.VolumeId),
		CapacityGiB: volSizeBytes,
	}, nil
}

func (c *cloud) getVolume(name string) (*ec2.Volume, error) {
	var volumes []*ec2.Volume
	var nextToken *string

	request := &ec2.DescribeVolumesInput{
		Filters: []*ec2.Filter{
			&ec2.Filter{
				Name:   aws.String("tag:" + VolumeNameTagKey),
				Values: []*string{aws.String(name)},
			},
		},
	}
	for {
		response, err := c.ec2.DescribeVolumes(request)
		if err != nil {
			return nil, err
		}
		for _, volume := range response.Volumes {
			volumes = append(volumes, volume)
		}
		nextToken = response.NextToken
		if aws.StringValue(nextToken) == "" {
			break
		}
		request.NextToken = nextToken
	}

	if len(volumes) > 1 {
		return nil, ErrMultiDisks
	}

	if len(volumes) == 0 {
		return nil, ErrVolumeNotFound
	}

	return volumes[0], nil
}

func (c *cloud) getInstance(nodeID string) (*ec2.Instance, error) {
	results := []*ec2.Instance{}
	request := &ec2.DescribeInstancesInput{
		InstanceIds: []*string{&nodeID},
	}

	var nextToken *string
	for {
		response, err := c.ec2.DescribeInstances(request)
		if err != nil {
			return nil, fmt.Errorf("error listing AWS instances: %q", err)
		}

		for _, reservation := range response.Reservations {
			results = append(results, reservation.Instances...)
		}

		nextToken = response.NextToken
		if aws.StringValue(nextToken) == "" {
			break
		}
		request.NextToken = nextToken
	}

	nInstances := len(results)
	if nInstances != 1 {
		return nil, fmt.Errorf("expected 1 instance with ID %q, got %d", nodeID, len(results))
	}

	return results[0], nil
}
