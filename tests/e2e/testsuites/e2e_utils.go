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

package testsuites

import (
	"context"
	"fmt"
	"time"

	"github.com/kubernetes-sigs/aws-ebs-csi-driver/pkg/util"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/kubernetes/test/e2e/framework"
)

const (
	DefaultVolumeName = "test-volume-1"
	DefaultMountPath  = "/mnt/default-mount"

	DefaultIopsIoVolumes = "100"

	DefaultSizeIncreaseGi = int32(1)

	DefaultModificationTimeout   = 3 * time.Minute
	DefaultResizeTimout          = 1 * time.Minute
	DefaultK8sApiPollingInterval = 5 * time.Second

	Iops       = "iops"
	Throughput = "throughput"
	VolumeType = "type"
	TagSpec    = "tagSpecification"
	TagDel     = "tagDeletion"
)

var DefaultGeneratedVolumeMount = VolumeMountDetails{
	NameGenerate:      "test-volume-",
	MountPathGenerate: "/mnt/test-",
}

// PodCmdWriteToVolume returns pod command that would write to mounted volume
func PodCmdWriteToVolume(volumeMountPath string) string {
	return fmt.Sprintf("echo 'hello world' >> %s/data && grep 'hello world' %s/data && sync", volumeMountPath, volumeMountPath)
}

// PodCmdContinuousWrite returns pod command that would continuously write to mounted volume
func PodCmdContinuousWrite(volumeMountPath string) string {
	return fmt.Sprintf("while true; do echo \"$(date -u)\" >> /%s/out.txt; sleep 5; done", volumeMountPath)
}

// PodCmdGrepVolumeData returns pod command that would check that a volume was written to by PodCmdWriteToVolume
func PodCmdGrepVolumeData(volumeMountPath string) string {
	return fmt.Sprintf("grep 'hello world' %s/data", volumeMountPath)
}

// IncreasePvcObjectStorage increases `storage` of a K8s PVC object by specified Gigabytes
func IncreasePvcObjectStorage(pvc *v1.PersistentVolumeClaim, sizeIncreaseGi int32) resource.Quantity {
	pvcSize := pvc.Spec.Resources.Requests["storage"]
	delta := resource.Quantity{}
	delta.Set(util.GiBToBytes(sizeIncreaseGi))
	pvcSize.Add(delta)
	pvc.Spec.Resources.Requests["storage"] = pvcSize
	return pvcSize
}

// WaitForPvToResize waiting for pvc size to be resized to desired size
func WaitForPvToResize(c clientset.Interface, ns *v1.Namespace, pvName string, desiredSize resource.Quantity, timeout time.Duration, interval time.Duration) error {
	framework.Logf("waiting up to %v for pv resize in namespace %q to be complete", timeout, ns.Name)
	for start := time.Now(); time.Since(start) < timeout; time.Sleep(interval) {
		newPv, _ := c.CoreV1().PersistentVolumes().Get(context.TODO(), pvName, metav1.GetOptions{})
		newPvSize := newPv.Spec.Capacity["storage"]
		if desiredSize.Equal(newPvSize) {
			framework.Logf("pv size is updated to %v", newPvSize.String())
			return nil
		}
	}
	return fmt.Errorf("gave up after waiting %v for pv %q to complete resizing", timeout, pvName)
}

