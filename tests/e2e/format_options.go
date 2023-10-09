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
	testedFsTypes = []string{ebscsidriver.FSTypeExt4}

	formatOptionTests = []testsuites.FormatOptionTest{
		{
			CreateVolumeParameterKey:   ebscsidriver.BlockSizeKey,
			CreateVolumeParameterValue: "1024",
		},
		{
			CreateVolumeParameterKey:   ebscsidriver.InodeSizeKey,
			CreateVolumeParameterValue: "512",
		},
		{
			CreateVolumeParameterKey:   ebscsidriver.BytesPerInodeKey,
			CreateVolumeParameterValue: "8192",
		},
		{
			CreateVolumeParameterKey:   ebscsidriver.NumberOfInodesKey,
			CreateVolumeParameterValue: "200192",
		},
	}
)

var _ = Describe("[ebs-csi-e2e] [single-az] [format-options] Formatting a volume", func() {
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

	for _, fsType := range testedFsTypes {
		Context(fmt.Sprintf("using an %s filesystem", fsType), func() {
			for _, formatOptionTestCase := range formatOptionTests {
				formatOptionTestCase := formatOptionTestCase
				if fsTypeDoesNotSupportFormatOptionParameter(fsType, formatOptionTestCase.CreateVolumeParameterKey) {
					continue
				}

				Context(fmt.Sprintf("with a custom %s parameter", formatOptionTestCase.CreateVolumeParameterKey), func() {
					It("successfully mounts and is resizable", func() {
						formatOptionTestCase.Run(cs, ns, ebsDriver, fsType)
					})
				})
			}
		})
	}
})

func fsTypeDoesNotSupportFormatOptionParameter(fsType string, createVolumeParameterKey string) bool {
	_, paramNotSupported := ebscsidriver.FileSystemConfigs[fsType].NotSupportedParams[createVolumeParameterKey]
	return paramNotSupported
}
