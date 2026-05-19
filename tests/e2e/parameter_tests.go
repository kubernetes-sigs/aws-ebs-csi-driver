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
	"os"
	"path/filepath"
	"runtime"

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
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/kubernetes/test/e2e/framework"
	admissionapi "k8s.io/pod-security-admission/api"
	"sigs.k8s.io/yaml"
)

// Parameter e2e tests that require a live cluster (AWS API calls, volume provisioning, runtime checks).
// Tests that only assert on rendered Kubernetes object specs are in tests/helm-template/.
//
// Expected values are loaded from the shared values YAML files in tests/helm-template/testdata/
// so that the same file drives both helm install (via param-sets.sh) and test assertions.

const (
	controllerLabel    = "app=ebs-csi-controller"
	ebsPluginContainer = "ebs-plugin"
	ebsNamespace       = "kube-system"
)

// standardValues holds the subset of Helm values we assert on from standard.yaml.
type standardValues struct {
	Controller struct {
		K8sTagClusterId string            `json:"k8sTagClusterId"`
		ExtraVolumeTags map[string]string `json:"extraVolumeTags"`
	} `json:"controller"`
}

// loadValues reads a values YAML file from tests/helm-template/testdata/.
func loadValues(name string, out interface{}) {
	_, thisFile, _, _ := runtime.Caller(0)
	path := filepath.Join(filepath.Dir(thisFile), "..", "helm-template", "testdata", name+".yaml")
	data, err := os.ReadFile(path)
	ExpectWithOffset(1, err).NotTo(HaveOccurred(), "failed to read values file %s", path)
	ExpectWithOffset(1, yaml.Unmarshal(data, out)).NotTo(HaveOccurred(), "failed to parse values file %s", path)
}

func defaultGP3Pods() []testsuites.PodDetails {
	return []testsuites.PodDetails{{
		Cmd: testsuites.PodCmdWriteToVolume("/mnt/test-1"),
		Volumes: []testsuites.VolumeDetails{{
			CreateVolumeParameters: map[string]string{
				ebscsidriver.VolumeTypeKey: awscloud.VolumeTypeGP3,
				ebscsidriver.FSTypeKey:     ebscsidriver.FSTypeExt4,
			},
			ClaimSize:   driver.MinimumSizeForVolumeType(awscloud.VolumeTypeGP3),
			VolumeMount: testsuites.DefaultGeneratedVolumeMount,
		}},
	}}
}

