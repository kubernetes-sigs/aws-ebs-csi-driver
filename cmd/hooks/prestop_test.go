package hooks

import (
	"fmt"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/kubernetes-sigs/aws-ebs-csi-driver/pkg/driver"
	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	"k8s.io/apimachinery/pkg/watch"
)

func TestPreStopHook(t *testing.T) {
	testCases := []struct {
		name     string
		nodeName string
		expErr   error
		mockFunc func(string, *driver.MockKubernetesClient, *driver.MockCoreV1Interface, *driver.MockNodeInterface, *driver.MockVolumeAttachmentInterface, *driver.MockStorageV1Interface) error
	}{
		{
			name:     "TestPreStopHook: CSI_NODE_NAME not set",
			nodeName: "",
			expErr:   fmt.Errorf("PreStop: CSI_NODE_NAME missing"),
			mockFunc: func(nodeName string, mockClient *driver.MockKubernetesClient, mockCoreV1 *driver.MockCoreV1Interface, mockNode *driver.MockNodeInterface, mockStorageV1 *driver.MockVolumeAttachmentInterface, mockStorageV1Interface *driver.MockStorageV1Interface) error {
				return nil
			},
		},
		{
			name:     "TestPreStopHook: failed to retrieve node information",
			nodeName: "test-node",
			expErr:   fmt.Errorf("fetchNode: failed to retrieve node information: non-existent node"),
			mockFunc: func(nodeName string, mockClient *driver.MockKubernetesClient, mockCoreV1 *driver.MockCoreV1Interface, mockNode *driver.MockNodeInterface, mockStorageV1 *driver.MockVolumeAttachmentInterface, mockStorageV1Interface *driver.MockStorageV1Interface) error {
				mockClient.EXPECT().CoreV1().Return(mockCoreV1).Times(1)
				mockCoreV1.EXPECT().Nodes().Return(mockNode).Times(1)
				mockNode.EXPECT().Get(gomock.Any(), gomock.Eq(nodeName), gomock.Any()).Return(nil, fmt.Errorf("non-existent node")).Times(1)

				return nil
			},
		},
		{
			name:     "TestPreStopHook: node is not being drained, skipping VolumeAttachments check - missing TaintNodeUnschedulable",
			nodeName: "test-node",
			expErr:   nil,
			mockFunc: func(nodeName string, mockClient *driver.MockKubernetesClient, mockCoreV1 *driver.MockCoreV1Interface, mockNode *driver.MockNodeInterface, mockStorageV1 *driver.MockVolumeAttachmentInterface, mockStorageV1Interface *driver.MockStorageV1Interface) error {
				mockNodeObj := &v1.Node{
					Spec: v1.NodeSpec{
						Taints: []v1.Taint{},
					},
				}

				mockClient.EXPECT().CoreV1().Return(mockCoreV1).Times(1)
				mockCoreV1.EXPECT().Nodes().Return(mockNode).Times(1)
				mockNode.EXPECT().Get(gomock.Any(), gomock.Eq(nodeName), gomock.Any()).Return(mockNodeObj, nil).Times(1)

				return nil
			},
		},
		{
			name:     "TestPreStopHook: node is not being drained, skipping VolumeAttachments check - missing TaintEffectNoSchedule",
			nodeName: "test-node",
			expErr:   nil,
			mockFunc: func(nodeName string, mockClient *driver.MockKubernetesClient, mockCoreV1 *driver.MockCoreV1Interface, mockNode *driver.MockNodeInterface, mockStorageV1 *driver.MockVolumeAttachmentInterface, mockStorageV1Interface *driver.MockStorageV1Interface) error {
				mockNodeObj := &v1.Node{
					Spec: v1.NodeSpec{
						Taints: []v1.Taint{
							{
								Key:    v1.TaintNodeUnschedulable,
								Effect: "",
							},
						},
					},
				}

				mockClient.EXPECT().CoreV1().Return(mockCoreV1).Times(1)
				mockCoreV1.EXPECT().Nodes().Return(mockNode).Times(1)
				mockNode.EXPECT().Get(gomock.Any(), gomock.Eq(nodeName), gomock.Any()).Return(mockNodeObj, nil).Times(1)

				return nil
			},
		},
		{
			name:     "TestPreStopHook: node is being drained, no volume attachments remain",
			nodeName: "test-node",
			expErr:   nil,
			mockFunc: func(nodeName string, mockClient *driver.MockKubernetesClient, mockCoreV1 *driver.MockCoreV1Interface, mockNode *driver.MockNodeInterface, mockVolumeAttachments *driver.MockVolumeAttachmentInterface, mockStorageV1Interface *driver.MockStorageV1Interface) error {

				fakeNode := &v1.Node{
					Spec: v1.NodeSpec{
						Taints: []v1.Taint{
							{
								Key:    v1.TaintNodeUnschedulable,
								Effect: v1.TaintEffectNoSchedule,
							},
						},
					},
				}

				emptyVolumeAttachments := &storagev1.VolumeAttachmentList{Items: []storagev1.VolumeAttachment{}}

				mockClient.EXPECT().CoreV1().Return(mockCoreV1).AnyTimes()
				mockClient.EXPECT().StorageV1().Return(mockStorageV1Interface).AnyTimes()

				mockCoreV1.EXPECT().Nodes().Return(mockNode).AnyTimes()
				mockNode.EXPECT().Get(gomock.Any(), gomock.Eq(nodeName), gomock.Any()).Return(fakeNode, nil).AnyTimes()

				mockStorageV1Interface.EXPECT().VolumeAttachments().Return(mockVolumeAttachments).AnyTimes()
				mockVolumeAttachments.EXPECT().List(gomock.Any(), gomock.Any()).Return(emptyVolumeAttachments, nil).AnyTimes()
				mockVolumeAttachments.EXPECT().Watch(gomock.Any(), gomock.Any()).Return(watch.NewFake(), nil).AnyTimes()

				return nil
			},
		},
		{
			name:     "TestPreStopHook: node is being drained, no volume attachments associated with node",
			nodeName: "test-node",
			expErr:   nil,
			mockFunc: func(nodeName string, mockClient *driver.MockKubernetesClient, mockCoreV1 *driver.MockCoreV1Interface, mockNode *driver.MockNodeInterface, mockVolumeAttachments *driver.MockVolumeAttachmentInterface, mockStorageV1Interface *driver.MockStorageV1Interface) error {

				fakeNode := &v1.Node{
					Spec: v1.NodeSpec{
						Taints: []v1.Taint{
							{
								Key:    v1.TaintNodeUnschedulable,
								Effect: v1.TaintEffectNoSchedule,
							},
						},
					},
				}

				fakeVolumeAttachments := &storagev1.VolumeAttachmentList{
					Items: []storagev1.VolumeAttachment{
						{
							Spec: storagev1.VolumeAttachmentSpec{
								NodeName: "test-node-2",
							},
						},
					},
				}

				mockClient.EXPECT().CoreV1().Return(mockCoreV1).AnyTimes()
				mockClient.EXPECT().StorageV1().Return(mockStorageV1Interface).AnyTimes()

				mockCoreV1.EXPECT().Nodes().Return(mockNode).AnyTimes()
				mockNode.EXPECT().Get(gomock.Any(), gomock.Eq(nodeName), gomock.Any()).Return(fakeNode, nil).AnyTimes()

				mockStorageV1Interface.EXPECT().VolumeAttachments().Return(mockVolumeAttachments).AnyTimes()
				mockVolumeAttachments.EXPECT().List(gomock.Any(), gomock.Any()).Return(fakeVolumeAttachments, nil).AnyTimes()
				mockVolumeAttachments.EXPECT().Watch(gomock.Any(), gomock.Any()).Return(watch.NewFake(), nil).AnyTimes()

				return nil
			},
		},
		{
			name:     "TestPreStopHook: Node is drained before timeout",
			nodeName: "test-node",
			expErr:   nil,
			mockFunc: func(nodeName string, mockClient *driver.MockKubernetesClient, mockCoreV1 *driver.MockCoreV1Interface, mockNode *driver.MockNodeInterface, mockVolumeAttachments *driver.MockVolumeAttachmentInterface, mockStorageV1Interface *driver.MockStorageV1Interface) error {

				fakeNode := &v1.Node{
					Spec: v1.NodeSpec{
						Taints: []v1.Taint{
							{
								Key:    v1.TaintNodeUnschedulable,
								Effect: v1.TaintEffectNoSchedule,
							},
						},
					},
				}

				fakeVolumeAttachments := &storagev1.VolumeAttachmentList{
					Items: []storagev1.VolumeAttachment{
						{
							Spec: storagev1.VolumeAttachmentSpec{
								NodeName: "test-node",
							},
						},
					},
				}

				fakeWatcher := watch.NewFake()
				deleteSignal := make(chan bool, 1)

				mockClient.EXPECT().CoreV1().Return(mockCoreV1).AnyTimes()
				mockClient.EXPECT().StorageV1().Return(mockStorageV1Interface).AnyTimes()

				mockCoreV1.EXPECT().Nodes().Return(mockNode).AnyTimes()
				mockNode.EXPECT().Get(gomock.Any(), gomock.Eq(nodeName), gomock.Any()).Return(fakeNode, nil).AnyTimes()

				mockStorageV1Interface.EXPECT().VolumeAttachments().Return(mockVolumeAttachments).AnyTimes()
				gomock.InOrder(
					mockVolumeAttachments.EXPECT().List(gomock.Any(), gomock.Any()).Return(fakeVolumeAttachments, nil).AnyTimes(),
					mockVolumeAttachments.EXPECT().Watch(gomock.Any(), gomock.Any()).DoAndReturn(func(signal, watchSignal interface{}) (watch.Interface, error) {
						deleteSignal <- true
						return fakeWatcher, nil
					}).AnyTimes(),
					mockVolumeAttachments.EXPECT().List(gomock.Any(), gomock.Any()).Return(&storagev1.VolumeAttachmentList{Items: []storagev1.VolumeAttachment{}}, nil).AnyTimes(),
				)

				go func() {
					<-deleteSignal
					fakeWatcher.Delete(&storagev1.VolumeAttachment{
						Spec: storagev1.VolumeAttachmentSpec{
							NodeName: "test-node",
						},
					})
				}()
				return nil
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockCtl := gomock.NewController(t)
			defer mockCtl.Finish()

			mockClient := driver.NewMockKubernetesClient(mockCtl)
			mockCoreV1 := driver.NewMockCoreV1Interface(mockCtl)
			mockStorageV1 := driver.NewMockStorageV1Interface(mockCtl)
			mockNode := driver.NewMockNodeInterface(mockCtl)
			mockVolumeAttachments := driver.NewMockVolumeAttachmentInterface(mockCtl)

			if tc.mockFunc != nil {
				err := tc.mockFunc(tc.nodeName, mockClient, mockCoreV1, mockNode, mockVolumeAttachments, mockStorageV1)
				if err != nil {
					t.Fatalf("TestPreStopHook: mockFunc returned error: %v", err)
				}
			}

			if tc.nodeName != "" {
				t.Setenv("CSI_NODE_NAME", tc.nodeName)
			}

			err := PreStop(mockClient)

			if tc.expErr != nil {
				assert.Error(t, err)
				assert.Equal(t, tc.expErr.Error(), err.Error())
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
