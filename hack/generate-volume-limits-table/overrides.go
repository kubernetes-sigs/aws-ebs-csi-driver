// Copyright 2026 The Kubernetes Authors.
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

package main

// This file contains overrides used to generate the volume limits and multi-card tables.
// Modifications in this file directly change the table values, and CANNOT be detected as invalid
// by the detect-potentially-invalid-limits script. This file should only be used when an
// override at the pkg/cloud/limits level is impossible, such as when multiple regions return
// mixed results for a single instance type.

// IMPORTANT: We MUST re-validate the information in this file is correct EVERY release!

type instanceOverride struct {
	maxAttachments *int32
	maxEbsCards    *int32
}

// wrongLimitInstances contains instance types whose EBS limits reported by
// DescribeInstanceTypes are incorrect and must be overridden.
var wrongLimitInstances = map[string]instanceOverride{}

// skipRegions contains regions to skip entirely when collecting instance data.
var skipRegions = map[string]struct{}{
	// AWS is currently experiencing significant disruptions in these two regions.
	// This is causing DIT information to be out of date or the region to fail to
	// respond at all. Because these regions are largely inoperable, exclude them.
	"me-central-1": {},
	"me-south-1":   {},
}
