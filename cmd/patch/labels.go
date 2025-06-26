package patch

import (
	"context"
	json "encoding/json"
	"strconv"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/kubernetes-sigs/aws-ebs-csi-driver/pkg/cloud"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/strategicpatch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
)

type ENIsVolumes struct {
	ENIs    int
	Volumes int
}

func UpdateMetadataEC2(kubeclient kubernetes.Interface) error {
	nodes, _ := kubeclient.CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{})
	region := nodes.Items[0].GetLabels()[v1.LabelTopologyRegion] // We assume every node in the cluster has the same region
	cfg, err := config.LoadDefaultConfig(context.TODO(), config.WithRegion(region))

	if err != nil {
		klog.ErrorS(err, "unable to load SDK config")
		return err
	}

	svc := ec2.NewFromConfig(cfg)

	ENIsVolumeMap, err := GetMetadata(svc, region)
	if err != nil {
		klog.ErrorS(err, "unable to get ENI/Volume count")
		return err
	}

	err = PatchNodes(nodes, ENIsVolumeMap, kubeclient)
	if err != nil {
		return err
	}
	return nil
}

func GetMetadata(client cloud.EC2API, region string) (map[string]ENIsVolumes, error) {
	resp, err := client.DescribeInstances(context.TODO(), &ec2.DescribeInstancesInput{})
	if err != nil {
		klog.ErrorS(err, "failed to describe instances")
		return nil, err
	}

	ENIsVolumesMap := make(map[string]ENIsVolumes)
	for _, reservation := range resp.Reservations {
		for _, instance := range reservation.Instances {
			numAttachedENIs := 1
			if instance.NetworkInterfaces != nil {
				numAttachedENIs = len(instance.NetworkInterfaces)
			}
			numBlockDeviceMappings := 0
			if instance.BlockDeviceMappings != nil {
				numBlockDeviceMappings = len(instance.BlockDeviceMappings)
			}
			instanceID := *instance.InstanceId
			ENIsVolumesMap[instanceID] = ENIsVolumes{ENIs: numAttachedENIs, Volumes: numBlockDeviceMappings}
		}
	}
	return ENIsVolumesMap, nil
}

func PatchNodes(nodes *v1.NodeList, ENIsVolumeMap map[string]ENIsVolumes, clientset kubernetes.Interface) error {
	for _, node := range nodes.Items {
		newNode := node.DeepCopy()
		numAttachedENIs := ENIsVolumeMap[node.Name].ENIs
		numBlockDeviceMappings := ENIsVolumeMap[node.Name].Volumes
		newNode.Labels["num-volumes"] = strconv.Itoa(numBlockDeviceMappings)
		newNode.Labels["num-ENIs"] = strconv.Itoa(numAttachedENIs)

		oldData, err := json.Marshal(node)
		if err != nil {
			klog.V(1).InfoS("failed to marshal the existing node", "node", node.Name)
			return err
		}
		newData, err := json.Marshal(newNode)
		if err != nil {
			klog.V(1).InfoS("failed to marshal the new node", "node", newNode.Name)
			return err
		}
		patchBytes, err := strategicpatch.CreateTwoWayMergePatch(oldData, newData, &v1.Node{})
		if err != nil {
			klog.V(1).InfoS("failed to create two way merge", "node", node.Name)
			return err
		}
		if _, err := clientset.CoreV1().Nodes().Patch(context.TODO(), node.Name, types.StrategicMergePatchType, patchBytes, metav1.PatchOptions{}); err != nil {
			klog.ErrorS(err, "Failed to patch node", "node", node.Name)
			return err
		}
	}
	return nil
}
