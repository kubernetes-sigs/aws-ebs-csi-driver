/*
Copyright 2025 The Kubernetes Authors.

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

package e2e

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	awscloud "github.com/kubernetes-sigs/aws-ebs-csi-driver/pkg/cloud"
	ebscsidriver "github.com/kubernetes-sigs/aws-ebs-csi-driver/pkg/driver"
	"github.com/kubernetes-sigs/aws-ebs-csi-driver/tests/e2e/driver"
	"github.com/kubernetes-sigs/aws-ebs-csi-driver/tests/e2e/testsuites"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/kubernetes/test/e2e/framework"
	admissionapi "k8s.io/pod-security-admission/api"
)

// Parameter-specific tests validate behavior when specific Helm parameters are enabled.
// Tests are tagged with [param:<parameterName>] labels for selective execution.

// Constants for expected test values
const (
	controllerLabel     = "app=ebs-csi-controller"
	nodeLabel           = "app=ebs-csi-node"
	ebsPluginContainer  = "ebs-plugin"
	ebsNamespace        = "kube-system"
	controllerDeploy    = "ebs-csi-controller"
	nodeDaemonSet       = "ebs-csi-node"
	metricsPort         = int32(3301)
	testLogLevel        = "5"
	debugLogLevel       = "7"
	testClusterID       = "e2e-param-test"
	testExtraTagKey     = "TestKey"
	testExtraTagValue   = "TestValue"
	testStorageClass    = "test-sc"
	testSnapshotClass   = "test-vsc"
	defaultStorageClass = "ebs-csi-default-sc"
)

// getControllerPods returns the list of controller pods.
func getControllerPods(cs clientset.Interface) []v1.Pod {
	pods, err := cs.CoreV1().Pods(ebsNamespace).List(context.Background(), metav1.ListOptions{
		LabelSelector: controllerLabel,
	})
	ExpectWithOffset(1, err).NotTo(HaveOccurred())
	ExpectWithOffset(1, pods.Items).NotTo(BeEmpty(), "Controller pods should exist")
	return pods.Items
}

// getNodePods returns the list of node pods.
func getNodePods(cs clientset.Interface) []v1.Pod {
	pods, err := cs.CoreV1().Pods(ebsNamespace).List(context.Background(), metav1.ListOptions{
		LabelSelector: nodeLabel,
	})
	ExpectWithOffset(1, err).NotTo(HaveOccurred())
	ExpectWithOffset(1, pods.Items).NotTo(BeEmpty(), "Node pods should exist")
	return pods.Items
}

// findContainer returns the named container from a pod, failing if not found.
func findContainer(pod v1.Pod, name string) v1.Container {
	for _, c := range pod.Spec.Containers {
		if c.Name == name {
			return c
		}
	}
	Fail(fmt.Sprintf("container %q not found in pod %s", name, pod.Name))
	return v1.Container{}
}

// findContainerInList returns the named container from a slice, failing if not found.
func findContainerInList(containers []v1.Container, name string) v1.Container {
	for _, c := range containers {
		if c.Name == name {
			return c
		}
	}
	Fail(fmt.Sprintf("container %q not found", name))
	return v1.Container{}
}

// hasContainer returns true if the pod has a container with the given name.
func hasContainer(pod v1.Pod, name string) bool {
	for _, c := range pod.Spec.Containers {
		if c.Name == name {
			return true
		}
	}
	return false
}

// containerHasArg returns true if the container has the given arg.
func containerHasArg(c v1.Container, arg string) bool {
	for _, a := range c.Args {
		if a == arg {
			return true
		}
	}
	return false
}

// containerHasArgAny returns true if the container has any of the given args.
func containerHasArgAny(c v1.Container, args ...string) bool {
	for _, arg := range args {
		if containerHasArg(c, arg) {
			return true
		}
	}
	return false
}

// expectContainerArgOnPods checks that every pod's named container has the expected arg.
func expectContainerArgOnPods(pods []v1.Pod, containerName, expectedArg, msg string) {
	for _, pod := range pods {
		c := findContainer(pod, containerName)
		ExpectWithOffset(1, containerHasArg(c, expectedArg)).To(BeTrue(), msg)
	}
}

// expectContainerArgAnyOnPods checks that every pod's named container has one of the expected args.
func expectContainerArgAnyOnPods(pods []v1.Pod, containerName string, msg string, args ...string) {
	for _, pod := range pods {
		c := findContainer(pod, containerName)
		ExpectWithOffset(1, containerHasArgAny(c, args...)).To(BeTrue(), msg)
	}
}

// expectContainerHasMetricsPort checks that at least one container in each pod exposes the metrics port.
func expectContainerHasMetricsPort(pods []v1.Pod, msg string) {
	for _, pod := range pods {
		var found bool
		for _, c := range pod.Spec.Containers {
			for _, port := range c.Ports {
				if port.Name == "metrics" || port.ContainerPort == metricsPort {
					found = true
					break
				}
			}
		}
		ExpectWithOffset(1, found).To(BeTrue(), msg)
	}
}

// defaultGP3Pods returns a standard pod spec for volume provisioning tests.
func defaultGP3Pods() []testsuites.PodDetails {
	return []testsuites.PodDetails{
		{
			Cmd: testsuites.PodCmdWriteToVolume("/mnt/test-1"),
			Volumes: []testsuites.VolumeDetails{
				{
					CreateVolumeParameters: map[string]string{
						ebscsidriver.VolumeTypeKey: awscloud.VolumeTypeGP3,
						ebscsidriver.FSTypeKey:     ebscsidriver.FSTypeExt4,
					},
					ClaimSize:   driver.MinimumSizeForVolumeType(awscloud.VolumeTypeGP3),
					VolumeMount: testsuites.DefaultGeneratedVolumeMount,
				},
			},
		},
	}
}

// defaultGP3PodsNoFsType returns a standard pod spec without explicit fsType.
func defaultGP3PodsNoFsType() []testsuites.PodDetails {
	return []testsuites.PodDetails{
		{
			Cmd: testsuites.PodCmdWriteToVolume("/mnt/test-1"),
			Volumes: []testsuites.VolumeDetails{
				{
					CreateVolumeParameters: map[string]string{
						ebscsidriver.VolumeTypeKey: awscloud.VolumeTypeGP3,
					},
					ClaimSize:   driver.MinimumSizeForVolumeType(awscloud.VolumeTypeGP3),
					VolumeMount: testsuites.DefaultGeneratedVolumeMount,
				},
			},
		},
	}
}

// --- Volume Tagging Tests ---

var _ = Describe("[ebs-csi-e2e] [param:extraCreateMetadata]", func() {
	f := framework.NewDefaultFramework("ebs")
	f.NamespacePodSecurityEnforceLevel = admissionapi.LevelPrivileged

	var (
		cs        clientset.Interface
		ns        *v1.Namespace
		ebsDriver driver.PVTestDriver
		ec2Client *ec2.Client
	)

	BeforeEach(func() {
		cs = f.ClientSet
		ns = f.Namespace
		ebsDriver = driver.InitEbsCSIDriver()

		cfg, err := config.LoadDefaultConfig(context.Background())
		Expect(err).NotTo(HaveOccurred(), "Failed to load AWS config")
		ec2Client = ec2.NewFromConfig(cfg)
	})

	It("should add PVC namespace tag to provisioned volume", func() {
		test := testsuites.DynamicallyProvisionedCmdVolumeTest{
			CSIDriver: ebsDriver,
			Pods:      defaultGP3Pods(),
			ValidateFunc: func() {
				result, err := ec2Client.DescribeVolumes(context.Background(), &ec2.DescribeVolumesInput{
					Filters: []types.Filter{
						{
							Name:   aws.String("tag:kubernetes.io/created-for/pvc/namespace"),
							Values: []string{ns.Name},
						},
					},
				})
				Expect(err).NotTo(HaveOccurred())
				Expect(result.Volumes).NotTo(BeEmpty(), "Should find volume with PVC namespace tag")
			},
		}
		test.Run(cs, ns)
	})
})

var _ = Describe("[ebs-csi-e2e] [param:k8sTagClusterId]", func() {
	f := framework.NewDefaultFramework("ebs")
	f.NamespacePodSecurityEnforceLevel = admissionapi.LevelPrivileged

	var (
		cs        clientset.Interface
		ns        *v1.Namespace
		ebsDriver driver.PVTestDriver
		ec2Client *ec2.Client
	)

	BeforeEach(func() {
		cs = f.ClientSet
		ns = f.Namespace
		ebsDriver = driver.InitEbsCSIDriver()

		cfg, err := config.LoadDefaultConfig(context.Background())
		Expect(err).NotTo(HaveOccurred())
		ec2Client = ec2.NewFromConfig(cfg)
	})

	It("should tag volume with cluster ID", func() {
		test := testsuites.DynamicallyProvisionedCmdVolumeTest{
			CSIDriver: ebsDriver,
			Pods:      defaultGP3PodsNoFsType(),
			ValidateFunc: func() {
				result, err := ec2Client.DescribeVolumes(context.Background(), &ec2.DescribeVolumesInput{
					Filters: []types.Filter{
						{
							Name:   aws.String("tag:kubernetes.io/cluster/" + testClusterID),
							Values: []string{"owned"},
						},
					},
				})
				Expect(err).NotTo(HaveOccurred())
				Expect(result.Volumes).NotTo(BeEmpty(), "Should find volume with cluster ID tag")
			},
		}
		test.Run(cs, ns)
	})
})

var _ = Describe("[ebs-csi-e2e] [param:extraVolumeTags]", func() {
	f := framework.NewDefaultFramework("ebs")
	f.NamespacePodSecurityEnforceLevel = admissionapi.LevelPrivileged

	var (
		cs        clientset.Interface
		ns        *v1.Namespace
		ebsDriver driver.PVTestDriver
		ec2Client *ec2.Client
	)

	BeforeEach(func() {
		cs = f.ClientSet
		ns = f.Namespace
		ebsDriver = driver.InitEbsCSIDriver()

		cfg, err := config.LoadDefaultConfig(context.Background())
		Expect(err).NotTo(HaveOccurred())
		ec2Client = ec2.NewFromConfig(cfg)
	})

	It("should add extra volume tags from Helm values", func() {
		test := testsuites.DynamicallyProvisionedCmdVolumeTest{
			CSIDriver: ebsDriver,
			Pods:      defaultGP3PodsNoFsType(),
			ValidateFunc: func() {
				result, err := ec2Client.DescribeVolumes(context.Background(), &ec2.DescribeVolumesInput{
					Filters: []types.Filter{
						{
							Name:   aws.String("tag:" + testExtraTagKey),
							Values: []string{testExtraTagValue},
						},
					},
				})
				Expect(err).NotTo(HaveOccurred())
				Expect(result.Volumes).NotTo(BeEmpty(), "Should find volume with extra tag %s=%s", testExtraTagKey, testExtraTagValue)
			},
		}
		test.Run(cs, ns)
	})
})

// --- Parameter Tests (no AWS API needed) ---

var _ = Describe("[ebs-csi-e2e] Helm Parameter Tests", func() {
	f := framework.NewDefaultFramework("ebs")
	f.NamespacePodSecurityEnforceLevel = admissionapi.LevelPrivileged

	var cs clientset.Interface

	BeforeEach(func() {
		cs = f.ClientSet
	})

	It("[param:controllerMetrics] should expose metrics endpoint on controller", func() {
		expectContainerHasMetricsPort(getControllerPods(cs), "Controller pod should have metrics port")
	})

	It("[param:nodeMetrics] should expose metrics endpoint on node pods", func() {
		expectContainerHasMetricsPort(getNodePods(cs), "Node pod should have metrics port")
	})

	It("[param:batching] should have batching enabled in controller args", func() {
		expectContainerArgOnPods(getControllerPods(cs), ebsPluginContainer, "--batching=true", "Controller should have --batching=true arg")
	})

	It("[param:defaultFsType] should use xfs as default filesystem when not specified in StorageClass", func() {
		pods := []testsuites.PodDetails{
			{
				Cmd: "mount | grep /mnt/test-1 | grep xfs",
				Volumes: []testsuites.VolumeDetails{
					{
						CreateVolumeParameters: map[string]string{
							ebscsidriver.VolumeTypeKey: awscloud.VolumeTypeGP3,
						},
						ClaimSize:   driver.MinimumSizeForVolumeType(awscloud.VolumeTypeGP3),
						VolumeMount: testsuites.DefaultGeneratedVolumeMount,
					},
				},
			},
		}
		test := testsuites.DynamicallyProvisionedCmdVolumeTest{
			CSIDriver: driver.InitEbsCSIDriver(),
			Pods:      pods,
		}
		test.Run(cs, f.Namespace)
	})

	It("[param:volumeModification] should have volumemodifier sidecar running", func() {
		for _, pod := range getControllerPods(cs) {
			Expect(hasContainer(pod, "volumemodifier")).To(BeTrue(), "Controller should have volumemodifier sidecar")
		}
	})

	It("[param:snapshotterForceEnable] should have snapshotter sidecar running", func() {
		for _, pod := range getControllerPods(cs) {
			Expect(hasContainer(pod, "csi-snapshotter")).To(BeTrue(), "Controller should have csi-snapshotter sidecar when forceEnable=true")
		}
	})

	It("[param:hostNetwork] should run node pods on host network", func() {
		for _, pod := range getNodePods(cs) {
			Expect(pod.Spec.HostNetwork).To(BeTrue(), "Node pod should use host network")
		}
	})

	It("[param:sdkDebugLog] should have AWS SDK debug logging enabled", func() {
		expectContainerArgOnPods(getControllerPods(cs), ebsPluginContainer, "--aws-sdk-debug-log=true", "Controller should have --aws-sdk-debug-log=true arg")
	})

	It("[param:debugLogs] should have maximum verbosity in controller", func() {
		expectContainerArgAnyOnPods(getControllerPods(cs), ebsPluginContainer, "Controller ebs-plugin should have -v=7 arg when debugLogs=true", "-v="+debugLogLevel, "--v="+debugLogLevel)
	})

	It("[param:useOldCSIDriver] should use CSIDriver without fsGroupPolicy", func() {
		csiDriver, err := cs.StorageV1().CSIDrivers().Get(context.Background(), "ebs.csi.aws.com", metav1.GetOptions{})
		Expect(err).NotTo(HaveOccurred())
		Expect(csiDriver.Spec.FSGroupPolicy).To(BeNil(), "Old CSIDriver should not have fsGroupPolicy set")
	})

	It("[param:nodeAllocatableUpdatePeriodSeconds] should have configured update period in CSIDriver spec", func() {
		csiDriver, err := cs.StorageV1().CSIDrivers().Get(context.Background(), "ebs.csi.aws.com", metav1.GetOptions{})
		Expect(err).NotTo(HaveOccurred())
		Expect(csiDriver.Spec.NodeAllocatableUpdatePeriodSeconds).NotTo(BeNil())
		Expect(*csiDriver.Spec.NodeAllocatableUpdatePeriodSeconds).To(Equal(int64(30)))
	})

	It("[param:legacyXFS] should have legacy XFS flag in node args", func() {
		expectContainerArgOnPods(getNodePods(cs), ebsPluginContainer, "--legacy-xfs=true", "Node should have --legacy-xfs=true arg")
	})

	It("[param:nodeComponentOnly] should deploy only node DaemonSet without controller", func() {
		_, err := cs.AppsV1().Deployments(ebsNamespace).Get(context.Background(), controllerDeploy, metav1.GetOptions{})
		Expect(err).To(HaveOccurred(), "Controller deployment should not exist when nodeComponentOnly=true")

		ds, err := cs.AppsV1().DaemonSets(ebsNamespace).Get(context.Background(), nodeDaemonSet, metav1.GetOptions{})
		Expect(err).NotTo(HaveOccurred())
		Expect(ds.Status.DesiredNumberScheduled).To(BeNumerically(">", 0), "Node DaemonSet should have scheduled pods")
	})

	It("[param:fips] should use FIPS-compliant image", func() {
		for _, pod := range getControllerPods(cs) {
			c := findContainer(pod, ebsPluginContainer)
			Expect(c.Image).To(Or(
				ContainSubstring("-fips"),
				ContainSubstring("fips"),
			), "Controller should use FIPS image")
		}
	})

	It("[param:controllerUserAgentExtra] should have user agent extra in controller args", func() {
		expectContainerArgOnPods(getControllerPods(cs), ebsPluginContainer, "--user-agent-extra=e2e-test", "Controller should have --user-agent-extra arg")
	})

	It("[param:reservedVolumeAttachments] should have reserved volume attachments in node args", func() {
		expectContainerArgOnPods(getNodePods(cs), ebsPluginContainer, "--reserved-volume-attachments=2", "Node should have --reserved-volume-attachments=2 arg")
	})

	It("[param:volumeAttachLimit] should have volume attach limit in node args", func() {
		expectContainerArgOnPods(getNodePods(cs), ebsPluginContainer, "--volume-attach-limit=25", "Node should have --volume-attach-limit=25 arg")
	})

	It("[param:nodeTolerateAllTaints] should have toleration for all taints", func() {
		for _, pod := range getNodePods(cs) {
			var hasTolerateAll bool
			for _, toleration := range pod.Spec.Tolerations {
				if toleration.Operator == v1.TolerationOpExists && toleration.Key == "" {
					hasTolerateAll = true
					break
				}
			}
			Expect(hasTolerateAll).To(BeTrue(), "Node pod should tolerate all taints")
		}
	})

	It("[param:nodeTerminationGracePeriod] should have custom termination grace period", func() {
		for _, pod := range getNodePods(cs) {
			Expect(pod.Spec.TerminationGracePeriodSeconds).NotTo(BeNil())
			Expect(*pod.Spec.TerminationGracePeriodSeconds).To(Equal(int64(60)), "Node pod should have 60s termination grace period")
		}
	})

	It("[param:nodeKubeletPath] should have custom kubelet path mounted", func() {
		for _, pod := range getNodePods(cs) {
			c := findContainer(pod, ebsPluginContainer)
			var found bool
			for _, mount := range c.VolumeMounts {
				if mount.MountPath == "/var/lib/kubelet" {
					found = true
					break
				}
			}
			Expect(found).To(BeTrue(), "Node should have kubelet path mounted at /var/lib/kubelet")
		}
	})

	It("[param:controllerPodDisruptionBudget] should have PodDisruptionBudget for controller", func() {
		pdb, err := cs.PolicyV1().PodDisruptionBudgets(ebsNamespace).Get(context.Background(), controllerDeploy, metav1.GetOptions{})
		Expect(err).NotTo(HaveOccurred())
		Expect(pdb.Name).To(Equal(controllerDeploy))
	})

	It("[param:nodeDisableMutation] should have node service account without mutating permissions", func() {
		cr, err := cs.RbacV1().ClusterRoles().Get(context.Background(), "ebs-csi-node-role", metav1.GetOptions{})
		Expect(err).NotTo(HaveOccurred())
		for _, rule := range cr.Rules {
			for _, resource := range rule.Resources {
				if resource == "nodes" {
					for _, verb := range rule.Verbs {
						Expect(verb).NotTo(Equal("patch"), "Node role should not have patch permission when disableMutation=true")
						Expect(verb).NotTo(Equal("update"), "Node role should not have update permission when disableMutation=true")
					}
				}
			}
		}
	})

	It("[param:metadataLabeler] should deploy metadata-labeler sidecar in controller pods", func() {
		for _, pod := range getControllerPods(cs) {
			Expect(hasContainer(pod, "metadata-labeler")).To(BeTrue(), "Controller pod %s should have metadata-labeler container", pod.Name)
		}
	})

	It("[param:metadataLabeler] should label nodes with EBS volume and ENI counts", func() {
		nodes, err := cs.CoreV1().Nodes().List(context.Background(), metav1.ListOptions{})
		Expect(err).NotTo(HaveOccurred())
		Expect(nodes.Items).NotTo(BeEmpty())

		volumeLabelKey := "ebs.csi.aws.com/non-csi-ebs-volumes-count"
		eniLabelKey := "ebs.csi.aws.com/enis-count"

		var foundVolumeLabel, foundENILabel bool
		for _, node := range nodes.Items {
			if _, ok := node.Labels[volumeLabelKey]; ok {
				foundVolumeLabel = true
			}
			if _, ok := node.Labels[eniLabelKey]; ok {
				foundENILabel = true
			}
		}

		Expect(foundVolumeLabel).To(BeTrue(), "At least one node should have %s label", volumeLabelKey)
		Expect(foundENILabel).To(BeTrue(), "At least one node should have %s label", eniLabelKey)
	})

	It("[param:additionalDaemonSets] should create additional node DaemonSet", func() {
		ds, err := cs.AppsV1().DaemonSets(ebsNamespace).Get(context.Background(), "ebs-csi-node-extra", metav1.GetOptions{})
		Expect(err).NotTo(HaveOccurred())
		Expect(ds.Status.DesiredNumberScheduled).To(BeNumerically(">", 0), "Additional DaemonSet should have scheduled pods")
		c := findContainerInList(ds.Spec.Template.Spec.Containers, ebsPluginContainer)
		Expect(containerHasArg(c, "--volume-attach-limit=15")).To(BeTrue(), "Additional DaemonSet should have --volume-attach-limit=15")
	})

	It("[param:controllerLoggingFormat] should have json logging format in controller args", func() {
		expectContainerArgOnPods(getControllerPods(cs), ebsPluginContainer, "--logging-format=json", "Controller should have --logging-format=json arg")
	})

	It("[param:nodeLoggingFormat] should have json logging format in node args", func() {
		expectContainerArgOnPods(getNodePods(cs), ebsPluginContainer, "--logging-format=json", "Node should have --logging-format=json arg")
	})

	type logLevelTest struct {
		paramName     string
		podType       string // "controller" or "node"
		containerName string
	}

	controllerLogLevelTests := []logLevelTest{
		{"controllerLogLevel", "controller", ebsPluginContainer},
		{"provisionerLogLevel", "controller", "csi-provisioner"},
		{"attacherLogLevel", "controller", "csi-attacher"},
		{"snapshotterLogLevel", "controller", "csi-snapshotter"},
		{"resizerLogLevel", "controller", "csi-resizer"},
		{"volumemodifierLogLevel", "controller", "volumemodifier"},
		{"metadataLabelerLogLevel", "controller", "metadata-labeler"},
	}

	for _, tc := range controllerLogLevelTests {
		tc := tc
		It(fmt.Sprintf("[param:%s] should have log level %s in %s args", tc.paramName, testLogLevel, tc.containerName), func() {
			expectContainerArgAnyOnPods(getControllerPods(cs), tc.containerName,
				fmt.Sprintf("%s should have -v=%s arg", tc.containerName, testLogLevel),
				"-v="+testLogLevel, "--v="+testLogLevel)
		})
	}

	nodeLogLevelTests := []logLevelTest{
		{"nodeLogLevel", "node", ebsPluginContainer},
		{"nodeDriverRegistrarLogLevel", "node", "node-driver-registrar"},
	}

	for _, tc := range nodeLogLevelTests {
		tc := tc
		It(fmt.Sprintf("[param:%s] should have log level %s in %s args", tc.paramName, testLogLevel, tc.containerName), func() {
			expectContainerArgAnyOnPods(getNodePods(cs), tc.containerName,
				fmt.Sprintf("%s should have -v=%s arg", tc.containerName, testLogLevel),
				"-v="+testLogLevel, "--v="+testLogLevel)
		})
	}

	type leaderElectionTest struct {
		paramName     string
		containerName string
		enabled       bool
	}

	tests := []leaderElectionTest{
		{"provisionerLeaderElection", "csi-provisioner", true},
		{"attacherLeaderElection", "csi-attacher", true},
		{"resizerLeaderElection", "csi-resizer", true},
		{"volumemodifierLeaderElection", "volumemodifier", false},
	}

	for _, tc := range tests {
		tc := tc
		expectedArg := fmt.Sprintf("--leader-election=%v", tc.enabled)
		It(fmt.Sprintf("[param:%s] should have leader election %s=%v", tc.paramName, tc.containerName, tc.enabled), func() {
			expectContainerArgOnPods(getControllerPods(cs), tc.containerName, expectedArg,
				fmt.Sprintf("%s should have %s arg", tc.containerName, expectedArg))
		})
	}

	It("[param:storageClasses] should create StorageClass from Helm values", func() {
		sc, err := cs.StorageV1().StorageClasses().Get(context.Background(), testStorageClass, metav1.GetOptions{})
		Expect(err).NotTo(HaveOccurred())
		Expect(sc.Parameters["type"]).To(Equal("gp3"))
	})

	It("[param:volumeSnapshotClasses] should create VolumeSnapshotClass from Helm values", func() {
		vsc, err := f.DynamicClient.Resource(schema.GroupVersionResource{
			Group:    "snapshot.storage.k8s.io",
			Version:  "v1",
			Resource: "volumesnapshotclasses",
		}).Get(context.Background(), testSnapshotClass, metav1.GetOptions{})

		Expect(err).NotTo(HaveOccurred())
		Expect(vsc.GetName()).To(Equal(testSnapshotClass))

		deletionPolicy, found, err := unstructured.NestedString(vsc.Object, "deletionPolicy")
		Expect(err).NotTo(HaveOccurred())
		Expect(found).To(BeTrue())
		Expect(deletionPolicy).To(Equal("Delete"))
	})

	It("[param:defaultStorageClass] should create default StorageClass", func() {
		sc, err := cs.StorageV1().StorageClasses().Get(context.Background(), defaultStorageClass, metav1.GetOptions{})
		Expect(err).NotTo(HaveOccurred())
		Expect(sc.Annotations["storageclass.kubernetes.io/is-default-class"]).To(Equal("true"))
	})

	type prometheusTest struct {
		paramName   string
		serviceName string
	}

	for _, tc := range []prometheusTest{
		{"controllerEnablePrometheusAnnotations", controllerDeploy},
		{"nodeEnablePrometheusAnnotations", "ebs-csi-node"},
	} {
		tc := tc
		It(fmt.Sprintf("[param:%s] should have prometheus annotations on %s service", tc.paramName, tc.serviceName), func() {
			svc, err := cs.CoreV1().Services(ebsNamespace).Get(context.Background(), tc.serviceName, metav1.GetOptions{})
			Expect(err).NotTo(HaveOccurred(), "%s metrics service should exist", tc.serviceName)
			Expect(svc.Annotations["prometheus.io/scrape"]).To(Equal("true"))
			Expect(svc.Annotations["prometheus.io/port"]).NotTo(BeEmpty())
		})
	}

	It("[param:controllerReplicaCount] should have configured replica count", func() {
		deploy, err := cs.AppsV1().Deployments(ebsNamespace).Get(context.Background(), controllerDeploy, metav1.GetOptions{})
		Expect(err).NotTo(HaveOccurred())
		Expect(*deploy.Spec.Replicas).To(Equal(int32(3)))
	})

	It("[param:controllerTolerations] should have configured tolerations", func() {
		deploy, err := cs.AppsV1().Deployments(ebsNamespace).Get(context.Background(), controllerDeploy, metav1.GetOptions{})
		Expect(err).NotTo(HaveOccurred())
		var found bool
		for _, t := range deploy.Spec.Template.Spec.Tolerations {
			if t.Key == "test-key" && t.Value == "test-value" && t.Effect == v1.TaintEffectNoSchedule {
				found = true
				break
			}
		}
		Expect(found).To(BeTrue(), "Controller should have test toleration")
	})

	It("[param:controllerPriorityClassName] should have configured priority class", func() {
		deploy, err := cs.AppsV1().Deployments(ebsNamespace).Get(context.Background(), controllerDeploy, metav1.GetOptions{})
		Expect(err).NotTo(HaveOccurred())
		Expect(deploy.Spec.Template.Spec.PriorityClassName).To(Equal("system-node-critical"))
	})

	It("[param:controllerResources] should have configured resources", func() {
		deploy, err := cs.AppsV1().Deployments(ebsNamespace).Get(context.Background(), controllerDeploy, metav1.GetOptions{})
		Expect(err).NotTo(HaveOccurred())
		for _, c := range deploy.Spec.Template.Spec.Containers {
			if c.Name == ebsPluginContainer {
				Expect(c.Resources.Requests.Cpu().String()).To(Equal("100m"))
				Expect(c.Resources.Limits.Memory().String()).To(Equal("256Mi"))
				return
			}
		}
		Fail("ebs-plugin container not found")
	})

	It("[param:controllerPodAnnotations] should have configured pod annotations", func() {
		deploy, err := cs.AppsV1().Deployments(ebsNamespace).Get(context.Background(), controllerDeploy, metav1.GetOptions{})
		Expect(err).NotTo(HaveOccurred())
		Expect(deploy.Spec.Template.Annotations).To(HaveKeyWithValue("test-annotation", "test-value"))
	})

	It("[param:controllerPodLabels] should have configured pod labels", func() {
		deploy, err := cs.AppsV1().Deployments(ebsNamespace).Get(context.Background(), controllerDeploy, metav1.GetOptions{})
		Expect(err).NotTo(HaveOccurred())
		Expect(deploy.Spec.Template.Labels).To(HaveKeyWithValue("test-label", "test-value"))
	})

	It("[param:controllerDeploymentAnnotations] should have configured deployment annotations", func() {
		deploy, err := cs.AppsV1().Deployments(ebsNamespace).Get(context.Background(), controllerDeploy, metav1.GetOptions{})
		Expect(err).NotTo(HaveOccurred())
		Expect(deploy.Annotations).To(HaveKeyWithValue("deploy-annotation", "deploy-value"))
	})

	It("[param:controllerRevisionHistoryLimit] should have configured revision history limit", func() {
		deploy, err := cs.AppsV1().Deployments(ebsNamespace).Get(context.Background(), controllerDeploy, metav1.GetOptions{})
		Expect(err).NotTo(HaveOccurred())
		Expect(deploy.Spec.RevisionHistoryLimit).NotTo(BeNil())
		Expect(*deploy.Spec.RevisionHistoryLimit).To(Equal(int32(5)))
	})

	It("[param:controllerTopologySpreadConstraints] should have configured topology spread constraints", func() {
		deploy, err := cs.AppsV1().Deployments(ebsNamespace).Get(context.Background(), controllerDeploy, metav1.GetOptions{})
		Expect(err).NotTo(HaveOccurred())
		Expect(deploy.Spec.Template.Spec.TopologySpreadConstraints).NotTo(BeEmpty())
		Expect(deploy.Spec.Template.Spec.TopologySpreadConstraints[0].TopologyKey).To(Equal("topology.kubernetes.io/zone"))
	})

	It("[param:controllerSecurityContext] should have configured pod security context", func() {
		deploy, err := cs.AppsV1().Deployments(ebsNamespace).Get(context.Background(), controllerDeploy, metav1.GetOptions{})
		Expect(err).NotTo(HaveOccurred())
		Expect(deploy.Spec.Template.Spec.SecurityContext).NotTo(BeNil())
		Expect(*deploy.Spec.Template.Spec.SecurityContext.RunAsNonRoot).To(BeTrue())
	})

	It("[param:controllerContainerSecurityContext] should have configured container security context", func() {
		deploy, err := cs.AppsV1().Deployments(ebsNamespace).Get(context.Background(), controllerDeploy, metav1.GetOptions{})
		Expect(err).NotTo(HaveOccurred())
		for _, c := range deploy.Spec.Template.Spec.Containers {
			if c.Name == ebsPluginContainer {
				Expect(c.SecurityContext).NotTo(BeNil())
				Expect(*c.SecurityContext.ReadOnlyRootFilesystem).To(BeTrue())
				return
			}
		}
		Fail("ebs-plugin container not found")
	})

	It("[param:controllerUpdateStrategy] should have configured update strategy", func() {
		deploy, err := cs.AppsV1().Deployments(ebsNamespace).Get(context.Background(), controllerDeploy, metav1.GetOptions{})
		Expect(err).NotTo(HaveOccurred())
		Expect(string(deploy.Spec.Strategy.Type)).To(Equal("Recreate"))
	})

	It("[param:controllerEnv] should have configured environment variables", func() {
		deploy, err := cs.AppsV1().Deployments(ebsNamespace).Get(context.Background(), controllerDeploy, metav1.GetOptions{})
		Expect(err).NotTo(HaveOccurred())
		c := findContainerInList(deploy.Spec.Template.Spec.Containers, ebsPluginContainer)
		var found bool
		for _, env := range c.Env {
			if env.Name == "TEST_ENV" && env.Value == "test-value" {
				found = true
				break
			}
		}
		Expect(found).To(BeTrue(), "Controller should have TEST_ENV environment variable")
	})

	It("[param:controllerVolumes] should have configured extra volumes", func() {
		deploy, err := cs.AppsV1().Deployments(ebsNamespace).Get(context.Background(), controllerDeploy, metav1.GetOptions{})
		Expect(err).NotTo(HaveOccurred())
		var found bool
		for _, v := range deploy.Spec.Template.Spec.Volumes {
			if v.Name == "extra-volume" {
				found = true
				break
			}
		}
		Expect(found).To(BeTrue(), "Controller should have extra-volume")
	})

	It("[param:controllerVolumeMounts] should have configured extra volume mounts", func() {
		deploy, err := cs.AppsV1().Deployments(ebsNamespace).Get(context.Background(), controllerDeploy, metav1.GetOptions{})
		Expect(err).NotTo(HaveOccurred())
		c := findContainerInList(deploy.Spec.Template.Spec.Containers, ebsPluginContainer)
		var found bool
		for _, vm := range c.VolumeMounts {
			if vm.Name == "extra-volume" && vm.MountPath == "/extra" {
				found = true
				break
			}
		}
		Expect(found).To(BeTrue(), "Controller should have extra-volume mount")
	})

	It("[param:controllerAdditionalArgs] should have configured additional args", func() {
		deploy, err := cs.AppsV1().Deployments(ebsNamespace).Get(context.Background(), controllerDeploy, metav1.GetOptions{})
		Expect(err).NotTo(HaveOccurred())
		c := findContainerInList(deploy.Spec.Template.Spec.Containers, ebsPluginContainer)
		Expect(containerHasArg(c, "--warn-on-invalid-tag")).To(BeTrue(), "Controller should have --warn-on-invalid-tag")
	})

	It("[param:controllerDnsConfig] should have configured DNS config", func() {
		deploy, err := cs.AppsV1().Deployments(ebsNamespace).Get(context.Background(), controllerDeploy, metav1.GetOptions{})
		Expect(err).NotTo(HaveOccurred())
		Expect(deploy.Spec.Template.Spec.DNSConfig).NotTo(BeNil())
		Expect(deploy.Spec.Template.Spec.DNSConfig.Nameservers).To(ContainElement("8.8.8.8"))
	})

	It("[param:controllerInitContainers] should have configured init containers", func() {
		deploy, err := cs.AppsV1().Deployments(ebsNamespace).Get(context.Background(), controllerDeploy, metav1.GetOptions{})
		Expect(err).NotTo(HaveOccurred())
		var found bool
		for _, c := range deploy.Spec.Template.Spec.InitContainers {
			if c.Name == "init-container" {
				found = true
				break
			}
		}
		Expect(found).To(BeTrue(), "Controller should have init-container")
	})

	It("[param:nameOverride] should override app.kubernetes.io/name label", func() {
		deploy, err := cs.AppsV1().Deployments(ebsNamespace).Get(context.Background(), controllerDeploy, metav1.GetOptions{})
		Expect(err).NotTo(HaveOccurred())
		Expect(deploy.Labels["app.kubernetes.io/name"]).To(Equal("custom-ebs-name"))
	})

	It("[param:imagePullPolicy] should use configured image pull policy", func() {
		deploy, err := cs.AppsV1().Deployments(ebsNamespace).Get(context.Background(), controllerDeploy, metav1.GetOptions{})
		Expect(err).NotTo(HaveOccurred())
		c := findContainerInList(deploy.Spec.Template.Spec.Containers, ebsPluginContainer)
		Expect(string(c.ImagePullPolicy)).To(Equal("Always"))
	})

	It("[param:customLabels] should have custom labels on controller", func() {
		deploy, err := cs.AppsV1().Deployments(ebsNamespace).Get(context.Background(), controllerDeploy, metav1.GetOptions{})
		Expect(err).NotTo(HaveOccurred())
		Expect(deploy.Labels).To(HaveKeyWithValue("custom-label", "custom-value"))
	})

	It("[param:nodeTolerations] should have configured tolerations", func() {
		ds, err := cs.AppsV1().DaemonSets(ebsNamespace).Get(context.Background(), nodeDaemonSet, metav1.GetOptions{})
		Expect(err).NotTo(HaveOccurred())
		var found bool
		for _, t := range ds.Spec.Template.Spec.Tolerations {
			if t.Key == "node-key" && t.Value == "node-value" {
				found = true
				break
			}
		}
		Expect(found).To(BeTrue(), "Node should have test toleration")
	})

	It("[param:nodePriorityClassName] should have configured priority class", func() {
		ds, err := cs.AppsV1().DaemonSets(ebsNamespace).Get(context.Background(), nodeDaemonSet, metav1.GetOptions{})
		Expect(err).NotTo(HaveOccurred())
		Expect(ds.Spec.Template.Spec.PriorityClassName).To(Equal("system-node-critical"))
	})

	It("[param:nodeResources] should have configured resources", func() {
		ds, err := cs.AppsV1().DaemonSets(ebsNamespace).Get(context.Background(), nodeDaemonSet, metav1.GetOptions{})
		Expect(err).NotTo(HaveOccurred())
		for _, c := range ds.Spec.Template.Spec.Containers {
			if c.Name == ebsPluginContainer {
				Expect(c.Resources.Requests.Cpu().String()).To(Equal("50m"))
				Expect(c.Resources.Limits.Memory().String()).To(Equal("128Mi"))
				return
			}
		}
		Fail("ebs-plugin container not found")
	})

	It("[param:nodePodAnnotations] should have configured pod annotations", func() {
		ds, err := cs.AppsV1().DaemonSets(ebsNamespace).Get(context.Background(), nodeDaemonSet, metav1.GetOptions{})
		Expect(err).NotTo(HaveOccurred())
		Expect(ds.Spec.Template.Annotations).To(HaveKeyWithValue("node-annotation", "node-value"))
	})

	It("[param:nodeDaemonSetAnnotations] should have configured daemonset annotations", func() {
		ds, err := cs.AppsV1().DaemonSets(ebsNamespace).Get(context.Background(), nodeDaemonSet, metav1.GetOptions{})
		Expect(err).NotTo(HaveOccurred())
		Expect(ds.Annotations).To(HaveKeyWithValue("ds-annotation", "ds-value"))
	})

	It("[param:nodeRevisionHistoryLimit] should have configured revision history limit", func() {
		ds, err := cs.AppsV1().DaemonSets(ebsNamespace).Get(context.Background(), nodeDaemonSet, metav1.GetOptions{})
		Expect(err).NotTo(HaveOccurred())
		Expect(ds.Spec.RevisionHistoryLimit).NotTo(BeNil())
		Expect(*ds.Spec.RevisionHistoryLimit).To(Equal(int32(3)))
	})

	It("[param:nodeSecurityContext] should have configured pod security context", func() {
		ds, err := cs.AppsV1().DaemonSets(ebsNamespace).Get(context.Background(), nodeDaemonSet, metav1.GetOptions{})
		Expect(err).NotTo(HaveOccurred())
		Expect(ds.Spec.Template.Spec.SecurityContext).NotTo(BeNil())
	})

	It("[param:nodeUpdateStrategy] should have configured update strategy", func() {
		ds, err := cs.AppsV1().DaemonSets(ebsNamespace).Get(context.Background(), nodeDaemonSet, metav1.GetOptions{})
		Expect(err).NotTo(HaveOccurred())
		Expect(string(ds.Spec.UpdateStrategy.Type)).To(Equal("OnDelete"))
	})

	It("[param:nodeEnv] should have configured environment variables", func() {
		ds, err := cs.AppsV1().DaemonSets(ebsNamespace).Get(context.Background(), nodeDaemonSet, metav1.GetOptions{})
		Expect(err).NotTo(HaveOccurred())
		for _, c := range ds.Spec.Template.Spec.Containers {
			if c.Name == ebsPluginContainer {
				var found bool
				for _, env := range c.Env {
					if env.Name == "NODE_ENV" && env.Value == "node-value" {
						found = true
						break
					}
				}
				Expect(found).To(BeTrue(), "Node should have NODE_ENV environment variable")
				return
			}
		}
		Fail("ebs-plugin container not found")
	})

	It("[param:nodeVolumes] should have configured extra volumes", func() {
		ds, err := cs.AppsV1().DaemonSets(ebsNamespace).Get(context.Background(), nodeDaemonSet, metav1.GetOptions{})
		Expect(err).NotTo(HaveOccurred())
		var found bool
		for _, v := range ds.Spec.Template.Spec.Volumes {
			if v.Name == "node-extra-volume" {
				found = true
				break
			}
		}
		Expect(found).To(BeTrue(), "Node should have node-extra-volume")
	})

	It("[param:nodeVolumeMounts] should have configured extra volume mounts", func() {
		ds, err := cs.AppsV1().DaemonSets(ebsNamespace).Get(context.Background(), nodeDaemonSet, metav1.GetOptions{})
		Expect(err).NotTo(HaveOccurred())
		for _, c := range ds.Spec.Template.Spec.Containers {
			if c.Name == ebsPluginContainer {
				var found bool
				for _, vm := range c.VolumeMounts {
					if vm.Name == "node-extra-volume" && vm.MountPath == "/node-extra" {
						found = true
						break
					}
				}
				Expect(found).To(BeTrue(), "Node should have node-extra-volume mount")
				return
			}
		}
		Fail("ebs-plugin container not found")
	})

	It("[param:nodeAdditionalArgs] should have configured additional args", func() {
		ds, err := cs.AppsV1().DaemonSets(ebsNamespace).Get(context.Background(), nodeDaemonSet, metav1.GetOptions{})
		Expect(err).NotTo(HaveOccurred())
		for _, c := range ds.Spec.Template.Spec.Containers {
			if c.Name == ebsPluginContainer {
				Expect(containerHasArg(c, "--logtostderr")).To(BeTrue(), "Node should have --logtostderr")
				return
			}
		}
		Fail("ebs-plugin container not found")
	})

	It("[param:nodeDnsConfig] should have configured DNS config", func() {
		ds, err := cs.AppsV1().DaemonSets(ebsNamespace).Get(context.Background(), nodeDaemonSet, metav1.GetOptions{})
		Expect(err).NotTo(HaveOccurred())
		Expect(ds.Spec.Template.Spec.DNSConfig).NotTo(BeNil())
		Expect(ds.Spec.Template.Spec.DNSConfig.Nameservers).To(ContainElement("8.8.4.4"))
	})

	It("[param:nodeInitContainers] should have configured init containers", func() {
		ds, err := cs.AppsV1().DaemonSets(ebsNamespace).Get(context.Background(), nodeDaemonSet, metav1.GetOptions{})
		Expect(err).NotTo(HaveOccurred())
		var found bool
		for _, c := range ds.Spec.Template.Spec.InitContainers {
			if c.Name == "node-init-container" {
				found = true
				break
			}
		}
		Expect(found).To(BeTrue(), "Node should have node-init-container")
	})

	It("[param:customLabels] should have custom labels on node daemonset", func() {
		ds, err := cs.AppsV1().DaemonSets(ebsNamespace).Get(context.Background(), nodeDaemonSet, metav1.GetOptions{})
		Expect(err).NotTo(HaveOccurred())
		Expect(ds.Labels).To(HaveKeyWithValue("custom-label", "custom-value"))
	})

	type sidecarResourceTest struct {
		paramName     string
		resourceType  string // "deployment" or "daemonset"
		resourceName  string
		containerName string
		expectedCPU   string
	}

	sidecarTests := []sidecarResourceTest{
		{"provisionerResources", "deployment", controllerDeploy, "csi-provisioner", "20m"},
		{"attacherResources", "deployment", controllerDeploy, "csi-attacher", "15m"},
		{"snapshotterResources", "deployment", controllerDeploy, "csi-snapshotter", "15m"},
		{"resizerResources", "deployment", controllerDeploy, "csi-resizer", "15m"},
		{"nodeDriverRegistrarResources", "daemonset", nodeDaemonSet, "node-driver-registrar", "10m"},
		{"livenessProbeResources", "daemonset", nodeDaemonSet, "liveness-probe", "5m"},
	}

	for _, tc := range sidecarTests {
		tc := tc
		It(fmt.Sprintf("[param:%s] should have configured resources for %s", tc.paramName, tc.containerName), func() {
			var containers []v1.Container
			if tc.resourceType == "deployment" {
				deploy, err := cs.AppsV1().Deployments(ebsNamespace).Get(context.Background(), tc.resourceName, metav1.GetOptions{})
				Expect(err).NotTo(HaveOccurred())
				containers = deploy.Spec.Template.Spec.Containers
			} else {
				ds, err := cs.AppsV1().DaemonSets(ebsNamespace).Get(context.Background(), tc.resourceName, metav1.GetOptions{})
				Expect(err).NotTo(HaveOccurred())
				containers = ds.Spec.Template.Spec.Containers
			}
			for _, c := range containers {
				if c.Name == tc.containerName {
					Expect(c.Resources.Requests.Cpu().String()).To(Equal(tc.expectedCPU))
					return
				}
			}
			Fail(fmt.Sprintf("%s container not found", tc.containerName))
		})
	}

	It("[param:provisionerAdditionalArgs] should have configured additional args for provisioner", func() {
		deploy, err := cs.AppsV1().Deployments(ebsNamespace).Get(context.Background(), controllerDeploy, metav1.GetOptions{})
		Expect(err).NotTo(HaveOccurred())
		for _, c := range deploy.Spec.Template.Spec.Containers {
			if c.Name == "csi-provisioner" {
				Expect(containerHasArg(c, "--retry-interval-start=10s")).To(BeTrue(), "Provisioner should have --retry-interval-start=10s")
				return
			}
		}
		Fail("csi-provisioner container not found")
	})
})
