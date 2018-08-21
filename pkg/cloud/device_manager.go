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

package cloud

import (
	"fmt"
	"strings"
	"sync"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/golang/glog"
)

type DeviceManager interface {
	// Gets the mountDevice already assigned to the volume, or assigns an unused mountDevice.
	// If the volume is already assigned, this will return the existing mountDevice with alreadyAttached=true.
	// Otherwise the mountDevice is assigned by finding the first available mountDevice, and it is returned with alreadyAttached=false.
	GetDevice(instanceID string, info *ec2.Instance, volumeID string, assign bool) (assigned string, alreadyAttached bool, err error)

	// endAttaching removes the entry from the "attachments in progress" map
	// It returns true if it was found (and removed), false otherwise.
	EndAttaching(instanceID string, volumeID string, mountDevice string) bool

	// DeprioritizeDevice the device so as it can't be used immediately again.
	DeprioritizeDevice(instanceID string, mountDevice string)
}

type deviceManager struct {
	// state of our device allocator for each node
	deviceAllocators map[string]DeviceAllocator

	// We keep an active list of devices we have assigned but not yet
	// attached, to avoid a race condition where we assign a device mapping
	// and then get a second request before we attach the volume
	attachingMutex sync.Mutex
	attaching      map[string]map[string]string
}

var _ DeviceManager = &deviceManager{}

func NewDeviceManager() DeviceManager {
	return &deviceManager{
		deviceAllocators: make(map[string]DeviceAllocator),
		attaching:        make(map[string]map[string]string),
	}
}

func (d *deviceManager) DeprioritizeDevice(instanceID string, mountDevice string) {
	if da, ok := d.deviceAllocators[instanceID]; ok {
		da.Deprioritize(mountDevice)
	}
}

func (d *deviceManager) GetDevice(instanceID string, info *ec2.Instance, volumeID string, assign bool) (assigned string, alreadyAttached bool, err error) {
	//instanceType := i.getInstanceType()
	//if instanceType == nil {
	//return "", false, fmt.Errorf("could not get instance type for instance: %s", i.awsID)
	//}

	//volumeID := awsVolumeID(v)

	deviceMappings := map[string]string{}
	for _, blockDevice := range info.BlockDeviceMappings {
		name := aws.StringValue(blockDevice.DeviceName)
		if strings.HasPrefix(name, "/dev/sd") {
			name = name[7:]
		}
		if strings.HasPrefix(name, "/dev/xvd") {
			name = name[8:]
		}
		if len(name) < 1 || len(name) > 2 {
			glog.Warningf("Unexpected EBS DeviceName: %q", aws.StringValue(blockDevice.DeviceName))
		}
		deviceMappings[name] = aws.StringValue(blockDevice.Ebs.VolumeId)
	}

	// We lock to prevent concurrent mounts from conflicting
	// We may still conflict if someone calls the API concurrently,
	// but the AWS API will then fail one of the two attach operations
	d.attachingMutex.Lock()
	defer d.attachingMutex.Unlock()

	for mountDevice, volume := range d.attaching[instanceID] {
		deviceMappings[mountDevice] = volume
	}

	// Check to see if this volume is already assigned a device on this machine
	for mountDevice, mappingVolumeID := range deviceMappings {
		if volumeID == mappingVolumeID {
			if assign {
				glog.Warningf("Got assignment call for already-assigned volume: %s@%s", mountDevice, mappingVolumeID)
			}
			return mountDevice, true, nil
		}
	}

	if !assign {
		return "", false, nil
	}

	// Find the next unused device name
	deviceAllocator := d.deviceAllocators[instanceID]
	if deviceAllocator == nil {
		// we want device names with two significant characters, starting with /dev/xvdbb
		// the allowed range is /dev/xvd[b-c][a-z]
		// http://docs.aws.amazon.com/AWSEC2/latest/UserGuide/device_naming.html
		deviceAllocator = NewDeviceAllocator()
		d.deviceAllocators[instanceID] = deviceAllocator
	}
	// We need to lock deviceAllocator to prevent possible race with Deprioritize function
	deviceAllocator.Lock()
	defer deviceAllocator.Unlock()

	//chosen, err := deviceAllocator.GetNext(deviceMappings)
	chosen, err := deviceAllocator.GetNext(deviceMappings)
	if err != nil {
		glog.Warningf("Could not assign a mount device.  mappings=%v, error: %v", deviceMappings, err)
		return "", false, fmt.Errorf("Too many EBS volumes attached to node %s.", instanceID)
	}

	attaching := d.attaching[instanceID]
	if attaching == nil {
		attaching = make(map[string]string)
		d.attaching[instanceID] = attaching
	}
	attaching[chosen] = volumeID
	glog.V(2).Infof("Assigned mount device %s -> volume %s", chosen, volumeID)

	return chosen, false, nil
}

func (d *deviceManager) EndAttaching(nodeID string, volumeID string, mountDevice string) bool {
	d.attachingMutex.Lock()
	defer d.attachingMutex.Unlock()

	existingVolumeID, found := d.attaching[nodeID][mountDevice]
	if !found {
		glog.Errorf("EndAttaching called for disk %q when attach not in progress", volumeID)
		return false
	}
	if volumeID != existingVolumeID {
		// This actually can happen, because getMountDevice combines the attaching map with the volumes
		// attached to the instance (as reported by the EC2 API).  So if endAttaching comes after
		// a 10 second poll delay, we might well have had a concurrent request to allocate a mountpoint,
		// which because we allocate sequentially is _very_ likely to get the immediately freed volume
		glog.Infof("EndAttaching on device %q assigned to different volume: %q vs %q", mountDevice, volumeID, existingVolumeID)
		return false
	}
	glog.V(2).Infof("Releasing in-process attachment entry: %s -> volume %s", mountDevice, volumeID)
	delete(d.attaching[nodeID], mountDevice)
	return true
}
