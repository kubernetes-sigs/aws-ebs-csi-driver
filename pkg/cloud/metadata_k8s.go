package cloud

import (
	"context"
	"fmt"
	"os"
	"regexp"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

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

	var instanceType string
	if it, ok := node.GetLabels()[corev1.LabelInstanceTypeStable]; ok {
		instanceType = it
	}

	instanceInfo := Metadata{
		InstanceID:             instanceID,
		InstanceType:           instanceType,
		Region:                 region,
		AvailabilityZone:       availabilityZone,
		NumAttachedENIs:        1, // All nodes have at least 1 attached ENI, so we'll use that
		NumBlockDeviceMappings: 0,
	}

	return &instanceInfo, nil
}
