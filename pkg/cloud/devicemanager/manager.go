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
	"fmt"
	"strings"
	"sync"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/golang/glog"
)

const devicePreffix = "/dev/xvd"

type BlockDevice struct {
	Instance          *ec2.Instance
	Path              string
	VolumeID          string
	IsAlreadyAssigned bool

	isTainted   bool
	releaseFunc func() error
}

func (d *BlockDevice) Release(force bool) {
	if !d.isTainted || force {
		if err := d.releaseFunc(); err != nil {
			glog.Errorf("Error releasing device: %v", err)
		}
	}
}

func (d *BlockDevice) Taint() {
	d.isTainted = true
}

type BlockDeviceManager interface {
	// NewBlockDevice gets the device already assigned to the volume, or assigns an unused device.
	// If the volume is already assigned, this will return the existing device with alreadyAttached=true.
	// Otherwise the device is assigned by finding the first available device, and it is returned with alreadyAttached=false.
	NewBlockDevice(instance *ec2.Instance, volumeID string) (device *BlockDevice, err error)

	// GetBlockDevice returns device already assigned to the volume.
	GetBlockDevice(instance *ec2.Instance, volumeID string) (device *BlockDevice, err error)
}

type blockDeviceManager struct {
	// deviceAllocators holds the state of a device allocator for each node.
	deviceAllocators map[string]DeviceAllocator

	// We keep an active list of devices we have assigned but not yet
	// attached, to avoid a race condition where we assign a device mapping
	// and then get a second request before we attach the volume.
	mux       sync.Mutex
	attaching map[string]map[string]string
}

var _ BlockDeviceManager = &blockDeviceManager{}

func NewBlockDeviceManager() BlockDeviceManager {
	return &blockDeviceManager{
		deviceAllocators: make(map[string]DeviceAllocator),
		attaching:        make(map[string]map[string]string),
	}
}

func (d *blockDeviceManager) newBlockDevice(instance *ec2.Instance, volumeID string, path string, isAlreadyAssigned bool) *BlockDevice {
	device := &BlockDevice{
		Instance:          instance,
		Path:              path,
		VolumeID:          volumeID,
		IsAlreadyAssigned: isAlreadyAssigned,

		isTainted: false,
	}
	device.releaseFunc = func() error {
		return d.release(device)
	}
	return device
}

func (d *blockDeviceManager) NewBlockDevice(instance *ec2.Instance, volumeID string) (*BlockDevice, error) {
	nodeID, err := getInstanceID(instance)
	if err != nil {
		return nil, err
	}

	d.mux.Lock()
	defer d.mux.Unlock()

	// Get device suffixes being attached and already attached to this instance
	inUse, err := d.getSuffixesInUse(instance, nodeID)
	if err != nil {
		return nil, fmt.Errorf("could not get devices used in instance %q", nodeID)
	}

	// Check if this volume is already assigned a device on this machine
	if path := d.getPath(inUse, volumeID); path != "" {
		return d.newBlockDevice(instance, volumeID, path, true), nil
	}

	// Find the next unused device name
	deviceAllocator := d.deviceAllocators[nodeID]
	if deviceAllocator == nil {
		deviceAllocator = NewDeviceAllocator()
		d.deviceAllocators[nodeID] = deviceAllocator
	}

	//TODO rename device allocator to suffix allocator
	suffix, err := deviceAllocator.GetNext(inUse)
	if err != nil {
		glog.Warningf("Could not assign a mount device.  mappings=%v, error: %v", inUse, err)
		return nil, fmt.Errorf("too many EBS volumes attached to node %s", nodeID)
	}

	// Add the chosen device and volume to the "attachments in progress" map
	attaching := d.attaching[nodeID]
	if attaching == nil {
		attaching = make(map[string]string)
		d.attaching[nodeID] = attaching
	}
	attaching[suffix] = volumeID
	glog.V(5).Infof("Assigned device suffix %s to volume %s", suffix, volumeID)

	// Deprioritize this suffix so it's not picked again right away.
	deviceAllocator.Deprioritize(suffix)

	return d.newBlockDevice(instance, volumeID, devicePreffix+suffix, false), nil
}

func (d *blockDeviceManager) GetBlockDevice(instance *ec2.Instance, volumeID string) (*BlockDevice, error) {
	nodeID, err := getInstanceID(instance)
	if err != nil {
		return nil, err
	}

	d.mux.Lock()
	defer d.mux.Unlock()

	inUse, err := d.getSuffixesInUse(instance, nodeID)
	if err != nil {
		return nil, fmt.Errorf("could not get devices used in instance %q", nodeID)
	}

	path := d.getPath(inUse, volumeID)
	device := d.newBlockDevice(instance, volumeID, path, false)

	if path != "" {
		device.IsAlreadyAssigned = true
		device.releaseFunc = func() error { return d.release(device) }
	}

	return device, nil
}

func (d *blockDeviceManager) release(device *BlockDevice) error {
	nodeID, err := getInstanceID(device.Instance)
	if err != nil {
		return err
	}

	d.mux.Lock()
	defer d.mux.Unlock()

	var suffix string
	if len(device.Path) > 2 {
		suffix = device.Path[len(device.Path)-2:]
	}

	existingVolumeID, found := d.attaching[nodeID][suffix]
	if !found {
		return fmt.Errorf("release called for disk %q when attach not in progress", device.VolumeID)
	}

	if device.VolumeID != existingVolumeID {
		// This actually can happen, because getMountDevice combines the attaching map with the volumes
		// attached to the instance (as reported by the EC2 API).  So if endAttaching comes after
		// a 10 second poll delay, we might well have had a concurrent request to allocate a mountpoint,
		// which because we allocate sequentially is _very_ likely to get the immediately freed volume
		return fmt.Errorf("release on device %q assigned to different volume: %q vs %q", device.Path, device.VolumeID, existingVolumeID)
	}

	glog.V(5).Infof("Releasing in-process attachment entry: %s -> volume %s", device, device.VolumeID)
	delete(d.attaching[nodeID], suffix)

	return nil
}

func (d *blockDeviceManager) getSuffixesInUse(instance *ec2.Instance, nodeID string) (map[string]string, error) {
	inUse := map[string]string{}
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
		inUse[name] = aws.StringValue(blockDevice.Ebs.VolumeId)
	}

	for suffix, volumeID := range d.attaching[nodeID] {
		inUse[suffix] = volumeID
	}

	return inUse, nil
}

func (d *blockDeviceManager) getPath(inUse map[string]string, volumeID string) string {
	for suffix, volID := range inUse {
		if volumeID == volID {
			return devicePreffix + suffix
		}
	}
	return ""
}

func getInstanceID(instance *ec2.Instance) (string, error) {
	if instance == nil {
		return "", fmt.Errorf("can't get ID from a nil instance")
	}
	return aws.StringValue(instance.InstanceId), nil
}
