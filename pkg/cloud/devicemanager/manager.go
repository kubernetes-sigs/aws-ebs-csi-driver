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
	"errors"
	"fmt"
	"maps"
	"math"
	"sync"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/kubernetes-sigs/aws-ebs-csi-driver/pkg/cloud/limits"
	"k8s.io/klog/v2"
)

type Device struct {
	Instance          *types.Instance
	Path              string
	VolumeID          string
	IsAlreadyAssigned bool
	CardIndex         *int32

	isTainted   bool
	releaseFunc func() error
}

func (d *Device) Release(force bool) {
	if !d.isTainted || force {
		if err := d.releaseFunc(); err != nil {
			klog.ErrorS(err, "Error releasing device")
		}
	}
}

// Taint marks the device as no longer reusable.
func (d *Device) Taint() {
	d.isTainted = true
}

type DeviceManager interface {
	// NewDevice retrieves the device if the device is already assigned.
	// Otherwise it creates a new device with next available device name
	// and mark it as unassigned device.
	NewDevice(instance *types.Instance, volumeID string, likelyBadNames *sync.Map) (device *Device, err error)

	// GetDevice returns the device already assigned to the volume.
	GetDevice(instance *types.Instance, volumeID string) (device *Device, err error)
}

type deviceManager struct {
	// nameAllocator assigns new device name
	nameAllocator NameAllocator

	// We keep an active list of devices we have assigned but not yet
	// attached, to avoid a race condition where we assign a device mapping
	// and then get a second request before we attach the volume.
	mux      sync.Mutex
	inFlight inFlightAttaching
}

var _ DeviceManager = &deviceManager{}

// inFlightEntry represents a volume attachment in progress.
type inFlightEntry struct {
	DeviceName string
	CardIndex  *int32
}

// inFlightAttaching represents the volumes being currently attached to nodes.
// A valid pseudo-representation of it would be {"nodeID": {"volumeID": {deviceName, cardIndex}}}.
type inFlightAttaching map[string]map[string]inFlightEntry

func (i inFlightAttaching) Add(nodeID, volumeID, deviceName string, cardIndex *int32) {
	attaching := i[nodeID]
	if attaching == nil {
		attaching = make(map[string]inFlightEntry)
		i[nodeID] = attaching
	}
	attaching[volumeID] = inFlightEntry{DeviceName: deviceName, CardIndex: cardIndex}
}

func (i inFlightAttaching) Del(nodeID, volumeID string) {
	delete(i[nodeID], volumeID)
}

func (i inFlightAttaching) GetNames(nodeID string) map[string]string {
	result := make(map[string]string)
	for volumeID, entry := range i[nodeID] {
		result[entry.DeviceName] = volumeID
	}
	return result
}

func (i inFlightAttaching) GetEntry(nodeID, volumeID string) (inFlightEntry, bool) {
	entry, exists := i[nodeID][volumeID]
	return entry, exists
}

func (i inFlightAttaching) GetEntries(nodeID string) map[string]inFlightEntry {
	return i[nodeID]
}

func NewDeviceManager() DeviceManager {
	return &deviceManager{
		nameAllocator: &nameAllocator{},
		inFlight:      make(inFlightAttaching),
	}
}

func (d *deviceManager) NewDevice(instance *types.Instance, volumeID string, likelyBadNames *sync.Map) (*Device, error) {
	d.mux.Lock()
	defer d.mux.Unlock()

	if instance == nil {
		return nil, errors.New("instance is nil")
	}

	// Get device names being attached and already attached to this instance
	inUse := d.getDeviceNamesInUse(instance)

	// Check if this volume is already assigned a device on this machine
	if path := d.getPath(inUse, volumeID); path != "" {
		cardIndex := d.getCardIndexForExistingVolume(instance, volumeID)
		return d.newBlockDevice(instance, volumeID, path, true, cardIndex), nil
	}

	nodeID, err := getInstanceID(instance)
	if err != nil {
		return nil, err
	}

	name, err := d.nameAllocator.GetNext(inUse, likelyBadNames)
	if err != nil {
		return nil, fmt.Errorf("could not get a free device name to assign to node %s", nodeID)
	}

	// Calculate card index for new volume
	instanceType := string(instance.InstanceType)
	cardCounts := d.getCardCounts(instance)
	cardIndex := getNextCardIndex(instanceType, cardCounts)

	// Add the chosen device and volume to the "attachments in progress" map
	d.inFlight.Add(nodeID, volumeID, name, cardIndex)

	return d.newBlockDevice(instance, volumeID, name, false, cardIndex), nil
}

// getCardCounts returns a map of card index to volume count, accounting for both
// volumes on the instance device mapping and volumes in the inflight map.
// It ensures volumes are not double counted if they appear in both.
func (d *deviceManager) getCardCounts(instance *types.Instance) map[int32]int {
	cardCounts := make(map[int32]int)

	// Track volume IDs we've already counted to avoid double counting
	countedVolumes := make(map[string]bool)

	// Count volumes per card from existing block device mappings
	for _, blockDevice := range instance.BlockDeviceMappings {
		if blockDevice.Ebs != nil && blockDevice.Ebs.EbsCardIndex != nil {
			cardIndex := *blockDevice.Ebs.EbsCardIndex
			cardCounts[cardIndex]++
			if blockDevice.Ebs.VolumeId != nil {
				countedVolumes[aws.ToString(blockDevice.Ebs.VolumeId)] = true
			}
		}
	}

	// Count volumes from inflight map, avoiding double counting
	nodeID := aws.ToString(instance.InstanceId)
	for volumeID, entry := range d.inFlight.GetEntries(nodeID) {
		if entry.CardIndex != nil && !countedVolumes[volumeID] {
			cardCounts[*entry.CardIndex]++
		}
	}

	return cardCounts
}

