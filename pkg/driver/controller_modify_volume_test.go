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

package driver

import (
	"testing"

	"github.com/kubernetes-sigs/aws-ebs-csi-driver/pkg/cloud"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	validType                   = "gp3"
	validIops                   = "2000"
	validIopsInt                = 2000
	validThroughput             = "500"
	validThroughputInt          = 500
	invalidIops                 = "123.546"
	invalidThroughput           = "one hundred"
	validTagSpecificationInput  = "key1=tag1"
	tagSpecificationWithNoValue = "key3="
	tagSpecificationWithNoEqual = "key1"
	validTagDeletion            = "key2"
	invalidTagSpecification     = "aws:test=TEST"
	invalidParameter            = "invalid_parameter"
)

func TestMergeModifyVolumeRequest(t *testing.T) {
	testCases := []struct {
		name                        string
		input                       modifyVolumeRequest
		existing                    modifyVolumeRequest
		expectedModifyVolumeRequest modifyVolumeRequest
		expectError                 bool
	}{
		{
			name: "Valid merge of size and iops",
			input: modifyVolumeRequest{
				newSize: 5,
			},
			existing: modifyVolumeRequest{
				modifyDiskOptions: cloud.ModifyDiskOptions{
					IOPS: validIopsInt,
				},
			},
			expectedModifyVolumeRequest: modifyVolumeRequest{
				newSize: 5,
				modifyDiskOptions: cloud.ModifyDiskOptions{
					IOPS: validIopsInt,
				},
			},
			expectError: false,
		},
		{
			name: "Different size requested by a previous request",
			input: modifyVolumeRequest{
				newSize: 4,
			},
			existing: modifyVolumeRequest{
				newSize: 5,
			},
			expectedModifyVolumeRequest: modifyVolumeRequest{
				newSize: 5,
			},
			expectError: true,
		},
		{
			name: "Different IOPS requested by previous request",
			input: modifyVolumeRequest{
				modifyDiskOptions: cloud.ModifyDiskOptions{
					IOPS: validIopsInt,
				},
			},
			existing: modifyVolumeRequest{
				modifyDiskOptions: cloud.ModifyDiskOptions{
					IOPS: validIopsInt - 1,
				},
			},
			expectedModifyVolumeRequest: modifyVolumeRequest{
				modifyDiskOptions: cloud.ModifyDiskOptions{
					IOPS: validIopsInt - 1,
				},
			},
			expectError: true,
		},
		{
			name: "Different Throughput requested by previous request",
			input: modifyVolumeRequest{
				modifyDiskOptions: cloud.ModifyDiskOptions{
					Throughput: validThroughputInt,
				},
			},
			existing: modifyVolumeRequest{
				modifyDiskOptions: cloud.ModifyDiskOptions{
					Throughput: validThroughputInt - 1,
				},
			},
			expectedModifyVolumeRequest: modifyVolumeRequest{
				modifyDiskOptions: cloud.ModifyDiskOptions{
					Throughput: validThroughputInt - 1,
				},
			},
			expectError: true,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := mergeModifyVolumeRequest(tc.input, tc.existing)
			assert.Equal(t, tc.expectedModifyVolumeRequest, result)
			if tc.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestParseModifyVolumeParameters(t *testing.T) {
	testCases := []struct {
		name            string
		params          map[string]string
		expectedOptions *modifyVolumeRequest
		expectError     bool
	}{
		{
			name:   "blank params",
			params: map[string]string{},
			expectedOptions: &modifyVolumeRequest{
				modifyTagsOptions: cloud.ModifyTagsOptions{
					TagsToAdd:    map[string]string{},
					TagsToDelete: []string{},
				},
			},
		},
		{
			name: "basic params",
			params: map[string]string{
				ModificationKeyVolumeType: validType,
				ModificationKeyIOPS:       validIops,
				ModificationKeyThroughput: validThroughput,
				ModificationAddTag + "_1": validTagSpecificationInput,
				ModificationAddTag + "_2": "key2={{ .PVCName }}",
				ModificationAddTag + "_3": "key3={{ .PVCNamespace }}",
				ModificationAddTag + "_4": "key4={{ .PVName }}",
				ModificationDeleteTag:     validTagDeletion,
				PVCNameKey:                "ebs-claim",
				PVCNamespaceKey:           "test-namespace",
				PVNameKey:                 "testPV-Name",
			},
			expectedOptions: &modifyVolumeRequest{
				modifyDiskOptions: cloud.ModifyDiskOptions{
					VolumeType: validType,
					IOPS:       validIopsInt,
					Throughput: validThroughputInt,
				},
				modifyTagsOptions: cloud.ModifyTagsOptions{
					TagsToAdd: map[string]string{
						"key1": "tag1",
						"key2": "ebs-claim",
						"key3": "test-namespace",
						"key4": "testPV-Name",
					},
					TagsToDelete: []string{
						"key2",
					},
				},
			},
		},
		{
			name: "tag specification with inproper length",
			params: map[string]string{
				ModificationAddTag: tagSpecificationWithNoEqual,
			},
			expectError: true,
		},
		{
			name: "deprecated type but has validType",
			params: map[string]string{
				ModificationKeyVolumeType:           validType,
				DeprecatedModificationKeyVolumeType: "deprecated" + validType,
			},
			expectedOptions: &modifyVolumeRequest{
				modifyDiskOptions: cloud.ModifyDiskOptions{
					VolumeType: validType,
				},
				modifyTagsOptions: cloud.ModifyTagsOptions{
					TagsToAdd:    map[string]string{},
					TagsToDelete: []string{},
				},
			},
		},
		{
			name: "invalid iops",
			params: map[string]string{
				ModificationKeyIOPS: invalidIops,
			},
			expectError: true,
		},
		{
			name: "invalid throughput",
			params: map[string]string{
				ModificationKeyThroughput: invalidThroughput,
			},
			expectError: true,
		},
		{
			name: "invalid tag specification",
			params: map[string]string{
				ModificationAddTag: invalidTagSpecification,
			},
			expectError: true,
		},
		{
			name: "invalid VAC parameter",
			params: map[string]string{
				invalidParameter: "20",
			},
			expectError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := parseModifyVolumeParameters(tc.params)
			assert.Equal(t, tc.expectedOptions, result)
			if tc.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
