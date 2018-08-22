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

const devicePreffix = "/dev/xvd"

type DeviceManager interface {
	// GetDevice gets the device already assigned to the volume, or assigns an unused device.
	// If the volume is already assigned, this will return the existing device with alreadyAttached=true.
	// Otherwise the device is assigned by finding the first available device, and it is returned with alreadyAttached=false.
	GetDevice(instance *ec2.Instance, volumeID string) (device string, alreadyAttached bool, err error)

	// ReleaseDevice removes the entry from the "attachments in progress" map
	// It returns true if it was found (and removed), false otherwise.
	ReleaseDevice(instance *ec2.Instance, volumeID string, device string) (released bool, err error)

	// GetAssignedDevice returns device already assigned to the volume.
	GetAssignedDevice(instance *ec2.Instance, volumeID string) (device string, err error)
}

type deviceManager struct {
	// cloud is a pointer to a Cloud struct.
	// It's necessary to fetch the devices in used by instances.
	cloud *Cloud

	// deviceAllocators holds the state of a device allocator for each node.
	deviceAllocators map[string]DeviceAllocator

	// We keep an active list of devices we have assigned but not yet
	// attached, to avoid a race condition where we assign a device mapping
	// and then get a second request before we attach the volume.
	mux       sync.Mutex
	attaching map[string]map[string]string
}

var _ DeviceManager = &deviceManager{}

func NewDeviceManager() DeviceManager {
	return &deviceManager{
		deviceAllocators: make(map[string]DeviceAllocator),
		attaching:        make(map[string]map[string]string),
	}
}

func (d *deviceManager) GetDevice(instance *ec2.Instance, volumeID string) (string, bool, error) {
	nodeID, err := d.getInstanceID(instance)
	if err != nil {
		return "", false, err
	}

	d.mux.Lock()
	defer d.mux.Unlock()

	// Get devices being attached and already attached to this instance
	deviceMappings, err := d.getInUseDevices(instance, nodeID)
	if err != nil {
		return "", false, fmt.Errorf("could not get devices used in instance %q", nodeID)
	}

	// Check to see if this volume is already assigned a device on this machine
	if dev := d.getAssignedDevice(deviceMappings, volumeID); dev != "" {
		return dev, true, nil
	}

	// Find the next unused device name
	deviceAllocator := d.deviceAllocators[nodeID]
	if deviceAllocator == nil {
		deviceAllocator = NewDeviceAllocator()
		d.deviceAllocators[nodeID] = deviceAllocator
	}

	suffix, err := deviceAllocator.GetNext(deviceMappings)
	if err != nil {
		glog.Warningf("Could not assign a mount device.  mappings=%v, error: %v", deviceMappings, err)
		return "", false, fmt.Errorf("Too many EBS volumes attached to node %s.", nodeID)
	}

	device := devicePreffix + suffix

	// Add the chosen device and volume to the "attachments in progress" map
	attaching := d.attaching[nodeID]
	if attaching == nil {
		attaching = make(map[string]string)
		d.attaching[nodeID] = attaching
	}
	attaching[device] = volumeID
	glog.V(5).Infof("Assigned mount device %s -> volume %s", device, volumeID)

	deviceAllocator.Deprioritize(suffix)

	return device, false, nil
}

func (d *deviceManager) ReleaseDevice(instance *ec2.Instance, volumeID string, device string) (bool, error) {
	nodeID, err := d.getInstanceID(instance)
	if err != nil {
		return false, err
	}

	d.mux.Lock()
	defer d.mux.Unlock()

	existingVolumeID, found := d.attaching[nodeID][device]
	if !found {
		return false, fmt.Errorf("ReleaseDevice called for disk %q when attach not in progress", volumeID)
	}

	if volumeID != existingVolumeID {
		// This actually can happen, because getMountDevice combines the attaching map with the volumes
		// attached to the instance (as reported by the EC2 API).  So if endAttaching comes after
		// a 10 second poll delay, we might well have had a concurrent request to allocate a mountpoint,
		// which because we allocate sequentially is _very_ likely to get the immediately freed volume
		return false, fmt.Errorf("ReleaseDevice on device %q assigned to different volume: %q vs %q", device, volumeID, existingVolumeID)
	}

	glog.V(5).Infof("Releasing in-process attachment entry: %s -> volume %s", device, volumeID)
	delete(d.attaching[nodeID], device)

	return true, nil
}

func (d *deviceManager) GetAssignedDevice(instance *ec2.Instance, volumeID string) (string, error) {
	nodeID, err := d.getInstanceID(instance)
	if err != nil {
		return "", err
	}

	d.mux.Lock()
	defer d.mux.Unlock()

	inUse, err := d.getInUseDevices(instance, nodeID)
	if err != nil {
		return "", fmt.Errorf("could not get devices used in instance %q", nodeID)
	}

	return d.getAssignedDevice(inUse, volumeID), nil
}

func (d *deviceManager) getInUseDevices(instance *ec2.Instance, nodeID string) (map[string]string, error) {
	deviceMappings := map[string]string{}
	for _, blockDevice := range instance.BlockDeviceMappings {
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

	for device, volume := range d.attaching[nodeID] {
		deviceMappings[device] = volume
	}

	return deviceMappings, nil
}

func (d *deviceManager) getAssignedDevice(deviceMappings map[string]string, volumeID string) string {
	for device, mappingVolumeID := range deviceMappings {
		if volumeID == mappingVolumeID {
			return device
		}
	}
	return ""
}

func (d *deviceManager) getInstanceID(instance *ec2.Instance) (string, error) {
	if instance == nil {
		return "", fmt.Errorf("nil instance")
	}
	return aws.StringValue(instance.InstanceId), nil
}
