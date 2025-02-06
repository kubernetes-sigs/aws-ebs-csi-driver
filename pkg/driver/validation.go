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
	"strings"

	"github.com/kubernetes-sigs/aws-ebs-csi-driver/pkg/cloud"
	"k8s.io/klog/v2"
)

func ValidateDriverOptions(options *Options) error {
	if err := validateExtraTags(options.ExtraTags, false); err != nil {
		return fmt.Errorf("invalid extra tags: %w", err)
	}

	if err := validateMode(options.Mode); err != nil {
		return fmt.Errorf("invalid mode: %w", err)
	}

	if options.ModifyVolumeRequestHandlerTimeout == 0 && (options.Mode == ControllerMode || options.Mode == AllMode) {
		return errors.New("invalid modifyVolumeRequestHandlerTimeout: timeout cannot be zero")
	}

	return nil
}

func validateExtraTags(tags map[string]string, warnOnly bool) error {
	if len(tags) > cloud.MaxNumTagsPerResource {
		return fmt.Errorf("too many tags (actual: %d, limit: %d)", len(tags), cloud.MaxNumTagsPerResource)
	}

	validate := func(k, v string) error {
		if len(k) > cloud.MaxTagKeyLength {
			return fmt.Errorf("tag key too long (actual: %d, limit: %d)", len(k), cloud.MaxTagKeyLength)
		} else if len(k) < cloud.MinTagKeyLength {
			return errors.New("tag key cannot be empty (min: 1)")
		}
		if len(v) > cloud.MaxTagValueLength {
			return fmt.Errorf("tag value too long (actual: %d, limit: %d)", len(v), cloud.MaxTagValueLength)
		}
		if k == cloud.VolumeNameTagKey {
			return fmt.Errorf("tag key '%s' is reserved", cloud.VolumeNameTagKey)
		}
		if k == cloud.AwsEbsDriverTagKey {
			return fmt.Errorf("tag key '%s' is reserved", cloud.AwsEbsDriverTagKey)
		}
		if k == cloud.SnapshotNameTagKey {
			return fmt.Errorf("tag key '%s' is reserved", cloud.SnapshotNameTagKey)
		}
		if strings.HasPrefix(k, cloud.KubernetesTagKeyPrefix) {
			return fmt.Errorf("tag key prefix '%s' is reserved", cloud.KubernetesTagKeyPrefix)
		}
		if strings.HasPrefix(k, cloud.AWSTagKeyPrefix) {
			return fmt.Errorf("tag key prefix '%s' is reserved", cloud.AWSTagKeyPrefix)
		}
		return nil
	}

	for k, v := range tags {
		err := validate(k, v)
		if err != nil {
			if warnOnly {
				klog.InfoS("Skipping tag: the following key-value pair is not valid", "key", k, "value", v, "err", err)
			} else {
				return err
			}
		}
	}
	return nil
}

func validateMode(mode Mode) error {
	if mode != AllMode && mode != ControllerMode && mode != NodeMode {
		return fmt.Errorf("mode is not supported (actual: %s, supported: %v)", mode, []Mode{AllMode, ControllerMode, NodeMode})
	}

	return nil
}
