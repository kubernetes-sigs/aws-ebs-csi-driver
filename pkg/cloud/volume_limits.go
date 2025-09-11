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

package cloud

import "github.com/kubernetes-sigs/aws-ebs-csi-driver/pkg/util"

// GetVolumeLimits returns the volume limit and attachment type for a given instance type.
// Returns (limit, attachmentType) where limit is the maximum number of volumes
// and attachmentType is either "shared" or "dedicated".
func GetVolumeLimits(instanceType string) (int, string) {
	// Check non-nitro instances first (limit of 39)
	// The API calls these shared, but we treat them as dedicated
	if _, exists := nonNitroInstanceTypes[instanceType]; exists {
		return 39, util.AttachmentDedicated
	}

	// Check volume limits table
	if limit, exists := volumeLimits[instanceType]; exists {
		// These two instance types have the wrong type in the API, hardcode them as dedicated
		if instanceType == "c8gn.metal-24xl" || instanceType == "c8gn.metal-48xl" {
			limit.attachmentType = util.AttachmentDedicated
		}
		return limit.maxAttachments, limit.attachmentType
	}

	// Default to shared limit of 28
	return 28, util.AttachmentShared
}
