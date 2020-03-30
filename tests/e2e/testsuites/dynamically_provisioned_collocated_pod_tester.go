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

	"k8s.io/api/core/v1"
	clientset "k8s.io/client-go/kubernetes"

	. "github.com/onsi/ginkgo"
)

// DynamicallyProvisionedCollocatedPodTest will provision required StorageClass(es), PVC(s) and Pod(s)
// Waiting for the PV provisioner to create a new PV
// Testing if multiple Pod(s) can write simultaneously
type DynamicallyProvisionedCollocatedPodTest struct {
	CSIDriver    driver.DynamicPVTestDriver
	Pods         []PodDetails
	ColocatePods bool
}

func (t *DynamicallyProvisionedCollocatedPodTest) Run(client clientset.Interface, namespace *v1.Namespace) {
	nodeName := ""
	for _, pod := range t.Pods {
		tpod, cleanup := pod.SetupWithDynamicVolumes(client, namespace, t.CSIDriver)
		if t.ColocatePods && nodeName != "" {
			tpod.SetNodeSelector(map[string]string{"name": nodeName})
		}
		// defer must be called here for resources not get removed before using them
		for i := range cleanup {
			defer cleanup[i]()
		}

		By("deploying the pod")
		tpod.Create()
		defer tpod.Cleanup()

		By("checking that the pod is running")
		tpod.WaitForRunning()
		nodeName = tpod.pod.Spec.NodeName
	}

}
