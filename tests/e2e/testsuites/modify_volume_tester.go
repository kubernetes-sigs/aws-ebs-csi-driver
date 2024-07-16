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

	"github.com/kubernetes-sigs/aws-ebs-csi-driver/tests/e2e/driver"
	. "github.com/onsi/ginkgo/v2"
	v1 "k8s.io/api/core/v1"
	"k8s.io/api/storage/v1alpha1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/kubernetes/test/e2e/framework"
)

// ModifyVolumeTest will provision pod with attached volume, and test that modifying its pvc will modify the associated pv.
type ModifyVolumeTest struct {
	CreateVolumeParameters                map[string]string
	ModifyVolumeAnnotations               map[string]string
	ShouldResizeVolume                    bool
	ShouldTestInvalidModificationRecovery bool
	ExternalResizerOnly                   bool
}

var (
	invalidAnnotations = map[string]string{
		AnnotationIops: "1",
	}
	volumeSize = "10Gi" // Different from driver.MinimumSizeForVolumeType to simplify iops, throughput, volumeType modification
)

type ModifyTestType int64

const (
	VolumeModifierForK8s ModifyTestType = iota
	ExternalResizer
)

func (modifyVolumeTest *ModifyVolumeTest) Run(c clientset.Interface, ns *v1.Namespace, ebsDriver driver.PVTestDriver, testType ModifyTestType) {
	By("setting up pvc")
	volumeDetails := CreateVolumeDetails(modifyVolumeTest.CreateVolumeParameters, volumeSize)
	testVolume, _ := volumeDetails.SetupDynamicPersistentVolumeClaim(c, ns, ebsDriver)
	defer testVolume.Cleanup()

	By("deploying pod continuously writing to volume")
	formatOptionMountPod := createPodWithVolume(c, ns, PodCmdContinuousWrite(DefaultMountPath), testVolume, volumeDetails)
	defer formatOptionMountPod.Cleanup()
	formatOptionMountPod.WaitForRunning()

	if modifyVolumeTest.ShouldTestInvalidModificationRecovery {
		By("modifying the pvc with invalid annotations")
		attemptInvalidModification(c, ns, testVolume)
	}

	By("modifying the pvc")
	modifyingPvc, _ := c.CoreV1().PersistentVolumeClaims(ns.Name).Get(context.TODO(), testVolume.persistentVolumeClaim.Name, metav1.GetOptions{})
	if testType == VolumeModifierForK8s {
		AnnotatePvc(modifyingPvc, modifyVolumeTest.ModifyVolumeAnnotations)
	} else if testType == ExternalResizer {
		vac, err := c.StorageV1alpha1().VolumeAttributesClasses().Create(context.Background(), &v1alpha1.VolumeAttributesClass{
			ObjectMeta: metav1.ObjectMeta{
				Name:      formatOptionMountPod.pod.Name,
				Namespace: ns.Name,
			},
			DriverName: "ebs.csi.aws.com",
			Parameters: modifyVolumeTest.ModifyVolumeAnnotations,
		}, metav1.CreateOptions{})
		framework.ExpectNoError(err)

		vacName := vac.Name
		modifyingPvc.Spec.VolumeAttributesClassName = &vacName
	}

	var updatedPvcSize resource.Quantity
	if modifyVolumeTest.ShouldResizeVolume {
		By("resizing the pvc")
		updatedPvcSize = IncreasePvcObjectStorage(modifyingPvc, DefaultSizeIncreaseGi)
	}

	modifiedPvc, err := c.CoreV1().PersistentVolumeClaims(ns.Name).Update(context.TODO(), modifyingPvc, metav1.UpdateOptions{})
	if err != nil {
		framework.ExpectNoError(err, fmt.Sprintf("fail to modify pvc(%s): %v", modifyingPvc.Name, err))
	}
	framework.Logf("updated pvc: %s\n", modifiedPvc.Annotations)

	By("wait for and confirm pv modification")
	if testType == VolumeModifierForK8s {
		err = WaitForPvToModify(c, ns, testVolume.persistentVolume.Name, modifyVolumeTest.ModifyVolumeAnnotations, DefaultModificationTimeout, DefaultK8sApiPollingInterval)
	} else if testType == ExternalResizer {
		err = WaitForVacToApplyToPv(c, ns, testVolume.persistentVolume.Name, *modifyingPvc.Spec.VolumeAttributesClassName, DefaultModificationTimeout, DefaultK8sApiPollingInterval)
	}
	framework.ExpectNoError(err, fmt.Sprintf("fail to modify pv(%s): %v", modifyingPvc.Name, err))
	if modifyVolumeTest.ShouldResizeVolume {
		err = WaitForPvToResize(c, ns, testVolume.persistentVolume.Name, updatedPvcSize, DefaultResizeTimout, DefaultK8sApiPollingInterval)
		framework.ExpectNoError(err, fmt.Sprintf("fail to resize pv(%s): %v", modifyingPvc.Name, err))
	}
}

func attemptInvalidModification(c clientset.Interface, ns *v1.Namespace, testVolume *TestPersistentVolumeClaim) {
	modifyingPvc, _ := c.CoreV1().PersistentVolumeClaims(ns.Name).Get(context.TODO(), testVolume.persistentVolumeClaim.Name, metav1.GetOptions{})
	AnnotatePvc(modifyingPvc, invalidAnnotations)
	modifiedPvc, err := c.CoreV1().PersistentVolumeClaims(ns.Name).Update(context.TODO(), modifyingPvc, metav1.UpdateOptions{})
	if err != nil {
		framework.ExpectNoError(err, fmt.Sprintf("fail to modify pvc(%s): %v", modifyingPvc.Name, err))
	}
	framework.Logf("pvc %q/%q has been modified with invalid annotations: %s", ns.Name, modifiedPvc.Name, modifiedPvc.Annotations)
}
