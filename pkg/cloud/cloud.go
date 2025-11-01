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
	"math"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go-v2/service/sagemaker"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/aws/smithy-go"
	"github.com/kubernetes-sigs/aws-ebs-csi-driver/pkg/batcher"
	dm "github.com/kubernetes-sigs/aws-ebs-csi-driver/pkg/cloud/devicemanager"
	"github.com/kubernetes-sigs/aws-ebs-csi-driver/pkg/expiringcache"
	"github.com/kubernetes-sigs/aws-ebs-csi-driver/pkg/metrics"
	"github.com/kubernetes-sigs/aws-ebs-csi-driver/pkg/util"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/klog/v2"
)

// AWS volume types.
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

// AWS provisioning limits.
// Source: http://docs.aws.amazon.com/AWSEC2/latest/UserGuide/EBSVolumeTypes.html
const (
	io1MinTotalIOPS    = 100
	io1FallbackMaxIOPS = 64000
	io1MaxIOPSPerGB    = 50
	io2MinTotalIOPS    = 100
	io2FallbackMaxIOPS = 256000
	io2MaxIOPSPerGB    = 1000
	gp3FallbackMaxIOPS = 16000
	gp3MinTotalIOPS    = 3000
	gp3MaxIOPSPerGB    = 500
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
	cacheForgetDelay          = 1 * time.Hour
	volInitCacheForgetDelay   = 6 * time.Hour
	iopsLimitCacheForgetDelay = 12 * time.Hour

	dryRunInterval = 3 * time.Hour

	getCallerIdentityRetryDelay = 30 * time.Second
)

// VolumeStatusInitializingState is const reported by EC2 DescribeVolumeStatus which AWS SDK does not have type for.
const (
	VolumeStatusInitializingState = "initializing"
	VolumeStatusInitializedState  = "completed"
)

// Defaults.
const (
	// DefaultVolumeSize represents the default volume size.
	DefaultVolumeSize int64 = 100 * util.GiB
)

// Tags.
const (
	// VolumeNameTagKey is the key value that refers to the volume's name.
	VolumeNameTagKey = "CSIVolumeName"
	// SnapshotNameTagKey is the key value that refers to the snapshot's name.
	SnapshotNameTagKey = "CSIVolumeSnapshotName"
	// KubernetesTagKeyPrefix is the prefix of the key value that is reserved for Kubernetes.
	KubernetesTagKeyPrefix = "kubernetes.io"
	// AwsEbsDriverTagKey is the tag to identify if a volume/snapshot is managed by ebs csi driver.
	AwsEbsDriverTagKey = util.DriverName + "/cluster"
	// AllowAutoIOPSIncreaseOnModifyKey is the tag key for allowing IOPS increase on resizing if IOPSPerGB is set to ensure desired ratio is maintained.
	AllowAutoIOPSIncreaseOnModifyKey = util.DriverName + "/AllowAutoIOPSIncreaseOnModify"
	// IOPSPerGBKey represents the tag key for IOPS per GB.
	IOPSPerGBKey = util.DriverName + "/IOPSPerGb"
)

// Batcher.
const (
	volumeIDBatcher volumeBatcherType = iota
	volumeTagBatcher

	snapshotIDBatcher snapshotBatcherType = iota
	snapshotTagBatcher
)

const (
	batchDescribeTimeout = 30 * time.Second

	// Minimizes RPC latency and EC2 API calls. Tuned via scalability tests.
	batchMaxDelay = 500 * time.Millisecond

	// Tuned for EC2 DescribeVolumeStatus -- as of July 2025 it takes up to 5 min for initialization info to be updated.
	slowVolumeStatusBatchMaxDelay = 2 * time.Minute
	fastVolumeStatusBatchMaxDelay = 500 * time.Millisecond
)

const (
	// maxInstancesDescribed is the maximum number of instances described in each EC2 Describe Instances call.
	maxInstancesDescribed = 1000
)

var (
	// ErrMultiDisks is an error that is returned when multiple
	// disks are found with the same volume name.
	ErrMultiDisks = errors.New("multiple disks with same name")

	// ErrDiskExistsDiffSize is an error that is returned if a disk with a given
	// name, but different size, is found.
	ErrDiskExistsDiffSize = errors.New("there is already a disk with same name and different size")

	// ErrNotFound is returned when a resource is not found.
	ErrNotFound = errors.New("resource was not found")

	// ErrIdempotentParameterMismatch is returned when another request with same idempotent token is in-flight.
	ErrIdempotentParameterMismatch = errors.New("parameters on this idempotent request are inconsistent with parameters used in previous request(s)")

	// ErrAlreadyExists is returned when a resource is already existent.
	ErrAlreadyExists = errors.New("resource already exists")

	// ErrMultiSnapshots is returned when multiple snapshots are found
	// with the same ID.
	ErrMultiSnapshots = errors.New("multiple snapshots with the same name found")

	// ErrInvalidMaxResults is returned when a MaxResults pagination parameter is between 1 and 4.
	ErrInvalidMaxResults = errors.New("maxResults parameter must be 0 or greater than or equal to 5")

	// ErrVolumeNotBeingModified is returned if volume being described is not being modified.
	ErrVolumeNotBeingModified = errors.New("volume is not being modified")

	// ErrInvalidArgument is returned if parameters were rejected by cloud provider.
	ErrInvalidArgument = errors.New("invalid argument")

	// ErrInvalidRequest is returned if parameters were rejected by driver.
	ErrInvalidRequest = errors.New("invalid request")

	// ErrLimitExceeded is returned if a user exceeds a quota.
	ErrLimitExceeded = errors.New("limit exceeded")
)

// Set during build time via -ldflags.
var driverVersion string

// AWS error codes.
const (
	ValidationException = "ValidationException"
)

// Regex Patterns.
var (
	// For getting IOPS limit from gp3/io1 error.
	// Error example it is used for: "An error occurred (InvalidParameterValue) when calling the CreateVolume operation: Volume iops of 200000 is too high; maximum is 80000".
	nonIo2ErrRegex = regexp.MustCompile(`(?i)volume iops.*is too high.*maximum is (\d+)`)

	// For getting IOPS limit from io2 error.
	// Error example it is used for: "An error occurred (InvalidParameterCombination) when calling the CreateVolume operation: io2 volumes configured with greater than 64 TiB or 256K IOPS or 1000:1 IOPS:GB ratio are not supported".
	io2ErrRegex = regexp.MustCompile(`(?i)(\d+)K IOPS`)
)

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

// Disk represents a EBS volume.
type Disk struct {
	VolumeID           string
	CapacityGiB        int32
	AvailabilityZone   string
	AvailabilityZoneID string
	SourceVolumeID     string
	SnapshotID         string
	OutpostArn         string
	KmsKeyID           string
	Attachments        []string
}

// DiskOptions represents parameters to create an EBS volume.
type DiskOptions struct {
	CapacityBytes          int64
	Tags                   map[string]string
	VolumeType             string
	IOPSPerGB              int32
	AllowIOPSPerGBIncrease bool
	IOPS                   int32
	Throughput             int32
	AvailabilityZone       string
	AvailabilityZoneID     string
	OutpostArn             string
	Encrypted              bool
	MultiAttachEnabled     bool
	// KmsKeyID represents a fully qualified resource name to the key to use for encryption.
	// example: arn:aws:kms:us-east-1:012345678910:key/abcd1234-a123-456a-a12b-a123b4cd56ef
	KmsKeyID                 string
	SnapshotID               string
	SourceVolumeID           string
	VolumeInitializationRate int32
}

// ModifyDiskOptions represents parameters to modify an EBS volume.
type ModifyDiskOptions struct {
	VolumeType                string
	IOPS                      int32
	Throughput                int32
	IOPSPerGB                 int32
	AllowIopsIncreaseOnResize bool
}

// iopsLimits represents the IOPS limits set by EBS of a volume dependent on the volume type.
type iopsLimits struct {
	maxIops      int32
	minIops      int32
	maxIopsPerGb int32
}

// getVolumeLimitsParams represents the AZ parameters that getVolumeLimits will use to make the DryRun CreateVolume call.
type getVolumeLimitsParams struct {
	availabilityZone   string
	availabilityZoneId string
	outpostArn         string
}

// ModifyTagsOptions represents parameter to modify the tags of an existing EBS volume.
type ModifyTagsOptions struct {
	TagsToAdd    map[string]string
	TagsToDelete []string
}

// Snapshot represents an EBS volume snapshot.
type Snapshot struct {
	SnapshotID     string
	SourceVolumeID string
	Size           int32
	CreationTime   time.Time
	ReadyToUse     bool
}

// ListSnapshotsResponse is the container for our snapshots along with a pagination token to pass back to the caller.
type ListSnapshotsResponse struct {
	Snapshots []*Snapshot
	NextToken string
}

