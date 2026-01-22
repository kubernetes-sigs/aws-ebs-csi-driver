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

package hooks

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
)

func TestPreStopHook(t *testing.T) {
	testCases := []struct {
		name           string
		nodeName       string
		expErr         error
		expErrContains string
		setup          func(t *testing.T, nodeName string) kubernetes.Interface
		asyncAction    func(t *testing.T, client kubernetes.Interface, nodeName string)
	}{
		{
			name:     "TestPreStopHook: CSI_NODE_NAME not set",
			nodeName: "",
			expErr:   errors.New("PreStop: CSI_NODE_NAME missing"),
			setup: func(t *testing.T, nodeName string) kubernetes.Interface {
				t.Helper()
				return fake.NewClientset()
			},
		},
		{
			name:     "TestPreStopHook: node does not exist, checks for remaining VolumeAttachments",
			nodeName: "test-node",
			expErr:   nil,
			setup: func(t *testing.T, nodeName string) kubernetes.Interface {
				t.Helper()
				// Create client without the node - this will cause Get to return NotFound
				// The prestop hook treats this as a termination event and checks for VolumeAttachments
				return fake.NewClientset()
			},
		},
		{
			name:     "TestPreStopHook: node is not being drained, skipping VolumeAttachments check - missing TaintNodeUnschedulable",
			nodeName: "test-node",
			expErr:   nil,
			setup: func(t *testing.T, nodeName string) kubernetes.Interface {
				t.Helper()
				node := &v1.Node{
					ObjectMeta: metav1.ObjectMeta{
						Name: nodeName,
					},
					Spec: v1.NodeSpec{
						Taints: []v1.Taint{},
					},
				}
				return fake.NewClientset(node)
			},
		},
		{
			name:     "TestPreStopHook: node is being drained, no volume attachments remain",
			nodeName: "test-node",
			expErr:   nil,
			setup: func(t *testing.T, nodeName string) kubernetes.Interface {
				t.Helper()
				node := &v1.Node{
					ObjectMeta: metav1.ObjectMeta{
						Name: nodeName,
					},
					Spec: v1.NodeSpec{
						Taints: []v1.Taint{
							{
								Key:    v1.TaintNodeUnschedulable,
								Effect: v1.TaintEffectNoSchedule,
							},
						},
					},
				}
				return fake.NewClientset(node)
			},
		},
		{
			name:     "TestPreStopHook: node is being drained, no volume attachments associated with node",
			nodeName: "test-node",
			expErr:   nil,
			setup: func(t *testing.T, nodeName string) kubernetes.Interface {
				t.Helper()
				node := &v1.Node{
					ObjectMeta: metav1.ObjectMeta{
						Name: nodeName,
					},
					Spec: v1.NodeSpec{
						Taints: []v1.Taint{
							{
								Key:    v1.TaintNodeUnschedulable,
								Effect: v1.TaintEffectNoSchedule,
							},
						},
					},
				}
				va := &storagev1.VolumeAttachment{
					ObjectMeta: metav1.ObjectMeta{
						Name: "va-other-node",
					},
					Spec: storagev1.VolumeAttachmentSpec{
						NodeName: "test-node-2",
						Attacher: "ebs.csi.aws.com",
					},
				}
				return fake.NewClientset(node, va)
			},
		},
		{
			name:     "TestPreStopHook: Node is drained before timeout",
			nodeName: "test-node",
			expErr:   nil,
			setup: func(t *testing.T, nodeName string) kubernetes.Interface {
				t.Helper()
				node := &v1.Node{
					ObjectMeta: metav1.ObjectMeta{
						Name: nodeName,
					},
					Spec: v1.NodeSpec{
						Taints: []v1.Taint{
							{
								Key:    v1.TaintNodeUnschedulable,
								Effect: v1.TaintEffectNoSchedule,
							},
						},
					},
				}
				va := &storagev1.VolumeAttachment{
					ObjectMeta: metav1.ObjectMeta{
						Name: "va-test-node",
					},
					Spec: storagev1.VolumeAttachmentSpec{
						NodeName: nodeName,
						Attacher: "ebs.csi.aws.com",
					},
				}
				return fake.NewClientset(node, va)
			},
			asyncAction: func(t *testing.T, client kubernetes.Interface, nodeName string) {
				t.Helper()
				// Delete the volume attachment after a short delay
				time.Sleep(50 * time.Millisecond)
				err := client.StorageV1().VolumeAttachments().Delete(t.Context(), "va-test-node", metav1.DeleteOptions{})
				if err != nil {
					t.Logf("Failed to delete volume attachment: %v", err)
				}
			},
		},
		{
			name:     "TestPreStopHook: Karpenter node is being drained, no volume attachments remain",
			nodeName: "test-karpenter-node",
			expErr:   nil,
			setup: func(t *testing.T, nodeName string) kubernetes.Interface {
				t.Helper()
				node := &v1.Node{
					ObjectMeta: metav1.ObjectMeta{
						Name: nodeName,
					},
					Spec: v1.NodeSpec{
						Taints: []v1.Taint{
							{
								Key:    v1beta1KarpenterTaint,
								Effect: v1.TaintEffectNoSchedule,
							},
						},
					},
				}
				return fake.NewClientset(node)
			},
		},
		{
			name:     "TestPreStopHook: Karpenter node is being drained, no volume attachments associated with node",
			nodeName: "test-karpenter-node",
			expErr:   nil,
			setup: func(t *testing.T, nodeName string) kubernetes.Interface {
				t.Helper()
				node := &v1.Node{
					ObjectMeta: metav1.ObjectMeta{
						Name: nodeName,
					},
					Spec: v1.NodeSpec{
						Taints: []v1.Taint{
							{
								Key:    v1beta1KarpenterTaint,
								Effect: v1.TaintEffectNoSchedule,
							},
						},
					},
				}
				va := &storagev1.VolumeAttachment{
					ObjectMeta: metav1.ObjectMeta{
						Name: "va-other-node",
					},
					Spec: storagev1.VolumeAttachmentSpec{
						NodeName: "test-node-2",
						Attacher: "ebs.csi.aws.com",
					},
				}
				return fake.NewClientset(node, va)
			},
		},
		{
			name:     "TestPreStopHook: Karpenter Node is drained before timeout",
			nodeName: "test-karpenter-node",
			expErr:   nil,
			setup: func(t *testing.T, nodeName string) kubernetes.Interface {
				t.Helper()
				node := &v1.Node{
					ObjectMeta: metav1.ObjectMeta{
						Name: nodeName,
					},
					Spec: v1.NodeSpec{
						Taints: []v1.Taint{
							{
								Key:    v1beta1KarpenterTaint,
								Effect: v1.TaintEffectNoSchedule,
							},
						},
					},
				}
				va := &storagev1.VolumeAttachment{
					ObjectMeta: metav1.ObjectMeta{
						Name: "va-karpenter-node",
					},
					Spec: storagev1.VolumeAttachmentSpec{
						NodeName: nodeName,
						Attacher: "ebs.csi.aws.com",
					},
				}
				return fake.NewClientset(node, va)
			},
			asyncAction: func(t *testing.T, client kubernetes.Interface, nodeName string) {
				t.Helper()
				// Delete the volume attachment after a short delay
				time.Sleep(50 * time.Millisecond)
				err := client.StorageV1().VolumeAttachments().Delete(t.Context(), "va-karpenter-node", metav1.DeleteOptions{})
				if err != nil {
					t.Logf("Failed to delete volume attachment: %v", err)
				}
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			client := tc.setup(t, tc.nodeName)

			if tc.nodeName != "" {
				t.Setenv("CSI_NODE_NAME", tc.nodeName)
			}

			if tc.asyncAction != nil {
				go tc.asyncAction(t, client, tc.nodeName)
			}

			err := PreStop(client)

			switch {
			case tc.expErrContains != "":
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.expErrContains,
					"expected error containing %q, got %q", tc.expErrContains, err.Error())
			case tc.expErr != nil:
				require.Error(t, err)
				assert.Equal(t, tc.expErr.Error(), err.Error())
			default:
				require.NoError(t, err)
			}
		})
	}
}

func TestIsNodeBeingDrained(t *testing.T) {
	testCases := []struct {
		name         string
		nodeTaintKey string
		want         bool
	}{
		{"Should recognize common eviction taint", v1.TaintNodeUnschedulable, true},
		{"Should recognize cluster autoscaler taint", clusterAutoscalerTaint, true},
		{"Should recognize Karpenter v1beta1 taint", v1beta1KarpenterTaint, true},
		{"Should recognize Karpenter v1 taint", v1KarpenterTaint, true},
		{"Should not block on generic taint", "ebs/fake-taint", false},
		{"Should not block on no taint", "", false},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var taints []v1.Taint

			if tc.nodeTaintKey != "" {
				taint := v1.Taint{
					Key:    tc.nodeTaintKey,
					Value:  "",
					Effect: v1.TaintEffectNoSchedule,
				}

				taints = append(taints, taint)
			}

			testNode := &v1.Node{
				Spec: v1.NodeSpec{
					Taints: taints,
				},
			}

			got := isNodeBeingDrained(testNode)

			if tc.want != got {
				t.Fatalf("isNodeBeingDrained returned wrong answer when node contained taint with key: %s; got: %t, want: %t", tc.nodeTaintKey, got, tc.want)
			}
		})
	}
}
