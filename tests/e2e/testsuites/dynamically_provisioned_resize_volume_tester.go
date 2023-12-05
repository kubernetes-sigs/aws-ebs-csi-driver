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
	"github.com/kubernetes-sigs/aws-ebs-csi-driver/tests/e2e/driver"
	. "github.com/onsi/ginkgo/v2"
	v1 "k8s.io/api/core/v1"
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

	By("resizing the volume")
	ResizeTestPvc(client, namespace, tpvc, DefaultSizeIncreaseGi)

	By("validate volume can be attached")
	tpod := NewTestPod(client, namespace, t.Pod.Cmd)

	tpod.SetupVolume(tpvc.persistentVolumeClaim, volume.VolumeMount.NameGenerate+"1", volume.VolumeMount.MountPathGenerate+"1", volume.VolumeMount.ReadOnly)

	By("deploying the pod")
	tpod.Create()
	By("checking that the pods is running")
	tpod.WaitForSuccess()

	defer tpod.Cleanup()
}
