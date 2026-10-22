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

package testsuites

import (
	"context"
	"os"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/arn"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/fis"
	fistypes "github.com/aws/aws-sdk-go-v2/service/fis/types"
	"github.com/kubernetes-sigs/aws-ebs-csi-driver/tests/e2e/driver"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/kubernetes/test/e2e/framework"
)

// VolumeHealthTest pauses the I/O of a volume with AWS FIS and checks that its PVC reports the stall in
// status.healthStatus, and that the condition clears after I/O resumes.
type VolumeHealthTest struct {
	CreateVolumeParameters map[string]string
	ClaimSize              string
}

func (t *VolumeHealthTest) Run(c clientset.Interface, ns *v1.Namespace, ebsDriver driver.PVTestDriver) {
	ctx := context.Background()
	roleARN, err := arn.Parse(os.Getenv("FIS_ROLE_ARN"))
	framework.ExpectNoError(err, "FIS_ROLE_ARN must be the ARN of a role that AWS FIS can assume")
	cfg, err := config.LoadDefaultConfig(ctx)
	framework.ExpectNoError(err)
	fisClient := fis.NewFromConfig(cfg)

	By("deploying a pod that writes to a volume")
	volumeDetails := CreateVolumeDetails(t.CreateVolumeParameters, t.ClaimSize)
	testVolume, _ := volumeDetails.SetupDynamicPersistentVolumeClaim(c, ns, ebsDriver)
	defer testVolume.Cleanup()
	pod := createPodWithVolume(c, ns, PodCmdContinuousWrite(DefaultMountPath), testVolume, volumeDetails)
	defer pod.Cleanup()
	pod.WaitForRunning()

	By("pausing the I/O of the volume with AWS FIS")
	volumeARN := arn.ARN{
		Partition: roleARN.Partition,
		Service:   "ec2",
		Region:    cfg.Region,
		AccountID: roleARN.AccountID,
		Resource:  "volume/" + testVolume.persistentVolume.Spec.CSI.VolumeHandle,
	}
	template, err := fisClient.CreateExperimentTemplate(ctx, &fis.CreateExperimentTemplateInput{
		Description:    aws.String("Pause the I/O of an EBS CSI driver e2e test volume"),
		RoleArn:        aws.String(roleARN.String()),
		StopConditions: []fistypes.CreateExperimentTemplateStopConditionInput{{Source: aws.String("none")}},
		Targets: map[string]fistypes.CreateExperimentTemplateTargetInput{
			"Volumes": {
				ResourceType:  aws.String("aws:ec2:ebs-volume"),
				ResourceArns:  []string{volumeARN.String()},
				SelectionMode: aws.String("ALL"),
			},
		},
		Actions: map[string]fistypes.CreateExperimentTemplateActionInput{
			"PauseVolumeIO": {
				ActionId: aws.String("aws:ebs:pause-volume-io"),
				// Longer than the test waits for the condition, so the test stops the experiment itself.
				Parameters: map[string]string{"duration": "PT10M"},
				Targets:    map[string]string{"Volumes": "Volumes"},
			},
		},
	})
	framework.ExpectNoError(err)
	// A running experiment doesn't need its template. Deleting it now means an aborted job can't leak it.
	experiment, startErr := fisClient.StartExperiment(ctx, &fis.StartExperimentInput{ExperimentTemplateId: template.ExperimentTemplate.Id})
	_, err = fisClient.DeleteExperimentTemplate(ctx, &fis.DeleteExperimentTemplateInput{Id: template.ExperimentTemplate.Id})
	framework.ExpectNoError(startErr)
	framework.ExpectNoError(err)
	stopExperiment := func() error {
		_, err := fisClient.StopExperiment(ctx, &fis.StopExperimentInput{Id: experiment.Experiment.Id})
		return err
	}
	// Resume I/O before the pod and volume are deleted if the test fails. It errors if the experiment already stopped.
	defer func() { _ = stopExperiment() }()
	Eventually(func(g Gomega) {
		out, err := fisClient.GetExperiment(ctx, &fis.GetExperimentInput{Id: experiment.Experiment.Id})
		g.Expect(err).NotTo(HaveOccurred())
		if out.Experiment.State.Status == fistypes.ExperimentStatusFailed {
			StopTrying("AWS FIS experiment failed: " + aws.ToString(out.Experiment.State.Reason)).Now()
		}
		g.Expect(out.Experiment.State.Status).To(Equal(fistypes.ExperimentStatusRunning))
	}).WithTimeout(2 * time.Minute).WithPolling(5 * time.Second).Should(Succeed())

	By("waiting for the PVC to report the stalled I/O")
	Eventually(func(g Gomega) {
		pvc, err := c.CoreV1().PersistentVolumeClaims(ns.Name).Get(ctx, testVolume.persistentVolumeClaim.Name, metav1.GetOptions{})
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(pvc.Status.HealthStatus).NotTo(BeNil())
		g.Expect(pvc.Status.HealthStatus.HealthConditions).To(ContainElement(SatisfyAll(
			HaveField("Status", v1.VolumeHealthInaccessible),
			HaveField("Reason", "IOPerformanceStalled"),
		)))
	}).WithTimeout(8*time.Minute).WithPolling(10*time.Second).Should(Succeed(), "PVC healthStatus requires the CSIVolumeHealth feature gate on kube-apiserver")

	By("resuming the I/O of the volume")
	framework.ExpectNoError(stopExperiment())

	By("waiting for the PVC condition to clear")
	Eventually(func(g Gomega) {
		pvc, err := c.CoreV1().PersistentVolumeClaims(ns.Name).Get(ctx, testVolume.persistentVolumeClaim.Name, metav1.GetOptions{})
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(pvc.Status.HealthStatus).To(Or(BeNil(), HaveField("HealthConditions", BeEmpty())))
	}).WithTimeout(8 * time.Minute).WithPolling(10 * time.Second).Should(Succeed())
}
