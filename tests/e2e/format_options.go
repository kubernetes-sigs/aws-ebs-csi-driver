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
	awscloud "github.com/kubernetes-sigs/aws-ebs-csi-driver/pkg/cloud"
	ebscsidriver "github.com/kubernetes-sigs/aws-ebs-csi-driver/pkg/driver"
	"github.com/kubernetes-sigs/aws-ebs-csi-driver/tests/e2e/driver"
	"github.com/kubernetes-sigs/aws-ebs-csi-driver/tests/e2e/testsuites"
	. "github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega/format"
	v1 "k8s.io/api/core/v1"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/kubernetes/test/e2e/framework"
	admissionapi "k8s.io/pod-security-admission/api"
	"regexp"
	"strconv"
)

const (
	initialVolumeSizeGi     = 2
	volumeSizeIncreaseAmtGi = 2

	blockSizeTestValue      = "1024"
	inodeSizeTestValue      = "512"
	bytesPerInodeTestValue  = "8192"
	numberOfInodesTestValue = "200192"
)

type formatOptionTestCase struct {
	createVolumeParameterKey        string
	createVolumeParameterValue      string
	expectedFilesystemInfoParamName string
	expectedFilesystemInfoParamVal  string
}

// TODO is this a clean place for this? Or should we follow standard of other e2e tests with tester files in e2e/testsuites?
var (
	// TODO having this here is code smell. Hardcode? Test case from original inode PR #1661 https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/1661
	expectedBytesPerInodeTestResult = strconv.Itoa(131072 * initialVolumeSizeGi)

	formatOptionTests = []formatOptionTestCase{
		{
			createVolumeParameterKey:        ebscsidriver.BlockSizeKey,
			createVolumeParameterValue:      blockSizeTestValue,
			expectedFilesystemInfoParamName: "Block size",
			expectedFilesystemInfoParamVal:  blockSizeTestValue,
		},
		{
			createVolumeParameterKey:        ebscsidriver.INodeSizeKey,
			createVolumeParameterValue:      inodeSizeTestValue,
			expectedFilesystemInfoParamName: "Inode size",
			expectedFilesystemInfoParamVal:  inodeSizeTestValue},
		{
			createVolumeParameterKey:        ebscsidriver.BytesPerINodeKey,
			createVolumeParameterValue:      bytesPerInodeTestValue,
			expectedFilesystemInfoParamName: "Inode count",
			expectedFilesystemInfoParamVal:  expectedBytesPerInodeTestResult,
		},
		{
			createVolumeParameterKey:        ebscsidriver.NumberOfINodesKey,
			createVolumeParameterValue:      numberOfInodesTestValue,
			expectedFilesystemInfoParamName: "Inode count",
			expectedFilesystemInfoParamVal:  numberOfInodesTestValue},
	}
)

