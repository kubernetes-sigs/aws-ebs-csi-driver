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
	"context"
	"fmt"
	"math/rand"
	"time"

	"github.com/aws/aws-sdk-go/aws"

	volumesnapshotv1 "github.com/kubernetes-csi/external-snapshotter/client/v4/apis/volumesnapshot/v1"
	snapshotclientset "github.com/kubernetes-csi/external-snapshotter/client/v4/clientset/versioned"
	awscloud "github.com/kubernetes-sigs/aws-ebs-csi-driver/pkg/cloud"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	apps "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	clientset "k8s.io/client-go/kubernetes"
	restclientset "k8s.io/client-go/rest"
	"k8s.io/kubernetes/test/e2e/framework"
	e2edeployment "k8s.io/kubernetes/test/e2e/framework/deployment"
	e2elog "k8s.io/kubernetes/test/e2e/framework/log"
	e2epod "k8s.io/kubernetes/test/e2e/framework/pod"
	e2epv "k8s.io/kubernetes/test/e2e/framework/pv"
	imageutils "k8s.io/kubernetes/test/utils/image"
)

const (
	execTimeout = 10 * time.Second
	// Some pods can take much longer to get ready due to volume attach/detach latency.
	slowPodStartTimeout = 15 * time.Minute
	// Description that will printed during tests
	failedConditionDescription = "Error status code"

	volumeSnapshotNameStatic         = "volume-snapshot-tester"
	volumeSnapshotContenetNameStatic = "volume-snapshot-content-tester"
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
	t.storageClass, err = t.client.StorageV1().StorageClasses().Create(context.TODO(), t.storageClass, metav1.CreateOptions{})
	framework.ExpectNoError(err)
	return *t.storageClass
}

func (t *TestStorageClass) Cleanup() {
	e2elog.Logf("deleting StorageClass %s", t.storageClass.Name)
	err := t.client.StorageV1().StorageClasses().Delete(context.TODO(), t.storageClass.Name, metav1.DeleteOptions{})
	framework.ExpectNoError(err)
}

type TestVolumeSnapshotClass struct {
	client              restclientset.Interface
	volumeSnapshotClass *volumesnapshotv1.VolumeSnapshotClass
	namespace           *v1.Namespace
}

func NewTestVolumeSnapshotClass(c restclientset.Interface, ns *v1.Namespace, vsc *volumesnapshotv1.VolumeSnapshotClass) *TestVolumeSnapshotClass {
	return &TestVolumeSnapshotClass{
		client:              c,
		volumeSnapshotClass: vsc,
		namespace:           ns,
	}
}

func (t *TestVolumeSnapshotClass) Create() {
	By("creating a VolumeSnapshotClass")
	var err error
	t.volumeSnapshotClass, err = snapshotclientset.New(t.client).SnapshotV1().VolumeSnapshotClasses().Create(context.TODO(), t.volumeSnapshotClass, metav1.CreateOptions{})
	framework.ExpectNoError(err)
}

func (t *TestVolumeSnapshotClass) CreateSnapshot(pvc *v1.PersistentVolumeClaim) *volumesnapshotv1.VolumeSnapshot {
	By("creating a VolumeSnapshot for " + pvc.Name)
	snapshot := &volumesnapshotv1.VolumeSnapshot{
		TypeMeta: metav1.TypeMeta{
			Kind:       VolumeSnapshotKind,
			APIVersion: SnapshotAPIVersion,
		},
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "volume-snapshot-",
			Namespace:    t.namespace.Name,
		},
		Spec: volumesnapshotv1.VolumeSnapshotSpec{
			VolumeSnapshotClassName: &t.volumeSnapshotClass.Name,
			Source: volumesnapshotv1.VolumeSnapshotSource{
				PersistentVolumeClaimName: &pvc.Name,
			},
		},
	}
	snapshot, err := snapshotclientset.New(t.client).SnapshotV1().VolumeSnapshots(t.namespace.Name).Create(context.TODO(), snapshot, metav1.CreateOptions{})
	framework.ExpectNoError(err)
	return snapshot
}

