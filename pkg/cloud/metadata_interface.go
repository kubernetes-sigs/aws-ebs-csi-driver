package cloud

import (
	"github.com/aws/aws-sdk-go/aws/arn"
	"github.com/aws/aws-sdk-go/aws/ec2metadata"
)

// MetadataService represents AWS metadata service.
type MetadataService interface {
	GetInstanceID() string
	GetInstanceType() string
	GetRegion() string
	GetAvailabilityZone() string
	GetNumAttachedENIs() int
	GetNumBlockDeviceMappings() int
	GetOutpostArn() arn.ARN
}

type EC2Metadata interface {
	Available() bool
	// ec2 instance metadata endpoints: https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/instancedata-data-retrieval.html
	GetMetadata(string) (string, error)
	GetInstanceIdentityDocument() (ec2metadata.EC2InstanceIdentityDocument, error)
}
