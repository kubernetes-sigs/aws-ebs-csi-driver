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
	testedFsTypes = []string{ebscsidriver.FSTypeExt4, ebscsidriver.FSTypeExt3, ebscsidriver.FSTypeXfs}
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

		formatOptionTests := map[string]testsuites.FormatOptionTest{
			ebscsidriver.BlockSizeKey: {
				CreateVolumeParameters: map[string]string{
					ebscsidriver.BlockSizeKey: "1024",
					ebscsidriver.FSTypeKey:    fsType,
				},
			},
			ebscsidriver.InodeSizeKey: {
				CreateVolumeParameters: map[string]string{
					ebscsidriver.InodeSizeKey: "512",
					ebscsidriver.FSTypeKey:    fsType,
				},
			},
			ebscsidriver.BytesPerInodeKey: {
				CreateVolumeParameters: map[string]string{
					ebscsidriver.BytesPerInodeKey: "8192",
					ebscsidriver.FSTypeKey:        fsType,
				},
			},
			ebscsidriver.NumberOfInodesKey: {
				CreateVolumeParameters: map[string]string{
					ebscsidriver.NumberOfInodesKey: "200192",
					ebscsidriver.FSTypeKey:         fsType,
				},
			},
			ebscsidriver.Ext4BigAllocKey: {
				CreateVolumeParameters: map[string]string{
					ebscsidriver.Ext4BigAllocKey: "true",
					ebscsidriver.FSTypeKey:       fsType,
				},
			},
			ebscsidriver.Ext4ClusterSizeKey: {
				CreateVolumeParameters: map[string]string{
					ebscsidriver.Ext4BigAllocKey:    "true",
					ebscsidriver.Ext4ClusterSizeKey: "16384",
					ebscsidriver.FSTypeKey:          fsType,
				},
			},
		}

		Context(fmt.Sprintf("using an %s filesystem", fsType), func() {
			for testedParameter, formatOptionTestCase := range formatOptionTests {
				formatOptionTestCase := formatOptionTestCase
				if fsTypeDoesNotSupportFormatOptionParameter(fsType, testedParameter) {
					continue
				}

				Context(fmt.Sprintf("with a custom %s parameter", testedParameter), func() {
					It("successfully mounts and is resizable", func() {
						formatOptionTestCase.Run(cs, ns, ebsDriver)
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
