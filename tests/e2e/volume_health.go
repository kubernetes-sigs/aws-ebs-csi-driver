/*
Copyright 2026 The Kubernetes Authors.

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
	"k8s.io/kubernetes/test/e2e/framework"
	admissionapi "k8s.io/pod-security-admission/api"
)

var _ = framework.Describe("[ebs-csi-e2e] [Disruptive] Volume Health", framework.WithDisruptive(), func() {
	f := framework.NewDefaultFramework("ebs")
	f.NamespacePodSecurityEnforceLevel = admissionapi.LevelPrivileged

	It("should report paused I/O on the PVC and clear it after I/O resumes", func() {
		test := testsuites.VolumeHealthTest{
			CreateVolumeParameters: map[string]string{
				ebscsidriver.VolumeTypeKey: awscloud.VolumeTypeGP3,
				ebscsidriver.FSTypeKey:     ebscsidriver.FSTypeExt4,
			},
			ClaimSize: driver.MinimumSizeForVolumeType(awscloud.VolumeTypeGP3),
		}
		test.Run(f.ClientSet, f.Namespace, driver.InitEbsCSIDriver())
	})
})
