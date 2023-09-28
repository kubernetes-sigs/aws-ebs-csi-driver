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
	"github.com/kubernetes-sigs/aws-ebs-csi-driver/tests/e2e/driver"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/kubernetes/test/e2e/framework"

	. "github.com/onsi/ginkgo/v2"
	clientset "k8s.io/client-go/kubernetes"
)

// DynamicallyProvisionedResizeVolumeTest will provision required StorageClass(es), PVC(s) and Pod(s)
// Waiting for the PV provisioner to create a new PV
// Update pvc storage size
// Waiting for new PVC and PV to be ready
// And finally attach pvc to the pod and wait for pod to be ready.
type DynamicallyProvisionedResizeVolumeTest struct {
	CSIDriver driver.DynamicPVTestDriver
	Pod       PodDetails
}

func (t *DynamicallyProvisionedResizeVolumeTest) Run(client clientset.Interface, namespace *v1.Namespace) {
	volume := t.Pod.Volumes[0]
	tpvc, _ := volume.SetupDynamicPersistentVolumeClaim(client, namespace, t.CSIDriver)
	defer tpvc.Cleanup()

	ResizePvc(client, namespace, tpvc, 1)

	By("Validate volume can be attached")
	tpod := NewTestPod(client, namespace, t.Pod.Cmd)

	tpod.SetupVolume(tpvc.persistentVolumeClaim, volume.VolumeMount.NameGenerate+"1", volume.VolumeMount.MountPathGenerate+"1", volume.VolumeMount.ReadOnly)

	By("deploying the pod")
	tpod.Create()
	By("checking that the pods is running")
	tpod.WaitForSuccess()

	defer tpod.Cleanup()

}

// WaitForPvToResize waiting for pvc size to be resized to desired size
func WaitForPvToResize(c clientset.Interface, ns *v1.Namespace, pvName string, desiredSize resource.Quantity, timeout time.Duration, interval time.Duration) error {
	By(fmt.Sprintf("Waiting up to %v for pv in namespace %q to be complete", timeout, ns.Name))
	for start := time.Now(); time.Since(start) < timeout; time.Sleep(interval) {
		newPv, _ := c.CoreV1().PersistentVolumes().Get(context.TODO(), pvName, metav1.GetOptions{})
		newPvSize := newPv.Spec.Capacity["storage"]
		if desiredSize.Equal(newPvSize) {
			By(fmt.Sprintf("Pv size is updated to %v", newPvSize.String()))
			return nil
		}
	}
	return fmt.Errorf("Gave up after waiting %v for pv %q to complete resizing", timeout, pvName)
}

// TODO move to testsuites.go?

// ResizePvc increases size of given persistent volume claim by `sizeIncreaseGi` Gigabytes
func ResizePvc(client clientset.Interface, namespace *v1.Namespace, testPvc *TestPersistentVolumeClaim, sizeIncreaseGi int64) (updatedPvc *v1.PersistentVolumeClaim, updatedSize resource.Quantity) {
	By(fmt.Sprintf("getting pvc name: %v", testPvc.persistentVolumeClaim.Name))
	pvc, _ := client.CoreV1().PersistentVolumeClaims(namespace.Name).Get(context.TODO(), testPvc.persistentVolumeClaim.Name, metav1.GetOptions{})

	originalSize := pvc.Spec.Resources.Requests["storage"]
	delta := resource.Quantity{}
	delta.Set(util.GiBToBytes(sizeIncreaseGi))
	originalSize.Add(delta)
	pvc.Spec.Resources.Requests["storage"] = originalSize

	By("resizing the pvc")
	updatedPvc, err := client.CoreV1().PersistentVolumeClaims(namespace.Name).Update(context.TODO(), pvc, metav1.UpdateOptions{})
	if err != nil {
		framework.ExpectNoError(err, fmt.Sprintf("fail to resize pvc(%s): %v", pvc.Name, err))
	}
	updatedSize = updatedPvc.Spec.Resources.Requests["storage"]

	By("checking the resizing PV result")
	err = WaitForPvToResize(client, namespace, updatedPvc.Spec.VolumeName, updatedSize, 1*time.Minute, 5*time.Second)
	framework.ExpectNoError(err)

	return
}
