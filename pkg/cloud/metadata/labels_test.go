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
	"maps"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/golang/mock/gomock"
	"github.com/kubernetes-sigs/aws-ebs-csi-driver/pkg/cloud"
	"github.com/kubernetes-sigs/aws-ebs-csi-driver/pkg/util"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes/fake"
	clienttesting "k8s.io/client-go/testing"
	"k8s.io/client-go/tools/cache"
)

func init() {
	// Ensure variables are initialized
	// TODO: Figure out a cleaner way to do this in tests
	initVariables()
}

func TestGetMetadata(t *testing.T) {
	tests := []struct {
		name      string
		nodes     []corev1.Node
		pvs       []corev1.PersistentVolume
		instances []*types.Instance
		cloudErr  error
		want      map[string]enisVolumes
		wantErr   bool
	}{
		{
			name: "single node with volumes and ENIs",
			nodes: []corev1.Node{
				makeNode("i-001", "aws:///us-west-2a/i-001"),
			},
			instances: []*types.Instance{
				makeInstance("i-001", 2, []string{"vol-001", "vol-002"}),
			},
			want: map[string]enisVolumes{
				"i-001": {ENIs: 2, Volumes: 1, VolumeAttachmentLimit: 27, VolumeAttachmentType: "shared"},
			},
		},
		{
			name: "multiple nodes",
			nodes: []corev1.Node{
				makeNode("i-001", "aws:///us-west-2a/i-001"),
				makeNode("i-002", "aws:///us-west-2b/i-002"),
			},
			instances: []*types.Instance{
				makeInstance("i-001", 1, []string{"vol-001"}),
				makeInstance("i-002", 3, []string{"vol-002", "vol-003", "vol-004"}),
			},
			want: map[string]enisVolumes{
				"i-001": {ENIs: 1, Volumes: 0, VolumeAttachmentLimit: 27, VolumeAttachmentType: "shared"},
				"i-002": {ENIs: 3, Volumes: 2, VolumeAttachmentLimit: 27, VolumeAttachmentType: "shared"},
			},
		},
		{
			name: "exclude CSI managed volumes",
			nodes: []corev1.Node{
				makeNode("i-001", "aws:///us-west-2a/i-001"),
			},
			pvs: []corev1.PersistentVolume{
				makeCSIPV("pv-001", "vol-001"),
			},
			instances: []*types.Instance{
				makeInstance("i-001", 1, []string{"vol-001", "vol-002"}),
			},
			want: map[string]enisVolumes{
				"i-001": {ENIs: 1, Volumes: 0, VolumeAttachmentLimit: 27, VolumeAttachmentType: "shared"},
			},
		},
		{
			name: "exclude migrated volumes",
			nodes: []corev1.Node{
				makeNode("i-001", "aws:///us-west-2a/i-001"),
			},
			pvs: []corev1.PersistentVolume{
				makeMigratedPV("pv-001", "vol-001"),
			},
			instances: []*types.Instance{
				makeInstance("i-001", 1, []string{"vol-001", "vol-002"}),
			},
			want: map[string]enisVolumes{
				"i-001": {ENIs: 1, Volumes: 0, VolumeAttachmentLimit: 27, VolumeAttachmentType: "shared"},
			},
		},
		{
			name: "cloud error",
			nodes: []corev1.Node{
				makeNode("i-001", "aws:///us-west-2a/i-001"),
			},
			cloudErr: errors.New("EC2 API error"),
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockCloud := cloud.NewMockCloud(ctrl)
			nodeList := &corev1.NodeList{Items: tt.nodes}
			expectedNodeIDs := make([]string, 0, len(tt.nodes))
			for _, node := range tt.nodes {
				if id, err := parseProviderID(&node); err == nil && strings.HasPrefix(id, "i-") {
					expectedNodeIDs = append(expectedNodeIDs, id)
				}
			}
			if len(expectedNodeIDs) > 0 || tt.cloudErr != nil {
				mockCloud.EXPECT().GetInstancesPatching(ctx, expectedNodeIDs).
					Return(tt.instances, tt.cloudErr).Times(1)
			}
			if !tt.wantErr {
				mockCloud.EXPECT().GetInstanceTypesInfo(ctx, gomock.Eq([]string{"c5.large"})).
					Return(map[string]cloud.InstanceTypeInfo{
						"c5.large": {MaxAttachments: 27, AttachmentType: "shared"},
					}, nil).Times(1)
			}

			pvInformer := setupPVInformer(t, tt.pvs)

			got, err := getMetadata(ctx, mockCloud, nodeList, pvInformer)

			if (err != nil) != tt.wantErr {
				t.Errorf("getMetadata() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && !maps.Equal(got, tt.want) {
				t.Errorf("getMetadata() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestPatchSingleNode(t *testing.T) {
	tests := []struct {
		name        string
		node        corev1.Node
		metadata    map[string]enisVolumes
		wantENIs    string
		wantVolumes string
		wantLimit   string
		wantType    string
		wantErr     bool
	}{
		{
			name: "patch node successfully",
			node: makeNode("i-001", "aws:///us-west-2a/i-001"),
			metadata: map[string]enisVolumes{
				"i-001": {ENIs: 3, Volumes: 5, VolumeAttachmentLimit: 27, VolumeAttachmentType: "shared"},
			},
			wantENIs:    "3",
			wantVolumes: "5",
			wantLimit:   "27",
			wantType:    "shared",
		},
		{
			name: "invalid provider ID",
			node: makeNode("i-001", "invalid"),
			metadata: map[string]enisVolumes{
				"i-001": {ENIs: 1, Volumes: 1, VolumeAttachmentLimit: 27, VolumeAttachmentType: "shared"},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			clientset := fake.NewClientset(&tt.node)

			err := patchSingleNode(ctx, tt.node, tt.metadata, clientset)

			if (err != nil) != tt.wantErr {
				t.Errorf("patchSingleNode() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				node, _ := clientset.CoreV1().Nodes().Get(ctx, tt.node.Name, metav1.GetOptions{})
				if got := node.Labels[ENIsLabel]; got != tt.wantENIs {
					t.Errorf("ENIs label = %v, want %v", got, tt.wantENIs)
				}
				if got := node.Labels[VolumesLabel]; got != tt.wantVolumes {
					t.Errorf("Volumes label = %v, want %v", got, tt.wantVolumes)
				}
				if got := node.Labels[VolumeAttachmentLimitLabel]; got != tt.wantLimit {
					t.Errorf("VolumeAttachmentLimit label = %v, want %v", got, tt.wantLimit)
				}
				if got := node.Labels[VolumeAttachmentTypeLabel]; got != tt.wantType {
					t.Errorf("VolumeAttachmentType label = %v, want %v", got, tt.wantType)
				}
			}
		})
	}
}

func TestPatchNodes(t *testing.T) {
	tests := []struct {
		name       string
		nodes      []corev1.Node
		metadata   map[string]enisVolumes
		patchFails int
		wantErr    bool
	}{
		{
			name: "patch all nodes successfully",
			nodes: []corev1.Node{
				makeNode("i-001", "aws:///us-west-2a/i-001"),
				makeNode("i-002", "aws:///us-west-2b/i-002"),
			},
			metadata: map[string]enisVolumes{
				"i-001": {ENIs: 1, Volumes: 2},
				"i-002": {ENIs: 3, Volumes: 4},
			},
			patchFails: 5,
		},
		{
			name: "fail when too many errors",
			nodes: []corev1.Node{
				makeNode("i-001", "invalid"),
				makeNode("i-002", "invalid"),
				makeNode("i-003", "invalid"),
				makeNode("i-004", "invalid"),
				makeNode("i-005", "invalid"),
			},
			metadata:   map[string]enisVolumes{},
			patchFails: 5,
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			nodeList := &corev1.NodeList{Items: tt.nodes}
			clientset := fake.NewClientset(nodeList)

			err := patchNodes(ctx, nodeList, tt.metadata, clientset, tt.patchFails)

			if (err != nil) != tt.wantErr {
				t.Errorf("patchNodes() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestVolumeIDIndexFunc(t *testing.T) {
	tests := []struct {
		name string
		pv   any
		want []string
	}{
		{
			name: "CSI volume",
			pv:   makeCSIPVPtr("pv-001", "vol-001"),
			want: []string{"vol-001"},
		},
		{
			name: "migrated volume",
			pv:   makeMigratedPVPtr("pv-001", "vol-001"),
			want: []string{"vol-001"},
		},
		{
			name: "non-EBS volume",
			pv: &corev1.PersistentVolume{
				Spec: corev1.PersistentVolumeSpec{},
			},
			want: []string{},
		},
		{
			name: "invalid object",
			pv:   "not a PV",
			want: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := volumeIDIndexFunc(tt.pv)
			if err != nil {
				t.Errorf("volumeIDIndexFunc() error = %v", err)
				return
			}
			if !slices.Equal(got, tt.want) {
				t.Errorf("volumeIDIndexFunc() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetNonCSIManagedVolumes(t *testing.T) {
	tests := []struct {
		name    string
		pvs     []corev1.PersistentVolume
		volumes []types.InstanceBlockDeviceMapping
		want    int
	}{
		{
			name: "all non-CSI volumes",
			volumes: []types.InstanceBlockDeviceMapping{
				makeBlockDevice("vol-001"),
				makeBlockDevice("vol-002"),
			},
			want: 2,
		},
		{
			name: "one CSI managed volume",
			pvs: []corev1.PersistentVolume{
				makeCSIPV("pv-001", "vol-001"),
			},
			volumes: []types.InstanceBlockDeviceMapping{
				makeBlockDevice("vol-001"),
				makeBlockDevice("vol-002"),
			},
			want: 1,
		},
		{
			name: "all CSI managed volumes",
			pvs: []corev1.PersistentVolume{
				makeCSIPV("pv-001", "vol-001"),
				makeCSIPV("pv-002", "vol-002"),
			},
			volumes: []types.InstanceBlockDeviceMapping{
				makeBlockDevice("vol-001"),
				makeBlockDevice("vol-002"),
			},
			want: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pvInformer := setupPVInformer(t, tt.pvs)
			got := getNonCSIManagedVolumes(pvInformer, tt.volumes)
			if got != tt.want {
				t.Errorf("getNonCSIManagedVolumes() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestUpdateMetadataEC2(t *testing.T) {
	tests := []struct {
		name     string
		nodes    []corev1.Node
		metadata map[string]enisVolumes
		cloudErr error
		wantErr  bool
	}{
		{
			name: "update single node",
			nodes: []corev1.Node{
				makeNode("i-001", "aws:///us-west-2a/i-001"),
			},
			metadata: map[string]enisVolumes{
				"i-001": {ENIs: 2, Volumes: 3},
			},
		},
		{
			name: "cloud error",
			nodes: []corev1.Node{
				makeNode("i-001", "aws:///us-west-2a/i-001"),
			},
			cloudErr: errors.New("EC2 error"),
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockCloud := cloud.NewMockCloud(ctrl)
			nodeList := &corev1.NodeList{Items: tt.nodes}
			clientset := fake.NewClientset(nodeList)
			pvInformer := setupPVInformer(t, nil)

			if tt.cloudErr != nil {
				mockCloud.EXPECT().GetInstancesPatching(ctx, []string{"i-001"}).Return(nil, tt.cloudErr)
			} else {
				instances := []*types.Instance{makeInstance("i-001", tt.metadata["i-001"].ENIs, []string{"vol-001", "vol-002", "vol-003", "vol-004"})}
				mockCloud.EXPECT().GetInstancesPatching(ctx, []string{"i-001"}).Return(instances, nil)
				mockCloud.EXPECT().GetInstanceTypesInfo(ctx, gomock.Eq([]string{"c5.large"})).
					Return(map[string]cloud.InstanceTypeInfo{
						"c5.large": {MaxAttachments: 27, AttachmentType: "shared"},
					}, nil)
			}

			err := updateMetadataEC2(ctx, clientset, mockCloud, nodeList, pvInformer)

			if (err != nil) != tt.wantErr {
				t.Errorf("updateMetadataEC2() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				node, _ := clientset.CoreV1().Nodes().Get(ctx, tt.nodes[0].Name, metav1.GetOptions{})
				if got := node.Labels[ENIsLabel]; got != strconv.Itoa(tt.metadata["i-001"].ENIs) {
					t.Errorf("ENIs label = %v, want %v", got, tt.metadata["i-001"].ENIs)
				}
				if got := node.Labels[VolumesLabel]; got != strconv.Itoa(tt.metadata["i-001"].Volumes) {
					t.Errorf("Volumes label = %v, want %v", got, tt.metadata["i-001"].Volumes)
				}
			}
		})
	}
}

func TestPatchNewNodes(t *testing.T) {
	tests := []struct {
		name     string
		node     corev1.Node
		metadata map[string]enisVolumes
	}{
		{
			name: "new node added",
			node: makeNode("i-001", "aws:///us-west-2a/i-001"),
			metadata: map[string]enisVolumes{
				"i-001": {ENIs: 2, Volumes: 1},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockCloud := cloud.NewMockCloud(ctrl)
			clientset := fake.NewClientset()
			factory := informers.NewSharedInformerFactory(clientset, 0)
			nodesInformer := factory.Core().V1().Nodes().Informer()
			pvInformer := setupPVInformer(t, nil)

			instances := []*types.Instance{makeInstance("i-001", tt.metadata["i-001"].ENIs, []string{"vol-001", "vol-002"})}
			mockCloud.EXPECT().GetInstancesPatching(ctx, []string{"i-001"}).Return(instances, nil).MinTimes(1)
			mockCloud.EXPECT().GetInstanceTypesInfo(ctx, gomock.Eq([]string{"c5.large"})).
				Return(map[string]cloud.InstanceTypeInfo{
					"c5.large": {MaxAttachments: 27, AttachmentType: "shared"},
				}, nil).MinTimes(1)

			patched := make(chan struct{})
			clientset.PrependReactor("patch", "nodes", func(action clienttesting.Action) (bool, runtime.Object, error) {
				select {
				case patched <- struct{}{}:
				default:
				}
				return false, nil, nil
			})

			err := patchNewNodes(ctx, clientset, mockCloud, nodesInformer, pvInformer)
			if err != nil {
				t.Fatalf("patchNewNodes() error = %v", err)
			}

			factory.Start(ctx.Done())
			cache.WaitForCacheSync(ctx.Done(), nodesInformer.HasSynced)

			_, err = clientset.CoreV1().Nodes().Create(ctx, &tt.node, metav1.CreateOptions{})
			if err != nil {
				t.Fatalf("Failed to create node: %v", err)
			}

			select {
			case <-patched:
			case <-ctx.Done():
				t.Fatal("Timeout waiting for node to be patched")
			}

			node, _ := clientset.CoreV1().Nodes().Get(ctx, tt.node.Name, metav1.GetOptions{})
			if got := node.Labels[ENIsLabel]; got != strconv.Itoa(tt.metadata["i-001"].ENIs) {
				t.Errorf("ENIs label = %v, want %v", got, tt.metadata["i-001"].ENIs)
			}
			if got := node.Labels[VolumesLabel]; got != strconv.Itoa(tt.metadata["i-001"].Volumes) {
				t.Errorf("Volumes label = %v, want %v", got, tt.metadata["i-001"].Volumes)
			}
		})
	}
}

func makeNode(name, providerID string) corev1.Node {
	return corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			Labels: map[string]string{
				corev1.LabelInstanceTypeStable: "c5.large",
			},
		},
		Spec: corev1.NodeSpec{
			ProviderID: providerID,
		},
	}
}

func makeInstance(id string, numENIs int, volumeIDs []string) *types.Instance {
	blockDevices := make([]types.InstanceBlockDeviceMapping, len(volumeIDs))
	for i, volID := range volumeIDs {
		volIDCopy := volID
		blockDevices[i] = types.InstanceBlockDeviceMapping{
			Ebs: &types.EbsInstanceBlockDevice{
				VolumeId: &volIDCopy,
			},
		}
	}

	return &types.Instance{
		InstanceId:          &id,
		InstanceType:        types.InstanceTypeC5Large,
		NetworkInterfaces:   make([]types.InstanceNetworkInterface, numENIs),
		BlockDeviceMappings: blockDevices,
	}
}

func makeCSIPV(name, volumeHandle string) corev1.PersistentVolume {
	return corev1.PersistentVolume{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec: corev1.PersistentVolumeSpec{
			PersistentVolumeSource: corev1.PersistentVolumeSource{
				CSI: &corev1.CSIPersistentVolumeSource{
					Driver:       util.GetDriverName(),
					VolumeHandle: volumeHandle,
				},
			},
		},
	}
}

func makeCSIPVPtr(name, volumeHandle string) *corev1.PersistentVolume {
	pv := makeCSIPV(name, volumeHandle)
	return &pv
}

func makeMigratedPV(name, volumeID string) corev1.PersistentVolume {
	return corev1.PersistentVolume{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec: corev1.PersistentVolumeSpec{
			PersistentVolumeSource: corev1.PersistentVolumeSource{
				AWSElasticBlockStore: &corev1.AWSElasticBlockStoreVolumeSource{
					VolumeID: volumeID,
				},
			},
		},
	}
}

func makeMigratedPVPtr(name, volumeID string) *corev1.PersistentVolume {
	pv := makeMigratedPV(name, volumeID)
	return &pv
}

func makeBlockDevice(volumeID string) types.InstanceBlockDeviceMapping {
	volIDCopy := volumeID
	return types.InstanceBlockDeviceMapping{
		Ebs: &types.EbsInstanceBlockDevice{
			VolumeId: &volIDCopy,
		},
	}
}

func setupPVInformer(t *testing.T, pvs []corev1.PersistentVolume) cache.SharedIndexInformer {
	t.Helper()
	clientset := fake.NewClientset()
	for i := range pvs {
		_, err := clientset.CoreV1().PersistentVolumes().Create(context.Background(), &pvs[i], metav1.CreateOptions{})
		if err != nil {
			t.Fatalf("Failed to create PV: %v", err)
		}
	}
	factory := informers.NewSharedInformerFactory(clientset, 0)
	pvInformer := factory.Core().V1().PersistentVolumes().Informer()
	if err := pvInformer.AddIndexers(cache.Indexers{"volumeID": volumeIDIndexFunc}); err != nil {
		t.Fatalf("Failed to add indexer: %v", err)
	}
	stopCh := make(chan struct{})
	t.Cleanup(func() { close(stopCh) })
	factory.Start(stopCh)
	cache.WaitForCacheSync(stopCh, pvInformer.HasSynced)
	return pvInformer
}

func TestGetVolumeAttachmentInfo(t *testing.T) {
	tests := []struct {
		name              string
		instanceType      string
		instanceTypesInfo map[string]cloud.InstanceTypeInfo
		wantLimit         int
		wantType          string
	}{
		{
			name:         "uses API data when available",
			instanceType: "c5.large",
			instanceTypesInfo: map[string]cloud.InstanceTypeInfo{
				"c5.large": {MaxAttachments: 27, AttachmentType: "shared"},
			},
			wantLimit: 27,
			wantType:  "shared",
		},
		{
			name:         "applies dedicated override from API data",
			instanceType: "i8ge.metal-24xl",
			instanceTypesInfo: map[string]cloud.InstanceTypeInfo{
				"i8ge.metal-24xl": {MaxAttachments: 128, AttachmentType: "shared"},
			},
			wantLimit: 128,
			wantType:  "dedicated",
		},
		{
			name:              "falls back to static table when API data missing",
			instanceType:      "c5.large",
			instanceTypesInfo: map[string]cloud.InstanceTypeInfo{},
			wantLimit:         27,
			wantType:          "shared",
		},
		{
			name:              "falls back to static table when API data nil",
			instanceType:      "c5.large",
			instanceTypesInfo: nil,
			wantLimit:         27,
			wantType:          "shared",
		},
		{
			name:         "falls back when API returns zero attachments",
			instanceType: "c5.large",
			instanceTypesInfo: map[string]cloud.InstanceTypeInfo{
				"c5.large": {MaxAttachments: 0, AttachmentType: "shared"},
			},
			wantLimit: 27,
			wantType:  "shared",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotLimit, gotType := getVolumeAttachmentInfo(tt.instanceType, tt.instanceTypesInfo)
			if gotLimit != tt.wantLimit {
				t.Errorf("getVolumeAttachmentInfo() limit = %v, want %v", gotLimit, tt.wantLimit)
			}
			if gotType != tt.wantType {
				t.Errorf("getVolumeAttachmentInfo() type = %v, want %v", gotType, tt.wantType)
			}
		})
	}
}

func TestGetMetadataWithDITFallback(t *testing.T) {
	initVariables()

	ctx := context.Background()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockCloud := cloud.NewMockCloud(ctrl)
	nodes := []corev1.Node{makeNode("i-001", "aws:///us-west-2a/i-001")}
	nodeList := &corev1.NodeList{Items: nodes}

	instances := []*types.Instance{makeInstance("i-001", 2, []string{"vol-001", "vol-002"})}
	mockCloud.EXPECT().GetInstancesPatching(ctx, []string{"i-001"}).Return(instances, nil)
	// Simulate DIT failure - should fall back to static table
	mockCloud.EXPECT().GetInstanceTypesInfo(ctx, gomock.Eq([]string{"c5.large"})).Return(nil, errors.New("DIT API error"))

	pvInformer := setupPVInformer(t, nil)
	got, err := getMetadata(ctx, mockCloud, nodeList, pvInformer)

	if err != nil {
		t.Fatalf("getMetadata() unexpected error: %v", err)
	}

	want := enisVolumes{ENIs: 2, Volumes: 1, VolumeAttachmentLimit: 27, VolumeAttachmentType: "shared"}
	if got["i-001"] != want {
		t.Errorf("getMetadata() = %v, want %v", got["i-001"], want)
	}
}
