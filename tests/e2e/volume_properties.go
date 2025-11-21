// Copyright 2025 The Kubernetes Authors.
//
// Licensed under the Apache License, Version 2.0 (the 'License');
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//    http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an 'AS IS' BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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

var _ = Describe("[ebs-csi-e2e] [single-az] Volume Properties Verification", func() {
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

	It("should create a gp3 volume with custom IOPS", func() {
		test := testsuites.DynamicallyProvisionedVolumePropertiesTest{
			CreateVolumeParameters: map[string]string{
				ebscsidriver.VolumeTypeKey: awscloud.VolumeTypeGP3,
				ebscsidriver.IopsKey:       "4000",
			},
			ClaimSize: "10Gi",
		}
		test.Run(cs, ns, ebsDriver)
	})

	It("should create a gp3 volume with custom IOPS and throughput", func() {
		test := testsuites.DynamicallyProvisionedVolumePropertiesTest{
			CreateVolumeParameters: map[string]string{
				ebscsidriver.VolumeTypeKey: awscloud.VolumeTypeGP3,
				ebscsidriver.IopsKey:       "5000",
				ebscsidriver.ThroughputKey: "250",
			},
			ClaimSize: "10Gi",
		}
		test.Run(cs, ns, ebsDriver)
	})

	It("should create an io2 volume with custom IOPS", func() {
		test := testsuites.DynamicallyProvisionedVolumePropertiesTest{
			CreateVolumeParameters: map[string]string{
				ebscsidriver.VolumeTypeKey: awscloud.VolumeTypeIO2,
				ebscsidriver.IopsKey:       "10000",
			},
			ClaimSize: "10Gi",
		}
		test.Run(cs, ns, ebsDriver)
	})

	It("should create a io2 volume with custom IOPS and encryption", func() {
		test := testsuites.DynamicallyProvisionedVolumePropertiesTest{
			CreateVolumeParameters: map[string]string{
				ebscsidriver.VolumeTypeKey: awscloud.VolumeTypeIO2,
				ebscsidriver.IopsKey:       "5000",
				ebscsidriver.EncryptedKey:  "true",
			},
			ClaimSize: "10Gi",
		}
		test.Run(cs, ns, ebsDriver)
	})

	It("should create an io2 volume with encryption", func() {
		test := testsuites.DynamicallyProvisionedVolumePropertiesTest{
			CreateVolumeParameters: map[string]string{
				ebscsidriver.VolumeTypeKey: awscloud.VolumeTypeIO2,
				ebscsidriver.IopsKey:       "1000",
				ebscsidriver.EncryptedKey:  "true",
			},
			ClaimSize: "10Gi",
		}
		test.Run(cs, ns, ebsDriver)
	})

	It("should create an gp3 volume with custom size, IOPS, throughput, and encryption", func() {
		test := testsuites.DynamicallyProvisionedVolumePropertiesTest{
			CreateVolumeParameters: map[string]string{
				ebscsidriver.VolumeTypeKey: awscloud.VolumeTypeGP3,
				ebscsidriver.IopsKey:       "4000",
				ebscsidriver.ThroughputKey: "250",
				ebscsidriver.EncryptedKey:  "true",
			},
			ClaimSize: "10Gi",
		}
		test.Run(cs, ns, ebsDriver)
	})

})
