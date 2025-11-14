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
	"fmt"
	"strconv"
	"strings"
	"time"

	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/kubernetes-csi/csi-lib-utils/leaderelection"
	"github.com/kubernetes-sigs/aws-ebs-csi-driver/pkg/cloud"
	"github.com/kubernetes-sigs/aws-ebs-csi-driver/pkg/util"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/strategicpatch"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"
)

const (
	// ControllerMetadataLabelerInterval is the interval metadata-labeler mode refreshes node labels with volume and ENI count.
	ControllerMetadataLabelerInterval = 60 * time.Minute

	// patchFails is the number of nodes we fail to patch before returning an error.
	patchFails = 5

	// numWorkersPatchLabels is the number of worker threads patching node labels.
	numWorkersPatchLabels = 10
)

// Initialized in ContinuousUpdateLabelsLeaderElection (depends on driver name).
var (
	// VolumesLabel is the label name for the number of volumes on a node.
	VolumesLabel string

	// ENIsLabel is the label name for the number of ENIs on a node.
	ENIsLabel string
)

type enisVolumes struct {
	ENIs    int
	Volumes int
}

// initVariables initializes variables that depend on driver name.
// Separated into a spearate function from ContinuousUpdateLabelsLeaderElection so it can be called in tests.
func initVariables() {
	VolumesLabel = util.GetDriverName() + "/non-csi-ebs-volumes-count"
	ENIsLabel = util.GetDriverName() + "/enis-count"
}

// ContinuousUpdateLabelsLeaderElection uses leader election so that only one controller pod calls continuousUpdateLabels().
func ContinuousUpdateLabelsLeaderElection(clientset kubernetes.Interface, cloud cloud.Cloud, updateTime time.Duration) error {
	initVariables()
	var (
		lockName = "metadata-labeler-" + util.GetDriverName()
	)
	le := leaderelection.NewLeaderElection(clientset, lockName, func(ctx context.Context) {
		err := continuousUpdateLabels(ctx, clientset, cloud, updateTime)
		if err != nil {
			klog.ErrorS(err, "Failed to patch node labels with volume/ENI count")
			return
		}
	})
	err := le.Run()
	if err != nil {
		klog.ErrorS(err, "Could not run leader election")
		return err
	}
	return nil
}

// continuousUpdateLabels is a go routine that updates the metadata labels of each node once every
// `updateTime` minutes and uses an informer to update the labels of new nodes that join the cluster.
// A PV informer is also used to keep track of CSI managed volumes when updating labels to avoid
// double counting.
func continuousUpdateLabels(ctx context.Context, k8sClient kubernetes.Interface, cloud cloud.Cloud, updateTime time.Duration) error {
	factory := informers.NewSharedInformerFactory(k8sClient, 0)
	pvInformer := factory.Core().V1().PersistentVolumes().Informer()
	err := pvInformer.AddIndexers(cache.Indexers{
		"volumeID": volumeIDIndexFunc,
	})
	if err != nil {
		klog.ErrorS(err, "Failed to add volume ID indexer")
		return err
	}
	nodesInformer := factory.Core().V1().Nodes().Informer()
	err = patchNewNodes(ctx, k8sClient, cloud, nodesInformer, pvInformer)
	if err != nil {
		klog.ErrorS(err, "Could not add event handler to informer to patch new nodes")
		return err
	}
	factory.Start(ctx.Done())
	factory.WaitForCacheSync(ctx.Done())

	err = updateLabels(ctx, k8sClient, cloud, pvInformer)
	if err != nil {
		klog.ErrorS(err, "Could not patch node labels with updated volume/ENI count")
		return err
	}
	for range time.Tick(updateTime) {
		err = updateLabels(ctx, k8sClient, cloud, pvInformer)
		if err != nil {
			klog.ErrorS(err, "Could not patch node labels with updated volume/ENI count")
			return err
		}
	}
	return nil
}

