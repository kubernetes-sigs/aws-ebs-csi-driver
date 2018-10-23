/*
Copyright 2018 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package cloud

import (
	"fmt"

	"github.com/aws/aws-sdk-go/aws/ec2metadata"
)

type EC2Metadata interface {
	Available() bool
	GetInstanceIdentityDocument() (ec2metadata.EC2InstanceIdentityDocument, error)
}

// MetadataService represents AWS metadata service.
type MetadataService interface {
	GetInstanceID() string
	GetRegion() string
	GetAvailabilityZone() string
}

type metadata struct {
	instanceID       string
	region           string
	availabilityZone string
}

var _ MetadataService = &metadata{}

// GetInstanceID returns the instance identification.
func (m *metadata) GetInstanceID() string {
	return m.instanceID
}

// GetRegion returns the region which the instance is in.
func (m *metadata) GetRegion() string {
	return m.region
}

// GetAvailabilityZone returns the Availability Zone which the instance is in.
func (m *metadata) GetAvailabilityZone() string {
	return m.availabilityZone
}

// NewMetadataService returns a new MetadataServiceImplementation.
func NewMetadataService(svc EC2Metadata) (MetadataService, error) {
	if !svc.Available() {
		return nil, fmt.Errorf("EC2 instance metadata is not available")
	}

	doc, err := svc.GetInstanceIdentityDocument()
	if err != nil {
		return nil, fmt.Errorf("could not get EC2 instance identity metadata")
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

	return &metadata{
		instanceID:       doc.InstanceID,
		region:           doc.Region,
		availabilityZone: doc.AvailabilityZone,
	}, nil
}
