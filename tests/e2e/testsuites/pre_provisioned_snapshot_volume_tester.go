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
	v1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	clientset "k8s.io/client-go/kubernetes"
	restclientset "k8s.io/client-go/rest"

	awscloud "github.com/kubernetes-sigs/aws-ebs-csi-driver/pkg/cloud"

	. "github.com/onsi/ginkgo"
)

type PreProvisionedVolumeSnapshotTest struct {
	CSIDriver driver.PVTestDriver
	Pod       PodDetails
}

var (
	tCloud awscloud.Cloud
)

func (t *PreProvisionedVolumeSnapshotTest) Run(client clientset.Interface, restclient restclientset.Interface, namespace *v1.Namespace, snapshotId string) {

	By("taking snapshots")
	tvsc, cleanup := CreateVolumeSnapshotClass(restclient, namespace, t.CSIDriver)
	defer cleanup()

	tvolumeSnapshotContent := tvsc.CreateStaticVolumeSnapshotContent(snapshotId)
	tvs := tvsc.CreateStaticVolumeSnapshot(tvolumeSnapshotContent)

	defer tvsc.DeleteVolumeSnapshotContent(tvolumeSnapshotContent)
	defer tvsc.DeleteSnapshot(tvs)

	t.Pod.Volumes[0].DataSource = &DataSource{Name: tvs.Name}
	binding := storagev1.VolumeBindingWaitForFirstConsumer
	t.Pod.Volumes[0].VolumeBindingMode = &binding
	tPod := NewTestPod(client, namespace, t.Pod.Cmd)
	volume := t.Pod.Volumes[0]
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
