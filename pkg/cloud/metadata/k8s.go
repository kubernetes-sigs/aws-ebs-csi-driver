// Copyright 2024 The Kubernetes Authors.
//
// Licensed under the Apache License, Version 2.0 (the 'License');
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//    http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an 'AS IS' BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package metadata

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/cert"
	"k8s.io/klog/v2"
)

const (
	LabelSageMakerENICount                 = "sagemaker.amazonaws.com/num-eni-attachments"
	LabelSageMakerBlockDeviceMappingsCount = "sagemaker.amazonaws.com/num-block-device-mappings"
)

type KubernetesAPIClient func() (kubernetes.Interface, error)

func DefaultKubernetesAPIClient(kubeconfig string) KubernetesAPIClient {
	return func() (clientset kubernetes.Interface, cfg *rest.Config, err error) {
		var config *rest.Config // needed for leader election
		if kubeconfig != "" {
			config, err = clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
				&clientcmd.ClientConfigLoadingRules{ExplicitPath: kubeconfig},
				&clientcmd.ConfigOverrides{},
			).ClientConfig()
			if err != nil {
				return nil, nil, err
			}
		} else {
			// creates the in-cluster config
			config, err = rest.InClusterConfig()
			if err != nil {
				if errors.Is(err, os.ErrNotExist) {
					klog.InfoS("InClusterConfig failed to read token file, retrieving file from sandbox mount point")
					// CONTAINER_SANDBOX_MOUNT_POINT env is set upon container creation in containerd v1.6+
					// it provides the absolute host path to the container volume.
					sandboxMountPoint := os.Getenv("CONTAINER_SANDBOX_MOUNT_POINT")
					if sandboxMountPoint == "" {
						return nil, nil, errors.New("CONTAINER_SANDBOX_MOUNT_POINT environment variable is not set")
					}

					tokenFile := filepath.Join(sandboxMountPoint, "var", "run", "secrets", "kubernetes.io", "serviceaccount", "token")
					rootCAFile := filepath.Join(sandboxMountPoint, "var", "run", "secrets", "kubernetes.io", "serviceaccount", "ca.crt")

					token, tokenErr := os.ReadFile(tokenFile)
					if tokenErr != nil {
						return nil, nil, tokenErr
					}

					tlsClientConfig := rest.TLSClientConfig{}
					if _, certErr := cert.NewPool(rootCAFile); certErr != nil {
						return nil, nil, fmt.Errorf("expected to load root CA config from %s, but got err: %w", rootCAFile, certErr)
					} else {
						tlsClientConfig.CAFile = rootCAFile
					}

					config = &rest.Config{
						Host:            "https://" + net.JoinHostPort(os.Getenv("KUBERNETES_SERVICE_HOST"), os.Getenv("KUBERNETES_SERVICE_PORT")),
						TLSClientConfig: tlsClientConfig,
						BearerToken:     string(token),
						BearerTokenFile: tokenFile,
					}
				} else {
					return nil, nil, err
				}
			}
		}
		config.AcceptContentTypes = "application/vnd.kubernetes.protobuf,application/json"
		config.ContentType = "application/vnd.kubernetes.protobuf"
		// creates the clientset
		clientset, err = kubernetes.NewForConfig(config)
		if err != nil {
			return nil, nil, err
		}
		return clientset, config, nil
	}
}

