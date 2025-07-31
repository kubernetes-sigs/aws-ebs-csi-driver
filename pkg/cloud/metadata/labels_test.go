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
	"errors"
	"fmt"
	reflect "reflect"
	"strconv"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	gomock "github.com/golang/mock/gomock"
	"github.com/kubernetes-sigs/aws-ebs-csi-driver/pkg/cloud"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes/fake"
	clienttesting "k8s.io/client-go/testing"
	"k8s.io/client-go/tools/cache"
)

func TestMetadataInformer(t *testing.T) {
	testCases := []struct {
		name             string
		newNode          *corev1.Node
		newPV            *corev1.PersistentVolume
		expectedMetadata map[string]enisVolumes
		expErr           error
	}{
		{
			name: "success: normal, new node added",
			newNode: &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name:   "i-001",
					Labels: make(map[string]string),
				},
				Spec: corev1.NodeSpec{
					ProviderID: "example/i-001",
				},
			},
			expectedMetadata: map[string]enisVolumes{
				"i-001": {ENIs: 2, Volumes: 2},
			},
			expErr: nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockCtrl := gomock.NewController(t)
			mockCloud := cloud.NewMockCloud(mockCtrl)

			mockCloud.EXPECT().GetInstance(gomock.Any(), gomock.Any()).Return(
				newFakeInstance(tc.newNode.Name, tc.expectedMetadata[tc.newNode.Name].ENIs, tc.expectedMetadata[tc.newNode.Name].Volumes+1),
				tc.expErr,
			)

			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()

			pvWatcherStarted := make(chan struct{})
			pvClientset := fake.NewSimpleClientset()
			pvClientset.PrependWatchReactor("*", func(action clienttesting.Action) (handled bool, ret watch.Interface, err error) {
				gvr := action.GetResource()
				ns := action.GetNamespace()
				watch, err := pvClientset.Tracker().Watch(gvr, ns)
				if err != nil {
					return false, nil, err
				}
				close(pvWatcherStarted)
				return true, watch, nil
			})
			pvInformerFactory, pvInformer := pvInformer(pvClientset)
			pvInformerFactory.Start(ctx.Done())
			cache.WaitForCacheSync(ctx.Done())
			<-pvWatcherStarted

			nodeWatcherStarted := make(chan struct{})
			nodeClientset := fake.NewSimpleClientset()
			nodeClientset.PrependWatchReactor("*", func(action clienttesting.Action) (handled bool, ret watch.Interface, err error) {
				gvr := action.GetResource()
				ns := action.GetNamespace()
				watch, err := nodeClientset.Tracker().Watch(gvr, ns)
				if err != nil {
					return false, nil, err
				}
				close(nodeWatcherStarted)
				return true, watch, nil
			})
			nodeInformer := metadataInformer(ctx, nodeClientset, mockCloud, pvInformer)
			nodeInformer.Start(ctx.Done())
			cache.WaitForCacheSync(ctx.Done())
			<-nodeWatcherStarted

			_, err := nodeClientset.CoreV1().Nodes().Create(t.Context(), tc.newNode, metav1.CreateOptions{})
			if err != nil {
				t.Fatalf("error injecting node add: %v", err)
			}

			time.Sleep(5e8)
			node, _ := nodeClientset.CoreV1().Nodes().Get(t.Context(), tc.newNode.Name, metav1.GetOptions{})
			if err != nil {
				if tc.expErr == nil {
					t.Fatalf("MetadataInformer() failed: expected no error, got: %v", err)
				}
				if err.Error() != tc.expErr.Error() {
					t.Fatalf("MetadataInformer() failed: expected error %q, got %q", tc.expErr, err)
				}
			} else {
				expectedENIs := strconv.Itoa(tc.expectedMetadata[node.Name].ENIs)
				expectedVol := strconv.Itoa(tc.expectedMetadata[node.Name].Volumes)

				labeledENIs := node.GetLabels()[ENIsLabel]
				labeledVol := node.GetLabels()[VolumesLabel]

				if labeledENIs != expectedENIs {
					t.Fatalf("MetadataInformer() failed: expected %s ENIs, got %s", expectedENIs, labeledENIs)
				}
				if labeledVol != expectedVol {
					t.Fatalf("MetadataInformer() failed: expected %s volumes, got %s", expectedVol, labeledVol)
				}
			}
		})
	}
}

