/*
Copyright 2016 The Kubernetes Authors.

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
	"sort"
	"sync"
)

// ExistingNames is a map of assigned device names. Presence of a key with a device
// name in the map means that the device is allocated. Value is irrelevant and
// can be used for anything that NameAllocator user wants.  Only the relevant
// part of device name should be in the map, e.g. "ba" for "/dev/xvdba".
type ExistingNames map[string]string

// On AWS, we should assign new (not yet used) device names to attached volumes.
// If we reuse a previously used name, we may get the volume "attaching" forever,
// see https://aws.amazon.com/premiumsupport/knowledge-center/ebs-stuck-attaching/.
// NameAllocator finds available device name, taking into account already
// assigned device names from ExistingNames map. It tries to find the next
// device name to the previously assigned one (from previous NameAllocator
// call), so all available device names are used eventually and it minimizes
// device name reuse.
// All these allocations are in-memory, nothing is written to / read from
// /dev directory.
type NameAllocator interface {
	// GetNext returns a free device name or error when there is no free device
	// name. Only the device name is returned, e.g. "ba" for "/dev/xvdba".
	// It's up to the called to add appropriate "/dev/sd" or "/dev/xvd" prefix.
	GetNext(existingNames ExistingNames) (name string, err error)

	// Deprioritize the device name so as it can't be used immediately again
	Deprioritize(choosen string)
}

type nameAllocator struct {
	possibleNames map[string]int
	counter       int
	mux           sync.Mutex
}

var _ NameAllocator = &nameAllocator{}

type namePair struct {
	name  string
	index int
}

type namePairList []namePair

func (p namePairList) Len() int           { return len(p) }
func (p namePairList) Less(i, j int) bool { return p[i].index < p[j].index }
func (p namePairList) Swap(i, j int)      { p[i], p[j] = p[j], p[i] }

// Allocates device names according to scheme ba..bz, ca..cz
// it moves along the ring and always picks next device until
// device list is exhausted.
func NewNameAllocator() NameAllocator {
	possibleNames := make(map[string]int)
	for _, firstChar := range []rune{'b', 'c'} {
		for i := 'a'; i <= 'z'; i++ {
			name := string([]rune{firstChar, i})
			possibleNames[name] = 0
		}
	}
	return &nameAllocator{
		possibleNames: possibleNames,
		counter:       0,
	}
}

// GetNext gets next available device from the pool, this function assumes that caller
// holds the necessary lock on nameAllocator
func (d *nameAllocator) GetNext(existingNames ExistingNames) (string, error) {
	d.mux.Lock()
	defer d.mux.Unlock()

	for _, namePair := range d.sortByCount() {
		if _, found := existingNames[namePair.name]; !found {
			return namePair.name, nil
		}
	}
	return "", fmt.Errorf("there are no names available")
}

// Deprioritize the name so as it can't be used immediately again
func (d *nameAllocator) Deprioritize(chosen string) {
	d.mux.Lock()
	defer d.mux.Unlock()

	if _, ok := d.possibleNames[chosen]; ok {
		d.counter++
		d.possibleNames[chosen] = d.counter
	}
}

func (d *nameAllocator) sortByCount() namePairList {
	npl := make(namePairList, 0)
	for name, index := range d.possibleNames {
		npl = append(npl, namePair{name, index})
	}
	sort.Sort(npl)
	return npl
}
