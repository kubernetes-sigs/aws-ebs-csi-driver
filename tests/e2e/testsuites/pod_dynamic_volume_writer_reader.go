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
	"k8s.io/api/core/v1"
	clientset "k8s.io/client-go/kubernetes"

	. "github.com/onsi/ginkgo"
)

// PodDynamicVolumeWriterReaderTest will provision required StorageClass(es), PVC(s) and Pod(s)
// Waiting for the PV provisioner to create a new PV
// Testing if the Pod(s) can write and read to mounted volumes
type PodDynamicVolumeWriterReaderTest struct {
	CSIDriver driver.DynamicPVTestDriver
	Pods      []PodDetails
}

type PodDetails struct {
	Cmd     string
	Volumes []VolumeDetails
}

type VolumeDetails struct {
	VolumeType  string
	FSType      string
	ClaimSize   string
	VolumeMount VolumeMountDetails
}

type VolumeMountDetails struct {
	NameGenerate      string
	MountPathGenerate string
}

func (t *PodDynamicVolumeWriterReaderTest) Run(client clientset.Interface, namespace *v1.Namespace) {
	for _, pod := range t.Pods {
		tpod := NewTestPod(client, namespace, pod.Cmd)
		for n, v := range pod.Volumes {
			By("setting up the StorageClass")
			storageClass := t.CSIDriver.GetDynamicProvisionStorageClass(driver.GetParameters(v.VolumeType, v.FSType), nil, nil, namespace.Name)
			tsc := NewTestStorageClass(client, namespace, storageClass)
			createdStorageClass := tsc.Create()
			defer tsc.Cleanup()

			By("setting up the PVC and PV")
			tpvc := NewTestPersistentVolumeClaim(client, namespace, v.ClaimSize, &createdStorageClass)
			createdPVC := tpvc.Create()
			defer tpvc.Cleanup()
			tpvc.ValidateProvisionedPersistentVolume()

			tpod.SetupVolume(&createdPVC, fmt.Sprintf("%s%d", v.VolumeMount.NameGenerate, n+1), fmt.Sprintf("%s%d", v.VolumeMount.MountPathGenerate, n+1))
		}

		By("deploying the pod and checking that it's command exits with no error")
		tpod.Create()
		defer tpod.Cleanup()
	}
}
