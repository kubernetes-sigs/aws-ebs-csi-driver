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
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/endpoints"
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/ec2/ec2iface"
	dm "github.com/kubernetes-sigs/aws-ebs-csi-driver/pkg/cloud/devicemanager"
	"github.com/kubernetes-sigs/aws-ebs-csi-driver/pkg/util"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/klog/v2"
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
	// VolumeTypeSBG1 represents a capacity-optimized HDD type of volume. Only for SBE devices.
	VolumeTypeSBG1 = "sbg1"
	// VolumeTypeSBP1 represents a performance-optimized SSD type of volume. Only for SBE devices.
	VolumeTypeSBP1 = "sbp1"
	// VolumeTypeStandard represents a previous type of  volume.
	VolumeTypeStandard = "standard"
)

// AWS provisioning limits.
// Source: http://docs.aws.amazon.com/AWSEC2/latest/UserGuide/EBSVolumeTypes.html
const (
	io1MinTotalIOPS             = 100
	io1MaxTotalIOPS             = 64000
	io1MaxIOPSPerGB             = 50
	io2MinTotalIOPS             = 100
	io2MaxTotalIOPS             = 64000
	io2BlockExpressMaxTotalIOPS = 256000
	io2MaxIOPSPerGB             = 500
	gp3MaxTotalIOPS             = 16000
	gp3MinTotalIOPS             = 3000
	gp3MaxIOPSPerGB             = 500
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

	volumeAttachmentStatePollSteps = 13
)

const (
	volumeAttachmentStatePollDelay  = 1 * time.Second
	volumeAttachmentStatePollFactor = 1.8

	volumeDetachedState = "detached"
	volumeAttachedState = "attached"
)

// AWS provisioning limits.
// Source:
//
//	https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/Using_Tags.html#tag-restrictions
const (
	// MaxNumTagsPerResource represents the maximum number of tags per AWS resource.
	MaxNumTagsPerResource = 50
	// MinTagKeyLength represents the minimum key length for a tag.
	MinTagKeyLength = 1
	// MaxTagKeyLength represents the maximum key length for a tag.
	MaxTagKeyLength = 128
	// MaxTagValueLength represents the maximum value length for a tag.
	MaxTagValueLength = 256
)

// Defaults
const (
	// DefaultVolumeSize represents the default volume size.
	DefaultVolumeSize int64 = 100 * util.GiB
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
	//AwsEbsDriverTagKey is the tag to identify if a volume/snapshot is managed by ebs csi driver
	AwsEbsDriverTagKey = "ebs.csi.aws.com/cluster"
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

	// ErrIdempotent is returned when another request with same idempotent token is in-flight.
	ErrIdempotentParameterMismatch = errors.New("Parameters on this idempotent request are inconsistent with parameters used in previous request(s)")

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

// Set during build time via -ldflags
var driverVersion string

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
	CapacityBytes          int64
	Tags                   map[string]string
	VolumeType             string
	IOPSPerGB              int
	AllowIOPSPerGBIncrease bool
	IOPS                   int
	Throughput             int
	AvailabilityZone       string
	OutpostArn             string
	Encrypted              bool
	BlockExpress           bool
	MultiAttachEnabled     bool
	// KmsKeyID represents a fully qualified resource name to the key to use for encryption.
	// example: arn:aws:kms:us-east-1:012345678910:key/abcd1234-a123-456a-a12b-a123b4cd56ef
	KmsKeyID   string
	SnapshotID string
}

