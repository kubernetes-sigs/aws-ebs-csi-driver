package cloud

import (
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

type CloudProvider interface {
	CreateDisk(diskOptions *DiskOptions) (volumeID VolumeID, err error)
	DeleteDisk(volumeID VolumeID) (bool, error)
	GetVolumesByTagName(tagKey, tagVal string) ([]string, error)
}

type DiskOptions struct {
	CapacityGB int
	Tags       map[string]string
	VolumeType string
	IOPSPerGB  int
}

type awsEBS struct {
	ec2 *ec2.EC2
}

func NewCloudProvider() (*awsEBS, error) {
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

	// TODO: put this in a config file
	regionName := "us-east-1"
	awsConfig := &aws.Config{
		Region:      &regionName,
		Credentials: creds,
	}
	awsConfig = awsConfig.WithCredentialsChainVerboseErrors(true)

	return &awsEBS{
		ec2: ec2.New(session.New(awsConfig)),
	}, nil
}

func (c *awsEBS) CreateDisk(diskOptions *DiskOptions) (VolumeID, error) {
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

	var tags []*ec2.Tag
	for key, value := range diskOptions.Tags {
		tags = append(tags, &ec2.Tag{Key: &key, Value: &value})
	}

	resourceType := "volume"
	request := &ec2.CreateVolumeInput{
		AvailabilityZone:  aws.String("us-east-1d"), // TODO: read this from config file
		Size:              aws.Int64(int64(diskOptions.CapacityGB)),
		VolumeType:        aws.String(createType),
		TagSpecifications: []*ec2.TagSpecification{{ResourceType: &resourceType, Tags: tags}},
	}
	if iops > 0 {
		request.Iops = aws.Int64(iops)
	}

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

func (c *awsEBS) GetVolumesByTagName(tagKey, tagVal string) ([]string, error) {
	var volumes []string
	var nextToken *string
	request := &ec2.DescribeVolumesInput{}
	for {
		response, err := c.ec2.DescribeVolumes(request)
		if err != nil {
			return nil, err
		}
		for _, volume := range response.Volumes {
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
