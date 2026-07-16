/*
Copyright 2025 The Kubernetes Authors.

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
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/kubernetes/test/e2e/framework"
	admissionapi "k8s.io/pod-security-admission/api"
)

// Validates that the driver has ec2:DescribeInstanceTypes permission and that
// the API returns valid EBS card information for the cluster's instance types.
// This permission is required at runtime to resolve multi-card instance types.
var _ = Describe("[ebs-csi-e2e] [functional] Multi-Card", func() {
	f := framework.NewDefaultFramework("ebs")
	f.NamespacePodSecurityEnforceLevel = admissionapi.LevelPrivileged

	var cs clientset.Interface

	BeforeEach(func() {
		cs = f.ClientSet
	})

	cfg, err := config.LoadDefaultConfig(context.Background())
	if err != nil {
		Fail(fmt.Sprintf("failed to load AWS config: %v", err))
	}
	ec2Client := ec2.NewFromConfig(cfg)

	It("should successfully call DescribeInstanceTypes for cluster node instance types", func() {
		ctx := context.Background()

		nodes, err := cs.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
		framework.ExpectNoError(err, "listing nodes")
		Expect(nodes.Items).NotTo(BeEmpty(), "cluster should have at least one node")

		seen := map[string]bool{}
		for _, node := range nodes.Items {
			instanceType := node.Labels["node.kubernetes.io/instance-type"]
			if instanceType == "" || seen[instanceType] {
				continue
			}
			seen[instanceType] = true

			By(fmt.Sprintf("Calling DescribeInstanceTypes for %s", instanceType))
			resp, err := ec2Client.DescribeInstanceTypes(ctx, &ec2.DescribeInstanceTypesInput{
				InstanceTypes: []ec2types.InstanceType{ec2types.InstanceType(instanceType)},
			})
			framework.ExpectNoError(err, "DescribeInstanceTypes for %s", instanceType)
			Expect(resp.InstanceTypes).To(HaveLen(1), "expected exactly 1 result for %s", instanceType)

			info := resp.InstanceTypes[0]
			Expect(info.EbsInfo).NotTo(BeNil(), "EbsInfo should not be nil for %s", instanceType)

			cards := int32(1)
			if info.EbsInfo.MaximumEbsCards != nil {
				cards = *info.EbsInfo.MaximumEbsCards
			}
			Expect(cards).To(BeNumerically(">=", 1), "card count should be >= 1 for %s", instanceType)
			framework.Logf("Instance type %s has %d EBS card(s)", instanceType, cards)
		}
	})
})
