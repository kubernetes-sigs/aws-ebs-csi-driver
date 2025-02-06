// Copyright 2024 The Kubernetes Authors.
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
	"context"
	"fmt"
	"io"
	"net/http"
	"time"

	awscloud "github.com/kubernetes-sigs/aws-ebs-csi-driver/pkg/cloud"
	ebscsidriver "github.com/kubernetes-sigs/aws-ebs-csi-driver/pkg/driver"
	"github.com/kubernetes-sigs/aws-ebs-csi-driver/tests/e2e/driver"
	"github.com/kubernetes-sigs/aws-ebs-csi-driver/tests/e2e/testsuites"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/kubernetes/test/e2e/framework"
	"k8s.io/kubernetes/test/e2e/framework/kubectl"
	admissionapi "k8s.io/pod-security-admission/api"
)

var _ = Describe("[ebs-csi-e2e] [single-az] NVMe Metrics", func() {
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

	It("should create a stateful pod and read NVMe metrics from node plugin", func() {
		pod := testsuites.PodDetails{
			Cmd: "while true; do echo $(date -u) >> /mnt/test-1/out.txt; sleep 5; done",
			Volumes: []testsuites.VolumeDetails{
				{
					CreateVolumeParameters: map[string]string{
						ebscsidriver.VolumeTypeKey: awscloud.VolumeTypeGP3,
						ebscsidriver.FSTypeKey:     ebscsidriver.FSTypeExt4,
					},
					ClaimSize: driver.MinimumSizeForVolumeType(awscloud.VolumeTypeGP3),
					VolumeMount: testsuites.VolumeMountDetails{
						NameGenerate:      "test-volume-",
						MountPathGenerate: "/mnt/test-",
					},
				},
			},
		}

		By("Setting up pod and volumes")
		tpod, _ := pod.SetupWithDynamicVolumes(cs, ns, ebsDriver)
		defer tpod.Cleanup()

		By("Creating the stateful pod")
		tpod.Create()
		tpod.WaitForRunning()

		By("Getting name of the node where test pod is running")
		p, err := cs.CoreV1().Pods(ns.Name).Get(context.TODO(), tpod.GetName(), metav1.GetOptions{})
		framework.ExpectNoError(err, "Failed to get pod")
		nodeName := p.Spec.NodeName
		Expect(nodeName).NotTo(BeEmpty(), "Node name should not be empty")

		By("Finding the CSI node plugin on the node where test pod is running")
		csiPods, err := cs.CoreV1().Pods("kube-system").List(context.TODO(), metav1.ListOptions{
			LabelSelector: "app=ebs-csi-node",
		})
		framework.ExpectNoError(err, "Failed to get CSI node pods")

		var csiNodePod *v1.Pod
		for _, pod := range csiPods.Items {
			if pod.Spec.NodeName == nodeName {
				csiNodePod = &pod
				break
			}
		}
		Expect(csiNodePod).NotTo(BeNil(), "No CSI node pod found")

		By("Setting up port-forwarding")
		args := []string{"port-forward", "-n", "kube-system", csiNodePod.Name, "3302:3302"}

		go kubectl.RunKubectlOrDie("kube-system", args...)

		By("Getting metrics from endpoint")
		ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
		defer cancel()

		metricsOutput, err := getMetricsWithRetry(ctx)
		framework.ExpectNoError(err, "Failed to get metrics after retries")

		By("Verifying NVMe metrics")

		expectedMetrics := []string{
			"aws_ebs_csi_read_ops_total",
			"aws_ebs_csi_write_ops_total",
			"aws_ebs_csi_read_bytes_total",
			"aws_ebs_csi_write_bytes_total",
			"aws_ebs_csi_read_seconds_total",
			"aws_ebs_csi_write_seconds_total",
			"aws_ebs_csi_exceeded_iops_seconds_total",
			"aws_ebs_csi_exceeded_tp_seconds_total",
			"aws_ebs_csi_ec2_exceeded_iops_seconds_total",
			"aws_ebs_csi_ec2_exceeded_tp_seconds_total",
			"aws_ebs_csi_nvme_collector_scrapes_total",
			"aws_ebs_csi_nvme_collector_errors_total",
			"aws_ebs_csi_volume_queue_length",
			"aws_ebs_csi_read_io_latency_seconds",
			"aws_ebs_csi_write_io_latency_seconds",
			"aws_ebs_csi_nvme_collector_duration_seconds",
		}

		for _, metric := range expectedMetrics {
			Expect(metricsOutput).To(ContainSubstring(metric),
				fmt.Sprintf("Metric %s not found in response", metric))
		}
	})
})

func getMetricsWithRetry(ctx context.Context) (string, error) {
	var metricsOutput string

	backoff := wait.Backoff{
		Duration: 5 * time.Second,
		Factor:   2.0,
		Steps:    5,
	}

	err := wait.ExponentialBackoffWithContext(ctx, backoff, func(ctx context.Context) (bool, error) {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://localhost:3302/metrics", nil)
		if err != nil {
			return false, fmt.Errorf("failed to create request: %w", err)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			framework.Logf("Failed to get metrics: %v, retrying...", err)
			return false, nil
		}
		defer resp.Body.Close()

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			framework.Logf("Failed to read metrics: %v, retrying...", err)
			return false, nil
		}

		metricsOutput = string(body)
		return true, nil
	})

	return metricsOutput, err
}
