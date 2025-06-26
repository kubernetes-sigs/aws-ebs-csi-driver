package patch

import (
	"context"
	"reflect"
	"strconv"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/golang/mock/gomock"
	"github.com/kubernetes-sigs/aws-ebs-csi-driver/pkg/cloud"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func newFakeInstance(instanceID string, numENIs int, numVolumes int) types.Instance {
	return types.Instance{
		InstanceId:          &instanceID,
		BlockDeviceMappings: make([]types.InstanceBlockDeviceMapping, numVolumes),
		NetworkInterfaces:   make([]types.InstanceNetworkInterface, numENIs),
	}
}

func TestGetMetadata(t *testing.T) {
	testCases := []struct {
		name             string
		instances        []types.Instance
		expectedMetadata map[string]ENIsVolumes
		expErr           error
	}{
		{
			name:      "success: normal",
			instances: []types.Instance{newFakeInstance("i-001", 1, 1), newFakeInstance("i-002", 2, 0)},
			expectedMetadata: map[string]ENIsVolumes{
				"i-001": {ENIs: 1, Volumes: 1},
				"i-002": {ENIs: 2, Volumes: 0},
			},
			expErr: nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockCtrl := gomock.NewController(t)
			mockEC2 := cloud.NewMockEC2API(mockCtrl)

			mockEC2.EXPECT().DescribeInstances(gomock.Any(), gomock.Any()).Return(
				&ec2.DescribeInstancesOutput{
					Reservations: []types.Reservation{
						{
							Instances: tc.instances,
						},
					},
				},
				tc.expErr,
			)

			ENIsVolumesMap, err := GetMetadata(mockEC2, "us-west-2")
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
		instance       types.Instance
		ENIsVolumesMap map[string]ENIsVolumes
		expErr         error
	}{
		{
			name:     "success: normal",
			instance: newFakeInstance("i-001", 1, 1),
			ENIsVolumesMap: map[string]ENIsVolumes{
				"i-001": {ENIs: 1, Volumes: 1},
			},
			expErr: nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			node := corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name:   *tc.instance.InstanceId,
					Labels: map[string]string{},
				},
			}
			nodes := corev1.NodeList{Items: []corev1.Node{node}}
			clientset := fake.NewSimpleClientset(&node)
			err := PatchNodes(&nodes, tc.ENIsVolumesMap, clientset)
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
				node, _ := clientset.CoreV1().Nodes().Get(context.TODO(), *tc.instance.InstanceId, metav1.GetOptions{})
				expectedENIs := strconv.Itoa(tc.ENIsVolumesMap[*tc.instance.InstanceId].ENIs)
				gotENIs := node.GetLabels()["num-ENIs"]

				expectedVolumes := strconv.Itoa(tc.ENIsVolumesMap[*tc.instance.InstanceId].Volumes)
				gotVolumes := node.GetLabels()["num-volumes"]
				if node.GetLabels()["num-ENIs"] != strconv.Itoa(tc.ENIsVolumesMap[*tc.instance.InstanceId].ENIs) {
					t.Fatalf("PatchNodes() failed: expected %q ENIs, got %q", expectedENIs, gotENIs)
				}
				if node.GetLabels()["num-volumes"] != strconv.Itoa(tc.ENIsVolumesMap[*tc.instance.InstanceId].Volumes) {
					t.Fatalf("PatchNodes() failed: expected %q volumes, got %q", expectedVolumes, gotVolumes)
				}
			}
		})
	}
}
