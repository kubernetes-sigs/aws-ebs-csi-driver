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
	"strings"
	"time"

	"github.com/onsi/ginkgo"
	"github.com/onsi/gomega"

	v1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/dynamic"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/kubernetes/test/e2e/framework"
	e2epod "k8s.io/kubernetes/test/e2e/framework/pod"
	e2epv "k8s.io/kubernetes/test/e2e/framework/pv"
	e2eskipper "k8s.io/kubernetes/test/e2e/framework/skipper"
	e2evolume "k8s.io/kubernetes/test/e2e/framework/volume"
	storageframework "k8s.io/kubernetes/test/e2e/storage/framework"
	"k8s.io/kubernetes/test/e2e/storage/utils"
	storageutils "k8s.io/kubernetes/test/e2e/storage/utils"
)

// data file name
const datapath = "/mnt/test/data"

type snapshottableTestSuite struct {
	tsInfo storageframework.TestSuiteInfo
}

var (
	sDriver storageframework.SnapshottableTestDriver
	dDriver storageframework.DynamicPVTestDriver
)

// InitCustomSnapshottableTestSuite returns snapshottableTestSuite that implements TestSuite interface
// using custom test patterns
func InitCustomSnapshottableTestSuite(patterns []storageframework.TestPattern) storageframework.TestSuite {
	return &snapshottableTestSuite{
		tsInfo: storageframework.TestSuiteInfo{
			Name:         "snapshottable",
			TestPatterns: patterns,
			SupportedSizeRange: e2evolume.SizeRange{
				Min: "1Mi",
			},
			FeatureTag: "[Feature:VolumeSnapshotDataSource]",
		},
	}
}

// InitSnapshottableTestSuite returns snapshottableTestSuite that implements TestSuite interface
// using testsuite default patterns
func InitSnapshottableTestSuite() storageframework.TestSuite {
	patterns := []storageframework.TestPattern{
		storageframework.DynamicSnapshotDelete,
		storageframework.DynamicSnapshotRetain,
		storageframework.PreprovisionedSnapshotDelete,
		storageframework.PreprovisionedSnapshotRetain,
	}
	return InitCustomSnapshottableTestSuite(patterns)
}

func (s *snapshottableTestSuite) GetTestSuiteInfo() storageframework.TestSuiteInfo {
	return s.tsInfo
}

func (s *snapshottableTestSuite) SkipUnsupportedTests(driver storageframework.TestDriver, pattern storageframework.TestPattern) {
	// Check preconditions.
	dInfo := driver.GetDriverInfo()
	ok := false
	_, ok = driver.(storageframework.SnapshottableTestDriver)
	if !dInfo.Capabilities[storageframework.CapSnapshotDataSource] || !ok {
		e2eskipper.Skipf("Driver %q does not support snapshots - skipping", dInfo.Name)
	}
	_, ok = driver.(storageframework.DynamicPVTestDriver)
	if !ok {
		e2eskipper.Skipf("Driver %q does not support dynamic provisioning - skipping", driver.GetDriverInfo().Name)
	}
}

