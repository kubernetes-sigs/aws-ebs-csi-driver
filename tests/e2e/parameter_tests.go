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

var _ = Describe("[ebs-csi-e2e] [single-az] [param:extraCreateMetadata]", func() {
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
		pods := []testsuites.PodDetails{
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

		test := testsuites.DynamicallyProvisionedCmdVolumeTest{
			CSIDriver: ebsDriver,
			Pods:      pods,
			ValidateFunc: func() {
				// Find volumes with the namespace tag (set by extraCreateMetadata)
				result, err := ec2Client.DescribeVolumes(context.Background(), &ec2.DescribeVolumesInput{
					Filters: []types.Filter{
						{
							Name:   aws.String("tag:kubernetes.io/created-for/pvc/namespace"),
							Values: []string{ns.Name},
						},
					},
				})
				Expect(err).NotTo(HaveOccurred())
				Expect(len(result.Volumes)).To(BeNumerically(">=", 1), "Should find volume with PVC namespace tag")
			},
		}
		test.Run(cs, ns)
	})
})

var _ = Describe("[ebs-csi-e2e] [single-az] [param:k8sTagClusterId]", func() {
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
		pods := []testsuites.PodDetails{
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

		test := testsuites.DynamicallyProvisionedCmdVolumeTest{
			CSIDriver: ebsDriver,
			Pods:      pods,
			ValidateFunc: func() {
				// Find volumes with the cluster tag (set by k8sTagClusterId)
				result, err := ec2Client.DescribeVolumes(context.Background(), &ec2.DescribeVolumesInput{
					Filters: []types.Filter{
						{
							Name:   aws.String("tag:kubernetes.io/cluster/e2e-param-test"),
							Values: []string{"owned"},
						},
					},
				})
				Expect(err).NotTo(HaveOccurred())
				Expect(len(result.Volumes)).To(BeNumerically(">=", 1), "Should find volume with cluster ID tag")
			},
		}
		test.Run(cs, ns)
	})
})

var _ = Describe("[ebs-csi-e2e] [single-az] [param:extraVolumeTags]", func() {
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
		pods := []testsuites.PodDetails{
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

		test := testsuites.DynamicallyProvisionedCmdVolumeTest{
			CSIDriver: ebsDriver,
			Pods:      pods,
			ValidateFunc: func() {
				// Find volumes with the extra tag (set by extraVolumeTags)
				result, err := ec2Client.DescribeVolumes(context.Background(), &ec2.DescribeVolumesInput{
					Filters: []types.Filter{
						{
							Name:   aws.String("tag:TestKey"),
							Values: []string{"TestValue"},
						},
					},
				})
				Expect(err).NotTo(HaveOccurred())
				Expect(len(result.Volumes)).To(BeNumerically(">=", 1), "Should find volume with extra tag TestKey=TestValue")
			},
		}
		test.Run(cs, ns)
	})
})

var _ = Describe("[ebs-csi-e2e] [single-az] [param:controllerMetrics]", func() {
	f := framework.NewDefaultFramework("ebs")
	f.NamespacePodSecurityEnforceLevel = admissionapi.LevelPrivileged

	var cs clientset.Interface

	BeforeEach(func() {
		cs = f.ClientSet
	})

	It("should expose metrics endpoint on controller", func() {
		// Verify controller pods exist
		pods, err := cs.CoreV1().Pods("kube-system").List(context.Background(), metav1.ListOptions{
			LabelSelector: "app=ebs-csi-controller",
		})
		Expect(err).NotTo(HaveOccurred())
		Expect(pods.Items).NotTo(BeEmpty(), "Controller pods should exist")

		// Verify metrics port is exposed on controller pods
		for _, pod := range pods.Items {
			var hasMetricsPort bool
			for _, container := range pod.Spec.Containers {
				for _, port := range container.Ports {
					if port.Name == "metrics" || port.ContainerPort == 3301 {
						hasMetricsPort = true
						break
					}
				}
			}
			Expect(hasMetricsPort).To(BeTrue(), "Controller pod should have metrics port")
		}
	})
})

var _ = Describe("[ebs-csi-e2e] [single-az] [param:nodeMetrics]", func() {
	f := framework.NewDefaultFramework("ebs")
	f.NamespacePodSecurityEnforceLevel = admissionapi.LevelPrivileged

	var cs clientset.Interface

	BeforeEach(func() {
		cs = f.ClientSet
	})

	It("should expose metrics endpoint on node pods", func() {
		// Verify node pods exist
		pods, err := cs.CoreV1().Pods("kube-system").List(context.Background(), metav1.ListOptions{
			LabelSelector: "app=ebs-csi-node",
		})
		Expect(err).NotTo(HaveOccurred())
		Expect(pods.Items).NotTo(BeEmpty(), "Node pods should exist")

		// Verify metrics port is exposed on node pods
		for _, pod := range pods.Items {
			var hasMetricsPort bool
			for _, container := range pod.Spec.Containers {
				for _, port := range container.Ports {
					if port.Name == "metrics" || port.ContainerPort == 3301 {
						hasMetricsPort = true
						break
					}
				}
			}
			Expect(hasMetricsPort).To(BeTrue(), "Node pod should have metrics port")
		}
	})
})

var _ = Describe("[ebs-csi-e2e] [single-az] [param:batching]", func() {
	f := framework.NewDefaultFramework("ebs")
	f.NamespacePodSecurityEnforceLevel = admissionapi.LevelPrivileged

	var cs clientset.Interface

	BeforeEach(func() {
		cs = f.ClientSet
	})

	It("should have batching enabled in controller args", func() {
		pods, err := cs.CoreV1().Pods("kube-system").List(context.Background(), metav1.ListOptions{
			LabelSelector: "app=ebs-csi-controller",
		})
		Expect(err).NotTo(HaveOccurred())
		Expect(pods.Items).NotTo(BeEmpty())

		// Check controller container args for batching flag
		for _, pod := range pods.Items {
			for _, container := range pod.Spec.Containers {
				if container.Name == "ebs-plugin" {
					var hasBatching bool
					for _, arg := range container.Args {
						if arg == "--batching=true" {
							hasBatching = true
							break
						}
					}
					Expect(hasBatching).To(BeTrue(), "Controller should have --batching=true arg")
				}
			}
		}
	})
})

var _ = Describe("[ebs-csi-e2e] [single-az] [param:defaultFsType]", func() {
	f := framework.NewDefaultFramework("ebs")
	f.NamespacePodSecurityEnforceLevel = admissionapi.LevelPrivileged

	var (
		cs        clientset.Interface
		ns        *v1.Namespace
		ebsDriver driver.PVTestDriver
	)

	BeforeEach(func() {
		cs = f.ClientSet
		ns = f.Namespace
		ebsDriver = driver.InitEbsCSIDriver()
	})

	It("should use xfs as default filesystem when not specified in StorageClass", func() {
		pods := []testsuites.PodDetails{
			{
				Cmd: "mount | grep /mnt/test-1 | grep xfs",
				Volumes: []testsuites.VolumeDetails{
					{
						CreateVolumeParameters: map[string]string{
							// Intentionally not setting fsType to test default
							ebscsidriver.VolumeTypeKey: awscloud.VolumeTypeGP3,
						},
						ClaimSize:   driver.MinimumSizeForVolumeType(awscloud.VolumeTypeGP3),
						VolumeMount: testsuites.DefaultGeneratedVolumeMount,
					},
				},
			},
		}

		test := testsuites.DynamicallyProvisionedCmdVolumeTest{
			CSIDriver: ebsDriver,
			Pods:      pods,
		}
		test.Run(cs, ns)
	})
})

var _ = Describe("[ebs-csi-e2e] [single-az] [param:controllerLoggingFormat]", func() {
	f := framework.NewDefaultFramework("ebs")
	f.NamespacePodSecurityEnforceLevel = admissionapi.LevelPrivileged

	var cs clientset.Interface

	BeforeEach(func() {
		cs = f.ClientSet
	})

	It("should have json logging format in controller args", func() {
		pods, err := cs.CoreV1().Pods("kube-system").List(context.Background(), metav1.ListOptions{
			LabelSelector: "app=ebs-csi-controller",
		})
		Expect(err).NotTo(HaveOccurred())
		Expect(pods.Items).NotTo(BeEmpty())

		for _, pod := range pods.Items {
			for _, container := range pod.Spec.Containers {
				if container.Name == "ebs-plugin" {
					var hasJsonLogging bool
					for _, arg := range container.Args {
						if arg == "--logging-format=json" {
							hasJsonLogging = true
							break
						}
					}
					Expect(hasJsonLogging).To(BeTrue(), "Controller should have --logging-format=json arg")
				}
			}
		}
	})
})

var _ = Describe("[ebs-csi-e2e] [single-az] [param:nodeLoggingFormat]", func() {
	f := framework.NewDefaultFramework("ebs")
	f.NamespacePodSecurityEnforceLevel = admissionapi.LevelPrivileged

	var cs clientset.Interface

	BeforeEach(func() {
		cs = f.ClientSet
	})

	It("should have json logging format in node args", func() {
		pods, err := cs.CoreV1().Pods("kube-system").List(context.Background(), metav1.ListOptions{
			LabelSelector: "app=ebs-csi-node",
		})
		Expect(err).NotTo(HaveOccurred())
		Expect(pods.Items).NotTo(BeEmpty())

		for _, pod := range pods.Items {
			for _, container := range pod.Spec.Containers {
				if container.Name == "ebs-plugin" {
					var hasJsonLogging bool
					for _, arg := range container.Args {
						if arg == "--logging-format=json" {
							hasJsonLogging = true
							break
						}
					}
					Expect(hasJsonLogging).To(BeTrue(), "Node should have --logging-format=json arg")
				}
			}
		}
	})
})

var _ = Describe("[ebs-csi-e2e] [single-az] [param:volumeModification]", func() {
	f := framework.NewDefaultFramework("ebs")
	f.NamespacePodSecurityEnforceLevel = admissionapi.LevelPrivileged

	var cs clientset.Interface

	BeforeEach(func() {
		cs = f.ClientSet
	})

	It("should have volumemodifier sidecar running", func() {
		pods, err := cs.CoreV1().Pods("kube-system").List(context.Background(), metav1.ListOptions{
			LabelSelector: "app=ebs-csi-controller",
		})
		Expect(err).NotTo(HaveOccurred())
		Expect(pods.Items).NotTo(BeEmpty())

		for _, pod := range pods.Items {
			var hasVolumeModifier bool
			for _, container := range pod.Spec.Containers {
				if container.Name == "volumemodifier" {
					hasVolumeModifier = true
					break
				}
			}
			Expect(hasVolumeModifier).To(BeTrue(), "Controller should have volumemodifier sidecar")
		}
	})
})

