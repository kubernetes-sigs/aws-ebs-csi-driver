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

package metadata

import (
	"errors"
	"io"
	"slices"
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
		metadataSources  []string
		imdsDisabled     bool
		IMDSError        error
		k8sAPIError      error
		expectedMetadata *Metadata
		expectedError    error
	}{
		{
			name:            "TestNewMetadataService: Default MetadataSources, IMDS available",
			metadataSources: DefaultMetadataSources,
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
			name:            "TestNewMetadataService: Default MetadataSources, AWS_EC2_METADATA_DISABLED=true, K8s API available",
			metadataSources: DefaultMetadataSources,
			imdsDisabled:    true,
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
			name:            "TestNewMetadataService: Default MetadataSources, IMDS error, K8s API available",
			metadataSources: DefaultMetadataSources,
			IMDSError:       errors.New("IMDS error"),
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
			name:            "TestNewMetadataService: Default MetadataSources, IMDS error, K8s API error",
			metadataSources: DefaultMetadataSources,
			IMDSError:       errors.New("IMDS error"),
			k8sAPIError:     errors.New("K8s API error"),
			expectedError:   sourcesUnavailableErr(DefaultMetadataSources),
		},
		{
			name:            "TestNewMetadataService: MetadataSources IMDS-only, IMDS error",
			metadataSources: []string{SourceIMDS},
			IMDSError:       errors.New("IMDS error"),
			expectedError:   sourcesUnavailableErr([]string{SourceIMDS}),
		},
		{
			name:            "TestNewMetadataService: MetadataSources K8s-only, success",
			metadataSources: []string{SourceK8s},
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
			name:            "TestNewMetadataService: invalid source error",
			metadataSources: []string{"invalid"},
			expectedError:   InvalidSourceErr([]string{"invalid"}, "invalid"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockIMDS := NewMockIMDS(ctrl)
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

			t.Setenv("CSI_NODE_NAME", "test-node")
			if tc.imdsDisabled {
				t.Setenv("AWS_EC2_METADATA_DISABLED", "true")
			} else {
				t.Setenv("AWS_EC2_METADATA_DISABLED", "false")
			}

			if tc.IMDSError == nil && !tc.imdsDisabled && (slices.Contains(tc.metadataSources, SourceIMDS)) {
				mockIMDS.EXPECT().GetInstanceIdentityDocument(gomock.Any(), &imds.GetInstanceIdentityDocumentInput{}).Return(&imds.GetInstanceIdentityDocumentOutput{
					InstanceIdentityDocument: imds.InstanceIdentityDocument{
						InstanceID:       "i-1234567890abcdef0",
						InstanceType:     "c5.xlarge",
						Region:           "us-west-2",
						AvailabilityZone: "us-west-2a",
					},
				}, nil)
				mockIMDS.EXPECT().GetMetadata(gomock.Any(), &imds.GetMetadataInput{Path: EnisEndpoint}).Return(&imds.GetMetadataOutput{
					Content: io.NopCloser(strings.NewReader("01:23:45:67:89:ab")),
				}, nil)
				mockIMDS.EXPECT().GetMetadata(gomock.Any(), &imds.GetMetadataInput{Path: BlockDevicesEndpoint}).Return(&imds.GetMetadataOutput{
					Content: io.NopCloser(strings.NewReader("ebs\nebs\n")),
				}, nil)
				mockIMDS.EXPECT().GetMetadata(gomock.Any(), &imds.GetMetadataInput{Path: OutpostArnEndpoint}).Return(nil, errors.New("404 - Not Found"))
			}

			cfg := MetadataServiceConfig{
				MetadataSources: tc.metadataSources,
				IMDSClient: func() (IMDS, error) {
					if tc.IMDSError != nil {
						return nil, tc.IMDSError
					}
					return mockIMDS, nil
				},
				K8sAPIClient: mockK8sClient,
			}

			metadata, err := NewMetadataService(cfg, "us-west-2")

			if tc.expectedError != nil {
				require.EqualError(t, err, tc.expectedError.Error())
			} else {
				require.NoError(t, err)
				assert.Equal(t, tc.expectedMetadata.InstanceID, metadata.GetInstanceID())
				assert.Equal(t, tc.expectedMetadata.InstanceType, metadata.GetInstanceType())
				assert.Equal(t, tc.expectedMetadata.Region, metadata.GetRegion())
				assert.Equal(t, tc.expectedMetadata.AvailabilityZone, metadata.GetAvailabilityZone())
				assert.Equal(t, tc.expectedMetadata.NumAttachedENIs, metadata.GetNumAttachedENIs())
				assert.Equal(t, tc.expectedMetadata.NumBlockDeviceMappings, metadata.GetNumBlockDeviceMappings())
				assert.Equal(t, tc.expectedMetadata.OutpostArn, metadata.GetOutpostArn())
			}
		})
	}
}

