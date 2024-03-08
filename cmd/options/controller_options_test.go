/*
Copyright 2020 The Kubernetes Authors.

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

package options

import (
	"testing"

	flag "github.com/spf13/pflag"
)

func TestControllerOptions(t *testing.T) {
	testCases := []struct {
		name  string
		flag  string
		found bool
	}{
		{
			name:  "lookup desired flag",
			flag:  "extra-volume-tags",
			found: true,
		},
		{
			name:  "lookup k8s-tag-cluster-id",
			flag:  "k8s-tag-cluster-id",
			found: true,
		},
		{
			name:  "lookup aws-sdk-debug-log",
			flag:  "aws-sdk-debug-log",
			found: true,
		},
		{
			name:  "lookup batching",
			flag:  "batching",
			found: true,
		},
		{
			name:  "lookup user-agent-extra",
			flag:  "user-agent-extra",
			found: true,
		},
		{
			name:  "lookup modify-volume-request-handler-timeout",
			flag:  "modify-volume-request-handler-timeout",
			found: true,
		},
		{
			name:  "fail for non-desired flag",
			flag:  "some-other-flag",
			found: false,
		},
	}

	for _, tc := range testCases {
		flagSet := flag.NewFlagSet("test-flagset", flag.ContinueOnError)
		controllerOptions := &ControllerOptions{}

		t.Run(tc.name, func(t *testing.T) {
			controllerOptions.AddFlags(flagSet)

			flag := flagSet.Lookup(tc.flag)
			found := flag != nil
			if found != tc.found {
				t.Fatalf("result not equal\ngot:\n%v\nexpected:\n%v", found, tc.found)
			}
		})
	}
}
