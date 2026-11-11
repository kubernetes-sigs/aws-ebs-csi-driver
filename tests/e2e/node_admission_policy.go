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

package e2e

import (
	"context"
	"encoding/json"
	"slices"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	authenticationv1 "k8s.io/api/authentication/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/util/retry"
	"k8s.io/kubernetes/test/e2e/framework"
	admissionapi "k8s.io/pod-security-admission/api"
)

const (
	nodePolicyName     = "ebs-csi-node"
	nodePodLabel       = "app=ebs-csi-node"
	agentNotReadyTaint = "ebs.csi.aws.com/agent-not-ready"
	otherTestTaint     = "ebs-csi-e2e/node-admission-policy"
)

var _ = framework.Describe("[ebs-csi-e2e] [Disruptive] Node admission policy", framework.WithDisruptive(), func() {
	f := framework.NewDefaultFramework("ebs")
	f.NamespacePodSecurityEnforceLevel = admissionapi.LevelPrivileged

	var (
		cs        clientset.Interface
		nodeCS    clientset.Interface
		ownNode   string
		otherNode string
	)

	BeforeEach(func(ctx context.Context) {
		cs = f.ClientSet

		_, err := cs.AdmissionregistrationV1().ValidatingAdmissionPolicies().Get(ctx, nodePolicyName, metav1.GetOptions{})
		if apierrors.IsNotFound(err) {
			Skip("ValidatingAdmissionPolicy " + nodePolicyName + " is not installed")
		}
		Expect(err).NotTo(HaveOccurred())

		pods, err := cs.CoreV1().Pods(driverNamespace).List(ctx, metav1.ListOptions{LabelSelector: nodePodLabel})
		Expect(err).NotTo(HaveOccurred())
		var nodePod *corev1.Pod
		for i := range pods.Items {
			p := &pods.Items[i]
			if p.Spec.NodeName == "" || p.Status.Phase != corev1.PodRunning {
				continue
			}
			if nodePod == nil {
				nodePod = p
			} else if p.Spec.NodeName != nodePod.Spec.NodeName {
				otherNode = p.Spec.NodeName
				break
			}
		}
		if nodePod == nil || otherNode == "" {
			Skip("need running EBS CSI node pods on at least two nodes")
		}
		ownNode = nodePod.Spec.NodeName

		// A token bound to the node pod, as the kubelet would mount into it
		expirationSeconds := int64(600)
		tr, err := cs.CoreV1().ServiceAccounts(driverNamespace).CreateToken(ctx, nodePod.Spec.ServiceAccountName, &authenticationv1.TokenRequest{
			Spec: authenticationv1.TokenRequestSpec{
				ExpirationSeconds: &expirationSeconds,
				BoundObjectRef: &authenticationv1.BoundObjectReference{
					Kind:       "Pod",
					APIVersion: "v1",
					Name:       nodePod.Name,
					UID:        nodePod.UID,
				},
			},
		}, metav1.CreateOptions{})
		Expect(err).NotTo(HaveOccurred())
		cfg := rest.AnonymousClientConfig(f.ClientConfig())
		cfg.BearerToken = tr.Status.Token
		nodeCS, err = clientset.NewForConfig(cfg)
		Expect(err).NotTo(HaveOccurred())

		for _, n := range []string{ownNode, otherNode} {
			addTestTaints(ctx, cs, n)
			DeferCleanup(removeTestTaints, cs, n)
		}
	})

	It("allows the node pods' service account to remove the agent-not-ready taint from the pod's node", func(ctx context.Context) {
		Expect(replaceTaints(ctx, nodeCS, cs, ownNode, withoutTaint(agentNotReadyTaint))).To(Succeed())
	})

	It("denies the node pods' service account every other Node write", func(ctx context.Context) {
		expectDenied := func(desc string, err error) {
			GinkgoHelper()
			Expect(err).To(HaveOccurred(), desc+" should be denied")
			Expect(apierrors.IsForbidden(err) || apierrors.IsInvalid(err)).To(BeTrue(), "%s: unexpected error %v", desc, err)
		}
		mergePatch := func(node string, patch string) error {
			_, err := nodeCS.CoreV1().Nodes().Patch(ctx, node, types.MergePatchType, []byte(patch), metav1.PatchOptions{})
			return err
		}

		expectDenied("removing agent-not-ready from another node",
			replaceTaints(ctx, nodeCS, cs, otherNode, withoutTaint(agentNotReadyTaint)))
		expectDenied("removing another taint",
			replaceTaints(ctx, nodeCS, cs, ownNode, withoutTaint(otherTestTaint)))
		expectDenied("adding a taint",
			replaceTaints(ctx, nodeCS, cs, ownNode, func(t []corev1.Taint) []corev1.Taint {
				return append(t, corev1.Taint{Key: otherTestTaint + "-added", Effect: corev1.TaintEffectNoSchedule})
			}))
		expectDenied("changing a label", mergePatch(ownNode, `{"metadata":{"labels":{"ebs-csi-e2e/test":"x"}}}`))
		expectDenied("changing an annotation", mergePatch(ownNode, `{"metadata":{"annotations":{"ebs-csi-e2e/test":"x"}}}`))
		expectDenied("setting spec.unschedulable", mergePatch(ownNode, `{"spec":{"unschedulable":true}}`))
		_, err := nodeCS.CoreV1().Nodes().Patch(ctx, ownNode, types.MergePatchType, []byte(`{"status":{"phase":"Pending"}}`), metav1.PatchOptions{}, "status")
		expectDenied("updating nodes/status", err)
		expectDenied("deleting a node", nodeCS.CoreV1().Nodes().Delete(ctx, otherNode, metav1.DeleteOptions{DryRun: []string{metav1.DryRunAll}}))
	})

	It("still lets the driver remove its startup taint", func(ctx context.Context) {
		pods, err := cs.CoreV1().Pods(driverNamespace).List(ctx, metav1.ListOptions{
			LabelSelector: nodePodLabel,
			FieldSelector: "spec.nodeName=" + ownNode,
		})
		Expect(err).NotTo(HaveOccurred())
		Expect(pods.Items).NotTo(BeEmpty())
		By("restarting the EBS CSI node pod on " + ownNode)
		oldPod := pods.Items[0].Name
		Expect(cs.CoreV1().Pods(driverNamespace).Delete(ctx, oldPod, metav1.DeleteOptions{})).To(Succeed())

		// Wait for the old pod to be fully removed so it doesn't outlive the test and break log collection in hack/e2e/run.sh.
		Eventually(func(g Gomega) {
			_, err := cs.CoreV1().Pods(driverNamespace).Get(ctx, oldPod, metav1.GetOptions{})
			g.Expect(apierrors.IsNotFound(err)).To(BeTrue())
		}).WithContext(ctx).WithTimeout(2 * time.Minute).WithPolling(5 * time.Second).Should(Succeed())

		Eventually(func(g Gomega) {
			n, err := cs.CoreV1().Nodes().Get(ctx, ownNode, metav1.GetOptions{})
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(slices.ContainsFunc(n.Spec.Taints, func(t corev1.Taint) bool { return t.Key == agentNotReadyTaint })).To(BeFalse())
		}).WithContext(ctx).WithTimeout(5 * time.Minute).WithPolling(5 * time.Second).Should(Succeed())
	})
})

