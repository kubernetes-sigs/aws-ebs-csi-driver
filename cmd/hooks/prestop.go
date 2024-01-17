package hooks

import (
	"context"
	"fmt"
	"os"

	v1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"
)

/*
When a node is terminated, persistent workflows using EBS volumes can take 6+ minutes to start up again.
This happens when a volume is not cleanly unmounted, which causes the Attach/Detach controller (in kube-controller-manager)
to wait for 6 minutes before issuing a force detach and allowing the volume to be attached to another node.

This PreStop lifecycle hook aims to ensure that before the node (and the CSI driver node pod running on it) is shut down,
all VolumeAttachment objects associated with that node are removed, thereby indicating that all volumes have been successfully unmounted and detached.

No unnecessary delay is added to the termination workflow, as the PreStop hook logic is only executed when the node is being drained
(thus preventing delays in termination where the node pod is killed due to a rolling restart, or during driver upgrades, but the workload pods are expected to be running).
If the PreStop hook hangs during its execution, the driver node pod will be forcefully terminated after terminationGracePeriodSeconds, defined in the pod spec.
*/

func PreStop(clientset kubernetes.Interface) error {
	klog.InfoS("PreStop: executing PreStop lifecycle hook")

	nodeName := os.Getenv("CSI_NODE_NAME")
	if nodeName == "" {
		return fmt.Errorf("PreStop: CSI_NODE_NAME missing")
	}

	node, err := fetchNode(clientset, nodeName)
	if err != nil {
		return err
	}

	if isNodeBeingDrained(node) {
		klog.InfoS("PreStop: node is being drained, checking for remaining VolumeAttachments", "node", nodeName)
		return waitForVolumeAttachments(clientset, nodeName)
	}

	klog.InfoS("PreStop: node is not being drained, skipping VolumeAttachments check", "node", nodeName)
	return nil
}

func fetchNode(clientset kubernetes.Interface, nodeName string) (*v1.Node, error) {
	node, err := clientset.CoreV1().Nodes().Get(context.Background(), nodeName, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("fetchNode: failed to retrieve node information: %w", err)
	}
	return node, nil
}

func isNodeBeingDrained(node *v1.Node) bool {
	for _, taint := range node.Spec.Taints {
		if taint.Key == v1.TaintNodeUnschedulable && taint.Effect == v1.TaintEffectNoSchedule {
			return true
		}
	}
	return false
}

func waitForVolumeAttachments(clientset kubernetes.Interface, nodeName string) error {
	allAttachmentsDeleted := make(chan struct{})

	factory := informers.NewSharedInformerFactory(clientset, 0)
	informer := factory.Storage().V1().VolumeAttachments().Informer()

	_, err := informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		DeleteFunc: func(obj interface{}) {
			klog.V(5).InfoS("DeleteFunc: VolumeAttachment deleted", "node", nodeName)
			va := obj.(*storagev1.VolumeAttachment)
			if va.Spec.NodeName == nodeName {
				if err := checkVolumeAttachments(clientset, nodeName, allAttachmentsDeleted); err != nil {
					klog.ErrorS(err, "DeleteFunc: error checking VolumeAttachments")
				}
			}
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			klog.V(5).InfoS("UpdateFunc: VolumeAttachment updated", "node", nodeName)
			va := newObj.(*storagev1.VolumeAttachment)
			if va.Spec.NodeName == nodeName {
				if err := checkVolumeAttachments(clientset, nodeName, allAttachmentsDeleted); err != nil {
					klog.ErrorS(err, "UpdateFunc: error checking VolumeAttachments")
				}
			}
		},
	})
	if err != nil {
		return fmt.Errorf("failed to add event handler to VolumeAttachment informer: %w", err)
	}

	go informer.Run(allAttachmentsDeleted)

	if err := checkVolumeAttachments(clientset, nodeName, allAttachmentsDeleted); err != nil {
		klog.ErrorS(err, "waitForVolumeAttachments: error checking VolumeAttachments")
	}

	<-allAttachmentsDeleted
	klog.InfoS("waitForVolumeAttachments: finished waiting for VolumeAttachments to be deleted. preStopHook completed")
	return nil
}

func checkVolumeAttachments(clientset kubernetes.Interface, nodeName string, allAttachmentsDeleted chan struct{}) error {
	allAttachments, err := clientset.StorageV1().VolumeAttachments().List(context.Background(), metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("checkVolumeAttachments: failed to list VolumeAttachments: %w", err)
	}

	for _, attachment := range allAttachments.Items {
		if attachment.Spec.NodeName == nodeName {
			klog.InfoS("isVolumeAttachmentEmpty: not ready to exit, found VolumeAttachment", "attachment", attachment, "node", nodeName)
			return nil
		}
	}

	close(allAttachmentsDeleted)
	return nil
}
