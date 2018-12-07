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

import (
	"fmt"
	ebscsidriver "github.com/kubernetes-sigs/aws-ebs-csi-driver/pkg/driver"
	"k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
)

// Implement DynamicPVTestDriver interface
type ebsCSIDriver struct {
	driverName string
}

// InitEbsCSIDriver returns ebsCSIDriver that implements DynamicPVTestDriver interface
func InitEbsCSIDriver() DynamicPVTestDriver {
	return &ebsCSIDriver{
		driverName: ebscsidriver.DriverName,
	}
}

func (d *ebsCSIDriver) GetDynamicProvisionStorageClass(parameters map[string]string, reclaimPolicy *v1.PersistentVolumeReclaimPolicy, bindingMode *storagev1.VolumeBindingMode, namespace string) *storagev1.StorageClass {
	provisioner := d.driverName
	generatedName := fmt.Sprintf("%s-%s-sc-", namespace, provisioner)

	return getStorageClass(generatedName, provisioner, parameters, reclaimPolicy, bindingMode)
}

// GetParameters returns the parameters specific for this driver
func GetParameters(volumeType string, fsType string) map[string]string {
	parameters := map[string]string{
		"type":   volumeType,
		"fsType": fsType,
	}
	if iops := IOPSPerGBForVolumeType(volumeType); iops != "" {
		parameters["iopsPerGB"] = iops
	}
	return parameters
}

// MinimumSizeForVolumeType returns the minimum disk size for each volumeType
func MinimumSizeForVolumeType(volumeType string) string {
	switch volumeType {
	case "st1":
		return "500Gi"
	case "sc1":
		return "500Gi"
	case "gp2":
		return "1Gi"
	case "io1":
		return "4Gi"
	default:
		return "1Gi"
	}
}

// IOPSPerGBForVolumeType returns 25 for io1 volumeType
// Otherwise returns an empty string
func IOPSPerGBForVolumeType(volumeType string) string {
	if volumeType == "io1" {
		// Minimum disk size is 4, minimum IOPS is 100
		return "25"
	}
	return ""
}
