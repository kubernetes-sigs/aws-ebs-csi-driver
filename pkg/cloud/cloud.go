package cloud

import (
	"errors"
	"fmt"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/credentials/ec2rolecreds"
	"github.com/aws/aws-sdk-go/aws/credentials/stscreds"
	"github.com/aws/aws-sdk-go/aws/ec2metadata"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/sts"
	"github.com/bertinatto/ebs-csi-driver/pkg/util"
)

type CloudProvider interface {
	CreateDisk(string, *DiskOptions) (*Disk, error)
	DeleteDisk(string) (bool, error)
	AttachDisk(string, string) error
	DetachDisk(string, string) error
	GetDiskByNameAndSize(string, int64) (*Disk, error)
}

type DiskOptions struct {
	CapacityBytes int64
	Tags          map[string]string
	VolumeType    string
	IOPSPerGB     int64
}

type Disk struct {
	VolumeID    string
	CapacityGiB int64
}

type awsEBS struct {
	region string
	zone   string
	ec2    *ec2.EC2
}

func NewCloudProvider(region, zone string) (CloudProvider, error) {
	cfg, err := readAWSConfig(nil)
	if err != nil {
		return nil, fmt.Errorf("unable to read AWS config file: %v", err)
	}

	sess, err := session.NewSession(&aws.Config{})
	if err != nil {
		return nil, fmt.Errorf("unable to initialize AWS session: %v", err)
	}

	var provider credentials.Provider
	if cfg.Global.RoleARN == "" {
		provider = &ec2rolecreds.EC2RoleProvider{
			Client: ec2metadata.New(sess),
		}
	} else {
		provider = &stscreds.AssumeRoleProvider{
			Client:  sts.New(sess),
			RoleARN: cfg.Global.RoleARN,
		}
	}

	awsConfig := &aws.Config{
		Region: &region,
		Credentials: credentials.NewChainCredentials(
			[]credentials.Provider{
				&credentials.EnvProvider{},
				provider,
				&credentials.SharedCredentialsProvider{},
			},
		),
	}
	awsConfig = awsConfig.WithCredentialsChainVerboseErrors(true)

	return &awsEBS{
		region: region,
		zone:   zone,
		ec2:    ec2.New(session.New(awsConfig)),
	}, nil
}

func (c *awsEBS) CreateDisk(volumeName string, diskOptions *DiskOptions) (*Disk, error) {
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

	request := &ec2.CreateVolumeInput{
		AvailabilityZone:  aws.String(c.zone),
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

func (c *awsEBS) DeleteDisk(volumeID string) (bool, error) {
	request := &ec2.DeleteVolumeInput{VolumeId: &volumeID}
	if _, err := c.ec2.DeleteVolume(request); err != nil {
		return false, fmt.Errorf("DeleteDisk could not delete volume")
	}
	return true, nil
}

func (c *awsEBS) AttachDisk(volumeID, nodeID string) error {
	// TODO: choose a valid and non-duplicate device name
	device := "/dev/xvdbc"
	request := &ec2.AttachVolumeInput{
		Device:     aws.String(device),
		InstanceId: aws.String(nodeID),
		VolumeId:   aws.String(volumeID),
	}

	_, err := c.ec2.AttachVolume(request)
	if err != nil {
		return fmt.Errorf("could not attach volume %q to node %q: %v", volumeID, nodeID, err)
	}

	return nil
}

func (c *awsEBS) DetachDisk(volumeID, nodeID string) error {
	request := &ec2.DetachVolumeInput{
		InstanceId: aws.String(nodeID),
		VolumeId:   aws.String(volumeID),
	}

	_, err := c.ec2.DetachVolume(request)
	if err != nil {
		return fmt.Errorf("could not detach volume %q from node %q: %v", volumeID, nodeID, err)
	}

	return nil
}

var ErrMultiDisks = errors.New("Multiple disks with same name")
var ErrDiskExistsDiffSize = errors.New("There is already a disk with same name and different size")

func (c *awsEBS) GetDiskByNameAndSize(name string, capacityBytes int64) (*Disk, error) {
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
