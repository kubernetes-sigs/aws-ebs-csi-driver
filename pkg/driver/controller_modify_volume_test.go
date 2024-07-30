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
	validType          = "gp3"
	validIops          = "2000"
	validIopsInt       = 2000
	validThroughput    = "500"
	validThroughputInt = 500
	invalidIops        = "123.546"
	invalidThroughput  = "one hundred"
)

func TestParseModifyVolumeParameters(t *testing.T) {
	testCases := []struct {
		name            string
		params          map[string]string
		expectedOptions *cloud.ModifyDiskOptions
		expectError     bool
	}{
		{
			name:            "blank params",
			params:          map[string]string{},
			expectedOptions: &cloud.ModifyDiskOptions{},
		},
		{
			name: "basic params",
			params: map[string]string{
				ModificationKeyVolumeType: validType,
				ModificationKeyIOPS:       validIops,
				ModificationKeyThroughput: validThroughput,
			},
			expectedOptions: &cloud.ModifyDiskOptions{
				VolumeType: validType,
				IOPS:       validIopsInt,
				Throughput: validThroughputInt,
			},
		},
		{
			name: "deprecated type",
			params: map[string]string{
				ModificationKeyVolumeType:           validType,
				DeprecatedModificationKeyVolumeType: "deprecated" + validType,
			},
			expectedOptions: &cloud.ModifyDiskOptions{
				VolumeType: validType,
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
