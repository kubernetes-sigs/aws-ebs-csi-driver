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

package cloud

import (
	"context"
	"errors"
	"time"

	awsmiddleware "github.com/aws/aws-sdk-go-v2/aws/middleware"
	"github.com/aws/aws-sdk-go-v2/aws/retry"
	"github.com/aws/smithy-go"
	"github.com/aws/smithy-go/middleware"
	"github.com/kubernetes-sigs/aws-ebs-csi-driver/pkg/metrics"
	"k8s.io/klog/v2"
)

// RecordRequestsHandler is added to the Complete chain; called after any request.
func RecordRequestsMiddleware(deprecatedMetrics bool) func(*middleware.Stack) error {
	return func(stack *middleware.Stack) error {
		return stack.Finalize.Add(middleware.FinalizeMiddlewareFunc("RecordRequestsMiddleware", func(ctx context.Context, input middleware.FinalizeInput, next middleware.FinalizeHandler) (output middleware.FinalizeOutput, metadata middleware.Metadata, err error) {
			start := time.Now()
			output, metadata, err = next.HandleFinalize(ctx, input)
			labels := createLabels(ctx)
			if err != nil {
				var apiErr smithy.APIError
				if errors.As(err, &apiErr) {
					if _, isThrottleError := retry.DefaultThrottleErrorCodes[apiErr.ErrorCode()]; isThrottleError {
						operationName := awsmiddleware.GetOperationName(ctx)
						labels = map[string]string{
							"operation_name": operationName,
						}
						metrics.Recorder().IncreaseCount("aws_ebs_csi_api_request_throttles_total", labels)
						if deprecatedMetrics {
							metrics.Recorder().IncreaseCount("cloudprovider_aws_api_throttled_requests_total", labels)
						}
					} else {
						metrics.Recorder().IncreaseCount("aws_ebs_csi_api_request_errors_total", labels)
						if deprecatedMetrics {
							metrics.Recorder().IncreaseCount("cloudprovider_aws_api_request_errors", labels)
						}
					}
				}
			} else {
				duration := time.Since(start).Seconds()
				metrics.Recorder().ObserveHistogram("aws_ebs_csi_api_request_duration_seconds", duration, labels, nil)
				if deprecatedMetrics {
					metrics.Recorder().ObserveHistogram("cloudprovider_aws_api_request_duration_seconds", duration, labels, nil)
				}
			}
			return output, metadata, err
		}), middleware.After)
	}
}

// LogServerErrorsMiddleware is a middleware that logs server errors received when attempting to contact the AWS API
// A specialized middleware is used instead of the SDK's built-in retry logging to allow for customizing the verbosity
// of throttle errors vs server/unknown errors, to prevent flooding the logs with throttle error.
func LogServerErrorsMiddleware() func(*middleware.Stack) error {
	return func(stack *middleware.Stack) error {
		return stack.Finalize.Add(middleware.FinalizeMiddlewareFunc("LogServerErrorsMiddleware", func(ctx context.Context, input middleware.FinalizeInput, next middleware.FinalizeHandler) (output middleware.FinalizeOutput, metadata middleware.Metadata, err error) {
			output, metadata, err = next.HandleFinalize(ctx, input)
			if err != nil {
				var apiErr smithy.APIError
				if errors.As(err, &apiErr) {
					if _, isThrottleError := retry.DefaultThrottleErrorCodes[apiErr.ErrorCode()]; isThrottleError {
						// Only log throttle errors under a high verbosity as we expect to see many of them
						// under normal bursty/high-TPS workloads
						klog.V(4).ErrorS(apiErr, "Throttle error from AWS API")
					} else {
						klog.ErrorS(apiErr, "Error from AWS API")
					}
				} else {
					klog.ErrorS(err, "Unknown error attempting to contact AWS API")
				}
			}
			return output, metadata, err
		}), middleware.After)
	}
}

func createLabels(ctx context.Context) map[string]string {
	operationName := awsmiddleware.GetOperationName(ctx)
	if operationName == "" {
		operationName = "Unknown"
	}
	return map[string]string{
		"request": operationName,
	}
}
