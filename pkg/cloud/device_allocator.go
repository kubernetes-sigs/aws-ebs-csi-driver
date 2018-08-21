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

package cloud

import (
	"fmt"
	"sort"
	"sync"
)

// awsVolumeID represents the ID of the volume in the AWS API, e.g. vol-12345678
// The "traditional" format is "vol-12345678"
// A new longer format is also being introduced: "vol-12345678abcdef01"
// We should not assume anything about the length or format, though it seems
// reasonable to assume that volumes will continue to start with "vol-".
//type awsVolumeID string

// Used to represent a mount device for attaching an EBS volume
// This should be stored as a single letter (i.e. c, not sdc or /dev/sdc)
//type mountDevice string

// ExistingDevices is a map of assigned devices. Presence of a key with a device
// name in the map means that the device is allocated. Value is irrelevant and
// can be used for anything that DeviceAllocator user wants.
// Only the relevant part of device name should be in the map, e.g. "ba" for
// "/dev/xvdba".
type ExistingDevices map[string]string

// On AWS, we should assign new (not yet used) device names to attached volumes.
// If we reuse a previously used name, we may get the volume "attaching" forever,
// see https://aws.amazon.com/premiumsupport/knowledge-center/ebs-stuck-attaching/.
// DeviceAllocator finds available device name, taking into account already
// assigned device names from ExistingDevices map. It tries to find the next
// device name to the previously assigned one (from previous DeviceAllocator
// call), so all available device names are used eventually and it minimizes
// device name reuse.
// All these allocations are in-memory, nothing is written to / read from
// /dev directory.
type DeviceAllocator interface {
	// GetNext returns a free device name or error when there is no free device
	// name. Only the device suffix is returned, e.g. "ba" for "/dev/xvdba".
	// It's up to the called to add appropriate "/dev/sd" or "/dev/xvd" prefix.
	GetNext(existingDevices ExistingDevices) (string, error)

	// Deprioritize the device so as it can't be used immediately again
	Deprioritize(string)

	// Lock the deviceAllocator
	Lock()

	// Unlock the deviceAllocator
	Unlock()
}

type deviceAllocator struct {
	possibleDevices map[string]int
	counter         int
	deviceLock      sync.Mutex
}

var _ DeviceAllocator = &deviceAllocator{}

type devicePair struct {
	deviceName  string
	deviceIndex int
}

type devicePairList []devicePair

func (p devicePairList) Len() int           { return len(p) }
func (p devicePairList) Less(i, j int) bool { return p[i].deviceIndex < p[j].deviceIndex }
func (p devicePairList) Swap(i, j int)      { p[i], p[j] = p[j], p[i] }

// Allocates device names according to scheme ba..bz, ca..cz
// it moves along the ring and always picks next device until
// device list is exhausted.
func NewDeviceAllocator() DeviceAllocator {
	possibleDevices := make(map[string]int)
	for _, firstChar := range []rune{'b', 'c'} {
		for i := 'a'; i <= 'z'; i++ {
			dev := string([]rune{firstChar, i})
			possibleDevices[dev] = 0
		}
	}
	return &deviceAllocator{
		possibleDevices: possibleDevices,
		counter:         0,
	}
}

// GetNext gets next available device from the pool, this function assumes that caller
// holds the necessary lock on deviceAllocator
func (d *deviceAllocator) GetNext(existingDevices ExistingDevices) (string, error) {
	for _, devicePair := range d.sortByCount() {
		if _, found := existingDevices[devicePair.deviceName]; !found {
			return devicePair.deviceName, nil
		}
	}
	return "", fmt.Errorf("no devices are available")
}

func (d *deviceAllocator) sortByCount() devicePairList {
	dpl := make(devicePairList, 0)
	for deviceName, deviceIndex := range d.possibleDevices {
		dpl = append(dpl, devicePair{deviceName, deviceIndex})
	}
	sort.Sort(dpl)
	return dpl
}

func (d *deviceAllocator) Lock() {
	d.deviceLock.Lock()
}

func (d *deviceAllocator) Unlock() {
	d.deviceLock.Unlock()
}

// Deprioritize the device so as it can't be used immediately again
func (d *deviceAllocator) Deprioritize(chosen string) {
	d.deviceLock.Lock()
	defer d.deviceLock.Unlock()
	if _, ok := d.possibleDevices[chosen]; ok {
		d.counter++
		d.possibleDevices[chosen] = d.counter
	}
}
