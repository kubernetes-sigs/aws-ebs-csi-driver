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
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go/aws/arn"
	"github.com/aws/aws-sdk-go/aws/ec2metadata"
	"github.com/golang/mock/gomock"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
	k8s_testing "k8s.io/client-go/testing"
)

const (
	nodeName             = "ip-123-45-67-890.us-west-2.compute.internal"
	stdInstanceID        = "i-abcdefgh123456789"
	stdInstanceType      = "t2.medium"
	stdRegion            = "us-west-2"
	stdAvailabilityZone  = "us-west-2b"
	snowRegion           = "snow"
	snowAvailabilityZone = "snow"
)

func TestNewMetadataService(t *testing.T) {

	validRawOutpostArn := "arn:aws:outposts:us-west-2:111111111111:outpost/op-0aaa000a0aaaa00a0"
	validOutpostArn, _ := arn.Parse(strings.ReplaceAll(validRawOutpostArn, "outpost/", ""))

	testCases := []struct {
		name                             string
		ec2metadataAvailable             bool
		clientsetReactors                func(*fake.Clientset)
		getInstanceIdentityDocumentValue ec2metadata.EC2InstanceIdentityDocument
		getInstanceIdentityDocumentError error
		invalidInstanceIdentityDocument  bool
		getMetadataValue                 string
		getMetadataError                 error
		imdsBlockDeviceOutput            string
		imdsENIOutput                    string
		expectedENIs                     int
		expectedBlockDevices             int
		expectedOutpostArn               arn.ARN
		expectedErr                      error
		node                             v1.Node
		nodeNameEnvVar                   string
		regionFromSession                string
	}{
		{
			name:                 "success: normal",
			ec2metadataAvailable: true,
			getInstanceIdentityDocumentValue: ec2metadata.EC2InstanceIdentityDocument{
				InstanceID:       stdInstanceID,
				InstanceType:     stdInstanceType,
				Region:           stdRegion,
				AvailabilityZone: stdAvailabilityZone,
			},
			imdsENIOutput: "00:00:00:00:00:00",
			expectedENIs:  1,
		},
		{
			name:                 "success: outpost-arn is available",
			ec2metadataAvailable: true,
			getInstanceIdentityDocumentValue: ec2metadata.EC2InstanceIdentityDocument{
				InstanceID:       stdInstanceID,
				InstanceType:     stdInstanceType,
				Region:           stdRegion,
				AvailabilityZone: stdAvailabilityZone,
			},
			getMetadataValue:   validRawOutpostArn,
			expectedOutpostArn: validOutpostArn,
			imdsENIOutput:      "00:00:00:00:00:00",
			expectedENIs:       1,
		},
		{
			name:                 "success: outpost-arn is invalid",
			ec2metadataAvailable: true,
			getInstanceIdentityDocumentValue: ec2metadata.EC2InstanceIdentityDocument{
				InstanceID:       stdInstanceID,
				InstanceType:     stdInstanceType,
				Region:           stdRegion,
				AvailabilityZone: stdAvailabilityZone,
			},
			getMetadataValue: "foo",
			imdsENIOutput:    "00:00:00:00:00:00",
			expectedENIs:     1,
		},
		{
			name:                 "success: outpost-arn is not found",
			ec2metadataAvailable: true,
			getInstanceIdentityDocumentValue: ec2metadata.EC2InstanceIdentityDocument{
				InstanceID:       stdInstanceID,
				InstanceType:     stdInstanceType,
				Region:           stdRegion,
				AvailabilityZone: stdAvailabilityZone,
			},
			getMetadataError: fmt.Errorf("404"),
			imdsENIOutput:    "00:00:00:00:00:00",
			expectedENIs:     1,
		},
		{
			name:                 "success: metadata not available, used k8s api",
			ec2metadataAvailable: false,
			node: v1.Node{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Node",
					APIVersion: "v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"node.kubernetes.io/instance-type": stdInstanceType,
						"topology.kubernetes.io/region":    stdRegion,
						"topology.kubernetes.io/zone":      stdAvailabilityZone,
					},
					Name: nodeName,
				},
				Spec: v1.NodeSpec{
					ProviderID: "aws:///" + stdAvailabilityZone + "/" + stdInstanceID,
				},
				Status: v1.NodeStatus{},
			},
			expectedENIs:   1,
			nodeNameEnvVar: nodeName,
		},
		{
			name:                 "failure: metadata not available, k8s client error",
			ec2metadataAvailable: false,
			clientsetReactors: func(clientset *fake.Clientset) {
				clientset.PrependReactor("get", "*", func(action k8s_testing.Action) (handled bool, ret runtime.Object, err error) {
					return true, nil, fmt.Errorf("client failure")
				})
			},
			expectedErr:    fmt.Errorf("error getting Node %s: client failure", nodeName),
			nodeNameEnvVar: nodeName,
		},

		{
			name:                 "failure: metadata not available, node name env var not set",
			ec2metadataAvailable: false,
			expectedErr:          fmt.Errorf("CSI_NODE_NAME env var not set"),
			nodeNameEnvVar:       "",
		},
		{
			name:                 "failure: metadata not available, no provider ID",
			ec2metadataAvailable: false,
			expectedErr:          fmt.Errorf("node providerID empty, cannot parse"),
			node: v1.Node{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Node",
					APIVersion: "v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: nodeName,
				},
				Spec: v1.NodeSpec{
					ProviderID: "",
				},
				Status: v1.NodeStatus{},
			},
			nodeNameEnvVar: nodeName,
		},
		{
			name:                 "failure: metadata not available, could not retrieve region",
			ec2metadataAvailable: false,
			expectedErr:          fmt.Errorf("could not retrieve region from topology label"),
			node: v1.Node{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Node",
					APIVersion: "v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"node.kubernetes.io/instance-type": stdInstanceType,
						"topology.kubernetes.io/zone":      stdAvailabilityZone,
					},
					Name: nodeName,
				},
				Spec: v1.NodeSpec{
					ProviderID: "aws:///" + stdAvailabilityZone + "/" + stdInstanceID,
				},
				Status: v1.NodeStatus{},
			},
			nodeNameEnvVar: nodeName,
		},
		{
			name:                 "failure: metadata not available, could not retrieve AZ",
			ec2metadataAvailable: false,
			expectedErr:          fmt.Errorf("could not retrieve AZ from topology label"),
			node: v1.Node{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Node",
					APIVersion: "v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"node.kubernetes.io/instance-type": stdInstanceType,
						"topology.kubernetes.io/region":    stdRegion,
					},
					Name: nodeName,
				},
				Spec: v1.NodeSpec{
					ProviderID: "aws:///" + stdAvailabilityZone + "/" + stdInstanceID,
				},
				Status: v1.NodeStatus{},
			},
			nodeNameEnvVar: nodeName,
		},
		{
			name:                 "failure: metadata not available, invalid instance id",
			ec2metadataAvailable: false,
			expectedErr:          fmt.Errorf("did not find aws instance ID in node providerID string"),
			node: v1.Node{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Node",
					APIVersion: "v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"node.kubernetes.io/instance-type": stdInstanceType,
						"topology.kubernetes.io/region":    stdRegion,
						"topology.kubernetes.io/zone":      stdAvailabilityZone,
					},
					Name: nodeName,
				},
				Spec: v1.NodeSpec{
					ProviderID: "aws:///us-west-2b/i-", // invalid instance ID
				},
				Status: v1.NodeStatus{},
			},
			nodeNameEnvVar: nodeName,
		},
		{
			name:                             "fail: GetInstanceIdentityDocument returned error",
			ec2metadataAvailable:             true,
			getInstanceIdentityDocumentError: fmt.Errorf("foo"),
			expectedErr:                      fmt.Errorf("could not get EC2 instance identity metadata: foo"),
		},
		{
			name:                 "fail: GetInstanceIdentityDocument returned empty instance",
			ec2metadataAvailable: true,
			getInstanceIdentityDocumentValue: ec2metadata.EC2InstanceIdentityDocument{
				InstanceID:       "",
				InstanceType:     stdInstanceType,
				Region:           stdRegion,
				AvailabilityZone: stdAvailabilityZone,
			},
			invalidInstanceIdentityDocument: true,
			expectedErr:                     fmt.Errorf("could not get valid EC2 instance ID"),
		},
		{
			name:                 "fail: GetInstanceIdentityDocument returned empty region",
			ec2metadataAvailable: true,
			getInstanceIdentityDocumentValue: ec2metadata.EC2InstanceIdentityDocument{
				InstanceID:       stdInstanceID,
				InstanceType:     stdInstanceType,
				Region:           "",
				AvailabilityZone: stdAvailabilityZone,
			},
			invalidInstanceIdentityDocument: true,
			expectedErr:                     fmt.Errorf("could not get valid EC2 region"),
		},
		{
			name:                 "fail: GetInstanceIdentityDocument returned empty az",
			ec2metadataAvailable: true,
			getInstanceIdentityDocumentValue: ec2metadata.EC2InstanceIdentityDocument{
				InstanceID:       stdInstanceID,
				InstanceType:     stdInstanceType,
				Region:           stdRegion,
				AvailabilityZone: "",
			},
			invalidInstanceIdentityDocument: true,
			expectedErr:                     fmt.Errorf("could not get valid EC2 availability zone"),
		},
		{
			name:                 "fail: outpost-arn failed",
			ec2metadataAvailable: true,
			getInstanceIdentityDocumentValue: ec2metadata.EC2InstanceIdentityDocument{
				InstanceID:       stdInstanceID,
				InstanceType:     stdInstanceType,
				Region:           stdRegion,
				AvailabilityZone: stdAvailabilityZone,
			},
			imdsENIOutput:    "00:00:00:00:00:00",
			expectedENIs:     1,
			getMetadataError: fmt.Errorf("405"),
			expectedErr:      fmt.Errorf("something went wrong while getting EC2 outpost arn: 405"),
		},
		{
			name:                 "success: GetMetadata() returns correct number of ENIs",
			ec2metadataAvailable: true,
			getInstanceIdentityDocumentValue: ec2metadata.EC2InstanceIdentityDocument{
				InstanceID:       stdInstanceID,
				InstanceType:     stdInstanceType,
				Region:           stdRegion,
				AvailabilityZone: stdAvailabilityZone,
			},
			imdsENIOutput: "00:00:00:00:00:00\n00:00:00:00:00:01",
			expectedENIs:  2,
		},
		{
			name:                 "success: GetMetadata() returns correct number of block device mappings",
			ec2metadataAvailable: true,
			getInstanceIdentityDocumentValue: ec2metadata.EC2InstanceIdentityDocument{
				InstanceID:       stdInstanceID,
				InstanceType:     stdInstanceType,
				Region:           stdRegion,
				AvailabilityZone: stdAvailabilityZone,
			},
			imdsENIOutput:         "00:00:00:00:00:00",
			expectedENIs:          1,
			imdsBlockDeviceOutput: "ami\nroot\nebs1\nebs2",
			expectedBlockDevices:  3,
		},
		{
			name:                 "success: region from session is snow",
			ec2metadataAvailable: true,
			getInstanceIdentityDocumentValue: ec2metadata.EC2InstanceIdentityDocument{
				InstanceID:       stdInstanceID,
				InstanceType:     stdInstanceType,
				Region:           "",
				AvailabilityZone: "",
			},
			imdsENIOutput:        "00:00:00:00:00:00",
			expectedENIs:         1,
			regionFromSession:    snowRegion,
			expectedBlockDevices: 1,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			clientset := fake.NewSimpleClientset(&tc.node)
			clientsetInitialized := false
			if tc.clientsetReactors != nil {
				tc.clientsetReactors(clientset)
			}

			mockCtrl := gomock.NewController(t)
			mockEC2Metadata := NewMockEC2Metadata(mockCtrl)

			ec2MetadataClient := func() (EC2Metadata, error) { return mockEC2Metadata, nil }
			k8sAPIClient := func() (kubernetes.Interface, error) { clientsetInitialized = true; return clientset, nil }

			mockEC2Metadata.EXPECT().Available().Return(tc.ec2metadataAvailable)
			if tc.ec2metadataAvailable {
				mockEC2Metadata.EXPECT().GetInstanceIdentityDocument().Return(tc.getInstanceIdentityDocumentValue, tc.getInstanceIdentityDocumentError)

				// GetMetadata is to get the outpost ARN. It should be skipped if
				// GetInstanceIdentityDocument returns an error or (somehow?) partial
				// output
				if tc.getInstanceIdentityDocumentError == nil && !tc.invalidInstanceIdentityDocument {
					mockEC2Metadata.EXPECT().GetMetadata(enisEndpoint).Return(tc.imdsENIOutput, nil)
					mockEC2Metadata.EXPECT().GetMetadata(blockDevicesEndpoint).Return(tc.imdsBlockDeviceOutput, nil).AnyTimes()

					if tc.getMetadataValue != "" || tc.getMetadataError != nil {
						mockEC2Metadata.EXPECT().GetMetadata(outpostArnEndpoint).Return(tc.getMetadataValue, tc.getMetadataError)

					} else {
						mockEC2Metadata.EXPECT().GetMetadata(outpostArnEndpoint).Return("", fmt.Errorf("404"))
					}
				}

				if clientsetInitialized == true {
					t.Errorf("kubernetes client was unexpectedly initialized when metadata is available!")
					if len(clientset.Actions()) > 0 {
						t.Errorf("kubernetes client was unexpectedly called! %v", clientset.Actions())
					}
				}
			}

			os.Setenv("CSI_NODE_NAME", tc.nodeNameEnvVar)
			var m MetadataService
			var err error
			if tc.regionFromSession == snowRegion {
				m, err = NewMetadataService(ec2MetadataClient, k8sAPIClient, snowRegion)
			} else {
				m, err = NewMetadataService(ec2MetadataClient, k8sAPIClient, stdRegion)
			}
			if err != nil {
				if tc.expectedErr == nil {
					t.Errorf("got error %q, expected no error", err)
				} else if err.Error() != tc.expectedErr.Error() {
					t.Errorf("got error %q, expected %q", err, tc.expectedErr)
				}
			} else {
				if m == nil {
					t.Fatalf("metadataService is unexpectedly nil!")
				}
				if m.GetInstanceID() != stdInstanceID {
					t.Errorf("NewMetadataService() failed: got wrong instance ID %v, expected %v", m.GetInstanceID(), stdInstanceID)
				}
				if m.GetInstanceType() != stdInstanceType {
					t.Errorf("GetInstanceType() failed: got wrong instance type %v, expected %v", m.GetInstanceType(), stdInstanceType)
				}
				if m.GetRegion() != stdRegion && m.GetRegion() != snowRegion {
					t.Errorf("NewMetadataService() failed: got wrong region %v, expected %v", m.GetRegion(), stdRegion)
				}
				if m.GetAvailabilityZone() != stdAvailabilityZone && m.GetAvailabilityZone() != snowAvailabilityZone {
					t.Errorf("NewMetadataService() failed: got wrong AZ %v, expected %v", m.GetAvailabilityZone(), stdAvailabilityZone)
				}
				if m.GetOutpostArn() != tc.expectedOutpostArn {
					t.Errorf("GetOutpostArn() failed: got %v, expected %v", m.GetOutpostArn(), tc.expectedOutpostArn)
				}
				if m.GetNumAttachedENIs() != tc.expectedENIs {
					t.Errorf("GetMetadata() failed for %s: got %v, expected %v", enisEndpoint, m.GetNumAttachedENIs(), tc.expectedENIs)
				}
				if m.GetNumBlockDeviceMappings() != tc.expectedBlockDevices {
					t.Errorf("GetMetadata() failed for %s: got %v, expected %v", blockDevicesEndpoint, m.GetNumBlockDeviceMappings(), tc.expectedBlockDevices)
				}
			}
			mockCtrl.Finish()
		})
	}
}
