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

	volumesnapshotv1 "github.com/kubernetes-csi/external-snapshotter/client/v4/apis/volumesnapshot/v1"
	v1 "k8s.io/api/core/v1"
	clientset "k8s.io/client-go/kubernetes"
	restclientset "k8s.io/client-go/rest"

	. "github.com/onsi/ginkgo/v2"
)

// DynamicallyProvisionedVolumeSnapshotTest will provision required StorageClass(es),VolumeSnapshotClass(es), PVC(s) and Pod(s)
// Waiting for the PV provisioner to create a new PV
// Testing if the Pod(s) can write and read to mounted volumes
// Create a snapshot, validate the data is still on the disk, and then write and read to it again
// And finally delete the snapshot
// This test only supports a single volume
type DynamicallyProvisionedVolumeSnapshotTest struct {
	CSIDriver    driver.PVTestDriver
	Pod          PodDetails
	RestoredPod  PodDetails
	Parameters   map[string]string
	ValidateFunc func(*volumesnapshotv1.VolumeSnapshot)
}

func (t *DynamicallyProvisionedVolumeSnapshotTest) Run(client clientset.Interface, restclient restclientset.Interface, namespace *v1.Namespace) {
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

	By("taking snapshots")
	tvsc, cleanup := CreateVolumeSnapshotClass(restclient, namespace, t.CSIDriver, t.Parameters)
	defer cleanup()

	snapshot := tvsc.CreateSnapshot(tpvc.persistentVolumeClaim)
	defer tvsc.DeleteSnapshot(snapshot)
	tvsc.ReadyToUse(snapshot)

	t.RestoredPod.Volumes[0].DataSource = &DataSource{Name: snapshot.Name}
	trpod := NewTestPod(client, namespace, t.RestoredPod.Cmd)
	rvolume := t.RestoredPod.Volumes[0]
	trpvc, rpvcCleanup := rvolume.SetupDynamicPersistentVolumeClaim(client, namespace, t.CSIDriver)
	for i := range rpvcCleanup {
		defer rpvcCleanup[i]()
	}
	trpod.SetupVolume(trpvc.persistentVolumeClaim, rvolume.VolumeMount.NameGenerate+"1", rvolume.VolumeMount.MountPathGenerate+"1", rvolume.VolumeMount.ReadOnly)

	By("deploying a second pod with a volume restored from the snapshot")
	trpod.Create()
	defer trpod.Cleanup()
	By("checking that the pods command exits with no error")
	trpod.WaitForSuccess()

	if t.ValidateFunc != nil {
		t.ValidateFunc(snapshot)
	}
}