func volumeIDIndexFunc(obj interface{}) ([]string, error) {
	pv, ok := obj.(*v1.PersistentVolume)
	if !ok {
		return []string{}, nil
	}

	var volumeIDs []string
	var volumeID string

	if pv.Spec.CSI != nil && pv.Spec.CSI.Driver == util.GetDriverName() {
		volumeID = pv.Spec.CSI.VolumeHandle
	} else if pv.Spec.AWSElasticBlockStore != nil {
		volumeID = pv.Spec.AWSElasticBlockStore.VolumeID
	}

	if volumeID != "" {
		volumeIDs = append(volumeIDs, volumeID)
	}

	return volumeIDs, nil
}

func getNonCSIManagedVolumes(pvInformer cache.SharedIndexInformer, volumes []ec2types.InstanceBlockDeviceMapping) int {
	nonCSIVolumes := len(volumes)
	for _, vol := range volumes {
		if vol.Ebs != nil {
			volumeID := *vol.Ebs.VolumeId

			pvs, err := pvInformer.GetIndexer().ByIndex("volumeID", volumeID)
			if err != nil {
				klog.ErrorS(err, "Failed to query volume ID index", "volumeID", volumeID)
				continue
			}
			if len(pvs) > 0 {
				nonCSIVolumes -= 1
			}
		}
	}

	return nonCSIVolumes
}

// patchNewNodes patches metadata labels for new nodes that join the cluster.
func patchNewNodes(ctx context.Context, clientset kubernetes.Interface, cloud cloud.Cloud, nodesInformer, pvInformer cache.SharedIndexInformer) error {
	var handler cache.ResourceEventHandlerFuncs
	handler.AddFunc = func(obj interface{}) {
		if nodeObj, ok := obj.(*v1.Node); ok {
			klog.V(4).InfoS("New node added to cluster", "node", nodeObj.Name)
			node := &v1.NodeList{
				Items: []v1.Node{*nodeObj},
			}
			err := updateMetadataEC2(ctx, clientset, cloud, node, pvInformer)
			if err != nil {
				klog.ErrorS(err, "Unable to update ENI/Volume count on node labels", "node", node.Items[0].Name)
			}
		}
	}
	_, err := nodesInformer.AddEventHandler(handler)
	if err != nil {
		klog.ErrorS(err, "Unable to add event handler for node informer")
		return err
	}
	return nil
}

func updateLabels(ctx context.Context, k8sClient kubernetes.Interface, cloud cloud.Cloud, pvCache cache.SharedIndexInformer) error {
	nodes, err := getNodes(ctx, k8sClient)
	if err != nil {
		klog.ErrorS(err, "Could not get nodes")
		return err
	}
	err = updateMetadataEC2(ctx, k8sClient, cloud, nodes, pvCache)
	if err != nil {
		klog.ErrorS(err, "Unable to update ENI/Volume count on node labels")
		return err
	}
	return nil
}

func getNodes(ctx context.Context, kubeclient kubernetes.Interface) (*v1.NodeList, error) {
	nodes, err := kubeclient.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		klog.ErrorS(err, "Could not get nodes")
		return nil, err
	}
	return nodes, nil
}

func updateMetadataEC2(ctx context.Context, kubeclient kubernetes.Interface, cloud cloud.Cloud, nodes *v1.NodeList, pvInformer cache.SharedIndexInformer) error {
	enisVolumeMap, err := getMetadata(ctx, cloud, nodes, pvInformer)
	if err != nil {
		klog.ErrorS(err, "Unable to get ENI/Volume count")
		return err
	}

	err = patchNodes(ctx, nodes, enisVolumeMap, kubeclient, patchFails)
	if err != nil {
		return err
	}
	return nil
}