func (t *TestVolumeSnapshotClass) CreateStaticVolumeSnapshot(vsc *volumesnapshotv1.VolumeSnapshotContent) *volumesnapshotv1.VolumeSnapshot {
	By("creating a VolumeSnapshot from vsc " + vsc.Name)
	snapshot := &volumesnapshotv1.VolumeSnapshot{
		TypeMeta: metav1.TypeMeta{
			Kind:       VolumeSnapshotKind,
			APIVersion: SnapshotAPIVersion,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      volumeSnapshotNameStatic,
			Namespace: t.namespace.Name,
		},
		Spec: volumesnapshotv1.VolumeSnapshotSpec{
			VolumeSnapshotClassName: &t.volumeSnapshotClass.Name,
			Source: volumesnapshotv1.VolumeSnapshotSource{
				VolumeSnapshotContentName: &vsc.Name,
			},
		},
	}
	snapshotObj, err := snapshotclientset.New(t.client).SnapshotV1().VolumeSnapshots(t.namespace.Name).Create(context.TODO(), snapshot, metav1.CreateOptions{})
	framework.ExpectNoError(err)
	return snapshotObj
}

func (t *TestVolumeSnapshotClass) CreateStaticVolumeSnapshotContent(snapshotId string) *volumesnapshotv1.VolumeSnapshotContent {
	By("creating a VolumeSnapshotContent from snapshotId: " + snapshotId)
	snapshotContent := &volumesnapshotv1.VolumeSnapshotContent{
		TypeMeta: metav1.TypeMeta{
			Kind:       VolumeSnapshotContentKind,
			APIVersion: SnapshotAPIVersion,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      volumeSnapshotContenetNameStatic,
			Namespace: t.namespace.Name,
		},
		Spec: volumesnapshotv1.VolumeSnapshotContentSpec{
			VolumeSnapshotClassName: &t.volumeSnapshotClass.Name,
			DeletionPolicy:          "Delete",
			VolumeSnapshotRef: v1.ObjectReference{
				Kind:      VolumeSnapshotKind,
				Name:      volumeSnapshotNameStatic,
				Namespace: t.namespace.Name,
			},
			Driver: "ebs.csi.aws.com",
			Source: volumesnapshotv1.VolumeSnapshotContentSource{
				SnapshotHandle: aws.String(snapshotId),
			},
		},
	}
	volumeSnapshotContent, err := snapshotclientset.New(t.client).SnapshotV1().VolumeSnapshotContents().Create(context.TODO(), snapshotContent, metav1.CreateOptions{})
	framework.ExpectNoError(err)
	return volumeSnapshotContent
}

func (t *TestVolumeSnapshotClass) UpdateStaticVolumeSnapshotContent(volumeSnapshot *volumesnapshotv1.VolumeSnapshot, volumeSnapshotContent *volumesnapshotv1.VolumeSnapshotContent) {
	volumeSnapshotContent.Spec.VolumeSnapshotRef.Name = volumeSnapshot.Name
	_, err := snapshotclientset.New(t.client).SnapshotV1().VolumeSnapshotContents().Update(context.TODO(), volumeSnapshotContent, metav1.UpdateOptions{})
	framework.ExpectNoError(err)

}
func (t *TestVolumeSnapshotClass) ReadyToUse(snapshot *volumesnapshotv1.VolumeSnapshot) {
	By("waiting for VolumeSnapshot to be ready to use - " + snapshot.Name)
	err := wait.Poll(15*time.Second, 5*time.Minute, func() (bool, error) {
		vs, err := snapshotclientset.New(t.client).SnapshotV1().VolumeSnapshots(t.namespace.Name).Get(context.TODO(), snapshot.Name, metav1.GetOptions{})
		if err != nil {
			return false, fmt.Errorf("did not see ReadyToUse: %w", err)
		}

		if vs.Status == nil || vs.Status.ReadyToUse == nil {
			return false, nil
		}
		return *vs.Status.ReadyToUse, nil
	})
	framework.ExpectNoError(err)
}

