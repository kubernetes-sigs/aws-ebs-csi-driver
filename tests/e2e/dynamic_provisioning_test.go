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

package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/kubernetes-sigs/aws-ebs-csi-driver/tests/utils"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var (
	testDataDir = "testdata/dynamic_provisioning"
	kubeconfig  = filepath.Join(os.Getenv("HOME"), ".kube/config")
)

var _ = Describe("Dynamic Provisioning", func() {

	It("should create a volume on demand", func() {
		var (
			claimSpec        = filepath.Join(testDataDir, "claim.yaml")
			storageclassSpec = filepath.Join(testDataDir, "storageclass.yaml")
			podSpec          = filepath.Join(testDataDir, "pod.yaml")
		)
		kk, err := utils.NewKubectl(kubeconfig)
		Expect(err).To(BeNil())

		logf("Creating StorageClass: %v", storageclassSpec)
		err = kk.Create(storageclassSpec)
		Expect(err).To(BeNil())
		defer func() {
			logf("Deleting StorageClass: %v", storageclassSpec)
			err = kk.Delete(storageclassSpec)
			Expect(err).To(BeNil())
		}()

		logf("Creating PVC: %v", claimSpec)
		err = kk.Create(claimSpec)
		Expect(err).To(BeNil())
		defer func() {
			logf("Deleting PVC: %v", claimSpec)
			err = kk.Delete(claimSpec)
			Expect(err).To(BeNil())
		}()

		logf("Creating Pod: %v", podSpec)
		err = kk.Create(podSpec)
		Expect(err).To(BeNil())
		defer func() {
			logf("Deleting Pod: %v", podSpec)
			err = kk.Delete(podSpec)
			Expect(err).To(BeNil())
		}()

		// ensure pod is ready
		err = kk.EnsurePodRunning(podSpec)
		Expect(err).To(BeNil())
		logf("Pod is running")
	})
})

func logf(format string, args ...interface{}) {
	fmt.Fprintln(GinkgoWriter, fmt.Sprintf(format, args...))
}
