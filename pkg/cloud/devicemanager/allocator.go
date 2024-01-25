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
	"fmt"
)

// ExistingNames is a map of assigned device names. Presence of a key with a device
// name in the map means that the device is allocated. Value is irrelevant and
// can be used for anything that NameAllocator user wants.
type ExistingNames map[string]string

// On AWS, we should assign new (not yet used) device names to attached volumes.
// If we reuse a previously used name, we may get the volume "attaching" forever,
// see https://aws.amazon.com/premiumsupport/knowledge-center/ebs-stuck-attaching/.
// NameAllocator finds available device name, taking into account already
// assigned device names from ExistingNames map. It tries to find the next
// device name to the previously assigned one (from previous NameAllocator
// call), so all available device names are used eventually and it minimizes
// device name reuse.
type NameAllocator interface {
	GetNext(existingNames ExistingNames) (name string, err error)
}

type nameAllocator struct{}

var _ NameAllocator = &nameAllocator{}

// GetNext returns a free device name or error when there is no free device name
// It does this by using a list of legal EBS device names from device_names.go
func (d *nameAllocator) GetNext(existingNames ExistingNames) (string, error) {
	for _, name := range deviceNames {
		if _, found := existingNames[name]; !found {
			return name, nil
		}
	}

	return "", fmt.Errorf("there are no names available")
}
