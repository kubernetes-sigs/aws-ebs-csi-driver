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

package driver

// constants of keys in PublishContext
const (
	// devicePathKey represents key for device path in PublishContext
	// devicePath is the device path where the volume is attached to
	DevicePathKey = "devicePath"
)

// constants of keys in volume parameters
const (
	// FsTypeKey represents key for filesystem type
	FsTypeKey = "fstype"

	// VolumeTypeKey represents key for volume type
	VolumeTypeKey = "type"

	// IopsPerGBKey represents key for IOPS per GB
	IopsPerGBKey = "iopspergb"

	// EncryptedKey represents key for whether filesystem is encrypted
	EncryptedKey = "encrypted"

	// KmsKeyId represents key for KMS encryption key
	KmsKeyIdKey = "kmskeyid"
)