func KubernetesAPIInstanceInfo(clientset kubernetes.Interface, ec2Labels bool) (*Metadata, error) {
	nodeName := os.Getenv("CSI_NODE_NAME")
	if nodeName == "" {
		return nil, errors.New("CSI_NODE_NAME env var not set")
	}

	// get node with k8s API
	node, err := clientset.CoreV1().Nodes().Get(context.TODO(), nodeName, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("error getting Node %v: %w", nodeName, err)
	}

	enis := 1    // All nodes have at least 1 attached ENI, so we'll use that
	volumes := 0 // Root volume already accounted for elsewhere (in getVolumesLimit())

	if ec2Labels {
		backoff := wait.Backoff{
			Duration: 1 * time.Second,
			Factor:   1.5,
			Steps:    6,
		}

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		backoffErr := wait.ExponentialBackoffWithContext(ctx, backoff, func(ctx context.Context) (bool, error) {
			if enis, volumes, err = getEC2ENIsVolumes(node); err != nil {
				klog.ErrorS(err, "Get ENI and volume labels failed, retrying...")
				node, err = clientset.CoreV1().Nodes().Get(ctx, nodeName, metav1.GetOptions{})
				//nolint: nilerr // Want to catch retry all errs until context times out
				if err != nil {
					return false, nil
				}
				return false, nil
			}
			return true, nil
		})

		if backoffErr != nil {
			klog.ErrorS(backoffErr, "Get ENI and volume labels failed after multiple retries")
			return nil, backoffErr
		}
	}

	providerID := node.Spec.ProviderID
	if providerID == "" {
		return nil, errors.New("node providerID empty, cannot parse")
	}

	awsInstanceIDRegex := "s\\.i-[a-z0-9]+|(hyperpod-[a-z0-9]+-)?i-[a-z0-9]+$"

	re := regexp.MustCompile(awsInstanceIDRegex)
	instanceID := re.FindString(providerID)
	if instanceID == "" {
		return nil, errors.New("did not find aws instance ID in node providerID string")
	}

	var instanceType string
	if val, ok := node.GetLabels()[corev1.LabelInstanceTypeStable]; ok {
		if strings.HasPrefix(val, "ml.") {
			// sagemaker instance type has 'ml.' prefix, remove 'ml.' prefix
			instanceType = strings.TrimPrefix(val, "ml.")
		} else {
			instanceType = val
		}
	} else {
		return nil, errors.New("could not retrieve instance type from topology label")
	}

	var region string
	if val, ok := node.GetLabels()[corev1.LabelTopologyRegion]; ok {
		region = val
	} else {
		return nil, errors.New("could not retrieve region from topology label")
	}

	var availabilityZone string
	if val, ok := node.GetLabels()[corev1.LabelTopologyZone]; ok {
		availabilityZone = val
	} else {
		return nil, errors.New("could not retrieve AZ from topology label")
	}

	numAttachedENIs := 1        // Default: All nodes have at least 1 attached ENI
	numBlockDeviceMappings := 0 // Default: 0

	if val, ok := node.GetLabels()[LabelSageMakerENICount]; ok {
		if num, err := strconv.Atoi(val); err == nil {
			numAttachedENIs = num
			klog.V(2).InfoS("Using ENI count from SageMaker label", "count", numAttachedENIs)
		} else {
			klog.ErrorS(err, "Invalid ENI count in SageMaker label, using default",
				"value", val,
				"default", numAttachedENIs)
		}
	}

	if val, ok := node.GetLabels()[LabelSageMakerBlockDeviceMappingsCount]; ok {
		if num, err := strconv.Atoi(val); err == nil {
			numBlockDeviceMappings = num
			klog.V(2).InfoS("Using block device mapping count from SageMaker label", "count", numBlockDeviceMappings)
		} else {
			klog.ErrorS(err, "Invalid block device mapping count in SageMaker label, using default",
				"value", val,
				"default", numBlockDeviceMappings)
		}
	}

	instanceInfo := Metadata{
		InstanceID:             instanceID,
		InstanceType:           instanceType,
		Region:                 region,
		AvailabilityZone:       availabilityZone,
		NumAttachedENIs:        enis,
		NumBlockDeviceMappings: volumes,
	}
	return &instanceInfo, nil
}

func getEC2ENIsVolumes(node *corev1.Node) (int, int, error) {
	eni, err := getLabelAsInt(node, ENIsLabel, 1)
	if err != nil {
		return 0, 0, err
	}
	vol, err := getLabelAsInt(node, VolumesLabel, 0)
	if err != nil {
		return 0, 0, err
	}
	return eni, vol, nil
}

func getLabelAsInt(node *corev1.Node, label string, defaultValue int) (int, error) {
	val, ok := node.GetLabels()[label]
	if !ok {
		klog.V(2).InfoS("label not found on node", "node", node.Name, "label", label)
		return defaultValue, fmt.Errorf("%s label not found on node", label)
	}

	result, err := strconv.Atoi(val)
	if err != nil {
		klog.ErrorS(err, "failed to convert label to int", "label", label)
		return defaultValue, err
	}
	return result, nil
}
