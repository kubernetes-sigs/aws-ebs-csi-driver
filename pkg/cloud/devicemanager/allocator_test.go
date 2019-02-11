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
	tests := []struct {
		name           string
		existingNames  ExistingNames
		deviceMap      map[string]int
		expectedOutput string
	}{
		{
			"empty device list with wrap",
			ExistingNames{},
			generateUnsortedNameList(),
			"bd", // next to 'cz' is the first one, 'ba'
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			allocator := NewNameAllocator().(*nameAllocator)
			for k, v := range test.deviceMap {
				allocator.possibleNames[k] = v
			}

			got, err := allocator.GetNext(test.existingNames)
			if err != nil {
				t.Errorf("text %q: unexpected error: %v", test.name, err)
			}
			if got != test.expectedOutput {
				t.Errorf("text %q: expected %q, got %q", test.name, test.expectedOutput, got)
			}
		})
	}
}

func generateUnsortedNameList() map[string]int {
	possibleNames := make(map[string]int)
	for _, firstChar := range []rune{'b', 'c'} {
		for i := 'a'; i <= 'z'; i++ {
			dev := string([]rune{firstChar, i})
			possibleNames[dev] = 3
		}
	}
	possibleNames["bd"] = 0
	return possibleNames
}

func TestNameAllocatorError(t *testing.T) {
	allocator := NewNameAllocator().(*nameAllocator)
	existingNames := ExistingNames{}

	// make all devices used
	var first, second byte
	for first = 'b'; first <= 'c'; first++ {
		for second = 'a'; second <= 'z'; second++ {
			device := [2]byte{first, second}
			existingNames[string(device[:])] = "used"
		}
	}

	device, err := allocator.GetNext(existingNames)
	if err == nil {
		t.Errorf("expected error, got device  %q", device)
	}
}
