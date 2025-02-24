// Copyright 2025 The Kubernetes Authors.
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

package metrics

// constants for prometheus metrics use.
const (
	APIRequestDuration            = "aws_ebs_csi_api_request_duration_seconds"
	APIRequestErrors              = "aws_ebs_csi_api_request_errors_total"
	APIRequestThrottles           = "aws_ebs_csi_api_request_throttles_total"
	HelpText                      = "ebs_csi_aws_com metric"
	DeprecatedHelpText            = "cloudprovider_aws_api metric"
	DeprecatedAPIRequestDuration  = "cloudprovider_aws_api_request_duration_seconds"
	DeprecatedAPIRequestErrors    = "cloudprovider_aws_api_request_errors"
	DeprecatedAPIRequestThrottles = "cloudprovider_aws_api_throttled_requests_total"
)
