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
	"os"

	"github.com/aws/aws-sdk-go-v2/aws/arn"
	"k8s.io/klog/v2"
)

// Metadata is info about the ec2 instance on which the driver is running
type Metadata struct {
	InstanceID             string
	InstanceType           string
	Region                 string
	AvailabilityZone       string
	NumAttachedENIs        int
	NumBlockDeviceMappings int
	OutpostArn             arn.ARN
}

type MetadataServiceConfig struct {
	EC2MetadataClient EC2MetadataClient
	K8sAPIClient      KubernetesAPIClient
}

var _ MetadataService = &Metadata{}

func NewMetadataService(cfg MetadataServiceConfig, region string) (MetadataService, error) {
	metadata, err := retrieveEC2Metadata(cfg.EC2MetadataClient, region)
	if err == nil {
		klog.InfoS("ec2 metadata is available")
		return metadata, nil
	}

	klog.InfoS("failed to retrieve instance data from ec2 metadata; retrieving instance data from kubernetes api", "err", err)
	metadata, err = retrieveK8sMetadata(cfg.K8sAPIClient)
	if err == nil {
		klog.InfoS("kubernetes api is available")
		return metadata, nil
	}

	return nil, fmt.Errorf("error getting instance data from ec2 metadata or kubernetes api")
}

func retrieveEC2Metadata(ec2MetadataClient EC2MetadataClient, region string) (*Metadata, error) {
	envValue := os.Getenv("AWS_EC2_METADATA_DISABLED")
	if envValue != "" {
		klog.InfoS("The AWS_EC2_METADATA_DISABLED environment variable disables access to EC2 IMDS", "enabled", envValue)
	}

	svc, err := ec2MetadataClient()
	if err != nil {
		klog.ErrorS(err, "Failed to initialize EC2 Metadata client")
		return nil, err
	}
	return EC2MetadataInstanceInfo(svc, region)
}

func retrieveK8sMetadata(k8sAPIClient KubernetesAPIClient) (*Metadata, error) {
	clientset, err := k8sAPIClient()
	if err != nil {
		klog.InfoS("error creating kubernetes api client", "err", err)
		return nil, err
	}

	return KubernetesAPIInstanceInfo(clientset)
}

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

// GetNumAttachedENIs returns the number of attached ENIs.
func (m *Metadata) GetNumAttachedENIs() int {
	return m.NumAttachedENIs
}

// GetNumBlockDeviceMappings returns the number of block device mappings.
func (m *Metadata) GetNumBlockDeviceMappings() int {
	return m.NumBlockDeviceMappings
}

// GetOutpostArn returns outpost arn if instance is running on an outpost. empty otherwise.
func (m *Metadata) GetOutpostArn() arn.ARN {
	return m.OutpostArn
}