// SnapshotOptions represents parameters to create an EBS volume.
type SnapshotOptions struct {
	Tags       map[string]string
	OutpostArn string
}

// ec2ListSnapshotsResponse is a helper struct returned from the AWS API calling function to the main ListSnapshots function.
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
			Steps:    11,
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
	volumeStatusIDBatcherSlow   *batcher.Batcher[string, *types.VolumeStatusItem]
	volumeStatusIDBatcherFast   *batcher.Batcher[string, *types.VolumeStatusItem]
}

type cloud struct {
	awsConfig             aws.Config
	region                string
	ec2                   EC2API
	sm                    SageMakerAPI
	dm                    dm.DeviceManager
	bm                    *batcherManager
	rm                    *retryManager
	vwp                   volumeWaitParameters
	likelyBadDeviceNames  expiringcache.ExpiringCache[string, sync.Map]
	latestClientTokens    expiringcache.ExpiringCache[string, int]
	volumeInitializations expiringcache.ExpiringCache[string, volumeInitialization]
	latestIOPSLimits      expiringcache.ExpiringCache[string, iopsLimits]
	accountID             string
	accountIDOnce         sync.Once
	attemptDryRun         atomic.Bool
}

var _ Cloud = &cloud{}

// NewCloud returns a new instance of AWS cloud
// It panics if session is invalid.
func NewCloud(region string, awsSdkDebugLog bool, userAgentExtra string, batchingEnabled bool, deprecatedMetrics bool) Cloud {
	cfg, err := config.LoadDefaultConfig(context.Background(), config.WithRegion(region))
	if err != nil {
		panic(err)
	}

	if awsSdkDebugLog {
		cfg.ClientLogMode = aws.LogRequestWithBody | aws.LogResponseWithBody
	}

	// Set the env var so that the session appends custom user agent string
	if userAgentExtra != "" {
		if err := os.Setenv("AWS_EXECUTION_ENV", "aws-ebs-csi-driver-"+driverVersion+"-"+userAgentExtra); err != nil {
			klog.ErrorS(err, "Failed to set AWS_EXECUTION_ENV")
		}
	} else {
		if err := os.Setenv("AWS_EXECUTION_ENV", "aws-ebs-csi-driver-"+driverVersion); err != nil {
			klog.ErrorS(err, "Failed to set AWS_EXECUTION_ENV")
		}
	}

	svc := ec2.NewFromConfig(cfg, func(o *ec2.Options) {
		o.APIOptions = append(o.APIOptions,
			RecordRequestsMiddleware(deprecatedMetrics),
			LogServerErrorsMiddleware(), // This middlware should always be last so it sees an unmangled error
		)

		endpoint := os.Getenv("AWS_EC2_ENDPOINT")
		if endpoint != "" {
			o.BaseEndpoint = &endpoint
		}

		o.RetryMaxAttempts = retryMaxAttempt
	})

	// Create SageMaker client
	smClient := sagemaker.NewFromConfig(cfg, func(o *sagemaker.Options) {
		o.RetryMaxAttempts = retryMaxAttempt

		// Allow custom SageMaker endpoint for testing
		endpoint := os.Getenv("AWS_SAGEMAKER_ENDPOINT")
		if endpoint != "" {
			o.BaseEndpoint = &endpoint
		}
	})

	var bm *batcherManager
	if batchingEnabled {
		klog.V(4).InfoS("NewCloud: batching enabled")
		bm = newBatcherManager(svc)
	}

	c := &cloud{
		awsConfig:             cfg,
		region:                region,
		dm:                    dm.NewDeviceManager(),
		ec2:                   svc,
		sm:                    smClient,
		bm:                    bm,
		rm:                    newRetryManager(),
		vwp:                   vwp,
		likelyBadDeviceNames:  expiringcache.New[string, sync.Map](cacheForgetDelay),
		latestClientTokens:    expiringcache.New[string, int](cacheForgetDelay),
		volumeInitializations: expiringcache.New[string, volumeInitialization](volInitCacheForgetDelay),
		latestIOPSLimits:      expiringcache.New[string, iopsLimits](iopsLimitCacheForgetDelay),
	}

	// Ensure an EC2 Dry-run API call is made on startup and every dryRunInterval
	c.attemptDryRun.Store(true)
	go func() {
		for range time.Tick(dryRunInterval) {
			c.attemptDryRun.Store(true)
		}
	}()

	return c
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
		volumeStatusIDBatcherSlow: batcher.New(1000, slowVolumeStatusBatchMaxDelay, func(ids []string) (map[string]*types.VolumeStatusItem, error) {
			return execBatchDescribeVolumeStatus(svc, ids)
		}),
		volumeStatusIDBatcherFast: batcher.New(1000, fastVolumeStatusBatchMaxDelay, func(ids []string) (map[string]*types.VolumeStatusItem, error) {
			return execBatchDescribeVolumeStatus(svc, ids)
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
		return nil, errors.New("execBatchDescribeVolumes: unsupported request type")
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
		createType string
		iops       int32
		err        error
		size       int32
		outpostArn string
		volumeID   string
	)

	isClone := diskOptions.SourceVolumeID != ""

	capacityGiB := util.BytesToGiB(diskOptions.CapacityBytes)

	if diskOptions.IOPS > 0 && diskOptions.IOPSPerGB > 0 {
		return nil, errors.New("invalid StorageClass parameters; specify either IOPS or IOPSPerGb, not both")
	}

	createType = diskOptions.VolumeType
	// If no volume type is specified, GP3 is used as default for newly created volumes.
	if createType == "" {
		createType = VolumeTypeGP3
	}

	if diskOptions.MultiAttachEnabled && createType != VolumeTypeIO2 {
		return nil, errors.New("CreateDisk: multi-attach is only supported for io2 volumes")
	}

	tags := make([]types.Tag, 0, len(diskOptions.Tags))
	for key, value := range diskOptions.Tags {
		tags = append(tags, types.Tag{Key: aws.String(key), Value: aws.String(value)})
	}
	tagSpec := types.TagSpecification{
		ResourceType: types.ResourceTypeVolume,
		Tags:         tags,
	}

	zone := diskOptions.AvailabilityZone
	zoneID := diskOptions.AvailabilityZoneID
	if zone == "" && zoneID == "" {
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

	azParams := getVolumeLimitsParams{
		availabilityZone:   zone,
		availabilityZoneId: zoneID,
		outpostArn:         diskOptions.OutpostArn,
	}

	iopsLimits := c.getVolumeLimits(ctx, createType, azParams)

	if diskOptions.IOPS > 0 {
		iops = diskOptions.IOPS
	} else if diskOptions.IOPSPerGB > 0 {
		iops = diskOptions.IOPSPerGB * capacityGiB
	}
	if iops > 0 {
		iops = capIOPS(createType, capacityGiB, iops, iopsLimits, diskOptions.AllowIOPSPerGBIncrease)
	}

	if isClone {
		copyRequestInput := &ec2.CopyVolumesInput{
			SourceVolumeId:     aws.String(diskOptions.SourceVolumeID),
			ClientToken:        aws.String(hex.EncodeToString(clientToken[:])),
			Size:               aws.Int32(capacityGiB),
			VolumeType:         types.VolumeType(createType),
			MultiAttachEnabled: aws.Bool(diskOptions.MultiAttachEnabled),
			TagSpecifications:  []types.TagSpecification{tagSpec},
		}
		size, outpostArn, volumeID, err = c.createCloneHelper(ctx, copyRequestInput, iops, diskOptions.Throughput)
	} else {
		createRequestInput := &ec2.CreateVolumeInput{
			ClientToken:        aws.String(hex.EncodeToString(clientToken[:])),
			Size:               aws.Int32(capacityGiB),
			VolumeType:         types.VolumeType(createType),
			Encrypted:          aws.Bool(diskOptions.Encrypted),
			MultiAttachEnabled: aws.Bool(diskOptions.MultiAttachEnabled),
			TagSpecifications:  []types.TagSpecification{tagSpec},
		}
		size, outpostArn, volumeID, err = c.createVolumeHelper(ctx, diskOptions, createRequestInput, iops, diskOptions.Throughput, zone, zoneID)
	}
	if err != nil {
		switch {
		case isAWSErrorSnapshotNotFound(err):
			return nil, ErrNotFound
		case isAWSErrorIdempotentParameterMismatch(err):
			nextTokenNumber := 2
			if tokenNumber, ok := c.latestClientTokens.Get(volumeName); ok {
				nextTokenNumber = *tokenNumber + 1
			}
			c.latestClientTokens.Set(volumeName, &nextTokenNumber)
			return nil, ErrIdempotentParameterMismatch
		case isAWSErrorInvalidParameterCombination(err):
			return nil, fmt.Errorf("%w: %w", ErrInvalidArgument, err)
		case isAWSErrorVolumeLimitExceeded(err):
			// EC2 API does NOT handle idempotency correctly when a theoretical volume
			// would put the caller over a limit for their account
			//
			// To avoid leaking volumes, make a DescribeVolumes call here
			request := &ec2.DescribeVolumesInput{
				Filters: []types.Filter{
					{
						Name:   aws.String("tag:" + VolumeNameTagKey),
						Values: []string{volumeName},
					},
				},
			}
			// Call DescribeVolumes directly as there is a high chance this volume
			// will return a NotFound error and would poison a batch call
			volumes, describeErr := describeVolumes(ctx, c.ec2, request)
			if describeErr != nil {
				if isAWSErrorVolumeNotFound(describeErr) {
					return nil, fmt.Errorf("%w: %w", ErrLimitExceeded, err)
				} else {
					return nil, describeErr
				}
			}

			// Volume with requested name exists, continue with it
			if l := len(volumes); l > 1 {
				return nil, ErrMultiDisks
			} else if l < 1 {
				// This should in theory be impossible, but if the API
				// changes or breaks it would cause a panic, so handle it
				return nil, fmt.Errorf("%w: %w", ErrLimitExceeded, err)
			}
			volumeID = aws.ToString(volumes[0].VolumeId)
			size = aws.ToInt32(volumes[0].Size)
			outpostArn = aws.ToString(volumes[0].OutpostArn)
		case isAwsErrorMaxIOPSLimitExceeded(err):
			return nil, fmt.Errorf("%w: %w", ErrLimitExceeded, err)
		default:
			return nil, fmt.Errorf("could not create volume in EC2: %w", err)
		}
	}

	if len(volumeID) == 0 {
		return nil, errors.New("volume ID was not returned by CreateVolume")
	}

	if size == 0 {
		return nil, errors.New("disk size was not returned by CreateVolume")
	}

	volume, err := c.waitForVolume(ctx, volumeID)
	if err != nil {
		return nil, fmt.Errorf("timed out waiting for volume to create: %w", err)
	}

	klog.V(7).InfoS("CreateDisk: volume created successfully", "volumeName", volumeName, "volume", volume)

	return &Disk{CapacityGiB: size, VolumeID: volumeID, AvailabilityZone: zone, SnapshotID: diskOptions.SnapshotID, SourceVolumeID: diskOptions.SourceVolumeID, OutpostArn: outpostArn}, nil
}

func (c *cloud) createCloneHelper(ctx context.Context, input *ec2.CopyVolumesInput, iops int32, throughput int32) (int32, string, string, error) {
	if iops > 0 {
		input.Iops = aws.Int32(iops)
	}
	if throughput > 0 {
		input.Throughput = aws.Int32(throughput)
	}
	copyResponse, err := c.ec2.CopyVolumes(ctx, input, func(o *ec2.Options) {
		o.Retryer = c.rm.copyVolumeRetryer
	})
	if err != nil {
		return 0, "", "", err
	}
	if len(copyResponse.Volumes) != 1 {
		return 0, "", "", errors.New("copyResponse does not contain volume information")
	}
	return *copyResponse.Volumes[0].Size, aws.ToString(copyResponse.Volumes[0].OutpostArn), aws.ToString(copyResponse.Volumes[0].VolumeId), nil
}

func (c *cloud) createVolumeHelper(ctx context.Context, diskOptions *DiskOptions, input *ec2.CreateVolumeInput, iops int32, throughput int32, zone string, zoneID string) (int32, string, string, error) {
	if len(zone) > 0 {
		input.AvailabilityZone = aws.String(zone)
	}
	if len(zoneID) > 0 {
		input.AvailabilityZoneId = aws.String(zoneID)
	}
	if len(diskOptions.OutpostArn) > 0 {
		input.OutpostArn = aws.String(diskOptions.OutpostArn)
	}

	if len(diskOptions.KmsKeyID) > 0 {
		input.KmsKeyId = aws.String(diskOptions.KmsKeyID)
		input.Encrypted = aws.Bool(true)
	}

	if iops > 0 {
		input.Iops = aws.Int32(iops)
	}

	if diskOptions.Throughput > 0 {
		input.Throughput = aws.Int32(throughput)
	}
	snapshotID := diskOptions.SnapshotID
	if len(snapshotID) > 0 {
		input.SnapshotId = aws.String(snapshotID)
	}
	if diskOptions.VolumeInitializationRate > 0 {
		input.VolumeInitializationRate = aws.Int32(diskOptions.VolumeInitializationRate)
	}

	createResponse, err := c.ec2.CreateVolume(ctx, input, func(o *ec2.Options) {
		o.Retryer = c.rm.createVolumeRetryer
	})
	if err != nil {
		return 0, "", "", err
	}
	return *createResponse.Size, aws.ToString(createResponse.OutpostArn), aws.ToString(createResponse.VolumeId), nil
}

// execBatchDescribeVolumesModifications executes a batched DescribeVolumesModifications API call.
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
	request := &ec2.DescribeVolumesInput{
		VolumeIds: []string{volumeID},
	}
	volume, err := c.getVolume(ctx, request)
	if err != nil {
		return 0, err
	}

	needsModification, volumeSize, err := c.validateVolumeState(ctx, volumeID, newSizeGiB, *volume.Size, options)
	if err != nil || !needsModification {
		return volumeSize, err
	}

	if options.IOPS > 0 && options.IOPSPerGB > 0 {
		return 0, errors.New("invalid VAC parameter; specify either IOPS or IOPSPerGb, not both")
	}

	req := &ec2.ModifyVolumeInput{
		VolumeId: aws.String(volumeID),
	}
	if newSizeBytes != 0 {
		req.Size = aws.Int32(newSizeGiB)
	}
	volTypeToUse := volume.VolumeType
	if options.VolumeType != "" {
		req.VolumeType = types.VolumeType(options.VolumeType)
		volTypeToUse = req.VolumeType
	}
	if options.Throughput != 0 {
		req.Throughput = aws.Int32(options.Throughput)
	}

	var sizeToUse int32
	if req.Size != nil {
		sizeToUse = *req.Size
	} else {
		sizeToUse = *volume.Size
	}

	allowAutoIncreaseIsSet, iopsPerGbVal, err := c.checkIfIopsIncreaseOnExpansion(volume.Tags)
	if err != nil {
		return 0, err
	}
	var iopsForModify int32
	switch {
	case options.IOPS != 0:
		iopsForModify = options.IOPS
	case options.IOPSPerGB != 0 && (options.AllowIopsIncreaseOnResize || allowAutoIncreaseIsSet):
		iopsForModify = sizeToUse * options.IOPSPerGB
	case iopsPerGbVal > 0 && (options.AllowIopsIncreaseOnResize || allowAutoIncreaseIsSet):
		iopsForModify = sizeToUse * iopsPerGbVal
	}
	if iopsForModify != 0 {
		azParams := getVolumeLimitsParams{}

		if volume.AvailabilityZone != nil {
			azParams.availabilityZone = *volume.AvailabilityZone
		}
		if volume.AvailabilityZoneId != nil {
			azParams.availabilityZoneId = *volume.AvailabilityZoneId
		}
		// EBS doesn't handle empty outpost arn, so we have to include it only when it's non-empty
		if volume.OutpostArn != nil {
			azParams.outpostArn = *volume.OutpostArn
		}
		iopsLimits := c.getVolumeLimits(ctx, string(volTypeToUse), azParams)
		req.Iops = aws.Int32(capIOPS(string(volTypeToUse), sizeToUse, iopsForModify, iopsLimits, true))
		options.IOPS = *req.Iops
	}

	needsModification, volumeSize, err = c.validateModifyVolume(ctx, volumeID, newSizeGiB, options, *volume)
	if err != nil || !needsModification {
		return volumeSize, err
	}

	response, err := c.ec2.ModifyVolume(ctx, req, func(o *ec2.Options) {
		o.Retryer = c.rm.modifyVolumeRetryer
	})
	if err != nil {
		if isAWSErrorInvalidParameter(err) {
			// Wrap error to preserve original message from AWS as to why this was an invalid argument
			return 0, fmt.Errorf("%w: %w", ErrInvalidArgument, err)
		}
		if isAWSErrorVolumeModificationSizeLimitExceeded(err) {
			return 0, fmt.Errorf("%w: %w", ErrLimitExceeded, err)
		}
		if isAwsErrorMaxIOPSLimitExceeded(err) {
			return 0, fmt.Errorf("%w: %w", ErrLimitExceeded, err)
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
	return c.checkDesiredState(ctx, volumeID, newSizeGiB, options)
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

// execBatchDescribeInstances executes a batched DescribeInstances API call.
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
	if isHyperPodNode(nodeID) {
		return c.attachDiskHyperPod(ctx, volumeID, nodeID)
	}

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
			if isAWSErrorAttachmentLimitExceeded(attachErr) {
				return "", fmt.Errorf("%w: %w", ErrLimitExceeded, attachErr)
			}
			return "", fmt.Errorf("could not attach volume %q to node %q: %w", volumeID, nodeID, attachErr)
		}
		likelyBadDeviceNames.Delete(device.Path)
		klog.V(5).InfoS("[Debug] AttachVolume", "volumeID", volumeID, "nodeID", nodeID, "resp", resp)
	}

	_, err = c.WaitForAttachmentState(ctx, types.VolumeAttachmentStateAttached, volumeID, *instance.InstanceId, device.Path, device.IsAlreadyAssigned)

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

func (c *cloud) attachDiskHyperPod(ctx context.Context, volumeID, nodeID string) (string, error) {
	klog.V(2).InfoS("AttachDisk: HyperPod node detected", "volumeID", volumeID, "nodeID", nodeID)

	instanceID := getInstanceIDFromHyperPodNode(nodeID)
	accountID, err := c.getAccountID(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to get account ID: %w", err)
	}
	clusterArn := buildHyperPodClusterArn(nodeID, c.region, accountID)

	klog.V(5).InfoS("HyperPod attachment details",
		"volumeID", volumeID,
		"instanceID", instanceID,
		"clusterArn", clusterArn)

	// Construct real SageMaker AttachClusterNodeVolumeInput
	input := &sagemaker.AttachClusterNodeVolumeInput{
		ClusterArn: aws.String(clusterArn),
		NodeId:     aws.String(instanceID),
		VolumeId:   aws.String(volumeID),
	}

	klog.V(5).InfoS("Calling AttachClusterNodeVolume", "input", input)

	resp, attachErr := c.sm.AttachClusterNodeVolume(ctx, input)
	if attachErr != nil {
		if isAWSHyperPodErrorAttachmentLimitExceeded(attachErr) {
			return "", fmt.Errorf("%w: %w", ErrLimitExceeded, attachErr)
		}
		return "", fmt.Errorf("could not attach volume %q to node %q: %w", volumeID, nodeID, attachErr)
	}

	klog.V(5).InfoS("[Debug] AttachVolume", "volumeID", volumeID, "nodeID", nodeID, "resp", resp)

	// Wait for attachment completion
	deviceName := aws.ToString(resp.DeviceName)
	_, err = c.WaitForAttachmentState(
		ctx,
		types.VolumeAttachmentStateAttached,
		volumeID,
		nodeID,
		deviceName,
		false,
	)
	if err != nil {
		return "", fmt.Errorf("error waiting for volume attachment: %w", err)
	}

	klog.V(5).InfoS("Volume attached from HyperPod node successfully",
		"volumeID", volumeID,
		"nodeID", nodeID,
		"deviceName", deviceName)

	return deviceName, nil
}

func (c *cloud) DetachDisk(ctx context.Context, volumeID, nodeID string) error {
	if isHyperPodNode(nodeID) {
		return c.detachDiskHyperPod(ctx, volumeID, nodeID)
	}
	instance, err := c.getInstance(ctx, nodeID)
	if err != nil {
		return err
	}

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
			metrics.AsyncEC2Metrics().ClearDetachMetric(volumeID, nodeID)
			return ErrNotFound
		}
		return fmt.Errorf("could not detach volume %q from node %q: %w", volumeID, nodeID, err)
	}

	attachment, err := c.WaitForAttachmentState(ctx, types.VolumeAttachmentStateDetached, volumeID, *instance.InstanceId, "", false)
	if err != nil {
		return err
	}
	if attachment != nil {
		// We expect it to be nil, it is (maybe) interesting if it is not
		klog.V(2).InfoS("waitForAttachmentState returned non-nil attachment with state=detached", "attachment", attachment)
	}
	metrics.AsyncEC2Metrics().ClearDetachMetric(volumeID, nodeID)

	return nil
}