func (t *TestVolumeSnapshotClass) DeleteSnapshot(vs *volumesnapshotv1.VolumeSnapshot) {
	By("deleting a VolumeSnapshot " + vs.Name)
	err := snapshotclientset.New(t.client).SnapshotV1().VolumeSnapshots(t.namespace.Name).Delete(context.TODO(), vs.Name, metav1.DeleteOptions{})
	framework.ExpectNoError(err)

	err = t.waitForSnapshotDeleted(t.namespace.Name, vs.Name, 5*time.Second, 5*time.Minute)
	framework.ExpectNoError(err)
}

func (t *TestVolumeSnapshotClass) DeleteVolumeSnapshotContent(vsc *volumesnapshotv1.VolumeSnapshotContent) {
	By("deleting a VolumeSnapshotContent " + vsc.Name)
	snapshotclientset.New(t.client).SnapshotV1().VolumeSnapshotContents().Delete(context.TODO(), vsc.Name, metav1.DeleteOptions{}) //nolint
	err := t.waitForVolumeSnapshotContentDeleted(vsc.Name, 5*time.Second, 5*time.Minute)
	framework.ExpectNoError(err)
}

func (t *TestVolumeSnapshotClass) Cleanup() {
	e2elog.Logf("deleting VolumeSnapshotClass %s", t.volumeSnapshotClass.Name)
	err := snapshotclientset.New(t.client).SnapshotV1().VolumeSnapshotClasses().Delete(context.TODO(), t.volumeSnapshotClass.Name, metav1.DeleteOptions{})
	framework.ExpectNoError(err)
}

func (t *TestVolumeSnapshotClass) waitForSnapshotDeleted(ns string, snapshotName string, poll, timeout time.Duration) error {
	e2elog.Logf("Waiting up to %v for VolumeSnapshot %s to be removed", timeout, snapshotName)
	c := snapshotclientset.New(t.client).SnapshotV1()
	for start := time.Now(); time.Since(start) < timeout; time.Sleep(poll) {
		_, err := c.VolumeSnapshots(ns).Get(context.TODO(), snapshotName, metav1.GetOptions{})
		if err != nil {
			if apierrs.IsNotFound(err) {
				e2elog.Logf("Snapshot %q in namespace %q doesn't exist in the system", snapshotName, ns)
				return nil
			}
			e2elog.Logf("Failed to get snapshot %q in namespace %q, retrying in %v. Error: %v", snapshotName, ns, poll, err)
		}
	}
	return fmt.Errorf("VolumeSnapshot %s is not removed from the system within %v", snapshotName, timeout)
}

func (t *TestVolumeSnapshotClass) waitForVolumeSnapshotContentDeleted(vscName string, poll, timeout time.Duration) error {
	e2elog.Logf("Waiting up to %v for VolumeSnapshotContent %s to be removed", timeout, vscName)
	c := snapshotclientset.New(t.client).SnapshotV1()
	for start := time.Now(); time.Since(start) < timeout; time.Sleep(poll) {
		_, err := c.VolumeSnapshotContents().Get(context.TODO(), vscName, metav1.GetOptions{})
		if err != nil {
			if apierrs.IsNotFound(err) {
				e2elog.Logf("VolumeSnapshotContent %q doesn't exist in the system", vscName)
				return nil
			}
			e2elog.Logf("Failed to get VolumeSnapshotContent %q, retrying in %v. Error: %v", vscName, poll, err)
		}
	}
	return fmt.Errorf("VolumeSnapshot %s is not removed from the system within %v", vscName, timeout)
}

type TestPreProvisionedPersistentVolume struct {
	client                    clientset.Interface
	persistentVolume          *v1.PersistentVolume
	requestedPersistentVolume *v1.PersistentVolume
}

