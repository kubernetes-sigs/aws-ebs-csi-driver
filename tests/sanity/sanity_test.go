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

	"github.com/bertinatto/ebs-csi-driver/pkg/cloud"
	"github.com/bertinatto/ebs-csi-driver/pkg/driver"
	sanity "github.com/kubernetes-csi/csi-test/pkg/sanity"
)

func TestSanity(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "AWS EBS CSI Driver Sanity Tests")
}

var _ = Describe("AWS EBS CSI Driver", func() {
	const (
		mountPath = "/tmp/csi/mount"
		stagePath = "/tmp/csi/stage"
		socket    = "/tmp/csi.sock"
		endpoint  = "unix://" + socket
	)

	config := &sanity.Config{
		Address:     endpoint,
		TargetPath:  mountPath,
		StagingPath: stagePath,
	}

	var ebsDriver *driver.Driver

	BeforeEach(func() {
		ebsDriver = driver.NewDriver(cloud.NewFakeCloudProvider(), driver.NewFakeMounter(), endpoint)
		go func() {
			err := ebsDriver.Run()
			Expect(err).To(BeNil())
		}()
	})

	AfterEach(func() {
		ebsDriver.Stop()
		if err := os.Remove(socket); err != nil && !os.IsNotExist(err) {
			Expect(err).To(BeNil())
		}
	})

	Describe("Sanity Test", func() {
		sanity.GinkgoTest(config)
	})

})
