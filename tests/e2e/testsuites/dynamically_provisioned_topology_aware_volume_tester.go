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
	clientset "k8s.io/client-go/kubernetes"

	. "github.com/onsi/ginkgo/v2"
)

// DynamicallyProvisionedTopologyAwareVolumeTest will provision required StorageClass(es), PVC(s) and Pod(s)
// Waiting for the PV provisioner to create a new PV
// Testing if the Pod(s) can write and read to mounted volumes
// Validate PVs have expected PV nodeAffinity
type DynamicallyProvisionedTopologyAwareVolumeTest struct {
	CSIDriver driver.DynamicPVTestDriver
	Pods      []PodDetails
}

func (t *DynamicallyProvisionedTopologyAwareVolumeTest) Run(client clientset.Interface, namespace *v1.Namespace) {
	for _, pod := range t.Pods {
		tpod := NewTestPod(client, namespace, pod.Cmd)
		tpvcs := make([]*TestPersistentVolumeClaim, len(pod.Volumes))
		for n, v := range pod.Volumes {
			var cleanup []func()
			tpvcs[n], cleanup = v.SetupDynamicPersistentVolumeClaim(client, namespace, t.CSIDriver)
			for i := range cleanup {
				defer cleanup[i]()
			}

			tpod.SetupVolume(tpvcs[n].persistentVolumeClaim, fmt.Sprintf("%s%d", v.VolumeMount.NameGenerate, n+1), fmt.Sprintf("%s%d", v.VolumeMount.MountPathGenerate, n+1), v.VolumeMount.ReadOnly)
		}

		By("deploying the pod")
		tpod.Create()
		defer tpod.Cleanup()
		By("checking that the pods command exits with no error")
		tpod.WaitForSuccess()
		By("validating provisioned PVs")
		for n := range tpvcs {
			tpvcs[n].WaitForBound()
			tpvcs[n].ValidateProvisionedPersistentVolume()
		}
	}
}