var _ = Describe("[ebs-csi-e2e] [format-options] Formatting a volume", func() {
	f := framework.NewDefaultFramework("ebs")
	f.NamespacePodSecurityEnforceLevel = admissionapi.LevelPrivileged // TODO Maybe don't need this if Connor big brain pulls thru

	var (
		cs        clientset.Interface
		ns        *v1.Namespace
		ebsDriver driver.PVTestDriver

		testedFsTypes = []string{ebscsidriver.FSTypeExt4} // TODO Is this right place for this?

		volumeMountPath     = "/mnt/test-format-option"                                                                                            // TODO maybe keep this as mnt/test-1, and refactor to be `DefaultMountPath` globally in testsuites.
		podCmdGetFsInfo     = fmt.Sprintf("tune2fs -l $(df -k '%s'| tail -1 | awk '{ print $1 }')", volumeMountPath)                               // Gets the filesystem info for the mounted volume
		podCmdWriteToVolume = fmt.Sprintf("echo 'hello world' >> %s/data && grep 'hello world' %s/data && sync", volumeMountPath, volumeMountPath) // TODO Debt: All the dynamic provisioning tests use this same cmd. Should we refactor out into exported constant?
	)

	BeforeEach(func() {
		cs = f.ClientSet
		ns = f.Namespace
		ebsDriver = driver.InitEbsCSIDriver()
	})

	for _, fsType := range testedFsTypes {
		Context(fmt.Sprintf("with an %s filesystem", fsType), func() {
			// TODO: is t clear? Or should it be 'formatOptionTestCaseValues'
			for _, t := range formatOptionTests {
				if fsTypeDoesNotSupportFormatOptionParameter(fsType, t.createVolumeParameterKey) {
					continue
				}

				Context(fmt.Sprintf("with a custom %s parameter", t.createVolumeParameterKey), func() {
					It("successfully mounts and is resizable", func() {
						By("setting up pvc")
						volumeDetails := createFormatOptionVolumeDetails(fsType, volumeMountPath, t)
						testPvc, _ := volumeDetails.SetupDynamicPersistentVolumeClaim(cs, ns, ebsDriver)
						defer testPvc.Cleanup()

						By("deploying pod with custom format option")
						getFsInfoTestPod := createPodWithVolume(cs, ns, podCmdGetFsInfo, testPvc, volumeDetails)
						defer getFsInfoTestPod.Cleanup()
						getFsInfoTestPod.WaitForSuccess() // TODO e2e test implementation defaults to a 15 min wait instead of 5 min one... Is that fine or refactor worthy?

						By("confirming custom format option was applied")
						fsInfoSearchRegexp := fmt.Sprintf(`%s:\s+%s`, t.expectedFilesystemInfoParamName, t.expectedFilesystemInfoParamVal)
						if isFormatOptionApplied := FindRegexpInPodLogs(fsInfoSearchRegexp, getFsInfoTestPod); !isFormatOptionApplied {
							framework.Failf("Did not find expected %s value of %s in filesystem info", t.expectedFilesystemInfoParamName, t.expectedFilesystemInfoParamVal)
						}

						By("testing that pvc is able to be resized")
						testsuites.ResizePvc(cs, ns, testPvc, volumeSizeIncreaseAmtGi)

						By("validating resized pvc by deploying new pod")
						resizeTestPod := createPodWithVolume(cs, ns, podCmdWriteToVolume, testPvc, volumeDetails)
						defer resizeTestPod.Cleanup()

						By("confirming new pod can write to resized volume")
						resizeTestPod.WaitForSuccess()
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

// TODO should we improve this across e2e tests via builder design pattern? Or is that not go-like?
func createFormatOptionVolumeDetails(fsType string, volumeMountPath string, t formatOptionTestCase) *testsuites.VolumeDetails {
	allowVolumeExpansion := true

	volume := testsuites.VolumeDetails{
		VolumeType:   awscloud.VolumeTypeGP2,
		FSType:       fsType,
		MountOptions: []string{"rw"},
		ClaimSize:    fmt.Sprintf("%vGi", initialVolumeSizeGi),
		VolumeMount: testsuites.VolumeMountDetails{
			NameGenerate:      "test-volume-format-option",
			MountPathGenerate: volumeMountPath,
		},
		AllowVolumeExpansion: &allowVolumeExpansion,
		AdditionalParameters: map[string]string{
			t.createVolumeParameterKey: t.createVolumeParameterValue,
		},
	}

	return &volume
}

// TODO putting this in function may be overkill? In an ideal world we refactor out testsuites.TestEverything objects so testPod.SetupVolume isn't gross.
func createPodWithVolume(client clientset.Interface, namespace *v1.Namespace, cmd string, testPvc *testsuites.TestPersistentVolumeClaim, volumeDetails *testsuites.VolumeDetails) *testsuites.TestPod {
	testPod := testsuites.NewTestPod(client, namespace, cmd)

	// TODO Will Refactor in PR 2
	pvc := testPvc.GetPvc()
	testPod.SetupVolume(pvc, volumeDetails.VolumeMount.NameGenerate, volumeDetails.VolumeMount.MountPathGenerate, volumeDetails.VolumeMount.ReadOnly)

	testPod.Create()

	return testPod
}

// TODO: Maybe should use something other than Find(), but *shrug*
// TODO  I should move this to testsuites.go, yes?
// TODO Should I instead use RunHostCmd or LookForString from https://github.com/kubernetes/kubernetes/blob/master/test/e2e/framework/pod/output/output.go ?

// FindRegexpInPodLogs searches given testPod's logs for a given regular expression. Returns `true` if found.
func FindRegexpInPodLogs(regexpPattern string, testPod *testsuites.TestPod) bool {
	podLogs, err := testPod.Logs()
	framework.ExpectNoError(err, "Tried getting logs for pod %s", format.Object(testPod, 2))

	var expectedLine = regexp.MustCompile(regexpPattern)
	res := expectedLine.Find(podLogs)
	return res != nil
}
