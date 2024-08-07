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

package driver

import (
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/kubernetes-sigs/aws-ebs-csi-driver/pkg/cloud"
	"github.com/kubernetes-sigs/aws-ebs-csi-driver/pkg/cloud/metadata"
	"github.com/kubernetes-sigs/aws-ebs-csi-driver/pkg/mounter"
	"github.com/stretchr/testify/require"
)

func TestNewDriver(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockCloud := cloud.NewMockCloud(ctrl)
	mockMetadataService := metadata.NewMockMetadataService(ctrl)
	mockMounter := mounter.NewMockMounter(ctrl)
	mockKubernetesClient := NewMockKubernetesClient(ctrl)
	testCases := []struct {
		name          string
		o             *Options
		expectError   bool
		hasController bool
		hasNode       bool
	}{
		{
			name: "Valid driver controllerMode",
			o: &Options{
				Mode:                              ControllerMode,
				ModifyVolumeRequestHandlerTimeout: 1,
			},
			expectError:   false,
			hasController: true,
			hasNode:       false,
		},
		{
			name: "Valid driver nodeMode",
			o: &Options{
				Mode: NodeMode,
			},
			expectError:   false,
			hasController: false,
			hasNode:       true,
		},
		{
			name: "Valid driver allMode",
			o: &Options{
				Mode:                              AllMode,
				ModifyVolumeRequestHandlerTimeout: 1,
			},
			expectError:   false,
			hasController: true,
			hasNode:       true,
		},
		{
			name: "Invalid driver options",
			o: &Options{
				Mode: "InvalidMode",
			},
			expectError:   true,
			hasController: false,
			hasNode:       false,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			driver, err := NewDriver(mockCloud, tc.o, mockMounter, mockMetadataService, mockKubernetesClient)
			if tc.hasNode && driver.node == nil {
				t.Fatalf("Expected driver to have node but driver does not have node")
			}
			if tc.hasController && driver.controller == nil {
				t.Fatalf("Expected driver to have controller but driver does not have controller")
			}
			if tc.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
