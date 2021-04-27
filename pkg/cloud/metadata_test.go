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
	"github.com/kubernetes-sigs/aws-ebs-csi-driver/pkg/cloud/mocks"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	k8s_testing "k8s.io/client-go/testing"
)

var (
	stdInstanceID       = "instance-1"
	stdInstanceType     = "t2.medium"
	stdRegion           = "instance-1"
	stdAvailabilityZone = "az-1"
)

const (
	nodeName             = "ip-123-45-67-890.us-west-2.compute.internal"
	nodeObjectInstanceID = "i-abcdefgh123456789"
)

func TestNewMetadataService(t *testing.T) {

	validRawOutpostArn := "arn:aws:outposts:us-west-2:111111111111:outpost/op-0aaa000a0aaaa00a0"
	validOutpostArn, _ := arn.Parse(strings.ReplaceAll(validRawOutpostArn, "outpost/", ""))

	testCases := []struct {
		name              string
		isAvailable       bool
		isPartial         bool
		identityDocument  ec2metadata.EC2InstanceIdentityDocument
		rawOutpostArn     string
		outpostArn        arn.ARN
		getInstanceDocErr error
		getOutpostArnErr  error // We should keep this specific to outpost-arn until we need to use more endpoints
		getNodeErr        error
		node              v1.Node
		nodeNameEnvVar    string
	}{
		{
			name:        "success: normal",
			isAvailable: true,
			identityDocument: ec2metadata.EC2InstanceIdentityDocument{
				InstanceID:       stdInstanceID,
				InstanceType:     stdInstanceType,
				Region:           stdRegion,
				AvailabilityZone: stdAvailabilityZone,
			},
			getInstanceDocErr: nil,
		},
		{
			name:        "success: outpost-arn is available",
			isAvailable: true,
			identityDocument: ec2metadata.EC2InstanceIdentityDocument{
				InstanceID:       stdInstanceID,
				InstanceType:     stdInstanceType,
				Region:           stdRegion,
				AvailabilityZone: stdAvailabilityZone,
			},
			rawOutpostArn:     validRawOutpostArn,
			outpostArn:        validOutpostArn,
			getInstanceDocErr: nil,
		},
		{
			name:        "success: outpost-arn is invalid",
			isAvailable: true,
			identityDocument: ec2metadata.EC2InstanceIdentityDocument{
				InstanceID:       stdInstanceID,
				InstanceType:     stdInstanceType,
				Region:           stdRegion,
				AvailabilityZone: stdAvailabilityZone,
			},
			getInstanceDocErr: nil,
		},
		{
			name:        "success: outpost-arn is not found",
			isAvailable: true,
			identityDocument: ec2metadata.EC2InstanceIdentityDocument{
				InstanceID:       stdInstanceID,
				InstanceType:     stdInstanceType,
				Region:           stdRegion,
				AvailabilityZone: stdAvailabilityZone,
			},
			getInstanceDocErr: nil,
			getOutpostArnErr:  fmt.Errorf("404"),
		},
		{
			name:        "success: metadata not available, used k8s api",
			isAvailable: false,
			identityDocument: ec2metadata.EC2InstanceIdentityDocument{
				InstanceID:       stdInstanceID,
				InstanceType:     stdInstanceType,
				Region:           stdRegion,
				AvailabilityZone: stdAvailabilityZone,
			},
			getInstanceDocErr: nil,
			node: v1.Node{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Node",
					APIVersion: "v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: nodeName,
				},
				Spec: v1.NodeSpec{
					ProviderID: "aws:///us-west-2b/i-abcdefgh123456789",
				},
				Status: v1.NodeStatus{},
			},
			nodeNameEnvVar: nodeName,
		},
		{
			name:        "failure: metadata not available, k8s client error",
			isAvailable: false,
			identityDocument: ec2metadata.EC2InstanceIdentityDocument{
				InstanceID:       stdInstanceID,
				InstanceType:     stdInstanceType,
				Region:           stdRegion,
				AvailabilityZone: stdAvailabilityZone,
			},
			getInstanceDocErr: nil,
			getNodeErr:        fmt.Errorf("client failure"),
			nodeNameEnvVar:    nodeName,
		},

		{
			name:        "failure: metadata not available, node name env var not set",
			isAvailable: false,
			identityDocument: ec2metadata.EC2InstanceIdentityDocument{
				InstanceID:       stdInstanceID,
				InstanceType:     stdInstanceType,
				Region:           stdRegion,
				AvailabilityZone: stdAvailabilityZone,
			},
			getInstanceDocErr: nil,
			getNodeErr:        fmt.Errorf("instance metadata is unavailable and CSI_NODE_NAME env var not set"),
			nodeNameEnvVar:    "",
		},
		{
			name:        "failure: metadata not available, no provider ID",
			isAvailable: false,
			identityDocument: ec2metadata.EC2InstanceIdentityDocument{
				InstanceID:       stdInstanceID,
				InstanceType:     stdInstanceType,
				Region:           stdRegion,
				AvailabilityZone: stdAvailabilityZone,
			},
			getInstanceDocErr: nil,
			getNodeErr:        fmt.Errorf("node providerID empty, cannot parse"),
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
			name:        "failure: metadata not available, invalid region",
			isAvailable: false,
			identityDocument: ec2metadata.EC2InstanceIdentityDocument{
				InstanceID:       stdInstanceID,
				InstanceType:     stdInstanceType,
				Region:           stdRegion,
				AvailabilityZone: stdAvailabilityZone,
			},
			getInstanceDocErr: nil,
			getNodeErr:        fmt.Errorf("did not find aws region in node providerID string"),
			node: v1.Node{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Node",
					APIVersion: "v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: nodeName,
				},
				Spec: v1.NodeSpec{
					ProviderID: "aws:///us-est-2b/i-abcdefgh123456789", // invalid region
				},
				Status: v1.NodeStatus{},
			},
			nodeNameEnvVar: nodeName,
		},
		{
			name:        "failure: metadata not available, invalid az",
			isAvailable: false,
			identityDocument: ec2metadata.EC2InstanceIdentityDocument{
				InstanceID:       stdInstanceID,
				InstanceType:     stdInstanceType,
				Region:           stdRegion,
				AvailabilityZone: stdAvailabilityZone,
			},
			getInstanceDocErr: nil,
			getNodeErr:        fmt.Errorf("did not find aws availability zone in node providerID string"),
			node: v1.Node{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Node",
					APIVersion: "v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: nodeName,
				},
				Spec: v1.NodeSpec{
					ProviderID: "aws:///us-west-21/i-abcdefgh123456789", // invalid AZ
				},
				Status: v1.NodeStatus{},
			},
			nodeNameEnvVar: nodeName,
		},
		{
			name:        "failure: metadata not available, invalid instance id",
			isAvailable: false,
			identityDocument: ec2metadata.EC2InstanceIdentityDocument{
				InstanceID:       stdInstanceID,
				InstanceType:     stdInstanceType,
				Region:           stdRegion,
				AvailabilityZone: stdAvailabilityZone,
			},
			getInstanceDocErr: nil,
			getNodeErr:        fmt.Errorf("did not find aws instance ID in node providerID string"),
			node: v1.Node{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Node",
					APIVersion: "v1",
				},
				ObjectMeta: metav1.ObjectMeta{
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
			name:        "fail: GetInstanceIdentityDocument returned error",
			isAvailable: true,
			identityDocument: ec2metadata.EC2InstanceIdentityDocument{
				InstanceID:       stdInstanceID,
				InstanceType:     stdInstanceType,
				Region:           stdRegion,
				AvailabilityZone: stdAvailabilityZone,
			},
			getInstanceDocErr: fmt.Errorf(""),
		},
		{
			name:        "fail: GetInstanceIdentityDocument returned empty instance",
			isAvailable: true,
			isPartial:   true,
			identityDocument: ec2metadata.EC2InstanceIdentityDocument{
				InstanceID:       "",
				InstanceType:     stdInstanceType,
				Region:           stdRegion,
				AvailabilityZone: stdAvailabilityZone,
			},
			getInstanceDocErr: nil,
		},
		{
			name:        "fail: GetInstanceIdentityDocument returned empty region",
			isAvailable: true,
			isPartial:   true,
			identityDocument: ec2metadata.EC2InstanceIdentityDocument{
				InstanceID:       stdInstanceID,
				InstanceType:     stdInstanceType,
				Region:           "",
				AvailabilityZone: stdAvailabilityZone,
			},
			getInstanceDocErr: nil,
		},
		{
			name:        "fail: GetInstanceIdentityDocument returned empty az",
			isAvailable: true,
			isPartial:   true,
			identityDocument: ec2metadata.EC2InstanceIdentityDocument{
				InstanceID:       stdInstanceID,
				InstanceType:     stdInstanceType,
				Region:           stdRegion,
				AvailabilityZone: "",
			},
			getInstanceDocErr: nil,
		},
		{
			name:        "fail: outpost-arn failed",
			isAvailable: true,
			identityDocument: ec2metadata.EC2InstanceIdentityDocument{
				InstanceID:       stdInstanceID,
				InstanceType:     stdInstanceType,
				Region:           stdRegion,
				AvailabilityZone: stdAvailabilityZone,
			},
			getInstanceDocErr: nil,
			getOutpostArnErr:  fmt.Errorf("405"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			clientset := fake.NewSimpleClientset(&tc.node)
			if tc.name == "failure: metadata not available, k8s client error" {
				clientset.PrependReactor("get", "*", func(action k8s_testing.Action) (handled bool, ret runtime.Object, err error) {
					return true, nil, fmt.Errorf("client failure")
				})
			}
			mockCtrl := gomock.NewController(t)
			mockEC2Metadata := mocks.NewMockEC2Metadata(mockCtrl)

			mockEC2Metadata.EXPECT().Available().Return(tc.isAvailable)
			os.Setenv("CSI_NODE_NAME", tc.nodeNameEnvVar)
			if tc.isAvailable {
				mockEC2Metadata.EXPECT().GetInstanceIdentityDocument().Return(tc.identityDocument, tc.getInstanceDocErr)
			}

			if tc.isAvailable && tc.getInstanceDocErr == nil && !tc.isPartial {
				mockEC2Metadata.EXPECT().GetMetadata(OutpostArnEndpoint).Return(tc.rawOutpostArn, tc.getOutpostArnErr)
			}

			m, err := NewMetadataService(mockEC2Metadata, clientset)
			if tc.isAvailable && tc.getInstanceDocErr == nil && tc.getOutpostArnErr == nil && !tc.isPartial {
				if err != nil {
					t.Fatalf("NewMetadataService() failed: expected no error, got %v", err)
				}

				if m.GetInstanceID() != tc.identityDocument.InstanceID {
					t.Fatalf("GetInstanceID() failed: expected %v, got %v", tc.identityDocument.InstanceID, m.GetInstanceID())
				}

				if m.GetInstanceType() != tc.identityDocument.InstanceType {
					t.Fatalf("GetInstanceType() failed: expected %v, got %v", tc.identityDocument.InstanceType, m.GetInstanceType())
				}

				if m.GetRegion() != tc.identityDocument.Region {
					t.Fatalf("GetRegion() failed: expected %v, got %v", tc.identityDocument.Region, m.GetRegion())
				}

				if m.GetAvailabilityZone() != tc.identityDocument.AvailabilityZone {
					t.Fatalf("GetAvailabilityZone() failed: expected %v, got %v", tc.identityDocument.AvailabilityZone, m.GetAvailabilityZone())
				}

				if m.GetOutpostArn() != tc.outpostArn {
					t.Fatalf("GetOutpostArn() failed: expected %v, got %v", tc.outpostArn, m.GetOutpostArn())
				}
			} else if !tc.isAvailable {
				if tc.name == "success: metadata not available, used k8s api" {
					if err != nil {
						t.Fatalf("NewMetadataService() failed: expected no error, got %v", err)
					}
					if m.GetInstanceID() != nodeObjectInstanceID {
						t.Fatalf("NewMetadataService() failed: got wrong instance ID %v, expected %v", m.GetInstanceID(), nodeObjectInstanceID)
					}
					if m.GetRegion() != "us-west-2" {
						t.Fatalf("NewMetadataService() failed: got wrong region %v, expected %v", m.GetRegion(), "us-west-2")
					}
					if m.GetAvailabilityZone() != "us-west-2b" {
						t.Fatalf("NewMetadataService() failed: got wrong AZ %v, expected %v", m.GetRegion(), "us-west-2b")
					}
					if m.GetOutpostArn() != tc.outpostArn {
						t.Fatalf("GetOutpostArn() failed: got %v, expected %v", m.GetOutpostArn(), tc.outpostArn)
					}
				} else {
					if err == nil {
						t.Fatalf("NewMetadataService() failed: expected error but got nothing")
					}
					if err.Error() != tc.getNodeErr.Error() {
						t.Fatalf("NewMetadataService() returned an unexpected error. Expected %v, got %v", tc.getNodeErr, err)
					}
				}
			} else {
				if err == nil && tc.getOutpostArnErr == nil {
					t.Fatal("NewMetadataService() failed: expected error when GetInstanceIdentityDocument returns partial data, got nothing")
				}
			}

			mockCtrl.Finish()
		})
	}
}