func (c *cloud) detachDiskHyperPod(ctx context.Context, volumeID, nodeID string) error {
	klog.V(2).InfoS("DetachDisk: HyperPod node detected", "volumeID", volumeID, "nodeID", nodeID)

	instanceID := getInstanceIDFromHyperPodNode(nodeID)
	accountID, err := c.getAccountID(ctx)
	if err != nil {
		return fmt.Errorf("failed to get account ID: %w", err)
	}
	clusterArn := buildHyperPodClusterArn(nodeID, c.region, accountID)

	klog.V(4).InfoS("HyperPod detachment details",
		"volumeID", volumeID,
		"instanceID", instanceID,
		"clusterArn", clusterArn)

	// Construct real SageMaker DetachClusterNodeVolumeInput
	input := &sagemaker.DetachClusterNodeVolumeInput{
		ClusterArn: aws.String(clusterArn),
		NodeId:     aws.String(instanceID),
		VolumeId:   aws.String(volumeID),
	}
	klog.V(4).InfoS("Calling DetachClusterNodeVolumeInput", "input", input)

	_, err = c.sm.DetachClusterNodeVolume(ctx, input)
	if err != nil {
		if isAWSHyperPodErrorIncorrectState(err) ||
			isAWSHyperPodErrorInvalidAttachmentNotFound(err) ||
			isAWSHyperPodErrorVolumeNotFound(err) {
			metrics.AsyncEC2Metrics().ClearDetachMetric(volumeID, nodeID)
			return ErrNotFound
		}
		return fmt.Errorf("could not detach volume %q from node %q: %w", volumeID, nodeID, err)
	}

	klog.V(5).InfoS("[Debug] DetachVolume", "volumeID", volumeID, "nodeID", nodeID)
	// Wait for detachment completion
	_, err = c.WaitForAttachmentState(
		ctx,
		types.VolumeAttachmentStateDetached,
		volumeID,
		nodeID,
		"",
		false,
	)
	if err != nil {
		return fmt.Errorf("error waiting for volume detachment: %w", err)
	}

	klog.V(5).InfoS("Volume detached from HyperPod node successfully",
		"volumeID", volumeID,
		"nodeID", nodeID)

	return nil
}