func NewTestPreProvisionedPersistentVolume(c clientset.Interface, pv *v1.PersistentVolume) *TestPreProvisionedPersistentVolume {
	return &TestPreProvisionedPersistentVolume{
		client:                    c,
		requestedPersistentVolume: pv,
	}
}

func (pv *TestPreProvisionedPersistentVolume) Create() v1.PersistentVolume {
	var err error
	By("creating a PV")
	pv.persistentVolume, err = pv.client.CoreV1().PersistentVolumes().Create(context.TODO(), pv.requestedPersistentVolume, metav1.CreateOptions{})
	framework.ExpectNoError(err)
	return *pv.persistentVolume
}

type TestPersistentVolumeClaim struct {
	client                         clientset.Interface
	claimSize                      string
	volumeMode                     v1.PersistentVolumeMode
	storageClass                   *storagev1.StorageClass
	namespace                      *v1.Namespace
	persistentVolume               *v1.PersistentVolume
	persistentVolumeClaim          *v1.PersistentVolumeClaim
	requestedPersistentVolumeClaim *v1.PersistentVolumeClaim
	dataSource                     *v1.TypedLocalObjectReference
}

func NewTestPersistentVolumeClaim(c clientset.Interface, ns *v1.Namespace, claimSize string, volumeMode VolumeMode, sc *storagev1.StorageClass) *TestPersistentVolumeClaim {
	mode := v1.PersistentVolumeFilesystem
	if volumeMode == Block {
		mode = v1.PersistentVolumeBlock
	}
	return &TestPersistentVolumeClaim{
		client:       c,
		claimSize:    claimSize,
		volumeMode:   mode,
		namespace:    ns,
		storageClass: sc,
	}
}

func NewTestPersistentVolumeClaimWithDataSource(c clientset.Interface, ns *v1.Namespace, claimSize string, volumeMode VolumeMode, sc *storagev1.StorageClass, dataSource *v1.TypedLocalObjectReference) *TestPersistentVolumeClaim {
	mode := v1.PersistentVolumeFilesystem
	if volumeMode == Block {
		mode = v1.PersistentVolumeBlock
	}
	By("Create tpvc with data source")
	return &TestPersistentVolumeClaim{
		client:       c,
		claimSize:    claimSize,
		volumeMode:   mode,
		namespace:    ns,
		storageClass: sc,
		dataSource:   dataSource,
	}
}

func (t *TestPersistentVolumeClaim) Create() {
	var err error

	By("creating a PVC")
	storageClassName := ""
	if t.storageClass != nil {
		storageClassName = t.storageClass.Name
	}
	t.requestedPersistentVolumeClaim = generatePVC(t.namespace.Name, storageClassName, t.claimSize, t.volumeMode, t.dataSource)
	t.persistentVolumeClaim, err = t.client.CoreV1().PersistentVolumeClaims(t.namespace.Name).Create(context.TODO(), t.requestedPersistentVolumeClaim, metav1.CreateOptions{})
	framework.ExpectNoError(err)
}

