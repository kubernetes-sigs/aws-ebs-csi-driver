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

package metadata

import (
	"context"
	json "encoding/json"
	"strconv"
	"strings"
	"time"

	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/kubernetes-sigs/aws-ebs-csi-driver/pkg/cloud"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/strategicpatch"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/leaderelection"
	rl "k8s.io/client-go/tools/leaderelection/resourcelock"
	"k8s.io/klog/v2"
)

const (
	// VolumesLabel is the label name for the number of volumes on a node.
	VolumesLabel = "ebs.csi.aws.com/non-csi-ebs-volumes-count"

	// ENIsLabel is the label name for the number of ENIs on a node.
	ENIsLabel = "ebs.csi.aws.com/enis-count"

	// RenewDeadline is lease duration of the resource lock for leader election to update Nodes with additional ebs-csi-driver metadata labels.
	RenewDeadline = 10
)

type enisVolumes struct {
	ENIs    int
	Volumes int
}

// Uses leader election so that only one controller pod calls containuousUpdateLabels().
func ContinuousUpdateLabelsLeaderElection(clientset kubernetes.Interface, k8sConfig *rest.Config, instanceID string, cloud cloud.Cloud, updateTime int) {
	var (
		lockName      = "my-lock"
		lockNamespace = "kube-system"
	)

	l, err := rl.NewFromKubeconfig(
		rl.LeasesResourceLock,
		lockNamespace,
		lockName,
		rl.ResourceLockConfig{
			Identity: instanceID, //TODO: question: what's a better identity to use
		},
		k8sConfig,
		time.Second*RenewDeadline,
	)
	if err != nil {
		klog.ErrorS(err, "Could not set up leader election for updating Nodes with additional ebs-csi-driver metadata labels")
	}
	el, err := leaderelection.NewLeaderElector(leaderelection.LeaderElectionConfig{
		Lock:          l,
		LeaseDuration: time.Second * 15,
		RenewDeadline: time.Second * 10,
		RetryPeriod:   time.Second * 2,
		Name:          lockName,
		Callbacks: leaderelection.LeaderCallbacks{
			OnStartedLeading: func(ctx context.Context) {
				klog.V(2).InfoS("This container became the leader for updating Nodes with additional ebs-csi-driver metadata labels")
				continuousUpdateLabels(ctx, clientset, cloud, updateTime)
			},
			OnStoppedLeading: func() {
				klog.V(2).InfoS("This container is no longer the leader for updating Nodes with additional ebs-csi-driver metadata labels")
			},
			OnNewLeader: func(identity string) {
				klog.V(2).InfoS("The leader for updating Nodes with additional ebs-csi-driver metadata labels is", "", identity)
			},
		},
	})
	if err != nil {
		klog.ErrorS(err, "Leader for updating Nodes with additional ebs-csi-driver metadata labels could not be created")
	}

	el.Run(context.Background())
}

// continuousUpdateLabels is a go routine that updates the metadata labels of each node once every
// `updateTime` minutes and uses an informer to update the labels of new nodes that join the cluster.
// A PV informer is also used to keep track of CSI managed volumes when updating labels to avoid
// double counting.
func continuousUpdateLabels(ctx context.Context, k8sClient kubernetes.Interface, cloud cloud.Cloud, updateTime int) {
	pvInformerFactory, pvInformer := pvInformer(k8sClient)
	pvInformerStopCh := make(chan struct{})
	pvInformerFactory.Start(pvInformerStopCh)
	pvInformerFactory.WaitForCacheSync(pvInformerStopCh)

	nodeInformer := metadataInformer(ctx, k8sClient, cloud, pvInformer)
	nodeInformerStopCh := make(chan struct{})
	nodeInformer.Start(nodeInformerStopCh)
	nodeInformer.WaitForCacheSync(nodeInformerStopCh)

	ticker := time.NewTicker(time.Duration(updateTime) * time.Minute)

	defer ticker.Stop()
	updateLabels(ctx, k8sClient, cloud, pvInformer)
	for range ticker.C {
		updateLabels(ctx, k8sClient, cloud, pvInformer)
	}
}

