/*
Copyright 2024 The Kubernetes Authors.

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
	"errors"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws/arn"
	"github.com/aws/aws-sdk-go-v2/feature/ec2/imds"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
)

func TestNewMetadataService(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	testCases := []struct {
		name             string
		region           string
		ec2MetadataError error
		k8sAPIError      error
		expectedMetadata *Metadata
		expectedError    error
	}{
		{
			name:   "TestNewMetadataService: EC2 metadata available",
			region: "us-west-2",
			expectedMetadata: &Metadata{
				InstanceID:             "i-1234567890abcdef0",
				InstanceType:           "c5.xlarge",
				Region:                 "us-west-2",
				AvailabilityZone:       "us-west-2a",
				NumAttachedENIs:        1,
				NumBlockDeviceMappings: 2,
			},
		},
		{
			name:             "TestNewMetadataService: EC2 metadata error, K8s API available",
			region:           "us-west-2",
			ec2MetadataError: errors.New("EC2 metadata error"),
			expectedMetadata: &Metadata{
				InstanceID:             "i-1234567890abcdef0",
				InstanceType:           "c5.xlarge",
				Region:                 "us-west-2",
				AvailabilityZone:       "us-west-2a",
				NumAttachedENIs:        1,
				NumBlockDeviceMappings: 0,
			},
		},
		{
			name:             "TestNewMetadataService: EC2 metadata error, K8s API error",
			region:           "us-west-2",
			ec2MetadataError: errors.New("EC2 metadata error"),
			k8sAPIError:      errors.New("K8s API error"),
			expectedError:    errors.New("error getting instance data from ec2 metadata or kubernetes api"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockEC2Metadata := NewMockEC2Metadata(ctrl)
			mockK8sClient := func() (kubernetes.Interface, error) {
				if tc.k8sAPIError != nil {
					return nil, tc.k8sAPIError
				}
				node := &corev1.Node{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-node",
						Labels: map[string]string{
							corev1.LabelInstanceTypeStable: "c5.xlarge",
							corev1.LabelTopologyRegion:     "us-west-2",
							corev1.LabelTopologyZone:       "us-west-2a",
						},
					},
					Spec: corev1.NodeSpec{
						ProviderID: "aws:///us-west-2a/i-1234567890abcdef0",
					},
				}
				return fake.NewSimpleClientset(node), nil
			}

			os.Setenv("CSI_NODE_NAME", "test-node")

			if tc.ec2MetadataError == nil {
				mockEC2Metadata.EXPECT().GetInstanceIdentityDocument(gomock.Any(), &imds.GetInstanceIdentityDocumentInput{}).Return(&imds.GetInstanceIdentityDocumentOutput{
					InstanceIdentityDocument: imds.InstanceIdentityDocument{
						InstanceID:       "i-1234567890abcdef0",
						InstanceType:     "c5.xlarge",
						Region:           "us-west-2",
						AvailabilityZone: "us-west-2a",
					},
				}, nil)
				mockEC2Metadata.EXPECT().GetMetadata(gomock.Any(), &imds.GetMetadataInput{Path: EnisEndpoint}).Return(&imds.GetMetadataOutput{
					Content: io.NopCloser(strings.NewReader("01:23:45:67:89:ab\n")),
				}, nil)
				mockEC2Metadata.EXPECT().GetMetadata(gomock.Any(), &imds.GetMetadataInput{Path: BlockDevicesEndpoint}).Return(&imds.GetMetadataOutput{
					Content: io.NopCloser(strings.NewReader("ebs\nebs\n")),
				}, nil)
				mockEC2Metadata.EXPECT().GetMetadata(gomock.Any(), &imds.GetMetadataInput{Path: OutpostArnEndpoint}).Return(nil, errors.New("404 - Not Found"))
			}

			cfg := MetadataServiceConfig{
				EC2MetadataClient: func() (EC2Metadata, error) {
					if tc.ec2MetadataError != nil {
						return nil, tc.ec2MetadataError
					}
					return mockEC2Metadata, nil
				},
				K8sAPIClient: mockK8sClient,
			}

			metadata, err := NewMetadataService(cfg, tc.region)

			if tc.expectedError != nil {
				require.EqualError(t, err, tc.expectedError.Error())
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.expectedMetadata, metadata)
			}
		})
	}
}

func TestEC2MetadataInstanceInfo(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	testCases := []struct {
		name              string
		regionFromSession string
		mockEC2Metadata   func(m *MockEC2Metadata)
		expectedMetadata  *Metadata
		expectedError     error
	}{
		{
			name: "TestEC2MetadataInstanceInfo: Error getting instance identity document",
			mockEC2Metadata: func(m *MockEC2Metadata) {
				m.EXPECT().GetInstanceIdentityDocument(gomock.Any(), &imds.GetInstanceIdentityDocumentInput{}).Return(nil, errors.New("failed to get instance identity document"))
			},
			expectedError: errors.New("could not get EC2 instance identity metadata: failed to get instance identity document"),
		},
		{
			name: "TestEC2MetadataInstanceInfo: Empty instance ID",
			mockEC2Metadata: func(m *MockEC2Metadata) {
				m.EXPECT().GetInstanceIdentityDocument(gomock.Any(), &imds.GetInstanceIdentityDocumentInput{}).Return(&imds.GetInstanceIdentityDocumentOutput{
					InstanceIdentityDocument: imds.InstanceIdentityDocument{
						InstanceID: "",
					},
				}, nil)
			},
			expectedError: errors.New("could not get valid EC2 instance ID"),
		},
		{
			name: "TestEC2MetadataInstanceInfo: Empty instance type",
			mockEC2Metadata: func(m *MockEC2Metadata) {
				m.EXPECT().GetInstanceIdentityDocument(gomock.Any(), &imds.GetInstanceIdentityDocumentInput{}).Return(&imds.GetInstanceIdentityDocumentOutput{
					InstanceIdentityDocument: imds.InstanceIdentityDocument{
						InstanceID:   "i-1234567890abcdef0",
						InstanceType: "",
					},
				}, nil)
			},
			expectedError: errors.New("could not get valid EC2 instance type"),
		},
		{
			name: "TestEC2MetadataInstanceInfo: Empty region and invalid region from session",
			mockEC2Metadata: func(m *MockEC2Metadata) {
				m.EXPECT().GetInstanceIdentityDocument(gomock.Any(), &imds.GetInstanceIdentityDocumentInput{}).Return(&imds.GetInstanceIdentityDocumentOutput{
					InstanceIdentityDocument: imds.InstanceIdentityDocument{
						InstanceID:   "i-1234567890abcdef0",
						InstanceType: "c5.xlarge",
						Region:       "",
					},
				}, nil)
			},
			expectedError: errors.New("could not get valid EC2 region"),
		},
		{
			name: "TestEC2MetadataInstanceInfo: Empty availability zone and invalid region from session",
			mockEC2Metadata: func(m *MockEC2Metadata) {
				m.EXPECT().GetInstanceIdentityDocument(gomock.Any(), &imds.GetInstanceIdentityDocumentInput{}).Return(&imds.GetInstanceIdentityDocumentOutput{
					InstanceIdentityDocument: imds.InstanceIdentityDocument{
						InstanceID:       "i-1234567890abcdef0",
						InstanceType:     "c5.xlarge",
						Region:           "us-west-2",
						AvailabilityZone: "",
					},
				}, nil)
			},
			expectedError: errors.New("could not get valid EC2 availability zone"),
		},
		{
			name: "TestEC2MetadataInstanceInfo: Error getting ENIs metadata",
			mockEC2Metadata: func(m *MockEC2Metadata) {
				m.EXPECT().GetInstanceIdentityDocument(gomock.Any(), &imds.GetInstanceIdentityDocumentInput{}).Return(&imds.GetInstanceIdentityDocumentOutput{
					InstanceIdentityDocument: imds.InstanceIdentityDocument{
						InstanceID:       "i-1234567890abcdef0",
						InstanceType:     "c5.xlarge",
						Region:           "us-west-2",
						AvailabilityZone: "us-west-2a",
					},
				}, nil)
				m.EXPECT().GetMetadata(gomock.Any(), &imds.GetMetadataInput{Path: EnisEndpoint}).Return(nil, errors.New("failed to get ENIs metadata"))
			},
			expectedError: errors.New("could not get metadata for ENIs: failed to get ENIs metadata"),
		},
		{
			name: "TestEC2MetadataInstanceInfo: Error reading ENIs metadata content",
			mockEC2Metadata: func(m *MockEC2Metadata) {
				m.EXPECT().GetInstanceIdentityDocument(gomock.Any(), &imds.GetInstanceIdentityDocumentInput{}).Return(&imds.GetInstanceIdentityDocumentOutput{
					InstanceIdentityDocument: imds.InstanceIdentityDocument{
						InstanceID:       "i-1234567890abcdef0",
						InstanceType:     "c5.xlarge",
						Region:           "us-west-2",
						AvailabilityZone: "us-west-2a",
					},
				}, nil)
				m.EXPECT().GetMetadata(gomock.Any(), &imds.GetMetadataInput{Path: EnisEndpoint}).Return(&imds.GetMetadataOutput{
					Content: io.NopCloser(errReader{}),
				}, nil)
			},
			expectedError: errors.New("could not read ENIs metadata content: failed to read"),
		},
		{
			name: "TestEC2MetadataInstanceInfo: Error getting block device mappings metadata",
			mockEC2Metadata: func(m *MockEC2Metadata) {
				m.EXPECT().GetInstanceIdentityDocument(gomock.Any(), &imds.GetInstanceIdentityDocumentInput{}).Return(&imds.GetInstanceIdentityDocumentOutput{
					InstanceIdentityDocument: imds.InstanceIdentityDocument{
						InstanceID:       "i-1234567890abcdef0",
						InstanceType:     "c5.xlarge",
						Region:           "us-west-2",
						AvailabilityZone: "us-west-2a",
					},
				}, nil)
				m.EXPECT().GetMetadata(gomock.Any(), &imds.GetMetadataInput{Path: EnisEndpoint}).Return(&imds.GetMetadataOutput{
					Content: io.NopCloser(strings.NewReader("eni-1\neni-2\n")),
				}, nil)
				m.EXPECT().GetMetadata(gomock.Any(), &imds.GetMetadataInput{Path: BlockDevicesEndpoint}).Return(nil, errors.New("failed to get block device mappings metadata"))
			},
			expectedError: errors.New("could not get metadata for block device mappings: failed to get block device mappings metadata"),
		},
		{
			name: "TestEC2MetadataInstanceInfo: Error reading block device mappings metadata content",
			mockEC2Metadata: func(m *MockEC2Metadata) {
				m.EXPECT().GetInstanceIdentityDocument(gomock.Any(), &imds.GetInstanceIdentityDocumentInput{}).Return(&imds.GetInstanceIdentityDocumentOutput{
					InstanceIdentityDocument: imds.InstanceIdentityDocument{
						InstanceID:       "i-1234567890abcdef0",
						InstanceType:     "c5.xlarge",
						Region:           "us-west-2",
						AvailabilityZone: "us-west-2a",
					},
				}, nil)
				m.EXPECT().GetMetadata(gomock.Any(), &imds.GetMetadataInput{Path: EnisEndpoint}).Return(&imds.GetMetadataOutput{
					Content: io.NopCloser(strings.NewReader("eni-1\neni-2\n")),
				}, nil)
				m.EXPECT().GetMetadata(gomock.Any(), &imds.GetMetadataInput{Path: BlockDevicesEndpoint}).Return(&imds.GetMetadataOutput{
					Content: io.NopCloser(errReader{}),
				}, nil)
			},
			expectedError: errors.New("could not read block device mappings metadata content: failed to read"),
		},
		{
			name: "TestEC2MetadataInstanceInfo: Valid metadata with outpost ARN",
			mockEC2Metadata: func(m *MockEC2Metadata) {
				m.EXPECT().GetInstanceIdentityDocument(gomock.Any(), &imds.GetInstanceIdentityDocumentInput{}).Return(&imds.GetInstanceIdentityDocumentOutput{
					InstanceIdentityDocument: imds.InstanceIdentityDocument{
						InstanceID:       "i-1234567890abcdef0",
						InstanceType:     "c5.xlarge",
						Region:           "us-west-2",
						AvailabilityZone: "us-west-2a",
					},
				}, nil)
				m.EXPECT().GetMetadata(gomock.Any(), &imds.GetMetadataInput{Path: EnisEndpoint}).Return(&imds.GetMetadataOutput{
					Content: io.NopCloser(strings.NewReader("eni-1\neni-2\n")),
				}, nil)
				m.EXPECT().GetMetadata(gomock.Any(), &imds.GetMetadataInput{Path: BlockDevicesEndpoint}).Return(&imds.GetMetadataOutput{
					Content: io.NopCloser(strings.NewReader("ebs\nebs\n")),
				}, nil)
				m.EXPECT().GetMetadata(gomock.Any(), &imds.GetMetadataInput{Path: OutpostArnEndpoint}).Return(&imds.GetMetadataOutput{
					Content: io.NopCloser(strings.NewReader("arn:aws:outposts:us-west-2:123456789012:outpost/op-1234567890abcdef0")),
				}, nil)
			},
			expectedMetadata: &Metadata{
				InstanceID:             "i-1234567890abcdef0",
				InstanceType:           "c5.xlarge",
				Region:                 "us-west-2",
				AvailabilityZone:       "us-west-2a",
				NumAttachedENIs:        2,
				NumBlockDeviceMappings: 2,
				OutpostArn: arn.ARN{
					Partition: "aws",
					Service:   "outposts",
					Region:    "us-west-2",
					AccountID: "123456789012",
					Resource:  "op-1234567890abcdef0",
				},
			},
		},
		{
			name: "TestEC2MetadataInstanceInfo: Valid metadata without outpost ARN",
			mockEC2Metadata: func(m *MockEC2Metadata) {
				m.EXPECT().GetInstanceIdentityDocument(gomock.Any(), &imds.GetInstanceIdentityDocumentInput{}).Return(&imds.GetInstanceIdentityDocumentOutput{
					InstanceIdentityDocument: imds.InstanceIdentityDocument{
						InstanceID:       "i-1234567890abcdef0",
						InstanceType:     "c5.xlarge",
						Region:           "us-west-2",
						AvailabilityZone: "us-west-2a",
					},
				}, nil)
				m.EXPECT().GetMetadata(gomock.Any(), &imds.GetMetadataInput{Path: EnisEndpoint}).Return(&imds.GetMetadataOutput{
					Content: io.NopCloser(strings.NewReader("eni-1\neni-2\n")),
				}, nil)
				m.EXPECT().GetMetadata(gomock.Any(), &imds.GetMetadataInput{Path: BlockDevicesEndpoint}).Return(&imds.GetMetadataOutput{
					Content: io.NopCloser(strings.NewReader("ebs\nebs\n")),
				}, nil)
				m.EXPECT().GetMetadata(gomock.Any(), &imds.GetMetadataInput{Path: OutpostArnEndpoint}).Return(nil, errors.New("404 - Not Found"))
			},
			expectedMetadata: &Metadata{
				InstanceID:             "i-1234567890abcdef0",
				InstanceType:           "c5.xlarge",
				Region:                 "us-west-2",
				AvailabilityZone:       "us-west-2a",
				NumAttachedENIs:        2,
				NumBlockDeviceMappings: 2,
				OutpostArn:             arn.ARN{},
			},
		},
		{
			name:              "TestEC2MetadataInstanceInfo: Valid metadata retrieving snow region/AZ from session",
			regionFromSession: "snow",
			mockEC2Metadata: func(m *MockEC2Metadata) {
				m.EXPECT().GetInstanceIdentityDocument(gomock.Any(), &imds.GetInstanceIdentityDocumentInput{}).Return(&imds.GetInstanceIdentityDocumentOutput{
					InstanceIdentityDocument: imds.InstanceIdentityDocument{
						InstanceID:       "i-1234567890abcdef0",
						InstanceType:     "c5.xlarge",
						Region:           "",
						AvailabilityZone: "",
					},
				}, nil)
				m.EXPECT().GetMetadata(gomock.Any(), &imds.GetMetadataInput{Path: EnisEndpoint}).Return(&imds.GetMetadataOutput{
					Content: io.NopCloser(strings.NewReader("eni-1\neni-2\n")),
				}, nil)
				m.EXPECT().GetMetadata(gomock.Any(), &imds.GetMetadataInput{Path: OutpostArnEndpoint}).Return(nil, errors.New("404 - Not Found"))
			},
			expectedMetadata: &Metadata{
				InstanceID:             "i-1234567890abcdef0",
				InstanceType:           "c5.xlarge",
				Region:                 "snow",
				AvailabilityZone:       "snow",
				NumAttachedENIs:        2,
				NumBlockDeviceMappings: 0,
				OutpostArn:             arn.ARN{},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockCtrl := gomock.NewController(t)
			defer mockCtrl.Finish()

			mockEC2Metadata := NewMockEC2Metadata(mockCtrl)
			tc.mockEC2Metadata(mockEC2Metadata)

			metadata, err := EC2MetadataInstanceInfo(mockEC2Metadata, tc.regionFromSession)

			if tc.expectedError != nil {
				require.EqualError(t, err, tc.expectedError.Error())
				require.Nil(t, metadata)
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.expectedMetadata, metadata)
			}
		})
	}
}

func TestDefaultEC2MetadataClient(t *testing.T) {
	_, err := DefaultEC2MetadataClient()
	if err != nil {
		t.Errorf("Error: %v", err)
	}
}

func TestKubernetesAPIInstanceInfo(t *testing.T) {
	testCases := []struct {
		name             string
		nodeName         string
		node             *corev1.Node
		expectedError    string
		expectedMetadata *Metadata
	}{
		{
			name:          "TestKubernetesAPIInstanceInfo: Node name not set",
			nodeName:      "",
			expectedError: "CSI_NODE_NAME env var not set",
		},
		{
			name:          "TestKubernetesAPIInstanceInfo: Error getting node",
			nodeName:      "test-node",
			expectedError: "error getting Node test-node: nodes \"test-node\" not found",
		},
		{
			name:     "TestKubernetesAPIInstanceInfo: Empty provider ID",
			nodeName: "test-node",
			node: &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-node",
				},
				Spec: corev1.NodeSpec{
					ProviderID: "",
				},
			},
			expectedError: "node providerID empty, cannot parse",
		},
		{
			name:     "Instance ID not found",
			nodeName: "test-node",
			node: &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-node",
				},
				Spec: corev1.NodeSpec{
					ProviderID: "aws:///us-west-2a/invalid-instance-id",
				},
			},
			expectedError: "did not find aws instance ID in node providerID string",
		},
		{
			name:     "TestKubernetesAPIInstanceInfo: Missing instance type label",
			nodeName: "test-node",
			node: &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-node",
				},
				Spec: corev1.NodeSpec{
					ProviderID: "aws:///us-west-2a/i-1234567890abcdef0",
				},
			},
			expectedError: "could not retrieve instance type from topology label",
		},
		{
			name:     "TestKubernetesAPIInstanceInfo: Missing region label",
			nodeName: "test-node",
			node: &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-node",
					Labels: map[string]string{
						corev1.LabelInstanceTypeStable: "c5.xlarge",
					},
				},
				Spec: corev1.NodeSpec{
					ProviderID: "aws:///us-west-2a/i-1234567890abcdef0",
				},
			},
			expectedError: "could not retrieve region from topology label",
		},
		{
			name:     "TestKubernetesAPIInstanceInfo: Missing availability zone label",
			nodeName: "test-node",
			node: &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-node",
					Labels: map[string]string{
						corev1.LabelInstanceTypeStable: "c5.xlarge",
						corev1.LabelTopologyRegion:     "us-west-2",
					},
				},
				Spec: corev1.NodeSpec{
					ProviderID: "aws:///us-west-2a/i-1234567890abcdef0",
				},
			},
			expectedError: "could not retrieve AZ from topology label",
		},
		{
			name:     "TestKubernetesAPIInstanceInfo: Valid instance info",
			nodeName: "test-node",
			node: &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-node",
					Labels: map[string]string{
						corev1.LabelInstanceTypeStable: "c5.xlarge",
						corev1.LabelTopologyRegion:     "us-west-2",
						corev1.LabelTopologyZone:       "us-west-2a",
					},
				},
				Spec: corev1.NodeSpec{
					ProviderID: "aws:///us-west-2a/i-1234567890abcdef0",
				},
			},
			expectedMetadata: &Metadata{
				InstanceID:             "i-1234567890abcdef0",
				InstanceType:           "c5.xlarge",
				Region:                 "us-west-2",
				AvailabilityZone:       "us-west-2a",
				NumAttachedENIs:        1,
				NumBlockDeviceMappings: 0,
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			os.Setenv("CSI_NODE_NAME", tc.nodeName)
			defer os.Unsetenv("CSI_NODE_NAME")

			clientset := fake.NewSimpleClientset()
			if tc.node != nil {
				clientset = fake.NewSimpleClientset(tc.node)
			}

			metadata, err := KubernetesAPIInstanceInfo(clientset)

			if tc.expectedError != "" {
				require.EqualError(t, err, tc.expectedError)
				require.Nil(t, metadata)
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.expectedMetadata, metadata)
			}
		})
	}
}

func TestGetInstanceID(t *testing.T) {
	metadata := &Metadata{
		InstanceID: "i-1234567890abcdef0",
	}
	assert.Equal(t, "i-1234567890abcdef0", metadata.GetInstanceID())
}

func TestGetInstanceType(t *testing.T) {
	metadata := &Metadata{
		InstanceType: "c5.xlarge",
	}
	assert.Equal(t, "c5.xlarge", metadata.GetInstanceType())
}

func TestGetRegion(t *testing.T) {
	metadata := &Metadata{
		Region: "us-west-2",
	}
	assert.Equal(t, "us-west-2", metadata.GetRegion())
}

func TestGetAvailabilityZone(t *testing.T) {
	metadata := &Metadata{
		AvailabilityZone: "us-west-2a",
	}
	assert.Equal(t, "us-west-2a", metadata.GetAvailabilityZone())
}

func TestGetNumAttachedENIs(t *testing.T) {
	metadata := &Metadata{
		NumAttachedENIs: 2,
	}
	assert.Equal(t, 2, metadata.GetNumAttachedENIs())
}

func TestGetNumBlockDeviceMappings(t *testing.T) {
	metadata := &Metadata{
		NumBlockDeviceMappings: 3,
	}
	assert.Equal(t, 3, metadata.GetNumBlockDeviceMappings())
}

func TestGetOutpostArn(t *testing.T) {
	outpostArn := arn.ARN{
		Partition: "aws",
		Service:   "outposts",
		Region:    "us-west-2",
		AccountID: "123456789012",
		Resource:  "outpost/op-1234567890abcdef0",
	}
	metadata := &Metadata{
		OutpostArn: outpostArn,
	}
	assert.Equal(t, outpostArn, metadata.GetOutpostArn())
}

type errReader struct{}

func (e errReader) Read(p []byte) (n int, err error) {
	return 0, errors.New("failed to read")
}
