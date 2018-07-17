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
	"github.com/golang/glog"
)

const (
	DefaultVolumeSize int64  = 4000000000 //TODO: what should be the default size?
	VolumeNameTagKey  string = "VolumeName"
)

type CloudProvider interface {
	CreateDisk(volumeName string, diskOptions *DiskOptions) (VolumeID, error)
	DeleteDisk(volumeID VolumeID) (bool, error)
	GetVolumesByNameAndSize(tagKey, name string, size int) ([]string, error)
}

type DiskOptions struct {
	CapacityGB int
	Tags       map[string]string
	VolumeType string
	IOPSPerGB  int
}

type awsEBS struct {
	region string
	zone   string
	ec2    *ec2.EC2
}

func NewCloudProvider(region, zone string) (*awsEBS, error) {
	cfg, err := readAWSCloudConfig(nil)
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
		glog.Infof("Using AWS assumed role %v", cfg.Global.RoleARN)
		provider = &stscreds.AssumeRoleProvider{
			Client:  sts.New(sess),
			RoleARN: cfg.Global.RoleARN,
		}
	}

	creds := credentials.NewChainCredentials(
		[]credentials.Provider{
			&credentials.EnvProvider{},
			provider,
			&credentials.SharedCredentialsProvider{},
		})

	awsConfig := &aws.Config{
		Region:      &region, // TODO: point this to value in awsEBS struct
		Credentials: creds,
	}
	awsConfig = awsConfig.WithCredentialsChainVerboseErrors(true)

	return &awsEBS{
		region: region,
		zone:   zone,
		ec2:    ec2.New(session.New(awsConfig)),
	}, nil
}

var ErrWrongDiskSize = errors.New("disk sizes are different")

func (c *awsEBS) CreateDisk(volumeName string, diskOptions *DiskOptions) (VolumeID, error) {
	var createType string
	var iops int64
	switch diskOptions.VolumeType {
	case VolumeTypeGP2, VolumeTypeSC1, VolumeTypeST1:
		createType = diskOptions.VolumeType

	case VolumeTypeIO1:
		createType = diskOptions.VolumeType
		iops = int64(diskOptions.CapacityGB * diskOptions.IOPSPerGB)
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

	// Build tags
	var tags []*ec2.Tag
	for key, value := range diskOptions.Tags {
		tags = append(tags, &ec2.Tag{Key: &key, Value: &value})
	}
	tagSpec := ec2.TagSpecification{
		ResourceType: aws.String("volume"),
		Tags:         tags,
	}

	// Build request
	request := &ec2.CreateVolumeInput{
		AvailabilityZone:  aws.String(c.zone),
		Size:              aws.Int64(int64(diskOptions.CapacityGB)),
		VolumeType:        aws.String(createType),
		TagSpecifications: []*ec2.TagSpecification{&tagSpec},
	}
	if iops > 0 {
		request.Iops = aws.Int64(iops)
	}

	// Create volume
	response, err := c.ec2.CreateVolume(request)
	if err != nil {
		return "", err
	}

	awsID := awsVolumeID(aws.StringValue(response.VolumeId))
	if awsID == "" {
		return "", fmt.Errorf("VolumeID was not returned by CreateVolume")
	}
	volumeID := VolumeID("aws://" + aws.StringValue(response.AvailabilityZone) + "/" + string(awsID))

	return volumeID, nil
}

func (c *awsEBS) DeleteDisk(volumeID VolumeID) (bool, error) {
	awsVolID, err := volumeID.MapToAWSVolumeID()
	if err != nil {
		return false, err
	}

	request := &ec2.DeleteVolumeInput{VolumeId: awsVolID.awsString()}
	_, err = c.ec2.DeleteVolume(request)
	if err != nil {
		return false, err
	}

	return true, nil
}

func (c *awsEBS) GetVolumesByNameAndSize(tagKey, tagVal string, size int) ([]string, error) {
	var volumes []string
	var nextToken *string
	request := &ec2.DescribeVolumesInput{}
	for {
		response, err := c.ec2.DescribeVolumes(request)
		if err != nil {
			return nil, err
		}
		for _, volume := range response.Volumes {
			if *volume.Size == int64(size) {
				continue
			}
			for _, tag := range volume.Tags {
				if *tag.Key == tagKey && *tag.Value == tagVal {
					volumes = append(volumes, *volume.VolumeId)
					break
				}
			}
		}
		nextToken = response.NextToken
		if aws.StringValue(nextToken) == "" {
			break
		}
		request.NextToken = nextToken
	}
	return volumes, nil
}
