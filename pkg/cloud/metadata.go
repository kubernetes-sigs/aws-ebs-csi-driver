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
	"context"
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/arn"
	"github.com/aws/aws-sdk-go/aws/ec2metadata"
	"github.com/aws/aws-sdk-go/aws/session"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
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

type EC2MetadataClient func() (EC2Metadata, error)

var DefaultEC2MetadataClient = func() (EC2Metadata, error) {
	sess := session.Must(session.NewSession(&aws.Config{}))
	svc := ec2metadata.New(sess)
	return svc, nil
}

type KubernetesAPIClient func() (kubernetes.Interface, error)

var DefaultKubernetesAPIClient = func() (kubernetes.Interface, error) {
	// creates the in-cluster config
	config, err := rest.InClusterConfig()
	if err != nil {
		return nil, err
	}
	// creates the clientset
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}
	return clientset, nil
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

func EC2MetadataInstanceInfo(svc EC2Metadata) (*Metadata, error) {
	doc, err := svc.GetInstanceIdentityDocument()
	if err != nil {
		return nil, fmt.Errorf("could not get EC2 instance identity metadata: %v", err)
	}

	if len(doc.InstanceID) == 0 {
		return nil, fmt.Errorf("could not get valid EC2 instance ID")
	}

	if len(doc.InstanceType) == 0 {
		return nil, fmt.Errorf("could not get valid EC2 instance type")
	}

	if len(doc.Region) == 0 {
		return nil, fmt.Errorf("could not get valid EC2 region")
	}

	if len(doc.AvailabilityZone) == 0 {
		return nil, fmt.Errorf("could not get valid EC2 availability zone")
	}

	instanceInfo := Metadata{
		InstanceID:       doc.InstanceID,
		InstanceType:     doc.InstanceType,
		Region:           doc.Region,
		AvailabilityZone: doc.AvailabilityZone,
	}

	outpostArn, err := svc.GetMetadata(OutpostArnEndpoint)
	// "outpust-arn" returns 404 for non-outpost instances. note that the request is made to a link-local address.
	// it's guaranteed to be in the form `arn:<partition>:outposts:<region>:<account>:outpost/<outpost-id>`
	// There's a case to be made here to ignore the error so a failure here wouldn't affect non-outpost calls.
	if err != nil && !strings.Contains(err.Error(), "404") {
		return nil, fmt.Errorf("something went wrong while getting EC2 outpost arn: %s", err.Error())
	} else if err == nil {
		klog.Infof("Running in an outpost environment with arn: %s", outpostArn)
		outpostArn = strings.ReplaceAll(outpostArn, "outpost/", "")
		parsedArn, err := arn.Parse(outpostArn)
		if err != nil {
			klog.Warningf("Failed to parse the outpost arn: %s", outpostArn)
		} else {
			klog.Infof("Using outpost arn: %v", parsedArn)
			instanceInfo.OutpostArn = parsedArn
		}
	}

	return &instanceInfo, nil
}

func KubernetesAPIInstanceInfo(clientset kubernetes.Interface) (*Metadata, error) {
	nodeName := os.Getenv("CSI_NODE_NAME")
	if nodeName == "" {
		return nil, fmt.Errorf("CSI_NODE_NAME env var not set")
	}

	// get node with k8s API
	node, err := clientset.CoreV1().Nodes().Get(context.TODO(), nodeName, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("error getting Node %v: %v", nodeName, err)
	}

	providerID := node.Spec.ProviderID
	if providerID == "" {
		return nil, fmt.Errorf("node providerID empty, cannot parse")
	}

	awsRegionRegex := "([a-z]{2}(-gov)?)-(central|(north|south)?(east|west)?)-[0-9]"
	awsAvailabilityZoneRegex := "([a-z]{2}(-gov)?)-(central|(north|south)?(east|west)?)-[0-9][a-z]"
	awsInstanceIDRegex := "i-[a-z0-9]+$"

	re := regexp.MustCompile(awsRegionRegex)
	region := re.FindString(providerID)
	if region == "" {
		return nil, fmt.Errorf("did not find aws region in node providerID string")
	}

	re = regexp.MustCompile(awsAvailabilityZoneRegex)
	availabilityZone := re.FindString(providerID)
	if availabilityZone == "" {
		return nil, fmt.Errorf("did not find aws availability zone in node providerID string")
	}

	re = regexp.MustCompile(awsInstanceIDRegex)
	instanceID := re.FindString(providerID)
	if instanceID == "" {
		return nil, fmt.Errorf("did not find aws instance ID in node providerID string")
	}

	instanceInfo := Metadata{
		InstanceID:       instanceID,
		InstanceType:     "", // we have no way to find this, so we leave it empty
		Region:           region,
		AvailabilityZone: availabilityZone,
	}

	return &instanceInfo, nil
}