// addTestTaints adds the agent-not-ready taint and another test taint. Both use NoSchedule so no pods are evicted.
func addTestTaints(ctx context.Context, cs clientset.Interface, node string) {
	GinkgoHelper()
	Expect(replaceTaints(ctx, cs, cs, node, func(taints []corev1.Taint) []corev1.Taint {
		for _, key := range []string{agentNotReadyTaint, otherTestTaint} {
			if !slices.ContainsFunc(taints, func(t corev1.Taint) bool { return t.Key == key }) {
				taints = append(taints, corev1.Taint{Key: key, Effect: corev1.TaintEffectNoSchedule})
			}
		}
		return taints
	})).To(Succeed())
}

func removeTestTaints(ctx context.Context, cs clientset.Interface, node string) {
	Expect(replaceTaints(ctx, cs, cs, node, func(taints []corev1.Taint) []corev1.Taint {
		return slices.DeleteFunc(taints, func(t corev1.Taint) bool {
			return t.Key == agentNotReadyTaint || t.Key == otherTestTaint
		})
	})).To(Succeed())
}

func withoutTaint(key string) func([]corev1.Taint) []corev1.Taint {
	return func(taints []corev1.Taint) []corev1.Taint {
		return slices.DeleteFunc(taints, func(t corev1.Taint) bool { return t.Key == key })
	}
}

// replaceTaints reads the node with reader and replaces its taints with a JSON patch sent by writer, like the driver does.
// The patch also sets the resourceVersion that was read, so the API server rejects it with a conflict instead of
// overwriting a taint change made by another controller since the read. replaceTaints then reads the node again and retries.
func replaceTaints(ctx context.Context, writer, reader clientset.Interface, node string, mutate func([]corev1.Taint) []corev1.Taint) error {
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		n, err := reader.CoreV1().Nodes().Get(ctx, node, metav1.GetOptions{})
		if err != nil {
			return err
		}
		taints := mutate(slices.Clone(n.Spec.Taints))
		if taints == nil {
			taints = []corev1.Taint{}
		}
		patch, err := json.Marshal([]map[string]any{
			{"op": "add", "path": "/metadata/resourceVersion", "value": n.ResourceVersion},
			{"op": "add", "path": "/spec/taints", "value": taints},
		})
		if err != nil {
			return err
		}
		_, err = writer.CoreV1().Nodes().Patch(ctx, node, types.JSONPatchType, patch, metav1.PatchOptions{})
		return err
	})
}
