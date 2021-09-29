/*
Copyright 2019 The Kubernetes Authors.

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

	"github.com/aws/aws-sdk-go/aws/arn"

	"k8s.io/klog"
)

// Metadata is info about the ec2 instance on which the driver is running
type Metadata struct {
	InstanceID       string
	InstanceType     string
	Region           string
	AvailabilityZone string
	OutpostArn       arn.ARN
}

// OutpostArnEndpoint is the ec2 instance metadata endpoint to query to get the outpost arn
const OutpostArnEndpoint string = "outpost-arn"

var _ MetadataService = &Metadata{}

// GetInstanceID returns the instance identification.
func (m *Metadata) GetInstanceID() string {
	return m.InstanceID
}

// GetInstanceType returns the instance type.
func (m *Metadata) GetInstanceType() string {
	return m.InstanceType
}

// GetRegion returns the region which the instance is in.
func (m *Metadata) GetRegion() string {
	return m.Region
}

// GetAvailabilityZone returns the Availability Zone which the instance is in.
func (m *Metadata) GetAvailabilityZone() string {
	return m.AvailabilityZone
}

// GetOutpostArn returns outpost arn if instance is running on an outpost. empty otherwise.
func (m *Metadata) GetOutpostArn() arn.ARN {
	return m.OutpostArn
}

func NewMetadataService(ec2MetadataClient EC2MetadataClient, k8sAPIClient KubernetesAPIClient) (MetadataService, error) {
	klog.Infof("retrieving instance data from ec2 metadata")
	svc, err := ec2MetadataClient()
	if !svc.Available() {
		klog.Warning("ec2 metadata is not available")
	} else if err != nil {
		klog.Warningf("error creating ec2 metadata client: %v", err)
	} else {
		klog.Infof("ec2 metadata is available")
		return EC2MetadataInstanceInfo(svc)
	}

	klog.Infof("retrieving instance data from kubernetes api")
	clientset, err := k8sAPIClient()
	if err != nil {
		klog.Warningf("error creating kubernetes api client: %v", err)
	} else {
		klog.Infof("kubernetes api is available")
		return KubernetesAPIInstanceInfo(clientset)
	}

	return nil, fmt.Errorf("error getting instance data from ec2 metadata or kubernetes api")
}
