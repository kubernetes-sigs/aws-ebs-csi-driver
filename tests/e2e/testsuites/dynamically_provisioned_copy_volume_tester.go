/*
Copyright 2023 The Kubernetes Authors.

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

// DynamicallyProvisionedCopyVolumeTest will provision required StorageClass(es), PVC(s) and Pod(s)
// Waiting for the PV provisioner to create a new PV
// Testing if the Pod(s) can write and read to mounted volumes
// Create a copy of the volume using PVC as data source, validate the data is copied, and then write and read to it again
// This test only supports a single volume.
type DynamicallyProvisionedCopyVolumeTest struct {
	CSIDriver    driver.PVTestDriver
	Pod          PodDetails
	ClonedPod    PodDetails
	ValidateFunc func()
}

func (t *DynamicallyProvisionedCopyVolumeTest) Run(client clientset.Interface, namespace *v1.Namespace) {
	tpod := NewTestPod(client, namespace, t.Pod.Cmd)
	volume := t.Pod.Volumes[0]
	tpvc, pvcCleanup := volume.SetupDynamicPersistentVolumeClaim(client, namespace, t.CSIDriver)
	for i := range pvcCleanup {
		defer pvcCleanup[i]()
	}
	tpod.SetupVolume(tpvc.persistentVolumeClaim, volume.VolumeMount.NameGenerate+"1", volume.VolumeMount.MountPathGenerate+"1", volume.VolumeMount.ReadOnly)

	By("deploying the pod")
	tpod.Create()
	defer tpod.Cleanup()
	By("checking that the pods command exits with no error")
	tpod.WaitForSuccess()

	By("creating a clone of the source volume")
	t.ClonedPod.Volumes[0].DataSource = &DataSource{
		Name: tpvc.persistentVolumeClaim.Name,
		Kind: PersistentVolumeClaimKind,
	}

	tcpod := NewTestPod(client, namespace, t.ClonedPod.Cmd)
	cvolume := t.ClonedPod.Volumes[0]
	tcpvc, cpvcCleanup := cvolume.SetupDynamicPersistentVolumeClaim(client, namespace, t.CSIDriver)
	for i := range cpvcCleanup {
		defer cpvcCleanup[i]()
	}
	tcpod.SetupVolume(tcpvc.persistentVolumeClaim, cvolume.VolumeMount.NameGenerate+"1", cvolume.VolumeMount.MountPathGenerate+"1", cvolume.VolumeMount.ReadOnly)

	By("deploying a second pod with a volume cloned from the original")
	tcpod.Create()
	defer tcpod.Cleanup()
	By("checking that the pods command exits with no error")
	tcpod.WaitForSuccess()
}
