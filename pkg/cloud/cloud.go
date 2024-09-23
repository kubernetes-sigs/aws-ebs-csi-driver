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
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/smithy-go"
	"github.com/kubernetes-sigs/aws-ebs-csi-driver/pkg/batcher"
	dm "github.com/kubernetes-sigs/aws-ebs-csi-driver/pkg/cloud/devicemanager"
	"github.com/kubernetes-sigs/aws-ebs-csi-driver/pkg/expiringcache"
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
)

const (
	volumeDetachedState = "detached"
	volumeAttachedState = "attached"
	cacheForgetDelay    = 1 * time.Hour
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

// Batcher
const (
	volumeIDBatcher volumeBatcherType = iota
	volumeTagBatcher

	snapshotIDBatcher snapshotBatcherType = iota
	snapshotTagBatcher

	batchDescribeTimeout = 30 * time.Second
	batchMaxDelay        = 500 * time.Millisecond // Minimizes RPC latency and EC2 API calls. Tuned via scalability tests.
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

	// ErrMultiSnapshots is returned when multiple snapshots are found
	// with the same ID
	ErrMultiSnapshots = errors.New("Multiple snapshots with the same name found")

	// ErrInvalidMaxResults is returned when a MaxResults pagination parameter is between 1 and 4
	ErrInvalidMaxResults = errors.New("MaxResults parameter must be 0 or greater than or equal to 5")

	// VolumeNotBeingModified is returned if volume being described is not being modified
	VolumeNotBeingModified = fmt.Errorf("volume is not being modified")

	// ErrInvalidArgument is returned if parameters were rejected by cloud provider
	ErrInvalidArgument = errors.New("invalid argument")

	// ErrInvalidRequest is returned if parameters were rejected by driver
	ErrInvalidRequest = errors.New("invalid request")
)

// Set during build time via -ldflags
var driverVersion string

var invalidParameterErrorCodes = map[string]struct{}{
	"InvalidParameter":            {},
	"InvalidParameterCombination": {},
	"InvalidParameterDependency":  {},
	"InvalidParameterValue":       {},
	"UnknownParameter":            {},
	"UnknownVolumeType":           {},
	"UnsupportedOperation":        {},
	"ValidationError":             {},
}

// Disk represents a EBS volume
type Disk struct {
	VolumeID         string
	CapacityGiB      int32
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
	IOPSPerGB              int32
	AllowIOPSPerGBIncrease bool
	IOPS                   int32
	Throughput             int32
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
	IOPS       int32
	Throughput int32
}

// ModifyTagsOptions represents parameter to modify the tags of an existing EBS volume
type ModifyTagsOptions struct {
	TagsToAdd    map[string]string
	TagsToDelete []string
}

// Snapshot represents an EBS volume snapshot
type Snapshot struct {
	SnapshotID     string
	SourceVolumeID string
	Size           int32
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
	Tags       map[string]string
	OutpostArn string
}

// ec2ListSnapshotsResponse is a helper struct returned from the AWS API calling function to the main ListSnapshots function
type ec2ListSnapshotsResponse struct {
	Snapshots []types.Snapshot
	NextToken *string
}

// volumeWaitParameters dictates how to poll for volume events.
// E.g. how often to check if volume is created after an EC2 CreateVolume call.
type volumeWaitParameters struct {
	creationInitialDelay time.Duration
	creationBackoff      wait.Backoff
	modificationBackoff  wait.Backoff
	attachmentBackoff    wait.Backoff
}

var (
	vwp = volumeWaitParameters{
		// Based on our testing in us-west-2 and ap-south-1, the median/p99 time until volume creation is ~1.5/~4 seconds.
		// We have found that the following parameters are optimal for minimizing provisioning time and DescribeVolumes calls
		// we queue DescribeVolume calls after [1.25, 0.5, 0.75, 1.125, 1.7, 2.5, 3] seconds.
		// In total, we wait for ~60 seconds.
		creationInitialDelay: 1250 * time.Millisecond,
		creationBackoff: wait.Backoff{
			Duration: 500 * time.Millisecond,
			Factor:   1.5,
			Steps:    25,
			Cap:      3 * time.Second,
		},

		// Most attach/detach operations on AWS finish within 1-4 seconds.
		// By using 1 second starting interval with a backoff of 1.8,
		// we get [1, 1.8, 3.24, 5.832000000000001, 10.4976].
		// In total, we wait for 2601 seconds.
		attachmentBackoff: wait.Backoff{
			Duration: 1 * time.Second,
			Factor:   1.8,
			Steps:    13,
		},

		modificationBackoff: wait.Backoff{
			Duration: 1 * time.Second,
			Factor:   1.7,
			Steps:    10,
		},
	}
)

// volumeBatcherType is an enum representing the types of volume batchers available.
type volumeBatcherType int

// snapshotBatcherType is an enum representing the types of snapshot batchers available.
type snapshotBatcherType int

// batcherManager maintains a collection of batchers for different types of tasks.
type batcherManager struct {
	volumeIDBatcher             *batcher.Batcher[string, *types.Volume]
	volumeTagBatcher            *batcher.Batcher[string, *types.Volume]
	instanceIDBatcher           *batcher.Batcher[string, *types.Instance]
	snapshotIDBatcher           *batcher.Batcher[string, *types.Snapshot]
	snapshotTagBatcher          *batcher.Batcher[string, *types.Snapshot]
	volumeModificationIDBatcher *batcher.Batcher[string, *types.VolumeModification]
}

type cloud struct {
	region               string
	ec2                  EC2API
	dm                   dm.DeviceManager
	bm                   *batcherManager
	rm                   *retryManager
	vwp                  volumeWaitParameters
	likelyBadDeviceNames expiringcache.ExpiringCache[string, sync.Map]
	latestClientTokens   expiringcache.ExpiringCache[string, int]
}

var _ Cloud = &cloud{}

// NewCloud returns a new instance of AWS cloud
// It panics if session is invalid
func NewCloud(region string, awsSdkDebugLog bool, userAgentExtra string, batching bool) (Cloud, error) {
	c := newEC2Cloud(region, awsSdkDebugLog, userAgentExtra, batching)
	return c, nil
}

func newEC2Cloud(region string, awsSdkDebugLog bool, userAgentExtra string, batchingEnabled bool) Cloud {
	cfg, err := config.LoadDefaultConfig(context.Background(), config.WithRegion(region))
	if err != nil {
		panic(err)
	}

	if awsSdkDebugLog {
		cfg.ClientLogMode = aws.LogRequestWithBody | aws.LogResponseWithBody
	}

	// Set the env var so that the session appends custom user agent string
	if userAgentExtra != "" {
		os.Setenv("AWS_EXECUTION_ENV", "aws-ebs-csi-driver-"+driverVersion+"-"+userAgentExtra)
	} else {
		os.Setenv("AWS_EXECUTION_ENV", "aws-ebs-csi-driver-"+driverVersion)
	}

	svc := ec2.NewFromConfig(cfg, func(o *ec2.Options) {
		o.APIOptions = append(o.APIOptions,
			RecordRequestsMiddleware(),
		)

		endpoint := os.Getenv("AWS_EC2_ENDPOINT")
		if endpoint != "" {
			o.BaseEndpoint = &endpoint
		}

		o.RetryMaxAttempts = retryMaxAttempt
	})

	var bm *batcherManager
	if batchingEnabled {
		klog.V(4).InfoS("newEC2Cloud: batching enabled")
		bm = newBatcherManager(svc)
	}

	return &cloud{
		region:               region,
		dm:                   dm.NewDeviceManager(),
		ec2:                  svc,
		bm:                   bm,
		rm:                   newRetryManager(),
		vwp:                  vwp,
		likelyBadDeviceNames: expiringcache.New[string, sync.Map](cacheForgetDelay),
		latestClientTokens:   expiringcache.New[string, int](cacheForgetDelay),
	}
}

// newBatcherManager initializes a new instance of batcherManager.
// Each batcher's `entries` set to maximum results returned by relevant EC2 API call without pagination.
// Each batcher's `delay` minimizes RPC latency and EC2 API calls. Tuned via scalability tests.
func newBatcherManager(svc EC2API) *batcherManager {
	return &batcherManager{
		volumeIDBatcher: batcher.New(500, batchMaxDelay, func(ids []string) (map[string]*types.Volume, error) {
			return execBatchDescribeVolumes(svc, ids, volumeIDBatcher)
		}),
		volumeTagBatcher: batcher.New(500, batchMaxDelay, func(names []string) (map[string]*types.Volume, error) {
			return execBatchDescribeVolumes(svc, names, volumeTagBatcher)
		}),
		instanceIDBatcher: batcher.New(50, batchMaxDelay, func(ids []string) (map[string]*types.Instance, error) {
			return execBatchDescribeInstances(svc, ids)
		}),
		snapshotIDBatcher: batcher.New(1000, batchMaxDelay, func(ids []string) (map[string]*types.Snapshot, error) {
			return execBatchDescribeSnapshots(svc, ids, snapshotIDBatcher)
		}),
		snapshotTagBatcher: batcher.New(1000, batchMaxDelay, func(names []string) (map[string]*types.Snapshot, error) {
			return execBatchDescribeSnapshots(svc, names, snapshotTagBatcher)
		}),
		volumeModificationIDBatcher: batcher.New(500, batchMaxDelay, func(names []string) (map[string]*types.VolumeModification, error) {
			return execBatchDescribeVolumesModifications(svc, names)
		}),
	}
}

// execBatchDescribeVolumes executes a batched DescribeVolumes API call depending on the type of batcher.
func execBatchDescribeVolumes(svc EC2API, input []string, batcher volumeBatcherType) (map[string]*types.Volume, error) {
	var request *ec2.DescribeVolumesInput

	switch batcher {
	case volumeIDBatcher:
		klog.V(7).InfoS("execBatchDescribeVolumes", "volumeIds", input)
		request = &ec2.DescribeVolumesInput{
			VolumeIds: input,
		}

	case volumeTagBatcher:
		klog.V(7).InfoS("execBatchDescribeVolumes", "names", input)
		filters := []types.Filter{
			{
				Name:   aws.String("tag:" + VolumeNameTagKey),
				Values: input,
			},
		}
		request = &ec2.DescribeVolumesInput{
			Filters: filters,
		}

	default:
		return nil, fmt.Errorf("execBatchDescribeVolumes: unsupported request type")
	}

	ctx, cancel := context.WithTimeout(context.Background(), batchDescribeTimeout)
	defer cancel()

	resp, err := describeVolumes(ctx, svc, request)
	if err != nil {
		return nil, err
	}

	result := make(map[string]*types.Volume)

	for _, v := range resp {
		volume := v
		key, err := extractVolumeKey(&volume, batcher)
		if err != nil {
			klog.Warningf("execBatchDescribeVolumes: skipping volume: %v, reason: %v", volume, err)
			continue
		}
		result[key] = &volume
	}

	klog.V(7).InfoS("execBatchDescribeVolumes: success", "result", result)
	return result, nil
}

// batchDescribeVolumes processes a DescribeVolumes request. Depending on the request,
// it determines the appropriate batcher to use, queues the task, and waits for the result.
func (c *cloud) batchDescribeVolumes(request *ec2.DescribeVolumesInput) (*types.Volume, error) {
	var b *batcher.Batcher[string, *types.Volume]
	var task string

	switch {
	case len(request.VolumeIds) == 1 && request.VolumeIds[0] != "":
		b = c.bm.volumeIDBatcher
		task = request.VolumeIds[0]

	case len(request.Filters) == 1 && *request.Filters[0].Name == "tag:"+VolumeNameTagKey && len(request.Filters[0].Values) == 1:
		b = c.bm.volumeTagBatcher
		task = request.Filters[0].Values[0]

	default:
		return nil, fmt.Errorf("%w: batchDescribeVolumes: request: %v", ErrInvalidRequest, request)
	}

	ch := make(chan batcher.BatchResult[*types.Volume])

	b.AddTask(task, ch)

	r := <-ch

	if r.Err != nil {
		return nil, r.Err
	}
	if r.Result == nil {
		return nil, ErrNotFound
	}
	return r.Result, nil
}

// extractVolumeKey retrieves the key associated with a given volume based on the batcher type.
// For the volumeIDBatcher type, it returns the volume's ID.
// For other types, it searches for the VolumeNameTagKey within the volume's tags.
func extractVolumeKey(v *types.Volume, batcher volumeBatcherType) (string, error) {
	if batcher == volumeIDBatcher {
		if v.VolumeId == nil {
			return "", errors.New("extractVolumeKey: missing volume ID")
		}
		return *v.VolumeId, nil
	}
	for _, tag := range v.Tags {
		klog.V(7).InfoS("extractVolumeKey: processing tag", "volume", v, "*tag.Key", *tag.Key, "VolumeNameTagKey", VolumeNameTagKey)
		if tag.Key == nil || tag.Value == nil {
			klog.V(7).InfoS("extractVolumeKey: skipping volume due to missing tag", "volume", v, "tag", tag)
			continue
		}
		if *tag.Key == VolumeNameTagKey {
			klog.V(7).InfoS("extractVolumeKey: found volume name tag", "volume", v, "tag", tag)
			return *tag.Value, nil
		}
	}
	return "", errors.New("extractVolumeKey: missing VolumeNameTagKey in volume tags")
}

func (c *cloud) CreateDisk(ctx context.Context, volumeName string, diskOptions *DiskOptions) (*Disk, error) {
	var (
		createType    string
		iops          int32
		throughput    int32
		err           error
		maxIops       int32
		minIops       int32
		maxIopsPerGb  int32
		requestedIops int32
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
		throughput = diskOptions.Throughput
	default:
		return nil, fmt.Errorf("invalid AWS VolumeType %q", diskOptions.VolumeType)
	}

	if diskOptions.MultiAttachEnabled && createType != VolumeTypeIO2 {
		return nil, fmt.Errorf("CreateDisk: multi-attach is only supported for io2 volumes")
	}

	if maxIops > 0 {
		if diskOptions.IOPS > 0 {
			requestedIops = diskOptions.IOPS
		} else if diskOptions.IOPSPerGB > 0 {
			requestedIops = diskOptions.IOPSPerGB * capacityGiB
		}
		iops = capIOPS(createType, capacityGiB, requestedIops, minIops, maxIops, maxIopsPerGb, diskOptions.AllowIOPSPerGBIncrease)
	}

	var tags []types.Tag
	for key, value := range diskOptions.Tags {
		tags = append(tags, types.Tag{Key: aws.String(key), Value: aws.String(value)})
	}
	tagSpec := types.TagSpecification{
		ResourceType: types.ResourceTypeVolume,
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

	// The first client token used for any volume is the volume name as provided via CSI
	// However, if a volume fails to create asyncronously (that is, the CreateVolume call
	// succeeds but the volume ultimately fails to create), the client token is burned until
	// EC2 forgets about its use (measured as 12 hours under normal conditions)
	//
	// To prevent becoming stuck for 12 hours when this occurs, we sequentially append "-2",
	// "-3", "-4", etc to the volume name before hashing on the subsequent attempt after a
	// volume fails to create because of an IdempotentParameterMismatch AWS error
	// The most recent appended value is stored in an expiring cache to prevent memory leaks
	tokenBase := volumeName
	if tokenNumber, ok := c.latestClientTokens.Get(volumeName); ok {
		tokenBase += "-" + strconv.Itoa(*tokenNumber)
	}

	// We use a sha256 hash to guarantee the token that is less than or equal to 64 characters
	clientToken := sha256.Sum256([]byte(tokenBase))

	requestInput := &ec2.CreateVolumeInput{
		AvailabilityZone:   aws.String(zone),
		ClientToken:        aws.String(hex.EncodeToString(clientToken[:])),
		Size:               aws.Int32(capacityGiB),
		VolumeType:         types.VolumeType(createType),
		Encrypted:          aws.Bool(diskOptions.Encrypted),
		MultiAttachEnabled: aws.Bool(diskOptions.MultiAttachEnabled),
	}

	if !util.IsSBE(zone) {
		requestInput.TagSpecifications = []types.TagSpecification{tagSpec}
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
		requestInput.Iops = aws.Int32(iops)
	}
	if throughput > 0 {
		requestInput.Throughput = aws.Int32(throughput)
	}
	snapshotID := diskOptions.SnapshotID
	if len(snapshotID) > 0 {
		requestInput.SnapshotId = aws.String(snapshotID)
	}

	response, err := c.ec2.CreateVolume(ctx, requestInput, func(o *ec2.Options) {
		o.Retryer = c.rm.createVolumeRetryer
	})
	if err != nil {
		if isAWSErrorSnapshotNotFound(err) {
			return nil, ErrNotFound
		}
		if isAWSErrorIdempotentParameterMismatch(err) {
			nextTokenNumber := 2
			if tokenNumber, ok := c.latestClientTokens.Get(volumeName); ok {
				nextTokenNumber = *tokenNumber + 1
			}
			c.latestClientTokens.Set(volumeName, &nextTokenNumber)
			return nil, ErrIdempotentParameterMismatch
		}
		return nil, fmt.Errorf("could not create volume in EC2: %w", err)
	}

	volumeID := aws.ToString(response.VolumeId)
	if len(volumeID) == 0 {
		return nil, fmt.Errorf("volume ID was not returned by CreateVolume")
	}

	size := *response.Size
	if size == 0 {
		return nil, fmt.Errorf("disk size was not returned by CreateVolume")
	}

	if err := c.waitForVolume(ctx, volumeID); err != nil {
		return nil, fmt.Errorf("timed out waiting for volume to create: %w", err)
	}

	outpostArn := aws.ToString(response.OutpostArn)
	var resources []string
	if util.IsSBE(zone) {
		requestTagsInput := &ec2.CreateTagsInput{
			Resources: append(resources, volumeID),
			Tags:      tags,
		}
		_, err := c.ec2.CreateTags(ctx, requestTagsInput)
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

// execBatchDescribeVolumesModifications executes a batched DescribeVolumesModifications API call
func execBatchDescribeVolumesModifications(svc EC2API, input []string) (map[string]*types.VolumeModification, error) {
	klog.V(7).InfoS("execBatchDescribeVolumeModifications", "volumeIds", input)
	request := &ec2.DescribeVolumesModificationsInput{
		VolumeIds: input,
	}

	ctx, cancel := context.WithTimeout(context.Background(), batchDescribeTimeout)
	defer cancel()

	resp, err := describeVolumesModifications(ctx, svc, request)
	if err != nil {
		return nil, err
	}

	result := make(map[string]*types.VolumeModification)

	for _, m := range resp {
		volumeModification := m
		result[*volumeModification.VolumeId] = &volumeModification
	}

	klog.V(7).InfoS("execBatchDescribeVolumeModifications: success", "result", result)
	return result, nil
}

// batchDescribeVolumesModifications processes a DescribeVolumesModifications request by queuing the task and waiting for the result.
func (c *cloud) batchDescribeVolumesModifications(request *ec2.DescribeVolumesModificationsInput) (*types.VolumeModification, error) {
	var task string

	if len(request.VolumeIds) == 1 && request.VolumeIds[0] != "" {
		task = request.VolumeIds[0]
	} else {
		return nil, fmt.Errorf("%w: batchDescribeVolumesModifications: invalid request, request: %v", ErrInvalidRequest, request)
	}

	ch := make(chan batcher.BatchResult[*types.VolumeModification])

	b := c.bm.volumeModificationIDBatcher
	b.AddTask(task, ch)

	r := <-ch

	if r.Err != nil {
		return nil, r.Err
	}
	return r.Result, nil
}

// ModifyTags adds, updates, and deletes tags for the specified EBS volume.
func (c *cloud) ModifyTags(ctx context.Context, volumeID string, tagOptions ModifyTagsOptions) error {
	if len(tagOptions.TagsToDelete) > 0 {
		deleteTagsInput := &ec2.DeleteTagsInput{
			Resources: []string{volumeID},
			Tags:      make([]types.Tag, 0, len(tagOptions.TagsToDelete)),
		}
		for _, tagKey := range tagOptions.TagsToDelete {
			deleteTagsInput.Tags = append(deleteTagsInput.Tags, types.Tag{Key: aws.String(tagKey)})
		}
		_, deleteErr := c.ec2.DeleteTags(ctx, deleteTagsInput)
		if deleteErr != nil {
			klog.ErrorS(deleteErr, "failed to delete tags", "volumeID", volumeID)
			return deleteErr
		}
	}
	if len(tagOptions.TagsToAdd) > 0 {
		createTagsInput := &ec2.CreateTagsInput{
			Resources: []string{volumeID},
			Tags:      make([]types.Tag, 0, len(tagOptions.TagsToAdd)),
		}
		for k, v := range tagOptions.TagsToAdd {
			createTagsInput.Tags = append(createTagsInput.Tags, types.Tag{
				Key:   aws.String(k),
				Value: aws.String(v),
			})
		}
		_, addErr := c.ec2.CreateTags(ctx, createTagsInput)
		if addErr != nil {
			klog.ErrorS(addErr, "failed to create tags", "volumeID", volumeID)
			return addErr
		}
	}
	return nil
}

// ResizeOrModifyDisk resizes an EBS volume in GiB increments, rounding up to the next possible allocatable unit, and/or modifies an EBS
// volume with the parameters in ModifyDiskOptions.
// The resizing operation is performed only when newSizeBytes != 0.
// It returns the volume size after this call or an error if the size couldn't be determined or the volume couldn't be modified.
func (c *cloud) ResizeOrModifyDisk(ctx context.Context, volumeID string, newSizeBytes int64, options *ModifyDiskOptions) (int32, error) {
	if newSizeBytes != 0 {
		klog.V(4).InfoS("Received Resize and/or Modify Disk request", "volumeID", volumeID, "newSizeBytes", newSizeBytes, "options", options)
	} else {
		klog.V(4).InfoS("Received Modify Disk request", "volumeID", volumeID, "options", options)
	}

	newSizeGiB, err := util.RoundUpGiB(newSizeBytes)
	if err != nil {
		return 0, err
	}
	needsModification, volumeSize, err := c.validateModifyVolume(ctx, volumeID, newSizeGiB, options)

	if err != nil || !needsModification {
		return volumeSize, err
	}

	req := &ec2.ModifyVolumeInput{
		VolumeId: aws.String(volumeID),
	}
	if newSizeBytes != 0 {
		req.Size = aws.Int32(newSizeGiB)
	}
	if options.IOPS != 0 {
		req.Iops = aws.Int32(options.IOPS)
	}
	if options.VolumeType != "" {
		req.VolumeType = types.VolumeType(options.VolumeType)
	}
	if options.Throughput != 0 {
		req.Throughput = aws.Int32(options.Throughput)
	}
	response, err := c.ec2.ModifyVolume(ctx, req, func(o *ec2.Options) {
		o.Retryer = c.rm.modifyVolumeRetryer
	})
	if err != nil {
		if isAWSErrorInvalidParameter(err) {
			// Wrap error to preserve original message from AWS as to why this was an invalid argument
			return 0, fmt.Errorf("%w: %w", ErrInvalidArgument, err)
		}
		return 0, err
	}
	// If the volume modification isn't immediately completed, wait for it to finish
	state := string(response.VolumeModification.ModificationState)
	if !volumeModificationDone(state) {
		err = c.waitForVolumeModification(ctx, volumeID)
		if err != nil {
			return 0, err
		}
	}
	// Perform one final check on the volume
	return c.checkDesiredState(ctx, volumeID, int32(newSizeGiB), options)
}

func (c *cloud) DeleteDisk(ctx context.Context, volumeID string) (bool, error) {
	request := &ec2.DeleteVolumeInput{VolumeId: &volumeID}
	if _, err := c.ec2.DeleteVolume(ctx, request, func(o *ec2.Options) {
		o.Retryer = c.rm.deleteVolumeRetryer
	}); err != nil {
		if isAWSErrorVolumeNotFound(err) {
			return false, ErrNotFound
		}
		return false, fmt.Errorf("DeleteDisk could not delete volume: %w", err)
	}
	return true, nil
}

// execBatchDescribeInstances executes a batched DescribeInstances API call
func execBatchDescribeInstances(svc EC2API, input []string) (map[string]*types.Instance, error) {
	klog.V(7).InfoS("execBatchDescribeInstances", "instanceIds", input)
	request := &ec2.DescribeInstancesInput{
		InstanceIds: input,
	}

	ctx, cancel := context.WithTimeout(context.Background(), batchDescribeTimeout)
	defer cancel()

	resp, err := describeInstances(ctx, svc, request)
	if err != nil {
		return nil, err
	}

	result := make(map[string]*types.Instance)

	for _, i := range resp {
		instance := i
		if instance.InstanceId == nil {
			klog.Warningf("execBatchDescribeInstances: skipping instance: %v, reason: missing instance ID", instance)
			continue
		}
		result[*instance.InstanceId] = &instance
	}

	klog.V(7).InfoS("execBatchDescribeInstances: success", "result", result)
	return result, nil
}

// batchDescribeInstances processes a DescribeInstances request by queuing the task and waiting for the result.
func (c *cloud) batchDescribeInstances(request *ec2.DescribeInstancesInput) (*types.Instance, error) {
	var task string

	if len(request.InstanceIds) == 1 && request.InstanceIds[0] != "" {
		task = request.InstanceIds[0]
	} else {
		return nil, fmt.Errorf("%w: batchDescribeInstances: request: %v", ErrInvalidRequest, request)
	}

	ch := make(chan batcher.BatchResult[*types.Instance])

	b := c.bm.instanceIDBatcher
	b.AddTask(task, ch)

	r := <-ch

	if r.Err != nil {
		return nil, r.Err
	}
	if r.Result == nil {
		return nil, ErrNotFound
	}
	return r.Result, nil
}

func (c *cloud) AttachDisk(ctx context.Context, volumeID, nodeID string) (string, error) {
	instance, err := c.getInstance(ctx, nodeID)
	if err != nil {
		return "", err
	}

	likelyBadDeviceNames, ok := c.likelyBadDeviceNames.Get(nodeID)
	if !ok {
		likelyBadDeviceNames = new(sync.Map)
		c.likelyBadDeviceNames.Set(nodeID, likelyBadDeviceNames)
	}

	device, err := c.dm.NewDevice(instance, volumeID, likelyBadDeviceNames)
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

		resp, attachErr := c.ec2.AttachVolume(ctx, request, func(o *ec2.Options) {
			o.Retryer = c.rm.attachVolumeRetryer
		})
		if attachErr != nil {
			if isAWSErrorBlockDeviceInUse(attachErr) {
				// If block device is "in use", that likely indicates a bad name that is in use by a block
				// device that we do not know about (example: block devices attached in the AMI, which are
				// not reported in DescribeInstance's block device map)
				//
				// Store such bad names in the "likely bad" map to be considered last in future attempts
				likelyBadDeviceNames.Store(device.Path, struct{}{})
			}
			return "", fmt.Errorf("could not attach volume %q to node %q: %w", volumeID, nodeID, attachErr)
		}
		likelyBadDeviceNames.Delete(device.Path)
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

	_, err = c.ec2.DetachVolume(ctx, request, func(o *ec2.Options) {
		o.Retryer = c.rm.detachVolumeRetryer
	})
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
func (c *cloud) WaitForAttachmentState(ctx context.Context, volumeID, expectedState string, expectedInstance string, expectedDevice string, alreadyAssigned bool) (*types.VolumeAttachment, error) {
	var attachment *types.VolumeAttachment

	verifyVolumeFunc := func(ctx context.Context) (bool, error) {
		request := &ec2.DescribeVolumesInput{
			VolumeIds: []string{volumeID},
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
			a := a
			if a.InstanceId != nil {
				if aws.ToString(a.InstanceId) == expectedInstance {
					attachmentState = string(a.State)
					attachment = &a
				}
			}
		}

		if attachmentState == "" {
			attachmentState = volumeDetachedState
		}

		if attachment != nil && attachment.Device != nil && expectedState == volumeAttachedState {
			device := aws.ToString(attachment.Device)
			if device != expectedDevice {
				klog.InfoS("WaitForAttachmentState: device mismatch", "device", device, "expectedDevice", expectedDevice, "attachment", attachment)
				return false, nil
			}
		}

		// if we expected volume to be attached and it was reported as already attached via DescribeInstance call
		// but DescribeVolume told us volume is detached, we will short-circuit this long wait loop and return error
		// so as AttachDisk can be retried without waiting for 20 minutes.
		if (expectedState == volumeAttachedState) && alreadyAssigned && (attachmentState != expectedState) {
			request := &ec2.AttachVolumeInput{
				Device:     aws.String(expectedDevice),
				InstanceId: aws.String(expectedInstance),
				VolumeId:   aws.String(volumeID),
			}
			_, err := c.ec2.AttachVolume(ctx, request)
			if err != nil {
				return false, fmt.Errorf("WaitForAttachmentState AttachVolume error, expected device but be attached but was %s, volumeID=%q, instanceID=%q, Device=%q, err=%w", attachmentState, volumeID, expectedInstance, expectedDevice, err)
			}
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
		klog.InfoS("Waiting for volume state", "volumeID", volumeID, "actual", attachmentState, "desired", expectedState)
		return false, nil
	}

	return attachment, wait.ExponentialBackoffWithContext(ctx, c.vwp.attachmentBackoff, verifyVolumeFunc)
}

func (c *cloud) GetDiskByName(ctx context.Context, name string, capacityBytes int64) (*Disk, error) {
	request := &ec2.DescribeVolumesInput{
		Filters: []types.Filter{
			{
				Name:   aws.String("tag:" + VolumeNameTagKey),
				Values: []string{name},
			},
		},
	}

	volume, err := c.getVolume(ctx, request)
	if err != nil {
		return nil, err
	}

	volSizeBytes := util.GiBToBytes(*volume.Size)
	if volSizeBytes != capacityBytes {
		return nil, ErrDiskExistsDiffSize
	}

	return &Disk{
		VolumeID:         aws.ToString(volume.VolumeId),
		CapacityGiB:      *volume.Size,
		AvailabilityZone: aws.ToString(volume.AvailabilityZone),
		SnapshotID:       aws.ToString(volume.SnapshotId),
		OutpostArn:       aws.ToString(volume.OutpostArn),
	}, nil
}

func (c *cloud) GetDiskByID(ctx context.Context, volumeID string) (*Disk, error) {
	request := &ec2.DescribeVolumesInput{
		VolumeIds: []string{volumeID},
	}

	volume, err := c.getVolume(ctx, request)
	if err != nil {
		return nil, err
	}

	disk := &Disk{
		VolumeID:         aws.ToString(volume.VolumeId),
		AvailabilityZone: aws.ToString(volume.AvailabilityZone),
		OutpostArn:       aws.ToString(volume.OutpostArn),
		Attachments:      getVolumeAttachmentsList(*volume),
	}

	if volume.Size != nil {
		disk.CapacityGiB = *volume.Size
	}

	return disk, nil
}

// execBatchDescribeSnapshots executes a batched DescribeSnapshots API call depending on the type of batcher.
func execBatchDescribeSnapshots(svc EC2API, input []string, batcher snapshotBatcherType) (map[string]*types.Snapshot, error) {
	var request *ec2.DescribeSnapshotsInput

	switch batcher {
	case snapshotIDBatcher:
		klog.V(7).InfoS("execBatchDescribeSnapshots", "snapshotIds", input)
		request = &ec2.DescribeSnapshotsInput{
			SnapshotIds: input,
		}

	case snapshotTagBatcher:
		klog.V(7).InfoS("execBatchDescribeSnapshots", "names", input)
		filters := []types.Filter{
			{
				Name:   aws.String("tag:" + SnapshotNameTagKey),
				Values: input,
			},
		}
		request = &ec2.DescribeSnapshotsInput{
			Filters: filters,
		}

	default:
		return nil, fmt.Errorf("execBatchDescribeSnapshots: unsupported request type")
	}

	ctx, cancel := context.WithTimeout(context.Background(), batchDescribeTimeout)
	defer cancel()

	resp, err := describeSnapshots(ctx, svc, request)
	if err != nil {
		return nil, err
	}

	result := make(map[string]*types.Snapshot)

	for _, snapshot := range resp {
		snapshot := snapshot
		key, err := extractSnapshotKey(&snapshot, batcher)
		if err != nil {
			klog.Warningf("execBatchDescribeSnapshots: skipping snapshot: %v, reason: %v", snapshot, err)
			continue
		}
		result[key] = &snapshot
	}

	klog.V(7).InfoS("execBatchDescribeSnapshots: success", "result", result)
	return result, nil
}

// batchDescribeSnapshots processes a DescribeSnapshots request. Depending on the request,
// it determines the appropriate batcher to use, queues the task, and waits for the result.
func (c *cloud) batchDescribeSnapshots(request *ec2.DescribeSnapshotsInput) (*types.Snapshot, error) {
	var b *batcher.Batcher[string, *types.Snapshot]
	var task string

	switch {
	case len(request.SnapshotIds) == 1 && request.SnapshotIds[0] != "":
		b = c.bm.snapshotIDBatcher
		task = request.SnapshotIds[0]

	case len(request.Filters) == 1 && *request.Filters[0].Name == "tag:"+SnapshotNameTagKey && len(request.Filters[0].Values) == 1:
		b = c.bm.snapshotTagBatcher
		task = request.Filters[0].Values[0]

	default:
		return nil, fmt.Errorf("%w: batchDescribeSnapshots: request: %v", ErrInvalidRequest, request)
	}

	ch := make(chan batcher.BatchResult[*types.Snapshot])

	b.AddTask(task, ch)

	r := <-ch

	if r.Err != nil {
		return nil, r.Err
	}
	if r.Result == nil {
		return nil, ErrNotFound
	}
	return r.Result, nil
}

// extractSnapshotKey retrieves the key associated with a given snapshot based on the batcher type.
// For the snapshotIDBatcher type, it returns the snapshot's ID.
// For other types, it searches for the SnapshotNameTagKey within the snapshot's tags.
func extractSnapshotKey(s *types.Snapshot, batcher snapshotBatcherType) (string, error) {
	if batcher == snapshotIDBatcher {
		if s.SnapshotId == nil {
			return "", errors.New("extractSnapshotKey: missing snapshot ID")
		}
		return *s.SnapshotId, nil
	}
	for _, tag := range s.Tags {
		klog.V(7).InfoS("extractSnapshotKey: processing tag", "snapshot", s, "*tag.Key", *tag.Key, "SnapshotNameTagKey", SnapshotNameTagKey)
		if tag.Key == nil || tag.Value == nil {
			klog.V(7).InfoS("extractSnapshotKey: skipping snapshot due to missing tag", "snapshot", s, "tag", tag)
			continue
		}
		if *tag.Key == SnapshotNameTagKey {
			klog.V(7).InfoS("extractSnapshotKey: found snapshot name tag", "snapshot", s, "tag", tag)
			return *tag.Value, nil
		}
	}
	return "", errors.New("extractSnapshotKey: missing SnapshotNameTagKey in snapshot tags")
}

func (c *cloud) CreateSnapshot(ctx context.Context, volumeID string, snapshotOptions *SnapshotOptions) (snapshot *Snapshot, err error) {
	descriptions := "Created by AWS EBS CSI driver for volume " + volumeID

	var tags []types.Tag
	var request *ec2.CreateSnapshotInput
	for key, value := range snapshotOptions.Tags {
		tags = append(tags, types.Tag{Key: aws.String(key), Value: aws.String(value)})
	}
	tagSpec := types.TagSpecification{
		ResourceType: types.ResourceTypeSnapshot,
		Tags:         tags,
	}
	request = &ec2.CreateSnapshotInput{
		VolumeId:          aws.String(volumeID),
		TagSpecifications: []types.TagSpecification{tagSpec},
		Description:       aws.String(descriptions),
	}
	if snapshotOptions.OutpostArn != "" {
		request.OutpostArn = aws.String(snapshotOptions.OutpostArn)
	}
	res, err := c.ec2.CreateSnapshot(ctx, request, func(o *ec2.Options) {
		o.Retryer = c.rm.createSnapshotRetryer
	})
	if err != nil {
		return nil, fmt.Errorf("error creating snapshot of volume %s: %w", volumeID, err)
	}
	if res == nil {
		return nil, fmt.Errorf("nil CreateSnapshotResponse")
	}

	return &Snapshot{
		SnapshotID:     aws.ToString(res.SnapshotId),
		SourceVolumeID: aws.ToString(res.VolumeId),
		Size:           *res.VolumeSize,
		CreationTime:   aws.ToTime(res.StartTime),
		ReadyToUse:     res.State == types.SnapshotStateCompleted,
	}, nil
}

func (c *cloud) DeleteSnapshot(ctx context.Context, snapshotID string) (success bool, err error) {
	request := &ec2.DeleteSnapshotInput{}
	request.SnapshotId = aws.String(snapshotID)
	request.DryRun = aws.Bool(false)
	if _, err := c.ec2.DeleteSnapshot(ctx, request, func(o *ec2.Options) {
		o.Retryer = c.rm.deleteSnapshotRetryer
	}); err != nil {
		if isAWSErrorSnapshotNotFound(err) {
			return false, ErrNotFound
		}
		return false, fmt.Errorf("DeleteSnapshot could not delete snapshot: %w", err)
	}
	return true, nil
}

func (c *cloud) GetSnapshotByName(ctx context.Context, name string) (snapshot *Snapshot, err error) {
	request := &ec2.DescribeSnapshotsInput{
		Filters: []types.Filter{
			{
				Name:   aws.String("tag:" + SnapshotNameTagKey),
				Values: []string{name},
			},
		},
	}

	ec2snapshot, err := c.getSnapshot(ctx, request)
	if err != nil {
		return nil, err
	}

	return c.ec2SnapshotResponseToStruct(*ec2snapshot), nil
}

func (c *cloud) GetSnapshotByID(ctx context.Context, snapshotID string) (snapshot *Snapshot, err error) {
	request := &ec2.DescribeSnapshotsInput{
		SnapshotIds: []string{snapshotID},
	}

	ec2snapshot, err := c.getSnapshot(ctx, request)
	if err != nil {
		return nil, err
	}

	return c.ec2SnapshotResponseToStruct(*ec2snapshot), nil
}

// ListSnapshots retrieves AWS EBS snapshots for an optionally specified volume ID.  If maxResults is set, it will return up to maxResults snapshots.  If there are more snapshots than maxResults,
// a next token value will be returned to the client as well.  They can use this token with subsequent calls to retrieve the next page of results.  If maxResults is not set (0),
// there will be no restriction up to 1000 results (https://docs.aws.amazon.com/sdk-for-go/api/service/ec2/#DescribeSnapshotsInput).
func (c *cloud) ListSnapshots(ctx context.Context, volumeID string, maxResults int32, nextToken string) (listSnapshotsResponse *ListSnapshotsResponse, err error) {
	if maxResults > 0 && maxResults < 5 {
		return nil, ErrInvalidMaxResults
	}

	describeSnapshotsInput := &ec2.DescribeSnapshotsInput{
		MaxResults: aws.Int32(maxResults),
	}

	if len(nextToken) != 0 {
		describeSnapshotsInput.NextToken = aws.String(nextToken)
	}
	if len(volumeID) != 0 {
		describeSnapshotsInput.Filters = []types.Filter{
			{
				Name:   aws.String("volume-id"),
				Values: []string{volumeID},
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
		NextToken: aws.ToString(ec2SnapshotsResponse.NextToken),
	}, nil
}

// Helper method converting EC2 snapshot type to the internal struct
func (c *cloud) ec2SnapshotResponseToStruct(ec2Snapshot types.Snapshot) *Snapshot {
	snapshotSize := *ec2Snapshot.VolumeSize
	snapshot := &Snapshot{
		SnapshotID:     aws.ToString(ec2Snapshot.SnapshotId),
		SourceVolumeID: aws.ToString(ec2Snapshot.VolumeId),
		Size:           snapshotSize,
		CreationTime:   *ec2Snapshot.StartTime,
	}
	if ec2Snapshot.State == types.SnapshotStateCompleted {
		snapshot.ReadyToUse = true
	} else {
		snapshot.ReadyToUse = false
	}

	return snapshot
}

func (c *cloud) EnableFastSnapshotRestores(ctx context.Context, availabilityZones []string, snapshotID string) (*ec2.EnableFastSnapshotRestoresOutput, error) {
	request := &ec2.EnableFastSnapshotRestoresInput{
		AvailabilityZones: availabilityZones,
		SourceSnapshotIds: []string{snapshotID},
	}
	klog.V(4).InfoS("Creating Fast Snapshot Restores", "snapshotID", snapshotID, "availabilityZones", availabilityZones)
	response, err := c.ec2.EnableFastSnapshotRestores(ctx, request, func(o *ec2.Options) {
		o.Retryer = c.rm.enableFastSnapshotRestoresRetryer
	})
	if err != nil {
		return nil, err
	}
	if len(response.Unsuccessful) > 0 {
		return response, fmt.Errorf("failed to create fast snapshot restores for snapshot %s: %v", snapshotID, response.Unsuccessful)
	}
	return response, nil
}

func describeVolumes(ctx context.Context, svc EC2API, request *ec2.DescribeVolumesInput) ([]types.Volume, error) {
	var volumes []types.Volume
	var nextToken *string
	for {
		response, err := svc.DescribeVolumes(ctx, request)
		if err != nil {
			return nil, err
		}
		volumes = append(volumes, response.Volumes...)
		nextToken = response.NextToken
		if aws.ToString(nextToken) == "" {
			break
		}
		request.NextToken = nextToken
	}
	return volumes, nil
}

func (c *cloud) getVolume(ctx context.Context, request *ec2.DescribeVolumesInput) (*types.Volume, error) {
	if c.bm == nil {
		volumes, err := describeVolumes(ctx, c.ec2, request)
		if err != nil {
			return nil, err
		}
		if l := len(volumes); l > 1 {
			return nil, ErrMultiDisks
		} else if l < 1 {
			return nil, ErrNotFound
		}
		return &volumes[0], nil
	} else {
		return c.batchDescribeVolumes(request)
	}
}

func describeInstances(ctx context.Context, svc EC2API, request *ec2.DescribeInstancesInput) ([]types.Instance, error) {
	instances := []types.Instance{}
	var nextToken *string
	for {
		response, err := svc.DescribeInstances(ctx, request)
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
		if aws.ToString(nextToken) == "" {
			break
		}
		request.NextToken = nextToken
	}
	return instances, nil
}

func (c *cloud) getInstance(ctx context.Context, nodeID string) (*types.Instance, error) {
	request := &ec2.DescribeInstancesInput{
		InstanceIds: []string{nodeID},
	}

	if c.bm == nil {
		instances, err := describeInstances(ctx, c.ec2, request)
		if err != nil {
			return nil, err
		}

		if l := len(instances); l > 1 {
			return nil, fmt.Errorf("found %d instances with ID %q", l, nodeID)
		} else if l < 1 {
			return nil, ErrNotFound
		}

		return &instances[0], nil
	} else {
		return c.batchDescribeInstances(request)
	}
}

func describeSnapshots(ctx context.Context, svc EC2API, request *ec2.DescribeSnapshotsInput) ([]types.Snapshot, error) {
	var snapshots []types.Snapshot
	var nextToken *string
	for {
		response, err := svc.DescribeSnapshots(ctx, request)
		if err != nil {
			return nil, err
		}
		snapshots = append(snapshots, response.Snapshots...)
		nextToken = response.NextToken
		if aws.ToString(nextToken) == "" {
			break
		}
		request.NextToken = nextToken
	}

	return snapshots, nil
}

func (c *cloud) getSnapshot(ctx context.Context, request *ec2.DescribeSnapshotsInput) (*types.Snapshot, error) {
	if c.bm == nil {
		snapshots, err := describeSnapshots(ctx, c.ec2, request)
		if err != nil {
			return nil, err
		}

		if l := len(snapshots); l > 1 {
			return nil, ErrMultiSnapshots
		} else if l < 1 {
			return nil, ErrNotFound
		}
		return &snapshots[0], nil
	} else {
		return c.batchDescribeSnapshots(request)
	}
}

// listSnapshots returns all snapshots based from a request
func (c *cloud) listSnapshots(ctx context.Context, request *ec2.DescribeSnapshotsInput) (*ec2ListSnapshotsResponse, error) {
	var snapshots []types.Snapshot
	var nextToken *string

	response, err := c.ec2.DescribeSnapshots(ctx, request)
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
func (c *cloud) waitForVolume(ctx context.Context, volumeID string) error {
	time.Sleep(c.vwp.creationInitialDelay)

	request := &ec2.DescribeVolumesInput{
		VolumeIds: []string{volumeID},
	}

	err := wait.ExponentialBackoffWithContext(ctx, c.vwp.creationBackoff, func(ctx context.Context) (done bool, err error) {
		vol, err := c.getVolume(ctx, request)
		if err != nil {
			return true, err
		}
		if vol.State != "" {
			return vol.State == types.VolumeStateAvailable, nil
		}
		return false, nil
	})

	return err
}

// isAWSError returns a boolean indicating whether the error is AWS-related
// and has the given code. More information on AWS error codes at:
// https://docs.aws.amazon.com/AWSEC2/latest/APIReference/errors-overview.html
func isAWSError(err error, code string) bool {
	var apiErr smithy.APIError
	if errors.As(err, &apiErr) {
		if apiErr.ErrorCode() == code {
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

// isAWSErrorBlockDeviceInUse returns a boolean indicating whether the
// given error appears to be a block device name already in use error.
func isAWSErrorBlockDeviceInUse(err error) bool {
	var apiErr smithy.APIError
	if errors.As(err, &apiErr) {
		if apiErr.ErrorCode() == "InvalidParameterValue" && strings.Contains(apiErr.ErrorMessage(), "already in use") {
			return true
		}
	}
	return false
}

// isAWSErrorInvalidParameter returns a boolean indicating whether the
// given error is caused by invalid parameters in a EC2 API request.
func isAWSErrorInvalidParameter(err error) bool {
	var apiError smithy.APIError
	if errors.As(err, &apiError) {
		_, found := invalidParameterErrorCodes[apiError.ErrorCode()]
		return found
	}
	return false
}

// Checks for desired size on volume by also verifying volume size by describing volume.
// This is to get around potential eventual consistency problems with describing volume modifications
// objects and ensuring that we read two different objects to verify volume state.
func (c *cloud) checkDesiredState(ctx context.Context, volumeID string, desiredSizeGiB int32, options *ModifyDiskOptions) (int32, error) {
	request := &ec2.DescribeVolumesInput{
		VolumeIds: []string{volumeID},
	}
	volume, err := c.getVolume(ctx, request)
	if err != nil {
		return 0, err
	}

	// AWS resizes in chunks of GiB (not GB)
	realSizeGiB := *volume.Size

	// Check if there is a mismatch between the requested modification and the current volume
	// If there is, the volume is still modifying and we should not return a success
	if realSizeGiB < desiredSizeGiB {
		return realSizeGiB, fmt.Errorf("volume %q is still being expanded to %d size", volumeID, desiredSizeGiB)
	} else if options.IOPS != 0 && (volume.Iops == nil || *volume.Iops != options.IOPS) {
		return realSizeGiB, fmt.Errorf("volume %q is still being modified to iops %d", volumeID, options.IOPS)
	} else if options.VolumeType != "" && !strings.EqualFold(string(volume.VolumeType), options.VolumeType) {
		return realSizeGiB, fmt.Errorf("volume %q is still being modified to type %q", volumeID, options.VolumeType)
	} else if options.Throughput != 0 && (volume.Throughput == nil || *volume.Throughput != options.Throughput) {
		return realSizeGiB, fmt.Errorf("volume %q is still being modified to throughput %d", volumeID, options.Throughput)
	}

	return realSizeGiB, nil
}

// waitForVolumeModification waits for a volume modification to finish.
func (c *cloud) waitForVolumeModification(ctx context.Context, volumeID string) error {
	waitErr := wait.ExponentialBackoff(c.vwp.modificationBackoff, func() (bool, error) {
		m, err := c.getLatestVolumeModification(ctx, volumeID, true)
		// Consider volumes that have never been modified as done
		if err != nil && errors.Is(err, VolumeNotBeingModified) {
			return true, nil
		} else if err != nil {
			return false, err
		}

		state := string(m.ModificationState)
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

func describeVolumesModifications(ctx context.Context, svc EC2API, request *ec2.DescribeVolumesModificationsInput) ([]types.VolumeModification, error) {
	volumeModifications := []types.VolumeModification{}
	var nextToken *string
	for {
		response, err := svc.DescribeVolumesModifications(ctx, request)
		if err != nil {
			if isAWSErrorModificationNotFound(err) {
				return nil, VolumeNotBeingModified
			}
			return nil, fmt.Errorf("error describing volume modifications: %w", err)
		}

		volumeModifications = append(volumeModifications, response.VolumesModifications...)

		nextToken = response.NextToken
		if aws.ToString(nextToken) == "" {
			break
		}
		request.NextToken = nextToken
	}
	return volumeModifications, nil
}

// getLatestVolumeModification returns the last modification of the volume.
func (c *cloud) getLatestVolumeModification(ctx context.Context, volumeID string, isBatchable bool) (*types.VolumeModification, error) {
	request := &ec2.DescribeVolumesModificationsInput{
		VolumeIds: []string{volumeID},
	}

	if c.bm == nil || !isBatchable {
		mod, err := c.ec2.DescribeVolumesModifications(ctx, request, func(o *ec2.Options) {
			o.Retryer = c.rm.unbatchableDescribeVolumesModificationsRetryer
		})
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

		return &volumeMods[len(volumeMods)-1], nil
	} else {
		return c.batchDescribeVolumesModifications(request)
	}
}

// randomAvailabilityZone returns a random zone from the given region
// the randomness relies on the response of DescribeAvailabilityZones
func (c *cloud) randomAvailabilityZone(ctx context.Context) (string, error) {
	request := &ec2.DescribeAvailabilityZonesInput{}
	response, err := c.ec2.DescribeAvailabilityZones(ctx, request)
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
	response, err := c.ec2.DescribeAvailabilityZones(ctx, &ec2.DescribeAvailabilityZonesInput{})
	if err != nil {
		return nil, fmt.Errorf("error describing availability zones: %w", err)
	}
	zones := make(map[string]struct{})
	for _, zone := range response.AvailabilityZones {
		zones[*zone.ZoneName] = struct{}{}
	}
	return zones, nil
}

func needsVolumeModification(volume types.Volume, newSizeGiB int32, options *ModifyDiskOptions) bool {
	oldSizeGiB := *volume.Size
	needsModification := false

	if oldSizeGiB < newSizeGiB {
		needsModification = true
	}

	if options.IOPS != 0 && (volume.Iops == nil || *volume.Iops != options.IOPS) {
		needsModification = true
	}

	if options.VolumeType != "" && !strings.EqualFold(string(volume.VolumeType), options.VolumeType) {
		needsModification = true
	}

	if options.Throughput != 0 && (volume.Throughput == nil || *volume.Throughput != options.Throughput) {
		needsModification = true
	}

	return needsModification
}

func (c *cloud) validateModifyVolume(ctx context.Context, volumeID string, newSizeGiB int32, options *ModifyDiskOptions) (bool, int32, error) {
	request := &ec2.DescribeVolumesInput{
		VolumeIds: []string{volumeID},
	}

	volume, err := c.getVolume(ctx, request)
	if err != nil {
		return true, 0, err
	}

	if volume.Size == nil {
		return true, 0, fmt.Errorf("volume %q has no size", volumeID)
	}
	oldSizeGiB := *volume.Size

	// This call must NOT be batched because a missing volume modification will return client error
	latestMod, err := c.getLatestVolumeModification(ctx, volumeID, false)
	if err != nil && !errors.Is(err, VolumeNotBeingModified) {
		return true, oldSizeGiB, fmt.Errorf("error fetching volume modifications for %q: %w", volumeID, err)
	}

	state := ""
	// latestMod can be nil if the volume has never been modified
	if latestMod != nil {
		state = string(latestMod.ModificationState)
		if state == string(types.VolumeModificationStateModifying) {
			// If volume is already modifying, detour to waiting for it to modify
			klog.V(5).InfoS("[Debug] Watching ongoing modification", "volumeID", volumeID)
			err = c.waitForVolumeModification(ctx, volumeID)
			if err != nil {
				return true, oldSizeGiB, err
			}
			returnGiB, returnErr := c.checkDesiredState(ctx, volumeID, newSizeGiB, options)
			return false, returnGiB, returnErr
		}
	}

	// At this point, we know we are starting a new volume modification
	// If we're asked to modify a volume to its current state, ignore the request and immediately return a success
	// This is because as of March 2024, EC2 ModifyVolume calls that don't change any parameters still modify the volume
	if !needsVolumeModification(*volume, newSizeGiB, options) {
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

	if state == string(types.VolumeModificationStateOptimizing) {
		return true, 0, fmt.Errorf("volume %q in OPTIMIZING state, cannot currently modify", volumeID)
	}

	return true, 0, nil
}

func volumeModificationDone(state string) bool {
	return state == string(types.VolumeModificationStateCompleted) || state == string(types.VolumeModificationStateOptimizing)
}

func getVolumeAttachmentsList(volume types.Volume) []string {
	var volumeAttachmentList []string
	for _, attachment := range volume.Attachments {
		if attachment.State == volumeAttachedState {
			volumeAttachmentList = append(volumeAttachmentList, aws.ToString(attachment.InstanceId))
		}
	}

	return volumeAttachmentList
}

// Calculate actual IOPS for a volume and cap it at supported AWS limits.
func capIOPS(volumeType string, requestedCapacityGiB int32, requestedIops int32, minTotalIOPS, maxTotalIOPS, maxIOPSPerGB int32, allowIncrease bool) int32 {
	// If requestedIops is zero the user did not request a specific amount, and the default will be used instead
	if requestedIops == 0 {
		return 0
	}

	iops := requestedIops

	if iops < minTotalIOPS {
		if allowIncrease {
			iops = minTotalIOPS
			klog.V(5).InfoS("[Debug] Increased IOPS to the min supported limit", "volumeType", volumeType, "requestedCapacityGiB", requestedCapacityGiB, "limit", iops)
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
	return iops
}
