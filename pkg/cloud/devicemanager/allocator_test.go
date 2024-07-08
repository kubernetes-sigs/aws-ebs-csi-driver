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
	"sync"
	"testing"
)

func TestNameAllocator(t *testing.T) {
	existingNames := map[string]string{}
	allocator := nameAllocator{}

	for _, name := range deviceNames {
		t.Run(name, func(t *testing.T) {
			actual, err := allocator.GetNext(existingNames, new(sync.Map))
			if err != nil {
				t.Errorf("test %q: unexpected error: %v", name, err)
			}
			if actual != name {
				t.Errorf("test %q: expected %q, got %q", name, name, actual)
			}
			existingNames[actual] = ""
		})
	}
}

func TestNameAllocatorLikelyBadName(t *testing.T) {
	skippedNameExisting := deviceNames[11]
	skippedNameNew := deviceNames[32]
	likelyBadNames := new(sync.Map)
	likelyBadNames.Store(skippedNameExisting, struct{}{})
	likelyBadNames.Store(skippedNameNew, struct{}{})
	existingNames := map[string]string{
		skippedNameExisting: "",
	}
	allocator := nameAllocator{}

	for _, name := range deviceNames {
		if name == skippedNameExisting || name == skippedNameNew {
			// Names in likelyBadNames should be skipped until it is the last available name
			continue
		}

		t.Run(name, func(t *testing.T) {
			actual, err := allocator.GetNext(existingNames, likelyBadNames)
			if err != nil {
				t.Errorf("test %q: unexpected error: %v", name, err)
			}
			if actual != name {
				t.Errorf("test %q: expected %q, got %q", name, name, actual)
			}
			existingNames[actual] = ""
		})
	}

	onlyExisting := new(sync.Map)
	onlyExisting.Store(skippedNameExisting, struct{}{})
	_, err := allocator.GetNext(existingNames, onlyExisting)
	if err != nil {
		t.Errorf("got nil when error expected (likelyBadNames with only existing names)")
	}

	lastName, _ := allocator.GetNext(existingNames, likelyBadNames)
	if lastName != skippedNameNew {
		t.Errorf("test %q: expected %q, got %q (likelyBadNames fallback)", skippedNameNew, skippedNameNew, lastName)
	}
}

func TestNameAllocatorError(t *testing.T) {
	allocator := nameAllocator{}
	existingNames := map[string]string{}

	for i := 0; i < len(deviceNames); i++ {
		name, _ := allocator.GetNext(existingNames, new(sync.Map))
		existingNames[name] = ""
	}
	name, err := allocator.GetNext(existingNames, new(sync.Map))
	if err == nil {
		t.Errorf("expected error, got device  %q", name)
	}
}
