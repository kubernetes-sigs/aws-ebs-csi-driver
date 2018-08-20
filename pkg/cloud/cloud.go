package cloud

import (
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/credentials/ec2rolecreds"
	"github.com/aws/aws-sdk-go/aws/ec2metadata"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/bertinatto/ebs-csi-driver/pkg/util"
	"github.com/golang/glog"
)

const (
	// TODO: what should be the default size?
	// DefaultVolumeSize represents the default volume size.
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

type Compute interface {
	GetMetadata() *Metadata
	CreateDisk(string, *DiskOptions) (*Disk, error)
	DeleteDisk(string) (bool, error)
	AttachDisk(string, string) (string, error)
	DetachDisk(string, string) error
	GetDiskByNameAndSize(string, int64) (*Disk, error)
}

type Cloud struct {
	metadata *Metadata
	ec2      EC2

	// state of our device allocator for each node
	deviceAllocators map[string]DeviceAllocator

	// We keep an active list of devices we have assigned but not yet
	// attached, to avoid a race condition where we assign a device mapping
	// and then get a second request before we attach the volume
	attachingMutex sync.Mutex
	attaching      map[string]map[mountDevice]awsVolumeID
}

var _ Compute = &Cloud{}

func NewCloud() (*Cloud, error) {
	sess, err := session.NewSession(&aws.Config{})
	if err != nil {
		return nil, fmt.Errorf("unable to initialize AWS session: %v", err)
	}

	svc := ec2metadata.New(sess)

	metadata, err := NewMetadata(svc)
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

	return &Cloud{
		metadata:         metadata,
		ec2:              ec2.New(session.New(awsConfig)),
		deviceAllocators: make(map[string]DeviceAllocator),
		attaching:        make(map[string]map[mountDevice]awsVolumeID),
	}, nil
}

func (c *Cloud) GetMetadata() *Metadata {
	return c.metadata
}

func (c *Cloud) CreateDisk(volumeName string, diskOptions *DiskOptions) (*Disk, error) {
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

func (c *Cloud) DeleteDisk(volumeID string) (bool, error) {
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

func (c *Cloud) DescribeInstances(instanceID string) ([]*ec2.Instance, error) {
	// Instances are paged
	results := []*ec2.Instance{}

	request := &ec2.DescribeInstancesInput{
		InstanceIds: []*string{&instanceID},
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
	return results, nil
}

// Gets the mountDevice already assigned to the volume, or assigns an unused mountDevice.
// If the volume is already assigned, this will return the existing mountDevice with alreadyAttached=true.
// Otherwise the mountDevice is assigned by finding the first available mountDevice, and it is returned with alreadyAttached=false.
func (c *Cloud) getMountDevice(instanceID string, info *ec2.Instance, v string, assign bool) (assigned string, alreadyAttached bool, err error) {
	//instanceType := i.getInstanceType()
	//if instanceType == nil {
	//return "", false, fmt.Errorf("could not get instance type for instance: %s", i.awsID)
	//}

	volumeID := awsVolumeID(v)

	deviceMappings := map[mountDevice]awsVolumeID{}
	for _, blockDevice := range info.BlockDeviceMappings {
		name := mountDevice(aws.StringValue(blockDevice.DeviceName))
		if strings.HasPrefix(string(name), "/dev/sd") {
			name = mountDevice(name[7:])
		}
		if strings.HasPrefix(string(name), "/dev/xvd") {
			name = mountDevice(name[8:])
		}
		if len(name) < 1 || len(name) > 2 {
			glog.Warningf("Unexpected EBS DeviceName: %q", aws.StringValue(blockDevice.DeviceName))
		}
		deviceMappings[name] = awsVolumeID(aws.StringValue(blockDevice.Ebs.VolumeId))
	}

	// We lock to prevent concurrent mounts from conflicting
	// We may still conflict if someone calls the API concurrently,
	// but the AWS API will then fail one of the two attach operations
	c.attachingMutex.Lock()
	defer c.attachingMutex.Unlock()

	for mountDevice, volume := range c.attaching[instanceID] {
		deviceMappings[mountDevice] = volume
	}

	// Check to see if this volume is already assigned a device on this machine
	for mountDevice, mappingVolumeID := range deviceMappings {
		if volumeID == mappingVolumeID {
			if assign {
				glog.Warningf("Got assignment call for already-assigned volume: %s@%s", mountDevice, mappingVolumeID)
			}
			return string(mountDevice), true, nil
		}
	}

	if !assign {
		return "", false, nil
	}

	// Find the next unused device name
	deviceAllocator := c.deviceAllocators[instanceID]
	if deviceAllocator == nil {
		// we want device names with two significant characters, starting with /dev/xvdbb
		// the allowed range is /dev/xvd[b-c][a-z]
		// http://docs.aws.amazon.com/AWSEC2/latest/UserGuide/device_naming.html
		deviceAllocator = NewDeviceAllocator()
		c.deviceAllocators[instanceID] = deviceAllocator
	}
	// We need to lock deviceAllocator to prevent possible race with Deprioritize function
	deviceAllocator.Lock()
	defer deviceAllocator.Unlock()

	//chosen, err := deviceAllocator.GetNext(deviceMappings)
	chosen, err := deviceAllocator.GetNext(deviceMappings)
	if err != nil {
		glog.Warningf("Could not assign a mount device.  mappings=%v, error: %v", deviceMappings, err)
		return "", false, fmt.Errorf("Too many EBS volumes attached to node %s.", instanceID)
	}

	attaching := c.attaching[instanceID]
	if attaching == nil {
		attaching = make(map[mountDevice]awsVolumeID)
		c.attaching[instanceID] = attaching
	}
	attaching[chosen] = volumeID
	glog.V(2).Infof("Assigned mount device %s -> volume %s", chosen, volumeID)

	return string(chosen), false, nil
}

func (c *Cloud) AttachDisk(volumeID, nodeID string) (string, error) {
	instances, err := c.DescribeInstances(nodeID)
	if err != nil {
		return "", fmt.Errorf("could not describe instance %q: %v", nodeID, err)
	}

	nInstances := len(instances)
	if nInstances != 1 {
		return "", fmt.Errorf("expected 1 instance with ID %q, got %d", nodeID, len(instances))
	}

	instance := instances[0]

	mntDevice, alreadyAttached, mntErr := c.getMountDevice(nodeID, instance, volumeID, true)
	if mntErr != nil {
		return "", mntErr
	}

	// attachEnded is set to true if the attach operation completed
	// (successfully or not), and is thus no longer in progress
	attachEnded := false
	defer func() {
		if attachEnded {
			if !c.endAttaching(nodeID, awsVolumeID(volumeID), mountDevice(mntDevice)) {
				glog.Errorf("endAttaching called for disk %q when attach not in progress", volumeID)
			}
		}
	}()

	device := "/dev/xvd" + string(mntDevice)
	if !alreadyAttached {
		// See http://docs.aws.amazon.com/AWSEC2/latest/UserGuide/device_naming.html
		request := &ec2.AttachVolumeInput{
			Device:     aws.String(device),
			InstanceId: aws.String(nodeID),
			VolumeId:   aws.String(volumeID),
		}

		resp, err := c.ec2.AttachVolume(request)
		if err != nil {
			attachEnded = true
			return "", fmt.Errorf("could not attach volume %q to node %q: %v", volumeID, nodeID, err)
		}
		glog.V(2).Infof("AttachVolume volume=%q instance=%q request returned %v", volumeID, nodeID, resp)

		if da, ok := c.deviceAllocators[nodeID]; ok {
			da.Deprioritize(mountDevice(mntDevice))
		}
	}

	// TODO: wait attaching
	//attachment, err := disk.waitForAttachmentStatus("attached")
	time.Sleep(time.Second * 7)

	// The attach operation has finished
	attachEnded = true

	//if err != nil {
	//if err == wait.ErrWaitTimeout {
	//c.applyUnSchedulableTaint(nodeName, "Volume stuck in attaching state - node needs reboot to fix impaired state.")
	//}
	//return "", err
	//}

	return device, nil
}

func (c *Cloud) DetachDisk(volumeID, nodeID string) error {
	// TODO: check if attached

	instances, err := c.DescribeInstances(nodeID)
	if err != nil {
		return fmt.Errorf("could not describe instance %q: %v", nodeID, err)
	}

	nInstances := len(instances)
	if nInstances != 1 {
		return fmt.Errorf("expected 1 instance with ID %q, got %d", nodeID, len(instances))
	}

	instance := instances[0]

	mntDevice, _, mntErr := c.getMountDevice(nodeID, instance, volumeID, true)
	if mntErr != nil {
		return mntErr
	}
	request := &ec2.DetachVolumeInput{
		InstanceId: aws.String(nodeID),
		VolumeId:   aws.String(volumeID),
	}

	_, err = c.ec2.DetachVolume(request)
	if err != nil {
		return fmt.Errorf("could not detach volume %q from node %q: %v", volumeID, nodeID, err)
	}

	if da, ok := c.deviceAllocators[nodeID]; ok {
		da.Deprioritize(mountDevice(mntDevice))
	}

	if mntDevice != "" {
		c.endAttaching(nodeID, awsVolumeID(volumeID), mountDevice(mntDevice))
		// We don't check the return value - we don't really expect the attachment to have been
		// in progress, though it might have been
	}

	return nil
}

// endAttaching removes the entry from the "attachments in progress" map
// It returns true if it was found (and removed), false otherwise
func (c *Cloud) endAttaching(nodeID string, volumeID awsVolumeID, mountDevice mountDevice) bool {
	c.attachingMutex.Lock()
	defer c.attachingMutex.Unlock()

	existingVolumeID, found := c.attaching[nodeID][mountDevice]
	if !found {
		return false
	}
	if volumeID != existingVolumeID {
		// This actually can happen, because getMountDevice combines the attaching map with the volumes
		// attached to the instance (as reported by the EC2 API).  So if endAttaching comes after
		// a 10 second poll delay, we might well have had a concurrent request to allocate a mountpoint,
		// which because we allocate sequentially is _very_ likely to get the immediately freed volume
		glog.Infof("endAttaching on device %q assigned to different volume: %q vs %q", mountDevice, volumeID, existingVolumeID)
		return false
	}
	glog.V(2).Infof("Releasing in-process attachment entry: %s -> volume %s", mountDevice, volumeID)
	delete(c.attaching[nodeID], mountDevice)
	return true
}

func (c *Cloud) GetDiskByNameAndSize(name string, capacityBytes int64) (*Disk, error) {
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
		return nil, nil
	}

	volSizeBytes := aws.Int64Value(volumes[0].Size)
	if volSizeBytes != util.BytesToGiB(capacityBytes) {
		return nil, ErrDiskExistsDiffSize
	}

	return &Disk{
		VolumeID:    aws.StringValue(volumes[0].VolumeId),
		CapacityGiB: volSizeBytes,
	}, nil
}