// getMetadata calls the EC2 API to get the number of ENIs and non-CSI managed volumes attached to each node.
func getMetadata(ctx context.Context, cloud cloud.Cloud, nodes *v1.NodeList, pvInformer cache.SharedIndexInformer) (map[string]enisVolumes, error) {
	nodeIds := make([]string, 0, len(nodes.Items))
	for _, node := range nodes.Items {
		id, err := parseProviderID(&node)
		if err != nil {
			return nil, err
		}
		if strings.HasPrefix(id, "i-") {
			nodeIds = append(nodeIds, id)
		}
	}
	respList, err := cloud.GetInstancesPatching(ctx, nodeIds)

	if err != nil {
		klog.ErrorS(err, "Failed to describe instances")
		return nil, err
	}

	enisVolumesMap := make(map[string]enisVolumes, len(respList))
	for _, instance := range respList {
		numAttachedENIs := 1
		if instance.NetworkInterfaces != nil {
			numAttachedENIs = len(instance.NetworkInterfaces)
		}
		numBlockDeviceMappings := 0
		if instance.BlockDeviceMappings != nil {
			// -1 for root volume because we eventually add this back in when calculating allocatable count in getVolumesLimit()
			numBlockDeviceMappings = getNonCSIManagedVolumes(pvInformer, instance.BlockDeviceMappings) - 1
		}
		enisVolumesMap[*instance.InstanceId] = enisVolumes{ENIs: numAttachedENIs, Volumes: numBlockDeviceMappings}
	}

	return enisVolumesMap, nil
}

// patchNodes patches the labels of each node to have the number of ENIs and non-CSI managed volumes attached to each node.
func patchNodes(ctx context.Context, nodes *v1.NodeList, enisVolumeMap map[string]enisVolumes, clientset kubernetes.Interface, patchFails int) error {
	numWorkers := min(len(nodes.Items), numWorkersPatchLabels)
	if numWorkers == 0 {
		return nil
	}

	jobs := make(chan v1.Node, len(nodes.Items))
	results := make(chan error, len(nodes.Items))

	for range numWorkers {
		go func() {
			for node := range jobs {
				results <- patchSingleNode(ctx, node, enisVolumeMap, clientset)
			}
		}()
	}

	for _, node := range nodes.Items {
		jobs <- node
	}
	close(jobs)

	var failures int
	for range len(nodes.Items) {
		if err := <-results; err != nil {
			failures++
			if failures == patchFails {
				return fmt.Errorf("failed to patch %d nodes", patchFails)
			}
		}
	}

	return nil
}

func patchSingleNode(ctx context.Context, node v1.Node, enisVolumeMap map[string]enisVolumes, clientset kubernetes.Interface) error {
	instanceID, err := parseProviderID(&node)
	if err != nil {
		klog.Error(err, "Could not get instanceID", "node", node.Name)
		return err
	}

	newNode := node.DeepCopy()
	numAttachedENIs := enisVolumeMap[instanceID].ENIs
	numBlockDeviceMappings := enisVolumeMap[instanceID].Volumes
	newNode.Labels[VolumesLabel] = strconv.Itoa(numBlockDeviceMappings)
	newNode.Labels[ENIsLabel] = strconv.Itoa(numAttachedENIs)

	oldData, err := json.Marshal(node)
	if err != nil {
		klog.ErrorS(err, "Failed to marshal the existing node", "node", node.Name)
		return err
	}
	newData, err := json.Marshal(newNode)
	if err != nil {
		klog.ErrorS(err, "Failed to marshal the new node", "node", newNode.Name)
		return err
	}
	patchBytes, err := strategicpatch.CreateTwoWayMergePatch(oldData, newData, &v1.Node{})
	if err != nil {
		klog.ErrorS(err, "Failed to create two way merge", "node", node.Name)
		return err
	}
	if _, err := clientset.CoreV1().Nodes().Patch(ctx, node.Name, types.StrategicMergePatchType, patchBytes, metav1.PatchOptions{}); err != nil {
		klog.ErrorS(err, "Failed to patch node", "node", node.Name)
		return err
	}
	klog.V(6).InfoS("Patched labels", "node", node.Name, "num volumes label", VolumesLabel, "num ENIs label", ENIsLabel)
	return nil
}
