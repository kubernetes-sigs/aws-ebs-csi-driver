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

package e2e

import (
	"fmt"
	"math/rand"
	"os"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	v1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	clientset "k8s.io/client-go/kubernetes"
	restclientset "k8s.io/client-go/rest"
	"k8s.io/kubernetes/test/e2e/framework"

	"github.com/kubernetes-sigs/aws-ebs-csi-driver/tests/e2e/driver"
	"github.com/kubernetes-sigs/aws-ebs-csi-driver/tests/e2e/testsuites"

	awscloud "github.com/kubernetes-sigs/aws-ebs-csi-driver/pkg/cloud"
	ebscsidriver "github.com/kubernetes-sigs/aws-ebs-csi-driver/pkg/driver"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	admissionapi "k8s.io/pod-security-admission/api"
)

var _ = Describe("[ebs-csi-e2e] [single-az] Dynamic Provisioning", func() {
	f := framework.NewDefaultFramework("ebs")
	f.NamespacePodSecurityEnforceLevel = admissionapi.LevelPrivileged

	var (
		cs          clientset.Interface
		ns          *v1.Namespace
		ebsDriver   driver.PVTestDriver
		volumeTypes = awscloud.ValidVolumeTypes
		fsTypes     = []string{ebscsidriver.FSTypeXfs}
	)

	BeforeEach(func() {
		cs = f.ClientSet
		ns = f.Namespace
		ebsDriver = driver.InitEbsCSIDriver()
	})

	for _, t := range volumeTypes {
		for _, fs := range fsTypes {
			volumeType := t
			fsType := fs

			createVolumeParameters := map[string]string{
				ebscsidriver.VolumeTypeKey: volumeType,
				ebscsidriver.FSTypeKey:     fsType,
			}
			if volumeType == awscloud.VolumeTypeIO1 || volumeType == awscloud.VolumeTypeIO2 {
				createVolumeParameters[ebscsidriver.IopsKey] = testsuites.DefaultIopsIoVolumes
			}

			It(fmt.Sprintf("should create a volume on demand with volume type %q and fs type %q", volumeType, fsType), func() {
				pods := []testsuites.PodDetails{
					{
						Cmd: "echo 'hello world' > /mnt/test-1/data && grep 'hello world' /mnt/test-1/data",
						Volumes: []testsuites.VolumeDetails{
							{
								CreateVolumeParameters: createVolumeParameters,
								ClaimSize:              driver.MinimumSizeForVolumeType(volumeType),
								VolumeMount: testsuites.VolumeMountDetails{
									NameGenerate:      "test-volume-",
									MountPathGenerate: "/mnt/test-",
								},
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
		}
	}

	for _, t := range volumeTypes {
		volumeType := t

		createVolumeParameters := map[string]string{
			ebscsidriver.VolumeTypeKey: volumeType,
			ebscsidriver.FSTypeKey:     ebscsidriver.FSTypeExt4,
			ebscsidriver.EncryptedKey:  "true",
		}
		if volumeType == awscloud.VolumeTypeIO1 || volumeType == awscloud.VolumeTypeIO2 {
			createVolumeParameters[ebscsidriver.IopsKey] = testsuites.DefaultIopsIoVolumes
		}

		It(fmt.Sprintf("should create a volume on demand with volumeType %q and encryption", volumeType), func() {
			pods := []testsuites.PodDetails{
				{
					Cmd: "echo 'hello world' > /mnt/test-1/data && grep 'hello world' /mnt/test-1/data",
					Volumes: []testsuites.VolumeDetails{
						{
							CreateVolumeParameters: createVolumeParameters,
							ClaimSize:              driver.MinimumSizeForVolumeType(volumeType),
							VolumeMount: testsuites.VolumeMountDetails{
								NameGenerate:      "test-volume-",
								MountPathGenerate: "/mnt/test-",
							},
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
	}

	It("should create a volume on demand with provided mountOptions", func() {
		pods := []testsuites.PodDetails{
			{
				Cmd: "echo 'hello world' > /mnt/test-1/data && grep 'hello world' /mnt/test-1/data",
				Volumes: []testsuites.VolumeDetails{
					{
						CreateVolumeParameters: map[string]string{
							ebscsidriver.VolumeTypeKey: awscloud.VolumeTypeGP2,
							ebscsidriver.FSTypeKey:     ebscsidriver.FSTypeExt4,
						},
						MountOptions: []string{"rw"},
						ClaimSize:    driver.MinimumSizeForVolumeType(awscloud.VolumeTypeGP2),
						VolumeMount: testsuites.VolumeMountDetails{
							NameGenerate:      "test-volume-",
							MountPathGenerate: "/mnt/test-",
						},
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

	It("should create multiple PV objects, bind to PVCs and attach all to a single pod", func() {
		volumeBindingMode := storagev1.VolumeBindingWaitForFirstConsumer
		pods := []testsuites.PodDetails{
			{
				Cmd: "echo 'hello world' > /mnt/test-1/data && echo 'hello world' > /mnt/test-2/data && grep 'hello world' /mnt/test-1/data  && grep 'hello world' /mnt/test-2/data",
				Volumes: []testsuites.VolumeDetails{
					{
						CreateVolumeParameters: map[string]string{
							ebscsidriver.VolumeTypeKey: awscloud.VolumeTypeGP2,
							ebscsidriver.FSTypeKey:     ebscsidriver.FSTypeExt3,
						},
						VolumeBindingMode: &volumeBindingMode,
						ClaimSize:         driver.MinimumSizeForVolumeType(awscloud.VolumeTypeGP2),
						VolumeMount: testsuites.VolumeMountDetails{
							NameGenerate:      "test-volume-",
							MountPathGenerate: "/mnt/test-",
						},
					},
					{
						CreateVolumeParameters: map[string]string{
							ebscsidriver.VolumeTypeKey: awscloud.VolumeTypeIO1,
							ebscsidriver.IopsKey:       testsuites.DefaultIopsIoVolumes,
							ebscsidriver.FSTypeKey:     ebscsidriver.FSTypeExt4,
						},
						ClaimSize:         driver.MinimumSizeForVolumeType(awscloud.VolumeTypeIO1),
						VolumeBindingMode: &volumeBindingMode,
						VolumeMount: testsuites.VolumeMountDetails{
							NameGenerate:      "test-volume-",
							MountPathGenerate: "/mnt/test-",
						},
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

	It("should create multiple PV objects, bind to PVCs and attach all to different pods", func() {
		pods := []testsuites.PodDetails{
			{
				Cmd: "echo 'hello world' > /mnt/test-1/data && grep 'hello world' /mnt/test-1/data",
				Volumes: []testsuites.VolumeDetails{
					{
						CreateVolumeParameters: map[string]string{
							ebscsidriver.VolumeTypeKey: awscloud.VolumeTypeGP2,
							ebscsidriver.FSTypeKey:     ebscsidriver.FSTypeExt3,
						},
						ClaimSize: driver.MinimumSizeForVolumeType(awscloud.VolumeTypeGP2),
						VolumeMount: testsuites.VolumeMountDetails{
							NameGenerate:      "test-volume-",
							MountPathGenerate: "/mnt/test-",
						},
					},
				},
			},
			{
				Cmd: "echo 'hello world' > /mnt/test-1/data && grep 'hello world' /mnt/test-1/data",
				Volumes: []testsuites.VolumeDetails{
					{
						CreateVolumeParameters: map[string]string{
							ebscsidriver.VolumeTypeKey: awscloud.VolumeTypeIO1,
							ebscsidriver.IopsKey:       testsuites.DefaultIopsIoVolumes,
							ebscsidriver.FSTypeKey:     ebscsidriver.FSTypeExt4,
						},
						ClaimSize: driver.MinimumSizeForVolumeType(awscloud.VolumeTypeIO1),
						VolumeMount: testsuites.VolumeMountDetails{
							NameGenerate:      "test-volume-",
							MountPathGenerate: "/mnt/test-",
						},
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

	It("should create a raw block volume on demand", func() {
		pods := []testsuites.PodDetails{
			{
				Cmd: "dd if=/dev/zero of=/dev/xvda bs=1024k count=100",
				Volumes: []testsuites.VolumeDetails{
					{
						CreateVolumeParameters: map[string]string{
							ebscsidriver.VolumeTypeKey: awscloud.VolumeTypeGP2,
							ebscsidriver.FSTypeKey:     ebscsidriver.FSTypeExt4,
						},
						ClaimSize:  driver.MinimumSizeForVolumeType(awscloud.VolumeTypeGP2),
						VolumeMode: testsuites.Block,
						VolumeDevice: testsuites.VolumeDeviceDetails{
							NameGenerate: "test-block-volume-",
							DevicePath:   "/dev/xvda",
						},
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

	It("should succeed multi-attach with dynamically provisioned IO2 block device", func() {
		volumeBindingMode := storagev1.VolumeBindingWaitForFirstConsumer
		pods := []testsuites.PodDetails{
			{
				Volumes: []testsuites.VolumeDetails{
					{
						CreateVolumeParameters: map[string]string{
							ebscsidriver.VolumeTypeKey: awscloud.VolumeTypeIO2,
							ebscsidriver.IopsKey:       testsuites.DefaultIopsIoVolumes,
						},
						ClaimSize:  driver.MinimumSizeForVolumeType(awscloud.VolumeTypeIO2),
						VolumeMode: testsuites.Block,
						VolumeDevice: testsuites.VolumeDeviceDetails{
							NameGenerate: "test-block-volume-",
							DevicePath:   "/dev/xvda",
						},
						AccessMode:        v1.ReadWriteMany,
						VolumeBindingMode: &volumeBindingMode,
					},
				},
			},
			{
				Volumes: []testsuites.VolumeDetails{
					{
						CreateVolumeParameters: map[string]string{
							ebscsidriver.VolumeTypeKey: awscloud.VolumeTypeIO2,
							ebscsidriver.IopsKey:       testsuites.DefaultIopsIoVolumes,
						},
						ClaimSize:  driver.MinimumSizeForVolumeType(awscloud.VolumeTypeIO2),
						VolumeMode: testsuites.Block,
						VolumeDevice: testsuites.VolumeDeviceDetails{
							NameGenerate: "test-block-volume-",
							DevicePath:   "/dev/xvda",
						},
						AccessMode:        v1.ReadWriteMany,
						VolumeBindingMode: &volumeBindingMode,
					},
				},
			},
		}
		test := testsuites.DynamicallyProvisionedMultiAttachTest{
			CSIDriver:  ebsDriver,
			Pods:       pods,
			VolumeMode: testsuites.Block,
			VolumeType: awscloud.VolumeTypeIO2,
			AccessMode: v1.ReadWriteMany,
			RunningPod: true,
		}
		test.Run(cs, ns)
	})

	It("should fail to multi-attach dynamically provisioned IO2 block device - not enabled", func() {
		volumeBindingMode := storagev1.VolumeBindingWaitForFirstConsumer
		pods := []testsuites.PodDetails{
			{
				Volumes: []testsuites.VolumeDetails{
					{
						CreateVolumeParameters: map[string]string{
							ebscsidriver.VolumeTypeKey: awscloud.VolumeTypeIO2,
							ebscsidriver.IopsKey:       testsuites.DefaultIopsIoVolumes,
						},
						ClaimSize:  driver.MinimumSizeForVolumeType(awscloud.VolumeTypeIO2),
						VolumeMode: testsuites.Block,
						VolumeDevice: testsuites.VolumeDeviceDetails{
							NameGenerate: "test-block-volume-",
							DevicePath:   "/dev/xvda",
						},
						AccessMode:        v1.ReadWriteOnce,
						VolumeBindingMode: &volumeBindingMode,
					},
				},
			},
			{
				Volumes: []testsuites.VolumeDetails{
					{
						CreateVolumeParameters: map[string]string{
							ebscsidriver.VolumeTypeKey: awscloud.VolumeTypeIO2,
							ebscsidriver.IopsKey:       testsuites.DefaultIopsIoVolumes,
						},
						ClaimSize:  driver.MinimumSizeForVolumeType(awscloud.VolumeTypeIO2),
						VolumeMode: testsuites.Block,
						VolumeDevice: testsuites.VolumeDeviceDetails{
							NameGenerate: "test-block-volume-",
							DevicePath:   "/dev/xvda",
						},
						AccessMode:        v1.ReadWriteOnce,
						VolumeBindingMode: &volumeBindingMode,
					},
				},
			},
		}
		test := testsuites.DynamicallyProvisionedMultiAttachTest{
			CSIDriver:  ebsDriver,
			Pods:       pods,
			VolumeMode: testsuites.Block,
			AccessMode: v1.ReadWriteOnce,
			VolumeType: awscloud.VolumeTypeIO2,
		}
		test.Run(cs, ns)
	})

	It("should fail to multi-attach when VolumeMode is not Block", func() {
		volumeBindingMode := storagev1.VolumeBindingWaitForFirstConsumer
		pods := []testsuites.PodDetails{
			{
				Volumes: []testsuites.VolumeDetails{
					{
						CreateVolumeParameters: map[string]string{
							ebscsidriver.VolumeTypeKey: awscloud.VolumeTypeIO2,
							ebscsidriver.IopsKey:       testsuites.DefaultIopsIoVolumes,
							ebscsidriver.FSTypeKey:     ebscsidriver.FSTypeExt4,
						},
						VolumeMode: testsuites.FileSystem,
						ClaimSize:  driver.MinimumSizeForVolumeType(awscloud.VolumeTypeIO2),
						VolumeMount: testsuites.VolumeMountDetails{
							NameGenerate:      "test-volume-",
							MountPathGenerate: "/mnt/test-",
						},
						AccessMode:        v1.ReadWriteMany,
						VolumeBindingMode: &volumeBindingMode,
					},
				},
			},
			{
				Volumes: []testsuites.VolumeDetails{
					{
						CreateVolumeParameters: map[string]string{
							ebscsidriver.VolumeTypeKey: awscloud.VolumeTypeIO2,
							ebscsidriver.IopsKey:       testsuites.DefaultIopsIoVolumes,
							ebscsidriver.FSTypeKey:     ebscsidriver.FSTypeExt4,
						},
						VolumeMode: testsuites.FileSystem,
						ClaimSize:  driver.MinimumSizeForVolumeType(awscloud.VolumeTypeIO2),
						VolumeMount: testsuites.VolumeMountDetails{
							NameGenerate:      "test-volume-",
							MountPathGenerate: "/mnt/test-",
						},
						AccessMode:        v1.ReadWriteMany,
						VolumeBindingMode: &volumeBindingMode,
					},
				},
			},
		}
		test := testsuites.DynamicallyProvisionedMultiAttachTest{
			CSIDriver:  ebsDriver,
			Pods:       pods,
			VolumeMode: testsuites.FileSystem,
			AccessMode: v1.ReadWriteMany,
			VolumeType: awscloud.VolumeTypeIO2,
			PendingPVC: true,
		}
		test.Run(cs, ns)
	})

	It("should fail to multi-attach non io2 VolumeType", func() {
		volumeBindingMode := storagev1.VolumeBindingWaitForFirstConsumer
		pods := []testsuites.PodDetails{
			{
				Volumes: []testsuites.VolumeDetails{
					{
						CreateVolumeParameters: map[string]string{
							ebscsidriver.VolumeTypeKey: awscloud.VolumeTypeGP3,
						},
						ClaimSize:         driver.MinimumSizeForVolumeType(awscloud.VolumeTypeGP3),
						VolumeBindingMode: &volumeBindingMode,
						VolumeMode:        testsuites.Block,
						VolumeDevice: testsuites.VolumeDeviceDetails{
							NameGenerate: "test-block-volume-",
							DevicePath:   "/dev/xvda",
						},
						AccessMode: v1.ReadWriteMany,
					},
				},
			},
			{
				Volumes: []testsuites.VolumeDetails{
					{
						CreateVolumeParameters: map[string]string{
							ebscsidriver.VolumeTypeKey: awscloud.VolumeTypeGP3,
						},
						ClaimSize:         driver.MinimumSizeForVolumeType(awscloud.VolumeTypeGP3),
						VolumeBindingMode: &volumeBindingMode,
						VolumeMode:        testsuites.Block,
						VolumeDevice: testsuites.VolumeDeviceDetails{
							NameGenerate: "test-block-volume-",
							DevicePath:   "/dev/xvda",
						},
						AccessMode: v1.ReadWriteMany,
					},
				},
			},
		}
		test := testsuites.DynamicallyProvisionedMultiAttachTest{
			CSIDriver:  ebsDriver,
			Pods:       pods,
			VolumeMode: testsuites.FileSystem,
			AccessMode: v1.ReadWriteMany,
			VolumeType: awscloud.VolumeTypeIO2,
			PendingPVC: true,
		}
		test.Run(cs, ns)
	})

	It("should create a raw block volume and a filesystem volume on demand and bind to the same pod", func() {
		volumeBindingMode := storagev1.VolumeBindingWaitForFirstConsumer
		pods := []testsuites.PodDetails{
			{
				Cmd: "dd if=/dev/zero of=/dev/xvda bs=1024k count=100 && echo 'hello world' > /mnt/test-1/data && grep 'hello world' /mnt/test-1/data",
				Volumes: []testsuites.VolumeDetails{
					{
						CreateVolumeParameters: map[string]string{
							ebscsidriver.VolumeTypeKey: awscloud.VolumeTypeIO1,
							ebscsidriver.IopsKey:       testsuites.DefaultIopsIoVolumes,
							ebscsidriver.FSTypeKey:     ebscsidriver.FSTypeExt4,
						},
						ClaimSize:         driver.MinimumSizeForVolumeType(awscloud.VolumeTypeIO1),
						VolumeBindingMode: &volumeBindingMode,
						VolumeMount: testsuites.VolumeMountDetails{
							NameGenerate:      "test-volume-",
							MountPathGenerate: "/mnt/test-",
						},
					},
					{
						CreateVolumeParameters: map[string]string{
							ebscsidriver.VolumeTypeKey: awscloud.VolumeTypeGP2,
							ebscsidriver.FSTypeKey:     ebscsidriver.FSTypeExt4,
						},
						MountOptions:      []string{"rw"},
						ClaimSize:         driver.MinimumSizeForVolumeType(awscloud.VolumeTypeGP2),
						VolumeBindingMode: &volumeBindingMode,
						VolumeMode:        testsuites.Block,
						VolumeDevice: testsuites.VolumeDeviceDetails{
							NameGenerate: "test-block-volume-",
							DevicePath:   "/dev/xvda",
						},
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

	It("should create multiple PV objects, bind to PVCs and attach all to different pods on the same node", func() {
		pods := []testsuites.PodDetails{
			{
				Cmd: "while true; do echo $(date -u) >> /mnt/test-1/data; sleep 1; done",
				Volumes: []testsuites.VolumeDetails{
					{
						CreateVolumeParameters: map[string]string{
							ebscsidriver.VolumeTypeKey: awscloud.VolumeTypeGP2,
							ebscsidriver.FSTypeKey:     ebscsidriver.FSTypeExt3,
						},
						ClaimSize: driver.MinimumSizeForVolumeType(awscloud.VolumeTypeGP2),
						VolumeMount: testsuites.VolumeMountDetails{
							NameGenerate:      "test-volume-",
							MountPathGenerate: "/mnt/test-",
						},
					},
				},
			},
			{
				Cmd: "while true; do echo $(date -u) >> /mnt/test-1/data; sleep 1; done",
				Volumes: []testsuites.VolumeDetails{
					{
						CreateVolumeParameters: map[string]string{
							ebscsidriver.VolumeTypeKey: awscloud.VolumeTypeIO1,
							ebscsidriver.IopsKey:       testsuites.DefaultIopsIoVolumes,
							ebscsidriver.FSTypeKey:     ebscsidriver.FSTypeExt4,
						},
						ClaimSize: driver.MinimumSizeForVolumeType(awscloud.VolumeTypeIO1),
						VolumeMount: testsuites.VolumeMountDetails{
							NameGenerate:      "test-volume-",
							MountPathGenerate: "/mnt/test-",
						},
					},
				},
			},
		}
		test := testsuites.DynamicallyProvisionedCollocatedPodTest{
			CSIDriver:    ebsDriver,
			Pods:         pods,
			ColocatePods: true,
		}
		test.Run(cs, ns)
	})

	// Track issue https://github.com/kubernetes/kubernetes/issues/70505
	It("should create a volume on demand and mount it as readOnly in a pod", func() {
		pods := []testsuites.PodDetails{
			{
				Cmd: "touch /mnt/test-1/data",
				Volumes: []testsuites.VolumeDetails{
					{
						CreateVolumeParameters: map[string]string{
							ebscsidriver.VolumeTypeKey: awscloud.VolumeTypeGP2,
							ebscsidriver.FSTypeKey:     ebscsidriver.FSTypeExt4,
						},
						ClaimSize: driver.MinimumSizeForVolumeType(awscloud.VolumeTypeGP2),
						VolumeMount: testsuites.VolumeMountDetails{
							NameGenerate:      "test-volume-",
							MountPathGenerate: "/mnt/test-",
							ReadOnly:          true,
						},
					},
				},
			},
		}
		test := testsuites.DynamicallyProvisionedReadOnlyVolumeTest{
			CSIDriver: ebsDriver,
			Pods:      pods,
		}
		test.Run(cs, ns)
	})

	It(fmt.Sprintf("should delete PV with reclaimPolicy %q", v1.PersistentVolumeReclaimDelete), func() {
		reclaimPolicy := v1.PersistentVolumeReclaimDelete
		volumes := []testsuites.VolumeDetails{
			{
				CreateVolumeParameters: map[string]string{
					ebscsidriver.VolumeTypeKey: awscloud.VolumeTypeGP2,
					ebscsidriver.FSTypeKey:     ebscsidriver.FSTypeExt4,
				},
				ClaimSize:     driver.MinimumSizeForVolumeType(awscloud.VolumeTypeGP2),
				ReclaimPolicy: &reclaimPolicy,
			},
		}
		test := testsuites.DynamicallyProvisionedReclaimPolicyTest{
			CSIDriver: ebsDriver,
			Volumes:   volumes,
		}
		test.Run(cs, ns)
	})

	It(fmt.Sprintf("[env] should retain PV with reclaimPolicy %q", v1.PersistentVolumeReclaimRetain), func() {
		if os.Getenv(awsAvailabilityZonesEnv) == "" {
			Skip(fmt.Sprintf("env %q not set", awsAvailabilityZonesEnv))
		}
		reclaimPolicy := v1.PersistentVolumeReclaimRetain
		volumes := []testsuites.VolumeDetails{
			{
				CreateVolumeParameters: map[string]string{
					ebscsidriver.VolumeTypeKey: awscloud.VolumeTypeGP2,
					ebscsidriver.FSTypeKey:     ebscsidriver.FSTypeExt4,
				},
				ClaimSize:     driver.MinimumSizeForVolumeType(awscloud.VolumeTypeGP2),
				ReclaimPolicy: &reclaimPolicy,
			},
		}
		availabilityZones := strings.Split(os.Getenv(awsAvailabilityZonesEnv), ",")
		availabilityZone := availabilityZones[rand.Intn(len(availabilityZones))]
		region := availabilityZone[0 : len(availabilityZone)-1]
		cloud, err := awscloud.NewCloud(region, false, "", true, "")
		if err != nil {
			Fail(fmt.Sprintf("could not get NewCloud: %v", err))
		}

		test := testsuites.DynamicallyProvisionedReclaimPolicyTest{
			CSIDriver: ebsDriver,
			Volumes:   volumes,
			Cloud:     cloud,
		}
		test.Run(cs, ns)
	})

	It("should create a deployment object, write and read to it, delete the pod and write and read to it again", func() {
		pod := testsuites.PodDetails{
			Cmd: "echo 'hello world' >> /mnt/test-1/data && while true; do sleep 1; done",
			Volumes: []testsuites.VolumeDetails{
				{
					CreateVolumeParameters: map[string]string{
						ebscsidriver.VolumeTypeKey: awscloud.VolumeTypeGP2,
						ebscsidriver.FSTypeKey:     ebscsidriver.FSTypeExt3,
					},
					ClaimSize: driver.MinimumSizeForVolumeType(awscloud.VolumeTypeGP2),
					VolumeMount: testsuites.VolumeMountDetails{
						NameGenerate:      "test-volume-",
						MountPathGenerate: "/mnt/test-",
					},
				},
			},
		}
		test := testsuites.DynamicallyProvisionedDeletePodTest{
			CSIDriver: ebsDriver,
			Pod:       pod,
			PodCheck: &testsuites.PodExecCheck{
				Cmd:            []string{"cat", "/mnt/test-1/data"},
				ExpectedString: "hello world\nhello world\n", // pod will be restarted so expect to see 2 instances of string
			},
		}
		test.Run(cs, ns)
	})

	It("should create a volume on demand and resize it ", func() {
		allowVolumeExpansion := true
		pod := testsuites.PodDetails{
			Cmd: "echo 'hello world' >> /mnt/test-1/data && grep 'hello world' /mnt/test-1/data && sync",
			Volumes: []testsuites.VolumeDetails{
				{
					CreateVolumeParameters: map[string]string{
						ebscsidriver.VolumeTypeKey: awscloud.VolumeTypeGP2,
						ebscsidriver.FSTypeKey:     ebscsidriver.FSTypeExt4,
					},
					ClaimSize: driver.MinimumSizeForVolumeType(awscloud.VolumeTypeGP2),
					VolumeMount: testsuites.VolumeMountDetails{
						NameGenerate:      "test-volume-",
						MountPathGenerate: "/mnt/test-",
					},
					AllowVolumeExpansion: &allowVolumeExpansion,
				},
			},
		}
		test := testsuites.DynamicallyProvisionedResizeVolumeTest{
			CSIDriver: ebsDriver,
			Pod:       pod,
		}
		test.Run(cs, ns)
	})
})

var _ = Describe("[ebs-csi-e2e] [single-az] Snapshot", func() {
	f := framework.NewDefaultFramework("ebs")
	f.NamespacePodSecurityEnforceLevel = admissionapi.LevelPrivileged

	var (
		cs          clientset.Interface
		snapshotrcs restclientset.Interface
		ns          *v1.Namespace
		ebsDriver   driver.PVTestDriver
	)

	BeforeEach(func() {
		cs = f.ClientSet
		var err error
		snapshotrcs, err = restClient(testsuites.SnapshotAPIGroup, testsuites.APIVersionv1)
		if err != nil {
			Fail(fmt.Sprintf("could not get rest clientset: %v", err))
		}
		ns = f.Namespace
		ebsDriver = driver.InitEbsCSIDriver()
	})

	It("should create a pod, write and read to it, take a volume snapshot, and create another pod from the snapshot", func() {
		pod := testsuites.PodDetails{
			// sync before taking a snapshot so that any cached data is written to the EBS volume
			Cmd: "echo 'hello world' >> /mnt/test-1/data && grep 'hello world' /mnt/test-1/data && sync",
			Volumes: []testsuites.VolumeDetails{
				{
					CreateVolumeParameters: map[string]string{
						ebscsidriver.VolumeTypeKey: awscloud.VolumeTypeGP2,
						ebscsidriver.FSTypeKey:     ebscsidriver.FSTypeExt4,
					},
					ClaimSize: driver.MinimumSizeForVolumeType(awscloud.VolumeTypeGP2),
					VolumeMount: testsuites.VolumeMountDetails{
						NameGenerate:      "test-volume-",
						MountPathGenerate: "/mnt/test-",
					},
				},
			},
		}
		restoredPod := testsuites.PodDetails{
			Cmd: "grep 'hello world' /mnt/test-1/data",
			Volumes: []testsuites.VolumeDetails{
				{
					CreateVolumeParameters: map[string]string{
						ebscsidriver.VolumeTypeKey: awscloud.VolumeTypeGP2,
						ebscsidriver.FSTypeKey:     ebscsidriver.FSTypeExt4,
					},
					ClaimSize: driver.MinimumSizeForVolumeType(awscloud.VolumeTypeGP2),
					VolumeMount: testsuites.VolumeMountDetails{
						NameGenerate:      "test-volume-",
						MountPathGenerate: "/mnt/test-",
					},
				},
			},
		}
		test := testsuites.DynamicallyProvisionedVolumeSnapshotTest{
			CSIDriver:   ebsDriver,
			Pod:         pod,
			RestoredPod: restoredPod,
		}
		test.Run(cs, snapshotrcs, ns)
	})
})

var _ = Describe("[ebs-csi-e2e] [multi-az] Dynamic Provisioning", func() {
	f := framework.NewDefaultFramework("ebs")
	f.NamespacePodSecurityEnforceLevel = admissionapi.LevelPrivileged

	var (
		cs        clientset.Interface
		ns        *v1.Namespace
		ebsDriver driver.DynamicPVTestDriver
	)

	BeforeEach(func() {
		cs = f.ClientSet
		ns = f.Namespace
		ebsDriver = driver.InitEbsCSIDriver()
	})

	It("should allow for topology aware volume scheduling", func() {
		volumeBindingMode := storagev1.VolumeBindingWaitForFirstConsumer
		pods := []testsuites.PodDetails{
			{
				Cmd: "echo 'hello world' > /mnt/test-1/data && grep 'hello world' /mnt/test-1/data",
				Volumes: []testsuites.VolumeDetails{
					{
						CreateVolumeParameters: map[string]string{
							ebscsidriver.VolumeTypeKey: awscloud.VolumeTypeGP2,
							ebscsidriver.FSTypeKey:     ebscsidriver.FSTypeExt4,
						},
						ClaimSize:         driver.MinimumSizeForVolumeType(awscloud.VolumeTypeGP2),
						VolumeBindingMode: &volumeBindingMode,
						VolumeMount: testsuites.VolumeMountDetails{
							NameGenerate:      "test-volume-",
							MountPathGenerate: "/mnt/test-",
						},
					},
				},
			},
		}
		test := testsuites.DynamicallyProvisionedTopologyAwareVolumeTest{
			CSIDriver: ebsDriver,
			Pods:      pods,
		}
		test.Run(cs, ns)
	})

	// Requires env AWS_AVAILABILITY_ZONES, a comma separated list of AZs
	It("[env] should allow for topology aware volume with specified zone in allowedTopologies", func() {
		if os.Getenv(awsAvailabilityZonesEnv) == "" {
			Skip(fmt.Sprintf("env %q not set", awsAvailabilityZonesEnv))
		}
		allowedTopologyZones := strings.Split(os.Getenv(awsAvailabilityZonesEnv), ",")
		volumeBindingMode := storagev1.VolumeBindingWaitForFirstConsumer
		pods := []testsuites.PodDetails{
			{
				Cmd: "echo 'hello world' > /mnt/test-1/data && grep 'hello world' /mnt/test-1/data",
				Volumes: []testsuites.VolumeDetails{
					{
						CreateVolumeParameters: map[string]string{
							ebscsidriver.VolumeTypeKey: awscloud.VolumeTypeGP2,
							ebscsidriver.FSTypeKey:     ebscsidriver.FSTypeExt4,
						},
						ClaimSize:             driver.MinimumSizeForVolumeType(awscloud.VolumeTypeGP2),
						VolumeBindingMode:     &volumeBindingMode,
						AllowedTopologyValues: allowedTopologyZones,
						VolumeMount: testsuites.VolumeMountDetails{
							NameGenerate:      "test-volume-",
							MountPathGenerate: "/mnt/test-",
						},
					},
				},
			},
		}
		test := testsuites.DynamicallyProvisionedTopologyAwareVolumeTest{
			CSIDriver: ebsDriver,
			Pods:      pods,
		}
		test.Run(cs, ns)
	})
})

func restClient(group string, version string) (restclientset.Interface, error) {
	// setup rest client
	config, err := framework.LoadConfig()
	if err != nil {
		Fail(fmt.Sprintf("could not load config: %v", err))
	}
	gv := schema.GroupVersion{Group: group, Version: version}
	config.GroupVersion = &gv
	config.APIPath = "/apis"
	config.NegotiatedSerializer = serializer.WithoutConversionCodecFactory{CodecFactory: serializer.NewCodecFactory(runtime.NewScheme())}
	return restclientset.RESTClientFor(config)
}