type volumeInitialization struct {
	initialized                 bool
	estimatedInitializationTime time.Time
}

// IsVolumeInitialized calls EC2 DescribeVolumeStatus and returns whether the volume is initialized.
func (c *cloud) IsVolumeInitialized(ctx context.Context, volumeID string) (bool, error) {
	var volumeStatusItem *types.VolumeStatusItem
	var err error

	// Because volumes can take hours to initialize, we shouldn't poll DescribeVolumeStatus (DVS) as aggressively as we
	// do for other EC2 APIs.
	//
	// We use a volumeInitializations cache to keep track of initializing volumes that we should poll at a slower rate.
	//
	// Furthermore, if initializationRate was set during volume creation, DVS returns an estimated initialization time.
	// We cache that estimate and defer polling of DVS until we reach that time.
	// We clamp to a minimum of 1 min because as of July 2025 it can take up to 5 min for volume initialization info to update.

	// Check volumeInitializations cache to potentially delay EC2 DescribeVolumeStatus call
	volInit, ok := c.volumeInitializations.Get(volumeID)
	switch {
	// Case 1: We've never called DVS for volume. Call DVS ASAP.
	case !ok:
		volumeStatusItem, err = c.describeVolumeStatus(volumeID, true /* callASAP */)
	// Case 2: We already know volume is initialized. Don't call DVS.
	case volInit.initialized:
		return true, nil
	// Case 3: We know volume is initializing, but there is no SLA. Call DVS eventually during next slow batch.
	case volInit.estimatedInitializationTime.IsZero():
		volumeStatusItem, err = c.describeVolumeStatus(volumeID, false /* callASAP */)
	// Case 4: We have an estimated time for initialization. Wait to call DVS again until then unless RPC ctx is done.
	case !volInit.initialized:
		util.WaitUntilTimeOrContext(ctx, volInit.estimatedInitializationTime)
		if err := ctx.Err(); err != nil {
			return false, err
		}
		volumeStatusItem, err = c.describeVolumeStatus(volumeID, true /* callASAP */)
	}
	if err != nil {
		return false, err
	}

	// Parse volume status
	if volumeStatusItem == nil || volumeStatusItem.VolumeStatus == nil || volumeStatusItem.VolumeStatus.Details == nil {
		return false, errors.New("IsVolumeInitialized: EC2 DescribeVolumeStatus response missing volume status details")
	}
	isVolInitializing := isVolumeStatusInitializing(*volumeStatusItem)

	// Update cache
	var newExpectedInitTime time.Time
	if isVolInitializing && volumeStatusItem.InitializationStatusDetails != nil && volumeStatusItem.InitializationStatusDetails.EstimatedTimeToCompleteInSeconds != nil {
		secondsLeft := *volumeStatusItem.InitializationStatusDetails.EstimatedTimeToCompleteInSeconds
		klog.V(4).InfoS("IsVolumeInitialized: volume still initializing according to EC2 DescribeVolumeStatus", "volumeID", volumeID, "estimatedTimeToCompleteInSeconds", secondsLeft)
		// Clamp to a minimum of 1 min because as of July 2025 it can take up to 5 min for volume initialization info to update.
		if secondsLeft < 60 {
			secondsLeft = 60
		}
		newExpectedInitTime = time.Now().Add(time.Duration(secondsLeft) * time.Second)
	}
	c.volumeInitializations.Set(volumeID, &volumeInitialization{initialized: !isVolInitializing, estimatedInitializationTime: newExpectedInitTime})

	if isVolInitializing {
		klog.V(4).InfoS("IsVolumeInitialized: volume not initialized yet", "volumeID", volumeID)
	} else {
		klog.V(4).InfoS("IsVolumeInitialized: volume is initialized", "volumeID", volumeID)
	}

	return !isVolInitializing, nil
}

