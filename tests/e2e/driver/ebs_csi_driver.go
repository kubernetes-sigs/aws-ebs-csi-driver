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
	"github.com/kubernetes-csi/external-snapshotter/pkg/apis/volumesnapshot/v1alpha1"
	ebscsidriver "github.com/kubernetes-sigs/aws-ebs-csi-driver/pkg/driver"
	"k8s.io/api/core/v1"
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

func (d *ebsCSIDriver) GetVolumeSnapshotClass(namespace string) *v1alpha1.VolumeSnapshotClass {
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
		"type":   volumeType,
		"fsType": fsType,
	}
	if iops := IOPSPerGBForVolumeType(volumeType); iops != "" {
		parameters["iopsPerGB"] = iops
	}
	if encrypted {
		parameters["encrypted"] = True
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
