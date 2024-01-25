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

package cloud

import (
	"time"

	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/kubernetes-sigs/aws-ebs-csi-driver/pkg/metrics"
	"k8s.io/klog/v2"
)

// RecordRequestsHandler is added to the Complete chain; called after any request
func RecordRequestsHandler(r *request.Request) {
	labels := map[string]string{
		"request": operationName(r),
	}

	if r.Error != nil {
		metrics.Recorder().IncreaseCount("cloudprovider_aws_api_request_errors", labels)
	} else {
		duration := time.Since(r.Time).Seconds()
		metrics.Recorder().ObserveHistogram("cloudprovider_aws_api_request_duration_seconds", duration, labels, nil)
	}
}

// RecordThrottledRequestsHandler is added to the AfterRetry chain; called after any error
func RecordThrottledRequestsHandler(r *request.Request) {
	labels := map[string]string{
		"operation_name": operationName(r),
	}

	if r.IsErrorThrottle() {
		metrics.Recorder().IncreaseCount("cloudprovider_aws_api_throttled_requests_total", labels)
		klog.InfoS("Got RequestLimitExceeded error on AWS request", "request", describeRequest(r))
	}
}

// Return the operation name, for use in log messages and metrics
func operationName(r *request.Request) string {
	name := "N/A"
	if r.Operation != nil {
		name = r.Operation.Name
	}
	return name
}

// Return a user-friendly string describing the request, for use in log messages
func describeRequest(r *request.Request) string {
	service := r.ClientInfo.ServiceName
	return service + "::" + operationName(r)
}
