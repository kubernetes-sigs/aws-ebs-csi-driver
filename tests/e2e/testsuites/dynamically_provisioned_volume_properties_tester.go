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

package testsuites

import (
	"github.com/kubernetes-sigs/aws-ebs-csi-driver/tests/e2e/driver"
	. "github.com/onsi/ginkgo/v2"
	v1 "k8s.io/api/core/v1"
	clientset "k8s.io/client-go/kubernetes"
)

// DynamicallyProvisionedVolumePropertiesTest will create a pod along with a volume.
// It will then wait until the volume is created and verify input parameters were
// properly applied to the volume.
type DynamicallyProvisionedVolumePropertiesTest struct {
	CreateVolumeParameters map[string]string
	ClaimSize              string
}

func (t *DynamicallyProvisionedVolumePropertiesTest) Run(c clientset.Interface, ns *v1.Namespace, ebsDriver driver.PVTestDriver) {
	volumeDetails := CreateVolumeDetails(t.CreateVolumeParameters, t.ClaimSize)
	testVolume, _ := volumeDetails.SetupDynamicPersistentVolumeClaim(c, ns, ebsDriver)
	defer testVolume.Cleanup()

	pod := createPodWithVolume(c, ns, "", testVolume, volumeDetails)
	defer pod.Cleanup()
	pod.WaitForSuccess()

	By("verifying volume properties")
	volumeID := testVolume.persistentVolume.Spec.CSI.VolumeHandle

	expected := BuildExpectedParameters(t.CreateVolumeParameters, t.ClaimSize)
	VerifyVolumeProperties(volumeID, expected)
}
