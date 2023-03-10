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
		{"aa"}, {"ab"}, {"ac"}, {"ad"}, {"ae"}, {"af"}, {"ag"}, {"ah"}, {"ai"}, {"aj"},
		{"ak"}, {"al"}, {"am"}, {"an"}, {"ao"}, {"ap"}, {"aq"}, {"ar"}, {"as"}, {"at"},
		{"au"}, {"av"}, {"aw"}, {"ax"}, {"ay"}, {"az"},
		{"ba"}, {"bb"}, {"bc"}, {"bd"}, {"be"}, {"bf"}, {"bg"}, {"bh"}, {"bi"}, {"bj"},
		{"bk"}, {"bl"}, {"bm"}, {"bn"}, {"bo"}, {"bp"}, {"bq"}, {"br"}, {"bs"}, {"bt"},
		{"bu"}, {"bv"}, {"bw"}, {"bx"}, {"by"}, {"bz"},
		{"ca"}, {"cb"}, {"cc"}, {"cd"}, {"ce"}, {"cf"}, {"cg"}, {"ch"}, {"ci"}, {"cj"},
		{"ck"}, {"cl"}, {"cm"}, {"cn"}, {"co"}, {"cp"}, {"cq"}, {"cr"}, {"cs"}, {"ct"},
		{"cu"}, {"cv"}, {"cw"}, {"cx"}, {"cy"}, {"cz"},
		{"da"}, {"db"}, {"dc"}, {"dd"}, {"de"}, {"df"}, {"dg"}, {"dh"}, {"di"}, {"dj"},
		{"dk"}, {"dl"}, {"dm"}, {"dn"}, {"do"}, {"dp"}, {"dq"}, {"dr"}, {"ds"}, {"dt"},
		{"du"}, {"dv"}, {"dw"}, {"dx"},
	}

	for _, test := range tests {
		t.Run(test.expectedName, func(t *testing.T) {
			actual, err := allocator.GetNext(existingNames, "")
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

	// 102 == number of allocations from aa ... dx (see allocator.go for why we stop at dx)
	for i := 0; i < 102; i++ {
		name, _ := allocator.GetNext(existingNames, "/dev/xvd")
		existingNames[name] = ""
	}
	name, err := allocator.GetNext(existingNames, "/dev/xvd")
	if err == nil {
		t.Errorf("expected error, got device  %q", name)
	}
}
