package cloud

import (
	"fmt"

	"github.com/aws/aws-sdk-go/aws/ec2metadata"
)

type Metadata struct {
	instanceID       string
	region           string
	availabilityZone string
}

func (m *Metadata) GetInstanceID() string {
	return m.instanceID
}

func (m *Metadata) GetRegion() string {
	return m.region
}

func (m *Metadata) GetAvailabilityZone() string {
	return m.availabilityZone
}

func NewMetadata(svc *ec2metadata.EC2Metadata) (*Metadata, error) {
	if !svc.Available() {
		return nil, fmt.Errorf("EC2 instance metadata is not available")
	}

	doc, err := svc.GetInstanceIdentityDocument()
	if err != nil {
		return nil, fmt.Errorf("could not EC2 instance identity metadata")
	}

	if len(doc.InstanceID) == 0 {
		return nil, fmt.Errorf("could not get valid EC2 instance ID")
	}

	if len(doc.Region) == 0 {
		return nil, fmt.Errorf("could not get valid EC2 region")
	}

	if len(doc.AvailabilityZone) == 0 {
		return nil, fmt.Errorf("could not get valid EC2 availavility zone")
	}

	return &Metadata{
		instanceID:       doc.InstanceID,
		region:           doc.Region,
		availabilityZone: doc.AvailabilityZone,
	}, nil
}
