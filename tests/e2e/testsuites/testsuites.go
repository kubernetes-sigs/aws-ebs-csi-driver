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
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/kubernetes/test/e2e/framework"
	imageutils "k8s.io/kubernetes/test/utils/image"
)

type TestStorageClass struct {
	client       clientset.Interface
	storageClass *storagev1.StorageClass
	namespace    *v1.Namespace
}

func NewTestStorageClass(c clientset.Interface, ns *v1.Namespace, sc *storagev1.StorageClass) *TestStorageClass {
	return &TestStorageClass{
		client:       c,
		storageClass: sc,
		namespace:    ns,
	}
}

func (t *TestStorageClass) Create() storagev1.StorageClass {
	var err error

	By("creating a StorageClass " + t.storageClass.Name)
	t.storageClass, err = t.client.StorageV1().StorageClasses().Create(t.storageClass)
	framework.ExpectNoError(err)
	return *t.storageClass
}

func (t *TestStorageClass) Cleanup() {
	framework.Logf("deleting StorageClass %s", t.storageClass.Name)
	err := t.client.StorageV1().StorageClasses().Delete(t.storageClass.Name, nil)
	framework.ExpectNoError(err)
}

type TestPersistentVolumeClaim struct {
	client                         clientset.Interface
	claimSize                      string
	storageClass                   *storagev1.StorageClass
	namespace                      *v1.Namespace
	persistentVolume               *v1.PersistentVolume
	persistentVolumeClaim          *v1.PersistentVolumeClaim
	requestedPersistentVolumeClaim *v1.PersistentVolumeClaim
}

func NewTestPersistentVolumeClaim(c clientset.Interface, ns *v1.Namespace, claimSize string, sc *storagev1.StorageClass) *TestPersistentVolumeClaim {
	return &TestPersistentVolumeClaim{
		client:       c,
		claimSize:    claimSize,
		namespace:    ns,
		storageClass: sc,
	}
}

func (t *TestPersistentVolumeClaim) Create() v1.PersistentVolumeClaim {
	var err error

	By("creating a PVC")
	t.requestedPersistentVolumeClaim = generatePVC(t.namespace.Name, t.storageClass.Name, t.claimSize)
	t.persistentVolumeClaim, err = t.client.CoreV1().PersistentVolumeClaims(t.namespace.Name).Create(t.requestedPersistentVolumeClaim)
	framework.ExpectNoError(err)

	err = framework.WaitForPersistentVolumeClaimPhase(v1.ClaimBound, t.client, t.namespace.Name, t.persistentVolumeClaim.Name, framework.Poll, framework.ClaimProvisionTimeout)
	framework.ExpectNoError(err)

	By("checking the PVC")
	// Get new copy of the claim
	t.persistentVolumeClaim, err = t.client.CoreV1().PersistentVolumeClaims(t.namespace.Name).Get(t.persistentVolumeClaim.Name, metav1.GetOptions{})
	framework.ExpectNoError(err)

	return *t.persistentVolumeClaim
}

func generatePVC(namespace, storageClassName, claimSize string) *v1.PersistentVolumeClaim {
	return &v1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "pvc-",
			Namespace:    namespace,
		},
		Spec: v1.PersistentVolumeClaimSpec{
			StorageClassName: &storageClassName,
			AccessModes: []v1.PersistentVolumeAccessMode{
				v1.ReadWriteOnce,
			},
			Resources: v1.ResourceRequirements{
				Requests: v1.ResourceList{
					v1.ResourceName(v1.ResourceStorage): resource.MustParse(claimSize),
				},
			},
		},
	}
}

func (t *TestPersistentVolumeClaim) ValidateProvisionedPersistentVolume() {
	var err error

	// Get the bound PersistentVolume
	By("validating provisioned PV")
	t.persistentVolume, err = t.client.CoreV1().PersistentVolumes().Get(t.persistentVolumeClaim.Spec.VolumeName, metav1.GetOptions{})
	framework.ExpectNoError(err)

	// Check sizes
	expectedCapacity := t.requestedPersistentVolumeClaim.Spec.Resources.Requests[v1.ResourceName(v1.ResourceStorage)]
	claimCapacity := t.persistentVolumeClaim.Spec.Resources.Requests[v1.ResourceName(v1.ResourceStorage)]
	Expect(claimCapacity.Value()).To(Equal(expectedCapacity.Value()), "claimCapacity is not equal to requestedCapacity")

	pvCapacity := t.persistentVolume.Spec.Capacity[v1.ResourceName(v1.ResourceStorage)]
	Expect(pvCapacity.Value()).To(Equal(expectedCapacity.Value()), "pvCapacity is not equal to requestedCapacity")

	// Check PV properties
	By("checking the PV")
	expectedAccessModes := t.requestedPersistentVolumeClaim.Spec.AccessModes
	Expect(t.persistentVolume.Spec.AccessModes).To(Equal(expectedAccessModes))
	Expect(t.persistentVolume.Spec.ClaimRef.Name).To(Equal(t.persistentVolumeClaim.ObjectMeta.Name))
	Expect(t.persistentVolume.Spec.ClaimRef.Namespace).To(Equal(t.persistentVolumeClaim.ObjectMeta.Namespace))
	Expect(t.persistentVolume.Spec.PersistentVolumeReclaimPolicy).To(Equal(*t.storageClass.ReclaimPolicy))
	Expect(t.persistentVolume.Spec.MountOptions).To(Equal(t.storageClass.MountOptions))
}

