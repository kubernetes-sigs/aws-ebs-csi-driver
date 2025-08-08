/*
Copyright 2023 The Kubernetes Authors.

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
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	v1 "k8s.io/api/storage/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/kubernetes/test/e2e/framework"
	admissionapi "k8s.io/pod-security-admission/api"
)

type metadata struct {
	ENIs             int
	Volumes          int
	InstanceType     string
	AllocatableCount int32
	NodeID           string
	AvailabilityZone string
}

const (
	// VolumesLabel is the label name for the number of volumes on a node
	VolumesLabel = "ebs.csi.aws.com/non-csi-ebs-volumes-count"

	// ENIsLabel is the label name for the number of ENIs on a node
	ENIsLabel = "ebs.csi.aws.com/enis-count"
)

var _ = Describe("[disruptive] EBS CSI Driver Node Labeling", func() {
	f := framework.NewDefaultFramework("ebs")
	f.NamespacePodSecurityEnforceLevel = admissionapi.LevelPrivileged

	var (
		ec2Client        *ec2.Client
		cs               clientset.Interface
		expectedMetadata map[string]*metadata
		labeledMetadata  map[string]*metadata
		cleanUp          []func()
	)

	BeforeEach(func() {
		cfg, err := config.LoadDefaultConfig(context.TODO())
		Expect(err).NotTo(HaveOccurred(), "Failed to load AWS SDK config")
		ec2Client = ec2.NewFromConfig(cfg)

		cs = f.ClientSet

		labeledMetadata = make(map[string]*metadata)
	})

	AfterEach(func() {
		for i := len(cleanUp) - 1; i >= 0; i-- {
			cleanUp[i]()
		}

		checkLabelsUpdated(cs, labeledMetadata, expectedMetadata)
		By("Deleting the EBS CSI node pods to reset allocatable counts")
		for instance := range labeledMetadata {
			deletePod(labeledMetadata[instance].NodeID, cs)
		}
		checkCSINodesUpdated(cs, labeledMetadata, expectedMetadata)
	})

	Describe("Node labeling volumes and ENIs", func() {
		It("should correctly label nodes with volume and ENI counts and have correct csinode allocatable counts", func() {
			By("Getting EC2 instance information")
			resp, err := ec2Client.DescribeInstances(context.TODO(), &ec2.DescribeInstancesInput{})
			Expect(err).NotTo(HaveOccurred(), "Failed to describe EC2 instances")
			expectedMetadata = getVolENIs(resp)

			By("Checking initial node labels")
			nodes, err := cs.CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{})
			Expect(err).NotTo(HaveOccurred(), "Failed to list nodes")
			checkVolENI(expectedMetadata, labeledMetadata, nodes)

			By("Checking CSI node allocatable counts")
			csiNodes, err := cs.StorageV1().CSINodes().List(context.TODO(), metav1.ListOptions{})
			Expect(err).NotTo(HaveOccurred(), "Failed to list CSI nodes")
			checkAllocatable(expectedMetadata, labeledMetadata, csiNodes)

			// For the instance that the new volume is attached to, the volume labels should increase by 1 and the allocatable count should decrease by 1
			By("Creating a new non CSI managed volume")
			firstChangedNonCSIVolumeInstance, firstCreatedNonCSIVolumeID := createNonCSIManagedVolume(ec2Client, expectedMetadata, "")
			cleanUp = append(cleanUp, func() {
				cleanUpVolume(firstCreatedNonCSIVolumeID, firstChangedNonCSIVolumeInstance, ec2Client, expectedMetadata)
			})
			By("Attaching the new non CSI managed volume")
			attachVolume(ec2Client, firstCreatedNonCSIVolumeID, firstChangedNonCSIVolumeInstance, "/dev/sdf", expectedMetadata)
			checkLabelsUpdated(cs, labeledMetadata, expectedMetadata)
			By("Deleting the EBS CSI node pod to trigger label update")
			deletePod(labeledMetadata[firstChangedNonCSIVolumeInstance].NodeID, cs)
			checkCSINodesUpdated(cs, labeledMetadata, expectedMetadata)

			// For the instance that the new volume is attached to, the volume labels and allocatable count should not change
			By("Creating and attaching a new CSI managed volume")
			createStorageClass(cs)
			cleanUp = append(cleanUp, func() { cleanUpStorageClass(cs, "ebs-sc") })
			pvc := createPVC(cs)
			cleanUp = append(cleanUp, func() { cleanUpPVC(cs, "default", "ebs-claim") })
			pod := createPod(cs)
			cleanUp = append(cleanUp, func() { cleanUpPod(cs, "default", "app") })
			changedCSIVolumeInstance := createCSIManagedVolume(cs, pvc, pod)

			// Because the previous step should not change volume labels/allocatable count, we add a non CSI managed volume to know that the
			// volume labels/allocatable count updated accordingly
			By("Creating a new non CSI managed volume")
			changedNonCSIVolumeInstance, createdNonCSIVolumeID := createNonCSIManagedVolume(ec2Client, expectedMetadata, changedCSIVolumeInstance)
			cleanUp = append(cleanUp, func() { cleanUpVolume(createdNonCSIVolumeID, changedNonCSIVolumeInstance, ec2Client, expectedMetadata) })
			By("Attaching the new non CSI managed volume")
			attachVolume(ec2Client, createdNonCSIVolumeID, changedNonCSIVolumeInstance, "/dev/sdh", expectedMetadata)
			checkLabelsUpdated(cs, labeledMetadata, expectedMetadata)
			By("Deleting the EBS CSI node pod to trigger label update")
			deletePod(labeledMetadata[changedNonCSIVolumeInstance].NodeID, cs)
			checkCSINodesUpdated(cs, labeledMetadata, expectedMetadata)

			By("Verifying updated node labels")
			updatedNodes, err := cs.CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{})
			Expect(err).NotTo(HaveOccurred(), "Failed to list updated nodes")
			checkVolENI(expectedMetadata, labeledMetadata, updatedNodes)

			By("Verifying updated CSI node allocatable counts")
			updatedCsiNodes, err := cs.StorageV1().CSINodes().List(context.TODO(), metav1.ListOptions{})
			Expect(err).NotTo(HaveOccurred(), "Failed to list updated CSI nodes")
			checkAllocatable(expectedMetadata, labeledMetadata, updatedCsiNodes)
		})
	})
})

// getAllocatableCount returns the limit of volumes that the node supports.
func getAllocatableCount(volumes, enis int) int32 {
	// because the expected number of volumes attached does not incude the root volume, we need to subtract 1 from
	// the allocatable count to account for this
	return int32(28-volumes-enis) - 1
}

// getVolENIs gets the expected metadata of each instance from the ec2 API
func getVolENIs(resp *ec2.DescribeInstancesOutput) map[string]*metadata {
	expectedMetadata := map[string]*metadata{}
	for _, reservation := range resp.Reservations {
		for _, instance := range reservation.Instances {
			instanceID := *instance.InstanceId

			numAttachedENIs := 0
			if instance.NetworkInterfaces != nil {
				numAttachedENIs = len(instance.NetworkInterfaces)
			}

			numBlockDeviceMappings := 0
			if instance.BlockDeviceMappings != nil {
				// we do not include the root volume in the expected number of volumes attached
				numBlockDeviceMappings = len(instance.BlockDeviceMappings) - 1
			}
			expectedMetadata[instanceID] = &metadata{
				ENIs:             numAttachedENIs,
				Volumes:          numBlockDeviceMappings,
				InstanceType:     string(instance.InstanceType),
				AvailabilityZone: *instance.Placement.AvailabilityZone,
			}
		}
	}
	return expectedMetadata
}

// checkVolENI compares `expectedMetadata` and `labeledMetadata` to have the same number of volumes and ENIs attached to each node in `nodes`
func checkVolENI(expectedMetadata, labeledMetadata map[string]*metadata, nodes *corev1.NodeList) {
	for _, node := range nodes.Items {
		vol, _ := strconv.Atoi(node.GetLabels()[VolumesLabel])
		enis, _ := strconv.Atoi(node.GetLabels()[ENIsLabel])
		id := parseNode(node.Spec.ProviderID)
		labeledMetadata[id] = &metadata{}
		labeledMetadata[id].ENIs = enis
		labeledMetadata[id].Volumes = vol
		labeledMetadata[id].NodeID = node.Name

		if expectedMetadata[id] != nil {
			if labeledMetadata[id].Volumes != expectedMetadata[id].Volumes {
				Fail(fmt.Sprintf("Volume count mismatch for node %s: expected %d, got %d\n",
					node.Name, expectedMetadata[id].Volumes, labeledMetadata[id].Volumes))
			}
			if labeledMetadata[id].ENIs != expectedMetadata[id].ENIs {
				Fail(fmt.Sprintf("ENI count mismatch for node %s: expected %d, got %d\n",
					node.Name, expectedMetadata[id].ENIs, labeledMetadata[id].ENIs))
			}
		}
	}
}

// checkAllocatable compares `expectedMetadata` and `labeledMetadata` to have the same allocatable count on each node in `nodes`
func checkAllocatable(expectedMetadata, labeledMetadata map[string]*metadata, csiNodes *storagev1.CSINodeList) {
	for _, csiNode := range csiNodes.Items {
		nodeID := csiNode.Name
		for _, driver := range csiNode.Spec.Drivers {
			labeledMetadata[nodeID].AllocatableCount = *driver.Allocatable.Count
			expectedMetadata[nodeID].AllocatableCount = getAllocatableCount(
				expectedMetadata[nodeID].Volumes,
				expectedMetadata[nodeID].ENIs)
			if labeledMetadata[nodeID].AllocatableCount != expectedMetadata[nodeID].AllocatableCount {
				Fail(fmt.Sprintf("Allocatable count mismatch for csi node %s, expected %d, got %d",
					nodeID, expectedMetadata[nodeID].AllocatableCount, labeledMetadata[nodeID].AllocatableCount))
			}
		}
	}
}

func createCSIManagedVolume(cs kubernetes.Interface, pvc *corev1.PersistentVolumeClaim, pod *corev1.Pod) string {
	Eventually(func() bool {
		pvcCheck, err := cs.CoreV1().PersistentVolumeClaims("default").Get(context.TODO(), pvc.Name, metav1.GetOptions{})
		if err != nil {
			fmt.Printf("Error getting PVC: %v\n", err)
			return false
		}

		if pvcCheck.Status.Phase == corev1.ClaimBound && pvcCheck.Spec.VolumeName != "" {
			return true
		}
		return false
	}, 5*time.Minute, 10*time.Second).Should(BeTrue(), "PVC should be bound with volume name")

	updatedPod, err := cs.CoreV1().Pods("default").Get(context.TODO(), pod.Name, metav1.GetOptions{})
	Expect(err).NotTo(HaveOccurred(), "failed to get updated pod")

	node, err := cs.CoreV1().Nodes().Get(context.TODO(), updatedPod.Spec.NodeName, metav1.GetOptions{})
	Expect(err).NotTo(HaveOccurred(), "failed to get node")

	instanceID := parseNode(node.Spec.ProviderID)

	return instanceID
}

func checkLabelsUpdated(cs kubernetes.Interface, labeledMetadata, expectedMetadata map[string]*metadata) {
	By("Waiting for labels to update")
	Eventually(func() bool {
		updatedNodes, err := cs.CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{})
		if err != nil {
			return false
		}

		for _, node := range updatedNodes.Items {
			id := parseNode(node.Spec.ProviderID)
			vol, _ := strconv.Atoi(node.Labels[VolumesLabel])
			eni, _ := strconv.Atoi(node.Labels[ENIsLabel])
			labeledMetadata[id].Volumes = vol
			labeledMetadata[id].ENIs = eni

			if vol != expectedMetadata[id].Volumes || eni != expectedMetadata[id].ENIs {
				return false
			}
		}
		return true
	}, "2m", "2s").Should(BeTrue(), "Node labels were not updated with correct volume count")
}

func checkCSINodesUpdated(cs kubernetes.Interface, labeledMetadata, expectedMetadata map[string]*metadata) {
	By("Waiting for CSI node allocatable count to update")
	Eventually(func() bool {
		updatedCsiNodes, err := cs.StorageV1().CSINodes().List(context.TODO(), metav1.ListOptions{})
		if err != nil {
			return false
		}

		for _, csiNode := range updatedCsiNodes.Items {
			nodeID := csiNode.Name
			for _, driver := range csiNode.Spec.Drivers {
				labeledMetadata[nodeID].AllocatableCount = *driver.Allocatable.Count
				expectedMetadata[nodeID].AllocatableCount = getAllocatableCount(
					expectedMetadata[nodeID].Volumes,
					expectedMetadata[nodeID].ENIs)
				if labeledMetadata[nodeID].AllocatableCount != expectedMetadata[nodeID].AllocatableCount {
					return false
				}
			}
		}
		return true
	}, "2m", "2s").Should(BeTrue(), "CSI node allocatable count were not updated with correct count")
}

func createNonCSIManagedVolume(ec2svc *ec2.Client, metadata map[string]*metadata, changedCSIVolumeInstance string) (string, string) {
	var instanceID string
	if changedCSIVolumeInstance != "" {
		instanceID = changedCSIVolumeInstance
	} else {
		for k := range metadata {
			instanceID = k
		}
	}

	createInput := &ec2.CreateVolumeInput{
		AvailabilityZone: aws.String(metadata[instanceID].AvailabilityZone),
		Size:             aws.Int32(1),
		VolumeType:       types.VolumeTypeGp3,
	}

	volumeResult, err := ec2svc.CreateVolume(context.TODO(), createInput)
	Expect(err).NotTo(HaveOccurred(), "Failed to create volume")
	volumeID := *volumeResult.VolumeId

	By("Waiting for volume to become available")
	Eventually(func() bool {
		describeInput := &ec2.DescribeVolumesInput{
			VolumeIds: []string{*volumeResult.VolumeId},
		}

		result, err := ec2svc.DescribeVolumes(context.TODO(), describeInput)
		if err != nil {
			return false
		}

		if len(result.Volumes) == 0 {
			return false
		}

		return result.Volumes[0].State == types.VolumeStateAvailable
	}, "2m", "5s").Should(BeTrue(), "Volume did not become available within expected time")

	return instanceID, volumeID
}

func attachVolume(ec2svc *ec2.Client, volumeID, instanceID, device string, metadata map[string]*metadata) bool {
	attachInput := &ec2.AttachVolumeInput{
		Device:     aws.String(device),
		InstanceId: aws.String(instanceID),
		VolumeId:   aws.String(volumeID),
	}

	_, err := ec2svc.AttachVolume(context.TODO(), attachInput)
	Expect(err).NotTo(HaveOccurred(), "Failed to attach volume")

	metadata[instanceID].Volumes += 1
	return true
}

func deletePod(nodeID string, cs clientset.Interface) {
	pods, err := cs.CoreV1().Pods("kube-system").List(context.TODO(), metav1.ListOptions{
		FieldSelector: "spec.nodeName=" + nodeID,
	})
	Expect(err).NotTo(HaveOccurred(), "Failed to list pods")

	var targetPod string
	for _, pod := range pods.Items {
		if strings.HasPrefix(pod.Name, "ebs-csi-node-") {
			targetPod = pod.Name
			break
		}
	}

	Expect(targetPod).NotTo(BeEmpty(), "Could not find ebs-csi-node pod on node "+nodeID)

	deletePolicy := metav1.DeletePropagationForeground
	deleteOptions := metav1.DeleteOptions{
		PropagationPolicy: &deletePolicy,
	}

	err = cs.CoreV1().Pods("kube-system").Delete(context.TODO(), targetPod, deleteOptions)
	Expect(err).NotTo(HaveOccurred(), "Failed to delete pod "+targetPod)
}

func parseNode(providerID string) string {
	if providerID != "" {
		parts := strings.Split(providerID, "/")
		if len(parts) > 0 {
			return parts[len(parts)-1]
		}
	}
	return ""
}

func createStorageClass(cs kubernetes.Interface) *v1.StorageClass {
	storageClass := &storagev1.StorageClass{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "storage.k8s.io/v1",
			Kind:       "StorageClass",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "ebs-sc",
		},
		Provisioner:       "ebs.csi.aws.com",
		VolumeBindingMode: func() *storagev1.VolumeBindingMode { v := storagev1.VolumeBindingWaitForFirstConsumer; return &v }()}

	_, err := cs.StorageV1().StorageClasses().Create(context.TODO(), storageClass, metav1.CreateOptions{})
	if err != nil {
		Expect(err).NotTo(HaveOccurred(), "Failed to create storage class")
	}
	return storageClass
}

func createPVC(cs kubernetes.Interface) *corev1.PersistentVolumeClaim {
	storageClassName := "ebs-sc"
	pvc := &corev1.PersistentVolumeClaim{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "PersistentVolumeClaim",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "ebs-claim",
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{
				corev1.ReadWriteOnce,
			},
			StorageClassName: &storageClassName,
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: resource.MustParse("4Gi"),
				},
			},
		},
	}
	_, err := cs.CoreV1().PersistentVolumeClaims("default").Create(context.TODO(), pvc, metav1.CreateOptions{})
	if err != nil {
		Expect(err).NotTo(HaveOccurred(), "Failed to create pvc")
	}
	return pvc
}

func createPod(cs kubernetes.Interface) *corev1.Pod {
	pod := &corev1.Pod{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Pod",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "app",
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:  "app",
					Image: "public.ecr.aws/amazonlinux/amazonlinux",
					VolumeMounts: []corev1.VolumeMount{
						{
							Name:      "persistent-storage",
							MountPath: "/data",
						},
					},
				},
			},
			Volumes: []corev1.Volume{
				{
					Name: "persistent-storage",
					VolumeSource: corev1.VolumeSource{
						PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
							ClaimName: "ebs-claim",
						},
					},
				},
			},
		},
	}
	_, err := cs.CoreV1().Pods("default").Create(context.TODO(), pod, metav1.CreateOptions{})
	if err != nil {
		Expect(err).NotTo(HaveOccurred(), "Failed to create pod")
	}
	return pod
}

func cleanUpPod(cs kubernetes.Interface, namespace, name string) {
	By("Deleting new pod")
	err := cs.CoreV1().Pods(namespace).Delete(context.TODO(), name, metav1.DeleteOptions{})
	Expect(err).NotTo(HaveOccurred())

	Eventually(func() bool {
		_, err := cs.CoreV1().Pods(namespace).Get(context.TODO(), name, metav1.GetOptions{})
		return err != nil
	}, "2m", "2s").Should(BeTrue())
}

func cleanUpVolume(volumeID, instanceID string, ec2Client *ec2.Client, expectedMetadata map[string]*metadata) {
	result, _ := ec2Client.DescribeVolumes(context.TODO(), &ec2.DescribeVolumesInput{
		VolumeIds: []string{volumeID},
	})

	volume := result.Volumes[0]

	if volume.State == types.VolumeStateInUse {
		By("Detaching the volume")

		detachInput := &ec2.DetachVolumeInput{
			VolumeId:   aws.String(volumeID),
			InstanceId: aws.String(instanceID),
		}

		_, err := ec2Client.DetachVolume(context.TODO(), detachInput)
		Expect(err).NotTo(HaveOccurred(), "Failed to detach volume")

		By("Waiting for volume to be detached")
		Eventually(func() bool {
			describeInput := &ec2.DescribeVolumesInput{
				VolumeIds: []string{volumeID},
			}

			result, err := ec2Client.DescribeVolumes(context.TODO(), describeInput)
			if err != nil || len(result.Volumes) == 0 {
				return false
			}

			return result.Volumes[0].State == types.VolumeStateAvailable
		}, "2m", "2s").Should(BeTrue(), "Volume did not detach within expected time")
	}

	By("Deleting the volume")
	deleteInput := &ec2.DeleteVolumeInput{
		VolumeId: aws.String(volumeID),
	}

	_, err := ec2Client.DeleteVolume(context.TODO(), deleteInput)
	Expect(err).NotTo(HaveOccurred(), "Failed to delete volume")
	expectedMetadata[instanceID].Volumes -= 1
}

func cleanUpPVC(cs kubernetes.Interface, namespace, name string) {
	By("Deleting PVC")
	err := cs.CoreV1().PersistentVolumeClaims(namespace).Delete(context.TODO(), name, metav1.DeleteOptions{})
	Expect(err).NotTo(HaveOccurred())

	Eventually(func() bool {
		_, err := cs.CoreV1().PersistentVolumeClaims(namespace).Get(context.TODO(), name, metav1.GetOptions{})
		return err != nil
	}, "2m", 5*time.Second).Should(BeTrue())
}

func cleanUpStorageClass(cs kubernetes.Interface, name string) {
	By("Deleting StorageClass")
	err := cs.StorageV1().StorageClasses().Delete(context.TODO(), name, metav1.DeleteOptions{})
	Expect(err).NotTo(HaveOccurred())

	Eventually(func() bool {
		_, err := cs.StorageV1().StorageClasses().Get(context.TODO(), name, metav1.GetOptions{})
		return err != nil
	}, 2*time.Minute, 5*time.Second).Should(BeTrue())
}
