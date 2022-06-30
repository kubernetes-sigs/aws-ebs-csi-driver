/*
Copyright 2019 The Kubernetes Authors.

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

	"github.com/onsi/ginkgo"
	"github.com/onsi/gomega"

	v1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/kubernetes/test/e2e/framework"
	e2epod "k8s.io/kubernetes/test/e2e/framework/pod"
	e2eskipper "k8s.io/kubernetes/test/e2e/framework/skipper"
	e2evolume "k8s.io/kubernetes/test/e2e/framework/volume"
	storageframework "k8s.io/kubernetes/test/e2e/storage/framework"
	storageutils "k8s.io/kubernetes/test/e2e/storage/utils"
)

type ephemeralTestSuite struct {
	tsInfo storageframework.TestSuiteInfo
}

// InitCustomEphemeralTestSuite returns ephemeralTestSuite that implements TestSuite interface
// using custom test patterns
func InitCustomEphemeralTestSuite(patterns []storageframework.TestPattern) storageframework.TestSuite {
	return &ephemeralTestSuite{
		tsInfo: storageframework.TestSuiteInfo{
			Name:         "ephemeral",
			TestPatterns: patterns,
		},
	}
}

// InitEphemeralTestSuite returns ephemeralTestSuite that implements TestSuite interface
// using test suite default patterns
func InitEphemeralTestSuite() storageframework.TestSuite {
	genericLateBinding := storageframework.DefaultFsGenericEphemeralVolume
	genericLateBinding.Name += " (late-binding)"
	genericLateBinding.BindingMode = storagev1.VolumeBindingWaitForFirstConsumer

	genericImmediateBinding := storageframework.DefaultFsGenericEphemeralVolume
	genericImmediateBinding.Name += " (immediate-binding)"
	genericImmediateBinding.BindingMode = storagev1.VolumeBindingImmediate

	patterns := []storageframework.TestPattern{
		storageframework.DefaultFsCSIEphemeralVolume,
		genericLateBinding,
		genericImmediateBinding,
	}

	return InitCustomEphemeralTestSuite(patterns)
}

func (p *ephemeralTestSuite) GetTestSuiteInfo() storageframework.TestSuiteInfo {
	return p.tsInfo
}

func (p *ephemeralTestSuite) SkipUnsupportedTests(driver storageframework.TestDriver, pattern storageframework.TestPattern) {
}

func (p *ephemeralTestSuite) DefineTests(driver storageframework.TestDriver, pattern storageframework.TestPattern) {
	type local struct {
		config        *storageframework.PerTestConfig
		driverCleanup func()

		testCase *EphemeralTest
		resource *storageframework.VolumeResource
	}
	var (
		eDriver storageframework.EphemeralTestDriver
		l       local
	)

	// Beware that it also registers an AfterEach which renders f unusable. Any code using
	// f must run inside an It or Context callback.
	f := framework.NewFrameworkWithCustomTimeouts("ephemeral", storageframework.GetDriverTimeouts(driver))

	init := func() {
		if pattern.VolType == storageframework.CSIInlineVolume {
			eDriver, _ = driver.(storageframework.EphemeralTestDriver)
		}
		if pattern.VolType == storageframework.GenericEphemeralVolume {
			enabled, err := GenericEphemeralVolumesEnabled(f.ClientSet, f.Timeouts, f.Namespace.Name)
			framework.ExpectNoError(err, "check GenericEphemeralVolume feature")
			if !enabled {
				e2eskipper.Skipf("Cluster doesn't support %q volumes -- skipping", pattern.VolType)
			}
		}

		l = local{}

		// Now do the more expensive test initialization.
		l.config, l.driverCleanup = driver.PrepareTest(f)
		l.resource = storageframework.CreateVolumeResource(driver, l.config, pattern, e2evolume.SizeRange{})

		switch pattern.VolType {
		case storageframework.CSIInlineVolume:
			l.testCase = &EphemeralTest{
				Client:     l.config.Framework.ClientSet,
				Timeouts:   f.Timeouts,
				Namespace:  f.Namespace.Name,
				DriverName: eDriver.GetCSIDriverName(l.config),
				Node:       l.config.ClientNodeSelection,
				GetVolume: func(volumeNumber int) (map[string]string, bool, bool) {
					return eDriver.GetVolume(l.config, volumeNumber)
				},
			}
		case storageframework.GenericEphemeralVolume:
			l.testCase = &EphemeralTest{
				Client:    l.config.Framework.ClientSet,
				Timeouts:  f.Timeouts,
				Namespace: f.Namespace.Name,
				Node:      l.config.ClientNodeSelection,
				VolSource: l.resource.VolSource,
			}
		}
	}

	cleanup := func() {
		var cleanUpErrs []error
		cleanUpErrs = append(cleanUpErrs, l.resource.CleanupResource())
		cleanUpErrs = append(cleanUpErrs, storageutils.TryFunc(l.driverCleanup))
		err := utilerrors.NewAggregate(cleanUpErrs)
		framework.ExpectNoError(err, "while cleaning up")
	}

	ginkgo.It("should create read-only inline ephemeral volume", func() {
		init()
		defer cleanup()

		l.testCase.ReadOnly = true
		l.testCase.RunningPodCheck = func(pod *v1.Pod) interface{} {
			command := "mount | grep /mnt/test | grep ro,"
			if framework.NodeOSDistroIs("windows") {
				// attempt to create a dummy file and expect for it not to be created
				command = "ls /mnt/test* && (touch /mnt/test-0/hello-world || true) && [ ! -f /mnt/test-0/hello-world ]"
			}
			e2evolume.VerifyExecInPodSucceed(f, pod, command)
			return nil
		}
		l.testCase.TestEphemeral()
	})

	ginkgo.It("should create read/write inline ephemeral volume", func() {
		init()
		defer cleanup()

		l.testCase.ReadOnly = false
		l.testCase.RunningPodCheck = func(pod *v1.Pod) interface{} {
			command := "mount | grep /mnt/test | grep rw,"
			if framework.NodeOSDistroIs("windows") {
				// attempt to create a dummy file and expect for it to be created
				command = "ls /mnt/test* && touch /mnt/test-0/hello-world && [ -f /mnt/test-0/hello-world ]"
			}
			e2evolume.VerifyExecInPodSucceed(f, pod, command)
			return nil
		}
		l.testCase.TestEphemeral()
	})

	ginkgo.It("should support two pods which share the same volume", func() {
		init()
		defer cleanup()

		// We test in read-only mode if that is all that the driver supports,
		// otherwise read/write. For PVC, both are assumed to be false.
		shared := false
		readOnly := false
		if eDriver != nil {
			_, shared, readOnly = eDriver.GetVolume(l.config, 0)
		}

		l.testCase.RunningPodCheck = func(pod *v1.Pod) interface{} {
			// Create another pod with the same inline volume attributes.
			pod2 := StartInPodWithInlineVolume(f.ClientSet, f.Namespace.Name, "inline-volume-tester2", "sleep 100000",
				[]v1.VolumeSource{pod.Spec.Volumes[0].VolumeSource},
				readOnly,
				l.testCase.Node)
			framework.ExpectNoError(e2epod.WaitTimeoutForPodRunningInNamespace(f.ClientSet, pod2.Name, pod2.Namespace, f.Timeouts.PodStartSlow), "waiting for second pod with inline volume")

			// If (and only if) we were able to mount
			// read/write and volume data is not shared
			// between pods, then we can check whether
			// data written in one pod is really not
			// visible in the other.
			if !readOnly && !shared {
				ginkgo.By("writing data in one pod and checking for it in the second")
				e2evolume.VerifyExecInPodSucceed(f, pod, "touch /mnt/test-0/hello-world")
				e2evolume.VerifyExecInPodSucceed(f, pod2, "[ ! -f /mnt/test-0/hello-world ]")
			}

			defer StopPodAndDependents(f.ClientSet, f.Timeouts, pod2)
			return nil
		}

		l.testCase.TestEphemeral()
	})

	ginkgo.It("should support multiple inline ephemeral volumes", func() {
		if pattern.BindingMode == storagev1.VolumeBindingImmediate &&
			pattern.VolType == storageframework.GenericEphemeralVolume {
			e2eskipper.Skipf("Multiple generic ephemeral volumes with immediate binding may cause pod startup failures when the volumes get created in separate topology segments.")
		}

		init()
		defer cleanup()

		l.testCase.NumInlineVolumes = 2
		l.testCase.TestEphemeral()
	})
}

// EphemeralTest represents parameters to be used by tests for inline volumes.
// Not all parameters are used by all tests.
type EphemeralTest struct {
	Client     clientset.Interface
	Timeouts   *framework.TimeoutContext
	Namespace  string
	DriverName string
	VolSource  *v1.VolumeSource
	Node       e2epod.NodeSelection

	// GetVolume returns the volume attributes for a
	// certain inline ephemeral volume, enumerated starting with
	// #0. Some tests might require more than one volume. They can
	// all be the same or different, depending what the driver supports
	// and/or wants to test.
	//
	// For each volume, the test driver can specify the
	// attributes, whether two pods using those attributes will
	// end up sharing the same backend storage (i.e. changes made
	// in one pod will be visible in the other), and whether
	// the volume can be mounted read/write or only read-only.
	GetVolume func(volumeNumber int) (attributes map[string]string, shared bool, readOnly bool)

	// RunningPodCheck is invoked while a pod using an inline volume is running.
	// It can execute additional checks on the pod and its volume(s). Any data
	// returned by it is passed to StoppedPodCheck.
	RunningPodCheck func(pod *v1.Pod) interface{}

	// StoppedPodCheck is invoked after ensuring that the pod is gone.
	// It is passed the data gather by RunningPodCheck or nil if that
	// isn't defined and then can do additional checks on the node,
	// like for example verifying that the ephemeral volume was really
	// removed. How to do such a check is driver-specific and not
	// covered by the generic storage test suite.
	StoppedPodCheck func(nodeName string, runningPodData interface{})

	// NumInlineVolumes sets the number of ephemeral inline volumes per pod.
	// Unset (= zero) is the same as one.
	NumInlineVolumes int

	// ReadOnly limits mounting to read-only.
	ReadOnly bool
}

// TestEphemeral tests pod creation with one ephemeral volume.
func (t EphemeralTest) TestEphemeral() {
	client := t.Client
	gomega.Expect(client).NotTo(gomega.BeNil(), "EphemeralTest.Client is required")

	ginkgo.By(fmt.Sprintf("checking the requested inline volume exists in the pod running on node %+v", t.Node))
	command := "mount | grep /mnt/test && sleep 10000"
	if framework.NodeOSDistroIs("windows") {
		command = "ls /mnt/test* && sleep 10000"
	}

	var volumes []v1.VolumeSource
	numVolumes := t.NumInlineVolumes
	if numVolumes == 0 {
		numVolumes = 1
	}
	for i := 0; i < numVolumes; i++ {
		var volume v1.VolumeSource
		switch {
		case t.GetVolume != nil:
			attributes, _, readOnly := t.GetVolume(i)
			if readOnly && !t.ReadOnly {
				e2eskipper.Skipf("inline ephemeral volume #%d is read-only, but the test needs a read/write volume", i)
			}
			volume = v1.VolumeSource{
				CSI: &v1.CSIVolumeSource{
					Driver:           t.DriverName,
					VolumeAttributes: attributes,
				},
			}
		case t.VolSource != nil:
			volume = *t.VolSource
		default:
			framework.Failf("EphemeralTest has neither GetVolume nor VolSource")
		}
		volumes = append(volumes, volume)
	}
	pod := StartInPodWithInlineVolume(client, t.Namespace, "inline-volume-tester", command, volumes, t.ReadOnly, t.Node)
	defer func() {
		// pod might be nil now.
		StopPodAndDependents(client, t.Timeouts, pod)
	}()
	framework.ExpectNoError(e2epod.WaitTimeoutForPodRunningInNamespace(client, pod.Name, pod.Namespace, t.Timeouts.PodStartSlow), "waiting for pod with inline volume")
	runningPod, err := client.CoreV1().Pods(pod.Namespace).Get(context.TODO(), pod.Name, metav1.GetOptions{})
	framework.ExpectNoError(err, "get pod")
	actualNodeName := runningPod.Spec.NodeName

	// Run the checker of the running pod.
	var runningPodData interface{}
	if t.RunningPodCheck != nil {
		runningPodData = t.RunningPodCheck(pod)
	}

	StopPodAndDependents(client, t.Timeouts, pod)
	pod = nil // Don't stop twice.

	// There should be no dangling PVCs in the namespace now. There might be for
	// generic ephemeral volumes, if something went wrong...
	pvcs, err := client.CoreV1().PersistentVolumeClaims(t.Namespace).List(context.TODO(), metav1.ListOptions{})
	framework.ExpectNoError(err, "list PVCs")
	gomega.Expect(pvcs.Items).Should(gomega.BeEmpty(), "no dangling PVCs")

	if t.StoppedPodCheck != nil {
		t.StoppedPodCheck(actualNodeName, runningPodData)
	}
}

// StartInPodWithInlineVolume starts a command in a pod with given volume(s) mounted to /mnt/test-<number> directory.
// The caller is responsible for checking the pod and deleting it.
func StartInPodWithInlineVolume(c clientset.Interface, ns, podName, command string, volumes []v1.VolumeSource, readOnly bool, node e2epod.NodeSelection) *v1.Pod {
	pod := &v1.Pod{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Pod",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: podName + "-",
			Labels: map[string]string{
				"app": podName,
			},
		},
		Spec: v1.PodSpec{
			Containers: []v1.Container{
				{
					Name:  "csi-volume-tester",
					Image: e2epod.GetDefaultTestImage(),
					// NOTE: /bin/sh works on both agnhost and busybox
					Command: []string{"/bin/sh", "-c", command},
				},
			},
			RestartPolicy: v1.RestartPolicyNever,
		},
	}
	e2epod.SetNodeSelection(&pod.Spec, node)

	for i, volume := range volumes {
		name := fmt.Sprintf("my-volume-%d", i)
		pod.Spec.Containers[0].VolumeMounts = append(pod.Spec.Containers[0].VolumeMounts,
			v1.VolumeMount{
				Name:      name,
				MountPath: fmt.Sprintf("/mnt/test-%d", i),
				ReadOnly:  readOnly,
			})
		pod.Spec.Volumes = append(pod.Spec.Volumes,
			v1.Volume{
				Name:         name,
				VolumeSource: volume,
			})
	}

	pod, err := c.CoreV1().Pods(ns).Create(context.TODO(), pod, metav1.CreateOptions{})
	framework.ExpectNoError(err, "failed to create pod")
	return pod
}

// CSIInlineVolumesEnabled checks whether the running cluster has the CSIInlineVolumes feature gate enabled.
// It does that by trying to create a pod that uses that feature.
func CSIInlineVolumesEnabled(c clientset.Interface, t *framework.TimeoutContext, ns string) (bool, error) {
	return VolumeSourceEnabled(c, t, ns, v1.VolumeSource{
		CSI: &v1.CSIVolumeSource{
			Driver: "no-such-driver.example.com",
		},
	})
}

// GenericEphemeralVolumesEnabled checks whether the running cluster has the GenericEphemeralVolume feature gate enabled.
// It does that by trying to create a pod that uses that feature.
func GenericEphemeralVolumesEnabled(c clientset.Interface, t *framework.TimeoutContext, ns string) (bool, error) {
	storageClassName := "no-such-storage-class"
	return VolumeSourceEnabled(c, t, ns, v1.VolumeSource{
		Ephemeral: &v1.EphemeralVolumeSource{
			VolumeClaimTemplate: &v1.PersistentVolumeClaimTemplate{
				Spec: v1.PersistentVolumeClaimSpec{
					StorageClassName: &storageClassName,
					AccessModes:      []v1.PersistentVolumeAccessMode{v1.ReadWriteOnce},
					Resources: v1.ResourceRequirements{
						Requests: v1.ResourceList{
							v1.ResourceStorage: resource.MustParse("1Gi"),
						},
					},
				},
			},
		},
	})
}

// VolumeSourceEnabled checks whether a certain kind of volume source is enabled by trying
// to create a pod that uses it.
func VolumeSourceEnabled(c clientset.Interface, t *framework.TimeoutContext, ns string, volume v1.VolumeSource) (bool, error) {
	pod := &v1.Pod{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Pod",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "inline-volume-",
		},
		Spec: v1.PodSpec{
			Containers: []v1.Container{
				{
					Name:  "volume-tester",
					Image: "no-such-registry/no-such-image",
					VolumeMounts: []v1.VolumeMount{
						{
							Name:      "my-volume",
							MountPath: "/mnt/test",
						},
					},
				},
			},
			RestartPolicy: v1.RestartPolicyNever,
			Volumes: []v1.Volume{
				{
					Name:         "my-volume",
					VolumeSource: volume,
				},
			},
		},
	}

	pod, err := c.CoreV1().Pods(ns).Create(context.TODO(), pod, metav1.CreateOptions{})

	switch {
	case err == nil:
		// Pod was created, feature supported.
		StopPodAndDependents(c, t, pod)
		return true, nil
	case apierrors.IsInvalid(err):
		// "Invalid" because it uses a feature that isn't supported.
		return false, nil
	default:
		// Unexpected error.
		return false, err
	}
}
