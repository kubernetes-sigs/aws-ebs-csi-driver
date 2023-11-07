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
	"fmt"

	"github.com/kubernetes-sigs/aws-ebs-csi-driver/tests/e2e/driver"
	. "github.com/onsi/ginkgo/v2"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientset "k8s.io/client-go/kubernetes"
)

type StaticallyProvisionedMultiAttachTest struct {
	CSIDriver  driver.PreProvisionedVolumeTestDriver
	Pods       []PodDetails
	AccessMode v1.PersistentVolumeAccessMode
	VolumeMode v1.PersistentVolumeMode
	VolumeType string
	RunningPod bool
	PendingPVC bool
	VolumeID   string
}

func (t *StaticallyProvisionedMultiAttachTest) Run(client clientset.Interface, namespace *v1.Namespace) {
	tpvc, _ := t.Pods[0].Volumes[0].SetupPreProvisionedPersistentVolumeClaim(client, namespace, t.CSIDriver)

	for n, podDetail := range t.Pods {
		tpod := NewTestPod(client, namespace, "tail -f /dev/null")
		name := fmt.Sprintf("%s%d", podDetail.Volumes[0].VolumeDevice.NameGenerate, n+1)
		devicePath := podDetail.Volumes[0].VolumeDevice.DevicePath

		tpod.SetupRawBlockVolume(tpvc.persistentVolumeClaim, name, devicePath)
		tpod.pod.ObjectMeta.Labels = map[string]string{"app": "my-service"}
		tpod.pod.Spec.TopologySpreadConstraints = []v1.TopologySpreadConstraint{
			{
				MaxSkew:           1,
				TopologyKey:       "kubernetes.io/hostname",
				WhenUnsatisfiable: v1.DoNotSchedule,
				LabelSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{"app": "my-service"},
				},
			},
		}

		By("deploying the pod")
		tpod.Create()
		defer tpod.Cleanup()

		By("checking that the pods command exits with no error")
		tpod.WaitForRunning()
	}
}