func defaultGP3PodsNoFsType() []testsuites.PodDetails {
	return []testsuites.PodDetails{{
		Cmd: testsuites.PodCmdWriteToVolume("/mnt/test-1"),
		Volumes: []testsuites.VolumeDetails{{
			CreateVolumeParameters: map[string]string{
				ebscsidriver.VolumeTypeKey: awscloud.VolumeTypeGP3,
			},
			ClaimSize:   driver.MinimumSizeForVolumeType(awscloud.VolumeTypeGP3),
			VolumeMount: testsuites.DefaultGeneratedVolumeMount,
		}},
	}}
}

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
		Expect(err).NotTo(HaveOccurred())
		ec2Client = ec2.NewFromConfig(cfg)
	})

	It("should add PVC namespace tag to provisioned volume", func() {
		test := testsuites.DynamicallyProvisionedCmdVolumeTest{
			CSIDriver: ebsDriver,
			Pods:      defaultGP3Pods(),
			ValidateFunc: func() {
				result, err := ec2Client.DescribeVolumes(context.Background(), &ec2.DescribeVolumesInput{
					Filters: []types.Filter{{
						Name:   aws.String("tag:kubernetes.io/created-for/pvc/namespace"),
						Values: []string{ns.Name},
					}},
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
		vals      standardValues
	)

	BeforeEach(func() {
		cs = f.ClientSet
		ns = f.Namespace
		ebsDriver = driver.InitEbsCSIDriver()
		cfg, err := config.LoadDefaultConfig(context.Background())
		Expect(err).NotTo(HaveOccurred())
		ec2Client = ec2.NewFromConfig(cfg)
		loadValues("e2e-standard", &vals)
	})

	It("should tag volume with cluster ID", func() {
		test := testsuites.DynamicallyProvisionedCmdVolumeTest{
			CSIDriver: ebsDriver,
			Pods:      defaultGP3PodsNoFsType(),
			ValidateFunc: func() {
				result, err := ec2Client.DescribeVolumes(context.Background(), &ec2.DescribeVolumesInput{
					Filters: []types.Filter{{
						Name:   aws.String("tag:kubernetes.io/cluster/" + vals.Controller.K8sTagClusterId),
						Values: []string{"owned"},
					}},
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
		vals      standardValues
	)

	BeforeEach(func() {
		cs = f.ClientSet
		ns = f.Namespace
		ebsDriver = driver.InitEbsCSIDriver()
		cfg, err := config.LoadDefaultConfig(context.Background())
		Expect(err).NotTo(HaveOccurred())
		ec2Client = ec2.NewFromConfig(cfg)
		loadValues("e2e-standard", &vals)
	})

	It("should add extra volume tags from Helm values", func() {
		for key, value := range vals.Controller.ExtraVolumeTags {
			test := testsuites.DynamicallyProvisionedCmdVolumeTest{
				CSIDriver: ebsDriver,
				Pods:      defaultGP3PodsNoFsType(),
				ValidateFunc: func() {
					result, err := ec2Client.DescribeVolumes(context.Background(), &ec2.DescribeVolumesInput{
						Filters: []types.Filter{{
							Name:   aws.String("tag:" + key),
							Values: []string{value},
						}},
					})
					Expect(err).NotTo(HaveOccurred())
					Expect(result.Volumes).NotTo(BeEmpty(), "Should find volume with extra tag %s=%s", key, value)
				},
			}
			test.Run(cs, ns)
		}
	})
})

var _ = Describe("[ebs-csi-e2e] Live Cluster Parameter Tests", func() {
	f := framework.NewDefaultFramework("ebs")
	f.NamespacePodSecurityEnforceLevel = admissionapi.LevelPrivileged

	var cs clientset.Interface

	BeforeEach(func() {
		cs = f.ClientSet
	})

	It("[param:defaultFsType] should use xfs as default filesystem when not specified in StorageClass", func() {
		pods := []testsuites.PodDetails{{
			Cmd: "mount | grep /mnt/test-1 | grep xfs",
			Volumes: []testsuites.VolumeDetails{{
				CreateVolumeParameters: map[string]string{
					ebscsidriver.VolumeTypeKey: awscloud.VolumeTypeGP3,
				},
				ClaimSize:   driver.MinimumSizeForVolumeType(awscloud.VolumeTypeGP3),
				VolumeMount: testsuites.DefaultGeneratedVolumeMount,
			}},
		}}
		test := testsuites.DynamicallyProvisionedCmdVolumeTest{
			CSIDriver: driver.InitEbsCSIDriver(),
			Pods:      pods,
		}
		test.Run(cs, f.Namespace)
	})

	It("[param:legacyXFS] should format XFS volumes with reflink disabled when legacyXFS is enabled", func() {
		// node.legacyXFS=true makes the driver pass `-m reflink=0` to
		// mkfs.xfs. FICLONE returns EOPNOTSUPP on a filesystem formatted without reflink, so
		// `cp --reflink=always` exits non-zero with "Operation not
		// supported". busybox's cp has no --reflink, so use amazonlinux.
		const reflinkProbe = `set -e
dd if=/dev/zero of=/mnt/test-1/src bs=4k count=4 status=none
if cp --reflink=always /mnt/test-1/src /mnt/test-1/dst 2>/tmp/cp.err; then
  echo "FAIL: cp --reflink=always succeeded; expected reflink to be disabled" >&2
  exit 1
fi
if ! grep -qi "not supported" /tmp/cp.err; then
  echo "FAIL: cp --reflink=always failed for an unexpected reason:" >&2
  cat /tmp/cp.err >&2
  exit 1
fi
echo "PASS: reflink is disabled on /mnt/test-1"
`
		pods := []testsuites.PodDetails{{
			Cmd:   reflinkProbe,
			Image: "public.ecr.aws/amazonlinux/amazonlinux:2023",
			Volumes: []testsuites.VolumeDetails{{
				CreateVolumeParameters: map[string]string{
					ebscsidriver.VolumeTypeKey: awscloud.VolumeTypeGP3,
					ebscsidriver.FSTypeKey:     ebscsidriver.FSTypeXfs,
				},
				ClaimSize:   driver.MinimumSizeForVolumeType(awscloud.VolumeTypeGP3),
				VolumeMount: testsuites.DefaultGeneratedVolumeMount,
			}},
		}}
		test := testsuites.DynamicallyProvisionedCmdVolumeTest{
			CSIDriver: driver.InitEbsCSIDriver(),
			Pods:      pods,
		}
		test.Run(cs, f.Namespace)
	})

	It("[param:nodeComponentOnly] should deploy only node DaemonSet without controller", func() {
		_, err := cs.AppsV1().Deployments(ebsNamespace).Get(context.Background(), "ebs-csi-controller", metav1.GetOptions{})
		Expect(apierrors.IsNotFound(err)).To(BeTrue(), "Controller deployment should not exist, but got error: %v", err)

		ds, err := cs.AppsV1().DaemonSets(ebsNamespace).Get(context.Background(), "ebs-csi-node", metav1.GetOptions{})
		Expect(err).NotTo(HaveOccurred())
		Expect(ds.Status.DesiredNumberScheduled).To(BeNumerically(">", 0))
	})

	It("[param:fips] should use FIPS-compliant image", func() {
		pods, err := cs.CoreV1().Pods(ebsNamespace).List(context.Background(), metav1.ListOptions{
			LabelSelector: controllerLabel,
		})
		Expect(err).NotTo(HaveOccurred())
		Expect(pods.Items).NotTo(BeEmpty())
		for _, pod := range pods.Items {
			for _, c := range pod.Spec.Containers {
				if c.Name == ebsPluginContainer {
					Expect(c.Image).To(ContainSubstring("fips"), "Controller should use FIPS image")
				}
			}
		}
	})

	It("[param:metadataLabeler] should label nodes with EBS volume and ENI counts", func() {
		nodes, err := cs.CoreV1().Nodes().List(context.Background(), metav1.ListOptions{})
		Expect(err).NotTo(HaveOccurred())
		Expect(nodes.Items).NotTo(BeEmpty())

		var foundVolumeLabel, foundENILabel bool
		for _, node := range nodes.Items {
			if _, ok := node.Labels["ebs.csi.aws.com/non-csi-ebs-volumes-count"]; ok {
				foundVolumeLabel = true
			}
			if _, ok := node.Labels["ebs.csi.aws.com/enis-count"]; ok {
				foundENILabel = true
			}
		}
		Expect(foundVolumeLabel).To(BeTrue(), "At least one node should have volume count label")
		Expect(foundENILabel).To(BeTrue(), "At least one node should have ENI count label")
	})

	It("[param:additionalDaemonSets] should create additional node DaemonSet with scheduled pods", func() {
		ds, err := cs.AppsV1().DaemonSets(ebsNamespace).Get(context.Background(), "ebs-csi-node-extra", metav1.GetOptions{})
		Expect(err).NotTo(HaveOccurred())
		Expect(ds.Status.DesiredNumberScheduled).To(BeNumerically(">", 0))
	})
})