func (t *TestPersistentVolumeClaim) ValidateProvisionedPersistentVolume() {
	var err error

	// Get the bound PersistentVolume
	By("validating provisioned PV")
	t.persistentVolume, err = t.client.CoreV1().PersistentVolumes().Get(context.TODO(), t.persistentVolumeClaim.Spec.VolumeName, metav1.GetOptions{})
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
	// If storageClass is nil, PV was pre-provisioned with these values already set
	if t.storageClass != nil {
		Expect(t.persistentVolume.Spec.PersistentVolumeReclaimPolicy).To(Equal(*t.storageClass.ReclaimPolicy))
		Expect(t.persistentVolume.Spec.MountOptions).To(Equal(t.storageClass.MountOptions))
		if *t.storageClass.VolumeBindingMode == storagev1.VolumeBindingWaitForFirstConsumer {
			Expect(t.persistentVolume.Spec.NodeAffinity.Required.NodeSelectorTerms[0].MatchExpressions[0].Values).
				To(HaveLen(1))
		}
		if len(t.storageClass.AllowedTopologies) > 0 {
			// Since we're chaging our topology key, assume we have the values below to compare:
			// NodeSelectorTerms: [{[{topology.ebs.csi.aws.com/zone In [us-west-2a]} {topology.kubernetes.io/zone In [us-west-2a]}] []}]
			// AllowedTopologies: [{[{topology.ebs.csi.aws.com/zone [us-west-2a us-west-2b us-west-2c]}]}]
			// As you can see tests might fail depending on the ordering of the NodeSelectorTerms. That's why we're doing this "hack".
			// This is a quick fix to unblock the PRs we have. We really need to improve this. TODO

			keyFound := false
			for _, v := range t.persistentVolume.Spec.NodeAffinity.Required.NodeSelectorTerms[0].MatchExpressions {
				if v.Key == "topology.ebs.csi.aws.com/zone" {
					keyFound = true
					Expect(v.Key).To(Equal(t.storageClass.AllowedTopologies[0].MatchLabelExpressions[0].Key))
				}
			}

			// additional sanity check so we can catch an unintended test case that'd hide failures
			if !keyFound {
				Fail("Volume is expected to have a node selector term.")
			}

			for _, v := range t.persistentVolume.Spec.NodeAffinity.Required.NodeSelectorTerms[0].MatchExpressions[0].Values {
				Expect(t.storageClass.AllowedTopologies[0].MatchLabelExpressions[0].Values).To(ContainElement(v))
			}

		}
	}
}

func (t *TestPersistentVolumeClaim) WaitForBound() v1.PersistentVolumeClaim {
	var err error

	By(fmt.Sprintf("waiting for PVC to be in phase %q", v1.ClaimBound))
	err = e2epv.WaitForPersistentVolumeClaimPhase(v1.ClaimBound, t.client, t.namespace.Name, t.persistentVolumeClaim.Name, framework.Poll, framework.ClaimProvisionTimeout)
	framework.ExpectNoError(err)

	By("checking the PVC")
	// Get new copy of the claim
	t.persistentVolumeClaim, err = t.client.CoreV1().PersistentVolumeClaims(t.namespace.Name).Get(context.TODO(), t.persistentVolumeClaim.Name, metav1.GetOptions{})
	framework.ExpectNoError(err)

	return *t.persistentVolumeClaim
}

func generatePVC(namespace, storageClassName, claimSize string, volumeMode v1.PersistentVolumeMode, dataSource *v1.TypedLocalObjectReference) *v1.PersistentVolumeClaim {
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
			VolumeMode: &volumeMode,
			DataSource: dataSource,
		},
	}
}

func (t *TestPersistentVolumeClaim) Cleanup() {
	e2elog.Logf("deleting PVC %q/%q", t.namespace.Name, t.persistentVolumeClaim.Name)
	err := e2epv.DeletePersistentVolumeClaim(t.client, t.persistentVolumeClaim.Name, t.namespace.Name)
	framework.ExpectNoError(err)
	// Wait for the PV to get deleted if reclaim policy is Delete. (If it's
	// Retain, there's no use waiting because the PV won't be auto-deleted and
	// it's expected for the caller to do it.) Technically, the first few delete
	// attempts may fail, as the volume is still attached to a node because
	// kubelet is slowly cleaning up the previous pod, however it should succeed
	// in a couple of minutes.
	if t.persistentVolume != nil && t.persistentVolume.Spec.PersistentVolumeReclaimPolicy == v1.PersistentVolumeReclaimDelete {
		By(fmt.Sprintf("waiting for claim's PV %q to be deleted", t.persistentVolume.Name))
		err = e2epv.WaitForPersistentVolumeDeleted(t.client, t.persistentVolume.Name, 5*time.Second, 10*time.Minute)
		framework.ExpectNoError(err)
	}
	// Wait for the PVC to be deleted
	err = waitForPersistentVolumeClaimDeleted(t.client, t.namespace.Name, t.persistentVolumeClaim.Name, 5*time.Second, 5*time.Minute)
	framework.ExpectNoError(err)
}

