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

package driver

import (
	"errors"
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/kubernetes-sigs/aws-ebs-csi-driver/pkg/cloud"
)

func TestValidateExtraTags(t *testing.T) {
	testCases := []struct {
		name   string
		tags   map[string]string
		expErr error
	}{
		{
			name: "valid tags",
			tags: map[string]string{
				"extra-tag-key": "extra-tag-value",
			},
			expErr: nil,
		},
		{
			name: "invalid tag: reserved CSI key",
			tags: map[string]string{
				cloud.VolumeNameTagKey: "extra-tag-value",
			},
			expErr: fmt.Errorf("tag key '%s' is reserved", cloud.VolumeNameTagKey),
		},
		{
			name: "invalid tag: reserved driver key",
			tags: map[string]string{
				cloud.AwsEbsDriverTagKey: "false",
			},
			expErr: fmt.Errorf("tag key '%s' is reserved", cloud.AwsEbsDriverTagKey),
		},
		{
			name: "invaid tag: reserved snapshot key",
			tags: map[string]string{
				cloud.SnapshotNameTagKey: "false",
			},
			expErr: fmt.Errorf("tag key '%s' is reserved", cloud.SnapshotNameTagKey),
		},
		{
			name: "invalid tag: reserved Kubernetes key prefix",
			tags: map[string]string{
				cloud.KubernetesTagKeyPrefix + "/cluster": "extra-tag-value",
			},
			expErr: fmt.Errorf("tag key prefix '%s' is reserved", cloud.KubernetesTagKeyPrefix),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateExtraTags(tc.tags, false)
			if !reflect.DeepEqual(err, tc.expErr) {
				t.Fatalf("error not equal\ngot:\n%s\nexpected:\n%s", err, tc.expErr)
			}
		})
	}
}

func TestValidateMode(t *testing.T) {
	testCases := []struct {
		name   string
		mode   Mode
		expErr error
	}{
		{
			name:   "valid mode: all",
			mode:   AllMode,
			expErr: nil,
		},
		{
			name:   "valid mode: controller",
			mode:   ControllerMode,
			expErr: nil,
		},
		{
			name:   "valid mode: node",
			mode:   NodeMode,
			expErr: nil,
		},
		{
			name:   "invalid mode: unknown",
			mode:   Mode("unknown"),
			expErr: fmt.Errorf("mode is not supported (actual: unknown, supported: %v)", []Mode{AllMode, ControllerMode, NodeMode}),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateMode(tc.mode)
			if !reflect.DeepEqual(err, tc.expErr) {
				t.Fatalf("error not equal\ngot:\n%s\nexpected:\n%s", err, tc.expErr)
			}
		})
	}
}

func TestValidateDriverOptions(t *testing.T) {
	testCases := []struct {
		name                string
		mode                Mode
		extraVolumeTags     map[string]string
		modifyVolumeTimeout time.Duration
		expErr              error
	}{
		{
			name:                "success",
			mode:                AllMode,
			modifyVolumeTimeout: 5 * time.Second,
			expErr:              nil,
		},
		{
			name:                "fail because validateMode fails",
			mode:                Mode("unknown"),
			modifyVolumeTimeout: 5 * time.Second,
			expErr:              fmt.Errorf("invalid mode: %w", fmt.Errorf("mode is not supported (actual: unknown, supported: %v)", []Mode{AllMode, ControllerMode, NodeMode})),
		},
		{
			name: "fail because validateExtraVolumeTags fails",
			mode: AllMode,
			extraVolumeTags: map[string]string{
				cloud.AwsEbsDriverTagKey: "extra-tag-value",
			},
			modifyVolumeTimeout: 5 * time.Second,
			expErr:              fmt.Errorf("invalid extra tags: %w", fmt.Errorf("tag key '%s' is reserved", cloud.AwsEbsDriverTagKey)),
		},
		{
			name:                "fail because modifyVolumeRequestHandlerTimeout is zero",
			mode:                AllMode,
			modifyVolumeTimeout: 0,
			expErr:              errors.New("invalid modifyVolumeRequestHandlerTimeout: timeout cannot be zero"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateDriverOptions(&Options{
				ExtraTags:                         tc.extraVolumeTags,
				Mode:                              tc.mode,
				ModifyVolumeRequestHandlerTimeout: tc.modifyVolumeTimeout,
			})
			if !reflect.DeepEqual(err, tc.expErr) {
				t.Fatalf("error not equal\ngot:\n%s\nexpected:\n%s", err, tc.expErr)
			}
		})
	}
}
