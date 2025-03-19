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
	APIRequestDuration                    = "aws_ebs_csi_api_request_duration_seconds"
	APIRequestErrors                      = "aws_ebs_csi_api_request_errors_total"
	APIRequestThrottles                   = "aws_ebs_csi_api_request_throttles_total"
	APIRequestDurationHelpText            = "AWS SDK API request duration by request type in seconds"
	APIRequestErrorsHelpText              = "Total number of AWS SDK API errors by error code and request type"
	APIRequestThrottlesHelpText           = "Total number of throttled AWS SDK API requests per request type"
	DeprecatedAPIRequestDurationHelpText  = APIRequestDurationHelpText + " (deprecated)"
	DeprecatedAPIRequestErrorsHelpText    = APIRequestErrorsHelpText + " (deprecated)"
	DeprecatedAPIRequestThrottlesHelpText = APIRequestThrottlesHelpText + " (deprecated)"
	DeprecatedAPIRequestDuration          = "cloudprovider_aws_api_request_duration_seconds"
	DeprecatedAPIRequestErrors            = "cloudprovider_aws_api_request_errors"
	DeprecatedAPIRequestThrottles         = "cloudprovider_aws_api_throttled_requests_total"
)
