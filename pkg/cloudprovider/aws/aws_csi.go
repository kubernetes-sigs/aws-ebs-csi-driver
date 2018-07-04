package aws

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

func NewCloudProvider() (*Cloud, error) {
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

	aws := newAWSSDKProvider(creds)
	return newAWSCloud(*cfg, aws)

}

func (c *Cloud) GetVolumesByTagName(tagKey, tagVal string) ([]string, error) {
	volumes, err := c.ec2.DescribeVolumes(&ec2.DescribeVolumesInput{})
	if err != nil {
		return nil, fmt.Errorf("uname to get volumes %v", err)
	}

	// TODO: fjb: can I trust all pointers here won't be nil?
	var result []string
	for _, volume := range volumes {
		for _, tag := range volume.Tags {
			if *tag.Key == tagKey && *tag.Value == tagVal {
				result = append(result, *volume.VolumeId)
			}
		}
	}
	return result, nil
}