func (t *TestPersistentVolumeClaim) ReclaimPolicy() v1.PersistentVolumeReclaimPolicy {
	return t.persistentVolume.Spec.PersistentVolumeReclaimPolicy
}

func (t *TestPersistentVolumeClaim) WaitForPersistentVolumePhase(phase v1.PersistentVolumePhase) {
	err := e2epv.WaitForPersistentVolumePhase(phase, t.client, t.persistentVolume.Name, 5*time.Second, 10*time.Minute)
	framework.ExpectNoError(err)
}

func (t *TestPersistentVolumeClaim) DeleteBoundPersistentVolume() {
	By(fmt.Sprintf("deleting PV %q", t.persistentVolume.Name))
	err := e2epv.DeletePersistentVolume(t.client, t.persistentVolume.Name)
	framework.ExpectNoError(err)
	By(fmt.Sprintf("waiting for claim's PV %q to be deleted", t.persistentVolume.Name))
	err = e2epv.WaitForPersistentVolumeDeleted(t.client, t.persistentVolume.Name, 5*time.Second, 10*time.Minute)
	framework.ExpectNoError(err)
}

func (t *TestPersistentVolumeClaim) DeleteBackingVolume(cloud awscloud.Cloud) {
	volumeID := t.persistentVolume.Spec.CSI.VolumeHandle
	By(fmt.Sprintf("deleting EBS volume %q", volumeID))
	ok, err := cloud.DeleteDisk(context.Background(), volumeID)
	if err != nil || !ok {
		Fail(fmt.Sprintf("could not delete volume %q: %v", volumeID, err))
	}
}

type TestDeployment struct {
	client     clientset.Interface
	deployment *apps.Deployment
	namespace  *v1.Namespace
	podName    string
}

func NewTestDeployment(c clientset.Interface, ns *v1.Namespace, command string, pvc *v1.PersistentVolumeClaim, volumeName, mountPath string, readOnly bool) *TestDeployment {
	generateName := "ebs-volume-tester-"
	selectorValue := fmt.Sprintf("%s%d", generateName, rand.Int())
	replicas := int32(1)
	return &TestDeployment{
		client:    c,
		namespace: ns,
		deployment: &apps.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: generateName,
			},
			Spec: apps.DeploymentSpec{
				Replicas: &replicas,
				Selector: &metav1.LabelSelector{
					MatchLabels: map[string]string{"app": selectorValue},
				},
				Template: v1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{"app": selectorValue},
					},
					Spec: v1.PodSpec{
						Containers: []v1.Container{
							{
								Name:    "volume-tester",
								Image:   imageutils.GetE2EImage(imageutils.BusyBox),
								Command: []string{"/bin/sh"},
								Args:    []string{"-c", command},
								VolumeMounts: []v1.VolumeMount{
									{
										Name:      volumeName,
										MountPath: mountPath,
										ReadOnly:  readOnly,
									},
								},
							},
						},
						RestartPolicy: v1.RestartPolicyAlways,
						Volumes: []v1.Volume{
							{
								Name: volumeName,
								VolumeSource: v1.VolumeSource{
									PersistentVolumeClaim: &v1.PersistentVolumeClaimVolumeSource{
										ClaimName: pvc.Name,
									},
								},
							},
						},
					},
				},
			},
		},
	}
}

