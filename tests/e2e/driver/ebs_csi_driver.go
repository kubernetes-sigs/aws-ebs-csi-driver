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

	"github.com/kubernetes-csi/external-snapshotter/v2/pkg/apis/volumesnapshot/v1beta1"
	ebscsidriver "github.com/kubernetes-sigs/aws-ebs-csi-driver/pkg/driver"
	v1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	True = "true"
)

// Implement DynamicPVTestDriver interface
type ebsCSIDriver struct {
	driverName string
}

// InitEbsCSIDriver returns ebsCSIDriver that implements DynamicPVTestDriver interface
func InitEbsCSIDriver() PVTestDriver {
	return &ebsCSIDriver{
		driverName: ebscsidriver.DriverName,
	}
}

func (d *ebsCSIDriver) GetDynamicProvisionStorageClass(parameters map[string]string, mountOptions []string, reclaimPolicy *v1.PersistentVolumeReclaimPolicy, bindingMode *storagev1.VolumeBindingMode, allowedTopologyValues []string, namespace string) *storagev1.StorageClass {
	provisioner := d.driverName
	generateName := fmt.Sprintf("%s-%s-dynamic-sc-", namespace, provisioner)
	allowedTopologies := []v1.TopologySelectorTerm{}
	if len(allowedTopologyValues) > 0 {
		allowedTopologies = []v1.TopologySelectorTerm{
			{
				MatchLabelExpressions: []v1.TopologySelectorLabelRequirement{
					{
						Key:    ebscsidriver.TopologyKey,
						Values: allowedTopologyValues,
					},
				},
			},
		}
	}
	return getStorageClass(generateName, provisioner, parameters, mountOptions, reclaimPolicy, bindingMode, allowedTopologies)
}

func (d *ebsCSIDriver) GetVolumeSnapshotClass(namespace string) *v1beta1.VolumeSnapshotClass {
	provisioner := d.driverName
	generateName := fmt.Sprintf("%s-%s-dynamic-sc-", namespace, provisioner)
	return getVolumeSnapshotClass(generateName, provisioner)
}

func (d *ebsCSIDriver) GetPersistentVolume(volumeID string, fsType string, size string, reclaimPolicy *v1.PersistentVolumeReclaimPolicy, namespace string) *v1.PersistentVolume {
	provisioner := d.driverName
	generateName := fmt.Sprintf("%s-%s-preprovsioned-pv-", namespace, provisioner)
	// Default to Retain ReclaimPolicy for pre-provisioned volumes
	pvReclaimPolicy := v1.PersistentVolumeReclaimRetain
	if reclaimPolicy != nil {
		pvReclaimPolicy = *reclaimPolicy
	}
	return &v1.PersistentVolume{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: generateName,
			Namespace:    namespace,
			// TODO remove if https://github.com/kubernetes-csi/external-provisioner/issues/202 is fixed
			Annotations: map[string]string{
				"pv.kubernetes.io/provisioned-by": provisioner,
			},
		},
		Spec: v1.PersistentVolumeSpec{
			AccessModes: []v1.PersistentVolumeAccessMode{v1.ReadWriteOnce},
			Capacity: v1.ResourceList{
				v1.ResourceName(v1.ResourceStorage): resource.MustParse(size),
			},
			PersistentVolumeReclaimPolicy: pvReclaimPolicy,
			PersistentVolumeSource: v1.PersistentVolumeSource{
				CSI: &v1.CSIPersistentVolumeSource{
					Driver:       provisioner,
					VolumeHandle: volumeID,
					FSType:       fsType,
				},
			},
		},
	}
}

// GetParameters returns the parameters specific for this driver
func GetParameters(volumeType string, fsType string, encrypted bool) map[string]string {
	parameters := map[string]string{
		"type":                      volumeType,
		"csi.storage.k8s.io/fstype": fsType,
	}
	if iopsPerGB := IOPSPerGBForVolumeType(volumeType); iopsPerGB != "" {
		parameters[ebscsidriver.IopsPerGBKey] = iopsPerGB
	}
	if iops := IOPSForVolumeType(volumeType); iops != "" {
		parameters[ebscsidriver.IopsKey] = iops
	}
	if throughput := ThroughputForVolumeType(volumeType); throughput != "" {
		parameters[ebscsidriver.ThroughputKey] = throughput
	}
	if encrypted {
		parameters[ebscsidriver.EncryptedKey] = True
	}
	return parameters
}

// MinimumSizeForVolumeType returns the minimum disk size for each volumeType
func MinimumSizeForVolumeType(volumeType string) string {
	switch volumeType {
	case "st1", "sc1":
		return "500Gi"
	case "gp2", "gp3":
		return "1Gi"
	case "io1", "io2":
		return "4Gi"
	case "standard":
		return "10Gi"
	default:
		return "1Gi"
	}
}

// IOPSPerGBForVolumeType returns the maximum iops per GB for each volumeType
// Otherwise returns an empty string
func IOPSPerGBForVolumeType(volumeType string) string {
	switch volumeType {
	case "io1":
		// Maximum IOPS/GB for io1 is 50
		return "50"
	case "io2":
		// Maximum IOPS/GB for io2 is 500
		return "500"
	default:
		return ""
	}
}

// IOPSForVolumeType returns the maximum iops for each volumeType
// Otherwise returns an empty string
func IOPSForVolumeType(volumeType string) string {
	switch volumeType {
	case "gp3":
		// Maximum IOPS for gp3 is 16000. However, maximum IOPS/GB for gp3 is 500.
		// Since the tests will run using minimum volume capacity (1GB), set to 500.
		return "500"
	default:
		return ""
	}
}

// ThroughputPerVolumeType returns the maximum throughput for each volumeType
// Otherwise returns an empty string
func ThroughputForVolumeType(volumeType string) string {
	switch volumeType {
	case "gp3":
		// Maximum throughput for gp3 is 1000. However, maximum throughput/iops for gp3 is 0.25
		// Since the default iops is 3000, set to 750.
		return "750"
	default:
		return ""
	}
}
