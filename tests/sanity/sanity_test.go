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

package sanity

import (
	"os"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"k8s.io/kubernetes/pkg/util/mount"

	sanity "github.com/kubernetes-csi/csi-test/pkg/sanity"

	"github.com/kubernetes-sigs/aws-ebs-csi-driver/pkg/driver"
)

const (
	mountPath = "/tmp/csi-mount"
	stagePath = "/tmp/csi-stage"
	socket    = "/tmp/csi.sock"
	endpoint  = "unix://" + socket
)

var ebsDriver *driver.Driver

func TestSanity(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Sanity Tests Suite")
}

var _ = BeforeSuite(func() {
	fakeMounter := &mount.FakeMounter{
		Filesystem: map[string]mount.FileType{
			"/dev/xvdbc": mount.FileTypeFile,
		},
	}
	ebsDriver = driver.NewFakeDriver(endpoint, newFakeCloudProvider(), fakeMounter)
	go func() {
		Expect(ebsDriver.Run()).NotTo(HaveOccurred())
	}()
})

var _ = AfterSuite(func() {
	ebsDriver.Stop()
	Expect(os.RemoveAll(socket)).NotTo(HaveOccurred())
})

var _ = Describe("AWS EBS CSI Driver", func() {
	config := &sanity.Config{
		Address:     endpoint,
		TargetPath:  mountPath,
		StagingPath: stagePath,
	}
	sanity.GinkgoTest(config)
})