// ResizeTestPvc increases size of given `TestPersistentVolumeClaim` by specified Gigabytes
func ResizeTestPvc(client clientset.Interface, namespace *v1.Namespace, testPvc *TestPersistentVolumeClaim, sizeIncreaseGi int32) (updatedSize resource.Quantity) {
	framework.Logf("getting pvc name: %v", testPvc.persistentVolumeClaim.Name)
	pvc, _ := client.CoreV1().PersistentVolumeClaims(namespace.Name).Get(context.TODO(), testPvc.persistentVolumeClaim.Name, metav1.GetOptions{})

	IncreasePvcObjectStorage(pvc, sizeIncreaseGi)

	framework.Logf("updating the pvc object")
	updatedPvc, err := client.CoreV1().PersistentVolumeClaims(namespace.Name).Update(context.TODO(), pvc, metav1.UpdateOptions{})
	if err != nil {
		framework.ExpectNoError(err, fmt.Sprintf("fail to resize pvc(%s): %v", pvc.Name, err))
	}
	updatedSize = updatedPvc.Spec.Resources.Requests["storage"]

	framework.Logf("checking the resizing PV result")
	err = WaitForPvToResize(client, namespace, updatedPvc.Spec.VolumeName, updatedSize, DefaultResizeTimout, DefaultK8sApiPollingInterval)
	framework.ExpectNoError(err)
	return updatedSize
}

// AnnotatePvc annotates supplied k8s pvc object with supplied annotations
func AnnotatePvc(pvc *v1.PersistentVolumeClaim, annotations map[string]string) {
	for annotation, value := range annotations {
		pvc.Annotations[annotation] = value
	}
}

// CheckPvAnnotations checks whether supplied k8s pv object contains supplied annotations
func CheckPvAnnotations(pv *v1.PersistentVolume, annotations map[string]string) bool {
	for annotation, value := range annotations {
		if pv.Annotations[annotation] != value {
			return false
		}
	}
	return true
}

// WaitForPvToModify waiting for PV to be modified
func WaitForPvToModify(c clientset.Interface, ns *v1.Namespace, pvName string, expectedAnnotations map[string]string, timeout time.Duration, interval time.Duration) error {
	framework.Logf("waiting up to %v for pv in namespace %q to be modified", timeout, ns.Name)

	for start := time.Now(); time.Since(start) < timeout; time.Sleep(interval) {
		modifyingPv, _ := c.CoreV1().PersistentVolumes().Get(context.TODO(), pvName, metav1.GetOptions{})

		if CheckPvAnnotations(modifyingPv, expectedAnnotations) {
			framework.Logf("pv annotations are updated to %v", modifyingPv.Annotations)
			return nil
		}
	}
	return fmt.Errorf("gave up after waiting %v for pv %q to complete modifying", timeout, pvName)
}

// WaitForVacToApplyToPv waits for a PV's VAC to match the PVC's VAC
func WaitForVacToApplyToPv(c clientset.Interface, ns *v1.Namespace, pvName string, expectedVac string, timeout time.Duration, interval time.Duration) error {
	framework.Logf("waiting up to %v for pv in namespace %q to be modified via VAC", timeout, ns.Name)

	for start := time.Now(); time.Since(start) < timeout; time.Sleep(interval) {
		modifyingPv, _ := c.CoreV1().PersistentVolumes().Get(context.TODO(), pvName, metav1.GetOptions{})

		if modifyingPv.Spec.VolumeAttributesClassName != nil && *modifyingPv.Spec.VolumeAttributesClassName == expectedVac {
			framework.Logf("vac updated to %v", *modifyingPv.Spec.VolumeAttributesClassName)
			return nil
		}
	}
	return fmt.Errorf("gave up after waiting %v for pv %q to complete modifying via VAC", timeout, pvName)
}

func CreateVolumeDetails(createVolumeParameters map[string]string, volumeSize string) *VolumeDetails {
	allowVolumeExpansion := true

	volume := VolumeDetails{
		MountOptions: []string{"rw"},
		ClaimSize:    volumeSize,
		VolumeMount: VolumeMountDetails{
			NameGenerate:      DefaultVolumeName,
			MountPathGenerate: DefaultMountPath,
		},
		AllowVolumeExpansion:   &allowVolumeExpansion,
		CreateVolumeParameters: createVolumeParameters,
	}

	return &volume
}

func PrefixAnnotations(prefix string, parameters map[string]string) map[string]string {
	result := make(map[string]string)
	for key, value := range parameters {
		result[prefix+key] = value
	}
	return result
}