func (t *TestDeployment) Create() {
	var err error
	t.deployment, err = t.client.AppsV1().Deployments(t.namespace.Name).Create(context.TODO(), t.deployment, metav1.CreateOptions{})
	framework.ExpectNoError(err)
	err = e2edeployment.WaitForDeploymentComplete(t.client, t.deployment)
	framework.ExpectNoError(err)
	pods, err := e2edeployment.GetPodsForDeployment(t.client, t.deployment)
	framework.ExpectNoError(err)
	// always get first pod as there should only be one
	t.podName = pods.Items[0].Name
}

func (t *TestDeployment) WaitForPodReady() {
	pods, err := e2edeployment.GetPodsForDeployment(t.client, t.deployment)
	framework.ExpectNoError(err)
	// always get first pod as there should only be one
	pod := pods.Items[0]
	t.podName = pod.Name
	err = e2epod.WaitForPodRunningInNamespace(t.client, &pod)
	framework.ExpectNoError(err)
}

func (t *TestDeployment) Exec(command []string, expectedString string) {
	_, err := framework.LookForStringInPodExec(t.namespace.Name, t.podName, command, expectedString, execTimeout)
	framework.ExpectNoError(err)
}

func (t *TestDeployment) DeletePodAndWait() {
	e2elog.Logf("Deleting pod %q in namespace %q", t.podName, t.namespace.Name)
	err := t.client.CoreV1().Pods(t.namespace.Name).Delete(context.TODO(), t.podName, metav1.DeleteOptions{})
	if err != nil {
		if !apierrs.IsNotFound(err) {
			framework.ExpectNoError(fmt.Errorf("pod %q Delete API error: %w", t.podName, err))
		}
		return
	}
	e2elog.Logf("Waiting for pod %q in namespace %q to be fully deleted", t.podName, t.namespace.Name)
	err = e2epod.WaitForPodNotFoundInNamespace(t.client, t.podName, t.namespace.Name, 3*time.Minute)
	if err != nil {
		if !apierrs.IsNotFound(err) {
			framework.ExpectNoError(fmt.Errorf("pod %q error waiting for delete: %w", t.podName, err))
		}
	}
}

func (t *TestDeployment) Cleanup() {
	e2elog.Logf("deleting Deployment %q/%q", t.namespace.Name, t.deployment.Name)
	body, err := t.Logs()
	if err != nil {
		e2elog.Logf("Error getting logs for pod %s: %v", t.podName, err)
	} else {
		e2elog.Logf("Pod %s has the following logs: %s", t.podName, body)
	}
	err = t.client.AppsV1().Deployments(t.namespace.Name).Delete(context.TODO(), t.deployment.Name, metav1.DeleteOptions{})
	framework.ExpectNoError(err)
}

func (t *TestDeployment) Logs() ([]byte, error) {
	return podLogs(t.client, t.podName, t.namespace.Name)
}

// waitForPersistentVolumeClaimDeleted waits for a PersistentVolumeClaim to be removed from the system until timeout occurs, whichever comes first.
func waitForPersistentVolumeClaimDeleted(c clientset.Interface, ns string, pvcName string, Poll, timeout time.Duration) error {
	framework.Logf("Waiting up to %v for PersistentVolumeClaim %s to be removed", timeout, pvcName)
	for start := time.Now(); time.Since(start) < timeout; time.Sleep(Poll) {
		_, err := c.CoreV1().PersistentVolumeClaims(ns).Get(context.TODO(), pvcName, metav1.GetOptions{})
		if err != nil {
			if apierrs.IsNotFound(err) {
				framework.Logf("Claim %q in namespace %q doesn't exist in the system", pvcName, ns)
				return nil
			}
			framework.Logf("Failed to get claim %q in namespace %q, retrying in %v. Error: %v", pvcName, ns, Poll, err)
		}
	}
	return fmt.Errorf("PersistentVolumeClaim %s is not removed from the system within %v", pvcName, timeout)
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
				GenerateName: "ebs-volume-tester-",
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

	t.pod, err = t.client.CoreV1().Pods(t.namespace.Name).Create(context.TODO(), t.pod, metav1.CreateOptions{})
	framework.ExpectNoError(err)
}

