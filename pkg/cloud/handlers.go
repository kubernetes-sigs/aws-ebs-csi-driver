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
	"github.com/aws/smithy-go"
	"github.com/aws/smithy-go/middleware"
	"github.com/kubernetes-sigs/aws-ebs-csi-driver/pkg/metrics"
	"k8s.io/klog/v2"
)

const requestLimitExceededErrorCode = "RequestLimitExceeded"

// RecordRequestsHandler is added to the Complete chain; called after any request
func RecordRequestsMiddleware() func(*middleware.Stack) error {
	return func(stack *middleware.Stack) error {
		return stack.Finalize.Add(middleware.FinalizeMiddlewareFunc("RecordRequestsMiddleware", func(ctx context.Context, input middleware.FinalizeInput, next middleware.FinalizeHandler) (output middleware.FinalizeOutput, metadata middleware.Metadata, err error) {
			start := time.Now()
			output, metadata, err = next.HandleFinalize(ctx, input)
			labels := createLabels(ctx)
			if err != nil {
				var apiErr smithy.APIError
				if errors.As(err, &apiErr) {
					if apiErr.ErrorCode() == requestLimitExceededErrorCode {
						operationName := awsmiddleware.GetOperationName(ctx)
						labels = map[string]string{
							"operation_name": operationName,
						}
						metrics.Recorder().IncreaseCount("cloudprovider_aws_api_throttled_requests_total", labels)
						klog.InfoS("Got RequestLimitExceeded error on AWS request", "request", operationName)
					} else {
						metrics.Recorder().IncreaseCount("cloudprovider_aws_api_request_errors", labels)
					}
				}
			} else {
				duration := time.Since(start).Seconds()
				metrics.Recorder().ObserveHistogram("cloudprovider_aws_api_request_duration_seconds", duration, labels, nil)
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