// getNextCardIndex determines the card index to use for a new volume attachment.
// It implements a "least occupied" load balancing strategy.
// Returns nil when the instance has only 1 card. For instances with multiple cards,
// returns the card with the fewest volumes. When counts are equal, prefers lower card index.
func getNextCardIndex(instanceType string, cardCounts map[int32]int) *int32 {
	numCards := limits.GetCardCount(instanceType)

	// If instance has only 1 card, return nil (no index needed)
	if numCards <= 1 {
		return nil
	}

	// Initialize card counts for all cards (ensure all cards are represented)
	for i := range numCards {
		cardIndex := int32(i) //nolint:gosec // numCards is always small (from ebsCardCounts table)
		if _, exists := cardCounts[cardIndex]; !exists {
			cardCounts[cardIndex] = 0
		}
	}

	// Find the card with the fewest devices (prefer lower index on tie)
	minCount := math.MaxInt32
	selectedCard := int32(0)
	for cardIndex, count := range cardCounts {
		if count < minCount || (count == minCount && cardIndex < selectedCard) {
			minCount = count
			selectedCard = cardIndex
		}
	}

	return &selectedCard
}

// getCardIndexForExistingVolume finds the card index for an already attached volume.
func (d *deviceManager) getCardIndexForExistingVolume(instance *types.Instance, volumeID string) *int32 {
	for _, blockDevice := range instance.BlockDeviceMappings {
		if blockDevice.Ebs != nil &&
			blockDevice.Ebs.VolumeId != nil &&
			aws.ToString(blockDevice.Ebs.VolumeId) == volumeID {
			return blockDevice.Ebs.EbsCardIndex
		}
	}
	return nil
}

func (d *deviceManager) GetDevice(instance *types.Instance, volumeID string) (*Device, error) {
	d.mux.Lock()
	defer d.mux.Unlock()

	inUse := d.getDeviceNamesInUse(instance)

	if path := d.getPath(inUse, volumeID); path != "" {
		cardIndex := d.getCardIndexForExistingVolume(instance, volumeID)
		return d.newBlockDevice(instance, volumeID, path, true, cardIndex), nil
	}

	return d.newBlockDevice(instance, volumeID, "", false, nil), nil
}

func (d *deviceManager) newBlockDevice(instance *types.Instance, volumeID string, path string, isAlreadyAssigned bool, cardIndex *int32) *Device {
	device := &Device{
		Instance:          instance,
		Path:              path,
		VolumeID:          volumeID,
		IsAlreadyAssigned: isAlreadyAssigned,
		CardIndex:         cardIndex,

		isTainted: false,
	}
	device.releaseFunc = func() error {
		return d.release(device)
	}
	return device
}

func (d *deviceManager) release(device *Device) error {
	nodeID, err := getInstanceID(device.Instance)
	if err != nil {
		return err
	}

	d.mux.Lock()
	defer d.mux.Unlock()

	entry, exists := d.inFlight.GetEntry(nodeID, device.VolumeID)
	if !exists {
		// Attaching is not in progress, so there's nothing to release
		return nil
	}

	if device.Path != entry.DeviceName {
		// This actually can happen, because GetNext combines the inFlightAttaching map with the volumes
		// attached to the instance (as reported by the EC2 API).  So if release comes after
		// a 10 second poll delay, we might as well have had a concurrent request to allocate a mountpoint,
		// which because we allocate sequentially is very likely to get the immediately freed volume.
		return fmt.Errorf("release on device %q assigned to different path: %q vs %q", device.VolumeID, device.Path, entry.DeviceName)
	}

	klog.V(5).InfoS("[Debug] Releasing in-process", "attachment entry", device.Path, "volume", device.VolumeID)
	d.inFlight.Del(nodeID, device.VolumeID)

	return nil
}

// getDeviceNamesInUse returns the device to volume ID mapping
// the mapping includes both already attached and being attached volumes.
func (d *deviceManager) getDeviceNamesInUse(instance *types.Instance) map[string]string {
	nodeID := aws.ToString(instance.InstanceId)
	inUse := map[string]string{}
	for _, blockDevice := range instance.BlockDeviceMappings {
		name := aws.ToString(blockDevice.DeviceName)
		inUse[name] = aws.ToString(blockDevice.Ebs.VolumeId)
	}

	maps.Copy(inUse, d.inFlight.GetNames(nodeID))

	return inUse
}

func (d *deviceManager) getPath(inUse map[string]string, volumeID string) string {
	for name, volID := range inUse {
		if volumeID == volID {
			return name
		}
	}
	return ""
}

func getInstanceID(instance *types.Instance) (string, error) {
	if instance == nil {
		return "", errors.New("can't get ID from a nil instance")
	}
	return aws.ToString(instance.InstanceId), nil
}