// ModifyDiskOptions represents parameters to modify an EBS volume
type ModifyDiskOptions struct {
	VolumeType string
	IOPS       int
	Throughput int
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

type cloud struct {
	region string
	ec2    ec2iface.EC2API
	dm     dm.DeviceManager
}

var _ Cloud = &cloud{}

// NewCloud returns a new instance of AWS cloud
// It panics if session is invalid
func NewCloud(region string, awsSdkDebugLog bool, userAgentExtra string) (Cloud, error) {
	return newEC2Cloud(region, awsSdkDebugLog, userAgentExtra)
}

func newEC2Cloud(region string, awsSdkDebugLog bool, userAgentExtra string) (Cloud, error) {
	awsConfig := &aws.Config{
		Region:                        aws.String(region),
		CredentialsChainVerboseErrors: aws.Bool(true),
		// Set MaxRetries to a high value. It will be "ovewritten" if context deadline comes sooner.
		MaxRetries: aws.Int(8),
	}

	endpoint := os.Getenv("AWS_EC2_ENDPOINT")
	if endpoint != "" {
		customResolver := func(service, region string, optFns ...func(*endpoints.Options)) (endpoints.ResolvedEndpoint, error) {
			if service == ec2.EndpointsID {
				return endpoints.ResolvedEndpoint{
					URL:           endpoint,
					SigningRegion: region,
				}, nil
			}
			return endpoints.DefaultResolver().EndpointFor(service, region, optFns...)
		}
		awsConfig.EndpointResolver = endpoints.ResolverFunc(customResolver)
	}

	if awsSdkDebugLog {
		awsConfig.WithLogLevel(aws.LogDebugWithRequestErrors)
	}

	// Set the env var so that the session appends custom user agent string
	if userAgentExtra != "" {
		os.Setenv("AWS_EXECUTION_ENV", "aws-ebs-csi-driver-"+driverVersion+"-"+userAgentExtra)
	} else {
		os.Setenv("AWS_EXECUTION_ENV", "aws-ebs-csi-driver-"+driverVersion)
	}

	svc := ec2.New(session.Must(session.NewSession(awsConfig)))
	svc.Handlers.AfterRetry.PushFrontNamed(request.NamedHandler{
		Name: "recordThrottledRequestsHandler",
		Fn:   RecordThrottledRequestsHandler,
	})
	svc.Handlers.Complete.PushFrontNamed(request.NamedHandler{
		Name: "recordRequestsHandler",
		Fn:   RecordRequestsHandler,
	})

	return &cloud{
		region: region,
		dm:     dm.NewDeviceManager(),
		ec2:    svc,
	}, nil
}

func (c *cloud) CreateDisk(ctx context.Context, volumeName string, diskOptions *DiskOptions) (*Disk, error) {
	var (
		createType    string
		iops          int64
		throughput    int64
		err           error
		maxIops       int64
		minIops       int64
		maxIopsPerGb  int64
		requestedIops int64
	)

	capacityGiB := util.BytesToGiB(diskOptions.CapacityBytes)

	if diskOptions.IOPS > 0 && diskOptions.IOPSPerGB > 0 {
		return nil, fmt.Errorf("invalid StorageClass parameters; specify either IOPS or IOPSPerGb, not both")
	}

	createType = diskOptions.VolumeType
	// If no volume type is specified, GP3 is used as default for newly created volumes.
	if createType == "" {
		createType = VolumeTypeGP3
	}

	switch createType {
	case VolumeTypeGP2, VolumeTypeSC1, VolumeTypeST1, VolumeTypeSBG1, VolumeTypeSBP1, VolumeTypeStandard:
	case VolumeTypeIO1:
		maxIops = io1MaxTotalIOPS
		minIops = io1MinTotalIOPS
		maxIopsPerGb = io1MaxIOPSPerGB
	case VolumeTypeIO2:
		if diskOptions.BlockExpress {
			maxIops = io2BlockExpressMaxTotalIOPS
		} else {
			maxIops = io2MaxTotalIOPS
		}
		minIops = io2MinTotalIOPS
		maxIopsPerGb = io2MaxIOPSPerGB
	case VolumeTypeGP3:
		maxIops = gp3MaxTotalIOPS
		minIops = gp3MinTotalIOPS
		maxIopsPerGb = gp3MaxIOPSPerGB
		throughput = int64(diskOptions.Throughput)
	default:
		return nil, fmt.Errorf("invalid AWS VolumeType %q", diskOptions.VolumeType)
	}

	if diskOptions.MultiAttachEnabled && createType != VolumeTypeIO2 {
		return nil, fmt.Errorf("CreateDisk: multi-attach is only supported for io2 volumes")
	}

	if maxIops > 0 {
		if diskOptions.IOPS > 0 {
			requestedIops = int64(diskOptions.IOPS)
		} else if diskOptions.IOPSPerGB > 0 {
			requestedIops = int64(diskOptions.IOPSPerGB) * capacityGiB
		}
		iops, err = capIOPS(createType, capacityGiB, requestedIops, minIops, maxIops, maxIopsPerGb, diskOptions.AllowIOPSPerGBIncrease)
		if err != nil {
			return nil, err
		}
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
		zone, err = c.randomAvailabilityZone(ctx)
		klog.V(5).InfoS("[Debug] AZ is not provided. Using node AZ", "zone", zone)
		if err != nil {
			return nil, fmt.Errorf("failed to get availability zone %w", err)
		}
	}

	// We hash the volume name to generate a unique token that is less than or equal to 64 characters
	clientToken := sha256.Sum256([]byte(volumeName))

	requestInput := &ec2.CreateVolumeInput{
		AvailabilityZone:   aws.String(zone),
		ClientToken:        aws.String(hex.EncodeToString(clientToken[:])),
		Size:               aws.Int64(capacityGiB),
		VolumeType:         aws.String(createType),
		Encrypted:          aws.Bool(diskOptions.Encrypted),
		MultiAttachEnabled: aws.Bool(diskOptions.MultiAttachEnabled),
	}

	if !util.IsSBE(zone) {
		requestInput.TagSpecifications = []*ec2.TagSpecification{&tagSpec}
	}

	// EBS doesn't handle empty outpost arn, so we have to include it only when it's non-empty
	if len(diskOptions.OutpostArn) > 0 {
		requestInput.OutpostArn = aws.String(diskOptions.OutpostArn)
	}

	if len(diskOptions.KmsKeyID) > 0 {
		requestInput.KmsKeyId = aws.String(diskOptions.KmsKeyID)
		requestInput.Encrypted = aws.Bool(true)
	}
	if iops > 0 {
		requestInput.Iops = aws.Int64(iops)
	}
	if throughput > 0 {
		requestInput.Throughput = aws.Int64(throughput)
	}
	snapshotID := diskOptions.SnapshotID
	if len(snapshotID) > 0 {
		requestInput.SnapshotId = aws.String(snapshotID)
	}

	response, err := c.ec2.CreateVolumeWithContext(ctx, requestInput)
	if err != nil {
		if isAWSErrorSnapshotNotFound(err) {
			return nil, ErrNotFound
		}
		if isAWSErrorIdempotentParameterMismatch(err) {
			return nil, ErrIdempotentParameterMismatch
		}
		return nil, fmt.Errorf("could not create volume in EC2: %w", err)
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
		// To avoid leaking volume, we should delete the volume just created
		// TODO: Need to figure out how to handle DeleteDisk failed scenario instead of just log the error
		if _, error := c.DeleteDisk(ctx, volumeID); error != nil {
			klog.ErrorS(error, "failed to be deleted, this may cause volume leak", "volumeID", volumeID)
		} else {
			klog.V(5).InfoS("[Debug] volume is deleted because it is not in desired state within retry limit", "volumeID", volumeID)
		}
		return nil, fmt.Errorf("failed to get an available volume in EC2: %w", err)
	}

	outpostArn := aws.StringValue(response.OutpostArn)
	var resources []*string
	if util.IsSBE(zone) {
		requestTagsInput := &ec2.CreateTagsInput{
			Resources: append(resources, &volumeID),
			Tags:      tags,
		}
		_, err := c.ec2.CreateTagsWithContext(ctx, requestTagsInput)
		if err != nil {
			// To avoid leaking volume, we should delete the volume just created
			// TODO: Need to figure out how to handle DeleteDisk failed scenario instead of just log the error
			if _, error := c.DeleteDisk(ctx, volumeID); err != nil {
				klog.ErrorS(error, "failed to be deleted, this may cause volume leak", "volumeID", volumeID)
			} else {
				klog.V(5).InfoS("volume is deleted because there was an error while attaching the tags", "volumeID", volumeID)
			}
			return nil, fmt.Errorf("could not attach tags to volume: %v. %w", volumeID, err)
		}
	}
	return &Disk{CapacityGiB: size, VolumeID: volumeID, AvailabilityZone: zone, SnapshotID: snapshotID, OutpostArn: outpostArn}, nil
}

// ResizeOrModifyDisk resizes an EBS volume in GiB increments, rouding up to the next possible allocatable unit, and/or modifies an EBS
// volume with the parameters in ModifyDiskOptions.
// The resizing operation is performed only when newSizeBytes != 0.
// It returns the volume size after this call or an error if the size couldn't be determined or the volume couldn't be modified.
func (c *cloud) ResizeOrModifyDisk(ctx context.Context, volumeID string, newSizeBytes int64, options *ModifyDiskOptions) (int64, error) {
	if newSizeBytes != 0 {
		klog.V(4).InfoS("Received Resize and/or Modify Disk request", "volumeID", volumeID, "newSizeBytes", newSizeBytes, "options", options)
	} else {
		klog.V(4).InfoS("Received Modify Disk request", "volumeID", volumeID, "options", options)
	}

	newSizeGiB := util.RoundUpGiB(newSizeBytes)
	needsModification, volumeSize, err := c.validateModifyVolume(ctx, volumeID, newSizeGiB, options)
	if err != nil || !needsModification {
		return volumeSize, err
	}

	req := &ec2.ModifyVolumeInput{
		VolumeId: aws.String(volumeID),
	}
	if newSizeBytes != 0 {
		req.Size = aws.Int64(newSizeGiB)
	}
	if options.IOPS != 0 {
		req.Iops = aws.Int64(int64(options.IOPS))
	}
	if options.VolumeType != "" {
		req.VolumeType = aws.String(options.VolumeType)
	}
	if options.Throughput != 0 {
		req.Throughput = aws.Int64(int64(options.Throughput))
	}

	response, err := c.ec2.ModifyVolumeWithContext(ctx, req)
	if err != nil {
		return 0, fmt.Errorf("unable to modify AWS volume %q: %w", volumeID, err)
	}

	// If the volume modification isn't immediately completed, wait for it to finish
	state := aws.StringValue(response.VolumeModification.ModificationState)
	if !volumeModificationDone(state) {
		err = c.waitForVolumeModification(ctx, volumeID)
		if err != nil {
			return 0, err
		}
	}

	// Perform one final check on the volume
	return c.checkDesiredState(ctx, volumeID, newSizeGiB, options)
}

func (c *cloud) DeleteDisk(ctx context.Context, volumeID string) (bool, error) {
	request := &ec2.DeleteVolumeInput{VolumeId: &volumeID}
	if _, err := c.ec2.DeleteVolumeWithContext(ctx, request); err != nil {
		if isAWSErrorVolumeNotFound(err) {
			return false, ErrNotFound
		}
		return false, fmt.Errorf("DeleteDisk could not delete volume: %w", err)
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

		resp, attachErr := c.ec2.AttachVolumeWithContext(ctx, request)
		if attachErr != nil {
			return "", fmt.Errorf("could not attach volume %q to node %q: %w", volumeID, nodeID, attachErr)
		}
		klog.V(5).InfoS("[Debug] AttachVolume", "volumeID", volumeID, "nodeID", nodeID, "resp", resp)
	}

	_, err = c.WaitForAttachmentState(ctx, volumeID, volumeAttachedState, *instance.InstanceId, device.Path, device.IsAlreadyAssigned)

	// This is the only situation where we taint the device
	if err != nil {
		device.Taint()
		return "", err
	}

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
		klog.InfoS("DetachDisk: called on non-attached volume", "volumeID", volumeID)
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
		return fmt.Errorf("could not detach volume %q from node %q: %w", volumeID, nodeID, err)
	}

	attachment, err := c.WaitForAttachmentState(ctx, volumeID, volumeDetachedState, *instance.InstanceId, "", false)
	if err != nil {
		return err
	}
	if attachment != nil {
		// We expect it to be nil, it is (maybe) interesting if it is not
		klog.V(2).InfoS("waitForAttachmentState returned non-nil attachment with state=detached", "attachment", attachment)
	}

	return nil
}