func (s *snapshottableTestSuite) DefineTests(driver storageframework.TestDriver, pattern storageframework.TestPattern) {

	// Beware that it also registers an AfterEach which renders f unusable. Any code using
	// f must run inside an It or Context callback.
	f := framework.NewDefaultFramework("snapshotting")

	ginkgo.Describe("volume snapshot controller", func() {
		var (
			err           error
			config        *storageframework.PerTestConfig
			driverCleanup func()
			cleanupSteps  []func()

			cs                  clientset.Interface
			dc                  dynamic.Interface
			pvc                 *v1.PersistentVolumeClaim
			sc                  *storagev1.StorageClass
			claimSize           string
			originalMntTestData string
		)
		init := func() {
			sDriver, _ = driver.(storageframework.SnapshottableTestDriver)
			dDriver, _ = driver.(storageframework.DynamicPVTestDriver)
			cleanupSteps = make([]func(), 0)
			// init snap class, create a source PV, PVC, Pod
			cs = f.ClientSet
			dc = f.DynamicClient

			// Now do the more expensive test initialization.
			config, driverCleanup = driver.PrepareTest(f)
			cleanupSteps = append(cleanupSteps, driverCleanup)

			var volumeResource *storageframework.VolumeResource
			cleanupSteps = append(cleanupSteps, func() {
				framework.ExpectNoError(volumeResource.CleanupResource())
			})
			volumeResource = storageframework.CreateVolumeResource(dDriver, config, pattern, s.GetTestSuiteInfo().SupportedSizeRange)

			pvc = volumeResource.Pvc
			sc = volumeResource.Sc
			claimSize = pvc.Spec.Resources.Requests.Storage().String()

			ginkgo.By("[init] starting a pod to use the claim")
			originalMntTestData = fmt.Sprintf("hello from %s namespace", pvc.GetNamespace())
			command := fmt.Sprintf("echo '%s' > %s", originalMntTestData, datapath)

			pod := RunInPodWithVolume(cs, f.Timeouts, pvc.Namespace, pvc.Name, "pvc-snapshottable-tester", command, config.ClientNodeSelection)

			err = e2epv.WaitForPersistentVolumeClaimPhase(v1.ClaimBound, cs, pvc.Namespace, pvc.Name, framework.Poll, f.Timeouts.ClaimProvision)
			framework.ExpectNoError(err)

			// Get new copy of the claim
			ginkgo.By("[init] checking the claim")
			pvc, err = cs.CoreV1().PersistentVolumeClaims(pvc.Namespace).Get(context.TODO(), pvc.Name, metav1.GetOptions{})
			framework.ExpectNoError(err)

			// Get the bound PV
			ginkgo.By("[init] checking the PV")
			pv, err := cs.CoreV1().PersistentVolumes().Get(context.TODO(), pvc.Spec.VolumeName, metav1.GetOptions{})
			framework.ExpectNoError(err)

			// At this point we know that:
			// - a pod was created with a PV that's supposed to have data
			//
			// However there's a caching issue that @jinxu97 explained and it's related with the pod & volume
			// lifecycle in windows, to understand it we first analyze what the volumemanager does:
			// - when a pod is delete the volumemanager will try to cleanup the volume mounts
			//   - NodeUnpublishVolume: unbinds the bind mount from the container
			//     - Linux: the data is flushed to disk
			//     - Windows: we delete a symlink, data's not flushed yet to disk
			//   - NodeUnstageVolume: unmount the global mount
			//     - Linux: disk is detached
			//     - Windows: data is flushed to disk and the disk is detached
			//
			// Pod deletion might not guarantee a data flush to disk, however NodeUnstageVolume adds the logic
			// to flush the data to disk (see #81690 for details).
			//
			// In the following code by checking if the PV is not in the node.Status.VolumesInUse field we
			// ensure that the volume is not used by the node anymore (an indicator that NodeUnstageVolume has
			// already finished)
			if framework.NodeOSDistroIs("windows") {
				nodeName := pod.Spec.NodeName
				gomega.Expect(nodeName).NotTo(gomega.BeEmpty(), "pod.Spec.NodeName must not be empty")

				ginkgo.By(fmt.Sprintf("[init] waiting until the node=%s is not using the volume=%s", nodeName, pv.Name))
				success := storageutils.WaitUntil(framework.Poll, f.Timeouts.PVDelete, func() bool {
					node, err := cs.CoreV1().Nodes().Get(context.TODO(), nodeName, metav1.GetOptions{})
					framework.ExpectNoError(err)
					volumesInUse := node.Status.VolumesInUse
					framework.Logf("current volumes in use: %+v", volumesInUse)
					for i := 0; i < len(volumesInUse); i++ {
						if strings.HasSuffix(string(volumesInUse[i]), pv.Name) {
							return false
						}
					}
					return true
				})
				framework.ExpectEqual(success, true)
			}
		}

		cleanup := func() {
			// Don't register an AfterEach then a cleanup step because the order
			// of execution will do the AfterEach first then the cleanup step.
			// Also AfterEach cleanup registration is not fine grained enough
			// Adding to the cleanup steps allows you to register cleanup only when it is needed
			// Ideally we could replace this with https://golang.org/pkg/testing/#T.Cleanup

			// Depending on how far the test executed, cleanup accordingly
			// Execute in reverse order, similar to defer stack
			for i := len(cleanupSteps) - 1; i >= 0; i-- {
				err := storageutils.TryFunc(cleanupSteps[i])
				framework.ExpectNoError(err, "while running cleanup steps")
			}

		}
		ginkgo.BeforeEach(func() {
			init()
		})
		ginkgo.AfterEach(func() {
			cleanup()
		})

		ginkgo.Context("", func() {
			var (
				vs        *unstructured.Unstructured
				vscontent *unstructured.Unstructured
				vsc       *unstructured.Unstructured
			)

			ginkgo.BeforeEach(func() {
				var sr *storageframework.SnapshotResource
				cleanupSteps = append(cleanupSteps, func() {
					framework.ExpectNoError(sr.CleanupResource(f.Timeouts))
				})
				parameters := map[string]string{}
				sr = storageframework.CreateSnapshotResource(sDriver, config, pattern, pvc.GetName(), pvc.GetNamespace(), f.Timeouts, parameters)
				vs = sr.Vs
				vscontent = sr.Vscontent
				vsc = sr.Vsclass
			})
			ginkgo.It("should check snapshot fields, check restore correctly works after modifying source data, check deletion", func() {
				// Get new copy of the snapshot
				ginkgo.By("checking the snapshot")
				vs, err = dc.Resource(storageutils.SnapshotGVR).Namespace(vs.GetNamespace()).Get(context.TODO(), vs.GetName(), metav1.GetOptions{})
				framework.ExpectNoError(err)

				// Get the bound snapshotContent
				snapshotStatus := vs.Object["status"].(map[string]interface{})
				snapshotContentName := snapshotStatus["boundVolumeSnapshotContentName"].(string)
				vscontent, err = dc.Resource(storageutils.SnapshotContentGVR).Get(context.TODO(), snapshotContentName, metav1.GetOptions{})
				framework.ExpectNoError(err)

				snapshotContentSpec := vscontent.Object["spec"].(map[string]interface{})
				volumeSnapshotRef := snapshotContentSpec["volumeSnapshotRef"].(map[string]interface{})

				// Check SnapshotContent properties
				ginkgo.By("checking the SnapshotContent")
				// PreprovisionedCreatedSnapshot do not need to set volume snapshot class name
				if pattern.SnapshotType != storageframework.PreprovisionedCreatedSnapshot {
					framework.ExpectEqual(snapshotContentSpec["volumeSnapshotClassName"], vsc.GetName())
				}
				framework.ExpectEqual(volumeSnapshotRef["name"], vs.GetName())
				framework.ExpectEqual(volumeSnapshotRef["namespace"], vs.GetNamespace())

				ginkgo.By("Modifying source data test")
				var restoredPVC *v1.PersistentVolumeClaim
				var restoredPod *v1.Pod
				modifiedMntTestData := fmt.Sprintf("modified data from %s namespace", pvc.GetNamespace())

				ginkgo.By("modifying the data in the source PVC")

				command := fmt.Sprintf("echo '%s' > %s", modifiedMntTestData, datapath)
				RunInPodWithVolume(cs, f.Timeouts, pvc.Namespace, pvc.Name, "pvc-snapshottable-data-tester", command, config.ClientNodeSelection)

				ginkgo.By("creating a pvc from the snapshot")
				restoredPVC = e2epv.MakePersistentVolumeClaim(e2epv.PersistentVolumeClaimConfig{
					ClaimSize:        claimSize,
					StorageClassName: &(sc.Name),
				}, config.Framework.Namespace.Name)

				group := "snapshot.storage.k8s.io"

				restoredPVC.Spec.DataSource = &v1.TypedLocalObjectReference{
					APIGroup: &group,
					Kind:     "VolumeSnapshot",
					Name:     vs.GetName(),
				}

				restoredPVC, err = cs.CoreV1().PersistentVolumeClaims(restoredPVC.Namespace).Create(context.TODO(), restoredPVC, metav1.CreateOptions{})
				framework.ExpectNoError(err)
				cleanupSteps = append(cleanupSteps, func() {
					framework.Logf("deleting claim %q/%q", restoredPVC.Namespace, restoredPVC.Name)
					// typically this claim has already been deleted
					err = cs.CoreV1().PersistentVolumeClaims(restoredPVC.Namespace).Delete(context.TODO(), restoredPVC.Name, metav1.DeleteOptions{})
					if err != nil && !apierrors.IsNotFound(err) {
						framework.Failf("Error deleting claim %q. Error: %v", restoredPVC.Name, err)
					}
				})

				ginkgo.By("starting a pod to use the claim")

				restoredPod = StartInPodWithVolume(cs, restoredPVC.Namespace, restoredPVC.Name, "restored-pvc-tester", "sleep 300", config.ClientNodeSelection)
				cleanupSteps = append(cleanupSteps, func() {
					StopPod(cs, restoredPod)
				})
				framework.ExpectNoError(e2epod.WaitTimeoutForPodRunningInNamespace(cs, restoredPod.Name, restoredPod.Namespace, f.Timeouts.PodStartSlow))
				commands := e2evolume.GenerateReadFileCmd(datapath)
				_, err = framework.LookForStringInPodExec(restoredPod.Namespace, restoredPod.Name, commands, originalMntTestData, time.Minute)
				framework.ExpectNoError(err)

				ginkgo.By("should delete the VolumeSnapshotContent according to its deletion policy")

				// Delete both Snapshot and PVC at the same time because different storage systems
				// have different ordering of deletion. Some may require delete PVC first before
				// Snapshot deletion and some are opposite.
				err = storageutils.DeleteSnapshotWithoutWaiting(dc, vs.GetNamespace(), vs.GetName())
				framework.ExpectNoError(err)
				err = cs.CoreV1().PersistentVolumeClaims(restoredPVC.Namespace).Delete(context.TODO(), restoredPVC.Name, metav1.DeleteOptions{})
				framework.ExpectNoError(err)

				// Wait for the Snapshot to be actually deleted from API server
				err = storageutils.WaitForNamespacedGVRDeletion(dc, storageutils.SnapshotGVR, vs.GetNamespace(), vs.GetNamespace(), framework.Poll, f.Timeouts.SnapshotDelete)

				switch pattern.SnapshotDeletionPolicy {
				case storageframework.DeleteSnapshot:
					ginkgo.By("checking the SnapshotContent has been deleted")
					err = utils.WaitForGVRDeletion(dc, storageutils.SnapshotContentGVR, vscontent.GetName(), framework.Poll, f.Timeouts.SnapshotDelete)
					framework.ExpectNoError(err)
				case storageframework.RetainSnapshot:
					ginkgo.By("checking the SnapshotContent has not been deleted")
					err = utils.WaitForGVRDeletion(dc, storageutils.SnapshotContentGVR, vscontent.GetName(), 1*time.Second /* poll */, 30*time.Second /* timeout */)
					framework.ExpectError(err)
				}
			})
		})
	})
}