func isVolumeStatusInitializing(vsi types.VolumeStatusItem) bool {
	for _, detail := range vsi.VolumeStatus.Details {
		if detail.Name == types.VolumeStatusNameInitializationState && detail.Status != nil && *detail.Status == VolumeStatusInitializingState {
			return true
		}
	}
	return false
}

func execBatchDescribeVolumeStatus(svc EC2API, input []string) (map[string]*types.VolumeStatusItem, error) {
	klog.V(7).InfoS("execBatchDescribeVolumeStatus", "volumeIds", input)
	request := &ec2.DescribeVolumeStatusInput{
		VolumeIds: input,
	}

	ctx, cancel := context.WithTimeout(context.Background(), batchDescribeTimeout)
	defer cancel()

	var volumeStatusItems []types.VolumeStatusItem
	var nextToken *string
	for {
		response, err := svc.DescribeVolumeStatus(ctx, request)
		if err != nil {
			return nil, err
		}
		volumeStatusItems = append(volumeStatusItems, response.VolumeStatuses...)
		nextToken = response.NextToken
		if aws.ToString(nextToken) == "" {
			break
		}
		request.NextToken = nextToken
	}

	result := make(map[string]*types.VolumeStatusItem)

	for _, m := range volumeStatusItems {
		volumeStatus := m
		result[*volumeStatus.VolumeId] = &volumeStatus
	}

	klog.V(7).InfoS("execBatchDescribeVolumeStatus: success", "result", result)
	return result, nil
}

// describeVolumeStatus will return the VolumeStatusItem associated with volumeID from EC2 DescribeVolumeStatus
// Set callASAP to true if you need status within seconds (Otherwise it may take minutes).
func (c *cloud) describeVolumeStatus(volumeID string, callASAP bool) (*types.VolumeStatusItem, error) {
	ch := make(chan batcher.BatchResult[*types.VolumeStatusItem])

	var b *batcher.Batcher[string, *types.VolumeStatusItem]
	if callASAP {
		b = c.bm.volumeStatusIDBatcherFast
	} else {
		b = c.bm.volumeStatusIDBatcherSlow
	}
	b.AddTask(volumeID, ch)

	r := <-ch

	if r.Err != nil {
		return nil, r.Err
	}
	return r.Result, nil
}

