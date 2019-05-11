/*
Copyright 2018 The Kubernetes Authors.

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
	"testing"

	"github.com/aws/aws-sdk-go/aws/ec2metadata"
	"github.com/golang/mock/gomock"
	"github.com/kubernetes-sigs/aws-ebs-csi-driver/pkg/cloud/mocks"
)

var (
	stdInstanceID       = "instance-1"
	stdInstanceType     = "t2.medium"
	stdRegion           = "instance-1"
	stdAvailabilityZone = "az-1"
)

func TestNewMetadataService(t *testing.T) {
	testCases := []struct {
		name             string
		isAvailable      bool
		isPartial        bool
		identityDocument ec2metadata.EC2InstanceIdentityDocument
		err              error
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
			err: nil,
		},
		{
			name:        "fail: metadata not available",
			isAvailable: false,
			identityDocument: ec2metadata.EC2InstanceIdentityDocument{
				InstanceID:       stdInstanceID,
				InstanceType:     stdInstanceType,
				Region:           stdRegion,
				AvailabilityZone: stdAvailabilityZone,
			},
			err: nil,
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
			err: fmt.Errorf(""),
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
			err: nil,
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
			err: nil,
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
			err: nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockCtrl := gomock.NewController(t)
			mockEC2Metadata := mocks.NewMockEC2Metadata(mockCtrl)

			mockEC2Metadata.EXPECT().Available().Return(tc.isAvailable)
			if tc.isAvailable {
				mockEC2Metadata.EXPECT().GetInstanceIdentityDocument().Return(tc.identityDocument, tc.err)
			}

			m, err := NewMetadataService(mockEC2Metadata)
			if tc.isAvailable && tc.err == nil && !tc.isPartial {
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
			} else {
				if err == nil {
					t.Fatal("NewMetadataService() failed: expected error when GetInstanceIdentityDocument returns partial data, got nothing")
				}
			}

			mockCtrl.Finish()
		})
	}
}