func getPvVolumeIDs(pvInformer cache.SharedIndexInformer) []string {
	var volumeHandles []string
	pvCache := pvInformer.GetStore().List()
	for _, pvObj := range pvCache {
		if pv, ok := pvObj.(*v1.PersistentVolume); ok {
			handle := pv.Spec.CSI.VolumeHandle
			volumeHandles = append(volumeHandles, handle)
		}
	}
	return volumeHandles
}

func getNonCSIManagedVolumes(pvInformer cache.SharedIndexInformer, volumes []ec2types.InstanceBlockDeviceMapping) int {
	nonCSIVolumes := len(volumes)
	pvs := getPvVolumeIDs(pvInformer)
	for _, vol := range volumes {
		for _, pv := range pvs {
			if *vol.Ebs.VolumeId == pv {
				nonCSIVolumes -= 1
			}
		}
	}
	return nonCSIVolumes
}

func isEbsCsiVolume(pv *v1.PersistentVolume) bool {
	if pv.Spec.CSI != nil && pv.Spec.CSI.Driver == "ebs.csi.aws.com" {
		return true
	}

	return false
}

// pvInformer creates an informer that watches for CSI managed EBS volumes.
func pvInformer(clientset kubernetes.Interface) (informers.SharedInformerFactory, cache.SharedIndexInformer) {
	factory := informers.NewSharedInformerFactoryWithOptions(clientset, 0)
	pvInformer := factory.Core().V1().PersistentVolumes().Informer()
	_, err := pvInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(newObj interface{}) {
			if pv, ok := newObj.(*v1.PersistentVolume); ok {
				if !isEbsCsiVolume(pv) {
					if err := pvInformer.GetStore().Delete(newObj); err != nil {
						klog.ErrorS(err, "unable to delete pv from pv informer cache")
					}
				}
			}
		},
		UpdateFunc: func(oldObj, obj interface{}) {
			if pv, ok := obj.(*v1.PersistentVolume); ok {
				if !isEbsCsiVolume(pv) {
					if err := pvInformer.GetStore().Delete(obj); err != nil {
						klog.ErrorS(err, "unable to delete pv from pv informer cache")
					}
				}
			}
		},
	})
	if err != nil {
		klog.ErrorS(err, "unable to add event handler for pv informer")
	}
	return factory, pvInformer
}

// metadataInformer returns an informer factory that patches metadata labels for new nodes that join the cluster.
func metadataInformer(ctx context.Context, clientset kubernetes.Interface, cloud cloud.Cloud, pvInformer cache.SharedIndexInformer) informers.SharedInformerFactory {
	factory := informers.NewSharedInformerFactory(clientset, 0)
	nodesInformer := factory.Core().V1().Nodes().Informer()
	var handler cache.ResourceEventHandlerFuncs
	handler.AddFunc = func(obj interface{}) {
		if nodeObj, ok := obj.(*v1.Node); ok {
			klog.V(4).InfoS("new node added to cluster", "node", nodeObj.Name)
			node := &v1.NodeList{
				Items: []v1.Node{*nodeObj},
			}
			err := updateMetadataEC2(ctx, clientset, cloud, node, pvInformer)
			if err != nil {
				klog.ErrorS(err, "unable to update ENI/Volume count on node labels", "node", node.Items[0].Name)
			}
		}
	}
	_, err := nodesInformer.AddEventHandler(handler)
	if err != nil {
		klog.ErrorS(err, "unable to add event handler for node informer")
	}

	return factory
}

func updateLabels(ctx context.Context, k8sClient kubernetes.Interface, cloud cloud.Cloud, pvCache cache.SharedIndexInformer) {
	nodes, err := getNodes(ctx, k8sClient)
	if err != nil {
		klog.ErrorS(err, "could not get nodes")
		return
	}
	err = updateMetadataEC2(ctx, k8sClient, cloud, nodes, pvCache)
	if err != nil {
		klog.ErrorS(err, "unable to update ENI/Volume count on node labels")
	}
}

func getNodes(ctx context.Context, kubeclient kubernetes.Interface) (*v1.NodeList, error) {
	nodes, err := kubeclient.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		klog.ErrorS(err, "could not get nodes")
		return nil, err
	}
	return nodes, nil
}