// WaitForAttachmentState polls until the attachment status is the expected value.
func (c *cloud) WaitForAttachmentState(ctx context.Context, expectedState types.VolumeAttachmentState, volumeID string, expectedInstance string, expectedDevice string, alreadyAssigned bool) (*types.VolumeAttachment, error) {
	var attachment *types.VolumeAttachment
	isHyperPod := isHyperPodNode(expectedInstance)

	verifyVolumeFunc := func(ctx context.Context) (bool, error) {
		request := &ec2.DescribeVolumesInput{
			VolumeIds: []string{volumeID},
		}

		volume, err := c.getVolume(ctx, request)
		if err != nil {
			// The VolumeNotFound error is special -- we don't need to wait for it to repeat
			if isAWSErrorVolumeNotFound(err) {
				if expectedState == types.VolumeAttachmentStateDetached {
					// The disk doesn't exist, assume it's detached, log warning and stop waiting
					klog.InfoS("Waiting for volume to be detached but the volume does not exist", "volumeID", volumeID)
					return true, nil
				}
				if expectedState == types.VolumeAttachmentStateAttached {
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

		var attachmentState types.VolumeAttachmentState

		for _, a := range volume.Attachments {
			if isHyperPod {
				if a.AssociatedResource != nil {
					instanceID, err := getInstanceIDFromAssociatedResource(aws.ToString(a.AssociatedResource))
					if err != nil {
						return false, err
					}
					if instanceID == getInstanceIDFromHyperPodNode(expectedInstance) {
						attachmentState = a.State
						attachment = &a
					}
				}
			} else if a.InstanceId != nil && aws.ToString(a.InstanceId) == expectedInstance {
				attachmentState = a.State
				attachment = &a
			}
		}

		if attachmentState == "" {
			attachmentState = types.VolumeAttachmentStateDetached
		}

		if attachment != nil && attachment.Device != nil && expectedState == types.VolumeAttachmentStateAttached && !isHyperPod {
			device := aws.ToString(attachment.Device)
			if device != expectedDevice {
				klog.InfoS("WaitForAttachmentState: device mismatch", "device", device, "expectedDevice", expectedDevice, "attachment", attachment)
				return false, nil
			}
		}

		// if we expected volume to be attached and it was reported as already attached via DescribeInstance call
		// but DescribeVolume told us volume is detached, we will short-circuit this long wait loop and return error
		// so as AttachDisk can be retried without waiting for 20 minutes.
		if (expectedState == types.VolumeAttachmentStateAttached) && alreadyAssigned && (attachmentState == types.VolumeAttachmentStateDetached) {
			request := &ec2.AttachVolumeInput{
				Device:     aws.String(expectedDevice),
				InstanceId: aws.String(expectedInstance),
				VolumeId:   aws.String(volumeID),
			}
			_, err := c.ec2.AttachVolume(ctx, request)
			if err != nil {
				return false, fmt.Errorf("WaitForAttachmentState AttachVolume error, expected device to be attached but was %s, volumeID=%q, instanceID=%q, Device=%q, err=%w", attachmentState, volumeID, expectedInstance, expectedDevice, err)
			}
			return false, fmt.Errorf("attachment of disk %q failed, expected device to be attached but was %s", volumeID, attachmentState)
		}

		// Attachment is in requested state, finish waiting
		if attachmentState == expectedState {
			// But first, reset attachment to nil if expectedState equals volumeDetachedState.
			// Caller will not expect an attachment to be returned for a detached volume if we're not also returning an error.
			if expectedState == types.VolumeAttachmentStateDetached {
				attachment = nil
			}
			return true, nil
		}
		// continue waiting
		klog.InfoS("Waiting for volume state", "volumeID", volumeID, "actual", attachmentState, "desired", expectedState)

		if expectedState == types.VolumeAttachmentStateDetached {
			metrics.AsyncEC2Metrics().TrackDetachment(volumeID, expectedInstance, attachmentState)
		}
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
		KmsKeyID:         aws.ToString(volume.KmsKeyId),
	}

	if volume.Size != nil {
		disk.CapacityGiB = *volume.Size
	}

	return disk, nil
}

func (c *cloud) GetVolumeIDByNodeAndDevice(ctx context.Context, nodeID string, deviceName string) (string, error) {
	instance, err := c.getInstance(ctx, nodeID)
	if err != nil {
		return "", fmt.Errorf("failed to get instance %s: %w", nodeID, err)
	}

	if instance.RootDeviceName != nil && *instance.RootDeviceName == deviceName {
		return "", fmt.Errorf("device %s is the root device: %w", deviceName, ErrInvalidRequest)
	}

	for _, bdm := range instance.BlockDeviceMappings {
		if bdm.DeviceName != nil && *bdm.DeviceName == deviceName {
			if bdm.Ebs != nil && bdm.Ebs.VolumeId != nil {
				return *bdm.Ebs.VolumeId, nil
			}
		}
	}

	return "", fmt.Errorf("volume not found at device %s on node %s: %w", deviceName, nodeID, ErrNotFound)
}

func isHyperPodNode(nodeID string) bool {
	return strings.HasPrefix(nodeID, "hyperpod-")
}

// Only for hyperpod node, getInstanceIDFromHyperPodNode extracts the EC2 instance ID from a HyperPod node ID.
func getInstanceIDFromHyperPodNode(nodeID string) string {
	parts := strings.SplitN(nodeID, "-", 3)
	return parts[2]
}

// Only for hyperpod node, buildHyperPodClusterArn: arn:aws:sagemaker:region:account:cluster/clusterID.
func buildHyperPodClusterArn(nodeID string, region string, accountID string) string {
	parts := strings.Split(nodeID, "-")
	return fmt.Sprintf("arn:aws:sagemaker:%s:%s:cluster/%s", region, accountID, parts[1])
}

// For hyperpod node, AssociatedResource is in arn:aws:sagemaker:region:account:cluster/clusterID-instanceId format.
func getInstanceIDFromAssociatedResource(arn string) (string, error) {
	parts := strings.Split(arn, "-")
	if len(parts) < 2 {
		return "", fmt.Errorf("invalid ARN format: %s", arn)
	}
	lastTwo := parts[len(parts)-2:]
	instanceID := strings.Join(lastTwo, "-")
	if !strings.HasPrefix(instanceID, "i-") {
		return "", fmt.Errorf("invalid instance ID format: %s", instanceID)
	}
	return instanceID, nil
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
		return nil, errors.New("execBatchDescribeSnapshots: unsupported request type")
	}

	ctx, cancel := context.WithTimeout(context.Background(), batchDescribeTimeout)
	defer cancel()

	resp, err := describeSnapshots(ctx, svc, request)
	if err != nil {
		return nil, err
	}

	result := make(map[string]*types.Snapshot)

	for _, snapshot := range resp {
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

	var request *ec2.CreateSnapshotInput

	tags := make([]types.Tag, 0, len(snapshotOptions.Tags))
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
		if isAwsErrorSnapshotLimitExceeded(err) {
			return nil, fmt.Errorf("%w: %w", ErrLimitExceeded, err)
		}
		return nil, fmt.Errorf("error creating snapshot of volume %s: %w", volumeID, err)
	}
	if res == nil {
		return nil, errors.New("nil CreateSnapshotResponse")
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

	snapshots := make([]*Snapshot, 0, len(ec2SnapshotsResponse.Snapshots))
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

// Helper method converting EC2 snapshot type to the internal struct.
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
		var errDetails []string
		for _, r := range response.Unsuccessful {
			for _, e := range r.FastSnapshotRestoreStateErrors {
				errDetails = append(errDetails, fmt.Sprintf("Error Code: %s, Error Message: %s", aws.ToString(e.Error.Code), aws.ToString(e.Error.Message)))
			}
		}
		return nil, errors.New(strings.Join(errDetails, "; "))
	}
	return response, nil
}

// DryRun will make a dry-run EC2 API call. Nil return value means we successfully received EC2 DryRunOperation error code.
func (c *cloud) DryRun(ctx context.Context) error {
	if c.attemptDryRun.Load() {
		// Rely on EC2 DAZ because it is required in ebs controller IAM role, but not in instance default role.
		_, apiErr := c.ec2.DescribeAvailabilityZones(ctx,
			&ec2.DescribeAvailabilityZonesInput{DryRun: aws.Bool(true)},
			func(o *ec2.Options) {
				o.Retryer = aws.NopRetryer{} // Don't retry so we can catch network failures. CO should retry liveness check multiple times.
				o.APIOptions = nil           // Don't add our logging/metrics middleware because we expect errors.
			})
		if apiErr != nil {
			var awsErr smithy.APIError
			if errors.As(apiErr, &awsErr) && awsErr.ErrorCode() == "DryRunOperation" {
				c.attemptDryRun.Store(false)
				return nil
			}
			return fmt.Errorf("dry-run EC2 API call failed: %w", apiErr)
		}
	}

	return nil
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

// GetInstancesPatching returns the instance info associated with each node ID in `nodeIDs` and uses pagination
// to get instances for large clusters. The instances are also described in batches of size up to `maxInstancesDescribed`.
func (c *cloud) GetInstancesPatching(ctx context.Context, nodeIDs []string) ([]*types.Instance, error) {
	var allInstances []*types.Instance

	for i := 0; i < len(nodeIDs); i += maxInstancesDescribed {
		end := i + maxInstancesDescribed
		if end > len(nodeIDs) {
			end = len(nodeIDs)
		}

		batch := nodeIDs[i:end]
		batchInstances, err := c.getInstancesPatchingBatch(ctx, batch)
		if err != nil {
			return nil, err
		}

		allInstances = append(allInstances, batchInstances...)
	}

	return allInstances, nil
}

func (c *cloud) getInstancesPatchingBatch(ctx context.Context, nodeIDs []string) ([]*types.Instance, error) {
	var instances []*types.Instance
	var nextToken *string

	for {
		request := &ec2.DescribeInstancesInput{
			InstanceIds: nodeIDs,
			NextToken:   nextToken,
		}

		response, err := c.ec2.DescribeInstances(ctx, request)
		if err != nil {
			if isAWSErrorInstanceNotFound(err) {
				return nil, ErrNotFound
			}
			return nil, fmt.Errorf("error listing AWS instances: %w", err)
		}

		for _, reservation := range response.Reservations {
			for _, instance := range reservation.Instances {
				instances = append(instances, &instance)
			}
		}

		if response.NextToken == nil {
			break
		}
		nextToken = response.NextToken
	}

	return instances, nil
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

// listSnapshots returns all snapshots based from a request.
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
func (c *cloud) waitForVolume(ctx context.Context, volumeID string) (*types.Volume, error) {
	time.Sleep(c.vwp.creationInitialDelay)

	request := &ec2.DescribeVolumesInput{
		VolumeIds: []string{volumeID},
	}

	var volume *types.Volume
	err := wait.ExponentialBackoffWithContext(ctx, c.vwp.creationBackoff, func(ctx context.Context) (done bool, err error) {
		vol, err := c.getVolume(ctx, request)
		if err != nil {
			return true, err
		}
		if vol.State != "" {
			if vol.State == types.VolumeStateAvailable {
				volume = vol
				return true, nil
			}
		}
		return false, nil
	})

	return volume, err
}

// getAccountID returns the account ID of the AWS Account for the IAM credentials in use.
//
// In the first call (or any calls made before the first call succeeds), getAccountID
// will attempt to determine the Account ID via sts:GetCallerIdentity.
// This attempt will retry indefinitely, however getAccountID will return when ctx is cancelled,
// leaving the account ID thread to run in the background.
//
// In subsequent calls (after the first success), getAccountID will use a cached value.
func (c *cloud) getAccountID(ctx context.Context) (string, error) {
	accountIDRetrieved := make(chan struct{}, 1)

	// Start background thread if it isn't already.
	// Intentionally runs in the background until account ID is retrieved, so we don't pass the context.
	//nolint:contextcheck
	go func() {
		c.accountIDOnce.Do(func() {
			for c.accountID == "" {
				cfg, err := config.LoadDefaultConfig(context.Background(), config.WithRegion(c.region))
				if err != nil {
					klog.ErrorS(err, "Failed to create AWS config for account ID retrieval")
				}

				stsClient := sts.NewFromConfig(cfg)
				resp, err := stsClient.GetCallerIdentity(context.Background(), &sts.GetCallerIdentityInput{})
				if err != nil {
					klog.ErrorS(err, "Failed to get AWS account ID, required for HyperPod operations, will retry")
					time.Sleep(getCallerIdentityRetryDelay)
				} else {
					c.accountID = *resp.Account
					klog.V(5).InfoS("Retrieved AWS account ID for HyperPod operations", "accountID", c.accountID)
				}
			}
		})

		// Once.Do blocks until the function exits, even if we aren't the first caller.
		// So the account ID must be available now.
		accountIDRetrieved <- struct{}{}
	}()

	select {
	case <-ctx.Done():
		return "", ctx.Err()

	case <-accountIDRetrieved:
		return c.accountID, nil
	}
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
// error is an AWS InvalidVolumeModification.NotFound error.
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
// This error is reported when the two request contains same client-token but different parameters.
func isAWSErrorIdempotentParameterMismatch(err error) bool {
	return isAWSError(err, "IdempotentParameterMismatch")
}

// isAWSErrorInvalidParameterCombination returns a boolean indicating whether the
// given error is an AWS InvalidParameterCombination error.
// This error is reported when the combination of parameters passed to ec2 makes the request invalid.
func isAWSErrorInvalidParameterCombination(err error) bool {
	return isAWSError(err, "InvalidParameterCombination")
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

// isAWSErrorAttachmentLimitExceeded checks if the error is an AttachmentLimitExceeded error.
// This error is reported when the maximum number of attachments for an instance is exceeded.
func isAWSErrorAttachmentLimitExceeded(err error) bool {
	return isAWSError(err, "AttachmentLimitExceeded")
}

// isAWSHyperPodErrorAttachmentLimitExceeded checks if the error is an AttachmentLimitExceeded error.
// This error is reported when the maximum number of attachments for an instance is exceeded.
func isAWSHyperPodErrorAttachmentLimitExceeded(err error) bool {
	var apiErr smithy.APIError
	if errors.As(err, &apiErr) {
		if apiErr.ErrorCode() == ValidationException && strings.Contains(
			apiErr.ErrorMessage(), "HyperPod - Ec2ErrCode: AttachmentLimitExceeded") {
			return true
		}
	}
	return false
}

// isAWSHyperPodErrorVolumeNotFound returns a boolean indicating whether the
// given error is a ValidationException error. This error is
// reported when the specified volume doesn't exist.
func isAWSHyperPodErrorVolumeNotFound(err error) bool {
	var apiErr smithy.APIError
	if errors.As(err, &apiErr) {
		if apiErr.ErrorCode() == ValidationException && strings.Contains(
			apiErr.ErrorMessage(), "HyperPod - Ec2ErrCode: InvalidVolume.NotFound") {
			return true
		}
	}
	return false
}

// isAWSHyperPodErrorIncorrectState returns a boolean indicating whether the
// given error is a ValidationException error. This error is
// reported when the resource is not in a correct state for the request.
func isAWSHyperPodErrorIncorrectState(err error) bool {
	var apiErr smithy.APIError
	if errors.As(err, &apiErr) {
		if apiErr.ErrorCode() == ValidationException && strings.Contains(
			apiErr.ErrorMessage(), "HyperPod - Ec2ErrCode: IncorrectState") {
			return true
		}
	}
	return false
}

// isAWSHyperPodErrorInvalidAttachmentNotFound returns a boolean indicating whether the
// given error is a ValidationException error. This error is reported
// when attempting to detach a volume from an instance to which it is not attached.
func isAWSHyperPodErrorInvalidAttachmentNotFound(err error) bool {
	var apiErr smithy.APIError
	if errors.As(err, &apiErr) {
		if apiErr.ErrorCode() == ValidationException && strings.Contains(
			apiErr.ErrorMessage(), "HyperPod - Ec2ErrCode: InvalidAttachment.NotFound") {
			return true
		}
	}
	return false
}

// isAWSErrorModificationSizeLimitExceeded checks if the error is a VolumeModificationSizeLimitExceeded error.
// This error is reported when the limit on a volume modification storage in a region is exceeded.
func isAWSErrorVolumeModificationSizeLimitExceeded(err error) bool {
	return isAWSError(err, "VolumeModificationSizeLimitExceeded")
}

// isAWSErrorVolumeLimitExceeded checks if the error is a VolumeLimitExceeded error.
// This error is reported when the limit on the amount of volume storage is exceeded.
func isAWSErrorVolumeLimitExceeded(err error) bool {
	return isAWSError(err, "VolumeLimitExceeded")
}

// isAwsErrorMaxIOPSLimitExceeded checks if the error is a MaxIOPSLimitExceeded error.
// This error is reported when the limit on the IOPS usage for a region is exceeded.
func isAwsErrorMaxIOPSLimitExceeded(err error) bool {
	return isAWSError(err, "MaxIOPSLimitExceeded")
}

// isAwsErrorSnapshotLimitExceeded checks if the error is a SnapshotLimitExceeded error.
// This error is reported when the limit on the number of snapshots that can be created is exceeded.
func isAwsErrorSnapshotLimitExceeded(err error) bool {
	return isAWSError(err, "SnapshotLimitExceeded")
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
	switch {
	case realSizeGiB < desiredSizeGiB:
		return realSizeGiB, fmt.Errorf("volume %q is still being expanded to %d size", volumeID, desiredSizeGiB)
	case options.IOPS != 0 && (volume.Iops == nil || *volume.Iops != options.IOPS):
		return realSizeGiB, fmt.Errorf("volume %q is still being modified to iops %d", volumeID, options.IOPS)
	case options.VolumeType != "" && !strings.EqualFold(string(volume.VolumeType), options.VolumeType):
		return realSizeGiB, fmt.Errorf("volume %q is still being modified to type %q", volumeID, options.VolumeType)
	case options.Throughput != 0 && (volume.Throughput == nil || *volume.Throughput != options.Throughput):
		return realSizeGiB, fmt.Errorf("volume %q is still being modified to throughput %d", volumeID, options.Throughput)
	}

	return realSizeGiB, nil
}

// waitForVolumeModification waits for a volume modification to finish.
func (c *cloud) waitForVolumeModification(ctx context.Context, volumeID string) error {
	waitErr := wait.ExponentialBackoff(c.vwp.modificationBackoff, func() (bool, error) {
		m, err := c.getLatestVolumeModification(ctx, volumeID, true)
		// Consider volumes that have never been modified as done
		if err != nil && errors.Is(err, ErrVolumeNotBeingModified) {
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
				return nil, ErrVolumeNotBeingModified
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
				return nil, ErrVolumeNotBeingModified
			}
			return nil, fmt.Errorf("error describing modifications in volume %q: %w", volumeID, err)
		}

		volumeMods := mod.VolumesModifications
		if len(volumeMods) == 0 {
			return nil, ErrVolumeNotBeingModified
		}

		return &volumeMods[len(volumeMods)-1], nil
	} else {
		return c.batchDescribeVolumesModifications(request)
	}
}

// randomAvailabilityZone returns a random zone from the given region
// the randomness relies on the response of DescribeAvailabilityZones.
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

// AvailabilityZones returns availability zones from the given region.
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

func needsVolumeModification(volume types.Volume, newSizeGiB int32, req *ModifyDiskOptions) bool {
	oldSizeGiB := *volume.Size
	//nolint:staticcheck // staticcheck suggests merging all of the below conditionals into one line,
	// but that would be extremely difficult to read
	needsModification := false

	if oldSizeGiB < newSizeGiB {
		needsModification = true
	}

	if req.IOPS != 0 && (volume.Iops == nil || *volume.Iops != req.IOPS) {
		needsModification = true
	}

	if req.VolumeType != "" && !strings.EqualFold(string(volume.VolumeType), req.VolumeType) {
		needsModification = true
	}

	if req.Throughput != 0 && (volume.Throughput == nil || *volume.Throughput != req.Throughput) {
		needsModification = true
	}

	return needsModification
}

func getVolumeAttachmentsList(volume types.Volume) []string {
	var volumeAttachmentList []string
	for _, attachment := range volume.Attachments {
		if attachment.State == types.VolumeAttachmentStateAttached {
			volumeAttachmentList = append(volumeAttachmentList, aws.ToString(attachment.InstanceId))
		}
	}

	return volumeAttachmentList
}

// Checks if a volume's IOPS can be increased on expansion to adhere to IopsPerGB ratio.
func (c *cloud) checkIfIopsIncreaseOnExpansion(existingTags []types.Tag) (allowAutoIncreaseIsSet bool, iopsPerGbVal int32, err error) {
	for _, tag := range existingTags {
		switch *tag.Key {
		case IOPSPerGBKey:
			iopsPerGbVal64, err := strconv.ParseInt(*tag.Value, 10, 32)
			if err != nil {
				return false, 0, fmt.Errorf("%w: %w", ErrInvalidArgument, err)
			}
			iopsPerGbVal = int32(iopsPerGbVal64)
		case AllowAutoIOPSIncreaseOnModifyKey:
			allowAutoIncreaseIsSet, err = strconv.ParseBool(*tag.Value)
			if err != nil {
				return false, 0, fmt.Errorf("%w: %w", ErrInvalidArgument, err)
			}
		}
	}
	return allowAutoIncreaseIsSet, iopsPerGbVal, nil
}

func (c *cloud) getVolumeModificationState(ctx context.Context, volumeID string) (*types.VolumeModification, error) {
	latestMod, err := c.getLatestVolumeModification(ctx, volumeID, false)
	if err != nil && !errors.Is(err, ErrVolumeNotBeingModified) {
		return nil, fmt.Errorf("error fetching volume modifications for %q: %w", volumeID, err)
	}
	return latestMod, nil
}

func (c *cloud) validateVolumeState(ctx context.Context, volumeID string, newSizeGiB int32, oldSizeGiB int32, options *ModifyDiskOptions) (bool, int32, error) {
	latestMod, err := c.getVolumeModificationState(ctx, volumeID)
	if err != nil {
		return false, 0, err
	}

	// latestMod can be nil if the volume has never been modified
	if latestMod != nil && string(latestMod.ModificationState) == string(types.VolumeModificationStateModifying) {
		// If volume is already modifying, detour to waiting for it to modify
		klog.V(5).InfoS("[Debug] Watching ongoing modification", "volumeID", volumeID)
		err = c.waitForVolumeModification(ctx, volumeID)
		if err != nil {
			return false, oldSizeGiB, err
		}
		returnGiB, returnErr := c.checkDesiredState(ctx, volumeID, newSizeGiB, options)
		return false, returnGiB, returnErr
	}
	return true, 0, nil
}

func (c *cloud) validateModifyVolume(ctx context.Context, volumeID string, newSizeGiB int32, options *ModifyDiskOptions, volume types.Volume) (bool, int32, error) {
	if volume.Size == nil {
		return true, 0, fmt.Errorf("volume %q has no size", volumeID)
	}
	oldSizeGiB := *volume.Size

	// At this point, we know we are starting a new volume modification
	// If we're asked to modify a volume to its current state, ignore the request and immediately return a success
	// This is because as of March 2024, EC2 ModifyVolume calls that don't change any parameters still modify the volume
	if !needsVolumeModification(volume, newSizeGiB, options) {
		klog.V(5).InfoS("[Debug] Skipping modification for volume due to matching stats", "volumeID", volumeID)
		// Wait for any existing modifications to prevent race conditions where DescribeVolume(s) returns the new
		// state before the volume is actually finished modifying
		err := c.waitForVolumeModification(ctx, volumeID)
		if err != nil {
			return true, oldSizeGiB, err
		}
		returnGiB, returnErr := c.checkDesiredState(ctx, volumeID, newSizeGiB, options)
		return false, returnGiB, returnErr
	}

	latestMod, err := c.getVolumeModificationState(ctx, volumeID)
	if err != nil {
		return true, 0, err
	}

	if latestMod != nil && string(latestMod.ModificationState) == string(types.VolumeModificationStateOptimizing) {
		return true, 0, fmt.Errorf("volume %q in OPTIMIZING state, cannot currently modify", volumeID)
	}

	return true, 0, nil
}

func volumeModificationDone(state string) bool {
	return state == string(types.VolumeModificationStateCompleted) || state == string(types.VolumeModificationStateOptimizing)
}

// Calculate actual IOPS for a volume and cap it at supported AWS limits. Any limit of 0 is considered "infinite" (i.e. is not applied).
func capIOPS(volumeType string, requestedCapacityGiB int32, requestedIops int32, iopsLimits iopsLimits, allowIncrease bool) int32 {
	// If requestedIops is zero the user did not request a specific amount, and the default will be used instead
	if requestedIops == 0 {
		return 0
	}

	iops := requestedIops

	if iopsLimits.minIops > 0 && iops < iopsLimits.minIops && allowIncrease {
		iops = iopsLimits.minIops
		klog.V(5).InfoS("[Debug] Increased IOPS to the min supported limit", "volumeType", volumeType, "requestedCapacityGiB", requestedCapacityGiB, "limit", iops)
	}
	if iopsLimits.maxIops > 0 && iops > iopsLimits.maxIops {
		iops = iopsLimits.maxIops
		klog.V(5).InfoS("[Debug] Capped IOPS, volume at the max supported limit", "volumeType", volumeType, "requestedCapacityGiB", requestedCapacityGiB, "limit", iops)
	}
	maxIopsByCapacity := iopsLimits.maxIopsPerGb * requestedCapacityGiB
	if maxIopsByCapacity > 0 && iops > maxIopsByCapacity && maxIopsByCapacity >= iopsLimits.minIops {
		iops = maxIopsByCapacity
		klog.V(5).InfoS("[Debug] Capped IOPS for volume", "volumeType", volumeType, "requestedCapacityGiB", requestedCapacityGiB, "maxIOPSPerGB", iopsLimits.maxIopsPerGb, "limit", iops)
	}
	return iops
}

// Gets IOPS limits for a specific volume type in a specific Zone and caches it. If the limits are cached, simply return limits.
func (c *cloud) getVolumeLimits(ctx context.Context, volumeType string, azParams getVolumeLimitsParams) (iopsLimits iopsLimits) {
	cacheKey := fmt.Sprintf("%s|%s|%s|%s", volumeType, azParams.availabilityZone, azParams.availabilityZoneId, azParams.outpostArn)
	if value, ok := c.latestIOPSLimits.Get(cacheKey); ok {
		return *value
	}

	dryRunRequestInput := &ec2.CreateVolumeInput{
		VolumeType: types.VolumeType(volumeType),
		Size:       aws.Int32(4),
		Iops:       aws.Int32(math.MaxInt32),
		DryRun:     aws.Bool(true),
		// Required by default EBS CSI Driver IAM policy.
		TagSpecifications: []types.TagSpecification{
			{
				ResourceType: types.ResourceTypeVolume,
				Tags: []types.Tag{
					{
						Key:   aws.String(VolumeNameTagKey),
						Value: aws.String("IopsLimitDryRun"),
					},
				},
			},
		},
	}
	if azParams.availabilityZone != "" {
		dryRunRequestInput.AvailabilityZone = aws.String(azParams.availabilityZone)
	}
	if azParams.availabilityZoneId != "" {
		dryRunRequestInput.AvailabilityZoneId = aws.String(azParams.availabilityZoneId)
	}
	if azParams.outpostArn != "" {
		dryRunRequestInput.OutpostArn = aws.String(azParams.outpostArn)
	}

	volType := strings.ToLower(string(dryRunRequestInput.VolumeType))
	_, err := c.ec2.CreateVolume(ctx, dryRunRequestInput, func(o *ec2.Options) {
		o.APIOptions = nil // Don't add our logging/metrics middleware because we expect errors.
	})
	useFallBackLimits := (err == nil) // If DryRun unexpectedly succeeds, we use fallback values.

	if err != nil {
		maxIops, err := extractMaxIOPSFromError(err.Error(), volType)
		// Default To Hardcoded Limits if we can't get the max IOPS from the error message.
		if err != nil {
			klog.V(5).InfoS("[Debug] error getting IOPS limit, defaulting to hardcoded values", "volumeType", volumeType, "error", err.Error())
			useFallBackLimits = true
		} else {
			iopsLimits.maxIops = maxIops
		}
	}

	if useFallBackLimits {
		switch volType {
		case VolumeTypeIO1:
			iopsLimits.maxIops = io1FallbackMaxIOPS
		case VolumeTypeIO2:
			iopsLimits.maxIops = io2FallbackMaxIOPS
		case VolumeTypeGP3:
			iopsLimits.maxIops = gp3FallbackMaxIOPS
		}
	}

	// Set minIops and maxIopsPerGb because we do not fetch these from DryRun Error, we can also catch invalid volume.
	switch volType {
	case VolumeTypeIO1:
		iopsLimits.minIops = io1MinTotalIOPS
		iopsLimits.maxIopsPerGb = io1MaxIOPSPerGB
	case VolumeTypeIO2:
		iopsLimits.minIops = io2MinTotalIOPS
		iopsLimits.maxIopsPerGb = io2MaxIOPSPerGB
	case VolumeTypeGP3:
		iopsLimits.minIops = gp3MinTotalIOPS
		iopsLimits.maxIopsPerGb = gp3MaxIOPSPerGB
	default:
		klog.V(5).InfoS("[Debug] No known limits for volume type, not performing capping", "volumeType", volumeType)
	}

	if !useFallBackLimits {
		c.latestIOPSLimits.Set(cacheKey, &iopsLimits)
	}

	return iopsLimits
}

// Get what the maxIops is from DryRun error message.
func extractMaxIOPSFromError(errorMsg string, volumeType string) (int32, error) {
	// Volume does not support IOPS, so return a limit of 0 (considered infinite in capIOPS).
	if strings.Contains(errorMsg, "parameter iops is not supported") {
		return 0, nil
	}
	// io1 and gp3 have the same error message but io2 has different one, using by default.
	if volumeType == VolumeTypeIO2 {
		if matches := io2ErrRegex.FindStringSubmatch(errorMsg); len(matches) > 1 {
			if val, err := strconv.ParseInt(matches[1], 10, 32); err == nil {
				result := val * 1000
				// No real overflow concern here but adding for safety.
				if result > math.MaxInt32 || result < math.MinInt32 {
					return 0, fmt.Errorf("maximum IOPS value exceeds maximum value of int32: %d", val)
				}
				return int32(result), nil
			}
		}
	} else {
		if matches := nonIo2ErrRegex.FindStringSubmatch(errorMsg); len(matches) > 1 {
			if val, err := strconv.ParseInt(matches[1], 10, 32); err == nil {
				return int32(val), nil
			}
		}
	}

	return 0, fmt.Errorf("error getting IOPS limit, defaulting to hardcoded values for volume type %s", volumeType)
}
