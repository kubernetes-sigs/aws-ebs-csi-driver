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

package metadata

import (
	"fmt"
	"os"

	"github.com/aws/aws-sdk-go-v2/aws/arn"
	"github.com/kubernetes-sigs/aws-ebs-csi-driver/pkg/util"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
)

// Metadata is info about the ec2 instance on which the driver is running.
type Metadata struct {
	InstanceID             string
	InstanceType           string
	Region                 string
	AvailabilityZone       string
	NumAttachedENIs        int
	NumBlockDeviceMappings int
	OutpostArn             arn.ARN
	IMDSClient             IMDS
	K8sAPIClient           kubernetes.Interface
}

type MetadataServiceConfig struct {
	MetadataSources []string
	IMDSClient      IMDSClient
	K8sAPIClient    KubernetesAPIClient
}

const (
	SourceIMDS            = "imds"
	SourceMetadataLabeler = "metadata-labeler"
	SourceK8s             = "kubernetes"
)

var (
	// DefaultMetadataSources lists the default fallback order of driver Metadata sources.
	DefaultMetadataSources = []string{SourceIMDS, SourceK8s}
)

var _ MetadataService = &Metadata{}

// NewMetadataService retrieves instance Metadata from one of the clients in MetadataServiceConfig.
// It tries each client included in MetadataServiceConfig.MetadataSources in order until one succeeds.
func NewMetadataService(cfg MetadataServiceConfig, region string) (MetadataService, error) {
	for _, source := range cfg.MetadataSources {
		switch source {
		case SourceIMDS:
			isHyperPodNode := util.IsHyperPodNode(os.Getenv("CSI_NODE_NAME"))
			if os.Getenv("AWS_EC2_METADATA_DISABLED") == "true" || isHyperPodNode {
				if isHyperPodNode {
					klog.V(2).InfoS("HyperPod node detected. Will not rely on IMDS for instance metadata")
				} else {
					klog.V(2).InfoS("Environment variable AWS_EC2_METADATA_DISABLED set to 'true'. Will not rely on IMDS for instance metadata")
				}
			} else {
				klog.V(2).InfoS("Attempting to retrieve instance metadata from IMDS")
				metadata, err := retrieveIMDSMetadata(cfg.IMDSClient)
				if err == nil {
					klog.V(2).InfoS("Retrieved metadata from IMDS")
					return metadata.overrideRegion(region), nil
				}
				klog.ErrorS(err, "Retrieving IMDS metadata failed")
			}
		case SourceMetadataLabeler:
			klog.V(2).InfoS("Attempting to retrieve instance metadata from metadata labeler")
			metadata, err := retrieveK8sMetadata(cfg.K8sAPIClient, true)
			if err == nil {
				klog.V(2).InfoS("Retrieved metadata from metadata labeler")
				return metadata.overrideRegion(region), nil
			}
			klog.ErrorS(err, "Retrieving metadata labeler failed")
		case SourceK8s:
			klog.V(2).InfoS("Attempting to retrieve instance metadata from Kubernetes API")
			metadata, err := retrieveK8sMetadata(cfg.K8sAPIClient, false)
			if err == nil {
				klog.V(2).InfoS("Retrieved metadata from Kubernetes")
				return metadata.overrideRegion(region), nil
			}
			klog.ErrorS(err, "Retrieving Kubernetes metadata failed")
		default:
			// Unexpected cases should have been caught during driver option validation
			return nil, InvalidSourceErr(cfg.MetadataSources, source)
		}
	}

	return nil, sourcesUnavailableErr(cfg.MetadataSources)
}

// UpdateMetadata refreshes metadata cache based upon driver startup metadata source.
func (m *Metadata) UpdateMetadata() error {
	switch {
	case m.IMDSClient != nil:
		// We do not refresh blockDeviceMappings because IMDS only reports data from instance start (As of April 2025)
		attachedENIs, err := getAttachedENIs(m.IMDSClient)
		if err != nil {
			return fmt.Errorf("failed to update ENI count via IMDS metadata source: %w", err)
		}
		m.NumAttachedENIs = attachedENIs
	case m.K8sAPIClient != nil:
		updatedMetadata, err := KubernetesAPIInstanceInfo(m.K8sAPIClient, true /* metadataLabeler */)
		if updatedMetadata == nil || err != nil {
			return fmt.Errorf("failed to update ENI and Block Device count via metadataLabeler source: %w", err)
		}
		m.NumAttachedENIs = updatedMetadata.NumAttachedENIs
		m.NumBlockDeviceMappings = updatedMetadata.NumBlockDeviceMappings
	}

	return nil
}

func retrieveIMDSMetadata(imdsClient IMDSClient) (*Metadata, error) {
	svc, err := imdsClient()
	if err != nil {
		klog.ErrorS(err, "failed to initialize IMDS client")
		return nil, err
	}
	return IMDSInstanceInfo(svc)
}

func retrieveK8sMetadata(k8sAPIClient KubernetesAPIClient, metadataLabeler bool) (*Metadata, error) {
	clientset, err := k8sAPIClient()
	if err != nil {
		return nil, err
	}

	metadata, err := KubernetesAPIInstanceInfo(clientset, metadataLabeler)
	if err != nil {
		return nil, err
	}

	return metadata, nil
}

// Override the region on a Metadata object if it is non-empty.
func (m *Metadata) overrideRegion(region string) *Metadata {
	if region != "" {
		m.Region = region
	}
	return m
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

// InvalidSourceErr returns an error message when a metadata source is invalid.
func InvalidSourceErr(sources []string, invalidSource string) error {
	return fmt.Errorf("invalid source: argument --metadata-sources=%s included invalid option '%s', comma-separated string MUST only include tokens like '%s' or '%s'", sources, invalidSource, SourceIMDS, SourceK8s)
}

func sourcesUnavailableErr(metadataSources []string) error {
	return fmt.Errorf("all specified --metadata-sources '%s' are unavailable", metadataSources)
}
