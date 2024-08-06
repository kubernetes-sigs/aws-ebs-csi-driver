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

package e2e

import (
	awscloud "github.com/kubernetes-sigs/aws-ebs-csi-driver/pkg/cloud"
	ebscsidriver "github.com/kubernetes-sigs/aws-ebs-csi-driver/pkg/driver"
	"github.com/kubernetes-sigs/aws-ebs-csi-driver/tests/e2e/driver"
	"github.com/kubernetes-sigs/aws-ebs-csi-driver/tests/e2e/testsuites"
	. "github.com/onsi/ginkgo/v2"
	v1 "k8s.io/api/core/v1"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/kubernetes/test/e2e/framework"
	admissionapi "k8s.io/pod-security-admission/api"
)

var (
	defaultModifyVolumeTestGp3CreateVolumeParameters = map[string]string{
		ebscsidriver.VolumeTypeKey: awscloud.VolumeTypeGP3,
		ebscsidriver.FSTypeKey:     ebscsidriver.FSTypeExt4,
	}
)

var (
	modifyVolumeTests = map[string]testsuites.ModifyVolumeTest{
		"with a new iops annotation": {
			CreateVolumeParameters: defaultModifyVolumeTestGp3CreateVolumeParameters,
			ModifyVolumeParameters: map[string]string{
				testsuites.Iops: "4000",
			},
			ShouldResizeVolume:                    false,
			ShouldTestInvalidModificationRecovery: false,
		},
		"with a new io2 volumeType annotation": {
			CreateVolumeParameters: defaultModifyVolumeTestGp3CreateVolumeParameters,
			ModifyVolumeParameters: map[string]string{
				testsuites.VolumeType: awscloud.VolumeTypeIO2,
				testsuites.Iops:       testsuites.DefaultIopsIoVolumes, // As of aws-ebs-csi-driver v1.25.0, parameter iops must be re-specified when modifying volumeType io2 volumes.
			},
			ShouldResizeVolume:                    false,
			ShouldTestInvalidModificationRecovery: false,
		},
		"with a new throughput annotation": {
			CreateVolumeParameters: defaultModifyVolumeTestGp3CreateVolumeParameters,
			ModifyVolumeParameters: map[string]string{
				testsuites.Throughput: "150",
			},
			ShouldResizeVolume:                    false,
			ShouldTestInvalidModificationRecovery: false,
		},
		"with a new tag annotation": {
			CreateVolumeParameters: defaultModifyVolumeTestGp3CreateVolumeParameters,
			ModifyVolumeParameters: map[string]string{
				testsuites.TagSpec: "key1=test1",
			},
			ShouldResizeVolume:                    false,
			ShouldTestInvalidModificationRecovery: false,
			ExternalResizerOnly:                   true,
		},
		"with new throughput, and iops annotations": {
			CreateVolumeParameters: defaultModifyVolumeTestGp3CreateVolumeParameters,
			ModifyVolumeParameters: map[string]string{
				testsuites.Iops:       "4000",
				testsuites.Throughput: "150",
			},
			ShouldResizeVolume:                    false,
			ShouldTestInvalidModificationRecovery: false,
		},
		"with new throughput, iops, and tag annotations": {
			CreateVolumeParameters: defaultModifyVolumeTestGp3CreateVolumeParameters,
			ModifyVolumeParameters: map[string]string{
				testsuites.Iops:       "4000",
				testsuites.Throughput: "150",
				testsuites.TagSpec:    "key2=test2",
			},
			ShouldResizeVolume:                    false,
			ShouldTestInvalidModificationRecovery: false,
			ExternalResizerOnly:                   true,
		},
		"with a larger size and new throughput and iops annotations": {
			CreateVolumeParameters: defaultModifyVolumeTestGp3CreateVolumeParameters,
			ModifyVolumeParameters: map[string]string{
				testsuites.Iops:       "4000",
				testsuites.Throughput: "150",
			},
			ShouldResizeVolume:                    true,
			ShouldTestInvalidModificationRecovery: false,
		},
		"with a larger size and new throughput and iops annotations after providing an invalid annotation": {
			CreateVolumeParameters: defaultModifyVolumeTestGp3CreateVolumeParameters,
			ModifyVolumeParameters: map[string]string{
				testsuites.Iops:       "4000",
				testsuites.Throughput: "150",
			},
			ShouldResizeVolume:                    true,
			ShouldTestInvalidModificationRecovery: true,
		},
		"from io2 to gp3 with larger size and new iops and throughput annotations": {
			CreateVolumeParameters: map[string]string{
				ebscsidriver.VolumeTypeKey: awscloud.VolumeTypeIO2,
				ebscsidriver.FSTypeKey:     ebscsidriver.FSTypeExt4,
				ebscsidriver.IopsKey:       testsuites.DefaultIopsIoVolumes,
			},
			ModifyVolumeParameters: map[string]string{
				testsuites.VolumeType: awscloud.VolumeTypeGP3,
				testsuites.Iops:       "4000",
				testsuites.Throughput: "150",
			},
			ShouldResizeVolume:                    true,
			ShouldTestInvalidModificationRecovery: false,
		},
	}
)

var _ = Describe("[ebs-csi-e2e] [single-az] [modify-volume] Modifying a PVC", func() {
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

	for testName, modifyVolumeTest := range modifyVolumeTests {
		modifyVolumeTest := modifyVolumeTest
		Context(testName, func() {
			It("will modify associated PV and EBS Volume via volume-modifier-for-k8s", func() {
				if modifyVolumeTest.ExternalResizerOnly {
					Skip("Functionality being tested is not supported for Modification via volume-modifier-for-k8s, skipping test")
				}
				modifyVolumeTest.Run(cs, ns, ebsDriver, testsuites.VolumeModifierForK8s)
			})
			It("will modify associated PV and EBS Volume via external-resizer", func() {
				modifyVolumeTest.Run(cs, ns, ebsDriver, testsuites.ExternalResizer)
			})
		})
	}
})