func updateMetadataEC2(ctx context.Context, kubeclient kubernetes.Interface, cloud cloud.Cloud, nodes *v1.NodeList, pvInformer cache.SharedIndexInformer) error {
	ENIsVolumeMap, err := getMetadata(ctx, cloud, nodes, pvInformer)
	if err != nil {
		klog.ErrorS(err, "unable to get ENI/Volume count")
		return err
	}

	err = patchNodes(ctx, nodes, ENIsVolumeMap, kubeclient)
	if err != nil {
		return err
	}
	return nil
}

// parseNodes gets the instance name from parsing the provider ID.
func parseNode(providerID string) string {
	if providerID != "" {
		parts := strings.Split(providerID, "/")
		if len(parts) > 0 {
			return parts[len(parts)-1]
		}
	}
	return ""
}

// getMetadata calls the EC2 API to get the number of ENIs and non-CSI managed volumes attached to each node.
func getMetadata(ctx context.Context, cloud cloud.Cloud, nodes *v1.NodeList, pvInformer cache.SharedIndexInformer) (map[string]enisVolumes, error) {
	nodeIds := make([]string, 0, len(nodes.Items))
	for _, node := range nodes.Items {
		id := parseNode(node.Spec.ProviderID)
		if strings.HasPrefix(id, "i-") {
			nodeIds = append(nodeIds, id)
		}
	}

	var resp *ec2types.Instance
	var err error
	var respList []*ec2types.Instance

	if len(nodeIds) > 1 {
		respList, err = cloud.GetInstances(ctx, nodeIds)
	} else if len(nodeIds) == 1 {
		resp, err = cloud.GetInstance(ctx, nodeIds[0])
		respList = []*ec2types.Instance{resp}
	}

	if err != nil {
		klog.ErrorS(err, "failed to describe instances")
		return nil, err
	}

	ENIsVolumesMap := make(map[string]enisVolumes)
	for _, instance := range respList {
		numAttachedENIs := 1
		if instance.NetworkInterfaces != nil {
			numAttachedENIs = len(instance.NetworkInterfaces)
		}
		numBlockDeviceMappings := 0
		if instance.BlockDeviceMappings != nil {
			// -1 for root volume because we eventually add this back in when calculating allocatable count in getVolumesLimit()
			nonCSIManagedVolumes := getNonCSIManagedVolumes(pvInformer, instance.BlockDeviceMappings)
			numBlockDeviceMappings = nonCSIManagedVolumes - 1
		}
		instanceID := *instance.InstanceId
		ENIsVolumesMap[instanceID] = enisVolumes{ENIs: numAttachedENIs, Volumes: numBlockDeviceMappings}
	}

	return ENIsVolumesMap, nil
}

// patchNodes patches the labels of each node to have the number of ENIs and non-CSI managed volumes attached to each node.
func patchNodes(ctx context.Context, nodes *v1.NodeList, enisVolumeMap map[string]enisVolumes, clientset kubernetes.Interface) error {
	for _, node := range nodes.Items {
		newNode := node.DeepCopy()
		numAttachedENIs := enisVolumeMap[parseNode(node.Spec.ProviderID)].ENIs
		numBlockDeviceMappings := enisVolumeMap[parseNode(node.Spec.ProviderID)].Volumes
		newNode.Labels[VolumesLabel] = strconv.Itoa(numBlockDeviceMappings)
		newNode.Labels[ENIsLabel] = strconv.Itoa(numAttachedENIs)

		oldData, err := json.Marshal(node)
		if err != nil {
			klog.ErrorS(err, "failed to marshal the existing node", "node", node.Name)
			return err
		}
		newData, err := json.Marshal(newNode)
		if err != nil {
			klog.ErrorS(err, "failed to marshal the new node", "node", newNode.Name)
			return err
		}
		patchBytes, err := strategicpatch.CreateTwoWayMergePatch(oldData, newData, &v1.Node{})
		if err != nil {
			klog.ErrorS(err, "failed to create two way merge", "node", node.Name)
			return err
		}
		if _, err := clientset.CoreV1().Nodes().Patch(ctx, node.Name, types.StrategicMergePatchType, patchBytes, metav1.PatchOptions{}); err != nil {
			klog.ErrorS(err, "Failed to patch node", "node", node.Name)
			return err
		}
		klog.V(4).InfoS("patched labels", "node", node.Name, "num volumes label", VolumesLabel, "num ENIs label", ENIsLabel)
	}
	return nil
}