func newFakeInstance(instanceID string, numENIs, numVolumes int) *types.Instance {
	blockDevices := make([]types.InstanceBlockDeviceMapping, numVolumes)
	for i := range numVolumes {
		volumeID := fmt.Sprintf("vol-00%d", i+1)
		blockDevices[i] = types.InstanceBlockDeviceMapping{
			Ebs: &types.EbsInstanceBlockDevice{
				VolumeId: &volumeID,
			},
		}
	}

	return &types.Instance{
		InstanceId:          &instanceID,
		BlockDeviceMappings: blockDevices,
		NetworkInterfaces:   make([]types.InstanceNetworkInterface, numENIs),
	}
}

func mockAddPV(newPV *corev1.PersistentVolume, instances []*types.Instance) []*types.Instance {
	if newPV == nil {
		return instances
	}

	instances[0].BlockDeviceMappings = append(instances[0].BlockDeviceMappings,
		types.InstanceBlockDeviceMapping{
			Ebs: &types.EbsInstanceBlockDevice{
				VolumeId: &newPV.Spec.CSI.VolumeHandle,
			},
		})

	return instances
}

func TestGetMetadata(t *testing.T) {
	defaultNode := &corev1.NodeList{Items: []corev1.Node{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "i-001",
			},

			Spec: corev1.NodeSpec{
				ProviderID: "example/i-001",
			}},
	}}

	testCases := []struct {
		name             string
		instances        []*types.Instance
		nodes            *corev1.NodeList
		expectedMetadata map[string]enisVolumes
		newPV            *corev1.PersistentVolume
		expErr           error
	}{
		{
			name:      "success: normal with multiple instances",
			instances: []*types.Instance{newFakeInstance("i-001", 1, 1), newFakeInstance("i-002", 2, 3)},
			nodes: &corev1.NodeList{Items: []corev1.Node{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "i-001",
					},

					Spec: corev1.NodeSpec{
						ProviderID: "example/i-001",
					}},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "i-002",
					},

					Spec: corev1.NodeSpec{
						ProviderID: "example/i-002",
					}},
			}},
			expectedMetadata: map[string]enisVolumes{
				"i-001": {ENIs: 1, Volumes: 0},
				"i-002": {ENIs: 2, Volumes: 2},
			},
			newPV:  nil,
			expErr: nil,
		},
		{
			name:      "success: normal with one instance",
			instances: []*types.Instance{newFakeInstance("i-001", 5, 2)},
			nodes:     defaultNode,
			expectedMetadata: map[string]enisVolumes{
				"i-001": {ENIs: 5, Volumes: 1},
			},
			newPV:  nil,
			expErr: nil,
		},
		{
			name:      "success: normal with one instance and add one non csi managed PV",
			instances: []*types.Instance{newFakeInstance("i-001", 5, 2)},
			nodes:     defaultNode,
			expectedMetadata: map[string]enisVolumes{
				"i-001": {ENIs: 5, Volumes: 2},
			},
			newPV: &corev1.PersistentVolume{
				Spec: corev1.PersistentVolumeSpec{
					PersistentVolumeSource: corev1.PersistentVolumeSource{
						CSI: &corev1.CSIPersistentVolumeSource{
							Driver:       "",
							VolumeHandle: "vol-003",
						},
					},
				},
			},
			expErr: nil,
		},
		{
			name:      "success: normal with one instance and add one csi managed PV",
			instances: []*types.Instance{newFakeInstance("i-001", 5, 2)},
			nodes:     defaultNode,
			expectedMetadata: map[string]enisVolumes{
				"i-001": {ENIs: 5, Volumes: 1},
			},
			newPV: &corev1.PersistentVolume{
				Spec: corev1.PersistentVolumeSpec{
					PersistentVolumeSource: corev1.PersistentVolumeSource{
						CSI: &corev1.CSIPersistentVolumeSource{
							Driver:       "ebs.csi.aws.com",
							VolumeHandle: "vol-003",
						},
					},
				},
			},
			expErr: nil,
		},
		{
			name:             "error: describe instances error",
			instances:        []*types.Instance{newFakeInstance("i-001", 5, 2)},
			nodes:            defaultNode,
			expectedMetadata: map[string]enisVolumes{},
			newPV:            nil,
			expErr:           errors.New("failed to describe instances"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()

			pvWatcherStarted := make(chan struct{})
			pvClientset := fake.NewSimpleClientset()
			pvClientset.PrependWatchReactor("*", func(action clienttesting.Action) (handled bool, ret watch.Interface, err error) {
				gvr := action.GetResource()
				ns := action.GetNamespace()
				watch, err := pvClientset.Tracker().Watch(gvr, ns)
				if err != nil {
					return false, nil, err
				}
				close(pvWatcherStarted)
				return true, watch, nil
			})
			pvInformerFactory, pvInformer := pvInformer(pvClientset)
			pvInformerFactory.Start(ctx.Done())
			cache.WaitForCacheSync(ctx.Done())
			<-pvWatcherStarted

			if tc.newPV != nil {
				_, err := pvClientset.CoreV1().PersistentVolumes().Create(t.Context(), tc.newPV, metav1.CreateOptions{})
				if err != nil {
					t.Fatalf("error injecting PV add: %v", err)
				}
				time.Sleep(5e8)
			}

			tc.instances = mockAddPV(tc.newPV, tc.instances)
			mockCtrl := gomock.NewController(t)
			mockCloud := cloud.NewMockCloud(mockCtrl)

			if len(tc.instances) > 1 {
				mockCloud.EXPECT().GetInstances(gomock.Any(), gomock.Any()).Return(
					tc.instances,
					tc.expErr,
				)
			} else {
				mockCloud.EXPECT().GetInstance(gomock.Any(), gomock.Any()).Return(
					tc.instances[0],
					tc.expErr,
				)
			}

			ENIsVolumesMap, err := getMetadata(t.Context(), mockCloud, tc.nodes, pvInformer)
			if err != nil {
				if tc.expErr == nil {
					t.Fatalf("GetMetadata() failed: expected no error, got: %v", err)
				}
				if err.Error() != tc.expErr.Error() {
					t.Fatalf("GetMetadata() failed: expected error %q, got %q", tc.expErr, err)
				}
			} else {
				if tc.expErr != nil {
					t.Fatal("GetMetadata() failed: expected error, got nothing")
				}
				if !reflect.DeepEqual(ENIsVolumesMap, tc.expectedMetadata) {
					t.Fatalf("GetMetadata() failed: expected %v, go: %v", tc.expectedMetadata, ENIsVolumesMap)
				}
			}
			mockCtrl.Finish()
		})
	}
}

