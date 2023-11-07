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
	"context"
	"fmt"
	"time"

	"github.com/kubernetes-sigs/aws-ebs-csi-driver/tests/e2e/driver"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientset "k8s.io/client-go/kubernetes"
	e2epod "k8s.io/kubernetes/test/e2e/framework/pod"
	e2epv "k8s.io/kubernetes/test/e2e/framework/pv"
)

type DynamicallyProvisionedMultiAttachTest struct {
	CSIDriver  driver.DynamicPVTestDriver
	Pods       []PodDetails
	VolumeMode VolumeMode
	AccessMode v1.PersistentVolumeAccessMode
	VolumeType string
	RunningPod bool
	PendingPVC bool
}

func (t *DynamicallyProvisionedMultiAttachTest) Run(client clientset.Interface, namespace *v1.Namespace) {
	// Setup StorageClass and PVC
	tpvc, _ := t.Pods[0].Volumes[0].SetupDynamicPersistentVolumeClaim(client, namespace, t.CSIDriver)
	defer tpvc.Cleanup()

	for n, podDetail := range t.Pods {
		tpod := NewTestPod(client, namespace, "tail -f /dev/null")

		if podDetail.Volumes[0].VolumeMode == Block {
			name := fmt.Sprintf("%s%d", podDetail.Volumes[0].VolumeDevice.NameGenerate, n+1)
			devicePath := podDetail.Volumes[0].VolumeDevice.DevicePath
			tpod.SetupRawBlockVolume(tpvc.persistentVolumeClaim, name, devicePath)
		} else {
			name := fmt.Sprintf("%s%d", podDetail.Volumes[0].VolumeMount.NameGenerate, n+1)
			mountPath := fmt.Sprintf("%s%d", podDetail.Volumes[0].VolumeMount.MountPathGenerate, n+1)
			readOnly := podDetail.Volumes[0].VolumeMount.ReadOnly
			tpod.SetupVolume(tpvc.persistentVolumeClaim, name, mountPath, readOnly)
		}

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

		if t.PendingPVC {
			By("checking that the PVC is not bound")
			pvcList := []*v1.PersistentVolumeClaim{tpvc.persistentVolumeClaim}
			_, err := e2epv.WaitForPVClaimBoundPhase(context.Background(), client, pvcList, 30*time.Second)
			Expect(err).To(HaveOccurred(), "Failed to wait for PVC to be in Pending state")
			return
		}
		if t.RunningPod || n == 0 {
			By("checking that the pod is running")
			tpod.WaitForRunning()
		} else {
			By("checking that the pod is not running")
			err := e2epod.WaitTimeoutForPodRunningInNamespace(context.Background(), client, tpod.pod.Name, namespace.Name, 30*time.Second)
			Expect(err).To(HaveOccurred(), "Failed to wait for pod to be in a running state")
		}
	}
}
