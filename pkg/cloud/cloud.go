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
)

type CloudProvider interface {
	CreateDisk(volumeName string, diskOptions *DiskOptions) (string, error)
	DeleteDisk(volumeID string) (bool, error)
	GetVolumeByNameAndSize(name string, size int64) (string, error)
}

type DiskOptions struct {
	CapacityGB int64
	Tags       map[string]string
	VolumeType string
	IOPSPerGB  int64
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

func (c *awsEBS) CreateDisk(volumeName string, diskOptions *DiskOptions) (string, error) {
	var createType string
	var iops int64

	switch diskOptions.VolumeType {
	case VolumeTypeGP2, VolumeTypeSC1, VolumeTypeST1:
		createType = diskOptions.VolumeType
	case VolumeTypeIO1:
		createType = diskOptions.VolumeType
		iops = diskOptions.CapacityGB * diskOptions.IOPSPerGB
		if iops < MinTotalIOPS {
			iops = MinTotalIOPS
		}
		if iops > MaxTotalIOPS {
			iops = MaxTotalIOPS
		}
	case "":
		createType = DefaultVolumeType
	default:
		return "", fmt.Errorf("invalid AWS VolumeType %q", diskOptions.VolumeType)
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
		Size:              aws.Int64(int64(diskOptions.CapacityGB)),
		VolumeType:        aws.String(createType),
		TagSpecifications: []*ec2.TagSpecification{&tagSpec},
	}
	if iops > 0 {
		request.Iops = aws.Int64(iops)
	}

	response, err := c.ec2.CreateVolume(request)
	if err != nil {
		return "", err
	}

	volumeID := aws.StringValue(response.VolumeId)
	if len(volumeID) == 0 {
		return "", fmt.Errorf("VolumeID was not returned by CreateVolume")
	}

	return volumeID, nil
}

func (c *awsEBS) DeleteDisk(volumeID string) (bool, error) {
	request := &ec2.DeleteVolumeInput{VolumeId: &volumeID}
	if _, err := c.ec2.DeleteVolume(request); err != nil {
		return false, fmt.Errorf("DeleteDisk could not delete volume")
	}
	return true, nil
}

var ErrMultiDisks = errors.New("Multiple disks with same name")
var ErrDiskExistsDiffSize = errors.New("There is already a disk with same name and different size")

func (c *awsEBS) GetVolumeByNameAndSize(name string, size int64) (string, error) {
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
			return "", err
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

	nVol := len(volumes)
	if nVol > 1 {
		return "", ErrMultiDisks
	} else if nVol == 0 {
		return "", nil
	}

	vol := volumes[0]
	if aws.Int64Value(vol.Size) != int64(size) {
		return "", ErrDiskExistsDiffSize
	}
	return aws.StringValue(vol.VolumeId), nil
}
