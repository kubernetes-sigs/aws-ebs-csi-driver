package metadata

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws/arn"
	"github.com/aws/aws-sdk-go-v2/feature/ec2/imds"
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
	GetDynamicData(ctx context.Context, params *imds.GetDynamicDataInput, optFns ...func(*imds.Options)) (*imds.GetDynamicDataOutput, error)
	GetIAMInfo(ctx context.Context, params *imds.GetIAMInfoInput, optFns ...func(*imds.Options)) (*imds.GetIAMInfoOutput, error)
	GetInstanceIdentityDocument(ctx context.Context, params *imds.GetInstanceIdentityDocumentInput, optFns ...func(*imds.Options)) (*imds.GetInstanceIdentityDocumentOutput, error)
	GetMetadata(ctx context.Context, params *imds.GetMetadataInput, optFns ...func(*imds.Options)) (*imds.GetMetadataOutput, error)
	GetRegion(ctx context.Context, params *imds.GetRegionInput, optFns ...func(*imds.Options)) (*imds.GetRegionOutput, error)
	GetUserData(ctx context.Context, params *imds.GetUserDataInput, optFns ...func(*imds.Options)) (*imds.GetUserDataOutput, error)
}
