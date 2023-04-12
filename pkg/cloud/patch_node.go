package cloud

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8sTypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/klog/v2"
)

type JSONPatch struct {
	OP    string      `json:"op,omitempty"`
	Path  string      `json:"path,omitempty"`
	Value interface{} `json:"value"`
}

// RemoveNodeTaint() patched the node, removes the taint that match NodeTaintKey
func RemoveNodeTaint(k8sAPIClient KubernetesAPIClient, NodeTaintKey string) error {
	nodeName := os.Getenv("CSI_NODE_NAME")
	if nodeName == "" {
		return fmt.Errorf("CSI_NODE_NAME env var not set")
	}

	clientset, err := k8sAPIClient()
	if err != nil {
		return err
	}

	node, err := clientset.CoreV1().Nodes().Get(context.TODO(), nodeName, metav1.GetOptions{})
	if err != nil {
		return err
	}

	var taints []corev1.Taint
	hasTaint := false
	for _, taint := range node.Spec.Taints {
		if taint.Key != NodeTaintKey {
			taints = append(taints, taint)
		} else {
			hasTaint = true
			klog.InfoS("Node taint found")
		}
	}

	if !hasTaint {
		return fmt.Errorf("could not find node taint, key: %v, node: %v", NodeTaintKey, nodeName)
	}

	createStatusAndNodePatch := []JSONPatch{
		{
			OP:    "test",
			Path:  "/spec/taints",
			Value: node.Spec.Taints,
		},
		{
			OP:    "replace",
			Path:  "/spec/taints",
			Value: taints,
		},
	}

	patch, err := json.Marshal(createStatusAndNodePatch)
	if err != nil {
		return err
	}

	_, err = clientset.CoreV1().Nodes().Patch(context.TODO(), nodeName, k8sTypes.JSONPatchType, patch, metav1.PatchOptions{})
	if err != nil {
		return fmt.Errorf("failed to patch node when removing taint: error %w", err)
	}
	klog.InfoS("Successfully removed taint", "key", NodeTaintKey, "node", nodeName)
	return nil
}