func TestIMDSInstanceInfo(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	testCases := []struct {
		name             string
		mockIMDS         func(m *MockIMDS)
		expectedMetadata *Metadata
		expectedError    error
	}{
		{
			name: "TestIMDSInstanceInfo: Error getting instance identity document",
			mockIMDS: func(m *MockIMDS) {
				m.EXPECT().GetInstanceIdentityDocument(gomock.Any(), &imds.GetInstanceIdentityDocumentInput{}).Return(nil, errors.New("failed to get instance identity document"))
			},
			expectedError: errors.New("could not get IMDS metadata: failed to get instance identity document"),
		},
		{
			name: "TestIMDSInstanceInfo: Empty instance ID",
			mockIMDS: func(m *MockIMDS) {
				m.EXPECT().GetInstanceIdentityDocument(gomock.Any(), &imds.GetInstanceIdentityDocumentInput{}).Return(&imds.GetInstanceIdentityDocumentOutput{
					InstanceIdentityDocument: imds.InstanceIdentityDocument{
						InstanceID: "",
					},
				}, nil)
			},
			expectedError: errors.New("could not get valid EC2 instance ID"),
		},
		{
			name: "TestIMDSInstanceInfo: Empty instance type",
			mockIMDS: func(m *MockIMDS) {
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
			name: "TestIMDSInstanceInfo: Empty region and invalid region from session",
			mockIMDS: func(m *MockIMDS) {
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
			name: "TestIMDSInstanceInfo: Empty availability zone and invalid region from session",
			mockIMDS: func(m *MockIMDS) {
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
			name: "TestIMDSInstanceInfo: Error getting ENIs metadata",
			mockIMDS: func(m *MockIMDS) {
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
			name: "TestIMDSInstanceInfo: Error reading ENIs metadata content",
			mockIMDS: func(m *MockIMDS) {
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
			name: "TestIMDSInstanceInfo: Error getting block device mappings metadata",
			mockIMDS: func(m *MockIMDS) {
				m.EXPECT().GetInstanceIdentityDocument(gomock.Any(), &imds.GetInstanceIdentityDocumentInput{}).Return(&imds.GetInstanceIdentityDocumentOutput{
					InstanceIdentityDocument: imds.InstanceIdentityDocument{
						InstanceID:       "i-1234567890abcdef0",
						InstanceType:     "c5.xlarge",
						Region:           "us-west-2",
						AvailabilityZone: "us-west-2a",
					},
				}, nil)
				m.EXPECT().GetMetadata(gomock.Any(), &imds.GetMetadataInput{Path: EnisEndpoint}).Return(&imds.GetMetadataOutput{
					Content: io.NopCloser(strings.NewReader("eni-1\neni-2")),
				}, nil)
				m.EXPECT().GetMetadata(gomock.Any(), &imds.GetMetadataInput{Path: BlockDevicesEndpoint}).Return(nil, errors.New("failed to get block device mappings metadata"))
			},
			expectedError: errors.New("could not get metadata for block device mappings: failed to get block device mappings metadata"),
		},
		{
			name: "TestIMDSInstanceInfo: Error reading block device mappings metadata content",
			mockIMDS: func(m *MockIMDS) {
				m.EXPECT().GetInstanceIdentityDocument(gomock.Any(), &imds.GetInstanceIdentityDocumentInput{}).Return(&imds.GetInstanceIdentityDocumentOutput{
					InstanceIdentityDocument: imds.InstanceIdentityDocument{
						InstanceID:       "i-1234567890abcdef0",
						InstanceType:     "c5.xlarge",
						Region:           "us-west-2",
						AvailabilityZone: "us-west-2a",
					},
				}, nil)
				m.EXPECT().GetMetadata(gomock.Any(), &imds.GetMetadataInput{Path: EnisEndpoint}).Return(&imds.GetMetadataOutput{
					Content: io.NopCloser(strings.NewReader("01:23:45:67:89:ab\n02:23:45:67:89:ab")),
				}, nil)
				m.EXPECT().GetMetadata(gomock.Any(), &imds.GetMetadataInput{Path: BlockDevicesEndpoint}).Return(&imds.GetMetadataOutput{
					Content: io.NopCloser(errReader{}),
				}, nil)
			},
			expectedError: errors.New("could not read block device mappings metadata content: failed to read"),
		},
		{
			name: "TestIMDSInstanceInfo: Valid metadata with outpost ARN",
			mockIMDS: func(m *MockIMDS) {
				m.EXPECT().GetInstanceIdentityDocument(gomock.Any(), &imds.GetInstanceIdentityDocumentInput{}).Return(&imds.GetInstanceIdentityDocumentOutput{
					InstanceIdentityDocument: imds.InstanceIdentityDocument{
						InstanceID:       "i-1234567890abcdef0",
						InstanceType:     "c5.xlarge",
						Region:           "us-west-2",
						AvailabilityZone: "us-west-2a",
					},
				}, nil)
				m.EXPECT().GetMetadata(gomock.Any(), &imds.GetMetadataInput{Path: EnisEndpoint}).Return(&imds.GetMetadataOutput{
					Content: io.NopCloser(strings.NewReader("01:23:45:67:89:ab\n02:23:45:67:89:ab")),
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
			name: "TestIMDSInstanceInfo: Valid metadata without outpost ARN",
			mockIMDS: func(m *MockIMDS) {
				m.EXPECT().GetInstanceIdentityDocument(gomock.Any(), &imds.GetInstanceIdentityDocumentInput{}).Return(&imds.GetInstanceIdentityDocumentOutput{
					InstanceIdentityDocument: imds.InstanceIdentityDocument{
						InstanceID:       "i-1234567890abcdef0",
						InstanceType:     "c5.xlarge",
						Region:           "us-west-2",
						AvailabilityZone: "us-west-2a",
					},
				}, nil)
				m.EXPECT().GetMetadata(gomock.Any(), &imds.GetMetadataInput{Path: EnisEndpoint}).Return(&imds.GetMetadataOutput{
					Content: io.NopCloser(strings.NewReader("01:23:45:67:89:ab\n02:23:45:67:89:ab")),
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
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockCtrl := gomock.NewController(t)
			defer mockCtrl.Finish()

			mockIMDS := NewMockIMDS(mockCtrl)
			tc.mockIMDS(mockIMDS)

			metadata, err := IMDSInstanceInfo(mockIMDS)

			if tc.expectedError != nil {
				require.EqualError(t, err, tc.expectedError.Error())
				require.Nil(t, metadata)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tc.expectedMetadata.InstanceID, metadata.GetInstanceID())
				assert.Equal(t, tc.expectedMetadata.InstanceType, metadata.GetInstanceType())
				assert.Equal(t, tc.expectedMetadata.Region, metadata.GetRegion())
				assert.Equal(t, tc.expectedMetadata.AvailabilityZone, metadata.GetAvailabilityZone())
				assert.Equal(t, tc.expectedMetadata.NumAttachedENIs, metadata.GetNumAttachedENIs())
				assert.Equal(t, tc.expectedMetadata.NumBlockDeviceMappings, metadata.GetNumBlockDeviceMappings())
				assert.Equal(t, tc.expectedMetadata.OutpostArn, metadata.GetOutpostArn())
			}
		})
	}
}

func TestDefaultIMDSClient(t *testing.T) {
	_, err := DefaultIMDSClient()
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
			t.Setenv("CSI_NODE_NAME", tc.nodeName)

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