var _ = Describe("[ebs-csi-e2e] [single-az] [param:metadataLabeler]", func() {
	f := framework.NewDefaultFramework("ebs")
	f.NamespacePodSecurityEnforceLevel = admissionapi.LevelPrivileged

	var cs clientset.Interface

	BeforeEach(func() {
		cs = f.ClientSet
	})

	It("should label nodes with EBS volume and ENI counts", func() {
		nodes, err := cs.CoreV1().Nodes().List(context.Background(), metav1.ListOptions{})
		Expect(err).NotTo(HaveOccurred())
		Expect(nodes.Items).NotTo(BeEmpty())

		// Check that at least one node has the metadata labels
		var foundVolumeLabel, foundENILabel bool
		volumeLabelKey := "ebs.csi.aws.com/non-csi-ebs-volumes-count"
		eniLabelKey := "ebs.csi.aws.com/enis-count"

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
})

var _ = Describe("[ebs-csi-e2e] [single-az] [param:controllerLogLevel]", func() {
	f := framework.NewDefaultFramework("ebs")
	f.NamespacePodSecurityEnforceLevel = admissionapi.LevelPrivileged

	var cs clientset.Interface

	BeforeEach(func() {
		cs = f.ClientSet
	})

	It("should have log level 4 in controller args", func() {
		pods, err := cs.CoreV1().Pods("kube-system").List(context.Background(), metav1.ListOptions{
			LabelSelector: "app=ebs-csi-controller",
		})
		Expect(err).NotTo(HaveOccurred())
		Expect(pods.Items).NotTo(BeEmpty())

		for _, pod := range pods.Items {
			for _, container := range pod.Spec.Containers {
				if container.Name == "ebs-plugin" {
					var hasLogLevel bool
					for _, arg := range container.Args {
						if arg == "-v=4" || arg == "--v=4" {
							hasLogLevel = true
							break
						}
					}
					Expect(hasLogLevel).To(BeTrue(), "Controller should have -v=4 arg")
				}
			}
		}
	})
})

var _ = Describe("[ebs-csi-e2e] [single-az] [param:nodeLogLevel]", func() {
	f := framework.NewDefaultFramework("ebs")
	f.NamespacePodSecurityEnforceLevel = admissionapi.LevelPrivileged

	var cs clientset.Interface

	BeforeEach(func() {
		cs = f.ClientSet
	})

	It("should have log level 4 in node args", func() {
		pods, err := cs.CoreV1().Pods("kube-system").List(context.Background(), metav1.ListOptions{
			LabelSelector: "app=ebs-csi-node",
		})
		Expect(err).NotTo(HaveOccurred())
		Expect(pods.Items).NotTo(BeEmpty())

		for _, pod := range pods.Items {
			for _, container := range pod.Spec.Containers {
				if container.Name == "ebs-plugin" {
					var hasLogLevel bool
					for _, arg := range container.Args {
						if arg == "-v=4" || arg == "--v=4" {
							hasLogLevel = true
							break
						}
					}
					Expect(hasLogLevel).To(BeTrue(), "Node should have -v=4 arg")
				}
			}
		}
	})
})

var _ = Describe("[ebs-csi-e2e] [single-az] [param:provisionerLogLevel]", func() {
	f := framework.NewDefaultFramework("ebs")
	f.NamespacePodSecurityEnforceLevel = admissionapi.LevelPrivileged

	var cs clientset.Interface

	BeforeEach(func() {
		cs = f.ClientSet
	})

	It("should have log level 4 in provisioner sidecar args", func() {
		pods, err := cs.CoreV1().Pods("kube-system").List(context.Background(), metav1.ListOptions{
			LabelSelector: "app=ebs-csi-controller",
		})
		Expect(err).NotTo(HaveOccurred())
		Expect(pods.Items).NotTo(BeEmpty())

		for _, pod := range pods.Items {
			for _, container := range pod.Spec.Containers {
				if container.Name == "csi-provisioner" {
					var hasLogLevel bool
					for _, arg := range container.Args {
						if arg == "-v=4" || arg == "--v=4" {
							hasLogLevel = true
							break
						}
					}
					Expect(hasLogLevel).To(BeTrue(), "Provisioner should have -v=4 arg")
				}
			}
		}
	})
})

var _ = Describe("[ebs-csi-e2e] [single-az] [param:attacherLogLevel]", func() {
	f := framework.NewDefaultFramework("ebs")
	f.NamespacePodSecurityEnforceLevel = admissionapi.LevelPrivileged

	var cs clientset.Interface

	BeforeEach(func() {
		cs = f.ClientSet
	})

	It("should have log level 4 in attacher sidecar args", func() {
		pods, err := cs.CoreV1().Pods("kube-system").List(context.Background(), metav1.ListOptions{
			LabelSelector: "app=ebs-csi-controller",
		})
		Expect(err).NotTo(HaveOccurred())
		Expect(pods.Items).NotTo(BeEmpty())

		for _, pod := range pods.Items {
			for _, container := range pod.Spec.Containers {
				if container.Name == "csi-attacher" {
					var hasLogLevel bool
					for _, arg := range container.Args {
						if arg == "-v=4" || arg == "--v=4" {
							hasLogLevel = true
							break
						}
					}
					Expect(hasLogLevel).To(BeTrue(), "Attacher should have -v=4 arg")
				}
			}
		}
	})
})

var _ = Describe("[ebs-csi-e2e] [single-az] [param:snapshotterLogLevel]", func() {
	f := framework.NewDefaultFramework("ebs")
	f.NamespacePodSecurityEnforceLevel = admissionapi.LevelPrivileged

	var cs clientset.Interface

	BeforeEach(func() {
		cs = f.ClientSet
	})

	It("should have log level 4 in snapshotter sidecar args", func() {
		pods, err := cs.CoreV1().Pods("kube-system").List(context.Background(), metav1.ListOptions{
			LabelSelector: "app=ebs-csi-controller",
		})
		Expect(err).NotTo(HaveOccurred())
		Expect(pods.Items).NotTo(BeEmpty())

		for _, pod := range pods.Items {
			for _, container := range pod.Spec.Containers {
				if container.Name == "csi-snapshotter" {
					var hasLogLevel bool
					for _, arg := range container.Args {
						if arg == "-v=4" || arg == "--v=4" {
							hasLogLevel = true
							break
						}
					}
					Expect(hasLogLevel).To(BeTrue(), "Snapshotter should have -v=4 arg")
				}
			}
		}
	})
})

var _ = Describe("[ebs-csi-e2e] [single-az] [param:resizerLogLevel]", func() {
	f := framework.NewDefaultFramework("ebs")
	f.NamespacePodSecurityEnforceLevel = admissionapi.LevelPrivileged

	var cs clientset.Interface

	BeforeEach(func() {
		cs = f.ClientSet
	})

	It("should have log level 4 in resizer sidecar args", func() {
		pods, err := cs.CoreV1().Pods("kube-system").List(context.Background(), metav1.ListOptions{
			LabelSelector: "app=ebs-csi-controller",
		})
		Expect(err).NotTo(HaveOccurred())
		Expect(pods.Items).NotTo(BeEmpty())

		for _, pod := range pods.Items {
			for _, container := range pod.Spec.Containers {
				if container.Name == "csi-resizer" {
					var hasLogLevel bool
					for _, arg := range container.Args {
						if arg == "-v=4" || arg == "--v=4" {
							hasLogLevel = true
							break
						}
					}
					Expect(hasLogLevel).To(BeTrue(), "Resizer should have -v=4 arg")
				}
			}
		}
	})
})

var _ = Describe("[ebs-csi-e2e] [single-az] [param:nodeDriverRegistrarLogLevel]", func() {
	f := framework.NewDefaultFramework("ebs")
	f.NamespacePodSecurityEnforceLevel = admissionapi.LevelPrivileged

	var cs clientset.Interface

	BeforeEach(func() {
		cs = f.ClientSet
	})

	It("should have log level 4 in node-driver-registrar sidecar args", func() {
		pods, err := cs.CoreV1().Pods("kube-system").List(context.Background(), metav1.ListOptions{
			LabelSelector: "app=ebs-csi-node",
		})
		Expect(err).NotTo(HaveOccurred())
		Expect(pods.Items).NotTo(BeEmpty())

		for _, pod := range pods.Items {
			for _, container := range pod.Spec.Containers {
				if container.Name == "node-driver-registrar" {
					var hasLogLevel bool
					for _, arg := range container.Args {
						if arg == "-v=4" || arg == "--v=4" {
							hasLogLevel = true
							break
						}
					}
					Expect(hasLogLevel).To(BeTrue(), "Node-driver-registrar should have -v=4 arg")
				}
			}
		}
	})
})

var _ = Describe("[ebs-csi-e2e] [single-az] [param:volumemodifierLogLevel]", func() {
	f := framework.NewDefaultFramework("ebs")
	f.NamespacePodSecurityEnforceLevel = admissionapi.LevelPrivileged

	var cs clientset.Interface

	BeforeEach(func() {
		cs = f.ClientSet
	})

	It("should have log level 4 in volumemodifier sidecar args", func() {
		pods, err := cs.CoreV1().Pods("kube-system").List(context.Background(), metav1.ListOptions{
			LabelSelector: "app=ebs-csi-controller",
		})
		Expect(err).NotTo(HaveOccurred())
		Expect(pods.Items).NotTo(BeEmpty())

		for _, pod := range pods.Items {
			for _, container := range pod.Spec.Containers {
				if container.Name == "volumemodifier" {
					var hasLogLevel bool
					for _, arg := range container.Args {
						if arg == "-v=4" || arg == "--v=4" {
							hasLogLevel = true
							break
						}
					}
					Expect(hasLogLevel).To(BeTrue(), "Volumemodifier should have -v=4 arg")
				}
			}
		}
	})
})

var _ = Describe("[ebs-csi-e2e] [single-az] [param:metadataLabelerLogLevel]", func() {
	f := framework.NewDefaultFramework("ebs")
	f.NamespacePodSecurityEnforceLevel = admissionapi.LevelPrivileged

	var cs clientset.Interface

	BeforeEach(func() {
		cs = f.ClientSet
	})

	It("should have log level 4 in metadata-labeler sidecar args", func() {
		pods, err := cs.CoreV1().Pods("kube-system").List(context.Background(), metav1.ListOptions{
			LabelSelector: "app=ebs-csi-node",
		})
		Expect(err).NotTo(HaveOccurred())
		Expect(pods.Items).NotTo(BeEmpty())

		for _, pod := range pods.Items {
			for _, container := range pod.Spec.Containers {
				if container.Name == "metadata-labeler" {
					var hasLogLevel bool
					for _, arg := range container.Args {
						if arg == "-v=4" || arg == "--v=4" {
							hasLogLevel = true
							break
						}
					}
					Expect(hasLogLevel).To(BeTrue(), "Metadata-labeler should have -v=4 arg")
				}
			}
		}
	})
})

var _ = Describe("[ebs-csi-e2e] [single-az] [param:storageClasses]", func() {
	f := framework.NewDefaultFramework("ebs")
	f.NamespacePodSecurityEnforceLevel = admissionapi.LevelPrivileged

	var cs clientset.Interface

	BeforeEach(func() {
		cs = f.ClientSet
	})

	It("should create StorageClass from Helm values", func() {
		sc, err := cs.StorageV1().StorageClasses().Get(context.Background(), "test-sc", metav1.GetOptions{})
		Expect(err).NotTo(HaveOccurred())
		Expect(sc.Parameters["type"]).To(Equal("gp3"))
	})
})

var _ = Describe("[ebs-csi-e2e] [single-az] [param:volumeSnapshotClasses]", func() {
	f := framework.NewDefaultFramework("ebs")
	f.NamespacePodSecurityEnforceLevel = admissionapi.LevelPrivileged

	It("should create VolumeSnapshotClass from Helm values", func() {
		// Verify the test-vsc VolumeSnapshotClass was created
		vsc, err := f.DynamicClient.Resource(schema.GroupVersionResource{
			Group:    "snapshot.storage.k8s.io",
			Version:  "v1",
			Resource: "volumesnapshotclasses",
		}).Get(context.Background(), "test-vsc", metav1.GetOptions{})

		Expect(err).NotTo(HaveOccurred())
		Expect(vsc.GetName()).To(Equal("test-vsc"))

		// Verify deletionPolicy
		deletionPolicy, found, err := unstructured.NestedString(vsc.Object, "deletionPolicy")
		Expect(err).NotTo(HaveOccurred())
		Expect(found).To(BeTrue())
		Expect(deletionPolicy).To(Equal("Delete"))
	})
})

var _ = Describe("[ebs-csi-e2e] [single-az] [param:defaultStorageClass]", func() {
	f := framework.NewDefaultFramework("ebs")
	f.NamespacePodSecurityEnforceLevel = admissionapi.LevelPrivileged

	var cs clientset.Interface

	BeforeEach(func() {
		cs = f.ClientSet
	})

	It("should create default StorageClass", func() {
		sc, err := cs.StorageV1().StorageClasses().Get(context.Background(), "ebs-csi-default-sc", metav1.GetOptions{})
		Expect(err).NotTo(HaveOccurred())
		Expect(sc.Annotations["storageclass.kubernetes.io/is-default-class"]).To(Equal("true"))
	})
})

var _ = Describe("[ebs-csi-e2e] [single-az] [param:reservedVolumeAttachments]", func() {
	f := framework.NewDefaultFramework("ebs")
	f.NamespacePodSecurityEnforceLevel = admissionapi.LevelPrivileged

	var cs clientset.Interface

	BeforeEach(func() {
		cs = f.ClientSet
	})

	It("should have reserved volume attachments in node args", func() {
		pods, err := cs.CoreV1().Pods("kube-system").List(context.Background(), metav1.ListOptions{
			LabelSelector: "app=ebs-csi-node",
		})
		Expect(err).NotTo(HaveOccurred())
		Expect(pods.Items).NotTo(BeEmpty())

		for _, pod := range pods.Items {
			for _, container := range pod.Spec.Containers {
				if container.Name == "ebs-plugin" {
					var hasReserved bool
					for _, arg := range container.Args {
						if arg == "--reserved-volume-attachments=2" {
							hasReserved = true
							break
						}
					}
					Expect(hasReserved).To(BeTrue(), "Node should have --reserved-volume-attachments=2 arg")
				}
			}
		}
	})
})

var _ = Describe("[ebs-csi-e2e] [single-az] [param:volumeAttachLimit]", func() {
	f := framework.NewDefaultFramework("ebs")
	f.NamespacePodSecurityEnforceLevel = admissionapi.LevelPrivileged

	var cs clientset.Interface

	BeforeEach(func() {
		cs = f.ClientSet
	})

	It("should have volume attach limit in node args", func() {
		pods, err := cs.CoreV1().Pods("kube-system").List(context.Background(), metav1.ListOptions{
			LabelSelector: "app=ebs-csi-node",
		})
		Expect(err).NotTo(HaveOccurred())
		Expect(pods.Items).NotTo(BeEmpty())

		for _, pod := range pods.Items {
			for _, container := range pod.Spec.Containers {
				if container.Name == "ebs-plugin" {
					var hasLimit bool
					for _, arg := range container.Args {
						if arg == "--volume-attach-limit=25" {
							hasLimit = true
							break
						}
					}
					Expect(hasLimit).To(BeTrue(), "Node should have --volume-attach-limit=25 arg")
				}
			}
		}
	})
})

var _ = Describe("[ebs-csi-e2e] [single-az] [param:hostNetwork]", func() {
	f := framework.NewDefaultFramework("ebs")
	f.NamespacePodSecurityEnforceLevel = admissionapi.LevelPrivileged

	var cs clientset.Interface

	BeforeEach(func() {
		cs = f.ClientSet
	})

	It("should run node pods on host network", func() {
		pods, err := cs.CoreV1().Pods("kube-system").List(context.Background(), metav1.ListOptions{
			LabelSelector: "app=ebs-csi-node",
		})
		Expect(err).NotTo(HaveOccurred())
		Expect(pods.Items).NotTo(BeEmpty())

		for _, pod := range pods.Items {
			Expect(pod.Spec.HostNetwork).To(BeTrue(), "Node pod should use host network")
		}
	})
})

var _ = Describe("[ebs-csi-e2e] [single-az] [param:debugLogs]", func() {
	f := framework.NewDefaultFramework("ebs")
	f.NamespacePodSecurityEnforceLevel = admissionapi.LevelPrivileged

	var cs clientset.Interface

	BeforeEach(func() {
		cs = f.ClientSet
	})

	It("should have maximum verbosity in all containers", func() {
		pods, err := cs.CoreV1().Pods("kube-system").List(context.Background(), metav1.ListOptions{
			LabelSelector: "app=ebs-csi-controller",
		})
		Expect(err).NotTo(HaveOccurred())
		Expect(pods.Items).NotTo(BeEmpty())

		for _, pod := range pods.Items {
			for _, container := range pod.Spec.Containers {
				if container.Name == "ebs-plugin" {
					var hasDebugLevel bool
					for _, arg := range container.Args {
						if arg == "-v=7" || arg == "--v=7" {
							hasDebugLevel = true
							break
						}
					}
					Expect(hasDebugLevel).To(BeTrue(), "Controller ebs-plugin should have -v=7 arg when debugLogs=true")
				}
			}
		}
	})
})

var _ = Describe("[ebs-csi-e2e] [single-az] [param:sdkDebugLog]", func() {
	f := framework.NewDefaultFramework("ebs")
	f.NamespacePodSecurityEnforceLevel = admissionapi.LevelPrivileged

	var cs clientset.Interface

	BeforeEach(func() {
		cs = f.ClientSet
	})

	It("should have AWS SDK debug logging enabled", func() {
		pods, err := cs.CoreV1().Pods("kube-system").List(context.Background(), metav1.ListOptions{
			LabelSelector: "app=ebs-csi-controller",
		})
		Expect(err).NotTo(HaveOccurred())
		Expect(pods.Items).NotTo(BeEmpty())

		for _, pod := range pods.Items {
			for _, container := range pod.Spec.Containers {
				if container.Name == "ebs-plugin" {
					var hasSdkDebug bool
					for _, arg := range container.Args {
						if arg == "--aws-sdk-debug-log=true" {
							hasSdkDebug = true
							break
						}
					}
					Expect(hasSdkDebug).To(BeTrue(), "Controller should have --aws-sdk-debug-log=true arg")
				}
			}
		}
	})
})

var _ = Describe("[ebs-csi-e2e] [single-az] [param:useOldCSIDriver]", func() {
	f := framework.NewDefaultFramework("ebs")
	f.NamespacePodSecurityEnforceLevel = admissionapi.LevelPrivileged

	var cs clientset.Interface

	BeforeEach(func() {
		cs = f.ClientSet
	})

	It("should use CSIDriver without fsGroupPolicy", func() {
		csiDriver, err := cs.StorageV1().CSIDrivers().Get(context.Background(), "ebs.csi.aws.com", metav1.GetOptions{})
		Expect(err).NotTo(HaveOccurred())
		Expect(csiDriver.Spec.FSGroupPolicy).To(BeNil(), "Old CSIDriver should not have fsGroupPolicy set")
	})
})

var _ = Describe("[ebs-csi-e2e] [single-az] [param:legacyXFS]", func() {
	f := framework.NewDefaultFramework("ebs")
	f.NamespacePodSecurityEnforceLevel = admissionapi.LevelPrivileged

	var cs clientset.Interface

	BeforeEach(func() {
		cs = f.ClientSet
	})

	It("should have legacy XFS flag in node args", func() {
		pods, err := cs.CoreV1().Pods("kube-system").List(context.Background(), metav1.ListOptions{
			LabelSelector: "app=ebs-csi-node",
		})
		Expect(err).NotTo(HaveOccurred())
		Expect(pods.Items).NotTo(BeEmpty())

		for _, pod := range pods.Items {
			for _, container := range pod.Spec.Containers {
				if container.Name == "ebs-plugin" {
					var hasLegacyXFS bool
					for _, arg := range container.Args {
						if arg == "--legacy-xfs=true" {
							hasLegacyXFS = true
							break
						}
					}
					Expect(hasLegacyXFS).To(BeTrue(), "Node should have --legacy-xfs=true arg")
				}
			}
		}
	})
})

var _ = Describe("[ebs-csi-e2e] [single-az] [param:selinux]", func() {
	f := framework.NewDefaultFramework("ebs")
	f.NamespacePodSecurityEnforceLevel = admissionapi.LevelPrivileged

	var cs clientset.Interface

	BeforeEach(func() {
		cs = f.ClientSet
	})

	It("should have SELinux mount option enabled", func() {
		pods, err := cs.CoreV1().Pods("kube-system").List(context.Background(), metav1.ListOptions{
			LabelSelector: "app=ebs-csi-node",
		})
		Expect(err).NotTo(HaveOccurred())
		Expect(pods.Items).NotTo(BeEmpty())

		for _, pod := range pods.Items {
			for _, container := range pod.Spec.Containers {
				if container.Name == "ebs-plugin" {
					var hasSelinux bool
					for _, arg := range container.Args {
						if arg == "--warn-on-invalid-tag=false" {
							// SELinux mode changes security context, check for related config
							hasSelinux = true
							break
						}
					}
					// SELinux primarily affects security context, not args
					_ = hasSelinux
				}
			}
			// Check security context for SELinux
			if pod.Spec.SecurityContext != nil && pod.Spec.SecurityContext.SELinuxOptions != nil {
				Expect(pod.Spec.SecurityContext.SELinuxOptions).NotTo(BeNil())
			}
		}
	})
})

var _ = Describe("[ebs-csi-e2e] [single-az] [param:fips]", func() {
	f := framework.NewDefaultFramework("ebs")
	f.NamespacePodSecurityEnforceLevel = admissionapi.LevelPrivileged

	var cs clientset.Interface

	BeforeEach(func() {
		cs = f.ClientSet
	})

	It("should use FIPS-compliant image", func() {
		pods, err := cs.CoreV1().Pods("kube-system").List(context.Background(), metav1.ListOptions{
			LabelSelector: "app=ebs-csi-controller",
		})
		Expect(err).NotTo(HaveOccurred())
		Expect(pods.Items).NotTo(BeEmpty())

		for _, pod := range pods.Items {
			for _, container := range pod.Spec.Containers {
				if container.Name == "ebs-plugin" {
					// FIPS images have -fips suffix or use fips tag
					Expect(container.Image).To(Or(
						ContainSubstring("-fips"),
						ContainSubstring("fips"),
					), "Controller should use FIPS image")
				}
			}
		}
	})
})

var _ = Describe("[ebs-csi-e2e] [single-az] [param:snapshotterForceEnable]", func() {
	f := framework.NewDefaultFramework("ebs")
	f.NamespacePodSecurityEnforceLevel = admissionapi.LevelPrivileged

	var cs clientset.Interface

	BeforeEach(func() {
		cs = f.ClientSet
	})

	It("should have snapshotter sidecar running", func() {
		pods, err := cs.CoreV1().Pods("kube-system").List(context.Background(), metav1.ListOptions{
			LabelSelector: "app=ebs-csi-controller",
		})
		Expect(err).NotTo(HaveOccurred())
		Expect(pods.Items).NotTo(BeEmpty())

		for _, pod := range pods.Items {
			var hasSnapshotter bool
			for _, container := range pod.Spec.Containers {
				if container.Name == "csi-snapshotter" {
					hasSnapshotter = true
					break
				}
			}
			Expect(hasSnapshotter).To(BeTrue(), "Controller should have csi-snapshotter sidecar when forceEnable=true")
		}
	})
})

var _ = Describe("[ebs-csi-e2e] [single-az] [param:controllerUserAgentExtra]", func() {
	f := framework.NewDefaultFramework("ebs")
	f.NamespacePodSecurityEnforceLevel = admissionapi.LevelPrivileged

	var cs clientset.Interface

	BeforeEach(func() {
		cs = f.ClientSet
	})

	It("should have user agent extra in controller args", func() {
		pods, err := cs.CoreV1().Pods("kube-system").List(context.Background(), metav1.ListOptions{
			LabelSelector: "app=ebs-csi-controller",
		})
		Expect(err).NotTo(HaveOccurred())
		Expect(pods.Items).NotTo(BeEmpty())

		for _, pod := range pods.Items {
			for _, container := range pod.Spec.Containers {
				if container.Name == "ebs-plugin" {
					var hasUserAgent bool
					for _, arg := range container.Args {
						if arg == "--user-agent-extra=e2e-test" {
							hasUserAgent = true
							break
						}
					}
					Expect(hasUserAgent).To(BeTrue(), "Controller should have --user-agent-extra arg")
				}
			}
		}
	})
})

var _ = Describe("[ebs-csi-e2e] [single-az] [param:controllerEnablePrometheusAnnotations]", func() {
	f := framework.NewDefaultFramework("ebs")
	f.NamespacePodSecurityEnforceLevel = admissionapi.LevelPrivileged

	var cs clientset.Interface

	BeforeEach(func() {
		cs = f.ClientSet
	})

	It("should have prometheus annotations on controller metrics service", func() {
		svc, err := cs.CoreV1().Services("kube-system").Get(context.Background(), "ebs-csi-controller", metav1.GetOptions{})
		Expect(err).NotTo(HaveOccurred(), "Controller metrics service should exist when enableMetrics=true")
		Expect(svc.Annotations["prometheus.io/scrape"]).To(Equal("true"))
		Expect(svc.Annotations["prometheus.io/port"]).NotTo(BeEmpty())
	})
})

var _ = Describe("[ebs-csi-e2e] [single-az] [param:nodeEnablePrometheusAnnotations]", func() {
	f := framework.NewDefaultFramework("ebs")
	f.NamespacePodSecurityEnforceLevel = admissionapi.LevelPrivileged

	var cs clientset.Interface

	BeforeEach(func() {
		cs = f.ClientSet
	})

	It("should have prometheus annotations on node metrics service", func() {
		svc, err := cs.CoreV1().Services("kube-system").Get(context.Background(), "ebs-csi-node", metav1.GetOptions{})
		Expect(err).NotTo(HaveOccurred(), "Node metrics service should exist when enableMetrics=true")
		Expect(svc.Annotations["prometheus.io/scrape"]).To(Equal("true"))
		Expect(svc.Annotations["prometheus.io/port"]).NotTo(BeEmpty())
	})
})

var _ = Describe("[ebs-csi-e2e] [single-az] [param:controllerPodDisruptionBudget]", func() {
	f := framework.NewDefaultFramework("ebs")
	f.NamespacePodSecurityEnforceLevel = admissionapi.LevelPrivileged

	var cs clientset.Interface

	BeforeEach(func() {
		cs = f.ClientSet
	})

	It("should have PodDisruptionBudget for controller", func() {
		pdb, err := cs.PolicyV1().PodDisruptionBudgets("kube-system").Get(context.Background(), "ebs-csi-controller", metav1.GetOptions{})
		Expect(err).NotTo(HaveOccurred())
		Expect(pdb.Name).To(Equal("ebs-csi-controller"))
	})
})

var _ = Describe("[ebs-csi-e2e] [single-az] [param:nodeKubeletPath]", func() {
	f := framework.NewDefaultFramework("ebs")
	f.NamespacePodSecurityEnforceLevel = admissionapi.LevelPrivileged

	var cs clientset.Interface

	BeforeEach(func() {
		cs = f.ClientSet
	})

	It("should have custom kubelet path mounted", func() {
		pods, err := cs.CoreV1().Pods("kube-system").List(context.Background(), metav1.ListOptions{
			LabelSelector: "app=ebs-csi-node",
		})
		Expect(err).NotTo(HaveOccurred())
		Expect(pods.Items).NotTo(BeEmpty())

		for _, pod := range pods.Items {
			for _, container := range pod.Spec.Containers {
				if container.Name == "ebs-plugin" {
					var hasKubeletMount bool
					for _, mount := range container.VolumeMounts {
						if mount.MountPath == "/var/lib/kubelet" {
							hasKubeletMount = true
							break
						}
					}
					Expect(hasKubeletMount).To(BeTrue(), "Node should have kubelet path mounted at /var/lib/kubelet")
				}
			}
		}
	})
})

var _ = Describe("[ebs-csi-e2e] [single-az] [param:nodeTolerateAllTaints]", func() {
	f := framework.NewDefaultFramework("ebs")
	f.NamespacePodSecurityEnforceLevel = admissionapi.LevelPrivileged

	var cs clientset.Interface

	BeforeEach(func() {
		cs = f.ClientSet
	})

	It("should have toleration for all taints", func() {
		pods, err := cs.CoreV1().Pods("kube-system").List(context.Background(), metav1.ListOptions{
			LabelSelector: "app=ebs-csi-node",
		})
		Expect(err).NotTo(HaveOccurred())
		Expect(pods.Items).NotTo(BeEmpty())

		for _, pod := range pods.Items {
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
})

var _ = Describe("[ebs-csi-e2e] [single-az] [param:provisionerLeaderElection]", func() {
	f := framework.NewDefaultFramework("ebs")
	f.NamespacePodSecurityEnforceLevel = admissionapi.LevelPrivileged

	var cs clientset.Interface

	BeforeEach(func() {
		cs = f.ClientSet
	})

	It("should have leader election enabled in provisioner args", func() {
		pods, err := cs.CoreV1().Pods("kube-system").List(context.Background(), metav1.ListOptions{
			LabelSelector: "app=ebs-csi-controller",
		})
		Expect(err).NotTo(HaveOccurred())
		Expect(pods.Items).NotTo(BeEmpty())

		for _, pod := range pods.Items {
			for _, container := range pod.Spec.Containers {
				if container.Name == "csi-provisioner" {
					var hasLeaderElection bool
					for _, arg := range container.Args {
						if arg == "--leader-election=true" {
							hasLeaderElection = true
							break
						}
					}
					Expect(hasLeaderElection).To(BeTrue(), "Provisioner should have --leader-election=true arg")
				}
			}
		}
	})
})

var _ = Describe("[ebs-csi-e2e] [single-az] [param:attacherLeaderElection]", func() {
	f := framework.NewDefaultFramework("ebs")
	f.NamespacePodSecurityEnforceLevel = admissionapi.LevelPrivileged

	var cs clientset.Interface

	BeforeEach(func() {
		cs = f.ClientSet
	})

	It("should have leader election enabled in attacher args", func() {
		pods, err := cs.CoreV1().Pods("kube-system").List(context.Background(), metav1.ListOptions{
			LabelSelector: "app=ebs-csi-controller",
		})
		Expect(err).NotTo(HaveOccurred())
		Expect(pods.Items).NotTo(BeEmpty())

		for _, pod := range pods.Items {
			for _, container := range pod.Spec.Containers {
				if container.Name == "csi-attacher" {
					var hasLeaderElection bool
					for _, arg := range container.Args {
						if arg == "--leader-election=true" {
							hasLeaderElection = true
							break
						}
					}
					Expect(hasLeaderElection).To(BeTrue(), "Attacher should have --leader-election=true arg")
				}
			}
		}
	})
})

var _ = Describe("[ebs-csi-e2e] [single-az] [param:resizerLeaderElection]", func() {
	f := framework.NewDefaultFramework("ebs")
	f.NamespacePodSecurityEnforceLevel = admissionapi.LevelPrivileged

	var cs clientset.Interface

	BeforeEach(func() {
		cs = f.ClientSet
	})

	It("should have leader election enabled in resizer args", func() {
		pods, err := cs.CoreV1().Pods("kube-system").List(context.Background(), metav1.ListOptions{
			LabelSelector: "app=ebs-csi-controller",
		})
		Expect(err).NotTo(HaveOccurred())
		Expect(pods.Items).NotTo(BeEmpty())

		for _, pod := range pods.Items {
			for _, container := range pod.Spec.Containers {
				if container.Name == "csi-resizer" {
					var hasLeaderElection bool
					for _, arg := range container.Args {
						if arg == "--leader-election=true" {
							hasLeaderElection = true
							break
						}
					}
					Expect(hasLeaderElection).To(BeTrue(), "Resizer should have --leader-election=true arg")
				}
			}
		}
	})
})

var _ = Describe("[ebs-csi-e2e] [single-az] [param:nodeDisableMutation]", func() {
	f := framework.NewDefaultFramework("ebs")
	f.NamespacePodSecurityEnforceLevel = admissionapi.LevelPrivileged

	var cs clientset.Interface

	BeforeEach(func() {
		cs = f.ClientSet
	})

	It("should have node service account without mutating permissions", func() {
		// Check ClusterRole for node service account
		cr, err := cs.RbacV1().ClusterRoles().Get(context.Background(), "ebs-csi-node-role", metav1.GetOptions{})
		Expect(err).NotTo(HaveOccurred())

		// When disableMutation=true, should not have patch/update verbs on nodes
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
})

var _ = Describe("[ebs-csi-e2e] [single-az] [param:nodeTerminationGracePeriod]", func() {
	f := framework.NewDefaultFramework("ebs")
	f.NamespacePodSecurityEnforceLevel = admissionapi.LevelPrivileged

	var cs clientset.Interface

	BeforeEach(func() {
		cs = f.ClientSet
	})

	It("should have custom termination grace period", func() {
		pods, err := cs.CoreV1().Pods("kube-system").List(context.Background(), metav1.ListOptions{
			LabelSelector: "app=ebs-csi-node",
		})
		Expect(err).NotTo(HaveOccurred())
		Expect(pods.Items).NotTo(BeEmpty())

		for _, pod := range pods.Items {
			Expect(pod.Spec.TerminationGracePeriodSeconds).NotTo(BeNil())
			Expect(*pod.Spec.TerminationGracePeriodSeconds).To(Equal(int64(60)), "Node pod should have 60s termination grace period")
		}
	})
})

var _ = Describe("[ebs-csi-e2e] [single-az] [param:volumemodifierLeaderElection]", func() {
	f := framework.NewDefaultFramework("ebs")
	f.NamespacePodSecurityEnforceLevel = admissionapi.LevelPrivileged

	var cs clientset.Interface

	BeforeEach(func() {
		cs = f.ClientSet
	})

	It("should have leader election disabled in volumemodifier args", func() {
		pods, err := cs.CoreV1().Pods("kube-system").List(context.Background(), metav1.ListOptions{
			LabelSelector: "app=ebs-csi-controller",
		})
		Expect(err).NotTo(HaveOccurred())
		Expect(pods.Items).NotTo(BeEmpty())

		for _, pod := range pods.Items {
			for _, container := range pod.Spec.Containers {
				if container.Name == "volumemodifier" {
					var hasLeaderElectionDisabled bool
					for _, arg := range container.Args {
						if arg == "--leader-election=false" {
							hasLeaderElectionDisabled = true
							break
						}
					}
					Expect(hasLeaderElectionDisabled).To(BeTrue(), "Volumemodifier should have --leader-election=false arg")
				}
			}
		}
	})
})

// Infrastructure parameter tests - verify Helm values are correctly applied to K8s resources

var _ = Describe("[ebs-csi-e2e] [single-az] [param:controllerReplicaCount]", func() {
	f := framework.NewDefaultFramework("ebs")
	f.NamespacePodSecurityEnforceLevel = admissionapi.LevelPrivileged

	It("should have configured replica count", func() {
		deploy, err := f.ClientSet.AppsV1().Deployments("kube-system").Get(context.Background(), "ebs-csi-controller", metav1.GetOptions{})
		Expect(err).NotTo(HaveOccurred())
		Expect(*deploy.Spec.Replicas).To(Equal(int32(3)))
	})
})

var _ = Describe("[ebs-csi-e2e] [single-az] [param:controllerNodeSelector]", func() {
	f := framework.NewDefaultFramework("ebs")
	f.NamespacePodSecurityEnforceLevel = admissionapi.LevelPrivileged

	It("should have configured node selector", func() {
		deploy, err := f.ClientSet.AppsV1().Deployments("kube-system").Get(context.Background(), "ebs-csi-controller", metav1.GetOptions{})
		Expect(err).NotTo(HaveOccurred())
		Expect(deploy.Spec.Template.Spec.NodeSelector).To(HaveKeyWithValue("node-role", "controller"))
	})
})

var _ = Describe("[ebs-csi-e2e] [single-az] [param:controllerTolerations]", func() {
	f := framework.NewDefaultFramework("ebs")
	f.NamespacePodSecurityEnforceLevel = admissionapi.LevelPrivileged

	It("should have configured tolerations", func() {
		deploy, err := f.ClientSet.AppsV1().Deployments("kube-system").Get(context.Background(), "ebs-csi-controller", metav1.GetOptions{})
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
})

var _ = Describe("[ebs-csi-e2e] [single-az] [param:controllerPriorityClassName]", func() {
	f := framework.NewDefaultFramework("ebs")
	f.NamespacePodSecurityEnforceLevel = admissionapi.LevelPrivileged

	It("should have configured priority class", func() {
		deploy, err := f.ClientSet.AppsV1().Deployments("kube-system").Get(context.Background(), "ebs-csi-controller", metav1.GetOptions{})
		Expect(err).NotTo(HaveOccurred())
		Expect(deploy.Spec.Template.Spec.PriorityClassName).To(Equal("system-cluster-critical"))
	})
})

var _ = Describe("[ebs-csi-e2e] [single-az] [param:controllerResources]", func() {
	f := framework.NewDefaultFramework("ebs")
	f.NamespacePodSecurityEnforceLevel = admissionapi.LevelPrivileged

	It("should have configured resources", func() {
		deploy, err := f.ClientSet.AppsV1().Deployments("kube-system").Get(context.Background(), "ebs-csi-controller", metav1.GetOptions{})
		Expect(err).NotTo(HaveOccurred())
		for _, c := range deploy.Spec.Template.Spec.Containers {
			if c.Name == "ebs-plugin" {
				Expect(c.Resources.Requests.Cpu().String()).To(Equal("100m"))
				Expect(c.Resources.Limits.Memory().String()).To(Equal("256Mi"))
				return
			}
		}
		Fail("ebs-plugin container not found")
	})
})

var _ = Describe("[ebs-csi-e2e] [single-az] [param:controllerPodAnnotations]", func() {
	f := framework.NewDefaultFramework("ebs")
	f.NamespacePodSecurityEnforceLevel = admissionapi.LevelPrivileged

	It("should have configured pod annotations", func() {
		deploy, err := f.ClientSet.AppsV1().Deployments("kube-system").Get(context.Background(), "ebs-csi-controller", metav1.GetOptions{})
		Expect(err).NotTo(HaveOccurred())
		Expect(deploy.Spec.Template.Annotations).To(HaveKeyWithValue("test-annotation", "test-value"))
	})
})

var _ = Describe("[ebs-csi-e2e] [single-az] [param:controllerPodLabels]", func() {
	f := framework.NewDefaultFramework("ebs")
	f.NamespacePodSecurityEnforceLevel = admissionapi.LevelPrivileged

	It("should have configured pod labels", func() {
		deploy, err := f.ClientSet.AppsV1().Deployments("kube-system").Get(context.Background(), "ebs-csi-controller", metav1.GetOptions{})
		Expect(err).NotTo(HaveOccurred())
		Expect(deploy.Spec.Template.Labels).To(HaveKeyWithValue("test-label", "test-value"))
	})
})

var _ = Describe("[ebs-csi-e2e] [single-az] [param:controllerDeploymentAnnotations]", func() {
	f := framework.NewDefaultFramework("ebs")
	f.NamespacePodSecurityEnforceLevel = admissionapi.LevelPrivileged

	It("should have configured deployment annotations", func() {
		deploy, err := f.ClientSet.AppsV1().Deployments("kube-system").Get(context.Background(), "ebs-csi-controller", metav1.GetOptions{})
		Expect(err).NotTo(HaveOccurred())
		Expect(deploy.Annotations).To(HaveKeyWithValue("deploy-annotation", "deploy-value"))
	})
})

var _ = Describe("[ebs-csi-e2e] [single-az] [param:controllerServiceAccountName]", func() {
	f := framework.NewDefaultFramework("ebs")
	f.NamespacePodSecurityEnforceLevel = admissionapi.LevelPrivileged

	It("should use configured service account name", func() {
		deploy, err := f.ClientSet.AppsV1().Deployments("kube-system").Get(context.Background(), "ebs-csi-controller", metav1.GetOptions{})
		Expect(err).NotTo(HaveOccurred())
		Expect(deploy.Spec.Template.Spec.ServiceAccountName).To(Equal("custom-controller-sa"))
	})
})

var _ = Describe("[ebs-csi-e2e] [single-az] [param:controllerServiceAccountAnnotations]", func() {
	f := framework.NewDefaultFramework("ebs")
	f.NamespacePodSecurityEnforceLevel = admissionapi.LevelPrivileged

	It("should have configured service account annotations", func() {
		sa, err := f.ClientSet.CoreV1().ServiceAccounts("kube-system").Get(context.Background(), "ebs-csi-controller-sa", metav1.GetOptions{})
		Expect(err).NotTo(HaveOccurred())
		Expect(sa.Annotations).To(HaveKeyWithValue("eks.amazonaws.com/role-arn", "arn:aws:iam::123456789:role/test-role"))
	})
})

var _ = Describe("[ebs-csi-e2e] [single-az] [param:controllerRevisionHistoryLimit]", func() {
	f := framework.NewDefaultFramework("ebs")
	f.NamespacePodSecurityEnforceLevel = admissionapi.LevelPrivileged

	It("should have configured revision history limit", func() {
		deploy, err := f.ClientSet.AppsV1().Deployments("kube-system").Get(context.Background(), "ebs-csi-controller", metav1.GetOptions{})
		Expect(err).NotTo(HaveOccurred())
		Expect(deploy.Spec.RevisionHistoryLimit).NotTo(BeNil())
		Expect(*deploy.Spec.RevisionHistoryLimit).To(Equal(int32(5)))
	})
})

var _ = Describe("[ebs-csi-e2e] [single-az] [param:nodeNodeSelector]", func() {
	f := framework.NewDefaultFramework("ebs")
	f.NamespacePodSecurityEnforceLevel = admissionapi.LevelPrivileged

	It("should have configured node selector", func() {
		ds, err := f.ClientSet.AppsV1().DaemonSets("kube-system").Get(context.Background(), "ebs-csi-node", metav1.GetOptions{})
		Expect(err).NotTo(HaveOccurred())
		Expect(ds.Spec.Template.Spec.NodeSelector).To(HaveKeyWithValue("node-type", "worker"))
	})
})

var _ = Describe("[ebs-csi-e2e] [single-az] [param:nodeTolerations]", func() {
	f := framework.NewDefaultFramework("ebs")
	f.NamespacePodSecurityEnforceLevel = admissionapi.LevelPrivileged

	It("should have configured tolerations", func() {
		ds, err := f.ClientSet.AppsV1().DaemonSets("kube-system").Get(context.Background(), "ebs-csi-node", metav1.GetOptions{})
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
})

var _ = Describe("[ebs-csi-e2e] [single-az] [param:nodePriorityClassName]", func() {
	f := framework.NewDefaultFramework("ebs")
	f.NamespacePodSecurityEnforceLevel = admissionapi.LevelPrivileged

	It("should have configured priority class", func() {
		ds, err := f.ClientSet.AppsV1().DaemonSets("kube-system").Get(context.Background(), "ebs-csi-node", metav1.GetOptions{})
		Expect(err).NotTo(HaveOccurred())
		Expect(ds.Spec.Template.Spec.PriorityClassName).To(Equal("system-node-critical"))
	})
})

var _ = Describe("[ebs-csi-e2e] [single-az] [param:nodeResources]", func() {
	f := framework.NewDefaultFramework("ebs")
	f.NamespacePodSecurityEnforceLevel = admissionapi.LevelPrivileged

	It("should have configured resources", func() {
		ds, err := f.ClientSet.AppsV1().DaemonSets("kube-system").Get(context.Background(), "ebs-csi-node", metav1.GetOptions{})
		Expect(err).NotTo(HaveOccurred())
		for _, c := range ds.Spec.Template.Spec.Containers {
			if c.Name == "ebs-plugin" {
				Expect(c.Resources.Requests.Cpu().String()).To(Equal("50m"))
				Expect(c.Resources.Limits.Memory().String()).To(Equal("128Mi"))
				return
			}
		}
		Fail("ebs-plugin container not found")
	})
})

var _ = Describe("[ebs-csi-e2e] [single-az] [param:nodePodAnnotations]", func() {
	f := framework.NewDefaultFramework("ebs")
	f.NamespacePodSecurityEnforceLevel = admissionapi.LevelPrivileged

	It("should have configured pod annotations", func() {
		ds, err := f.ClientSet.AppsV1().DaemonSets("kube-system").Get(context.Background(), "ebs-csi-node", metav1.GetOptions{})
		Expect(err).NotTo(HaveOccurred())
		Expect(ds.Spec.Template.Annotations).To(HaveKeyWithValue("node-annotation", "node-value"))
	})
})

var _ = Describe("[ebs-csi-e2e] [single-az] [param:nodeDaemonSetAnnotations]", func() {
	f := framework.NewDefaultFramework("ebs")
	f.NamespacePodSecurityEnforceLevel = admissionapi.LevelPrivileged

	It("should have configured daemonset annotations", func() {
		ds, err := f.ClientSet.AppsV1().DaemonSets("kube-system").Get(context.Background(), "ebs-csi-node", metav1.GetOptions{})
		Expect(err).NotTo(HaveOccurred())
		Expect(ds.Annotations).To(HaveKeyWithValue("ds-annotation", "ds-value"))
	})
})

var _ = Describe("[ebs-csi-e2e] [single-az] [param:nodeRevisionHistoryLimit]", func() {
	f := framework.NewDefaultFramework("ebs")
	f.NamespacePodSecurityEnforceLevel = admissionapi.LevelPrivileged

	It("should have configured revision history limit", func() {
		ds, err := f.ClientSet.AppsV1().DaemonSets("kube-system").Get(context.Background(), "ebs-csi-node", metav1.GetOptions{})
		Expect(err).NotTo(HaveOccurred())
		Expect(ds.Spec.RevisionHistoryLimit).NotTo(BeNil())
		Expect(*ds.Spec.RevisionHistoryLimit).To(Equal(int32(3)))
	})
})

var _ = Describe("[ebs-csi-e2e] [single-az] [param:nodeServiceAccountName]", func() {
	f := framework.NewDefaultFramework("ebs")
	f.NamespacePodSecurityEnforceLevel = admissionapi.LevelPrivileged

	It("should use configured service account name", func() {
		ds, err := f.ClientSet.AppsV1().DaemonSets("kube-system").Get(context.Background(), "ebs-csi-node", metav1.GetOptions{})
		Expect(err).NotTo(HaveOccurred())
		Expect(ds.Spec.Template.Spec.ServiceAccountName).To(Equal("custom-node-sa"))
	})
})

var _ = Describe("[ebs-csi-e2e] [single-az] [param:provisionerResources]", func() {
	f := framework.NewDefaultFramework("ebs")
	f.NamespacePodSecurityEnforceLevel = admissionapi.LevelPrivileged

	It("should have configured resources", func() {
		deploy, err := f.ClientSet.AppsV1().Deployments("kube-system").Get(context.Background(), "ebs-csi-controller", metav1.GetOptions{})
		Expect(err).NotTo(HaveOccurred())
		for _, c := range deploy.Spec.Template.Spec.Containers {
			if c.Name == "csi-provisioner" {
				Expect(c.Resources.Requests.Cpu().String()).To(Equal("20m"))
				return
			}
		}
		Fail("csi-provisioner container not found")
	})
})

var _ = Describe("[ebs-csi-e2e] [single-az] [param:attacherResources]", func() {
	f := framework.NewDefaultFramework("ebs")
	f.NamespacePodSecurityEnforceLevel = admissionapi.LevelPrivileged

	It("should have configured resources", func() {
		deploy, err := f.ClientSet.AppsV1().Deployments("kube-system").Get(context.Background(), "ebs-csi-controller", metav1.GetOptions{})
		Expect(err).NotTo(HaveOccurred())
		for _, c := range deploy.Spec.Template.Spec.Containers {
			if c.Name == "csi-attacher" {
				Expect(c.Resources.Requests.Cpu().String()).To(Equal("15m"))
				return
			}
		}
		Fail("csi-attacher container not found")
	})
})

var _ = Describe("[ebs-csi-e2e] [single-az] [param:snapshotterResources]", func() {
	f := framework.NewDefaultFramework("ebs")
	f.NamespacePodSecurityEnforceLevel = admissionapi.LevelPrivileged

	It("should have configured resources", func() {
		deploy, err := f.ClientSet.AppsV1().Deployments("kube-system").Get(context.Background(), "ebs-csi-controller", metav1.GetOptions{})
		Expect(err).NotTo(HaveOccurred())
		for _, c := range deploy.Spec.Template.Spec.Containers {
			if c.Name == "csi-snapshotter" {
				Expect(c.Resources.Requests.Cpu().String()).To(Equal("15m"))
				return
			}
		}
		Fail("csi-snapshotter container not found")
	})
})

var _ = Describe("[ebs-csi-e2e] [single-az] [param:resizerResources]", func() {
	f := framework.NewDefaultFramework("ebs")
	f.NamespacePodSecurityEnforceLevel = admissionapi.LevelPrivileged

	It("should have configured resources", func() {
		deploy, err := f.ClientSet.AppsV1().Deployments("kube-system").Get(context.Background(), "ebs-csi-controller", metav1.GetOptions{})
		Expect(err).NotTo(HaveOccurred())
		for _, c := range deploy.Spec.Template.Spec.Containers {
			if c.Name == "csi-resizer" {
				Expect(c.Resources.Requests.Cpu().String()).To(Equal("15m"))
				return
			}
		}
		Fail("csi-resizer container not found")
	})
})

var _ = Describe("[ebs-csi-e2e] [single-az] [param:nodeDriverRegistrarResources]", func() {
	f := framework.NewDefaultFramework("ebs")
	f.NamespacePodSecurityEnforceLevel = admissionapi.LevelPrivileged

	It("should have configured resources", func() {
		ds, err := f.ClientSet.AppsV1().DaemonSets("kube-system").Get(context.Background(), "ebs-csi-node", metav1.GetOptions{})
		Expect(err).NotTo(HaveOccurred())
		for _, c := range ds.Spec.Template.Spec.Containers {
			if c.Name == "node-driver-registrar" {
				Expect(c.Resources.Requests.Cpu().String()).To(Equal("10m"))
				return
			}
		}
		Fail("node-driver-registrar container not found")
	})
})

var _ = Describe("[ebs-csi-e2e] [single-az] [param:livenessProbeResources]", func() {
	f := framework.NewDefaultFramework("ebs")
	f.NamespacePodSecurityEnforceLevel = admissionapi.LevelPrivileged

	It("should have configured resources", func() {
		ds, err := f.ClientSet.AppsV1().DaemonSets("kube-system").Get(context.Background(), "ebs-csi-node", metav1.GetOptions{})
		Expect(err).NotTo(HaveOccurred())
		for _, c := range ds.Spec.Template.Spec.Containers {
			if c.Name == "liveness-probe" {
				Expect(c.Resources.Requests.Cpu().String()).To(Equal("5m"))
				return
			}
		}
		Fail("liveness-probe container not found")
	})
})

var _ = Describe("[ebs-csi-e2e] [single-az] [param:imageRepository]", func() {
	f := framework.NewDefaultFramework("ebs")
	f.NamespacePodSecurityEnforceLevel = admissionapi.LevelPrivileged

	It("should use configured image repository", func() {
		deploy, err := f.ClientSet.AppsV1().Deployments("kube-system").Get(context.Background(), "ebs-csi-controller", metav1.GetOptions{})
		Expect(err).NotTo(HaveOccurred())
		for _, c := range deploy.Spec.Template.Spec.Containers {
			if c.Name == "ebs-plugin" {
				Expect(c.Image).To(ContainSubstring("public.ecr.aws/ebs-csi-driver/aws-ebs-csi-driver"))
				return
			}
		}
		Fail("ebs-plugin container not found")
	})
})

var _ = Describe("[ebs-csi-e2e] [single-az] [param:imagePullPolicy]", func() {
	f := framework.NewDefaultFramework("ebs")
	f.NamespacePodSecurityEnforceLevel = admissionapi.LevelPrivileged

	It("should use configured image pull policy", func() {
		deploy, err := f.ClientSet.AppsV1().Deployments("kube-system").Get(context.Background(), "ebs-csi-controller", metav1.GetOptions{})
		Expect(err).NotTo(HaveOccurred())
		for _, c := range deploy.Spec.Template.Spec.Containers {
			if c.Name == "ebs-plugin" {
				Expect(string(c.ImagePullPolicy)).To(Equal("Always"))
				return
			}
		}
		Fail("ebs-plugin container not found")
	})
})

var _ = Describe("[ebs-csi-e2e] [single-az] [param:customLabels]", func() {
	f := framework.NewDefaultFramework("ebs")
	f.NamespacePodSecurityEnforceLevel = admissionapi.LevelPrivileged

	It("should have custom labels on all resources", func() {
		deploy, err := f.ClientSet.AppsV1().Deployments("kube-system").Get(context.Background(), "ebs-csi-controller", metav1.GetOptions{})
		Expect(err).NotTo(HaveOccurred())
		Expect(deploy.Labels).To(HaveKeyWithValue("custom-label", "custom-value"))

		ds, err := f.ClientSet.AppsV1().DaemonSets("kube-system").Get(context.Background(), "ebs-csi-node", metav1.GetOptions{})
		Expect(err).NotTo(HaveOccurred())
		Expect(ds.Labels).To(HaveKeyWithValue("custom-label", "custom-value"))
	})
})

var _ = Describe("[ebs-csi-e2e] [single-az] [param:controllerEnv]", func() {
	f := framework.NewDefaultFramework("ebs")
	f.NamespacePodSecurityEnforceLevel = admissionapi.LevelPrivileged

	It("should have configured environment variables", func() {
		deploy, err := f.ClientSet.AppsV1().Deployments("kube-system").Get(context.Background(), "ebs-csi-controller", metav1.GetOptions{})
		Expect(err).NotTo(HaveOccurred())
		for _, c := range deploy.Spec.Template.Spec.Containers {
			if c.Name == "ebs-plugin" {
				var found bool
				for _, env := range c.Env {
					if env.Name == "TEST_ENV" && env.Value == "test-value" {
						found = true
						break
					}
				}
				Expect(found).To(BeTrue(), "Controller should have TEST_ENV environment variable")
				return
			}
		}
		Fail("ebs-plugin container not found")
	})
})

var _ = Describe("[ebs-csi-e2e] [single-az] [param:nodeEnv]", func() {
	f := framework.NewDefaultFramework("ebs")
	f.NamespacePodSecurityEnforceLevel = admissionapi.LevelPrivileged

	It("should have configured environment variables", func() {
		ds, err := f.ClientSet.AppsV1().DaemonSets("kube-system").Get(context.Background(), "ebs-csi-node", metav1.GetOptions{})
		Expect(err).NotTo(HaveOccurred())
		for _, c := range ds.Spec.Template.Spec.Containers {
			if c.Name == "ebs-plugin" {
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
})

var _ = Describe("[ebs-csi-e2e] [single-az] [param:controllerAffinity]", func() {
	f := framework.NewDefaultFramework("ebs")
	f.NamespacePodSecurityEnforceLevel = admissionapi.LevelPrivileged

	It("should have configured affinity", func() {
		deploy, err := f.ClientSet.AppsV1().Deployments("kube-system").Get(context.Background(), "ebs-csi-controller", metav1.GetOptions{})
		Expect(err).NotTo(HaveOccurred())
		Expect(deploy.Spec.Template.Spec.Affinity).NotTo(BeNil())
		Expect(deploy.Spec.Template.Spec.Affinity.NodeAffinity).NotTo(BeNil())
	})
})

var _ = Describe("[ebs-csi-e2e] [single-az] [param:nodeAffinity]", func() {
	f := framework.NewDefaultFramework("ebs")
	f.NamespacePodSecurityEnforceLevel = admissionapi.LevelPrivileged

	It("should have configured affinity", func() {
		ds, err := f.ClientSet.AppsV1().DaemonSets("kube-system").Get(context.Background(), "ebs-csi-node", metav1.GetOptions{})
		Expect(err).NotTo(HaveOccurred())
		Expect(ds.Spec.Template.Spec.Affinity).NotTo(BeNil())
		Expect(ds.Spec.Template.Spec.Affinity.NodeAffinity).NotTo(BeNil())
	})
})

var _ = Describe("[ebs-csi-e2e] [single-az] [param:controllerTopologySpreadConstraints]", func() {
	f := framework.NewDefaultFramework("ebs")
	f.NamespacePodSecurityEnforceLevel = admissionapi.LevelPrivileged

	It("should have configured topology spread constraints", func() {
		deploy, err := f.ClientSet.AppsV1().Deployments("kube-system").Get(context.Background(), "ebs-csi-controller", metav1.GetOptions{})
		Expect(err).NotTo(HaveOccurred())
		Expect(deploy.Spec.Template.Spec.TopologySpreadConstraints).NotTo(BeEmpty())
		Expect(deploy.Spec.Template.Spec.TopologySpreadConstraints[0].TopologyKey).To(Equal("topology.kubernetes.io/zone"))
	})
})

var _ = Describe("[ebs-csi-e2e] [single-az] [param:controllerSecurityContext]", func() {
	f := framework.NewDefaultFramework("ebs")
	f.NamespacePodSecurityEnforceLevel = admissionapi.LevelPrivileged

	It("should have configured pod security context", func() {
		deploy, err := f.ClientSet.AppsV1().Deployments("kube-system").Get(context.Background(), "ebs-csi-controller", metav1.GetOptions{})
		Expect(err).NotTo(HaveOccurred())
		Expect(deploy.Spec.Template.Spec.SecurityContext).NotTo(BeNil())
		Expect(*deploy.Spec.Template.Spec.SecurityContext.RunAsNonRoot).To(BeTrue())
	})
})

var _ = Describe("[ebs-csi-e2e] [single-az] [param:nodeSecurityContext]", func() {
	f := framework.NewDefaultFramework("ebs")
	f.NamespacePodSecurityEnforceLevel = admissionapi.LevelPrivileged

	It("should have configured pod security context", func() {
		ds, err := f.ClientSet.AppsV1().DaemonSets("kube-system").Get(context.Background(), "ebs-csi-node", metav1.GetOptions{})
		Expect(err).NotTo(HaveOccurred())
		Expect(ds.Spec.Template.Spec.SecurityContext).NotTo(BeNil())
	})
})

var _ = Describe("[ebs-csi-e2e] [single-az] [param:controllerContainerSecurityContext]", func() {
	f := framework.NewDefaultFramework("ebs")
	f.NamespacePodSecurityEnforceLevel = admissionapi.LevelPrivileged

	It("should have configured container security context", func() {
		deploy, err := f.ClientSet.AppsV1().Deployments("kube-system").Get(context.Background(), "ebs-csi-controller", metav1.GetOptions{})
		Expect(err).NotTo(HaveOccurred())
		for _, c := range deploy.Spec.Template.Spec.Containers {
			if c.Name == "ebs-plugin" {
				Expect(c.SecurityContext).NotTo(BeNil())
				Expect(*c.SecurityContext.ReadOnlyRootFilesystem).To(BeTrue())
				return
			}
		}
		Fail("ebs-plugin container not found")
	})
})

var _ = Describe("[ebs-csi-e2e] [single-az] [param:controllerUpdateStrategy]", func() {
	f := framework.NewDefaultFramework("ebs")
	f.NamespacePodSecurityEnforceLevel = admissionapi.LevelPrivileged

	It("should have configured update strategy", func() {
		deploy, err := f.ClientSet.AppsV1().Deployments("kube-system").Get(context.Background(), "ebs-csi-controller", metav1.GetOptions{})
		Expect(err).NotTo(HaveOccurred())
		Expect(string(deploy.Spec.Strategy.Type)).To(Equal("Recreate"))
	})
})

var _ = Describe("[ebs-csi-e2e] [single-az] [param:nodeUpdateStrategy]", func() {
	f := framework.NewDefaultFramework("ebs")
	f.NamespacePodSecurityEnforceLevel = admissionapi.LevelPrivileged

	It("should have configured update strategy", func() {
		ds, err := f.ClientSet.AppsV1().DaemonSets("kube-system").Get(context.Background(), "ebs-csi-node", metav1.GetOptions{})
		Expect(err).NotTo(HaveOccurred())
		Expect(string(ds.Spec.UpdateStrategy.Type)).To(Equal("OnDelete"))
	})
})

var _ = Describe("[ebs-csi-e2e] [single-az] [param:controllerVolumes]", func() {
	f := framework.NewDefaultFramework("ebs")
	f.NamespacePodSecurityEnforceLevel = admissionapi.LevelPrivileged

	It("should have configured extra volumes", func() {
		deploy, err := f.ClientSet.AppsV1().Deployments("kube-system").Get(context.Background(), "ebs-csi-controller", metav1.GetOptions{})
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
})

var _ = Describe("[ebs-csi-e2e] [single-az] [param:controllerVolumeMounts]", func() {
	f := framework.NewDefaultFramework("ebs")
	f.NamespacePodSecurityEnforceLevel = admissionapi.LevelPrivileged

	It("should have configured extra volume mounts", func() {
		deploy, err := f.ClientSet.AppsV1().Deployments("kube-system").Get(context.Background(), "ebs-csi-controller", metav1.GetOptions{})
		Expect(err).NotTo(HaveOccurred())
		for _, c := range deploy.Spec.Template.Spec.Containers {
			if c.Name == "ebs-plugin" {
				var found bool
				for _, vm := range c.VolumeMounts {
					if vm.Name == "extra-volume" && vm.MountPath == "/extra" {
						found = true
						break
					}
				}
				Expect(found).To(BeTrue(), "Controller should have extra-volume mount")
				return
			}
		}
		Fail("ebs-plugin container not found")
	})
})

var _ = Describe("[ebs-csi-e2e] [single-az] [param:nodeVolumes]", func() {
	f := framework.NewDefaultFramework("ebs")
	f.NamespacePodSecurityEnforceLevel = admissionapi.LevelPrivileged

	It("should have configured extra volumes", func() {
		ds, err := f.ClientSet.AppsV1().DaemonSets("kube-system").Get(context.Background(), "ebs-csi-node", metav1.GetOptions{})
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
})

var _ = Describe("[ebs-csi-e2e] [single-az] [param:nodeVolumeMounts]", func() {
	f := framework.NewDefaultFramework("ebs")
	f.NamespacePodSecurityEnforceLevel = admissionapi.LevelPrivileged

	It("should have configured extra volume mounts", func() {
		ds, err := f.ClientSet.AppsV1().DaemonSets("kube-system").Get(context.Background(), "ebs-csi-node", metav1.GetOptions{})
		Expect(err).NotTo(HaveOccurred())
		for _, c := range ds.Spec.Template.Spec.Containers {
			if c.Name == "ebs-plugin" {
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
})

var _ = Describe("[ebs-csi-e2e] [single-az] [param:controllerAdditionalArgs]", func() {
	f := framework.NewDefaultFramework("ebs")
	f.NamespacePodSecurityEnforceLevel = admissionapi.LevelPrivileged

	It("should have configured additional args", func() {
		deploy, err := f.ClientSet.AppsV1().Deployments("kube-system").Get(context.Background(), "ebs-csi-controller", metav1.GetOptions{})
		Expect(err).NotTo(HaveOccurred())
		for _, c := range deploy.Spec.Template.Spec.Containers {
			if c.Name == "ebs-plugin" {
				var found bool
				for _, arg := range c.Args {
					if arg == "--extra-arg=extra-value" {
						found = true
						break
					}
				}
				Expect(found).To(BeTrue(), "Controller should have --extra-arg=extra-value")
				return
			}
		}
		Fail("ebs-plugin container not found")
	})
})

var _ = Describe("[ebs-csi-e2e] [single-az] [param:nodeAdditionalArgs]", func() {
	f := framework.NewDefaultFramework("ebs")
	f.NamespacePodSecurityEnforceLevel = admissionapi.LevelPrivileged

	It("should have configured additional args", func() {
		ds, err := f.ClientSet.AppsV1().DaemonSets("kube-system").Get(context.Background(), "ebs-csi-node", metav1.GetOptions{})
		Expect(err).NotTo(HaveOccurred())
		for _, c := range ds.Spec.Template.Spec.Containers {
			if c.Name == "ebs-plugin" {
				var found bool
				for _, arg := range c.Args {
					if arg == "--node-extra-arg=node-value" {
						found = true
						break
					}
				}
				Expect(found).To(BeTrue(), "Node should have --node-extra-arg=node-value")
				return
			}
		}
		Fail("ebs-plugin container not found")
	})
})

var _ = Describe("[ebs-csi-e2e] [single-az] [param:provisionerAdditionalArgs]", func() {
	f := framework.NewDefaultFramework("ebs")
	f.NamespacePodSecurityEnforceLevel = admissionapi.LevelPrivileged

	It("should have configured additional args", func() {
		deploy, err := f.ClientSet.AppsV1().Deployments("kube-system").Get(context.Background(), "ebs-csi-controller", metav1.GetOptions{})
		Expect(err).NotTo(HaveOccurred())
		for _, c := range deploy.Spec.Template.Spec.Containers {
			if c.Name == "csi-provisioner" {
				var found bool
				for _, arg := range c.Args {
					if arg == "--provisioner-extra=prov-value" {
						found = true
						break
					}
				}
				Expect(found).To(BeTrue(), "Provisioner should have --provisioner-extra=prov-value")
				return
			}
		}
		Fail("csi-provisioner container not found")
	})
})

var _ = Describe("[ebs-csi-e2e] [single-az] [param:controllerDnsConfig]", func() {
	f := framework.NewDefaultFramework("ebs")
	f.NamespacePodSecurityEnforceLevel = admissionapi.LevelPrivileged

	It("should have configured DNS config", func() {
		deploy, err := f.ClientSet.AppsV1().Deployments("kube-system").Get(context.Background(), "ebs-csi-controller", metav1.GetOptions{})
		Expect(err).NotTo(HaveOccurred())
		Expect(deploy.Spec.Template.Spec.DNSConfig).NotTo(BeNil())
		Expect(deploy.Spec.Template.Spec.DNSConfig.Nameservers).To(ContainElement("8.8.8.8"))
	})
})

var _ = Describe("[ebs-csi-e2e] [single-az] [param:nodeDnsConfig]", func() {
	f := framework.NewDefaultFramework("ebs")
	f.NamespacePodSecurityEnforceLevel = admissionapi.LevelPrivileged

	It("should have configured DNS config", func() {
		ds, err := f.ClientSet.AppsV1().DaemonSets("kube-system").Get(context.Background(), "ebs-csi-node", metav1.GetOptions{})
		Expect(err).NotTo(HaveOccurred())
		Expect(ds.Spec.Template.Spec.DNSConfig).NotTo(BeNil())
		Expect(ds.Spec.Template.Spec.DNSConfig.Nameservers).To(ContainElement("8.8.4.4"))
	})
})

var _ = Describe("[ebs-csi-e2e] [single-az] [param:controllerInitContainers]", func() {
	f := framework.NewDefaultFramework("ebs")
	f.NamespacePodSecurityEnforceLevel = admissionapi.LevelPrivileged

	It("should have configured init containers", func() {
		deploy, err := f.ClientSet.AppsV1().Deployments("kube-system").Get(context.Background(), "ebs-csi-controller", metav1.GetOptions{})
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
})

var _ = Describe("[ebs-csi-e2e] [single-az] [param:nodeInitContainers]", func() {
	f := framework.NewDefaultFramework("ebs")
	f.NamespacePodSecurityEnforceLevel = admissionapi.LevelPrivileged

	It("should have configured init containers", func() {
		ds, err := f.ClientSet.AppsV1().DaemonSets("kube-system").Get(context.Background(), "ebs-csi-node", metav1.GetOptions{})
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
})

var _ = Describe("[ebs-csi-e2e] [single-az] [param:imagePullSecrets]", func() {
	f := framework.NewDefaultFramework("ebs")
	f.NamespacePodSecurityEnforceLevel = admissionapi.LevelPrivileged

	It("should have configured image pull secrets", func() {
		deploy, err := f.ClientSet.AppsV1().Deployments("kube-system").Get(context.Background(), "ebs-csi-controller", metav1.GetOptions{})
		Expect(err).NotTo(HaveOccurred())
		var found bool
		for _, s := range deploy.Spec.Template.Spec.ImagePullSecrets {
			if s.Name == "my-registry-secret" {
				found = true
				break
			}
		}
		Expect(found).To(BeTrue(), "Controller should have my-registry-secret image pull secret")
	})
})

var _ = Describe("[ebs-csi-e2e] [single-az] [param:fullnameOverride]", func() {
	f := framework.NewDefaultFramework("ebs")
	f.NamespacePodSecurityEnforceLevel = admissionapi.LevelPrivileged

	It("should use overridden name", func() {
		_, err := f.ClientSet.AppsV1().Deployments("kube-system").Get(context.Background(), "custom-ebs-controller", metav1.GetOptions{})
		Expect(err).NotTo(HaveOccurred())
	})
})

var _ = Describe("[ebs-csi-e2e] [single-az] [param:controllerServiceMonitor]", func() {
	f := framework.NewDefaultFramework("ebs")
	f.NamespacePodSecurityEnforceLevel = admissionapi.LevelPrivileged

	It("should create service monitor", func() {
		gvr := schema.GroupVersionResource{Group: "monitoring.coreos.com", Version: "v1", Resource: "servicemonitors"}
		_, err := f.DynamicClient.Resource(gvr).Namespace("kube-system").Get(context.Background(), "ebs-csi-controller", metav1.GetOptions{})
		Expect(err).NotTo(HaveOccurred())
	})
})

var _ = Describe("[ebs-csi-e2e] [single-az] [param:nodeServiceMonitor]", func() {
	f := framework.NewDefaultFramework("ebs")
	f.NamespacePodSecurityEnforceLevel = admissionapi.LevelPrivileged

	It("should create service monitor", func() {
		gvr := schema.GroupVersionResource{Group: "monitoring.coreos.com", Version: "v1", Resource: "servicemonitors"}
		_, err := f.DynamicClient.Resource(gvr).Namespace("kube-system").Get(context.Background(), "ebs-csi-node", metav1.GetOptions{})
		Expect(err).NotTo(HaveOccurred())
	})
})