func TestPatchLabels(t *testing.T) {
	testCases := []struct {
		name           string
		node           corev1.Node
		ENIsVolumesMap map[string]enisVolumes
		expErr         error
	}{
		{
			name: "success: normal",
			ENIsVolumesMap: map[string]enisVolumes{
				"i-001": {ENIs: 1, Volumes: 1},
			},
			node: corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name:   "i-001",
					Labels: map[string]string{},
				},
				Spec: corev1.NodeSpec{
					ProviderID: "example/i-001",
				},
			},
			expErr: nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			clientset := fake.NewSimpleClientset(&tc.node)
			err := patchNodes(t.Context(), &corev1.NodeList{Items: []corev1.Node{tc.node}}, tc.ENIsVolumesMap, clientset)
			if err != nil {
				if tc.expErr == nil {
					t.Fatalf("PatchNodes() failed: expected no error, got: %v", err)
				}
				if err.Error() != tc.expErr.Error() {
					t.Fatalf("PatchNodes() failed: expected error %q, got %q", tc.expErr, err)
				}
			} else {
				if tc.expErr != nil {
					t.Fatal("PatchNodes() failed: expected error, got nothing")
				}

				node, _ := clientset.CoreV1().Nodes().Get(t.Context(), tc.node.Name, metav1.GetOptions{})
				expectedENIs := strconv.Itoa(tc.ENIsVolumesMap[tc.node.Name].ENIs)
				gotENIs := node.GetLabels()[ENIsLabel]

				expectedVolumes := strconv.Itoa(tc.ENIsVolumesMap[tc.node.Name].Volumes)
				gotVolumes := node.GetLabels()[VolumesLabel]

				if node.GetLabels()[ENIsLabel] != strconv.Itoa(tc.ENIsVolumesMap[tc.node.Name].ENIs) {
					t.Fatalf("PatchNodes() failed: expected %q ENIs, got %q", expectedENIs, gotENIs)
				}
				if node.GetLabels()[VolumesLabel] != strconv.Itoa(tc.ENIsVolumesMap[tc.node.Name].Volumes) {
					t.Fatalf("PatchNodes() failed: expected %q volumes, got %q", expectedVolumes, gotVolumes)
				}
			}
		})
	}
}
