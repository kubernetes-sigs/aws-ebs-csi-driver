package cloud

import "github.com/aws/aws-sdk-go/aws/arn"

// MetadataService represents AWS metadata service.
type MetadataService interface {
	GetInstanceID() string
	GetInstanceType() string
	GetRegion() string
	GetAvailabilityZone() string
	GetOutpostArn() arn.ARN
}
