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
	ebscsidriver "github.com/kubernetes-sigs/aws-ebs-csi-driver/pkg/driver"
	"github.com/kubernetes-sigs/aws-ebs-csi-driver/tests/e2e/driver"
	. "github.com/onsi/ginkgo/v2"
	v1 "k8s.io/api/core/v1"
	clientset "k8s.io/client-go/kubernetes"
)

// FormatOptionTest will provision required StorageClass(es), PVC(s) and Pod(s) in order to test that volumes with
// a specified custom format options will mount and are able to be later resized.
type FormatOptionTest struct {
	CreateVolumeParameters map[string]string
}

func (t *FormatOptionTest) Run(client clientset.Interface, namespace *v1.Namespace, ebsDriver driver.PVTestDriver) {
	By("setting up pvc with custom format option")
	volumeDetails := CreateVolumeDetails(t.CreateVolumeParameters, driver.MinimumSizeForVolumeType(t.CreateVolumeParameters[ebscsidriver.VolumeTypeKey]))
	testPvc, _ := volumeDetails.SetupDynamicPersistentVolumeClaim(client, namespace, ebsDriver)
	defer testPvc.Cleanup()

	By("deploying pod with custom format option")
	formatOptionMountPod := createPodWithVolume(client, namespace, PodCmdWriteToVolume(DefaultMountPath), testPvc, volumeDetails)
	defer formatOptionMountPod.Cleanup()
	formatOptionMountPod.WaitForSuccess()

	By("testing that pvc is able to be resized")
	ResizeTestPvc(client, namespace, testPvc, DefaultSizeIncreaseGi)

	By("validating resized pvc by deploying new pod")
	resizeTestPod := createPodWithVolume(client, namespace, PodCmdWriteToVolume(DefaultMountPath), testPvc, volumeDetails)
	defer resizeTestPod.Cleanup()

	By("confirming new pod can write to resized volume")
	resizeTestPod.WaitForSuccess()
}

func createPodWithVolume(client clientset.Interface, namespace *v1.Namespace, cmd string, testPvc *TestPersistentVolumeClaim, volumeDetails *VolumeDetails) *TestPod {
	testPod := NewTestPod(client, namespace, cmd)
	testPod.SetupVolume(testPvc.persistentVolumeClaim, volumeDetails.VolumeMount.NameGenerate, volumeDetails.VolumeMount.MountPathGenerate, volumeDetails.VolumeMount.ReadOnly)
	testPod.Create()

	return testPod
}