func (t *TestPod) WaitForSuccess() {
	err := e2epod.WaitForPodSuccessInNamespaceSlow(t.client, t.pod.Name, t.namespace.Name)
	framework.ExpectNoError(err)
}

func (t *TestPod) WaitForRunning() {
	err := e2epod.WaitForPodRunningInNamespace(t.client, t.pod)
	framework.ExpectNoError(err)
}

// Ideally this would be in "k8s.io/kubernetes/test/e2e/framework"
// Similar to framework.WaitForPodSuccessInNamespaceSlow
var podFailedCondition = func(pod *v1.Pod) (bool, error) {
	switch pod.Status.Phase {
	case v1.PodFailed:
		By("Saw pod failure")
		return true, nil
	case v1.PodSucceeded:
		return true, fmt.Errorf("pod %q successed with reason: %q, message: %q", pod.Name, pod.Status.Reason, pod.Status.Message)
	case v1.PodPending, v1.PodRunning, v1.PodUnknown:
		return false, nil
	default:
		return false, nil
	}
}

func (t *TestPod) WaitForFailure() {
	err := e2epod.WaitForPodCondition(t.client, t.namespace.Name, t.pod.Name, failedConditionDescription, slowPodStartTimeout, podFailedCondition)
	framework.ExpectNoError(err)
}

func (t *TestPod) SetupVolume(pvc *v1.PersistentVolumeClaim, name, mountPath string, readOnly bool) {
	volumeMount := v1.VolumeMount{
		Name:      name,
		MountPath: mountPath,
		ReadOnly:  readOnly,
	}
	t.pod.Spec.Containers[0].VolumeMounts = append(t.pod.Spec.Containers[0].VolumeMounts, volumeMount)

	volume := v1.Volume{
		Name: name,
		VolumeSource: v1.VolumeSource{
			PersistentVolumeClaim: &v1.PersistentVolumeClaimVolumeSource{
				ClaimName: pvc.Name,
			},
		},
	}
	t.pod.Spec.Volumes = append(t.pod.Spec.Volumes, volume)
}

func (t *TestPod) SetupRawBlockVolume(pvc *v1.PersistentVolumeClaim, name, devicePath string) {
	volumeDevice := v1.VolumeDevice{
		Name:       name,
		DevicePath: devicePath,
	}
	t.pod.Spec.Containers[0].VolumeDevices = append(t.pod.Spec.Containers[0].VolumeDevices, volumeDevice)

	volume := v1.Volume{
		Name: name,
		VolumeSource: v1.VolumeSource{
			PersistentVolumeClaim: &v1.PersistentVolumeClaimVolumeSource{
				ClaimName: pvc.Name,
			},
		},
	}
	t.pod.Spec.Volumes = append(t.pod.Spec.Volumes, volume)
}

func (t *TestPod) SetNodeSelector(nodeSelector map[string]string) {
	t.pod.Spec.NodeSelector = nodeSelector
}

func (t *TestPod) Cleanup() {
	cleanupPodOrFail(t.client, t.pod.Name, t.namespace.Name)
}

func (t *TestPod) Logs() ([]byte, error) {
	return podLogs(t.client, t.pod.Name, t.namespace.Name)
}

func cleanupPodOrFail(client clientset.Interface, name, namespace string) {
	e2elog.Logf("deleting Pod %q/%q", namespace, name)
	body, err := podLogs(client, name, namespace)
	if err != nil {
		e2elog.Logf("Error getting logs for pod %s: %v", name, err)
	} else {
		e2elog.Logf("Pod %s has the following logs: %s", name, body)
	}
	e2epod.DeletePodOrFail(client, namespace, name)
}

func podLogs(client clientset.Interface, name, namespace string) ([]byte, error) {
	return client.CoreV1().Pods(namespace).GetLogs(name, &v1.PodLogOptions{}).Do(context.TODO()).Raw()
}
