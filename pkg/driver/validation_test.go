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
	"fmt"
	"math/rand"
	"reflect"
	"strconv"
	"testing"

	"github.com/kubernetes-sigs/aws-ebs-csi-driver/pkg/cloud"
)

func randomString(n int) string {
	var letter = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789")

	b := make([]rune, n)
	for i := range b {
		b[i] = letter[rand.Intn(len(letter))]
	}
	return string(b)
}

func randomStringMap(n int) map[string]string {
	result := map[string]string{}
	for i := 0; i < n; i++ {
		result[strconv.Itoa(i)] = randomString(10)
	}
	return result
}

func TestValidateExtraVolumeTags(t *testing.T) {
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
			name: "invalid tag: key too long",
			tags: map[string]string{
				randomString(cloud.MaxTagKeyLength + 1): "extra-tag-value",
			},
			expErr: fmt.Errorf("Volume tag key too long (actual: %d, limit: %d)", cloud.MaxTagKeyLength+1, cloud.MaxTagKeyLength),
		},
		{
			name: "invalid tag: value too long",
			tags: map[string]string{
				"extra-tag-key": randomString(cloud.MaxTagValueLength + 1),
			},
			expErr: fmt.Errorf("Volume tag value too long (actual: %d, limit: %d)", cloud.MaxTagValueLength+1, cloud.MaxTagValueLength),
		},
		{
			name: "invalid tag: reserved CSI key",
			tags: map[string]string{
				cloud.VolumeNameTagKey: "extra-tag-value",
			},
			expErr: fmt.Errorf("Volume tag key '%s' is reserved", cloud.VolumeNameTagKey),
		},
		{
			name: "invalid tag: reserved Kubernetes key prefix",
			tags: map[string]string{
				cloud.KubernetesTagKeyPrefix + "/cluster": "extra-tag-value",
			},
			expErr: fmt.Errorf("Volume tag key prefix '%s' is reserved", cloud.KubernetesTagKeyPrefix),
		},
		{
			name: "invalid tag: reserved AWS key prefix",
			tags: map[string]string{
				cloud.AWSTagKeyPrefix + "foo": "extra-tag-value",
			},
			expErr: fmt.Errorf("Volume tag key prefix '%s' is reserved", cloud.AWSTagKeyPrefix),
		},
		{
			name:   "invalid tag: too many volume tags",
			tags:   randomStringMap(cloud.MaxNumTagsPerResource + 1),
			expErr: fmt.Errorf("Too many volume tags (actual: %d, limit: %d)", cloud.MaxNumTagsPerResource+1, cloud.MaxNumTagsPerResource),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateExtraVolumeTags(tc.tags)
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
			expErr: fmt.Errorf("Mode is not supported (actual: unknown, supported: %v)", []Mode{AllMode, ControllerMode, NodeMode}),
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
		name            string
		mode            Mode
		extraVolumeTags map[string]string
		expErr          error
	}{
		{
			name:   "success",
			mode:   AllMode,
			expErr: nil,
		},
		{
			name:   "fail because validateMode fails",
			mode:   Mode("unknown"),
			expErr: fmt.Errorf("Invalid mode: Mode is not supported (actual: unknown, supported: %v)", []Mode{AllMode, ControllerMode, NodeMode}),
		},
		{
			name: "fail because validateExtraVolumeTags fails",
			mode: AllMode,
			extraVolumeTags: map[string]string{
				randomString(cloud.MaxTagKeyLength + 1): "extra-tag-value",
			},
			expErr: fmt.Errorf("Invalid extra volume tags: Volume tag key too long (actual: %d, limit: %d)", cloud.MaxTagKeyLength+1, cloud.MaxTagKeyLength),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateDriverOptions(&DriverOptions{
				extraVolumeTags: tc.extraVolumeTags,
				mode:            tc.mode,
			})
			if !reflect.DeepEqual(err, tc.expErr) {
				t.Fatalf("error not equal\ngot:\n%s\nexpected:\n%s", err, tc.expErr)
			}
		})
	}
}
