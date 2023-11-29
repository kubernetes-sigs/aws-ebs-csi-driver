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
	"fmt"

	"github.com/kubernetes-sigs/aws-ebs-csi-driver/tests/e2e/driver"
	v1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	clientset "k8s.io/client-go/kubernetes"
	k8srestclient "k8s.io/client-go/rest"
	"k8s.io/kubernetes/test/e2e/framework"

	. "github.com/onsi/ginkgo/v2"
)

type PreProvisionedVolumeSnapshotTest struct {
	CSIDriver driver.PVTestDriver
	Pod       PodDetails
}

func (t *PreProvisionedVolumeSnapshotTest) Run(client clientset.Interface, restclient k8srestclient.Interface, namespace *v1.Namespace, snapshotId string) {

	By("taking snapshots")
	tvsc, cleanup := CreateVolumeSnapshotClass(restclient, namespace, t.CSIDriver, nil)
	defer cleanup()

	tvolumeSnapshotContent := tvsc.CreateStaticVolumeSnapshotContent(snapshotId)
	tvs := tvsc.CreateStaticVolumeSnapshot(tvolumeSnapshotContent)

	defer tvsc.DeleteVolumeSnapshotContent(tvolumeSnapshotContent)
	defer tvsc.DeleteSnapshot(tvs)
	if len(t.Pod.Volumes) < 1 {
		err := fmt.Errorf("Volume is not setup for testing pod, exit. ")
		framework.ExpectNoError(err)
	}

	volume := t.Pod.Volumes[0]
	volume.DataSource = &DataSource{Name: tvs.Name}
	binding := storagev1.VolumeBindingWaitForFirstConsumer
	volume.VolumeBindingMode = &binding
	tPod := NewTestPod(client, namespace, t.Pod.Cmd)
	tpvc, pvcCleanup := volume.SetupDynamicPersistentVolumeClaim(client, namespace, t.CSIDriver)
	for i := range pvcCleanup {
		defer pvcCleanup[i]()
	}
	tPod.SetupVolume(tpvc.persistentVolumeClaim, volume.VolumeMount.NameGenerate+"1", volume.VolumeMount.MountPathGenerate+"1", volume.VolumeMount.ReadOnly)
	By("deploying a second pod with a volume restored from the snapshot")
	tPod.Create()
	defer tPod.Cleanup()
	By("checking that the pods command exits with no error")
	tPod.WaitForSuccess()
}
