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

func TestNodeOptions(t *testing.T) {
	testCases := []struct {
		name  string
		flag  string
		found bool
	}{
		{
			name:  "lookup desired flag",
			flag:  "volume-attach-limit",
			found: true,
		},
		{
			name:  "fail for non-desired flag",
			flag:  "some-flag",
			found: false,
		},
	}

	for _, tc := range testCases {
		flagSet := flag.NewFlagSet("test-flagset", flag.ContinueOnError)
		nodeOptions := &NodeOptions{}

		t.Run(tc.name, func(t *testing.T) {
			nodeOptions.AddFlags(flagSet)

			flag := flagSet.Lookup(tc.flag)
			found := flag != nil
			if found != tc.found {
				t.Fatalf("result not equal\ngot:\n%v\nexpected:\n%v", found, tc.found)
			}
		})
	}
}