func (t *TestPersistentVolumeClaim) Cleanup() {
	framework.Logf("deleting PVC %q/%q", t.namespace.Name, t.persistentVolumeClaim.Name)
	err := framework.DeletePersistentVolumeClaim(t.client, t.persistentVolumeClaim.Name, t.namespace.Name)
	framework.ExpectNoError(err)
	// Wait for the PV to get deleted if reclaim policy is Delete. (If it's
	// Retain, there's no use waiting because the PV won't be auto-deleted and
	// it's expected for the caller to do it.) Technically, the first few delete
	// attempts may fail, as the volume is still attached to a node because
	// kubelet is slowly cleaning up the previous pod, however it should succeed
	// in a couple of minutes.
	if t.persistentVolume.Spec.PersistentVolumeReclaimPolicy == v1.PersistentVolumeReclaimDelete {
		By(fmt.Sprintf("deleting the claim's PV %q", t.persistentVolume.Name))
		err = framework.WaitForPersistentVolumeDeleted(t.client, t.persistentVolume.Name, 5*time.Second, 10*time.Minute)
		framework.ExpectNoError(err)
	}
}

type TestPod struct {
	client    clientset.Interface
	pod       *v1.Pod
	namespace *v1.Namespace
}

func NewTestPod(c clientset.Interface, ns *v1.Namespace, command string) *TestPod {
	return &TestPod{
		client:    c,
		namespace: ns,
		pod: &v1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "pvc-volume-tester-",
			},
			Spec: v1.PodSpec{
				Containers: []v1.Container{
					{
						Name:         "volume-tester",
						Image:        imageutils.GetE2EImage(imageutils.BusyBox),
						Command:      []string{"/bin/sh"},
						Args:         []string{"-c", command},
						VolumeMounts: make([]v1.VolumeMount, 0),
					},
				},
				RestartPolicy: v1.RestartPolicyNever,
				Volumes:       make([]v1.Volume, 0),
			},
		},
	}
}

func (t *TestPod) Create() {
	var err error

	t.pod, err = t.client.CoreV1().Pods(t.namespace.Name).Create(t.pod)
	framework.ExpectNoError(err)
	err = framework.WaitForPodSuccessInNamespaceSlow(t.client, t.pod.Name, t.namespace.Name)
	framework.ExpectNoError(err)
}

func (t *TestPod) SetupVolume(pvc *v1.PersistentVolumeClaim, name, mountPath string) {
	volumeMount := v1.VolumeMount{
		Name:      name,
		MountPath: mountPath,
	}
	t.pod.Spec.Containers[0].VolumeMounts = append(t.pod.Spec.Containers[0].VolumeMounts, volumeMount)

	volume := v1.Volume{
		Name: name,
		VolumeSource: v1.VolumeSource{
			PersistentVolumeClaim: &v1.PersistentVolumeClaimVolumeSource{
				ClaimName: pvc.Name,
				ReadOnly:  false,
			},
		},
	}
	t.pod.Spec.Volumes = append(t.pod.Spec.Volumes, volume)
}

func (p *TestPod) Cleanup() {
	framework.Logf("deleting Pod %q/%q", p.namespace.Name, p.pod.Name)
	body, err := p.client.CoreV1().Pods(p.namespace.Name).GetLogs(p.pod.Name, &v1.PodLogOptions{}).Do().Raw()
	if err != nil {
		framework.Logf("Error getting logs for pod %s: %v", p.pod.Name, err)
	} else {
		framework.Logf("Pod %s has the following logs: %s", p.pod.Name, body)
	}
	framework.DeletePodOrFail(p.client, p.namespace.Name, p.pod.Name)
}
