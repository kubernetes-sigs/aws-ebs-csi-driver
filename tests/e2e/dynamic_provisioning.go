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
	. "github.com/onsi/ginkgo"
	"k8s.io/api/core/v1"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/kubernetes/test/e2e/framework"

	"github.com/kubernetes-sigs/aws-ebs-csi-driver/tests/e2e/driver"
	"github.com/kubernetes-sigs/aws-ebs-csi-driver/tests/e2e/testsuites"

	"github.com/kubernetes-sigs/aws-ebs-csi-driver/pkg/cloud"
	ebscsidriver "github.com/kubernetes-sigs/aws-ebs-csi-driver/pkg/driver"
)

var _ = Describe("[ebs-csi-e2e] [single-az] Dynamic Provisioning", func() {
	f := framework.NewDefaultFramework("ebs")

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

	for _, t := range cloud.ValidVolumeTypes {
		for _, fs := range ebscsidriver.ValidFSTypes {
			volumeType := t
			fsType := fs
			Context(fmt.Sprintf("with volumeType [%q] and fsType [%q]", volumeType, fsType), func() {
				It("should create a volume on demand", func() {
					pods := []testsuites.PodDetails{
						{
							Cmd: "echo 'hello world' > /mnt/test-1/data && grep 'hello world' /mnt/test-1/data",
							Volumes: []testsuites.VolumeDetails{
								{
									VolumeType: volumeType,
									FSType:     fsType,
									ClaimSize:  driver.MinimumSizeForVolumeType(volumeType),
									VolumeMount: testsuites.VolumeMountDetails{
										NameGenerate:      "test-volume-",
										MountPathGenerate: "/mnt/test-",
									},
								},
							},
						},
					}
					test := testsuites.PodDynamicVolumeWriterReaderTest{
						CSIDriver: ebsDriver,
						Pods:      pods,
					}
					test.Run(cs, ns)
				})
			})
		}
	}

	It("should create multiple PV objects, bind to PVCs and attach all to a single pod", func() {
		pods := []testsuites.PodDetails{
			{
				Cmd: "echo 'hello world' > /mnt/test-1/data && echo 'hello world' > /mnt/test-2/data && grep 'hello world' /mnt/test-1/data  && grep 'hello world' /mnt/test-2/data",
				Volumes: []testsuites.VolumeDetails{
					{
						VolumeType: cloud.VolumeTypeGP2,
						FSType:     ebscsidriver.FSTypeExt3,
						ClaimSize:  driver.MinimumSizeForVolumeType(cloud.VolumeTypeGP2),
						VolumeMount: testsuites.VolumeMountDetails{
							NameGenerate:      "test-volume-",
							MountPathGenerate: "/mnt/test-",
						},
					},
					{
						VolumeType: cloud.VolumeTypeIO1,
						FSType:     ebscsidriver.FSTypeExt4,
						ClaimSize:  driver.MinimumSizeForVolumeType(cloud.VolumeTypeIO1),
						VolumeMount: testsuites.VolumeMountDetails{
							NameGenerate:      "test-volume-",
							MountPathGenerate: "/mnt/test-",
						},
					},
				},
			},
		}
		test := testsuites.PodDynamicVolumeWriterReaderTest{
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
						VolumeType: cloud.VolumeTypeGP2,
						FSType:     ebscsidriver.FSTypeExt3,
						ClaimSize:  driver.MinimumSizeForVolumeType(cloud.VolumeTypeGP2),
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
						VolumeType: cloud.VolumeTypeIO1,
						FSType:     ebscsidriver.FSTypeExt4,
						ClaimSize:  driver.MinimumSizeForVolumeType(cloud.VolumeTypeIO1),
						VolumeMount: testsuites.VolumeMountDetails{
							NameGenerate:      "test-volume-",
							MountPathGenerate: "/mnt/test-",
						},
					},
				},
			},
		}
		test := testsuites.PodDynamicVolumeWriterReaderTest{
			CSIDriver: ebsDriver,
			Pods:      pods,
		}
		test.Run(cs, ns)
	})
})