// WaitForAttachmentState polls until the attachment status is the expected value.
func (c *cloud) WaitForAttachmentState(ctx context.Context, volumeID, expectedState string, expectedInstance string, expectedDevice string, alreadyAssigned bool) (*ec2.VolumeAttachment, error) {
	// Most attach/detach operations on AWS finish within 1-4 seconds.
	// By using 1 second starting interval with a backoff of 1.8,
	// we get [1, 1.8, 3.24, 5.832000000000001, 10.4976].
	// In total we wait for 2601 seconds.
	backoff := wait.Backoff{
		Duration: volumeAttachmentStatePollDelay,
		Factor:   volumeAttachmentStatePollFactor,
		Steps:    volumeAttachmentStatePollSteps,
	}

	var attachment *ec2.VolumeAttachment

	verifyVolumeFunc := func(ctx context.Context) (bool, error) {
		request := &ec2.DescribeVolumesInput{
			VolumeIds: []*string{
				aws.String(volumeID),
			},
		}

		volume, err := c.getVolume(ctx, request)
		if err != nil {
			// The VolumeNotFound error is special -- we don't need to wait for it to repeat
			if isAWSErrorVolumeNotFound(err) {
				if expectedState == volumeDetachedState {
					// The disk doesn't exist, assume it's detached, log warning and stop waiting
					klog.InfoS("Waiting for volume to be detached but the volume does not exist", "volumeID", volumeID)
					return true, nil
				}
				if expectedState == volumeAttachedState {
					// The disk doesn't exist, complain, give up waiting and report error
					klog.InfoS("Waiting for volume to be attached but the volume does not exist", "volumeID", volumeID)
					return false, err
				}
			}

			klog.InfoS("Ignoring error from describe volume, will retry", "volumeID", volumeID, "err", err)
			return false, nil
		}

		if volume.MultiAttachEnabled != nil && !*volume.MultiAttachEnabled && len(volume.Attachments) > 1 {
			klog.InfoS("Found multiple attachments for volume", "volumeID", volumeID, "volume", volume)
			return false, fmt.Errorf("volume %q has multiple attachments", volumeID)
		}

		attachmentState := ""

		for _, a := range volume.Attachments {
			if a.State != nil && a.InstanceId != nil {
				if aws.StringValue(a.InstanceId) == expectedInstance {
					attachmentState = aws.StringValue(a.State)
					attachment = a
				}
			}
		}

		if attachmentState == "" {
			attachmentState = volumeDetachedState
		}

		if attachment != nil && attachment.Device != nil && expectedState == volumeAttachedState {
			device := aws.StringValue(attachment.Device)
			if device != expectedDevice {
				klog.InfoS("WaitForAttachmentState: device mismatch", "device", device, "expectedDevice", expectedDevice, "attachment", attachment)
				return false, nil
			}
		}

		// if we expected volume to be attached and it was reported as already attached via DescribeInstance call
		// but DescribeVolume told us volume is detached, we will short-circuit this long wait loop and return error
		// so as AttachDisk can be retried without waiting for 20 minutes.
		if (expectedState == volumeAttachedState) && alreadyAssigned && (attachmentState != expectedState) {
			return false, fmt.Errorf("attachment of disk %q failed, expected device to be attached but was %s", volumeID, attachmentState)
		}

		// Attachment is in requested state, finish waiting
		if attachmentState == expectedState {
			// But first, reset attachment to nil if expectedState equals volumeDetachedState.
			// Caller will not expect an attachment to be returned for a detached volume if we're not also returning an error.
			if expectedState == volumeDetachedState {
				attachment = nil
			}
			return true, nil
		}
		// continue waiting
		klog.V(4).InfoS("Waiting for volume state", "volumeID", volumeID, "actual", attachmentState, "desired", expectedState)
		return false, nil
	}

	return attachment, wait.ExponentialBackoffWithContext(ctx, backoff, verifyVolumeFunc)
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
		return nil, fmt.Errorf("error creating snapshot of volume %s: %w", volumeID, err)
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
		return false, fmt.Errorf("DeleteSnapshot could not delete volume: %w", err)
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

func (c *cloud) EnableFastSnapshotRestores(ctx context.Context, availabilityZones []string, snapshotID string) (*ec2.EnableFastSnapshotRestoresOutput, error) {
	request := &ec2.EnableFastSnapshotRestoresInput{
		AvailabilityZones: aws.StringSlice(availabilityZones),
		SourceSnapshotIds: []*string{
			aws.String(snapshotID),
		},
	}
	klog.V(4).InfoS("Creating Fast Snapshot Restores", "snapshotID", snapshotID, "availabilityZones", availabilityZones)
	response, err := c.ec2.EnableFastSnapshotRestoresWithContext(ctx, request)
	if err != nil {
		return nil, err
	}
	if len(response.Unsuccessful) > 0 {
		return response, fmt.Errorf("failed to create fast snapshot restores for snapshot %s: %v", snapshotID, response.Unsuccessful)
	}
	return response, nil
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
			return nil, fmt.Errorf("error listing AWS instances: %w", err)
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

	err := wait.PollUntilContextTimeout(ctx, checkInterval, checkTimeout, false, func(ctx context.Context) (done bool, err error) {
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
	var awsErr awserr.Error
	if errors.As(err, &awsErr) {
		if awsErr.Code() == code {
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

// isAWSErrorIdempotentParameterMismatch returns a boolean indicating whether the
// given error is an AWS IdempotentParameterMismatch error.
// This error is reported when the two request contains same client-token but different parameters
func isAWSErrorIdempotentParameterMismatch(err error) bool {
	return isAWSError(err, "IdempotentParameterMismatch")
}

// Checks for desired size on volume by also verifying volume size by describing volume.
// This is to get around potential eventual consistency problems with describing volume modifications
// objects and ensuring that we read two different objects to verify volume state.
func (c *cloud) checkDesiredState(ctx context.Context, volumeID string, desiredSizeGiB int64, options *ModifyDiskOptions) (int64, error) {
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
	realSizeGiB := aws.Int64Value(volume.Size)

	// Check if there is a mismatch between the requested modification and the current volume
	// If there is, the volume is still modifying and we should not return a success
	if realSizeGiB < desiredSizeGiB {
		return realSizeGiB, fmt.Errorf("volume %q is still being expanded to %d size", volumeID, desiredSizeGiB)
	} else if options.IOPS != 0 && (volume.Iops == nil || *volume.Iops != int64(options.IOPS)) {
		return realSizeGiB, fmt.Errorf("volume %q is still being modified to iops %d", volumeID, options.IOPS)
	} else if options.VolumeType != "" && !strings.EqualFold(*volume.VolumeType, options.VolumeType) {
		return realSizeGiB, fmt.Errorf("volume %q is still being modified to type %q", volumeID, options.VolumeType)
	} else if options.Throughput != 0 && (volume.Throughput == nil || *volume.Throughput != int64(options.Throughput)) {
		return realSizeGiB, fmt.Errorf("volume %q is still being modified to throughput %d", volumeID, options.Throughput)
	}

	return realSizeGiB, nil
}

// waitForVolumeModification waits for a volume modification to finish.
func (c *cloud) waitForVolumeModification(ctx context.Context, volumeID string) error {
	backoff := wait.Backoff{
		Duration: volumeModificationDuration,
		Factor:   volumeModificationWaitFactor,
		Steps:    volumeModificationWaitSteps,
	}

	waitErr := wait.ExponentialBackoff(backoff, func() (bool, error) {
		m, err := c.getLatestVolumeModification(ctx, volumeID)
		// Consider volumes that have never been modified as done
		if err != nil && errors.Is(err, VolumeNotBeingModified) {
			return true, nil
		} else if err != nil {
			return false, err
		}

		state := aws.StringValue(m.ModificationState)
		if volumeModificationDone(state) {
			return true, nil
		}

		return false, nil
	})

	if waitErr != nil {
		return waitErr
	}

	return nil
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
		return nil, fmt.Errorf("error describing modifications in volume %q: %w", volumeID, err)
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

// AvailabilityZones returns availability zones from the given region
func (c *cloud) AvailabilityZones(ctx context.Context) (map[string]struct{}, error) {
	response, err := c.ec2.DescribeAvailabilityZonesWithContext(ctx, &ec2.DescribeAvailabilityZonesInput{})
	if err != nil {
		return nil, fmt.Errorf("error describing availability zones: %w", err)
	}
	zones := make(map[string]struct{})
	for _, zone := range response.AvailabilityZones {
		zones[*zone.ZoneName] = struct{}{}
	}
	return zones, nil
}

func needsVolumeModification(volume *ec2.Volume, newSizeGiB int64, options *ModifyDiskOptions) bool {
	oldSizeGiB := aws.Int64Value(volume.Size)
	needsModification := false

	if oldSizeGiB < newSizeGiB {
		needsModification = true
	}
	if options.IOPS != 0 && (volume.Iops == nil || *volume.Iops != int64(options.IOPS)) {
		needsModification = true
	}
	if options.VolumeType != "" && !strings.EqualFold(*volume.VolumeType, options.VolumeType) {
		needsModification = true
	}
	if options.Throughput != 0 && (volume.Throughput == nil || *volume.Throughput != int64(options.Throughput)) {
		needsModification = true
	}

	return needsModification
}

func (c *cloud) validateModifyVolume(ctx context.Context, volumeID string, newSizeGiB int64, options *ModifyDiskOptions) (bool, int64, error) {
	request := &ec2.DescribeVolumesInput{
		VolumeIds: []*string{
			aws.String(volumeID),
		},
	}
	volume, err := c.getVolume(ctx, request)
	if err != nil {
		return true, 0, err
	}

	oldSizeGiB := aws.Int64Value(volume.Size)

	latestMod, err := c.getLatestVolumeModification(ctx, volumeID)
	if err != nil && !errors.Is(err, VolumeNotBeingModified) {
		return true, oldSizeGiB, fmt.Errorf("error fetching volume modifications for %q: %w", volumeID, err)
	}

	// latestMod can be nil if the volume has never been modified
	if latestMod != nil {
		state := aws.StringValue(latestMod.ModificationState)
		if state == ec2.VolumeModificationStateModifying {
			// If volume is already modifying, detour to waiting for it to modify
			klog.V(5).InfoS("[Debug] Watching ongoing modification", "volumeID", volumeID)
			err = c.waitForVolumeModification(ctx, volumeID)
			if err != nil {
				return true, oldSizeGiB, err
			}
			returnGiB, returnErr := c.checkDesiredState(ctx, volumeID, newSizeGiB, options)
			return false, returnGiB, returnErr
		} else if state == ec2.VolumeModificationStateOptimizing {
			return true, 0, fmt.Errorf("volume %q in OPTIMIZING state, cannot currently modify", volumeID)
		}
	}

	// At this point, we know we are starting a new volume modification
	// If we're asked to modify a volume to its current state, ignore the request and immediately return a success
	if !needsVolumeModification(volume, newSizeGiB, options) {
		klog.V(5).InfoS("[Debug] Skipping modification for volume due to matching stats", "volumeID", volumeID)
		// Wait for any existing modifications to prevent race conditions where DescribeVolume(s) returns the new
		// state before the volume is actually finished modifying
		err = c.waitForVolumeModification(ctx, volumeID)
		if err != nil {
			return true, oldSizeGiB, err
		}
		returnGiB, returnErr := c.checkDesiredState(ctx, volumeID, newSizeGiB, options)
		return false, returnGiB, returnErr
	}

	return true, 0, nil
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
		if attachment.State != nil && strings.ToLower(aws.StringValue(attachment.State)) == volumeAttachedState {
			volumeAttachmentList = append(volumeAttachmentList, aws.StringValue(attachment.InstanceId))
		}
	}

	return volumeAttachmentList
}

// Calculate actual IOPS for a volume and cap it at supported AWS limits.
func capIOPS(volumeType string, requestedCapacityGiB int64, requestedIops int64, minTotalIOPS, maxTotalIOPS, maxIOPSPerGB int64, allowIncrease bool) (int64, error) {
	// If requestedIops is zero the user did not request a specific amount, and the default will be used instead
	if requestedIops == 0 {
		return 0, nil
	}

	iops := requestedIops

	if iops < minTotalIOPS {
		if allowIncrease {
			iops = minTotalIOPS
			klog.V(5).InfoS("[Debug] Increased IOPS to the min supported limit", "volumeType", volumeType, "requestedCapacityGiB", requestedCapacityGiB, "limit", iops)
		} else {
			return 0, fmt.Errorf("invalid IOPS: %d is too low, it must be at least %d", iops, minTotalIOPS)
		}
	}
	if iops > maxTotalIOPS {
		iops = maxTotalIOPS
		klog.V(5).InfoS("[Debug] Capped IOPS, volume at the max supported limit", "volumeType", volumeType, "requestedCapacityGiB", requestedCapacityGiB, "limit", iops)
	}
	maxIopsByCapacity := maxIOPSPerGB * requestedCapacityGiB
	if iops > maxIopsByCapacity && maxIopsByCapacity >= minTotalIOPS {
		iops = maxIopsByCapacity
		klog.V(5).InfoS("[Debug] Capped IOPS for volume", "volumeType", volumeType, "requestedCapacityGiB", requestedCapacityGiB, "maxIOPSPerGB", maxIOPSPerGB, "limit", iops)
	}
	return iops, nil
}
