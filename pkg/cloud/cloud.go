/*
Copyright 2019 The Kubernetes Authors.

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
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"
	dm "github.com/kubernetes-sigs/aws-ebs-csi-driver/pkg/cloud/devicemanager"
	"github.com/kubernetes-sigs/aws-ebs-csi-driver/pkg/util"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/klog"
)

// AWS volume types
const (
	// VolumeTypeIO1 represents a provisioned IOPS SSD type of volume.
	VolumeTypeIO1 = "io1"
	// VolumeTypeIO2 represents a provisioned IOPS SSD type of volume.
	VolumeTypeIO2 = "io2"
	// VolumeTypeGP2 represents a general purpose SSD type of volume.
	VolumeTypeGP2 = "gp2"
	// VolumeTypeGP3 represents a general purpose SSD type of volume.
	VolumeTypeGP3 = "gp3"
	// VolumeTypeSC1 represents a cold HDD (sc1) type of volume.
	VolumeTypeSC1 = "sc1"
	// VolumeTypeST1 represents a throughput-optimized HDD type of volume.
	VolumeTypeST1 = "st1"
	// VolumeTypeStandard represents a previous type of  volume.
	VolumeTypeStandard = "standard"
)

var (
	ValidVolumeTypes = []string{
		VolumeTypeIO1,
		VolumeTypeIO2,
		VolumeTypeGP2,
		VolumeTypeGP3,
		VolumeTypeSC1,
		VolumeTypeST1,
		VolumeTypeStandard,
	}

	volumeModificationDuration   = 1 * time.Second
	volumeModificationWaitFactor = 1.7
	volumeModificationWaitSteps  = 10
)

// AWS provisioning limits.
// Source:
//   https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/Using_Tags.html#tag-restrictions
const (
	// MaxNumTagsPerResource represents the maximum number of tags per AWS resource.
	MaxNumTagsPerResource = 50
	// MaxTagKeyLength represents the maximum key length for a tag.
	MaxTagKeyLength = 128
	// MaxTagValueLength represents the maximum value length for a tag.
	MaxTagValueLength = 256
)

// Defaults
const (
	// DefaultVolumeSize represents the default volume size.
	DefaultVolumeSize int64 = 100 * util.GiB
	// DefaultVolumeType specifies which storage to use for newly created Volumes.
	DefaultVolumeType = VolumeTypeGP3
)

// Tags
const (
	// VolumeNameTagKey is the key value that refers to the volume's name.
	VolumeNameTagKey = "CSIVolumeName"
	// SnapshotNameTagKey is the key value that refers to the snapshot's name.
	SnapshotNameTagKey = "CSIVolumeSnapshotName"
	// KubernetesTagKeyPrefix is the prefix of the key value that is reserved for Kubernetes.
	KubernetesTagKeyPrefix = "kubernetes.io"
	// AWSTagKeyPrefix is the prefix of the key value that is reserved for AWS.
	AWSTagKeyPrefix = "aws:"
)

var (
	// ErrMultiDisks is an error that is returned when multiple
	// disks are found with the same volume name.
	ErrMultiDisks = errors.New("Multiple disks with same name")

	// ErrDiskExistsDiffSize is an error that is returned if a disk with a given
	// name, but different size, is found.
	ErrDiskExistsDiffSize = errors.New("There is already a disk with same name and different size")

	// ErrNotFound is returned when a resource is not found.
	ErrNotFound = errors.New("Resource was not found")

	// ErrAlreadyExists is returned when a resource is already existent.
	ErrAlreadyExists = errors.New("Resource already exists")

	// ErrVolumeInUse is returned when a volume is already attached to an instance.
	ErrVolumeInUse = errors.New("Request volume is already attached to an instance")

	// ErrMultiSnapshots is returned when multiple snapshots are found
	// with the same ID
	ErrMultiSnapshots = errors.New("Multiple snapshots with the same name found")

	// ErrInvalidMaxResults is returned when a MaxResults pagination parameter is between 1 and 4
	ErrInvalidMaxResults = errors.New("MaxResults parameter must be 0 or greater than or equal to 5")

	// VolumeNotBeingModified is returned if volume being described is not being modified
	VolumeNotBeingModified = fmt.Errorf("volume is not being modified")
)

// Disk represents a EBS volume
type Disk struct {
	VolumeID         string
	CapacityGiB      int64
	AvailabilityZone string
	SnapshotID       string
	OutpostArn       string
	Attachments      []string
}

// DiskOptions represents parameters to create an EBS volume
type DiskOptions struct {
	CapacityBytes    int64
	Tags             map[string]string
	VolumeType       string
	IOPSPerGB        int
	IOPS             int
	Throughput       int
	AvailabilityZone string
	OutpostArn       string
	Encrypted        bool
	// KmsKeyID represents a fully qualified resource name to the key to use for encryption.
	// example: arn:aws:kms:us-east-1:012345678910:key/abcd1234-a123-456a-a12b-a123b4cd56ef
	KmsKeyID   string
	SnapshotID string
}

// Snapshot represents an EBS volume snapshot
type Snapshot struct {
	SnapshotID     string
	SourceVolumeID string
	Size           int64
	CreationTime   time.Time
	ReadyToUse     bool
}

// ListSnapshotsResponse is the container for our snapshots along with a pagination token to pass back to the caller
type ListSnapshotsResponse struct {
	Snapshots []*Snapshot
	NextToken string
}

// SnapshotOptions represents parameters to create an EBS volume
type SnapshotOptions struct {
	Tags map[string]string
}

// ec2ListSnapshotsResponse is a helper struct returned from the AWS API calling function to the main ListSnapshots function
type ec2ListSnapshotsResponse struct {
	Snapshots []*ec2.Snapshot
	NextToken *string
}

// EC2 abstracts aws.EC2 to facilitate its mocking.
// See https://docs.aws.amazon.com/sdk-for-go/api/service/ec2/ for details
type EC2 interface {
	DescribeVolumesWithContext(ctx aws.Context, input *ec2.DescribeVolumesInput, opts ...request.Option) (*ec2.DescribeVolumesOutput, error)
	CreateVolumeWithContext(ctx aws.Context, input *ec2.CreateVolumeInput, opts ...request.Option) (*ec2.Volume, error)
	DeleteVolumeWithContext(ctx aws.Context, input *ec2.DeleteVolumeInput, opts ...request.Option) (*ec2.DeleteVolumeOutput, error)
	DetachVolumeWithContext(ctx aws.Context, input *ec2.DetachVolumeInput, opts ...request.Option) (*ec2.VolumeAttachment, error)
	AttachVolumeWithContext(ctx aws.Context, input *ec2.AttachVolumeInput, opts ...request.Option) (*ec2.VolumeAttachment, error)
	DescribeInstancesWithContext(ctx aws.Context, input *ec2.DescribeInstancesInput, opts ...request.Option) (*ec2.DescribeInstancesOutput, error)
	CreateSnapshotWithContext(ctx aws.Context, input *ec2.CreateSnapshotInput, opts ...request.Option) (*ec2.Snapshot, error)
	DeleteSnapshotWithContext(ctx aws.Context, input *ec2.DeleteSnapshotInput, opts ...request.Option) (*ec2.DeleteSnapshotOutput, error)
	DescribeSnapshotsWithContext(ctx aws.Context, input *ec2.DescribeSnapshotsInput, opts ...request.Option) (*ec2.DescribeSnapshotsOutput, error)
	ModifyVolumeWithContext(ctx aws.Context, input *ec2.ModifyVolumeInput, opts ...request.Option) (*ec2.ModifyVolumeOutput, error)
	DescribeVolumesModificationsWithContext(ctx aws.Context, input *ec2.DescribeVolumesModificationsInput, opts ...request.Option) (*ec2.DescribeVolumesModificationsOutput, error)
	DescribeAvailabilityZonesWithContext(ctx aws.Context, input *ec2.DescribeAvailabilityZonesInput, opts ...request.Option) (*ec2.DescribeAvailabilityZonesOutput, error)
}

type Cloud interface {
	CreateDisk(ctx context.Context, volumeName string, diskOptions *DiskOptions) (disk *Disk, err error)
	DeleteDisk(ctx context.Context, volumeID string) (success bool, err error)
	AttachDisk(ctx context.Context, volumeID string, nodeID string) (devicePath string, err error)
	DetachDisk(ctx context.Context, volumeID string, nodeID string) (err error)
	ResizeDisk(ctx context.Context, volumeID string, reqSize int64) (newSize int64, err error)
	WaitForAttachmentState(ctx context.Context, volumeID, state string) error
	GetDiskByName(ctx context.Context, name string, capacityBytes int64) (disk *Disk, err error)
	GetDiskByID(ctx context.Context, volumeID string) (disk *Disk, err error)
	IsExistInstance(ctx context.Context, nodeID string) (success bool)
	CreateSnapshot(ctx context.Context, volumeID string, snapshotOptions *SnapshotOptions) (snapshot *Snapshot, err error)
	DeleteSnapshot(ctx context.Context, snapshotID string) (success bool, err error)
	GetSnapshotByName(ctx context.Context, name string) (snapshot *Snapshot, err error)
	GetSnapshotByID(ctx context.Context, snapshotID string) (snapshot *Snapshot, err error)
	ListSnapshots(ctx context.Context, volumeID string, maxResults int64, nextToken string) (listSnapshotsResponse *ListSnapshotsResponse, err error)
}

type cloud struct {
	region string
	ec2    EC2
	dm     dm.DeviceManager
}

var _ Cloud = &cloud{}

// NewCloud returns a new instance of AWS cloud
// It panics if session is invalid
func NewCloud(region string) (Cloud, error) {
	return newEC2Cloud(region)
}

func newEC2Cloud(region string) (Cloud, error) {
	awsConfig := &aws.Config{
		Region:                        aws.String(region),
		CredentialsChainVerboseErrors: aws.Bool(true),
	}

	endpoint := os.Getenv("AWS_EC2_ENDPOINT")
	if endpoint != "" {
		awsConfig.Endpoint = aws.String(endpoint)
	}

	return &cloud{
		region: region,
		dm:     dm.NewDeviceManager(),
		ec2:    ec2.New(session.Must(session.NewSession(awsConfig))),
	}, nil
}

func (c *cloud) CreateDisk(ctx context.Context, volumeName string, diskOptions *DiskOptions) (*Disk, error) {
	var (
		createType string
		iops       int64
		throughput int64
	)
	capacityGiB := util.BytesToGiB(diskOptions.CapacityBytes)

	switch diskOptions.VolumeType {
	case VolumeTypeGP2, VolumeTypeSC1, VolumeTypeST1, VolumeTypeStandard:
		createType = diskOptions.VolumeType
	case VolumeTypeIO1, VolumeTypeIO2:
		createType = diskOptions.VolumeType
		iops = capacityGiB * int64(diskOptions.IOPSPerGB)
	case VolumeTypeGP3:
		createType = diskOptions.VolumeType
		iops = int64(diskOptions.IOPS)
		throughput = int64(diskOptions.Throughput)
	case "":
		createType = DefaultVolumeType
	default:
		return nil, fmt.Errorf("invalid AWS VolumeType %q", diskOptions.VolumeType)
	}

	var tags []*ec2.Tag
	for key, value := range diskOptions.Tags {
		copiedKey := key
		copiedValue := value
		tags = append(tags, &ec2.Tag{Key: &copiedKey, Value: &copiedValue})
	}
	tagSpec := ec2.TagSpecification{
		ResourceType: aws.String("volume"),
		Tags:         tags,
	}

	zone := diskOptions.AvailabilityZone
	if zone == "" {
		klog.V(5).Infof("AZ is not provided. Using node AZ [%s]", zone)
		var err error
		zone, err = c.randomAvailabilityZone(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to get availability zone %s", err)
		}
	}

	request := &ec2.CreateVolumeInput{
		AvailabilityZone:  aws.String(zone),
		Size:              aws.Int64(capacityGiB),
		VolumeType:        aws.String(createType),
		TagSpecifications: []*ec2.TagSpecification{&tagSpec},
		Encrypted:         aws.Bool(diskOptions.Encrypted),
	}

	// EBS doesn't handle empty outpost arn, so we have to include it only when it's non-empty
	if len(diskOptions.OutpostArn) > 0 {
		request.OutpostArn = aws.String(diskOptions.OutpostArn)
	}

	if len(diskOptions.KmsKeyID) > 0 {
		request.KmsKeyId = aws.String(diskOptions.KmsKeyID)
		request.Encrypted = aws.Bool(true)
	}
	if iops > 0 {
		request.Iops = aws.Int64(iops)
	}
	if throughput > 0 && diskOptions.VolumeType == VolumeTypeGP3 {
		request.Throughput = aws.Int64(throughput)
	}
	snapshotID := diskOptions.SnapshotID
	if len(snapshotID) > 0 {
		request.SnapshotId = aws.String(snapshotID)
	}

	response, err := c.ec2.CreateVolumeWithContext(ctx, request)
	if err != nil {
		if isAWSErrorSnapshotNotFound(err) {
			return nil, ErrNotFound
		}
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

	if err := c.waitForVolume(ctx, volumeID); err != nil {
		return nil, fmt.Errorf("failed to get an available volume in EC2: %v", err)
	}

	outpostArn := aws.StringValue(response.OutpostArn)

	return &Disk{CapacityGiB: size, VolumeID: volumeID, AvailabilityZone: zone, SnapshotID: snapshotID, OutpostArn: outpostArn}, nil
}

func (c *cloud) DeleteDisk(ctx context.Context, volumeID string) (bool, error) {
	request := &ec2.DeleteVolumeInput{VolumeId: &volumeID}
	if _, err := c.ec2.DeleteVolumeWithContext(ctx, request); err != nil {
		if isAWSErrorVolumeNotFound(err) {
			return false, ErrNotFound
		}
		return false, fmt.Errorf("DeleteDisk could not delete volume: %v", err)
	}
	return true, nil
}

func (c *cloud) AttachDisk(ctx context.Context, volumeID, nodeID string) (string, error) {
	instance, err := c.getInstance(ctx, nodeID)
	if err != nil {
		return "", err
	}

	device, err := c.dm.NewDevice(instance, volumeID)
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

		resp, err := c.ec2.AttachVolumeWithContext(ctx, request)
		if err != nil {
			if awsErr, ok := err.(awserr.Error); ok {
				if awsErr.Code() == "VolumeInUse" {
					return "", ErrVolumeInUse
				}
			}
			return "", fmt.Errorf("could not attach volume %q to node %q: %v", volumeID, nodeID, err)
		}
		klog.V(5).Infof("AttachVolume volume=%q instance=%q request returned %v", volumeID, nodeID, resp)

	}

	// This is the only situation where we taint the device
	if err := c.WaitForAttachmentState(ctx, volumeID, "attached"); err != nil {
		device.Taint()
		return "", err
	}

	// TODO: Double check the attachment to be 100% sure we attached the correct volume at the correct mountpoint
	// It could happen otherwise that we see the volume attached from a previous/separate AttachVolume call,
	// which could theoretically be against a different device (or even instance).
	// TODO: Check volume capability matches for ALREADY_EXISTS
	// This could happen when request volume already attached to request node,
	// but is incompatible with the specified volume_capability or readonly flag
	return device.Path, nil
}

func (c *cloud) DetachDisk(ctx context.Context, volumeID, nodeID string) error {
	instance, err := c.getInstance(ctx, nodeID)
	if err != nil {
		return err
	}

	// TODO: check if attached
	device, err := c.dm.GetDevice(instance, volumeID)
	if err != nil {
		return err
	}
	defer device.Release(true)

	if !device.IsAlreadyAssigned {
		klog.Warningf("DetachDisk called on non-attached volume: %s", volumeID)
	}

	request := &ec2.DetachVolumeInput{
		InstanceId: aws.String(nodeID),
		VolumeId:   aws.String(volumeID),
	}

	_, err = c.ec2.DetachVolumeWithContext(ctx, request)
	if err != nil {
		if isAWSErrorIncorrectState(err) ||
			isAWSErrorInvalidAttachmentNotFound(err) ||
			isAWSErrorVolumeNotFound(err) {
			return ErrNotFound
		}
		return fmt.Errorf("could not detach volume %q from node %q: %v", volumeID, nodeID, err)
	}

	if err := c.WaitForAttachmentState(ctx, volumeID, "detached"); err != nil {
		return err
	}

	return nil
}

// WaitForAttachmentState polls until the attachment status is the expected value.
func (c *cloud) WaitForAttachmentState(ctx context.Context, volumeID, state string) error {
	// Most attach/detach operations on AWS finish within 1-4 seconds.
	// By using 1 second starting interval with a backoff of 1.8,
	// we get [1, 1.8, 3.24, 5.832000000000001, 10.4976].
	// In total we wait for 2601 seconds.
	backoff := wait.Backoff{
		Duration: 1 * time.Second,
		Factor:   1.8,
		Steps:    13,
	}

	verifyVolumeFunc := func() (bool, error) {
		request := &ec2.DescribeVolumesInput{
			VolumeIds: []*string{
				aws.String(volumeID),
			},
		}

		volume, err := c.getVolume(ctx, request)
		if err != nil {
			return false, err
		}

		if len(volume.Attachments) == 0 {
			if state == "detached" {
				return true, nil
			}
		}

		for _, a := range volume.Attachments {
			if a.State == nil {
				klog.Warningf("Ignoring nil attachment state for volume %q: %v", volumeID, a)
				continue
			}
			if *a.State == state {
				return true, nil
			}
		}
		return false, nil
	}

	return wait.ExponentialBackoff(backoff, verifyVolumeFunc)
}

func (c *cloud) GetDiskByName(ctx context.Context, name string, capacityBytes int64) (*Disk, error) {
	request := &ec2.DescribeVolumesInput{
		Filters: []*ec2.Filter{
			{
				Name:   aws.String("tag:" + VolumeNameTagKey),
				Values: []*string{aws.String(name)},
			},
		},
	}

	volume, err := c.getVolume(ctx, request)
	if err != nil {
		return nil, err
	}

	volSizeBytes := aws.Int64Value(volume.Size)
	if volSizeBytes != util.BytesToGiB(capacityBytes) {
		return nil, ErrDiskExistsDiffSize
	}

	return &Disk{
		VolumeID:         aws.StringValue(volume.VolumeId),
		CapacityGiB:      volSizeBytes,
		AvailabilityZone: aws.StringValue(volume.AvailabilityZone),
		SnapshotID:       aws.StringValue(volume.SnapshotId),
		OutpostArn:       aws.StringValue(volume.OutpostArn),
	}, nil
}

func (c *cloud) GetDiskByID(ctx context.Context, volumeID string) (*Disk, error) {
	request := &ec2.DescribeVolumesInput{
		VolumeIds: []*string{
			aws.String(volumeID),
		},
	}

	volume, err := c.getVolume(ctx, request)

	if err != nil {
		return nil, err
	}

	return &Disk{
		VolumeID:         aws.StringValue(volume.VolumeId),
		CapacityGiB:      aws.Int64Value(volume.Size),
		AvailabilityZone: aws.StringValue(volume.AvailabilityZone),
		OutpostArn:       aws.StringValue(volume.OutpostArn),
		Attachments:      getVolumeAttachmentsList(volume),
	}, nil
}

func (c *cloud) IsExistInstance(ctx context.Context, nodeID string) bool {
	instance, err := c.getInstance(ctx, nodeID)
	if err != nil || instance == nil {
		return false
	}
	return true
}

func (c *cloud) CreateSnapshot(ctx context.Context, volumeID string, snapshotOptions *SnapshotOptions) (snapshot *Snapshot, err error) {
	descriptions := "Created by AWS EBS CSI driver for volume " + volumeID

	var tags []*ec2.Tag
	for key, value := range snapshotOptions.Tags {
		copiedKey := key
		copiedValue := value
		tags = append(tags, &ec2.Tag{Key: &copiedKey, Value: &copiedValue})
	}
	tagSpec := ec2.TagSpecification{
		ResourceType: aws.String("snapshot"),
		Tags:         tags,
	}
	request := &ec2.CreateSnapshotInput{
		VolumeId:          aws.String(volumeID),
		DryRun:            aws.Bool(false),
		TagSpecifications: []*ec2.TagSpecification{&tagSpec},
		Description:       aws.String(descriptions),
	}

	res, err := c.ec2.CreateSnapshotWithContext(ctx, request)
	if err != nil {
		return nil, fmt.Errorf("error creating snapshot of volume %s: %v", volumeID, err)
	}
	if res == nil {
		return nil, fmt.Errorf("nil CreateSnapshotResponse")
	}

	return c.ec2SnapshotResponseToStruct(res), nil
}

func (c *cloud) DeleteSnapshot(ctx context.Context, snapshotID string) (success bool, err error) {
	request := &ec2.DeleteSnapshotInput{}
	request.SnapshotId = aws.String(snapshotID)
	request.DryRun = aws.Bool(false)
	if _, err := c.ec2.DeleteSnapshotWithContext(ctx, request); err != nil {
		if isAWSErrorSnapshotNotFound(err) {
			return false, ErrNotFound
		}
		return false, fmt.Errorf("DeleteSnapshot could not delete volume: %v", err)
	}
	return true, nil
}

func (c *cloud) GetSnapshotByName(ctx context.Context, name string) (snapshot *Snapshot, err error) {
	request := &ec2.DescribeSnapshotsInput{
		Filters: []*ec2.Filter{
			{
				Name:   aws.String("tag:" + SnapshotNameTagKey),
				Values: []*string{aws.String(name)},
			},
		},
	}

	ec2snapshot, err := c.getSnapshot(ctx, request)
	if err != nil {
		return nil, err
	}

	return c.ec2SnapshotResponseToStruct(ec2snapshot), nil
}

func (c *cloud) GetSnapshotByID(ctx context.Context, snapshotID string) (snapshot *Snapshot, err error) {
	request := &ec2.DescribeSnapshotsInput{
		SnapshotIds: []*string{
			aws.String(snapshotID),
		},
	}

	ec2snapshot, err := c.getSnapshot(ctx, request)
	if err != nil {
		return nil, err
	}

	return c.ec2SnapshotResponseToStruct(ec2snapshot), nil
}

// ListSnapshots retrieves AWS EBS snapshots for an optionally specified volume ID.  If maxResults is set, it will return up to maxResults snapshots.  If there are more snapshots than maxResults,
// a next token value will be returned to the client as well.  They can use this token with subsequent calls to retrieve the next page of results.  If maxResults is not set (0),
// there will be no restriction up to 1000 results (https://docs.aws.amazon.com/sdk-for-go/api/service/ec2/#DescribeSnapshotsInput).
func (c *cloud) ListSnapshots(ctx context.Context, volumeID string, maxResults int64, nextToken string) (listSnapshotsResponse *ListSnapshotsResponse, err error) {
	if maxResults > 0 && maxResults < 5 {
		return nil, ErrInvalidMaxResults
	}

	describeSnapshotsInput := &ec2.DescribeSnapshotsInput{
		MaxResults: aws.Int64(maxResults),
	}

	if len(nextToken) != 0 {
		describeSnapshotsInput.NextToken = aws.String(nextToken)
	}
	if len(volumeID) != 0 {
		describeSnapshotsInput.Filters = []*ec2.Filter{
			{
				Name:   aws.String("volume-id"),
				Values: []*string{aws.String(volumeID)},
			},
		}
	}

	ec2SnapshotsResponse, err := c.listSnapshots(ctx, describeSnapshotsInput)
	if err != nil {
		return nil, err
	}
	var snapshots []*Snapshot
	for _, ec2Snapshot := range ec2SnapshotsResponse.Snapshots {
		snapshots = append(snapshots, c.ec2SnapshotResponseToStruct(ec2Snapshot))
	}

	if len(snapshots) == 0 {
		return nil, ErrNotFound
	}

	return &ListSnapshotsResponse{
		Snapshots: snapshots,
		NextToken: aws.StringValue(ec2SnapshotsResponse.NextToken),
	}, nil
}

// Helper method converting EC2 snapshot type to the internal struct
func (c *cloud) ec2SnapshotResponseToStruct(ec2Snapshot *ec2.Snapshot) *Snapshot {
	if ec2Snapshot == nil {
		return nil
	}
	snapshotSize := util.GiBToBytes(aws.Int64Value(ec2Snapshot.VolumeSize))
	snapshot := &Snapshot{
		SnapshotID:     aws.StringValue(ec2Snapshot.SnapshotId),
		SourceVolumeID: aws.StringValue(ec2Snapshot.VolumeId),
		Size:           snapshotSize,
		CreationTime:   aws.TimeValue(ec2Snapshot.StartTime),
	}
	if aws.StringValue(ec2Snapshot.State) == "completed" {
		snapshot.ReadyToUse = true
	} else {
		snapshot.ReadyToUse = false
	}

	return snapshot
}

func (c *cloud) getVolume(ctx context.Context, request *ec2.DescribeVolumesInput) (*ec2.Volume, error) {
	var volumes []*ec2.Volume
	var nextToken *string
	for {
		response, err := c.ec2.DescribeVolumesWithContext(ctx, request)
		if err != nil {
			return nil, err
		}
		volumes = append(volumes, response.Volumes...)
		nextToken = response.NextToken
		if aws.StringValue(nextToken) == "" {
			break
		}
		request.NextToken = nextToken
	}

	if l := len(volumes); l > 1 {
		return nil, ErrMultiDisks
	} else if l < 1 {
		return nil, ErrNotFound
	}

	return volumes[0], nil
}

func (c *cloud) getInstance(ctx context.Context, nodeID string) (*ec2.Instance, error) {
	instances := []*ec2.Instance{}
	request := &ec2.DescribeInstancesInput{
		InstanceIds: []*string{&nodeID},
	}

	var nextToken *string
	for {
		response, err := c.ec2.DescribeInstancesWithContext(ctx, request)
		if err != nil {
			if isAWSErrorInstanceNotFound(err) {
				return nil, ErrNotFound
			}
			return nil, fmt.Errorf("error listing AWS instances: %q", err)
		}

		for _, reservation := range response.Reservations {
			instances = append(instances, reservation.Instances...)
		}

		nextToken = response.NextToken
		if aws.StringValue(nextToken) == "" {
			break
		}
		request.NextToken = nextToken
	}

	if l := len(instances); l > 1 {
		return nil, fmt.Errorf("found %d instances with ID %q", l, nodeID)
	} else if l < 1 {
		return nil, ErrNotFound
	}

	return instances[0], nil
}

func (c *cloud) getSnapshot(ctx context.Context, request *ec2.DescribeSnapshotsInput) (*ec2.Snapshot, error) {
	var snapshots []*ec2.Snapshot
	var nextToken *string
	for {
		response, err := c.ec2.DescribeSnapshotsWithContext(ctx, request)
		if err != nil {
			return nil, err
		}
		snapshots = append(snapshots, response.Snapshots...)
		nextToken = response.NextToken
		if aws.StringValue(nextToken) == "" {
			break
		}
		request.NextToken = nextToken
	}

	if l := len(snapshots); l > 1 {
		return nil, ErrMultiSnapshots
	} else if l < 1 {
		return nil, ErrNotFound
	}

	return snapshots[0], nil
}

// listSnapshots returns all snapshots based from a request
func (c *cloud) listSnapshots(ctx context.Context, request *ec2.DescribeSnapshotsInput) (*ec2ListSnapshotsResponse, error) {
	var snapshots []*ec2.Snapshot
	var nextToken *string

	response, err := c.ec2.DescribeSnapshotsWithContext(ctx, request)
	if err != nil {
		return nil, err
	}

	snapshots = append(snapshots, response.Snapshots...)

	if response.NextToken != nil {
		nextToken = response.NextToken
	}

	return &ec2ListSnapshotsResponse{
		Snapshots: snapshots,
		NextToken: nextToken,
	}, nil
}

// waitForVolume waits for volume to be in the "available" state.
// On a random AWS account (shared among several developers) it took 4s on average.
func (c *cloud) waitForVolume(ctx context.Context, volumeID string) error {
	var (
		checkInterval = 3 * time.Second
		// This timeout can be "ovewritten" if the value returned by ctx.Deadline()
		// comes sooner. That value comes from the external provisioner controller.
		checkTimeout = 1 * time.Minute
	)

	request := &ec2.DescribeVolumesInput{
		VolumeIds: []*string{
			aws.String(volumeID),
		},
	}

	err := wait.Poll(checkInterval, checkTimeout, func() (done bool, err error) {
		vol, err := c.getVolume(ctx, request)
		if err != nil {
			return true, err
		}
		if vol.State != nil {
			return *vol.State == "available", nil
		}
		return false, nil
	})

	return err
}

// isAWSError returns a boolean indicating whether the error is AWS-related
// and has the given code. More information on AWS error codes at:
// https://docs.aws.amazon.com/AWSEC2/latest/APIReference/errors-overview.html
func isAWSError(err error, code string) bool {
	if awsError, ok := err.(awserr.Error); ok {
		if awsError.Code() == code {
			return true
		}
	}
	return false
}

// isAWSErrorInstanceNotFound returns a boolean indicating whether the
// given error is an AWS InvalidInstanceID.NotFound error. This error is
// reported when the specified instance doesn't exist.
func isAWSErrorInstanceNotFound(err error) bool {
	return isAWSError(err, "InvalidInstanceID.NotFound")
}

// isAWSErrorVolumeNotFound returns a boolean indicating whether the
// given error is an AWS InvalidVolume.NotFound error. This error is
// reported when the specified volume doesn't exist.
func isAWSErrorVolumeNotFound(err error) bool {
	return isAWSError(err, "InvalidVolume.NotFound")
}

// isAWSErrorIncorrectState returns a boolean indicating whether the
// given error is an AWS IncorrectState error. This error is
// reported when the resource is not in a correct state for the request.
func isAWSErrorIncorrectState(err error) bool {
	return isAWSError(err, "IncorrectState")
}

// isAWSErrorInvalidAttachmentNotFound returns a boolean indicating whether the
// given error is an AWS InvalidAttachment.NotFound error. This error is reported
// when attempting to detach a volume from an instance to which it is not attached.
func isAWSErrorInvalidAttachmentNotFound(err error) bool {
	return isAWSError(err, "InvalidAttachment.NotFound")
}

// isAWSErrorModificationNotFound returns a boolean indicating whether the given
// error is an AWS InvalidVolumeModification.NotFound error
func isAWSErrorModificationNotFound(err error) bool {
	return isAWSError(err, "InvalidVolumeModification.NotFound")
}

// isAWSErrorSnapshotNotFound returns a boolean indicating whether the
// given error is an AWS InvalidSnapshot.NotFound error. This error is
// reported when the specified snapshot doesn't exist.
func isAWSErrorSnapshotNotFound(err error) bool {
	return isAWSError(err, "InvalidSnapshot.NotFound")
}

// ResizeDisk resizes an EBS volume in GiB increments, rouding up to the next possible allocatable unit.
// It returns the volume size after this call or an error if the size couldn't be determined.
func (c *cloud) ResizeDisk(ctx context.Context, volumeID string, newSizeBytes int64) (int64, error) {
	request := &ec2.DescribeVolumesInput{
		VolumeIds: []*string{
			aws.String(volumeID),
		},
	}
	volume, err := c.getVolume(ctx, request)
	if err != nil {
		return 0, err
	}

	// AWS resizes in chunks of GiB (not GB)
	newSizeGiB := util.RoundUpGiB(newSizeBytes)
	oldSizeGiB := aws.Int64Value(volume.Size)

	latestMod, modFetchError := c.getLatestVolumeModification(ctx, volumeID)

	if latestMod != nil && modFetchError == nil {
		state := aws.StringValue(latestMod.ModificationState)
		if state == ec2.VolumeModificationStateModifying {
			_, err = c.waitForVolumeSize(ctx, volumeID)
			if err != nil {
				return oldSizeGiB, err
			}
			return c.checkDesiredSize(ctx, volumeID, newSizeGiB)
		}
	}

	// if there was an error fetching volume modifications and it was anything other than VolumeNotBeingModified error
	// that means we have an API problem.
	if modFetchError != nil && modFetchError != VolumeNotBeingModified {
		return oldSizeGiB, fmt.Errorf("error fetching volume modifications for %q: %v", volumeID, modFetchError)
	}

	// Even if existing volume size is greater than user requested size, we should ensure that there are no pending
	// volume modifications objects or volume has completed previously issued modification request.
	if oldSizeGiB >= newSizeGiB {
		klog.V(5).Infof("Volume %q current size (%d GiB) is greater or equal to the new size (%d GiB)", volumeID, oldSizeGiB, newSizeGiB)
		_, err = c.waitForVolumeSize(ctx, volumeID)
		if err != nil && err != VolumeNotBeingModified {
			return oldSizeGiB, err
		}
		return oldSizeGiB, nil
	}

	req := &ec2.ModifyVolumeInput{
		VolumeId: aws.String(volumeID),
		Size:     aws.Int64(newSizeGiB),
	}

	klog.Infof("expanding volume %q to size %d", volumeID, newSizeGiB)
	response, err := c.ec2.ModifyVolumeWithContext(ctx, req)
	if err != nil {
		return 0, fmt.Errorf("could not modify AWS volume %q: %v", volumeID, err)
	}

	mod := response.VolumeModification

	state := aws.StringValue(mod.ModificationState)
	if volumeModificationDone(state) {
		return c.checkDesiredSize(ctx, volumeID, newSizeGiB)
	}

	_, err = c.waitForVolumeSize(ctx, volumeID)
	if err != nil {
		return oldSizeGiB, err
	}
	return c.checkDesiredSize(ctx, volumeID, newSizeGiB)
}

// Checks for desired size on volume by also verifying volume size by describing volume.
// This is to get around potential eventual consistency problems with describing volume modifications
// objects and ensuring that we read two different objects to verify volume state.
func (c *cloud) checkDesiredSize(ctx context.Context, volumeID string, newSizeGiB int64) (int64, error) {
	request := &ec2.DescribeVolumesInput{
		VolumeIds: []*string{
			aws.String(volumeID),
		},
	}
	volume, err := c.getVolume(ctx, request)
	if err != nil {
		return 0, err
	}

	// AWS resizes in chunks of GiB (not GB)
	oldSizeGiB := aws.Int64Value(volume.Size)
	if oldSizeGiB >= newSizeGiB {
		return oldSizeGiB, nil
	}
	return oldSizeGiB, fmt.Errorf("volume %q is still being expanded to %d size", volumeID, newSizeGiB)
}

// waitForVolumeSize waits for a volume modification to finish and return its size.
func (c *cloud) waitForVolumeSize(ctx context.Context, volumeID string) (int64, error) {
	backoff := wait.Backoff{
		Duration: volumeModificationDuration,
		Factor:   volumeModificationWaitFactor,
		Steps:    volumeModificationWaitSteps,
	}

	var modVolSizeGiB int64
	waitErr := wait.ExponentialBackoff(backoff, func() (bool, error) {
		m, err := c.getLatestVolumeModification(ctx, volumeID)
		if err != nil {
			return false, err
		}

		state := aws.StringValue(m.ModificationState)
		if volumeModificationDone(state) {
			modVolSizeGiB = aws.Int64Value(m.TargetSize)
			return true, nil
		}

		return false, nil
	})

	if waitErr != nil {
		return 0, waitErr
	}

	return modVolSizeGiB, nil
}

// getLatestVolumeModification returns the last modification of the volume.
func (c *cloud) getLatestVolumeModification(ctx context.Context, volumeID string) (*ec2.VolumeModification, error) {
	request := &ec2.DescribeVolumesModificationsInput{
		VolumeIds: []*string{
			aws.String(volumeID),
		},
	}
	mod, err := c.ec2.DescribeVolumesModificationsWithContext(ctx, request)
	if err != nil {
		if isAWSErrorModificationNotFound(err) {
			return nil, VolumeNotBeingModified
		}
		return nil, fmt.Errorf("error describing modifications in volume %q: %v", volumeID, err)
	}

	volumeMods := mod.VolumesModifications
	if len(volumeMods) == 0 {
		return nil, VolumeNotBeingModified
	}

	return volumeMods[len(volumeMods)-1], nil
}

// randomAvailabilityZone returns a random zone from the given region
// the randomness relies on the response of DescribeAvailabilityZones
func (c *cloud) randomAvailabilityZone(ctx context.Context) (string, error) {
	request := &ec2.DescribeAvailabilityZonesInput{}
	response, err := c.ec2.DescribeAvailabilityZonesWithContext(ctx, request)
	if err != nil {
		return "", err
	}

	zones := []string{}
	for _, zone := range response.AvailabilityZones {
		zones = append(zones, *zone.ZoneName)
	}

	return zones[0], nil
}

func volumeModificationDone(state string) bool {
	if state == ec2.VolumeModificationStateCompleted || state == ec2.VolumeModificationStateOptimizing {
		return true
	}
	return false
}

func getVolumeAttachmentsList(volume *ec2.Volume) []string {
	var volumeAttachmentList []string
	for _, attachment := range volume.Attachments {
		if attachment.State != nil && strings.ToLower(aws.StringValue(attachment.State)) == "attached" {
			volumeAttachmentList = append(volumeAttachmentList, aws.StringValue(attachment.InstanceId))
		}
	}

	return volumeAttachmentList
}
