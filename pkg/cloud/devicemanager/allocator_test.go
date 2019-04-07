/*
Copyright 2018 The Kubernetes Authors.

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

package devicemanager

import (
	"testing"
)

func TestNameAllocator(t *testing.T) {
	existingNames := map[string]string{}
	allocator := nameAllocator{}

	tests := []struct {
		expectedName string
	}{
		{"a"}, {"b"}, {"c"}, {"d"}, {"e"}, {"f"}, {"g"}, {"h"}, {"i"}, {"j"},
		{"k"}, {"l"}, {"m"}, {"n"}, {"o"}, {"p"}, {"q"}, {"r"}, {"s"}, {"t"},
		{"u"}, {"v"}, {"w"}, {"x"}, {"y"}, {"z"},
		{"aa"}, {"ab"}, {"ac"}, {"ad"}, {"ae"}, {"af"}, {"ag"}, {"ah"}, {"ai"}, {"aj"},
		{"ak"}, {"al"}, {"am"}, {"an"}, {"ao"}, {"ap"}, {"aq"}, {"ar"}, {"as"}, {"at"},
		{"au"}, {"av"}, {"aw"}, {"ax"}, {"ay"}, {"az"},
	}

	for _, test := range tests {
		t.Run(test.expectedName, func(t *testing.T) {
			actual, err := allocator.GetNext(existingNames)
			if err != nil {
				t.Errorf("test %q: unexpected error: %v", test.expectedName, err)
			}
			if actual != test.expectedName {
				t.Errorf("test %q: expected %q, got %q", test.expectedName, test.expectedName, actual)
			}
			existingNames[actual] = ""
		})
	}
}

func TestNameAllocatorError(t *testing.T) {
	allocator := nameAllocator{}
	existingNames := map[string]string{}

	for i := 0; i < 52; i++ {
		name, _ := allocator.GetNext(existingNames)
		existingNames[name] = ""
	}
	name, err := allocator.GetNext(existingNames)
	if err == nil {
		t.Errorf("expected error, got device  %q", name)
	}
}
