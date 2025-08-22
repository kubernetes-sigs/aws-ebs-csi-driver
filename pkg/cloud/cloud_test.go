/*
Copyright 2019 The Kubernetes Authors.

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

package cloud

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go-v2/service/sagemaker"
	smtypes "github.com/aws/aws-sdk-go-v2/service/sagemaker/types"
	"github.com/aws/smithy-go"
	"github.com/aws/smithy-go/ptr"
	"github.com/golang/mock/gomock"
	"github.com/kubernetes-sigs/aws-ebs-csi-driver/pkg/batcher"
	dm "github.com/kubernetes-sigs/aws-ebs-csi-driver/pkg/cloud/devicemanager"
	"github.com/kubernetes-sigs/aws-ebs-csi-driver/pkg/expiringcache"
	"github.com/kubernetes-sigs/aws-ebs-csi-driver/pkg/util"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/util/wait"
)

const (
	defaultZone     = "test-az"
	expZone         = "us-west-2b"
	expZoneID       = "use2-az2"
	defaultVolumeID = "vol-test-1234"
	defaultNodeID   = "node-1234"
	defaultPath     = "/dev/xvdaa"

	defaultCreateDiskDeadline = time.Second * 5

	testInitializationSleep = 200 * time.Microsecond
)

func generateVolumes(volIDCount, volTagCount int) []types.Volume {
	volumes := make([]types.Volume, 0, volIDCount+volTagCount)

	for i := range volIDCount {
		volumeID := fmt.Sprintf("vol-%d", i)
		volumes = append(volumes, types.Volume{VolumeId: aws.String(volumeID)})
	}

	for i := range volTagCount {
		volumeName := fmt.Sprintf("vol-name-%d", i)
		volumes = append(volumes, types.Volume{Tags: []types.Tag{{Key: aws.String(VolumeNameTagKey), Value: aws.String(volumeName)}}})
	}

	return volumes
}

func extractVolumeIdentifiers(volumes []types.Volume) (volumeIDs []string, volumeNames []string) {
	for _, volume := range volumes {
		if volume.VolumeId != nil {
			volumeIDs = append(volumeIDs, *volume.VolumeId)
		}
		for _, tag := range volume.Tags {
			if tag.Key != nil && *tag.Key == VolumeNameTagKey && tag.Value != nil {
				volumeNames = append(volumeNames, *tag.Value)
			}
		}
	}
	return volumeIDs, volumeNames
}

func TestNewCloud(t *testing.T) {
	testCases := []struct {
		name              string
		region            string
		awsSdkDebugLog    bool
		userAgentExtra    string
		batchingEnabled   bool
		deprecatedMetrics bool
	}{
		{
			name:            "success: with awsSdkDebugLog, userAgentExtra, and batchingEnabled",
			region:          "us-east-1",
			awsSdkDebugLog:  true,
			userAgentExtra:  "example_user_agent_extra",
			batchingEnabled: true,
		},
		{
			name:           "success: with only awsSdkDebugLog, and userAgentExtra",
			region:         "us-east-1",
			awsSdkDebugLog: true,
			userAgentExtra: "example_user_agent_extra",
		},
		{
			name:   "success: with only region",
			region: "us-east-1",
		},
	}
	for _, tc := range testCases {
		ec2Cloud := NewCloud(tc.region, tc.awsSdkDebugLog, tc.userAgentExtra, tc.batchingEnabled, tc.deprecatedMetrics)
		ec2CloudAscloud, ok := ec2Cloud.(*cloud)
		if !ok {
			t.Fatalf("could not assert object ec2Cloud as cloud type, %v", ec2Cloud)
		}
		assert.Equal(t, ec2CloudAscloud.region, tc.region)
		if tc.batchingEnabled {
			assert.NotNil(t, ec2CloudAscloud.bm)
		} else {
			assert.Nil(t, ec2CloudAscloud.bm)
		}
	}
}

func TestBatchDescribeVolumes(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		name     string
		volumes  []types.Volume
		expErr   error
		mockFunc func(mockEC2 *MockEC2API, expErr error, volumes []types.Volume)
	}{
		{
			name:    "success: volume by ID",
			volumes: generateVolumes(10, 0),
			mockFunc: func(mockEC2 *MockEC2API, expErr error, volumes []types.Volume) {
				volumeOutput := &ec2.DescribeVolumesOutput{Volumes: volumes}
				mockEC2.EXPECT().DescribeVolumes(gomock.Any(), gomock.Any()).Return(volumeOutput, expErr).Times(1)
			},
			expErr: nil,
		},
		{
			name:    "success: volume by tag",
			volumes: generateVolumes(0, 10),
			mockFunc: func(mockEC2 *MockEC2API, expErr error, volumes []types.Volume) {
				volumeOutput := &ec2.DescribeVolumesOutput{Volumes: volumes}
				mockEC2.EXPECT().DescribeVolumes(gomock.Any(), gomock.Any()).Return(volumeOutput, expErr).Times(1)
			},
			expErr: nil,
		},
		{
			name:    "success: volume by ID and tag",
			volumes: generateVolumes(10, 10),
			mockFunc: func(mockEC2 *MockEC2API, expErr error, volumes []types.Volume) {
				volumeOutput := &ec2.DescribeVolumesOutput{Volumes: volumes}
				mockEC2.EXPECT().DescribeVolumes(gomock.Any(), gomock.Any()).Return(volumeOutput, expErr).Times(2)
			},
			expErr: nil,
		},
		{
			name:    "success: max capacity",
			volumes: generateVolumes(500, 0),
			mockFunc: func(mockEC2 *MockEC2API, expErr error, volumes []types.Volume) {
				volumeOutput := &ec2.DescribeVolumesOutput{Volumes: volumes}
				mockEC2.EXPECT().DescribeVolumes(gomock.Any(), gomock.Any()).Return(volumeOutput, expErr).Times(1)
			},
			expErr: nil,
		},
		{
			name:    "success: capacity exceeded",
			volumes: generateVolumes(550, 0),
			mockFunc: func(mockEC2 *MockEC2API, expErr error, volumes []types.Volume) {
				volumeOutput := &ec2.DescribeVolumesOutput{Volumes: volumes}
				mockEC2.EXPECT().DescribeVolumes(gomock.Any(), gomock.Any()).Return(volumeOutput, expErr).Times(2)
			},
			expErr: nil,
		},
		{
			name:    "fail: EC2 API generic error",
			volumes: generateVolumes(4, 0),
			mockFunc: func(mockEC2 *MockEC2API, expErr error, volumes []types.Volume) {
				mockEC2.EXPECT().DescribeVolumes(gomock.Any(), gomock.Any()).Return(nil, expErr).Times(1)
			},
			expErr: errors.New("Generic EC2 API error"),
		},
		{
			name:    "fail: volume not found",
			volumes: generateVolumes(1, 0),
			mockFunc: func(mockEC2 *MockEC2API, expErr error, volumes []types.Volume) {
				mockEC2.EXPECT().DescribeVolumes(gomock.Any(), gomock.Any()).Return(nil, expErr).Times(1)
			},
			expErr: errors.New("volume not found"),
		},
		{
			name: "fail: invalid tag",
			volumes: []types.Volume{
				{
					Tags: []types.Tag{
						{Key: aws.String("InvalidKey"), Value: aws.String("InvalidValue")},
					},
				},
			},
			mockFunc: func(mockEC2 *MockEC2API, expErr error, volumes []types.Volume) {
				volumeOutput := &ec2.DescribeVolumesOutput{Volumes: volumes}
				mockEC2.EXPECT().DescribeVolumes(gomock.Any(), gomock.Any()).Return(volumeOutput, expErr).Times(0)
			},
			expErr: errors.New("invalid tag"),
		},
		{
			name:    "fail: invalid request",
			volumes: []types.Volume{{VolumeId: aws.String("")}},
			mockFunc: func(mockEC2 *MockEC2API, expErr error, volumes []types.Volume) {
				mockEC2.EXPECT().DescribeVolumes(gomock.Any(), gomock.Any()).Return(nil, nil).Times(0)
			},
			expErr: ErrInvalidRequest,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			mockCtrl := gomock.NewController(t)
			defer mockCtrl.Finish()

			mockEC2 := NewMockEC2API(mockCtrl)
			c := newCloud(mockEC2)
			cloudInstance, ok := c.(*cloud)
			if !ok {
				t.Fatalf("could not assert cloudInstance as type cloud, %v", cloudInstance)
			}
			cloudInstance.bm = newBatcherManager(cloudInstance.ec2)

			tc.mockFunc(mockEC2, tc.expErr, tc.volumes)
			volumeIDs, volumeNames := extractVolumeIdentifiers(tc.volumes)
			executeDescribeVolumesTest(t, cloudInstance, volumeIDs, volumeNames, tc.expErr)
		})
	}
}
func executeDescribeVolumesTest(t *testing.T, c *cloud, volumeIDs, volumeNames []string, expErr error) {
	t.Helper()
	var wg sync.WaitGroup

	getRequestForID := func(id string) *ec2.DescribeVolumesInput {
		return &ec2.DescribeVolumesInput{VolumeIds: []string{id}}
	}

	getRequestForTag := func(volName string) *ec2.DescribeVolumesInput {
		return &ec2.DescribeVolumesInput{
			Filters: []types.Filter{
				{
					Name:   aws.String("tag:" + VolumeNameTagKey),
					Values: []string{volName},
				},
			},
		}
	}

	requests := make([]*ec2.DescribeVolumesInput, 0, len(volumeIDs)+len(volumeNames))
	for _, volumeID := range volumeIDs {
		requests = append(requests, getRequestForID(volumeID))
	}
	for _, volumeName := range volumeNames {
		requests = append(requests, getRequestForTag(volumeName))
	}

	r := make([]chan *types.Volume, len(requests))
	e := make([]chan error, len(requests))

	for i, request := range requests {
		wg.Add(1)
		r[i] = make(chan *types.Volume, 1)
		e[i] = make(chan error, 1)
		go func(resultCh chan *types.Volume, errCh chan error) {
			defer wg.Done()
			volume, err := c.batchDescribeVolumes(request)
			if err != nil {
				errCh <- err
				return
			}
			resultCh <- volume
		}(r[i], e[i])
	}

	wg.Wait()

	for i := range requests {
		select {
		case result := <-r[i]:
			if result == nil {
				t.Errorf("Received nil result for a request")
			}
		case err := <-e[i]:
			if expErr == nil {
				t.Errorf("Error while processing request: %v", err)
			}
			if !errors.Is(err, expErr) {
				t.Errorf("Expected error %v, but got %v", expErr, err)
			}
		default:
			t.Errorf("Did not receive a result or an error for a request")
		}
	}
}

func TestBatchDescribeInstances(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		name        string
		instanceIDs []string
		mockFunc    func(mockEC2 *MockEC2API, expErr error, reservations []types.Reservation)
		expErr      error
	}{
		{
			name:        "success: instance by ID",
			instanceIDs: []string{"i-001", "i-002", "i-003"},
			mockFunc: func(mockEC2 *MockEC2API, expErr error, reservations []types.Reservation) {
				reservationOutput := &ec2.DescribeInstancesOutput{Reservations: reservations}
				mockEC2.EXPECT().DescribeInstances(gomock.Any(), gomock.Any()).Return(reservationOutput, expErr).Times(1)
			},
			expErr: nil,
		},
		{
			name:        "fail: EC2 API generic error",
			instanceIDs: []string{"i-001", "i-002", "i-003"},
			mockFunc: func(mockEC2 *MockEC2API, expErr error, reservations []types.Reservation) {
				mockEC2.EXPECT().DescribeInstances(gomock.Any(), gomock.Any()).Return(nil, expErr).Times(1)
			},
			expErr: errors.New("generic EC2 API error"),
		},
		{
			name:        "fail: invalid request",
			instanceIDs: []string{""},
			mockFunc: func(mockEC2 *MockEC2API, expErr error, reservations []types.Reservation) {
				mockEC2.EXPECT().DescribeInstances(gomock.Any(), gomock.Any()).Return(nil, nil).Times(0)
			},
			expErr: ErrInvalidRequest,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			mockCtrl := gomock.NewController(t)
			defer mockCtrl.Finish()

			mockEC2 := NewMockEC2API(mockCtrl)
			c := newCloud(mockEC2)
			cloudInstance, ok := c.(*cloud)
			if !ok {
				t.Fatalf("could not assert cloudInstance as type cloud, %v", cloudInstance)
			}
			cloudInstance.bm = newBatcherManager(cloudInstance.ec2)

			// Setup mocks
			var instances []types.Instance
			for _, instanceID := range tc.instanceIDs {
				instances = append(instances, types.Instance{InstanceId: aws.String(instanceID)})
			}
			reservation := types.Reservation{Instances: instances}
			reservations := []types.Reservation{reservation}
			tc.mockFunc(mockEC2, tc.expErr, reservations)

			executeDescribeInstancesTest(t, cloudInstance, tc.instanceIDs, tc.expErr)
		})
	}
}

func executeDescribeInstancesTest(t *testing.T, c *cloud, instanceIDs []string, expErr error) {
	t.Helper()
	var wg sync.WaitGroup

	getRequestForID := func(id string) *ec2.DescribeInstancesInput {
		return &ec2.DescribeInstancesInput{InstanceIds: []string{id}}
	}

	requests := make([]*ec2.DescribeInstancesInput, 0, len(instanceIDs))
	for _, instanceID := range instanceIDs {
		requests = append(requests, getRequestForID(instanceID))
	}

	r := make([]chan types.Instance, len(requests))
	e := make([]chan error, len(requests))

	for i, request := range requests {
		wg.Add(1)
		r[i] = make(chan types.Instance, 1)
		e[i] = make(chan error, 1)

		go func(resultCh chan types.Instance, errCh chan error) {
			defer wg.Done()
			instance, err := c.batchDescribeInstances(request)
			if err != nil {
				errCh <- err
				return
			}
			resultCh <- *instance
		}(r[i], e[i])
	}

	wg.Wait()

	for i := range requests {
		select {
		case result := <-r[i]:
			if &result == (&types.Instance{}) {
				t.Errorf("Received nil result for a request")
			}
		case err := <-e[i]:
			if expErr == nil {
				t.Errorf("Error while processing request: %v", err)
			}
			if !errors.Is(err, expErr) {
				t.Errorf("Expected error %v, but got %v", expErr, err)
			}
		default:
			t.Errorf("Did not receive a result or an error for a request")
		}
	}
}

func generateSnapshots(snapIDCount, snapTagCount int) []types.Snapshot {
	snapshots := make([]types.Snapshot, 0, snapIDCount+snapTagCount)

	for i := range snapIDCount {
		snapID := fmt.Sprintf("snap-%d", i)
		snapshots = append(snapshots, types.Snapshot{SnapshotId: aws.String(snapID)})
	}

	for i := range snapTagCount {
		snapshotName := fmt.Sprintf("snap-name-%d", i)
		snapshots = append(snapshots, types.Snapshot{Tags: []types.Tag{{Key: aws.String(SnapshotNameTagKey), Value: aws.String(snapshotName)}}})
	}

	return snapshots
}

func extractSnapshotIdentifiers(snapshots []types.Snapshot) (snapshotIDs []string, snapshotNames []string) {
	for _, snapshot := range snapshots {
		if snapshot.SnapshotId != nil {
			snapshotIDs = append(snapshotIDs, *snapshot.SnapshotId)
		}
		for _, tag := range snapshot.Tags {
			if tag.Key != nil && *tag.Key == SnapshotNameTagKey && tag.Value != nil {
				snapshotNames = append(snapshotNames, *tag.Value)
			}
		}
	}
	return snapshotIDs, snapshotNames
}

func TestBatchDescribeSnapshots(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		name      string
		snapshots []types.Snapshot
		mockFunc  func(mockEC2 *MockEC2API, expErr error, snapshots []types.Snapshot)
		expErr    error
	}{
		{
			name:      "success: snapshot by ID",
			snapshots: generateSnapshots(3, 0),
			mockFunc: func(mockEC2 *MockEC2API, expErr error, snapshots []types.Snapshot) {
				snapshotOutput := &ec2.DescribeSnapshotsOutput{Snapshots: snapshots}
				mockEC2.EXPECT().DescribeSnapshots(gomock.Any(), gomock.Any()).Return(snapshotOutput, expErr).Times(1)
			},
		},
		{
			name:      "success: snapshot by tag",
			snapshots: generateSnapshots(0, 3),
			mockFunc: func(mockEC2 *MockEC2API, expErr error, snapshots []types.Snapshot) {
				snapshotOutput := &ec2.DescribeSnapshotsOutput{Snapshots: snapshots}
				mockEC2.EXPECT().DescribeSnapshots(gomock.Any(), gomock.Any()).Return(snapshotOutput, expErr).Times(1)
			},
		},
		{
			name:      "success: snapshot by ID and tag",
			snapshots: generateSnapshots(3, 4),
			mockFunc: func(mockEC2 *MockEC2API, expErr error, snapshots []types.Snapshot) {
				snapshotOutput := &ec2.DescribeSnapshotsOutput{Snapshots: snapshots}
				mockEC2.EXPECT().DescribeSnapshots(gomock.Any(), gomock.Any()).Return(snapshotOutput, expErr).Times(2)
			},
		},
		{
			name:      "fail: EC2 API generic error",
			snapshots: generateSnapshots(3, 2),
			mockFunc: func(mockEC2 *MockEC2API, expErr error, snapshots []types.Snapshot) {
				mockEC2.EXPECT().DescribeSnapshots(gomock.Any(), gomock.Any()).Return(nil, expErr).Times(2)
			},
			expErr: errors.New("generic EC2 API error"),
		},
		{
			name:      "fail: Snapshot not found by ID",
			snapshots: generateSnapshots(3, 0),
			mockFunc: func(mockEC2 *MockEC2API, expErr error, snapshots []types.Snapshot) {
				snapshotOutput := &ec2.DescribeSnapshotsOutput{Snapshots: snapshots[1:]} // Leave out first snapshot
				mockEC2.EXPECT().DescribeSnapshots(gomock.Any(), gomock.Any()).Return(snapshotOutput, nil).Times(1)
			},
			expErr: ErrNotFound,
		},
		{
			name:      "fail: Snapshot not found by tag",
			snapshots: generateSnapshots(0, 2),
			mockFunc: func(mockEC2 *MockEC2API, expErr error, snapshots []types.Snapshot) {
				snapshotOutput := &ec2.DescribeSnapshotsOutput{Snapshots: snapshots[1:]} // Leave out first snapshot
				mockEC2.EXPECT().DescribeSnapshots(gomock.Any(), gomock.Any()).Return(snapshotOutput, nil).Times(1)
			},
			expErr: ErrNotFound,
		},
		{
			name:      "fail: invalid request",
			snapshots: []types.Snapshot{{SnapshotId: aws.String("")}},
			mockFunc: func(mockEC2 *MockEC2API, expErr error, snapshots []types.Snapshot) {
				mockEC2.EXPECT().DescribeSnapshots(gomock.Any(), gomock.Any()).Return(nil, nil).Times(0)
			},
			expErr: ErrInvalidRequest,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			mockCtrl := gomock.NewController(t)
			defer mockCtrl.Finish()

			mockEC2 := NewMockEC2API(mockCtrl)
			c := newCloud(mockEC2)
			cloudInstance, ok := c.(*cloud)
			if !ok {
				t.Fatalf("could not assert cloudInstance as type cloud, %v", cloudInstance)
			}
			cloudInstance.bm = newBatcherManager(cloudInstance.ec2)

			tc.mockFunc(mockEC2, tc.expErr, tc.snapshots)
			snapshotIDs, snapshotNames := extractSnapshotIdentifiers(tc.snapshots)
			executeDescribeSnapshotsTest(t, cloudInstance, snapshotIDs, snapshotNames, tc.expErr)
		})
	}
}

func executeDescribeSnapshotsTest(t *testing.T, c *cloud, snapshotIDs, snapshotNames []string, expErr error) {
	t.Helper()
	var wg sync.WaitGroup

	getRequestForID := func(id string) *ec2.DescribeSnapshotsInput {
		return &ec2.DescribeSnapshotsInput{SnapshotIds: []string{id}}
	}

	getRequestForTag := func(snapName string) *ec2.DescribeSnapshotsInput {
		return &ec2.DescribeSnapshotsInput{
			Filters: []types.Filter{
				{
					Name:   aws.String("tag:" + SnapshotNameTagKey),
					Values: []string{snapName},
				},
			},
		}
	}

	requests := make([]*ec2.DescribeSnapshotsInput, 0, len(snapshotIDs)+len(snapshotNames))
	for _, snapshotID := range snapshotIDs {
		requests = append(requests, getRequestForID(snapshotID))
	}
	for _, snapshotName := range snapshotNames {
		requests = append(requests, getRequestForTag(snapshotName))
	}

	r := make([]chan *types.Snapshot, len(requests))
	e := make([]chan error, len(requests))

	for i, request := range requests {
		wg.Add(1)
		r[i] = make(chan *types.Snapshot, 1)
		e[i] = make(chan error, 1)

		go func(resultCh chan *types.Snapshot, errCh chan error) {
			defer wg.Done()
			snapshot, err := c.batchDescribeSnapshots(request)
			if err != nil {
				errCh <- err
				return
			}
			resultCh <- snapshot
		}(r[i], e[i])
	}

	wg.Wait()

	for i := range requests {
		select {
		case result := <-r[i]:
			if result == nil {
				t.Errorf("Received nil for a request")
			}
		case err := <-e[i]:
			if expErr == nil {
				t.Errorf("Error while processing request: %v", err)
			}
			if !errors.Is(err, expErr) {
				t.Errorf("Expected error %v, but got %v", expErr, err)
			}
		default:
			t.Errorf("Did not receive a result or an error for a request")
		}
	}
}

func TestCheckDesiredState(t *testing.T) {
	testCases := []struct {
		name           string
		volumeID       string
		desiredSizeGiB int32
		options        *ModifyDiskOptions
		expErr         error
	}{
		{
			name:           "success: normal path",
			volumeID:       "vol-001",
			desiredSizeGiB: 5,
			options: &ModifyDiskOptions{
				VolumeType: VolumeTypeGP2,
				IOPS:       3000,
				Throughput: 1000,
			},
		},
		{
			name:           "failure: volume is still being expanded",
			volumeID:       "vol-001",
			desiredSizeGiB: 500,
			options: &ModifyDiskOptions{
				VolumeType: VolumeTypeGP2,
				IOPS:       3000,
				Throughput: 1000,
			},
			expErr: errors.New("volume \"vol-001\" is still being expanded to 500 size"),
		},
		{
			name:           "failure: volume is still being modified to iops",
			volumeID:       "vol-001",
			desiredSizeGiB: 50,
			options: &ModifyDiskOptions{
				VolumeType: VolumeTypeGP2,
				IOPS:       4000,
				Throughput: 1000,
			},
			expErr: errors.New("volume \"vol-001\" is still being modified to iops 4000"),
		},
		{
			name:           "failure: volume is still being modifed to type",
			volumeID:       "vol-001",
			desiredSizeGiB: 50,
			options: &ModifyDiskOptions{
				VolumeType: VolumeTypeGP3,
				IOPS:       3000,
				Throughput: 1000,
			},
			expErr: fmt.Errorf("volume \"vol-001\" is still being modified to type %q", VolumeTypeGP3),
		},
		{
			name:           "failure: volume is still being modified to throughput",
			volumeID:       "vol-001",
			desiredSizeGiB: 5,
			options: &ModifyDiskOptions{
				VolumeType: VolumeTypeGP2,
				IOPS:       3000,
				Throughput: 2000,
			},
			expErr: errors.New("volume \"vol-001\" is still being modified to throughput 2000"),
		},
	}
	for _, tc := range testCases {
		mockCtrl := gomock.NewController(t)
		defer mockCtrl.Finish()
		mockEC2 := NewMockEC2API(mockCtrl)
		c := newCloud(mockEC2)
		cloudInstance, ok := c.(*cloud)
		if !ok {
			t.Fatalf("could not assert cloudInstance as type cloud, %v", cloudInstance)
		}
		mockEC2.EXPECT().DescribeVolumes(gomock.Any(), gomock.Any()).Return(&ec2.DescribeVolumesOutput{
			Volumes: []types.Volume{
				{
					VolumeId:   aws.String("vol-001"),
					Size:       aws.Int32(50),
					VolumeType: types.VolumeTypeGp2,
					Iops:       aws.Int32(3000),
					Throughput: aws.Int32(1000),
				},
			},
		}, nil)
		_, err := cloudInstance.checkDesiredState(t.Context(), tc.volumeID, tc.desiredSizeGiB, tc.options)
		if err != nil {
			if tc.expErr == nil {
				t.Fatalf("Did not expect to get an error but got %q", err)
			} else if tc.expErr.Error() != err.Error() {
				t.Fatalf("checkDesiredState() failed: expected error %q, got: %q", tc.expErr, err)
			}
		} else {
			if tc.expErr != nil {
				t.Fatalf("checkDesiredState() failed: expected error got nothing")
			}
		}
	}
}

func TestBatchDescribeVolumesModifications(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		name      string
		volumeIDs []string
		mockFunc  func(mockEC2 *MockEC2API, expErr error, volumeModifications []types.VolumeModification)
		expErr    error
	}{
		{
			name:      "success: volumeModification by ID",
			volumeIDs: []string{"vol-001", "vol-002", "vol-003"},
			mockFunc: func(mockEC2 *MockEC2API, expErr error, volumeModifications []types.VolumeModification) {
				volumeModificationsOutput := &ec2.DescribeVolumesModificationsOutput{VolumesModifications: volumeModifications}
				mockEC2.EXPECT().DescribeVolumesModifications(gomock.Any(), gomock.Any()).Return(volumeModificationsOutput, expErr).Times(1)
			},
			expErr: nil,
		},
		{
			name:      "fail: EC2 API generic error",
			volumeIDs: []string{"vol-001", "vol-002", "vol-003"},
			mockFunc: func(mockEC2 *MockEC2API, expErr error, volumeModifications []types.VolumeModification) {
				mockEC2.EXPECT().DescribeVolumesModifications(gomock.Any(), gomock.Any()).Return(nil, expErr).Times(1)
			},
			expErr: errors.New("generic EC2 API error"),
		},
		{
			name:      "fail: invalid request",
			volumeIDs: []string{""},
			mockFunc: func(mockEC2 *MockEC2API, expErr error, volumeModifications []types.VolumeModification) {
				mockEC2.EXPECT().DescribeVolumesModifications(gomock.Any(), gomock.Any()).Return(nil, expErr).Times(0)
			},
			expErr: ErrInvalidRequest,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			mockCtrl := gomock.NewController(t)
			defer mockCtrl.Finish()

			mockEC2 := NewMockEC2API(mockCtrl)
			c := newCloud(mockEC2)
			cloudInstance, ok := c.(*cloud)
			if !ok {
				t.Fatalf("could not assert cloudInstance as type cloud, %v", cloudInstance)
			}
			cloudInstance.bm = newBatcherManager(cloudInstance.ec2)

			// Setup mocks
			var volumeModifications []types.VolumeModification
			for _, volumeID := range tc.volumeIDs {
				volumeModifications = append(volumeModifications, types.VolumeModification{VolumeId: aws.String(volumeID)})
			}
			tc.mockFunc(mockEC2, tc.expErr, volumeModifications)

			executeDescribeVolumesModificationsTest(t, cloudInstance, tc.volumeIDs, tc.expErr)
		})
	}
}

func executeDescribeVolumesModificationsTest(t *testing.T, c *cloud, volumeIDs []string, expErr error) {
	t.Helper()
	var wg sync.WaitGroup

	getRequestForID := func(id string) *ec2.DescribeVolumesModificationsInput {
		return &ec2.DescribeVolumesModificationsInput{VolumeIds: []string{id}}
	}

	requests := make([]*ec2.DescribeVolumesModificationsInput, 0, len(volumeIDs))
	for _, volumeID := range volumeIDs {
		requests = append(requests, getRequestForID(volumeID))
	}

	r := make([]chan types.VolumeModification, len(requests))
	e := make([]chan error, len(requests))

	for i, request := range requests {
		wg.Add(1)
		r[i] = make(chan types.VolumeModification, 1)
		e[i] = make(chan error, 1)

		go func(resultCh chan types.VolumeModification, errCh chan error) {
			defer wg.Done()
			volumeModification, err := c.batchDescribeVolumesModifications(request)
			if err != nil {
				errCh <- err
				return
			}
			resultCh <- *volumeModification
		}(r[i], e[i])
	}

	wg.Wait()

	for i := range requests {
		select {
		case result := <-r[i]:
			if &result == (&types.VolumeModification{}) {
				t.Errorf("Received nil result for a request")
			}
		case err := <-e[i]:
			if expErr == nil {
				t.Errorf("Error while processing request: %v", err)
			}
			if !errors.Is(err, expErr) {
				t.Errorf("Expected error %v, but got %v", expErr, err)
			}
		default:
			t.Errorf("Did not receive a result or an error for a request")
		}
	}
}

func TestCreateDisk(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		name                 string
		volumeName           string
		volState             string
		diskOptions          *DiskOptions
		expDisk              *Disk
		expErr               error
		expCreateVolumeErr   error
		expDescVolumeErr     error
		expCreateTagsErr     error
		expCreateVolumeInput *ec2.CreateVolumeInput
	}{
		{
			name:       "success: normal",
			volumeName: "vol-test-name",
			diskOptions: &DiskOptions{
				CapacityBytes: util.GiBToBytes(1),
				Tags:          map[string]string{VolumeNameTagKey: "vol-test", AwsEbsDriverTagKey: "true"},
			},
			expCreateVolumeInput: &ec2.CreateVolumeInput{},
			expDisk: &Disk{
				VolumeID:         "vol-test",
				CapacityGiB:      1,
				AvailabilityZone: defaultZone,
			},
			expErr: nil,
		},
		{
			name:       "success: normal with iops",
			volumeName: "vol-test-name",
			diskOptions: &DiskOptions{
				CapacityBytes: util.GiBToBytes(500),
				Tags:          map[string]string{VolumeNameTagKey: "vol-test", AwsEbsDriverTagKey: "true"},
				IOPS:          6000,
			},
			expDisk: &Disk{
				VolumeID:         "vol-test",
				CapacityGiB:      500,
				AvailabilityZone: defaultZone,
			},
			expCreateVolumeInput: &ec2.CreateVolumeInput{
				Iops: aws.Int32(6000),
			},
			expErr: nil,
		},
		{
			name:       "success: normal with gp2 options",
			volumeName: "vol-test-name",
			diskOptions: &DiskOptions{
				CapacityBytes: util.GiBToBytes(1),
				VolumeType:    VolumeTypeGP2,
				Tags:          map[string]string{VolumeNameTagKey: "vol-test"},
			},
			expCreateVolumeInput: &ec2.CreateVolumeInput{},
			expDisk: &Disk{
				VolumeID:         "vol-test",
				CapacityGiB:      1,
				AvailabilityZone: defaultZone,
			},
			expErr: nil,
		},
		{
			name:       "success: normal with io2 options",
			volumeName: "vol-test-name",
			diskOptions: &DiskOptions{
				CapacityBytes: util.GiBToBytes(1),
				Tags:          map[string]string{VolumeNameTagKey: "vol-test", AwsEbsDriverTagKey: "true"},
				VolumeType:    VolumeTypeIO2,
				IOPSPerGB:     100,
			},
			expDisk: &Disk{
				VolumeID:         "vol-test",
				CapacityGiB:      1,
				AvailabilityZone: defaultZone,
			},
			expCreateVolumeInput: &ec2.CreateVolumeInput{
				Iops: aws.Int32(100),
			},
			expErr: nil,
		},
		{
			name:       "success: io1 with IOPS parameter",
			volumeName: "vol-test-name",
			diskOptions: &DiskOptions{
				CapacityBytes: util.GiBToBytes(200),
				Tags:          map[string]string{VolumeNameTagKey: "vol-test", AwsEbsDriverTagKey: "true"},
				VolumeType:    VolumeTypeIO1,
				IOPS:          100,
			},
			expDisk: &Disk{
				VolumeID:         "vol-test",
				CapacityGiB:      200,
				AvailabilityZone: defaultZone,
			},
			expCreateVolumeInput: &ec2.CreateVolumeInput{
				Iops: aws.Int32(100),
			},
			expErr: nil,
		},
		{
			name:       "success: io2 with IOPS parameter",
			volumeName: "vol-test-name",
			diskOptions: &DiskOptions{
				CapacityBytes: util.GiBToBytes(1),
				Tags:          map[string]string{VolumeNameTagKey: "vol-test", AwsEbsDriverTagKey: "true"},
				VolumeType:    VolumeTypeIO2,
				IOPS:          100,
			},
			expDisk: &Disk{
				VolumeID:         "vol-test",
				CapacityGiB:      1,
				AvailabilityZone: defaultZone,
			},
			expCreateVolumeInput: &ec2.CreateVolumeInput{
				Iops: aws.Int32(100),
			},
			expErr: nil,
		},
		{
			name:       "success: normal with gp3 options",
			volumeName: "vol-test-name",
			diskOptions: &DiskOptions{
				CapacityBytes: util.GiBToBytes(400),
				Tags:          map[string]string{VolumeNameTagKey: "vol-test", AwsEbsDriverTagKey: "true"},
				VolumeType:    VolumeTypeGP3,
				IOPS:          3000,
				Throughput:    125,
			},
			expDisk: &Disk{
				VolumeID:         "vol-test",
				CapacityGiB:      400,
				AvailabilityZone: defaultZone,
			},
			expCreateVolumeInput: &ec2.CreateVolumeInput{
				Iops:       aws.Int32(3000),
				Throughput: aws.Int32(125),
			},
			expErr: nil,
		},
		{
			name:       "success: normal with provided zone",
			volumeName: "vol-test-name",
			diskOptions: &DiskOptions{
				CapacityBytes:    util.GiBToBytes(1),
				Tags:             map[string]string{VolumeNameTagKey: "vol-test", AwsEbsDriverTagKey: "true"},
				AvailabilityZone: expZone,
			},
			expDisk: &Disk{
				VolumeID:         "vol-test",
				CapacityGiB:      1,
				AvailabilityZone: expZone,
			},
			expCreateVolumeInput: &ec2.CreateVolumeInput{},
			expErr:               nil,
		},
		{
			name:       "success: normal with provided zone-id",
			volumeName: "vol-test-name",
			diskOptions: &DiskOptions{
				CapacityBytes:      util.GiBToBytes(1),
				Tags:               map[string]string{VolumeNameTagKey: "vol-test", AwsEbsDriverTagKey: "true"},
				AvailabilityZoneID: expZoneID,
			},
			expDisk: &Disk{
				VolumeID:           "vol-test",
				CapacityGiB:        1,
				AvailabilityZoneID: expZoneID,
			},
			expCreateVolumeInput: &ec2.CreateVolumeInput{},
			expErr:               nil,
		},
		{
			name:       "success: normal with encrypted volume",
			volumeName: "vol-test-name",
			diskOptions: &DiskOptions{
				CapacityBytes:    util.GiBToBytes(1),
				Tags:             map[string]string{VolumeNameTagKey: "vol-test", AwsEbsDriverTagKey: "true"},
				AvailabilityZone: expZone,
				Encrypted:        true,
				KmsKeyID:         "arn:aws:kms:us-east-1:012345678910:key/abcd1234-a123-456a-a12b-a123b4cd56ef",
			},
			expDisk: &Disk{
				VolumeID:         "vol-test",
				CapacityGiB:      1,
				AvailabilityZone: expZone,
			},
			expCreateVolumeInput: &ec2.CreateVolumeInput{},
			expErr:               nil,
		},
		{
			name:       "success: outpost volume",
			volumeName: "vol-test-name",
			diskOptions: &DiskOptions{
				CapacityBytes:    util.GiBToBytes(1),
				Tags:             map[string]string{VolumeNameTagKey: "vol-test", AwsEbsDriverTagKey: "true"},
				AvailabilityZone: expZone,
				OutpostArn:       "arn:aws:outposts:us-west-2:111111111111:outpost/op-0aaa000a0aaaa00a0",
			},
			expDisk: &Disk{
				VolumeID:         "vol-test",
				CapacityGiB:      1,
				AvailabilityZone: expZone,
				OutpostArn:       "arn:aws:outposts:us-west-2:111111111111:outpost/op-0aaa000a0aaaa00a0",
			},
			expCreateVolumeInput: &ec2.CreateVolumeInput{},
			expErr:               nil,
		},
		{
			name:       "success: empty outpost arn",
			volumeName: "vol-test-name",
			diskOptions: &DiskOptions{
				CapacityBytes:    util.GiBToBytes(1),
				Tags:             map[string]string{VolumeNameTagKey: "vol-test", AwsEbsDriverTagKey: "true"},
				AvailabilityZone: expZone,
			},
			expDisk: &Disk{
				VolumeID:         "vol-test",
				CapacityGiB:      1,
				AvailabilityZone: expZone,
				OutpostArn:       "",
			},
			expCreateVolumeInput: &ec2.CreateVolumeInput{},
			expErr:               nil,
		},
		{
			name:       "fail: ec2.CreateVolume returned CreateVolume error",
			volumeName: "vol-test-name-error",
			diskOptions: &DiskOptions{
				CapacityBytes:    util.GiBToBytes(1),
				Tags:             map[string]string{VolumeNameTagKey: "vol-test", AwsEbsDriverTagKey: "true"},
				AvailabilityZone: expZone,
			},
			expCreateVolumeInput: &ec2.CreateVolumeInput{},
			expErr:               errors.New("could not create volume in EC2: CreateVolume generic error"),
			expCreateVolumeErr:   errors.New("CreateVolume generic error"),
		},
		{
			name:       "fail: ec2.CreateVolume returned snapshot not found error",
			volumeName: "vol-test-name-error",
			diskOptions: &DiskOptions{
				CapacityBytes:    util.GiBToBytes(1),
				Tags:             map[string]string{VolumeNameTagKey: "vol-test", AwsEbsDriverTagKey: "true"},
				AvailabilityZone: expZone,
			},
			expCreateVolumeInput: &ec2.CreateVolumeInput{},
			expErr:               ErrNotFound,
			expCreateVolumeErr: &smithy.GenericAPIError{
				Code:    "InvalidSnapshot.NotFound",
				Message: "Snapshot not found",
			},
		},
		{
			name:       "fail: ec2.CreateVolume returned Idempotent Parameter Mismatch error",
			volumeName: "vol-test-name-error",
			diskOptions: &DiskOptions{
				CapacityBytes:    util.GiBToBytes(1),
				Tags:             map[string]string{VolumeNameTagKey: "vol-test", AwsEbsDriverTagKey: "true"},
				AvailabilityZone: expZone,
			},
			expCreateVolumeInput: &ec2.CreateVolumeInput{},
			expErr:               fmt.Errorf("could not create volume in EC2: %w", errors.New("an error occurred: IdempotentParameterMismatch")),
			expCreateVolumeErr:   fmt.Errorf("an error occurred: %w", errors.New("IdempotentParameterMismatch")),
		},
		{
			name:       "fail: ec2.DescribeVolumes error after volume created",
			volumeName: "vol-test-name-error",
			volState:   "creating",
			diskOptions: &DiskOptions{
				CapacityBytes:    util.GiBToBytes(1),
				Tags:             map[string]string{VolumeNameTagKey: "vol-test", AwsEbsDriverTagKey: "true"},
				AvailabilityZone: "",
			},
			expCreateVolumeInput: &ec2.CreateVolumeInput{},
			expErr:               errors.New("timed out waiting for volume to create: DescribeVolumes generic error"),
			expDescVolumeErr:     errors.New("DescribeVolumes generic error"),
		},
		{
			name:       "fail: Volume is not ready to use, volume stuck in creating status and controller context deadline exceeded",
			volumeName: "vol-test-name-error",
			volState:   "creating",
			diskOptions: &DiskOptions{
				CapacityBytes:    util.GiBToBytes(1),
				Tags:             map[string]string{VolumeNameTagKey: "vol-test", AwsEbsDriverTagKey: "true"},
				AvailabilityZone: "",
			},
			expCreateVolumeInput: &ec2.CreateVolumeInput{},
			expErr:               errors.New("timed out waiting for volume to create: timed out waiting for the condition"),
		},
		{
			name:       "success: normal from snapshot",
			volumeName: "vol-test-name",
			diskOptions: &DiskOptions{
				CapacityBytes:    util.GiBToBytes(1),
				Tags:             map[string]string{VolumeNameTagKey: "vol-test", AwsEbsDriverTagKey: "true"},
				AvailabilityZone: expZone,
				SnapshotID:       "snapshot-test",
			},
			expDisk: &Disk{
				VolumeID:         "vol-test",
				CapacityGiB:      1,
				AvailabilityZone: expZone,
			},
			expCreateVolumeInput: &ec2.CreateVolumeInput{},
			expErr:               nil,
		},
		{
			name:       "success: io1 with too low iopsPerGB and AllowIOPSPerGBIncrease",
			volumeName: "vol-test-name",
			diskOptions: &DiskOptions{
				CapacityBytes:          util.GiBToBytes(4),
				Tags:                   map[string]string{VolumeNameTagKey: "vol-test", AwsEbsDriverTagKey: "true"},
				VolumeType:             VolumeTypeIO1,
				IOPSPerGB:              1,
				AllowIOPSPerGBIncrease: true,
			},
			expDisk: &Disk{
				VolumeID:         "vol-test",
				CapacityGiB:      4,
				AvailabilityZone: defaultZone,
			},
			expCreateVolumeInput: &ec2.CreateVolumeInput{
				Iops: aws.Int32(100),
			},
			expErr: nil,
		},
		{
			name:       "fail: invalid StorageClass parameters; specified both IOPS and IOPSPerGb",
			volumeName: "vol-test-name",
			diskOptions: &DiskOptions{
				CapacityBytes: util.GiBToBytes(4),
				Tags:          map[string]string{VolumeNameTagKey: "vol-test", AwsEbsDriverTagKey: "true"},
				VolumeType:    VolumeTypeIO1,
				IOPS:          1,
				IOPSPerGB:     1,
			},
			expDisk: &Disk{
				VolumeID:         "vol-test",
				CapacityGiB:      4,
				AvailabilityZone: defaultZone,
			},
			expCreateVolumeInput: nil,
			expErr:               errors.New("invalid StorageClass parameters; specify either IOPS or IOPSPerGb, not both"),
		},
		{
			name:       "success: small io1 with too high iopsPerGB",
			volumeName: "vol-test-name",
			diskOptions: &DiskOptions{
				CapacityBytes: util.GiBToBytes(4),
				Tags:          map[string]string{VolumeNameTagKey: "vol-test", AwsEbsDriverTagKey: "true"},
				VolumeType:    VolumeTypeIO1,
				IOPSPerGB:     10000,
			},
			expDisk: &Disk{
				VolumeID:         "vol-test",
				CapacityGiB:      4,
				AvailabilityZone: defaultZone,
			},
			expCreateVolumeInput: &ec2.CreateVolumeInput{
				Iops: aws.Int32(200),
			},
			expErr: nil,
		},
		{
			name:       "success: large io1 with too high iopsPerGB",
			volumeName: "vol-test-name",
			diskOptions: &DiskOptions{
				CapacityBytes: util.GiBToBytes(4000),
				Tags:          map[string]string{VolumeNameTagKey: "vol-test", AwsEbsDriverTagKey: "true"},
				VolumeType:    VolumeTypeIO1,
				IOPSPerGB:     10000,
			},
			expDisk: &Disk{
				VolumeID:         "vol-test",
				CapacityGiB:      4000,
				AvailabilityZone: defaultZone,
			},
			expCreateVolumeInput: &ec2.CreateVolumeInput{
				Iops: aws.Int32(64000),
			},
			expErr: nil,
		},
		{
			name:       "success: io2 with too low iopsPerGB and AllowIOPSPerGBIncrease",
			volumeName: "vol-test-name",
			diskOptions: &DiskOptions{
				CapacityBytes:          util.GiBToBytes(4),
				Tags:                   map[string]string{VolumeNameTagKey: "vol-test", AwsEbsDriverTagKey: "true"},
				VolumeType:             VolumeTypeIO2,
				IOPSPerGB:              1,
				AllowIOPSPerGBIncrease: true,
			},
			expDisk: &Disk{
				VolumeID:         "vol-test",
				CapacityGiB:      4,
				AvailabilityZone: defaultZone,
			},
			expCreateVolumeInput: &ec2.CreateVolumeInput{
				Iops: aws.Int32(100),
			},
			expErr: nil,
		},
		{
			name:       "success: small io2 with too high iopsPerGB",
			volumeName: "vol-test-name",
			diskOptions: &DiskOptions{
				CapacityBytes: util.GiBToBytes(4),
				Tags:          map[string]string{VolumeNameTagKey: "vol-test", AwsEbsDriverTagKey: "true"},
				VolumeType:    VolumeTypeIO2,
				IOPSPerGB:     10000,
			},
			expDisk: &Disk{
				VolumeID:         "vol-test",
				CapacityGiB:      4,
				AvailabilityZone: defaultZone,
			},
			expCreateVolumeInput: &ec2.CreateVolumeInput{
				Iops: aws.Int32(2000),
			},
			expErr: nil,
		},
		{
			name:       "success: large io2 with too high iopsPerGB",
			volumeName: "vol-test-name",
			diskOptions: &DiskOptions{
				CapacityBytes: util.GiBToBytes(4000),
				Tags:          map[string]string{VolumeNameTagKey: "vol-test", AwsEbsDriverTagKey: "true"},
				VolumeType:    VolumeTypeIO2,
				IOPSPerGB:     100000,
			},
			expDisk: &Disk{
				VolumeID:         "vol-test",
				CapacityGiB:      4000,
				AvailabilityZone: defaultZone,
			},
			expCreateVolumeInput: &ec2.CreateVolumeInput{
				Iops: aws.Int32(64000),
			},
			expErr: nil,
		},
		{
			name:       "success: large io2 with too high iopsPerGB",
			volumeName: "vol-test-name",
			diskOptions: &DiskOptions{
				CapacityBytes: util.GiBToBytes(3333),
				Tags:          map[string]string{VolumeNameTagKey: "vol-test", AwsEbsDriverTagKey: "true"},
				VolumeType:    VolumeTypeIO2,
				IOPSPerGB:     100000,
			},
			expDisk: &Disk{
				VolumeID:         "vol-test",
				CapacityGiB:      3333,
				AvailabilityZone: defaultZone,
			},
			expCreateVolumeInput: &ec2.CreateVolumeInput{
				Iops: aws.Int32(256000),
			},
			expErr: nil,
		},
		{
			name:       "success: create default volume with throughput",
			volumeName: "vol-test-name",
			diskOptions: &DiskOptions{
				CapacityBytes: util.GiBToBytes(1),
				Tags:          map[string]string{VolumeNameTagKey: "vol-test", AwsEbsDriverTagKey: "true"},
				Throughput:    250,
			},
			expCreateVolumeInput: &ec2.CreateVolumeInput{
				Throughput: aws.Int32(250),
			},
			expDisk: &Disk{
				VolumeID:         "vol-test",
				CapacityGiB:      1,
				AvailabilityZone: defaultZone,
			},
			expErr: nil,
		},
		{
			name:       "success: multi-attach with IO2",
			volumeName: "vol-test-name",
			diskOptions: &DiskOptions{
				CapacityBytes:      util.GiBToBytes(4),
				Tags:               map[string]string{VolumeNameTagKey: "vol-test", AwsEbsDriverTagKey: "true"},
				VolumeType:         VolumeTypeIO2,
				MultiAttachEnabled: true,
				IOPSPerGB:          10000,
			},
			expDisk: &Disk{
				VolumeID:         "vol-test",
				CapacityGiB:      4,
				AvailabilityZone: defaultZone,
			},
			expCreateVolumeInput: &ec2.CreateVolumeInput{
				Iops: aws.Int32(2000),
			},
			expErr: nil,
		},
		{
			name:       "success: create volume from snapshot with initialization rate",
			volumeName: "vol-test-name",
			diskOptions: &DiskOptions{
				CapacityBytes:            util.GiBToBytes(1),
				Tags:                     map[string]string{VolumeNameTagKey: "vol-test", AwsEbsDriverTagKey: "true"},
				SnapshotID:               "snapshot-test",
				VolumeInitializationRate: 200,
			},
			expDisk: &Disk{
				VolumeID:         "vol-test",
				CapacityGiB:      1,
				AvailabilityZone: defaultZone,
			},
			expCreateVolumeInput: &ec2.CreateVolumeInput{
				VolumeInitializationRate: aws.Int32(200),
			},
			expErr: nil,
		},
		{
			name:       "failure: create volume from snapshot with initialization rate when snapshotID is missing",
			volumeName: "vol-test-name",
			diskOptions: &DiskOptions{
				CapacityBytes:            util.GiBToBytes(1),
				Tags:                     map[string]string{VolumeNameTagKey: "vol-test", AwsEbsDriverTagKey: "true"},
				VolumeInitializationRate: 200,
			},
			expDisk: nil,
			expCreateVolumeInput: &ec2.CreateVolumeInput{
				VolumeInitializationRate: aws.Int32(200),
			},
			expCreateVolumeErr: errors.New("InvalidParameterCombination"),
			expErr:             fmt.Errorf("could not create volume in EC2: %w", errors.New("InvalidParameterCombination")),
		},
		{
			name:       "failure: multi-attach with GP3",
			volumeName: "vol-test-name",
			diskOptions: &DiskOptions{
				CapacityBytes:      util.GiBToBytes(4),
				Tags:               map[string]string{VolumeNameTagKey: "vol-test", AwsEbsDriverTagKey: "true"},
				VolumeType:         VolumeTypeGP3,
				MultiAttachEnabled: true,
				IOPSPerGB:          10000,
			},
			expDisk: &Disk{
				VolumeID:         "vol-test",
				CapacityGiB:      4,
				AvailabilityZone: defaultZone,
			},
			expErr: errors.New("CreateDisk: multi-attach is only supported for io2 volumes"),
		},
		{
			name:       "failure: invalid VolumeType",
			volumeName: "vol-test-name",
			diskOptions: &DiskOptions{
				CapacityBytes: util.GiBToBytes(1),
				Tags:          map[string]string{VolumeNameTagKey: "vol-test", AwsEbsDriverTagKey: "true"},
				VolumeType:    "invalidVolumeType",
			},
			expDisk: &Disk{
				VolumeID:         "vol-test",
				CapacityGiB:      1,
				AvailabilityZone: defaultZone,
			},
			expErr: fmt.Errorf("invalid AWS VolumeType %q", "invalidVolumeType"),
		},
		{
			name:       "failure: create volume returned volume limit exceeded error",
			volumeName: "vol-test-name",
			diskOptions: &DiskOptions{
				CapacityBytes: util.GiBToBytes(1),
				Tags:          map[string]string{VolumeNameTagKey: "vol-test", AwsEbsDriverTagKey: "true"},
			},
			expDisk:              nil,
			expCreateVolumeInput: &ec2.CreateVolumeInput{},
			expCreateVolumeErr:   errors.New("VolumeLimitExceeded"),
			expErr:               fmt.Errorf("could not create volume in EC2: %w", errors.New("VolumeLimitExceeded")),
		},
		{
			name:       "failure: create volume returned max iops limit exceeded error",
			volumeName: "vol-test-name",
			diskOptions: &DiskOptions{
				CapacityBytes: util.GiBToBytes(1),
				Tags:          map[string]string{VolumeNameTagKey: "vol-test", AwsEbsDriverTagKey: "true"},
			},
			expDisk:              nil,
			expCreateVolumeInput: &ec2.CreateVolumeInput{},
			expCreateVolumeErr:   errors.New("MaxIOPSLimitExceeded"),
			expErr:               fmt.Errorf("could not create volume in EC2: %w", errors.New("MaxIOPSLimitExceeded")),
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			mockCtrl := gomock.NewController(t)
			mockEC2 := NewMockEC2API(mockCtrl)
			c := newCloud(mockEC2)

			volState := tc.volState
			if volState == "" {
				volState = "available"
			}
			snapshot := types.Snapshot{
				SnapshotId: aws.String(tc.diskOptions.SnapshotID),
				VolumeId:   aws.String("snap-test-volume"),
				State:      types.SnapshotStateCompleted,
			}
			ctx, ctxCancel := context.WithDeadline(t.Context(), time.Now().Add(defaultCreateDiskDeadline))
			defer ctxCancel()

			if tc.expCreateVolumeInput != nil {
				mockEC2.EXPECT().CreateVolume(gomock.Any(), gomock.Any(), gomock.Any()).Return(&ec2.CreateVolumeOutput{
					VolumeId:   aws.String(tc.diskOptions.Tags[VolumeNameTagKey]),
					Size:       aws.Int32(util.BytesToGiB(tc.diskOptions.CapacityBytes)),
					OutpostArn: aws.String(tc.diskOptions.OutpostArn),
				}, tc.expCreateVolumeErr)
				mockEC2.EXPECT().DescribeVolumes(gomock.Any(), gomock.Any()).Return(&ec2.DescribeVolumesOutput{
					Volumes: []types.Volume{
						{
							VolumeId:         aws.String(tc.diskOptions.Tags[VolumeNameTagKey]),
							Size:             aws.Int32(util.BytesToGiB(tc.diskOptions.CapacityBytes)),
							State:            types.VolumeState(volState),
							AvailabilityZone: aws.String(tc.diskOptions.AvailabilityZone),
							OutpostArn:       aws.String(tc.diskOptions.OutpostArn),
						},
					},
				}, tc.expDescVolumeErr).AnyTimes()
				if len(tc.diskOptions.SnapshotID) > 0 {
					mockEC2.EXPECT().DescribeSnapshots(gomock.Any(), gomock.Any()).Return(&ec2.DescribeSnapshotsOutput{Snapshots: []types.Snapshot{snapshot}}, nil).AnyTimes()
				}
				if len(tc.diskOptions.AvailabilityZone) == 0 && len(tc.diskOptions.AvailabilityZoneID) == 0 {
					mockEC2.EXPECT().DescribeAvailabilityZones(gomock.Any(), gomock.Any()).Return(&ec2.DescribeAvailabilityZonesOutput{
						AvailabilityZones: []types.AvailabilityZone{
							{ZoneName: aws.String(defaultZone)},
						},
					}, nil)
				}
			}

			disk, err := c.CreateDisk(ctx, tc.volumeName, tc.diskOptions)
			if err != nil {
				if tc.expErr == nil {
					t.Fatalf("CreateDisk() failed: expected no error, got: %v", err)
				} else if tc.expErr.Error() != err.Error() {
					t.Fatalf("CreateDisk() failed: expected error %q, got: %q", tc.expErr, err)
				}
			} else {
				if tc.expErr != nil {
					t.Fatal("CreateDisk() failed: expected error, got nothing")
				} else {
					if tc.expDisk.CapacityGiB != disk.CapacityGiB {
						t.Fatalf("CreateDisk() failed: expected capacity %d, got %d", tc.expDisk.CapacityGiB, disk.CapacityGiB)
					}
					if tc.expDisk.VolumeID != disk.VolumeID {
						t.Fatalf("CreateDisk() failed: expected capacity %q, got %q", tc.expDisk.VolumeID, disk.VolumeID)
					}
					if tc.expDisk.AvailabilityZone != disk.AvailabilityZone {
						t.Fatalf("CreateDisk() failed: expected availabilityZone %q, got %q", tc.expDisk.AvailabilityZone, disk.AvailabilityZone)
					}
					if tc.expDisk.OutpostArn != disk.OutpostArn {
						t.Fatalf("CreateDisk() failed: expected outpoustArn %q, got %q", tc.expDisk.OutpostArn, disk.OutpostArn)
					}
				}
			}

			mockCtrl.Finish()
		})
	}
}

// Test client error IdempotentParameterMismatch by forcing it to progress twice.
func TestCreateDiskClientToken(t *testing.T) {
	t.Parallel()

	const volumeName = "test-vol-client-token"
	const volumeID = "vol-abcd1234"
	diskOptions := &DiskOptions{
		CapacityBytes:    util.GiBToBytes(1),
		Tags:             map[string]string{VolumeNameTagKey: volumeName, AwsEbsDriverTagKey: "true"},
		AvailabilityZone: defaultZone,
	}

	// Hash of "test-vol-client-token"
	const expectedClientToken1 = "6a1b29bd7c5c5541d9d6baa2938e954fc5739dc77e97facf23590bd13f8582c2"
	// Hash of "test-vol-client-token-2"
	const expectedClientToken2 = "21465f5586388bb8804d0cec2df13c00f9a975c8cddec4bc35e964cdce59015b"
	// Hash of "test-vol-client-token-3"
	const expectedClientToken3 = "1bee5a79d83981c0041df2c414bb02e0c10aeb49343b63f50f71470edbaa736b"

	mockCtrl := gomock.NewController(t)
	mockEC2 := NewMockEC2API(mockCtrl)
	c := newCloud(mockEC2)

	gomock.InOrder(
		mockEC2.EXPECT().CreateVolume(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(
			func(_ context.Context, input *ec2.CreateVolumeInput, _ ...func(*ec2.Options)) (*ec2.CreateVolumeOutput, error) {
				assert.Equal(t, expectedClientToken1, *input.ClientToken)
				return nil, &smithy.GenericAPIError{Code: "IdempotentParameterMismatch"}
			}),
		mockEC2.EXPECT().CreateVolume(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(
			func(_ context.Context, input *ec2.CreateVolumeInput, _ ...func(*ec2.Options)) (*ec2.CreateVolumeOutput, error) {
				assert.Equal(t, expectedClientToken2, *input.ClientToken)
				return nil, &smithy.GenericAPIError{Code: "IdempotentParameterMismatch"}
			}),
		mockEC2.EXPECT().CreateVolume(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(
			func(_ context.Context, input *ec2.CreateVolumeInput, _ ...func(*ec2.Options)) (*ec2.CreateVolumeOutput, error) {
				assert.Equal(t, expectedClientToken3, *input.ClientToken)
				return &ec2.CreateVolumeOutput{
					VolumeId: aws.String(volumeID),
					Size:     aws.Int32(util.BytesToGiB(diskOptions.CapacityBytes)),
				}, nil
			}),
		mockEC2.EXPECT().DescribeVolumes(gomock.Any(), gomock.Any()).Return(&ec2.DescribeVolumesOutput{
			Volumes: []types.Volume{
				{
					VolumeId:         aws.String(volumeID),
					Size:             aws.Int32(util.BytesToGiB(diskOptions.CapacityBytes)),
					State:            types.VolumeState("available"),
					AvailabilityZone: aws.String(diskOptions.AvailabilityZone),
				},
			},
		}, nil).AnyTimes(),
	)

	ctx, cancel := context.WithDeadline(t.Context(), time.Now().Add(defaultCreateDiskDeadline))
	defer cancel()
	for i := range 3 {
		_, err := c.CreateDisk(ctx, volumeName, diskOptions)
		if i < 2 {
			assert.Error(t, err)
		} else {
			assert.NoError(t, err)
		}
	}
}

func TestDeleteDisk(t *testing.T) {
	testCases := []struct {
		name     string
		volumeID string
		expResp  bool
		expErr   error
	}{
		{
			name:     "success: normal",
			volumeID: "vol-test-1234",
			expResp:  true,
			expErr:   nil,
		},
		{
			name:     "fail: DeleteVolume returned generic error",
			volumeID: "vol-test-1234",
			expResp:  false,
			expErr:   errors.New("DeleteVolume generic error"),
		},
		{
			name:     "fail: DeleteVolume returned not found error",
			volumeID: "vol-test-1234",
			expResp:  false,
			expErr:   errors.New("InvalidVolume.NotFound"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockCtrl := gomock.NewController(t)
			mockEC2 := NewMockEC2API(mockCtrl)
			c := newCloud(mockEC2)

			ctx := t.Context()
			mockEC2.EXPECT().DeleteVolume(gomock.Any(), gomock.Any(), gomock.Any()).Return(&ec2.DeleteVolumeOutput{}, tc.expErr)

			ok, err := c.DeleteDisk(ctx, tc.volumeID)
			if err != nil && tc.expErr == nil {
				t.Fatalf("DeleteDisk() failed: expected no error, got: %v", err)
			}

			if err == nil && tc.expErr != nil {
				t.Fatal("DeleteDisk() failed: expected error, got nothing")
			}

			if tc.expResp != ok {
				t.Fatalf("DeleteDisk() failed: expected return %v, got %v", tc.expResp, ok)
			}

			mockCtrl.Finish()
		})
	}
}

func TestAttachDisk(t *testing.T) {
	blockDeviceInUseErr := &smithy.GenericAPIError{
		Code:    "InvalidParameterValue",
		Message: fmt.Sprintf("Invalid value '%s' for unixDevice. Attachment point %s is already in use", defaultPath, defaultPath),
	}

	testCases := []struct {
		name     string
		volumeID string
		nodeID   string
		nodeID2  string
		path     string
		expErr   error
		mockFunc func(*MockEC2API, context.Context, string, string, string, string, dm.DeviceManager)
	}{
		{
			name:     "success: AttachVolume normal",
			volumeID: defaultVolumeID,
			nodeID:   defaultNodeID,
			path:     defaultPath,
			expErr:   nil,
			mockFunc: func(mockEC2 *MockEC2API, ctx context.Context, volumeID, nodeID, nodeID2, path string, dm dm.DeviceManager) {
				volumeRequest := createVolumeRequest(volumeID)
				instanceRequest := createInstanceRequest(nodeID)
				attachRequest := createAttachRequest(volumeID, nodeID, path)

				gomock.InOrder(
					mockEC2.EXPECT().DescribeInstances(gomock.Any(), gomock.Eq(instanceRequest)).Return(newDescribeInstancesOutput(nodeID), nil),
					mockEC2.EXPECT().AttachVolume(gomock.Any(), gomock.Eq(attachRequest), gomock.Any()).Return(&ec2.AttachVolumeOutput{
						Device:     aws.String(path),
						InstanceId: aws.String(nodeID),
						VolumeId:   aws.String(volumeID),
						State:      types.VolumeAttachmentStateAttaching,
					}, nil),
					mockEC2.EXPECT().DescribeVolumes(gomock.Any(), volumeRequest).Return(createDescribeVolumesOutput([]*string{&volumeID}, nodeID, path, "attached"), nil),
				)
			},
		},
		{
			name:     "success: AttachVolume skip likely bad name",
			volumeID: defaultVolumeID,
			nodeID:   defaultNodeID,
			nodeID2:  defaultNodeID, // Induce second attach
			path:     "/dev/xvdab",
			expErr:   fmt.Errorf("could not attach volume %q to node %q: %w", defaultVolumeID, defaultNodeID, blockDeviceInUseErr),
			mockFunc: func(mockEC2 *MockEC2API, ctx context.Context, volumeID, nodeID, nodeID2, path string, dm dm.DeviceManager) {
				volumeRequest := createVolumeRequest(volumeID)
				instanceRequest := createInstanceRequest(nodeID)
				attachRequest1 := createAttachRequest(volumeID, nodeID, defaultPath)
				attachRequest2 := createAttachRequest(volumeID, nodeID, path)

				gomock.InOrder(
					// First call - fail with "already in use" error
					mockEC2.EXPECT().DescribeInstances(gomock.Any(), gomock.Eq(instanceRequest)).Return(newDescribeInstancesOutput(nodeID), nil),
					mockEC2.EXPECT().AttachVolume(gomock.Any(), gomock.Eq(attachRequest1), gomock.Any()).Return(nil, blockDeviceInUseErr),

					// Second call - succeed, expect bad device name to be skipped
					mockEC2.EXPECT().DescribeInstances(gomock.Any(), gomock.Eq(instanceRequest)).Return(newDescribeInstancesOutput(nodeID), nil),
					mockEC2.EXPECT().AttachVolume(gomock.Any(), gomock.Eq(attachRequest2), gomock.Any()).Return(&ec2.AttachVolumeOutput{
						Device:     aws.String(path),
						InstanceId: aws.String(nodeID),
						VolumeId:   aws.String(volumeID),
						State:      types.VolumeAttachmentStateAttaching,
					}, nil),
					mockEC2.EXPECT().DescribeVolumes(gomock.Any(), volumeRequest).Return(createDescribeVolumesOutput([]*string{&volumeID}, nodeID, path, "attached"), nil),
				)
			},
		},
		{
			name:     "success: AttachVolume device already assigned",
			volumeID: defaultVolumeID,
			nodeID:   defaultNodeID,
			path:     defaultPath,
			expErr:   nil,
			mockFunc: func(mockEC2 *MockEC2API, ctx context.Context, volumeID, nodeID, nodeID2, path string, dm dm.DeviceManager) {
				volumeRequest := createVolumeRequest(volumeID)
				instanceRequest := createInstanceRequest(nodeID)

				fakeInstance := newFakeInstance(nodeID, volumeID, path)
				_, err := dm.NewDevice(&fakeInstance, volumeID, new(sync.Map))
				require.NoError(t, err)

				gomock.InOrder(
					mockEC2.EXPECT().DescribeInstances(gomock.Any(), instanceRequest).Return(newDescribeInstancesOutput(nodeID, volumeID), nil),
					mockEC2.EXPECT().DescribeVolumes(gomock.Any(), volumeRequest).Return(createDescribeVolumesOutput([]*string{&volumeID}, nodeID, path, "attached"), nil))
			},
		},
		{
			name:     "fail: AttachVolume returned generic error",
			volumeID: defaultVolumeID,
			nodeID:   defaultNodeID,
			path:     defaultPath,
			expErr:   fmt.Errorf("could not attach volume %q to node %q: %w", defaultVolumeID, defaultNodeID, errors.New("AttachVolume error")),
			mockFunc: func(mockEC2 *MockEC2API, ctx context.Context, volumeID, nodeID, nodeID2, path string, dm dm.DeviceManager) {
				instanceRequest := createInstanceRequest(nodeID)
				attachRequest := createAttachRequest(volumeID, nodeID, path)

				gomock.InOrder(
					mockEC2.EXPECT().DescribeInstances(gomock.Any(), instanceRequest).Return(newDescribeInstancesOutput(nodeID), nil),
					mockEC2.EXPECT().AttachVolume(gomock.Any(), attachRequest, gomock.Any()).Return(nil, errors.New("AttachVolume error")),
				)
			},
		},
		{
			name:     "fail: AttachVolume returned block device already in use error",
			volumeID: defaultVolumeID,
			nodeID:   defaultNodeID,
			path:     defaultPath,
			expErr:   fmt.Errorf("could not attach volume %q to node %q: %w", defaultVolumeID, defaultNodeID, blockDeviceInUseErr),
			mockFunc: func(mockEC2 *MockEC2API, ctx context.Context, volumeID, nodeID, nodeID2, path string, dm dm.DeviceManager) {
				instanceRequest := createInstanceRequest(nodeID)
				attachRequest := createAttachRequest(volumeID, nodeID, path)

				gomock.InOrder(
					mockEC2.EXPECT().DescribeInstances(ctx, instanceRequest).Return(newDescribeInstancesOutput(nodeID), nil),
					mockEC2.EXPECT().AttachVolume(ctx, attachRequest, gomock.Any()).Return(nil, blockDeviceInUseErr),
				)
			},
		},
		{
			name:     "fail: AttachVolume returned attachment limit exceeded error",
			volumeID: defaultVolumeID,
			nodeID:   defaultNodeID,
			path:     defaultPath,
			expErr: fmt.Errorf("%w: %w", ErrLimitExceeded, &smithy.GenericAPIError{
				Code:    "AttachmentLimitExceeded",
				Message: "Volume attachment limit exceeded",
			}),
			mockFunc: func(mockEC2 *MockEC2API, ctx context.Context, volumeID, nodeID, nodeID2, path string, dm dm.DeviceManager) {
				instanceRequest := createInstanceRequest(nodeID)
				attachRequest := createAttachRequest(volumeID, nodeID, path)
				attachLimitErr := &smithy.GenericAPIError{
					Code:    "AttachmentLimitExceeded",
					Message: "Volume attachment limit exceeded",
				}

				gomock.InOrder(
					mockEC2.EXPECT().DescribeInstances(ctx, instanceRequest).Return(newDescribeInstancesOutput(nodeID), nil),
					mockEC2.EXPECT().AttachVolume(ctx, attachRequest, gomock.Any()).Return(nil, attachLimitErr),
				)
			},
		},

		{
			name:     "success: AttachVolume multi-attach",
			volumeID: defaultVolumeID,
			nodeID:   defaultNodeID,
			nodeID2:  "node-1239",
			path:     defaultPath,
			expErr:   nil,
			mockFunc: func(mockEC2 *MockEC2API, ctx context.Context, volumeID, nodeID, nodeID2, path string, dm dm.DeviceManager) {
				volumeRequest := createVolumeRequest(volumeID)
				instanceRequest := createInstanceRequest(nodeID)
				attachRequest := createAttachRequest(volumeID, nodeID, path)

				createInstanceRequest2 := createInstanceRequest(nodeID2)
				attachRequest2 := createAttachRequest(volumeID, nodeID2, path)

				dvOutput := &ec2.DescribeVolumesOutput{
					Volumes: []types.Volume{
						{
							VolumeId: aws.String(volumeID),
							Attachments: []types.VolumeAttachment{
								{
									Device:     aws.String(path),
									InstanceId: aws.String(nodeID),
									State:      types.VolumeAttachmentStateAttached,
								},
								{
									Device:     aws.String(path),
									InstanceId: aws.String(nodeID2),
									State:      types.VolumeAttachmentStateAttached,
								},
							},
						},
					},
				}

				gomock.InOrder(
					mockEC2.EXPECT().DescribeInstances(ctx, gomock.Eq(instanceRequest)).Return(newDescribeInstancesOutput(nodeID), nil),
					mockEC2.EXPECT().AttachVolume(ctx, gomock.Eq(attachRequest), gomock.Any()).Return(&ec2.AttachVolumeOutput{
						Device:     aws.String(path),
						InstanceId: aws.String(nodeID),
						VolumeId:   aws.String(volumeID),
						State:      types.VolumeAttachmentStateAttaching,
					}, nil),
					mockEC2.EXPECT().DescribeVolumes(ctx, gomock.Eq(volumeRequest)).Return(createDescribeVolumesOutput([]*string{&volumeID}, nodeID, path, "attached"), nil),

					mockEC2.EXPECT().DescribeInstances(ctx, gomock.Eq(createInstanceRequest2)).Return(newDescribeInstancesOutput(nodeID2), nil),
					mockEC2.EXPECT().AttachVolume(ctx, gomock.Eq(attachRequest2), gomock.Any()).Return(&ec2.AttachVolumeOutput{
						Device:     aws.String(path),
						InstanceId: aws.String(nodeID2),
						VolumeId:   aws.String(volumeID),
						State:      types.VolumeAttachmentStateAttaching,
					}, nil),
					mockEC2.EXPECT().DescribeVolumes(ctx, gomock.Eq(volumeRequest)).Return(dvOutput, nil),
				)
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockCtrl := gomock.NewController(t)
			mockEC2 := NewMockEC2API(mockCtrl)
			c := newCloud(mockEC2)
			cloudInstance, ok := c.(*cloud)
			if !ok {
				t.Fatalf("could not assert c as type cloud, %v", c)
			}

			ctx := t.Context()
			deviceManager := cloudInstance.dm

			tc.mockFunc(mockEC2, ctx, tc.volumeID, tc.nodeID, tc.nodeID2, tc.path, deviceManager)

			devicePath, err := c.AttachDisk(ctx, tc.volumeID, tc.nodeID)

			if tc.expErr != nil {
				require.Error(t, err)
				assert.Equal(t, tc.expErr, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tc.path, devicePath)
			}

			if tc.nodeID2 != "" {
				devicePath, err := c.AttachDisk(ctx, tc.volumeID, tc.nodeID2)
				require.NoError(t, err)
				assert.Equal(t, tc.path, devicePath)
			}

			mockCtrl.Finish()
		})
	}

	hyperPodTestCases := []struct {
		name       string
		volumeID   string
		nodeID     string
		setupMocks func(*MockEC2API, *MockSageMakerAPI, string, string)
		expDevice  string
		expErr     error
	}{
		{
			name:     "success: HyperPod AttachVolume normal",
			volumeID: "vol-test",
			nodeID:   "hyperpod-123456789012-i-1234567890",
			setupMocks: func(mockEC2 *MockEC2API, mockSM *MockSageMakerAPI, volumeID, nodeID string) {
				instanceID := getInstanceIDFromHyperPodNode(nodeID)

				// Setup SageMaker mock
				mockSM.EXPECT().AttachClusterNodeVolume(
					gomock.Any(),
					gomock.Any(),
				).Return(&sagemaker.AttachClusterNodeVolumeOutput{
					DeviceName: aws.String("/dev/xvdba"),
					Status:     smtypes.VolumeAttachmentStatusAttached,
				}, nil)

				// Setup EC2 mock for volume state checking
				volumeRequest := createVolumeRequest(volumeID)
				mockEC2.EXPECT().DescribeVolumes(gomock.Any(), volumeRequest).Return(
					createDescribeVolumesOutput([]*string{aws.String(volumeID)}, instanceID, "/dev/xvdba", "attached"),
					nil,
				).AnyTimes()
			},
			expDevice: "/dev/xvdba",
			expErr:    nil,
		},
		{
			name:     "fail: HyperPod attachment limit exceeded",
			volumeID: "vol-test",
			nodeID:   "hyperpod-cluster1-i-1234567890",
			setupMocks: func(mockEC2 *MockEC2API, mockSM *MockSageMakerAPI, volumeID, nodeID string) {
				// Setup SageMaker mock to return error
				mockSM.EXPECT().AttachClusterNodeVolume(
					gomock.Any(),
					gomock.Any(),
				).Return(nil, &smithy.GenericAPIError{
					Code:    "ValidationException",
					Message: "HyperPod - Ec2ErrCode: AttachmentLimitExceeded : Ec2ErrMsg: Volume attachment limit exceeded",
				})

				// Setup EC2 mock for volume state checking
				volumeRequest := createVolumeRequest(volumeID)
				mockEC2.EXPECT().DescribeVolumes(gomock.Any(), volumeRequest).Return(
					createDescribeVolumesOutput([]*string{aws.String(volumeID)}, "", "", "detached"),
					nil,
				).AnyTimes()
			},
			expErr: errors.New("limit exceeded: api error ValidationException: HyperPod - Ec2ErrCode: AttachmentLimitExceeded : Ec2ErrMsg: Volume attachment limit exceeded"),
		},
		{
			name:     "fail: HyperPod attachment generic error",
			volumeID: "vol-test",
			nodeID:   "hyperpod-cluster1-i-1234567890",
			setupMocks: func(mockEC2 *MockEC2API, mockSM *MockSageMakerAPI, volumeID, nodeID string) {
				// Setup SageMaker mock to return error
				mockSM.EXPECT().AttachClusterNodeVolume(
					gomock.Any(),
					gomock.Any(),
				).Return(nil, errors.New("AttachVolume error"))

				// Setup EC2 mock for volume state checking
				volumeRequest := createVolumeRequest(volumeID)
				mockEC2.EXPECT().DescribeVolumes(gomock.Any(), volumeRequest).Return(
					createDescribeVolumesOutput([]*string{aws.String(volumeID)}, "", "", "detached"),
					nil,
				).AnyTimes()
			},
			expErr: errors.New("could not attach volume \"vol-test\" to node \"hyperpod-cluster1-i-1234567890\": AttachVolume error"),
		},
	}

	for _, tc := range hyperPodTestCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create mock EC2 client (needed for base cloud struct)
			mockCtrl := gomock.NewController(t)
			mockEC2 := NewMockEC2API(mockCtrl)

			// Create mock SageMaker client
			mockSM := NewMockSageMakerAPI(mockCtrl)

			// Create cloud with both mocks
			c := &cloud{
				region:    "us-west-2",
				accountID: "123456789012",
				ec2:       mockEC2,
				sm:        mockSM,
				dm:        dm.NewDeviceManager(),
				rm:        newRetryManager(),
				vwp:       testVolumeWaitParameters(),
			}

			tc.setupMocks(mockEC2, mockSM, tc.volumeID, tc.nodeID)

			ctx := t.Context()
			devicePath, err := c.AttachDisk(ctx, tc.volumeID, tc.nodeID)

			// Verify results
			if tc.expErr != nil {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.expErr.Error())
			} else {
				require.NoError(t, err)
				assert.Equal(t, tc.expDevice, devicePath)
			}

			mockCtrl.Finish()
		})
	}
}

func TestDetachDisk(t *testing.T) {
	testCases := []struct {
		name     string
		volumeID string
		nodeID   string
		expErr   error
		mockFunc func(*MockEC2API, context.Context, string, string)
	}{
		{
			name:     "success: DetachDisk normal",
			volumeID: "vol-test-1234",
			nodeID:   "node-1234",
			expErr:   nil,
			mockFunc: func(mockEC2 *MockEC2API, ctx context.Context, volumeID, nodeID string) {
				volumeRequest := createVolumeRequest(volumeID)
				instanceRequest := createInstanceRequest(nodeID)
				detachRequest := createDetachRequest(volumeID, nodeID)

				gomock.InOrder(
					mockEC2.EXPECT().DescribeInstances(gomock.Any(), instanceRequest).Return(newDescribeInstancesOutput(nodeID), nil),
					mockEC2.EXPECT().DetachVolume(gomock.Any(), detachRequest, gomock.Any()).Return(nil, nil),
					mockEC2.EXPECT().DescribeVolumes(gomock.Any(), volumeRequest).Return(createDescribeVolumesOutput([]*string{&volumeID}, nodeID, "", "detached"), nil),
				)
			},
		},
		{
			name:     "fail: DetachVolume returned generic error",
			volumeID: "vol-test-1234",
			nodeID:   "node-1234",
			expErr:   fmt.Errorf("could not detach volume %q from node %q: %w", defaultVolumeID, defaultNodeID, errors.New("DetachVolume error")),
			mockFunc: func(mockEC2 *MockEC2API, ctx context.Context, volumeID, nodeID string) {
				instanceRequest := createInstanceRequest(nodeID)
				detachRequest := createDetachRequest(volumeID, nodeID)

				gomock.InOrder(
					mockEC2.EXPECT().DescribeInstances(gomock.Any(), instanceRequest).Return(newDescribeInstancesOutput(nodeID), nil),
					mockEC2.EXPECT().DetachVolume(gomock.Any(), detachRequest, gomock.Any()).Return(nil, errors.New("DetachVolume error")),
				)
			},
		},
		{
			name:     "fail: DetachVolume returned not found error",
			volumeID: "vol-test-1234",
			nodeID:   "node-1234",
			expErr:   fmt.Errorf("could not detach volume %q from node %q: %w", defaultVolumeID, defaultNodeID, ErrNotFound),
			mockFunc: func(mockEC2 *MockEC2API, ctx context.Context, volumeID, nodeID string) {
				instanceRequest := createInstanceRequest(nodeID)
				detachRequest := createDetachRequest(volumeID, nodeID)

				gomock.InOrder(
					mockEC2.EXPECT().DescribeInstances(gomock.Any(), instanceRequest).Return(newDescribeInstancesOutput(nodeID), nil),
					mockEC2.EXPECT().DetachVolume(gomock.Any(), detachRequest, gomock.Any()).Return(nil, ErrNotFound),
				)
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockCtrl := gomock.NewController(t)
			mockEC2 := NewMockEC2API(mockCtrl)
			c := newCloud(mockEC2)

			ctx := t.Context()
			tc.mockFunc(mockEC2, ctx, tc.volumeID, tc.nodeID)

			err := c.DetachDisk(ctx, tc.volumeID, tc.nodeID)

			if tc.expErr != nil {
				require.Error(t, err)
				assert.Equal(t, tc.expErr, err)
			} else {
				require.NoError(t, err)
			}

			mockCtrl.Finish()
		})
	}

	hyperPodTestCases := []struct {
		name     string
		volumeID string
		nodeID   string
		mockFunc func(*MockEC2API, *MockSageMakerAPI, string, string)
		expErr   error
	}{
		{
			name:     "success: HyperPod DetachVolume normal",
			volumeID: "vol-test",
			nodeID:   "hyperpod-cluster1-i-1234567890",
			mockFunc: func(mockEC2 *MockEC2API, mockSM *MockSageMakerAPI, volumeID, nodeID string) {
				instanceID := getInstanceIDFromHyperPodNode(nodeID)

				// Setup SageMaker mock
				mockSM.EXPECT().DetachClusterNodeVolume(
					gomock.Any(),
					gomock.Any(),
				).Return(&sagemaker.DetachClusterNodeVolumeOutput{
					Status: smtypes.VolumeAttachmentStatusDetached,
				}, nil)

				// Setup EC2 mock for volume state checking
				volumeRequest := createVolumeRequest(volumeID)
				mockEC2.EXPECT().DescribeVolumes(gomock.Any(), volumeRequest).Return(
					createDescribeVolumesOutput([]*string{aws.String(volumeID)}, instanceID, "", "detached"),
					nil,
				).AnyTimes()
			},
			expErr: nil,
		},
		{
			name:     "fail: HyperPod DetachVolume returned generic error",
			volumeID: "vol-test",
			nodeID:   "hyperpod-cluster1-i-1234567890",
			mockFunc: func(mockEC2 *MockEC2API, mockSM *MockSageMakerAPI, volumeID, nodeID string) {
				// Setup SageMaker mock to return error
				mockSM.EXPECT().DetachClusterNodeVolume(
					gomock.Any(),
					gomock.Any(),
				).Return(nil, errors.New("DetachVolume error"))

				// Setup EC2 mock for volume state checking
				volumeRequest := createVolumeRequest(volumeID)
				mockEC2.EXPECT().DescribeVolumes(gomock.Any(), volumeRequest).Return(
					createDescribeVolumesOutput([]*string{aws.String(volumeID)}, "", "", "detached"),
					nil,
				).AnyTimes()
			},
			expErr: errors.New("could not detach volume \"vol-test\" from node \"hyperpod-cluster1-i-1234567890\": DetachVolume error"),
		},
		{
			name:     "fail: HyperPod DetachVolume returned IncorrectState error",
			volumeID: "vol-test",
			nodeID:   "hyperpod-cluster1-i-1234567890",
			mockFunc: func(mockEC2 *MockEC2API, mockSM *MockSageMakerAPI, volumeID, nodeID string) {
				// Setup SageMaker mock to return error
				mockSM.EXPECT().DetachClusterNodeVolume(
					gomock.Any(),
					gomock.Any(),
				).Return(nil, &smithy.GenericAPIError{
					Code:    "ValidationException",
					Message: "HyperPod - Ec2ErrCode: IncorrectState : Ec2ErrMsg: State is not correct",
				})
			},
			expErr: ErrNotFound,
		},
		{
			name:     "fail: HyperPod DetachVolume returned InvalidAttachment.NotFound error",
			volumeID: "vol-test",
			nodeID:   "hyperpod-cluster1-i-1234567890",
			mockFunc: func(mockEC2 *MockEC2API, mockSM *MockSageMakerAPI, volumeID, nodeID string) {
				// Setup SageMaker mock to return error
				mockSM.EXPECT().DetachClusterNodeVolume(
					gomock.Any(),
					gomock.Any(),
				).Return(nil, &smithy.GenericAPIError{
					Code:    "ValidationException",
					Message: "HyperPod - Ec2ErrCode: InvalidAttachment.NotFound : Ec2ErrMsg: Attachment not found",
				})
			},
			expErr: ErrNotFound,
		},
		{
			name:     "fail: HyperPod DetachVolume returned InvalidVolume.NotFound error",
			volumeID: "vol-test",
			nodeID:   "hyperpod-cluster1-i-1234567890",
			mockFunc: func(mockEC2 *MockEC2API, mockSM *MockSageMakerAPI, volumeID, nodeID string) {
				// Setup SageMaker mock to return error
				mockSM.EXPECT().DetachClusterNodeVolume(
					gomock.Any(),
					gomock.Any(),
				).Return(nil, &smithy.GenericAPIError{
					Code:    "ValidationException",
					Message: "HyperPod - Ec2ErrCode: InvalidVolume.NotFound : Ec2ErrMsg: Volume not found",
				})
			},
			expErr: ErrNotFound,
		},
		{
			name:     "fail: HyperPod detachment timeout",
			volumeID: "vol-test",
			nodeID:   "hyperpod-cluster1-i-1234567890",
			mockFunc: func(mockEC2 *MockEC2API, mockSM *MockSageMakerAPI, volumeID, nodeID string) {
				// Setup SageMaker mock
				mockSM.EXPECT().DetachClusterNodeVolume(
					gomock.Any(),
					gomock.Any(),
				).Return(&sagemaker.DetachClusterNodeVolumeOutput{
					Status: smtypes.VolumeAttachmentStatusDetaching,
				}, nil)

				// Setup EC2 mock to simulate timeout by always returning "attached"
				volumeRequest := createVolumeRequest(volumeID)
				mockEC2.EXPECT().DescribeVolumes(gomock.Any(), volumeRequest).Return(
					createDescribeVolumesOutput([]*string{aws.String(volumeID)}, "", "", "attached"),
					nil,
				).AnyTimes()
			},
			expErr: errors.New("error waiting for volume detachment: timed out waiting for the condition"),
		},
	}

	// Run HyperPod test cases
	for _, tc := range hyperPodTestCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create mock controllers
			mockCtrl := gomock.NewController(t)
			defer mockCtrl.Finish()

			// Create mock clients
			mockEC2 := NewMockEC2API(mockCtrl)
			mockSM := NewMockSageMakerAPI(mockCtrl)

			// Setup mocks
			tc.mockFunc(mockEC2, mockSM, tc.volumeID, tc.nodeID)

			// Create cloud with mocks
			c := &cloud{
				region:    "us-west-2",
				accountID: "123456789012",
				ec2:       mockEC2,
				sm:        mockSM,
				dm:        dm.NewDeviceManager(),
				rm:        newRetryManager(),
				vwp:       testVolumeWaitParameters(),
			}

			ctx := t.Context()
			err := c.DetachDisk(ctx, tc.volumeID, tc.nodeID)

			// Verify results
			if tc.expErr != nil {
				require.Error(t, err)
				assert.Equal(t, tc.expErr.Error(), err.Error())
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestGetDiskByName(t *testing.T) {
	testCases := []struct {
		name             string
		volumeName       string
		volumeCapacity   int64
		availabilityZone string
		outpostArn       string
		expErr           error
	}{
		{
			name:             "success: normal",
			volumeName:       "vol-test-1234",
			volumeCapacity:   util.GiBToBytes(1),
			availabilityZone: expZone,
			expErr:           nil,
		},
		{
			name:             "success: outpost volume",
			volumeName:       "vol-test-1234",
			volumeCapacity:   util.GiBToBytes(1),
			availabilityZone: expZone,
			outpostArn:       "arn:aws:outposts:us-west-2:111111111111:outpost/op-0aaa000a0aaaa00a0",
			expErr:           nil,
		},
		{
			name:           "fail: DescribeVolumes returned generic error",
			volumeName:     "vol-test-1234",
			volumeCapacity: util.GiBToBytes(1),
			expErr:         errors.New("DescribeVolumes generic error"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockCtrl := gomock.NewController(t)
			mockEC2 := NewMockEC2API(mockCtrl)
			c := newCloud(mockEC2)

			vol := types.Volume{
				VolumeId:         aws.String(tc.volumeName),
				Size:             aws.Int32(util.BytesToGiB(tc.volumeCapacity)),
				AvailabilityZone: aws.String(tc.availabilityZone),
				OutpostArn:       aws.String(tc.outpostArn),
				Tags: []types.Tag{
					{
						Key:   aws.String(VolumeNameTagKey),
						Value: aws.String(tc.volumeName),
					},
				},
			}

			ctx := t.Context()
			mockEC2.EXPECT().DescribeVolumes(gomock.Any(), gomock.Any()).Return(&ec2.DescribeVolumesOutput{Volumes: []types.Volume{vol}}, tc.expErr)

			disk, err := c.GetDiskByName(ctx, tc.volumeName, tc.volumeCapacity)
			if err != nil {
				if tc.expErr == nil {
					t.Fatalf("GetDiskByName() failed: expected no error, got: %v", err)
				}
			} else {
				if tc.expErr != nil {
					t.Fatal("GetDiskByName() failed: expected error, got nothing")
				}
				if disk.CapacityGiB != util.BytesToGiB(tc.volumeCapacity) {
					t.Fatalf("GetDiskByName() failed: expected capacity %d, got %d", util.BytesToGiB(tc.volumeCapacity), disk.CapacityGiB)
				}
				if tc.availabilityZone != disk.AvailabilityZone {
					t.Fatalf("GetDiskByName() failed: expected availabilityZone %q, got %q", tc.availabilityZone, disk.AvailabilityZone)
				}
				if tc.outpostArn != disk.OutpostArn {
					t.Fatalf("GetDiskByName() failed: expected outpostArn %q, got %q", tc.outpostArn, disk.OutpostArn)
				}
			}

			mockCtrl.Finish()
		})
	}
}

func TestGetDiskByID(t *testing.T) {
	testCases := []struct {
		name             string
		volumeID         string
		availabilityZone string
		outpostArn       string
		attachments      []types.VolumeAttachment
		expDisk          *Disk
		expErr           error
	}{
		{
			name:             "success: normal",
			volumeID:         "vol-test-1234",
			availabilityZone: expZone,
			attachments:      []types.VolumeAttachment{},
			expDisk: &Disk{
				VolumeID:         "vol-test-1234",
				AvailabilityZone: expZone,
			},
			expErr: nil,
		},
		{
			name:             "success: outpost volume",
			volumeID:         "vol-test-1234",
			availabilityZone: expZone,
			outpostArn:       "arn:aws:outposts:us-west-2:111111111111:outpost/op-0aaa000a0aaaa00a0",
			attachments:      []types.VolumeAttachment{},
			expDisk: &Disk{
				VolumeID:         "vol-test-1234",
				AvailabilityZone: expZone,
				OutpostArn:       "arn:aws:outposts:us-west-2:111111111111:outpost/op-0aaa000a0aaaa00a0",
			},
			expErr: nil,
		},
		{
			name:             "success: attached instance list",
			volumeID:         "vol-test-1234",
			availabilityZone: expZone,
			outpostArn:       "arn:aws:outposts:us-west-2:111111111111:outpost/op-0aaa000a0aaaa00a0",
			attachments: []types.VolumeAttachment{
				{
					InstanceId: aws.String("test-instance"),
					State:      types.VolumeAttachmentStateAttached,
				},
			},
			expDisk: &Disk{
				VolumeID:         "vol-test-1234",
				AvailabilityZone: expZone,
				OutpostArn:       "arn:aws:outposts:us-west-2:111111111111:outpost/op-0aaa000a0aaaa00a0",
				Attachments:      []string{"test-instance"},
			},
			expErr: nil,
		},
		{
			name:     "fail: DescribeVolumes returned generic error",
			volumeID: "vol-test-1234",
			expErr:   errors.New("DescribeVolumes generic error"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockCtrl := gomock.NewController(t)
			mockEC2 := NewMockEC2API(mockCtrl)
			c := newCloud(mockEC2)

			ctx := t.Context()

			mockEC2.EXPECT().DescribeVolumes(gomock.Any(), gomock.Any()).Return(
				&ec2.DescribeVolumesOutput{
					Volumes: []types.Volume{
						{
							VolumeId:         aws.String(tc.volumeID),
							AvailabilityZone: aws.String(tc.availabilityZone),
							OutpostArn:       aws.String(tc.outpostArn),
							Attachments:      tc.attachments,
						},
					},
				},
				tc.expErr,
			)

			disk, err := c.GetDiskByID(ctx, tc.volumeID)
			if err != nil {
				if tc.expErr == nil {
					t.Fatalf("GetDiskByID() failed: expected no error, got: %v", err)
				}
				if err.Error() != tc.expErr.Error() {
					t.Fatalf("GetDiskByID() failed: expected error %q, got %q", tc.expErr, err)
				}
			} else {
				if tc.expErr != nil {
					t.Fatal("GetDiskByID() failed: expected error, got nothing")
				}
				if disk.VolumeID != tc.expDisk.VolumeID {
					t.Fatalf("GetDiskByID() failed: expected volume ID %q, got %q", tc.expDisk.VolumeID, disk.VolumeID)
				}
				if disk.AvailabilityZone != tc.expDisk.AvailabilityZone {
					t.Fatalf("GetDiskByID() failed: expected availability zone %q, got %q", tc.expDisk.AvailabilityZone, disk.AvailabilityZone)
				}
				if disk.OutpostArn != tc.expDisk.OutpostArn {
					t.Fatalf("GetDiskByID() failed: expected outpost ARN %q, got %q", tc.expDisk.OutpostArn, disk.OutpostArn)
				}
				if len(disk.Attachments) != len(tc.expDisk.Attachments) {
					t.Fatalf("GetDiskByID() failed: expected attachments length %d, got %d", len(tc.expDisk.Attachments), len(disk.Attachments))
				}
			}

			mockCtrl.Finish()
		})
	}
}

func TestIsHyperPodNode(t *testing.T) {
	tests := []struct {
		name     string
		nodeID   string
		expected bool
	}{
		{
			name:     "success: valid hyperpod node ID",
			nodeID:   "hyperpod-abc123-i-0123456789abcdef0",
			expected: true,
		},
		{
			name:     "success: regular EC2 instance ID",
			nodeID:   "i-0123456789abcdef0",
			expected: false,
		},
		{
			name:     "success: empty string",
			nodeID:   "",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isHyperPodNode(tt.nodeID); got != tt.expected {
				t.Errorf("isHyperPodNode() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestGetInstanceIDFromHyperPodNode(t *testing.T) {
	tests := []struct {
		name   string
		nodeID string
		expID  string
	}{
		{
			name:   "success: valid hyperpod node ID",
			nodeID: "hyperpod-abc123-i-0123456789abcdef0",
			expID:  "i-0123456789abcdef0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getInstanceIDFromHyperPodNode(tt.nodeID)
			assert.Equal(t, tt.expID, result)
		})
	}
}

func TestBuildHyperPodClusterArn(t *testing.T) {
	testCases := []struct {
		name        string
		nodeID      string
		region      string
		accountID   string
		expectedArn string
	}{
		{
			name:        "success: valid HyperPod node",
			nodeID:      "hyperpod-abc123-i-1234567890abcdef0",
			region:      "test-region",
			accountID:   "123456789012",
			expectedArn: "arn:aws:sagemaker:test-region:123456789012:cluster/abc123",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockCtrl := gomock.NewController(t)
			defer mockCtrl.Finish()

			result := buildHyperPodClusterArn(tc.nodeID, tc.region, tc.accountID)
			assert.Equal(t, tc.expectedArn, result)
		})
	}
}

func TestGetInstanceIDFromAssociatedResource(t *testing.T) {
	tests := []struct {
		name        string
		arn         string
		expectedID  string
		expectError bool
	}{
		{
			name:       "valid ARN",
			arn:        "arn:aws:sagemaker:us-west-2:123456789012:cluster/cluster1-i-1234567890abcdef0",
			expectedID: "i-1234567890abcdef0",
		},
		{
			name:        "invalid ARN format - too few parts",
			arn:         "invalid",
			expectError: true,
		},
		{
			name:        "invalid instance ID format",
			arn:         "arn:aws:sagemaker:us-west-2:123456789012:cluster/cluster1-invalid-id",
			expectError: true,
		},
		{
			name:        "empty ARN",
			arn:         "",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := getInstanceIDFromAssociatedResource(tt.arn)
			if (err != nil) != tt.expectError {
				t.Errorf("getInstanceIDFromAssociatedResource() error = %v, expectError %v", err, tt.expectError)
				return
			}
			if got != tt.expectedID {
				t.Errorf("getInstanceIDFromAssociatedResource() = %v, expectedID %v", got, tt.expectedID)
			}
		})
	}
}

func TestCreateSnapshot(t *testing.T) {
	testCases := []struct {
		name            string
		snapshotName    string
		snapshotOptions *SnapshotOptions
		expSnapshot     *Snapshot
		expErr          error
	}{
		{
			name:         "success: normal",
			snapshotName: "snap-test-name",
			snapshotOptions: &SnapshotOptions{
				Tags: map[string]string{
					SnapshotNameTagKey: "snap-test-name",
					AwsEbsDriverTagKey: "true",
					"extra-tag-key":    "extra-tag-value",
				},
			},
			expSnapshot: &Snapshot{
				SnapshotID:     "snap-test-name",
				SourceVolumeID: "snap-test-volume",
				Size:           10,
				ReadyToUse:     true,
			},
			expErr: nil,
		},
		{
			name:         "success: outpost",
			snapshotName: "snap-test-name",
			snapshotOptions: &SnapshotOptions{
				Tags: map[string]string{
					SnapshotNameTagKey: "snap-test-name",
					AwsEbsDriverTagKey: "true",
					"extra-tag-key":    "extra-tag-value",
				},
				OutpostArn: "arn:aws:outposts:us-east-1:222222222222:outpost/aa-aaaaaaaaaaaaaaaaa",
			},
			expSnapshot: &Snapshot{
				SnapshotID:     "snap-test-name",
				SourceVolumeID: "snap-test-volume",
				Size:           10,
				ReadyToUse:     true,
			},
			expErr: nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockCtrl := gomock.NewController(t)
			mockEC2 := NewMockEC2API(mockCtrl)
			c := newCloud(mockEC2)

			ctx := t.Context()

			mockEC2.EXPECT().CreateSnapshot(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(
				func(ctx context.Context, input *ec2.CreateSnapshotInput, optFns ...func(*ec2.Options)) (*ec2.CreateSnapshotOutput, error) {
					if input.VolumeId == nil || *input.VolumeId != tc.expSnapshot.SourceVolumeID {
						t.Errorf("Unexpected VolumeId. Expected: %s, Actual: %s", tc.expSnapshot.SourceVolumeID, aws.ToString(input.VolumeId))
					}
					if input.Description == nil || *input.Description != "Created by AWS EBS CSI driver for volume "+tc.expSnapshot.SourceVolumeID {
						t.Errorf("Unexpected Description. Expected: %s, Actual: %s", "Created by AWS EBS CSI driver for volume "+tc.expSnapshot.SourceVolumeID, aws.ToString(input.Description))
					}
					if len(input.TagSpecifications) != 1 {
						t.Errorf("Unexpected number of TagSpecifications. Expected: 1, Actual: %d", len(input.TagSpecifications))
					} else {
						for expectedTagKey, expectedTagValue := range tc.snapshotOptions.Tags {
							found := false
							for _, actualTag := range input.TagSpecifications[0].Tags {
								if aws.ToString(actualTag.Key) == expectedTagKey && aws.ToString(actualTag.Value) == expectedTagValue {
									found = true
									break
								}
							}
							if !found {
								t.Errorf("Expected tag not found. Key: %s, Value: %s", expectedTagKey, expectedTagValue)
							}
						}
					}
					return &ec2.CreateSnapshotOutput{
						SnapshotId: &tc.expSnapshot.SnapshotID,
						VolumeId:   &tc.expSnapshot.SourceVolumeID,
						VolumeSize: aws.Int32(tc.expSnapshot.Size),
						StartTime:  aws.Time(tc.expSnapshot.CreationTime),
						State:      types.SnapshotStateCompleted,
					}, tc.expErr
				},
			)

			snapshot, err := c.CreateSnapshot(ctx, tc.expSnapshot.SourceVolumeID, tc.snapshotOptions)
			if err != nil {
				if tc.expErr == nil {
					t.Fatalf("CreateSnapshot() failed: expected no error, got: %v", err)
				}
				if err.Error() != tc.expErr.Error() {
					t.Fatalf("CreateSnapshot() failed: expected error %q, got %q", tc.expErr, err)
				}
			} else {
				if tc.expErr != nil {
					t.Fatal("CreateSnapshot() failed: expected error, got nothing")
				}
				if snapshot.SnapshotID != tc.expSnapshot.SnapshotID {
					t.Fatalf("CreateSnapshot() failed: expected snapshot ID %q, got %q", tc.expSnapshot.SnapshotID, snapshot.SnapshotID)
				}
				if snapshot.SourceVolumeID != tc.expSnapshot.SourceVolumeID {
					t.Fatalf("CreateSnapshot() failed: expected source volume ID %q, got %q", tc.expSnapshot.SourceVolumeID, snapshot.SourceVolumeID)
				}
				if snapshot.Size != tc.expSnapshot.Size {
					t.Fatalf("CreateSnapshot() failed: expected size %d, got %d", tc.expSnapshot.Size, snapshot.Size)
				}
				if snapshot.ReadyToUse != tc.expSnapshot.ReadyToUse {
					t.Fatalf("CreateSnapshot() failed: expected ready to use %t, got %t", tc.expSnapshot.ReadyToUse, snapshot.ReadyToUse)
				}
			}
			mockCtrl.Finish()
		})
	}
}

func TestEnableFastSnapshotRestores(t *testing.T) {
	testCases := []struct {
		name              string
		snapshotID        string
		availabilityZones []string
		expOutput         *ec2.EnableFastSnapshotRestoresOutput
		expErr            error
	}{
		{
			name:              "success: normal",
			snapshotID:        "snap-test-id",
			availabilityZones: []string{"us-west-2a", "us-west-2b"},
			expOutput: &ec2.EnableFastSnapshotRestoresOutput{
				Successful: []types.EnableFastSnapshotRestoreSuccessItem{{
					AvailabilityZone: aws.String("us-west-2a,us-west-2b"),
					SnapshotId:       aws.String("snap-test-id")}},
				Unsuccessful: []types.EnableFastSnapshotRestoreErrorItem{},
			},
			expErr: nil,
		},
		{
			name:              "fail: unsuccessful response",
			snapshotID:        "snap-test-id",
			availabilityZones: []string{"us-west-2a", "invalid-zone"},
			expOutput: &ec2.EnableFastSnapshotRestoresOutput{
				Unsuccessful: []types.EnableFastSnapshotRestoreErrorItem{{
					SnapshotId: aws.String("snap-test-id"),
					FastSnapshotRestoreStateErrors: []types.EnableFastSnapshotRestoreStateErrorItem{
						{AvailabilityZone: aws.String("us-west-2a,invalid-zone"),
							Error: &types.EnableFastSnapshotRestoreStateError{
								Message: aws.String("failed to create fast snapshot restore")}},
					},
				}},
			},
			expErr: errors.New("failed to create fast snapshot restores for snapshot"),
		},
		{
			name:              "fail: error",
			snapshotID:        "",
			availabilityZones: nil,
			expOutput:         nil,
			expErr:            errors.New("EnableFastSnapshotRestores error"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockCtrl := gomock.NewController(t)
			mockEC2 := NewMockEC2API(mockCtrl)
			c := newCloud(mockEC2)

			ctx := t.Context()
			mockEC2.EXPECT().EnableFastSnapshotRestores(gomock.Any(), gomock.Any(), gomock.Any()).Return(tc.expOutput, tc.expErr).AnyTimes()

			response, err := c.EnableFastSnapshotRestores(ctx, tc.availabilityZones, tc.snapshotID)

			if err != nil {
				if tc.expErr == nil {
					t.Fatalf("EnableFastSnapshotRestores() failed: expected no error, got: %v", err)
				}
				if err.Error() != tc.expErr.Error() {
					t.Fatalf("EnableFastSnapshotRestores() failed: expected error %v, got %v", tc.expErr, err)
				}
			} else {
				if tc.expErr != nil {
					t.Fatalf("EnableFastSnapshotRestores() failed: expected error %v, got nothing", tc.expErr)
				}
				if len(response.Successful) == 0 || len(response.Unsuccessful) > 0 {
					t.Fatalf("EnableFastSnapshotRestores() failed: expected successful response, got %v", response)
				}
				if *response.Successful[0].SnapshotId != tc.snapshotID {
					t.Fatalf("EnableFastSnapshotRestores() failed: expected successful response to have SnapshotId %s, got %s", tc.snapshotID, *response.Successful[0].SnapshotId)
				}
				az := strings.Split(*response.Successful[0].AvailabilityZone, ",")
				if !reflect.DeepEqual(az, tc.availabilityZones) {
					t.Fatalf("EnableFastSnapshotRestores() failed: expected successful response to have AvailabilityZone %v, got %v", az, tc.availabilityZones)
				}
			}

			mockCtrl.Finish()
		})
	}
}

func TestAvailabilityZones(t *testing.T) {
	testCases := []struct {
		name             string
		availabilityZone string
		expOutput        *ec2.DescribeAvailabilityZonesOutput
		expErr           error
	}{
		{
			name:             "success: normal",
			availabilityZone: expZone,
			expOutput: &ec2.DescribeAvailabilityZonesOutput{
				AvailabilityZones: []types.AvailabilityZone{
					{ZoneName: aws.String(expZone)},
				}},
			expErr: nil,
		},
		{
			name:             "fail: error",
			availabilityZone: "",
			expOutput:        nil,
			expErr:           errors.New("TestAvailabilityZones error"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockCtrl := gomock.NewController(t)
			mockEC2 := NewMockEC2API(mockCtrl)
			c := newCloud(mockEC2)

			ctx := t.Context()
			mockEC2.EXPECT().DescribeAvailabilityZones(gomock.Any(), gomock.Any()).Return(tc.expOutput, tc.expErr).AnyTimes()

			az, err := c.AvailabilityZones(ctx)
			if err != nil {
				if tc.expErr == nil {
					t.Fatalf("AvailabilityZones() failed: expected no error, got: %v", err)
				}
			} else {
				if tc.expErr != nil {
					t.Fatalf("AvailabilityZones() failed: expected error, got nothing")
				}
				if val, ok := az[tc.availabilityZone]; !ok {
					t.Fatalf("AvailabilityZones() failed: expected to find %s, got %v", tc.availabilityZone, val)
				}
			}

			mockCtrl.Finish()
		})
	}
}

func TestDeleteSnapshot(t *testing.T) {
	testCases := []struct {
		name         string
		snapshotName string
		expErr       error
	}{
		{
			name:         "success: normal",
			snapshotName: "snap-test-name",
			expErr:       nil,
		},
		{
			name:         "fail: delete snapshot return generic error",
			snapshotName: "snap-test-name",
			expErr:       errors.New("DeleteSnapshot generic error"),
		},
		{
			name:         "fail: delete snapshot return not found error",
			snapshotName: "snap-test-name",
			expErr: &smithy.GenericAPIError{
				Code:    "InvalidSnapshot.NotFound",
				Message: "",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockCtrl := gomock.NewController(t)
			mockEC2 := NewMockEC2API(mockCtrl)
			c := newCloud(mockEC2)

			ctx := t.Context()
			mockEC2.EXPECT().DeleteSnapshot(gomock.Any(), gomock.Any(), gomock.Any()).Return(&ec2.DeleteSnapshotOutput{}, tc.expErr)

			_, err := c.DeleteSnapshot(ctx, tc.snapshotName)
			if err != nil {
				if tc.expErr == nil {
					t.Fatalf("DeleteSnapshot() failed: expected no error, got: %v", err)
				}
			} else {
				if tc.expErr != nil {
					t.Fatal("DeleteSnapshot() failed: expected error, got nothing")
				}
			}

			mockCtrl.Finish()
		})
	}
}

func TestResizeOrModifyDisk(t *testing.T) {
	testCases := []struct {
		name                string
		volumeID            string
		existingVolume      *types.Volume
		existingVolumeError error
		modifiedVolume      *ec2.ModifyVolumeOutput
		modifiedVolumeError error
		descModVolume       *ec2.DescribeVolumesModificationsOutput
		reqSizeGiB          int32
		modifyDiskOptions   *ModifyDiskOptions
		expErr              error
		shouldCallDescribe  bool
	}{
		{
			name:     "success: normal resize",
			volumeID: "vol-test",
			existingVolume: &types.Volume{
				VolumeId:         aws.String("vol-test"),
				Size:             aws.Int32(1),
				AvailabilityZone: aws.String(defaultZone),
			},
			modifiedVolume: &ec2.ModifyVolumeOutput{
				VolumeModification: &types.VolumeModification{
					VolumeId:          aws.String("vol-test"),
					TargetSize:        aws.Int32(2),
					ModificationState: types.VolumeModificationStateCompleted,
				},
			},
			reqSizeGiB:         2,
			modifyDiskOptions:  &ModifyDiskOptions{},
			expErr:             nil,
			shouldCallDescribe: true,
		},
		{
			name:     "success: normal modifying state",
			volumeID: "vol-test",
			existingVolume: &types.Volume{
				VolumeId:         aws.String("vol-test"),
				Size:             aws.Int32(1),
				AvailabilityZone: aws.String(defaultZone),
			},
			modifiedVolume: &ec2.ModifyVolumeOutput{
				VolumeModification: &types.VolumeModification{
					VolumeId:          aws.String("vol-test"),
					TargetSize:        aws.Int32(2),
					ModificationState: types.VolumeModificationStateModifying,
				},
			},
			descModVolume: &ec2.DescribeVolumesModificationsOutput{
				VolumesModifications: []types.VolumeModification{
					{
						VolumeId:          aws.String("vol-test"),
						TargetSize:        aws.Int32(2),
						ModificationState: types.VolumeModificationStateCompleted,
					},
				},
			},
			reqSizeGiB:         2,
			modifyDiskOptions:  &ModifyDiskOptions{},
			expErr:             nil,
			shouldCallDescribe: true,
		},
		{
			name:     "success: with previous expansion",
			volumeID: "vol-test",
			existingVolume: &types.Volume{
				VolumeId:         aws.String("vol-test"),
				Size:             aws.Int32(2),
				AvailabilityZone: aws.String(defaultZone),
			},
			descModVolume: &ec2.DescribeVolumesModificationsOutput{
				VolumesModifications: []types.VolumeModification{
					{
						VolumeId:          aws.String("vol-test"),
						TargetSize:        aws.Int32(2),
						ModificationState: types.VolumeModificationStateCompleted,
					},
				},
			},
			reqSizeGiB:         2,
			modifyDiskOptions:  &ModifyDiskOptions{},
			expErr:             nil,
			shouldCallDescribe: true,
		},
		{
			name:     "success: modify IOPS, throughput and volume type",
			volumeID: "vol-test",
			existingVolume: &types.Volume{
				VolumeId:   aws.String("vol-test"),
				VolumeType: types.VolumeTypeGp2,
				Size:       aws.Int32(1),
			},
			modifyDiskOptions: &ModifyDiskOptions{
				VolumeType: "GP3",
				IOPS:       3000,
				Throughput: 1000,
			},
			modifiedVolume: &ec2.ModifyVolumeOutput{
				VolumeModification: &types.VolumeModification{
					VolumeId:          aws.String("vol-test"),
					TargetVolumeType:  types.VolumeTypeGp3,
					TargetIops:        aws.Int32(3000),
					TargetThroughput:  aws.Int32(1000),
					ModificationState: types.VolumeModificationStateCompleted,
				},
			},
			reqSizeGiB:         1,
			expErr:             nil,
			shouldCallDescribe: true,
		},
		{
			name:     "success: modify size, IOPS, throughput and volume type",
			volumeID: "vol-test",
			existingVolume: &types.Volume{
				VolumeId:         aws.String("vol-test"),
				Size:             aws.Int32(1),
				AvailabilityZone: aws.String(defaultZone),
				VolumeType:       types.VolumeTypeGp2,
				Iops:             aws.Int32(2000),
			},
			modifyDiskOptions: &ModifyDiskOptions{
				VolumeType: "GP3",
				IOPS:       3000,
				Throughput: 1000,
			},
			reqSizeGiB: 2,
			modifiedVolume: &ec2.ModifyVolumeOutput{
				VolumeModification: &types.VolumeModification{
					VolumeId:          aws.String("vol-test"),
					TargetSize:        aws.Int32(2),
					TargetVolumeType:  types.VolumeTypeGp3,
					TargetIops:        aws.Int32(3000),
					TargetThroughput:  aws.Int32(1000),
					ModificationState: types.VolumeModificationStateCompleted,
				},
			},
			expErr:             nil,
			shouldCallDescribe: true,
		},
		{
			name:                "fail: volume doesn't exist",
			volumeID:            "vol-test",
			existingVolume:      &types.Volume{},
			existingVolumeError: errors.New("DescribeVolumes generic error"),
			reqSizeGiB:          2,
			expErr:              errors.New("ResizeDisk generic error"),
		},
		{
			name:     "failure: volume in modifying state",
			volumeID: "vol-test",
			existingVolume: &types.Volume{
				VolumeId:         aws.String("vol-test"),
				Size:             aws.Int32(1),
				AvailabilityZone: aws.String(defaultZone),
			},
			descModVolume: &ec2.DescribeVolumesModificationsOutput{
				VolumesModifications: []types.VolumeModification{
					{
						VolumeId:          aws.String("vol-test"),
						TargetSize:        aws.Int32(2),
						ModificationState: types.VolumeModificationStateModifying,
					},
				},
			},
			reqSizeGiB: 2,
			expErr:     errors.New("ResizeDisk generic error"),
		},
		{
			name:     "failure: ModifyVolume returned generic error",
			volumeID: "vol-test",
			modifyDiskOptions: &ModifyDiskOptions{
				VolumeType: "GP2",
				IOPS:       3000,
			},
			existingVolume: &types.Volume{
				VolumeId:         aws.String("vol-test"),
				AvailabilityZone: aws.String(defaultZone),
				VolumeType:       types.VolumeTypeGp2,
			},
			modifiedVolumeError: errors.New("GenericErr"),
			expErr:              errors.New("GenericErr"),
		},
		{
			name:     "failure: returned ErrInvalidArgument when ModifyVolume returned InvalidParameterCombination",
			volumeID: "vol-test",
			modifyDiskOptions: &ModifyDiskOptions{
				VolumeType: "GP2",
				IOPS:       3000,
			},
			existingVolume: &types.Volume{
				VolumeId:         aws.String("vol-test"),
				AvailabilityZone: aws.String(defaultZone),
				VolumeType:       types.VolumeTypeGp2,
				Size:             aws.Int32(1),
			},
			modifiedVolumeError: errors.New("InvalidParameterCombination: The parameter iops is not supported for gp2 volumes"),
			expErr:              errors.New("InvalidParameterCombination: The parameter iops is not supported for gp2 volumes"),
		},
		{
			name:     "failure: returned ErrInvalidArgument when ModifyVolume returned UnknownVolumeType",
			volumeID: "vol-test",
			modifyDiskOptions: &ModifyDiskOptions{
				VolumeType: "GPFake",
			},
			existingVolume: &types.Volume{
				VolumeId:         aws.String("vol-test"),
				AvailabilityZone: aws.String(defaultZone),
				VolumeType:       types.VolumeTypeGp2,
				Size:             aws.Int32(1),
			},
			modifiedVolumeError: errors.New("UnknownVolumeType: Unknown volume type: GPFake"),
			expErr:              errors.New("UnknownVolumeType: Unknown volume type: GPFake"),
		},
		{
			name:     "failure: returned ErrInvalidArgument when ModifyVolume returned InvalidParameterValue",
			volumeID: "vol-test",
			modifyDiskOptions: &ModifyDiskOptions{
				VolumeType: "GP3",
				IOPS:       9999999,
			},
			existingVolume: &types.Volume{
				VolumeId:         aws.String("vol-test"),
				AvailabilityZone: aws.String(defaultZone),
				VolumeType:       types.VolumeTypeGp2,
				Size:             aws.Int32(1),
			},
			modifiedVolumeError: errors.New("InvalidParameterValue: iops value 9999999 is not valid"),
			expErr:              errors.New("InvalidParameterValue: iops value 9999999 is not valid"),
		},
		{
			name:     "success: does not call ModifyVolume when no modification required",
			volumeID: "vol-test",
			existingVolume: &types.Volume{
				VolumeId:         aws.String("vol-test"),
				AvailabilityZone: aws.String(defaultZone),
				VolumeType:       types.VolumeTypeGp3,
				Iops:             aws.Int32(3000),
				Size:             aws.Int32(1),
			},
			modifyDiskOptions: &ModifyDiskOptions{
				VolumeType: "GP3",
				IOPS:       3000,
			},
			shouldCallDescribe: true,
			reqSizeGiB:         1,
		},
		{
			name:     "success: does not call ModifyVolume when no modification required (with size)",
			volumeID: "vol-test",
			existingVolume: &types.Volume{
				VolumeId:         aws.String("vol-test"),
				AvailabilityZone: aws.String(defaultZone),
				Size:             aws.Int32(13),
				Iops:             aws.Int32(3000),
			},
			reqSizeGiB: 13,
			modifyDiskOptions: &ModifyDiskOptions{
				IOPS: 3000,
			},
			shouldCallDescribe: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockCtrl := gomock.NewController(t)
			mockEC2 := NewMockEC2API(mockCtrl)
			c := newCloud(mockEC2)

			ctx := t.Context()
			if tc.existingVolume != nil || tc.existingVolumeError != nil {
				mockEC2.EXPECT().DescribeVolumes(gomock.Any(), gomock.Any()).Return(
					&ec2.DescribeVolumesOutput{
						Volumes: []types.Volume{
							*tc.existingVolume,
						},
					}, tc.existingVolumeError)

				if tc.shouldCallDescribe {
					newVolume := tc.existingVolume
					if tc.reqSizeGiB != 0 {
						newVolume.Size = aws.Int32(tc.reqSizeGiB)
					}
					if tc.modifyDiskOptions != nil {
						if tc.modifyDiskOptions.IOPS != 0 {
							newVolume.Iops = aws.Int32(tc.modifyDiskOptions.IOPS)
						}
						if tc.modifyDiskOptions.Throughput != 0 {
							newVolume.Throughput = aws.Int32(tc.modifyDiskOptions.Throughput)
						}
						if tc.modifyDiskOptions.VolumeType != "" {
							newVolume.VolumeType = types.VolumeType(tc.modifyDiskOptions.VolumeType)
						}
					}
					mockEC2.EXPECT().DescribeVolumes(gomock.Any(), gomock.Any()).Return(
						&ec2.DescribeVolumesOutput{
							Volumes: []types.Volume{
								*newVolume,
							},
						}, tc.existingVolumeError)
				}
			}
			if tc.modifiedVolume != nil || tc.modifiedVolumeError != nil {
				mockEC2.EXPECT().ModifyVolume(gomock.Any(), gomock.Any(), gomock.Any()).Return(tc.modifiedVolume, tc.modifiedVolumeError).AnyTimes()
			}
			if tc.descModVolume != nil {
				mockEC2.EXPECT().DescribeVolumesModifications(gomock.Any(), gomock.Any(), gomock.Any()).Return(tc.descModVolume, nil).AnyTimes()
			} else {
				emptyOutput := &ec2.DescribeVolumesModificationsOutput{}
				mockEC2.EXPECT().DescribeVolumesModifications(gomock.Any(), gomock.Any(), gomock.Any()).Return(emptyOutput, nil).AnyTimes()
			}

			newSize, err := c.ResizeOrModifyDisk(ctx, tc.volumeID, util.GiBToBytes(tc.reqSizeGiB), tc.modifyDiskOptions)
			switch {
			case errors.Is(tc.expErr, ErrInvalidArgument):
				require.ErrorIs(t, err, ErrInvalidArgument, "ResizeOrModifyDisk() should return ErrInvalidArgument")
			case tc.expErr != nil:
				require.Error(t, err, "ResizeOrModifyDisk() should return error")
			default:
				require.NoError(t, err, "ResizeOrModifyDisk() should not return error")
				assert.Equal(t, tc.reqSizeGiB, newSize, "ResizeOrModifyDisk() returned unexpected capacity")
			}

			mockCtrl.Finish()
		})
	}
}

func TestModifyTags(t *testing.T) {
	validTagsToAddInput := map[string]string{
		"key1": "value1",
		"key2": "value2",
		"key3": "",
	}

	validTagsToDeleteInput := []string{
		"key1",
		"key2",
	}

	emptyTagsToAddInput := map[string]string{}
	emptyTagsToDeleteInput := []string{}

	testCases := []struct {
		name              string
		volumeID          string
		negativeCase      bool
		modifyTagsOptions ModifyTagsOptions
		expErr            error
	}{
		{
			name:     "success normal tag addition",
			volumeID: "mod-tag-test-name",
			modifyTagsOptions: ModifyTagsOptions{
				TagsToAdd: validTagsToAddInput,
			},
			expErr: nil,
		},
		{
			name:     "success normal tag deletion",
			volumeID: "mod-tag-test-name",
			modifyTagsOptions: ModifyTagsOptions{
				TagsToDelete: validTagsToDeleteInput,
			},
			expErr: nil,
		},
		{
			name:     "success normal tag addition and tag deletion",
			volumeID: "mod-tag-test-name",
			modifyTagsOptions: ModifyTagsOptions{
				TagsToAdd:    validTagsToAddInput,
				TagsToDelete: validTagsToDeleteInput,
			},
			expErr: nil,
		},
		{
			name:         "fail: EC2 API generic error TagsToAdd",
			volumeID:     "mod-tag-test-name",
			negativeCase: true,
			expErr:       errors.New("Generic EC2 API error"),
			modifyTagsOptions: ModifyTagsOptions{
				TagsToAdd:    validTagsToAddInput,
				TagsToDelete: emptyTagsToDeleteInput,
			},
		},
		{
			name:         "fail: EC2 API generic error TagsToDelete",
			volumeID:     "mod-tag-test-name",
			negativeCase: true,
			expErr:       errors.New("Generic EC2 API error"),
			modifyTagsOptions: ModifyTagsOptions{
				TagsToAdd:    emptyTagsToAddInput,
				TagsToDelete: validTagsToDeleteInput,
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockCtrl := gomock.NewController(t)
			mockEC2 := NewMockEC2API(mockCtrl)
			c := newCloud(mockEC2)

			ctx := t.Context()

			if len(tc.modifyTagsOptions.TagsToAdd) > 0 {
				if tc.negativeCase {
					mockEC2.EXPECT().CreateTags(gomock.Any(), gomock.Any()).Return(nil, tc.expErr).Times(1)
				} else {
					mockEC2.EXPECT().CreateTags(gomock.Any(), gomock.Any()).Return(&ec2.CreateTagsOutput{}, tc.expErr).Times(1)
				}
			}
			if len(tc.modifyTagsOptions.TagsToDelete) > 0 {
				if tc.negativeCase {
					mockEC2.EXPECT().DeleteTags(gomock.Any(), gomock.Any()).Return(nil, tc.expErr).Times(1)
				} else {
					mockEC2.EXPECT().DeleteTags(gomock.Any(), gomock.Any()).Return(&ec2.DeleteTagsOutput{}, tc.expErr).Times(1)
				}
			}

			err := c.ModifyTags(ctx, tc.volumeID, tc.modifyTagsOptions)
			if err != nil {
				if tc.expErr == nil {
					t.Fatalf("ModifyTags() failed: expected no error, got: %v", err)
				} else if !strings.Contains(err.Error(), tc.expErr.Error()) {
					t.Fatalf("ModifyTags() failed: expected error %v, got: %v", tc.expErr, err)
				}
			} else if tc.expErr != nil {
				t.Fatal("ModifyTags() failed: expected error, got nothing")
			}

			mockCtrl.Finish()
		})
	}
}

func TestGetSnapshotByName(t *testing.T) {
	testCases := []struct {
		name            string
		snapshotName    string
		snapshotOptions *SnapshotOptions
		expSnapshot     *Snapshot
		expErr          error
	}{
		{
			name:         "success: normal",
			snapshotName: "snap-test-name",
			snapshotOptions: &SnapshotOptions{
				Tags: map[string]string{
					SnapshotNameTagKey: "snap-test-name",
					AwsEbsDriverTagKey: "true",
					"extra-tag-key":    "extra-tag-value",
				},
			},
			expSnapshot: &Snapshot{
				SnapshotID:     "snap-test-id",
				SourceVolumeID: "snap-test-volume",
				Size:           10,
				CreationTime:   time.Now(),
				ReadyToUse:     true,
			},
			expErr: nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockCtrl := gomock.NewController(t)
			mockEC2 := NewMockEC2API(mockCtrl)
			c := newCloud(mockEC2)

			ec2snapshot := types.Snapshot{
				SnapshotId: aws.String(tc.expSnapshot.SnapshotID),
				VolumeId:   aws.String(tc.expSnapshot.SourceVolumeID),
				VolumeSize: aws.Int32(tc.expSnapshot.Size),
				StartTime:  aws.Time(tc.expSnapshot.CreationTime),
				State:      types.SnapshotStateCompleted,
				Tags: []types.Tag{
					{
						Key:   aws.String(SnapshotNameTagKey),
						Value: aws.String(tc.snapshotName),
					},
					{
						Key:   aws.String(AwsEbsDriverTagKey),
						Value: aws.String("true"),
					},
					{
						Key:   aws.String("extra-tag-key"),
						Value: aws.String("extra-tag-value"),
					},
				},
			}

			ctx := t.Context()

			mockEC2.EXPECT().DescribeSnapshots(gomock.Any(), gomock.Any()).Return(&ec2.DescribeSnapshotsOutput{Snapshots: []types.Snapshot{ec2snapshot}}, nil)

			snapshot, err := c.GetSnapshotByName(ctx, tc.snapshotName)
			if err != nil {
				if tc.expErr == nil {
					t.Fatalf("GetSnapshotByName() failed: expected no error, got: %v", err)
				}
				if err.Error() != tc.expErr.Error() {
					t.Fatalf("GetSnapshotByName() failed: expected error %q, got %q", tc.expErr, err)
				}
			} else {
				if tc.expErr != nil {
					t.Fatal("GetSnapshotByName() failed: expected error, got nothing")
				}
				if snapshot.SnapshotID != tc.expSnapshot.SnapshotID {
					t.Fatalf("GetSnapshotByName() failed: expected snapshot ID %q, got %q", tc.expSnapshot.SnapshotID, snapshot.SnapshotID)
				}
				if snapshot.SourceVolumeID != tc.expSnapshot.SourceVolumeID {
					t.Fatalf("GetSnapshotByName() failed: expected source volume ID %q, got %q", tc.expSnapshot.SourceVolumeID, snapshot.SourceVolumeID)
				}
				if snapshot.Size != tc.expSnapshot.Size {
					t.Fatalf("GetSnapshotByName() failed: expected size %d, got %d", tc.expSnapshot.Size, snapshot.Size)
				}
				if !snapshot.CreationTime.Equal(tc.expSnapshot.CreationTime) {
					t.Fatalf("GetSnapshotByName() failed: expected creation time %v, got %v", tc.expSnapshot.CreationTime, snapshot.CreationTime)
				}
				if snapshot.ReadyToUse != tc.expSnapshot.ReadyToUse {
					t.Fatalf("GetSnapshotByName() failed: expected ready to use %t, got %t", tc.expSnapshot.ReadyToUse, snapshot.ReadyToUse)
				}
			}

			mockCtrl.Finish()
		})
	}
}

func TestGetSnapshotByID(t *testing.T) {
	testCases := []struct {
		name        string
		snapshotID  string
		expSnapshot *Snapshot
		expErr      error
	}{
		{
			name:       "success: normal",
			snapshotID: "snap-test-name",
			expSnapshot: &Snapshot{
				SnapshotID:     "snap-test-name",
				SourceVolumeID: "snap-test-volume",
				Size:           10,
				CreationTime:   time.Now(),
				ReadyToUse:     true,
			},
			expErr: nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockCtrl := gomock.NewController(t)
			mockEC2 := NewMockEC2API(mockCtrl)
			c := newCloud(mockEC2)

			ec2snapshot := types.Snapshot{
				SnapshotId: aws.String(tc.snapshotID),
				VolumeId:   aws.String(tc.expSnapshot.SourceVolumeID),
				VolumeSize: aws.Int32(tc.expSnapshot.Size),
				StartTime:  aws.Time(tc.expSnapshot.CreationTime),
				State:      types.SnapshotStateCompleted,
			}

			ctx := t.Context()

			mockEC2.EXPECT().DescribeSnapshots(gomock.Any(), gomock.Any()).Return(&ec2.DescribeSnapshotsOutput{Snapshots: []types.Snapshot{ec2snapshot}}, nil)

			snapshot, err := c.GetSnapshotByID(ctx, tc.snapshotID)
			if err != nil {
				if tc.expErr == nil {
					t.Fatalf("GetSnapshotByID() failed: expected no error, got: %v", err)
				}
				if err.Error() != tc.expErr.Error() {
					t.Fatalf("GetSnapshotByID() failed: expected error %q, got %q", tc.expErr, err)
				}
			} else {
				if tc.expErr != nil {
					t.Fatal("GetSnapshotByID() failed: expected error, got nothing")
				}
				if snapshot.SnapshotID != tc.expSnapshot.SnapshotID {
					t.Fatalf("GetSnapshotByID() failed: expected snapshot ID %q, got %q", tc.expSnapshot.SnapshotID, snapshot.SnapshotID)
				}
				if snapshot.SourceVolumeID != tc.expSnapshot.SourceVolumeID {
					t.Fatalf("GetSnapshotByID() failed: expected source volume ID %q, got %q", tc.expSnapshot.SourceVolumeID, snapshot.SourceVolumeID)
				}
				if snapshot.Size != tc.expSnapshot.Size {
					t.Fatalf("GetSnapshotByID() failed: expected size %d, got %d", tc.expSnapshot.Size, snapshot.Size)
				}
				if !snapshot.CreationTime.Equal(tc.expSnapshot.CreationTime) {
					t.Fatalf("GetSnapshotByID() failed: expected creation time %v, got %v", tc.expSnapshot.CreationTime, snapshot.CreationTime)
				}
				if snapshot.ReadyToUse != tc.expSnapshot.ReadyToUse {
					t.Fatalf("GetSnapshotByID() failed: expected ready to use %t, got %t", tc.expSnapshot.ReadyToUse, snapshot.ReadyToUse)
				}
			}

			mockCtrl.Finish()
		})
	}
}
func TestListSnapshots(t *testing.T) {
	testCases := []struct {
		name     string
		testFunc func(t *testing.T)
	}{
		{
			name: "success: normal",
			testFunc: func(t *testing.T) {
				t.Helper()
				expSnapshots := []*Snapshot{
					{
						SourceVolumeID: "snap-test-volume1",
						SnapshotID:     "snap-test-name1",
						Size:           10,
						CreationTime:   time.Now(),
						ReadyToUse:     true,
					},
					{
						SourceVolumeID: "snap-test-volume2",
						SnapshotID:     "snap-test-name2",
						Size:           20,
						CreationTime:   time.Now(),
						ReadyToUse:     true,
					},
				}
				ec2Snapshots := []types.Snapshot{
					{
						SnapshotId: aws.String(expSnapshots[0].SnapshotID),
						VolumeId:   aws.String(expSnapshots[0].SourceVolumeID),
						VolumeSize: aws.Int32(expSnapshots[0].Size),
						StartTime:  aws.Time(expSnapshots[0].CreationTime),
						State:      types.SnapshotStateCompleted,
					},
					{
						SnapshotId: aws.String(expSnapshots[1].SnapshotID),
						VolumeId:   aws.String(expSnapshots[1].SourceVolumeID),
						VolumeSize: aws.Int32(expSnapshots[1].Size),
						StartTime:  aws.Time(expSnapshots[1].CreationTime),
						State:      types.SnapshotStateCompleted,
					},
				}

				mockCtl := gomock.NewController(t)
				defer mockCtl.Finish()
				mockEC2 := NewMockEC2API(mockCtl)
				c := newCloud(mockEC2)

				ctx := t.Context()

				mockEC2.EXPECT().DescribeSnapshots(gomock.Any(), gomock.Any()).Return(&ec2.DescribeSnapshotsOutput{Snapshots: ec2Snapshots}, nil)

				resp, err := c.ListSnapshots(ctx, "", 0, "")
				if err != nil {
					t.Fatalf("ListSnapshots() failed: expected no error, got: %v", err)
				}

				if len(resp.Snapshots) != len(expSnapshots) {
					t.Fatalf("Expected %d snapshots, got %d", len(expSnapshots), len(resp.Snapshots))
				}

				for i, snap := range resp.Snapshots {
					if snap.SourceVolumeID != expSnapshots[i].SourceVolumeID {
						t.Fatalf("Unexpected source volume. Expected %s, got %s", expSnapshots[i].SourceVolumeID, snap.SourceVolumeID)
					}
					if snap.SnapshotID != expSnapshots[i].SnapshotID {
						t.Fatalf("Unexpected snapshot ID. Expected %s, got %s", expSnapshots[i].SnapshotID, snap.SnapshotID)
					}
					if snap.Size != expSnapshots[i].Size {
						t.Fatalf("Unexpected snapshot size. Expected %d, got %d", expSnapshots[i].Size, snap.Size)
					}
					if !snap.CreationTime.Equal(expSnapshots[i].CreationTime) {
						t.Fatalf("Unexpected creation time. Expected %v, got %v", expSnapshots[i].CreationTime, snap.CreationTime)
					}
					if snap.ReadyToUse != expSnapshots[i].ReadyToUse {
						t.Fatalf("Unexpected ready to use state. Expected %t, got %t", expSnapshots[i].ReadyToUse, snap.ReadyToUse)
					}
				}
			},
		},
		{
			name: "success: with volume ID",
			testFunc: func(t *testing.T) {
				t.Helper()
				sourceVolumeID := "snap-test-volume"
				expSnapshots := []*Snapshot{
					{
						SourceVolumeID: sourceVolumeID,
						SnapshotID:     "snap-test-name1",
						Size:           10,
						CreationTime:   time.Now(),
						ReadyToUse:     true,
					},
					{
						SourceVolumeID: sourceVolumeID,
						SnapshotID:     "snap-test-name2",
						Size:           20,
						CreationTime:   time.Now(),
						ReadyToUse:     true,
					},
				}
				ec2Snapshots := []types.Snapshot{
					{
						SnapshotId: aws.String(expSnapshots[0].SnapshotID),
						VolumeId:   aws.String(sourceVolumeID),
						VolumeSize: aws.Int32(expSnapshots[0].Size),
						StartTime:  aws.Time(expSnapshots[0].CreationTime),
						State:      types.SnapshotStateCompleted,
					},
					{
						SnapshotId: aws.String(expSnapshots[1].SnapshotID),
						VolumeId:   aws.String(sourceVolumeID),
						VolumeSize: aws.Int32(expSnapshots[1].Size),
						StartTime:  aws.Time(expSnapshots[1].CreationTime),
						State:      types.SnapshotStateCompleted,
					},
				}

				mockCtl := gomock.NewController(t)
				defer mockCtl.Finish()
				mockEC2 := NewMockEC2API(mockCtl)
				c := newCloud(mockEC2)

				ctx := t.Context()

				mockEC2.EXPECT().DescribeSnapshots(gomock.Any(), gomock.Any()).Return(&ec2.DescribeSnapshotsOutput{Snapshots: ec2Snapshots}, nil)

				resp, err := c.ListSnapshots(ctx, sourceVolumeID, 0, "")
				if err != nil {
					t.Fatalf("ListSnapshots() failed: expected no error, got: %v", err)
				}

				if len(resp.Snapshots) != len(expSnapshots) {
					t.Fatalf("Expected %d snapshots, got %d", len(expSnapshots), len(resp.Snapshots))
				}

				for i, snap := range resp.Snapshots {
					if snap.SourceVolumeID != expSnapshots[i].SourceVolumeID {
						t.Fatalf("Unexpected source volume. Expected %s, got %s", expSnapshots[i].SourceVolumeID, snap.SourceVolumeID)
					}
					if snap.SnapshotID != expSnapshots[i].SnapshotID {
						t.Fatalf("Unexpected snapshot ID. Expected %s, got %s", expSnapshots[i].SnapshotID, snap.SnapshotID)
					}
					if snap.Size != expSnapshots[i].Size {
						t.Fatalf("Unexpected snapshot size. Expected %d, got %d", expSnapshots[i].Size, snap.Size)
					}
					if !snap.CreationTime.Equal(expSnapshots[i].CreationTime) {
						t.Fatalf("Unexpected creation time. Expected %v, got %v", expSnapshots[i].CreationTime, snap.CreationTime)
					}
					if snap.ReadyToUse != expSnapshots[i].ReadyToUse {
						t.Fatalf("Unexpected ready to use state. Expected %t, got %t", expSnapshots[i].ReadyToUse, snap.ReadyToUse)
					}
				}
			},
		},
		{
			name: "success: max results, next token",
			testFunc: func(t *testing.T) {
				t.Helper()
				maxResults := 5
				nextTokenValue := "nextTokenValue"
				var expSnapshots []*Snapshot
				for i := range maxResults * 2 {
					expSnapshots = append(expSnapshots, &Snapshot{
						SourceVolumeID: "snap-test-volume1",
						SnapshotID:     fmt.Sprintf("snap-test-name%d", i),
						CreationTime:   time.Now(),
						ReadyToUse:     true,
					})
				}

				var ec2Snapshots []types.Snapshot
				for i := range maxResults * 2 {
					ec2Snapshots = append(ec2Snapshots, types.Snapshot{
						SnapshotId: aws.String(expSnapshots[i].SnapshotID),
						VolumeId:   aws.String(fmt.Sprintf("snap-test-volume%d", i)),
						VolumeSize: aws.Int32(expSnapshots[i].Size),
						StartTime:  aws.Time(expSnapshots[i].CreationTime),
						State:      types.SnapshotStateCompleted,
					})
				}

				mockCtl := gomock.NewController(t)
				defer mockCtl.Finish()
				mockEC2 := NewMockEC2API(mockCtl)
				c := newCloud(mockEC2)

				ctx := t.Context()

				firstCall := mockEC2.EXPECT().DescribeSnapshots(gomock.Any(), gomock.Any()).Return(&ec2.DescribeSnapshotsOutput{
					Snapshots: ec2Snapshots[:maxResults],
					NextToken: aws.String(nextTokenValue),
				}, nil)
				secondCall := mockEC2.EXPECT().DescribeSnapshots(gomock.Any(), gomock.Any()).Return(&ec2.DescribeSnapshotsOutput{
					Snapshots: ec2Snapshots[maxResults:],
				}, nil)
				gomock.InOrder(
					firstCall,
					secondCall,
				)

				firstSnapshotsResponse, err := c.ListSnapshots(ctx, "", 5, "")
				if err != nil {
					t.Fatalf("ListSnapshots() failed: expected no error, got: %v", err)
				}

				if len(firstSnapshotsResponse.Snapshots) != maxResults {
					t.Fatalf("Expected %d snapshots, got %d", maxResults, len(firstSnapshotsResponse.Snapshots))
				}

				if firstSnapshotsResponse.NextToken != nextTokenValue {
					t.Fatalf("Expected next token value '%s' got '%s'", nextTokenValue, firstSnapshotsResponse.NextToken)
				}

				secondSnapshotsResponse, err := c.ListSnapshots(ctx, "", 0, firstSnapshotsResponse.NextToken)
				if err != nil {
					t.Fatalf("CreateSnapshot() failed: expected no error, got: %v", err)
				}

				if len(secondSnapshotsResponse.Snapshots) != maxResults {
					t.Fatalf("Expected %d snapshots, got %d", maxResults, len(secondSnapshotsResponse.Snapshots))
				}

				if secondSnapshotsResponse.NextToken != "" {
					t.Fatalf("Expected next token value to be empty got %s", secondSnapshotsResponse.NextToken)
				}
			},
		},
		{
			name: "fail: AWS DescribeSnapshots error",
			testFunc: func(t *testing.T) {
				t.Helper()
				mockCtl := gomock.NewController(t)
				defer mockCtl.Finish()
				mockEC2 := NewMockEC2API(mockCtl)
				c := newCloud(mockEC2)

				ctx := t.Context()

				mockEC2.EXPECT().DescribeSnapshots(gomock.Any(), gomock.Any()).Return(nil, errors.New("test error"))

				if _, err := c.ListSnapshots(ctx, "", 0, ""); err == nil {
					t.Fatalf("ListSnapshots() failed: expected an error, got none")
				}
			},
		},
		{
			name: "fail: no snapshots ErrNotFound",
			testFunc: func(t *testing.T) {
				t.Helper()
				mockCtl := gomock.NewController(t)
				defer mockCtl.Finish()
				mockEC2 := NewMockEC2API(mockCtl)
				c := newCloud(mockEC2)

				ctx := t.Context()

				mockEC2.EXPECT().DescribeSnapshots(gomock.Any(), gomock.Any()).Return(&ec2.DescribeSnapshotsOutput{}, nil)

				_, err := c.ListSnapshots(ctx, "", 0, "")
				if err != nil {
					if !errors.Is(err, ErrNotFound) {
						t.Fatalf("Expected error %v, got %v", ErrNotFound, err)
					}
				} else {
					t.Fatalf("Expected error, got none")
				}
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, tc.testFunc)
	}
}

func TestWaitForAttachmentState(t *testing.T) {
	testCases := []struct {
		name               string
		volumeID           string
		expectedState      types.VolumeAttachmentState
		expectedInstance   string
		expectedDevice     string
		alreadyAssigned    bool
		expectError        bool
		associatedResource *string
	}{
		{
			name:             "success: attached",
			volumeID:         "vol-test-1234",
			expectedState:    types.VolumeAttachmentStateAttached,
			expectedInstance: "1234",
			expectedDevice:   defaultPath,
			alreadyAssigned:  false,
			expectError:      false,
		},
		{
			name:             "success: detached",
			volumeID:         "vol-test-1234",
			expectedState:    types.VolumeAttachmentStateDetached,
			expectedInstance: "1234",
			expectedDevice:   defaultPath,
			alreadyAssigned:  false,
			expectError:      false,
		},
		{
			name:             "success: disk not found, assumed detached",
			volumeID:         "vol-test-1234",
			expectedState:    types.VolumeAttachmentStateDetached,
			expectedInstance: "1234",
			expectedDevice:   defaultPath,
			alreadyAssigned:  false,
			expectError:      false,
		},
		{
			name:             "success: multiple attachments with Multi-Attach enabled",
			volumeID:         "vol-test-1234",
			expectedState:    types.VolumeAttachmentStateAttached,
			expectedInstance: "1234",
			expectedDevice:   defaultPath,
			alreadyAssigned:  false,
			expectError:      false,
		},
		{
			name:             "failure: disk not found, expected attached",
			volumeID:         "vol-test-1234",
			expectedState:    types.VolumeAttachmentStateAttached,
			expectedInstance: "1234",
			expectedDevice:   defaultPath,
			alreadyAssigned:  false,
			expectError:      true,
		},
		{
			name:             "failure: unexpected device",
			volumeID:         "vol-test-1234",
			expectedState:    types.VolumeAttachmentStateAttached,
			expectedInstance: "1234",
			expectedDevice:   "/dev/xvdab",
			alreadyAssigned:  false,
			expectError:      true,
		},
		{
			name:             "failure: unexpected instance",
			volumeID:         "vol-test-1234",
			expectedState:    types.VolumeAttachmentStateAttached,
			expectedInstance: "1235",
			expectedDevice:   defaultPath,
			alreadyAssigned:  false,
			expectError:      true,
		},
		{
			name:             "failure: already assigned but detached state",
			volumeID:         "vol-test-1234",
			expectedState:    types.VolumeAttachmentStateAttached,
			expectedInstance: "1234",
			expectedDevice:   defaultPath,
			alreadyAssigned:  true,
			expectError:      true,
		},
		{
			name:             "failure: already assigned but attaching state",
			volumeID:         "vol-test-1234",
			expectedState:    types.VolumeAttachmentStateAttached,
			expectedInstance: "1234",
			expectedDevice:   defaultPath,
			alreadyAssigned:  true,
			expectError:      false,
		},
		{
			name:             "failure: multiple attachments with Multi-Attach disabled",
			volumeID:         "vol-test-1234",
			expectedState:    types.VolumeAttachmentStateAttached,
			expectedInstance: "1234",
			expectedDevice:   defaultPath,
			alreadyAssigned:  false,
			expectError:      true,
		},
		{
			name:               "success: HyperPod attached",
			volumeID:           "vol-test-1234",
			expectedState:      types.VolumeAttachmentStateAttached,
			expectedInstance:   "hyperpod-cluster1-i-1234567890",
			associatedResource: aws.String("arn:aws:sagemaker:us-east-1:123456789012:cluster/cluster1-i-1234567890"),
			alreadyAssigned:    false,
			expectError:        false,
		},
		{
			name:               "success: HyperPod detached",
			volumeID:           "vol-test-1234",
			expectedState:      types.VolumeAttachmentStateDetached,
			expectedInstance:   "hyperpod-cluster1-i-1234567890",
			associatedResource: aws.String("arn:aws:sagemaker:us-east-1:123456789012:cluster/cluster1-i-1234567890"),
			alreadyAssigned:    false,
			expectError:        false,
		},
		{
			name:               "failure: HyperPod with mismatch AssociatedResource",
			volumeID:           "vol-test-1234",
			expectedState:      types.VolumeAttachmentStateAttached,
			expectedInstance:   "hyperpod-cluster1-i-1234567890",
			associatedResource: aws.String("arn:aws:sagemaker:us-east-1:123456789012:cluster/cluster1-i-0987654321"),
			alreadyAssigned:    false,
			expectError:        true,
		},
		{
			name:               "failure: HyperPod with invalid instanceId in AssociatedResource",
			volumeID:           "vol-test-1234",
			expectedState:      types.VolumeAttachmentStateAttached,
			expectedInstance:   "hyperpod-cluster1-i-1234567890",
			associatedResource: aws.String("arn:aws:sagemaker:us-east-1:123456789012:cluster/cluster1-invalid-id"),
			alreadyAssigned:    false,
			expectError:        true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockCtrl := gomock.NewController(t)
			mockEC2 := NewMockEC2API(mockCtrl)
			c := newCloud(mockEC2)

			attachedVol := types.Volume{
				VolumeId:    aws.String(tc.volumeID),
				Attachments: []types.VolumeAttachment{{Device: aws.String(defaultPath), InstanceId: aws.String("1234"), State: types.VolumeAttachmentStateAttached}},
			}

			attachingVol := types.Volume{
				VolumeId:    aws.String(tc.volumeID),
				Attachments: []types.VolumeAttachment{{Device: aws.String(defaultPath), InstanceId: aws.String("1234"), State: types.VolumeAttachmentStateAttaching}},
			}

			detachedVol := types.Volume{
				VolumeId:    aws.String(tc.volumeID),
				Attachments: []types.VolumeAttachment{{Device: aws.String(defaultPath), InstanceId: aws.String("1234"), State: types.VolumeAttachmentStateDetached}},
			}

			multipleAttachmentsVol := types.Volume{
				VolumeId:           aws.String(tc.volumeID),
				Attachments:        []types.VolumeAttachment{{Device: aws.String(defaultPath), InstanceId: aws.String("1235"), State: types.VolumeAttachmentStateAttached}, {Device: aws.String(defaultPath), InstanceId: aws.String("1234"), State: types.VolumeAttachmentStateAttached}},
				MultiAttachEnabled: aws.Bool(false),
			}

			hyperpodAttachedVol := types.Volume{
				VolumeId:    aws.String(tc.volumeID),
				Attachments: []types.VolumeAttachment{{State: types.VolumeAttachmentStateAttached, AssociatedResource: tc.associatedResource}},
			}

			hyperpodDetachedVol := types.Volume{
				VolumeId:    aws.String(tc.volumeID),
				Attachments: []types.VolumeAttachment{{State: types.VolumeAttachmentStateDetached, AssociatedResource: tc.associatedResource}},
			}

			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()

			switch tc.name {
			case "success: detached":
				mockEC2.EXPECT().DescribeVolumes(gomock.Any(), gomock.Any()).Return(&ec2.DescribeVolumesOutput{Volumes: []types.Volume{detachedVol}}, nil).AnyTimes()
			case "failure: already assigned but detached state":
				mockEC2.EXPECT().DescribeVolumes(gomock.Any(), gomock.Any()).Return(&ec2.DescribeVolumesOutput{Volumes: []types.Volume{detachedVol}}, nil)
				mockEC2.EXPECT().AttachVolume(gomock.Any(), gomock.Any()).Return(nil, nil)
			case "failure: already assigned but attaching state":
				mockEC2.EXPECT().DescribeVolumes(gomock.Any(), gomock.Any()).Return(&ec2.DescribeVolumesOutput{Volumes: []types.Volume{attachingVol}}, nil)
				mockEC2.EXPECT().DescribeVolumes(gomock.Any(), gomock.Any()).Return(&ec2.DescribeVolumesOutput{Volumes: []types.Volume{attachedVol}}, nil)
			case "success: disk not found, assumed detached", "failure: disk not found, expected attached":
				mockEC2.EXPECT().DescribeVolumes(gomock.Any(), gomock.Any()).Return(nil, &smithy.GenericAPIError{
					Code:    "InvalidVolume.NotFound",
					Message: "foo",
				}).AnyTimes()
			case "success: multiple attachments with Multi-Attach enabled":
				multipleAttachmentsVol.MultiAttachEnabled = aws.Bool(true)
				mockEC2.EXPECT().DescribeVolumes(gomock.Any(), gomock.Any()).Return(&ec2.DescribeVolumesOutput{Volumes: []types.Volume{multipleAttachmentsVol}}, nil).AnyTimes()
			case "success: HyperPod attached":
				mockEC2.EXPECT().DescribeVolumes(gomock.Any(), gomock.Any()).Return(&ec2.DescribeVolumesOutput{Volumes: []types.Volume{hyperpodAttachedVol}}, nil).AnyTimes()
			case "success: HyperPod detached":
				mockEC2.EXPECT().DescribeVolumes(gomock.Any(), gomock.Any()).Return(&ec2.DescribeVolumesOutput{Volumes: []types.Volume{hyperpodDetachedVol}}, nil).AnyTimes()
			case "failure: HyperPod with mismatch AssociatedResource":
				mockEC2.EXPECT().DescribeVolumes(gomock.Any(), gomock.Any()).Return(&ec2.DescribeVolumesOutput{Volumes: []types.Volume{hyperpodAttachedVol}}, nil).AnyTimes()
			case "failure: HyperPod with invalid instanceId in AssociatedResource":
				mockEC2.EXPECT().DescribeVolumes(gomock.Any(), gomock.Any()).Return(&ec2.DescribeVolumesOutput{Volumes: []types.Volume{hyperpodAttachedVol}}, nil).AnyTimes()
			case "failure: multiple attachments with Multi-Attach disabled":
				mockEC2.EXPECT().DescribeVolumes(gomock.Any(), gomock.Any()).Return(&ec2.DescribeVolumesOutput{Volumes: []types.Volume{multipleAttachmentsVol}}, nil).AnyTimes()
			case "failure: disk still attaching":
				mockEC2.EXPECT().DescribeVolumes(gomock.Any(), gomock.Any()).Return(&ec2.DescribeVolumesOutput{Volumes: []types.Volume{attachingVol}}, nil).AnyTimes()
			case "failure: context cancelled":
				mockEC2.EXPECT().DescribeVolumes(ctx, gomock.Any()).Return(&ec2.DescribeVolumesOutput{Volumes: []types.Volume{attachingVol}}, nil).AnyTimes()
				cancel()
			default:
				mockEC2.EXPECT().DescribeVolumes(gomock.Any(), gomock.Any()).Return(&ec2.DescribeVolumesOutput{Volumes: []types.Volume{attachedVol}}, nil).AnyTimes()
			}

			attachment, err := c.WaitForAttachmentState(ctx, tc.expectedState, tc.volumeID, tc.expectedInstance, tc.expectedDevice, tc.alreadyAssigned)

			if tc.expectError {
				if err == nil {
					t.Fatal("WaitForAttachmentState() failed: expected error, got nothing")
				}
			} else {
				if err != nil {
					t.Fatalf("WaitForAttachmentState() failed: expected no error, got %v", err)
				}

				if tc.expectedState == types.VolumeAttachmentStateAttached {
					if attachment == nil {
						t.Fatal("WaiForAttachmentState() failed: expected attachment, got nothing")
					}
				} else {
					if attachment != nil {
						t.Fatalf("WaiForAttachmentState() failed: expected no attachment, got %v", attachment)
					}
				}
			}
		})
	}
}

func TestIsVolumeInitialized(t *testing.T) {
	volID := "vol-test"
	volumeStatusInitialized := types.VolumeStatusItem{
		InitializationStatusDetails: nil,
		VolumeStatus: &types.VolumeStatusInfo{
			Details: []types.VolumeStatusDetails{{
				Name:   types.VolumeStatusNameInitializationState,
				Status: ptr.String("completed"),
			}},
		},
		VolumeId: ptr.String(volID),
	}
	volumeStatusInitializingNoEta := types.VolumeStatusItem{
		InitializationStatusDetails: &types.InitializationStatusDetails{
			EstimatedTimeToCompleteInSeconds: nil,
			InitializationType:               types.InitializationTypeDefault,
			Progress:                         nil,
		},
		VolumeStatus: &types.VolumeStatusInfo{
			Details: []types.VolumeStatusDetails{{
				Name:   types.VolumeStatusNameInitializationState,
				Status: ptr.String("initializing"),
			}},
		},
		VolumeId: ptr.String(volID),
	}
	volumeStatusInitializingYesEta := volumeStatusInitializingNoEta
	volumeStatusInitializingYesEta.InitializationStatusDetails = &types.InitializationStatusDetails{
		EstimatedTimeToCompleteInSeconds: ptr.Int64(60 * 10),
		InitializationType:               types.InitializationTypeProvisionedRate,
		Progress:                         ptr.Int64(64),
	}

	testCases := []struct {
		name      string
		dvsOutput *types.VolumeStatusItem
		// These cache test-case variables will pre-populate volumeInitializations cache
		cacheHit           bool
		cachedInitialized  bool
		cachedAddedETATime time.Duration
		expectSleep        bool
		expErr             error
	}{
		{
			name:      "cache miss: DSV returns initialized",
			dvsOutput: &volumeStatusInitialized,
		},
		{
			name:      "cache miss: DSV returns initializing, no ETA",
			dvsOutput: &volumeStatusInitializingNoEta,
		},
		{
			name:      "cache miss: DSV returns initializing, yes ETA",
			dvsOutput: &volumeStatusInitializingYesEta,
		},
		{
			name:              "cache hit + volume initialized: don't call DSV",
			cacheHit:          true,
			cachedInitialized: true,
		},
		{
			name:        "cache hit + no ETA: call DSV slowly",
			cacheHit:    true,
			dvsOutput:   &volumeStatusInitialized,
			expectSleep: true,
		},
		{
			name:               "cache hit + yes ETA: sleep then call DSV which returns initialized",
			cacheHit:           true,
			cachedAddedETATime: testInitializationSleep,
			dvsOutput:          &volumeStatusInitialized,
			expectSleep:        true,
		},
		{
			name:               "cache hit + yes ETA: should have initialized, call DSV ASAP",
			dvsOutput:          &volumeStatusInitialized,
			cacheHit:           true,
			cachedAddedETATime: 1,
		},
		{
			name: "edge case cache miss: DSV doesn't return initialization-state, still return true",
			dvsOutput: &types.VolumeStatusItem{
				VolumeStatus: &types.VolumeStatusInfo{
					Details: []types.VolumeStatusDetails{{
						Name:   types.VolumeStatusNameIoPerformance,
						Status: ptr.String("test-value"),
					}},
				},
				VolumeId: ptr.String(volID),
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Prepare volumeInitializations cache
			volInitCache := expiringcache.New[string, volumeInitialization](cacheForgetDelay)
			if tc.cacheHit {
				vi := &volumeInitialization{
					initialized: tc.cachedInitialized,
				}
				if tc.cachedAddedETATime != 0 {
					vi.estimatedInitializationTime = time.Now().Add(tc.cachedAddedETATime)
				}
				volInitCache.Set(volID, vi)
			}

			mockCtrl := gomock.NewController(t)
			mockEC2 := NewMockEC2API(mockCtrl)
			c := &cloud{
				region:                "test-region",
				ec2:                   mockEC2,
				volumeInitializations: volInitCache,
				bm: &batcherManager{
					volumeStatusIDBatcherFast: batcher.New(500, 0, func(ids []string) (map[string]*types.VolumeStatusItem, error) {
						return execBatchDescribeVolumeStatus(mockEC2, ids)
					}),
					volumeStatusIDBatcherSlow: batcher.New(500, testInitializationSleep, func(ids []string) (map[string]*types.VolumeStatusItem, error) {
						return execBatchDescribeVolumeStatus(mockEC2, ids) // TODO remove test sleeps once Go 1.25 releases with testing/synctest package
					}),
				},
			}

			// If tc.dvsOutput nil, we should NOT expect a DVS call.
			if tc.dvsOutput != nil {
				mockEC2.EXPECT().DescribeVolumeStatus(gomock.Any(), gomock.Any()).Return(&ec2.DescribeVolumeStatusOutput{VolumeStatuses: []types.VolumeStatusItem{*tc.dvsOutput}}, nil).AnyTimes()
			}

			startTime := time.Now()
			res, err := c.IsVolumeInitialized(t.Context(), volID)

			if (err == nil) != (tc.expErr == nil) || !errors.Is(err, tc.expErr) {
				t.Fatalf("IsVolumeInitialized() didn't return expected err. Expected %v got %v", tc.expErr, err)
			}

			if tc.dvsOutput != nil && res != !isVolumeStatusInitializing(*tc.dvsOutput) {
				t.Fatalf("IsVolumeInitialized() returned wrong result. Expected %t got %t", !isVolumeStatusInitializing(*tc.dvsOutput), res)
			}

			if tc.expectSleep && time.Since(startTime) < (testInitializationSleep/2) {
				t.Fatalf("IsVolumeInitialized() polled DescribeVolumeStatus too early when we know volume %s is not ready yet", volID)
			}

			// Check cache after test
			if tc.dvsOutput != nil {
				confirmInitializationCacheUpdated(t, c.volumeInitializations, volID, *tc.dvsOutput)
			} else {
				confirmInitializationCacheUpdated(t, c.volumeInitializations, volID, volumeStatusInitialized)
			}
		})
	}
}

func confirmInitializationCacheUpdated(tb testing.TB, cache expiringcache.ExpiringCache[string, volumeInitialization], volID string, dvsOutput types.VolumeStatusItem) {
	tb.Helper()

	wasVolumeInitializing := isVolumeStatusInitializing(dvsOutput)

	val, ok := cache.Get(volID)
	switch {
	// Check 1: Cache entry should always exist
	case !ok:
		tb.Fatalf("IsVolumeInitialized() did not cache DescribeVolumeStatus result for volume %s", volID)
	// Check 2: Cache entry should match initialization state of DescribeVolumeStatus output
	case wasVolumeInitializing == val.initialized:
		tb.Fatalf("IsVolumeInitialized() did not cache DescribeVolumeStatus result correctly for volume %s. Expected initialized to be %t got %t", volID, wasVolumeInitializing, val.initialized)
	// Check 3: Cache should have estimated initialization time if volume initializing at provisioned rate
	case dvsOutput.InitializationStatusDetails != nil &&
		dvsOutput.InitializationStatusDetails.EstimatedTimeToCompleteInSeconds != nil &&
		val.estimatedInitializationTime.IsZero():
		tb.Fatalf("IsVolumeInitialized() did not cache DescribeVolumeStatus result correctly for volume %s. Expected estimatedInitializationTime to be non-zero because volume created with initializationRate", volID)
	// Check 4: Cache should not have estimated initialization time if volume status didn't have estimated initialization time
	case wasVolumeInitializing &&
		dvsOutput.InitializationStatusDetails != nil &&
		dvsOutput.InitializationStatusDetails.EstimatedTimeToCompleteInSeconds != nil &&
		val.estimatedInitializationTime.IsZero():
		tb.Fatalf("IsVolumeInitialized() did not cache DescribeVolumeStatus result correctly for volume %s. Expected estimatedInitializationTime to be zero because volume not created with initializationRate", volID)
	default:
		return
	}
}

func testVolumeWaitParameters() volumeWaitParameters {
	testBackoff := wait.Backoff{
		Duration: 100 * time.Millisecond,
		Factor:   1,
		Steps:    3,
	}

	return volumeWaitParameters{
		creationInitialDelay: 0,
		creationBackoff:      testBackoff,
		attachmentBackoff:    testBackoff,
		modificationBackoff:  testBackoff,
	}
}

func newCloud(mockEC2 EC2API) Cloud {
	c := &cloud{
		region:                "test-region",
		accountID:             "123456789012",
		dm:                    dm.NewDeviceManager(),
		ec2:                   mockEC2,
		rm:                    newRetryManager(),
		vwp:                   testVolumeWaitParameters(),
		likelyBadDeviceNames:  expiringcache.New[string, sync.Map](cacheForgetDelay),
		latestClientTokens:    expiringcache.New[string, int](cacheForgetDelay),
		volumeInitializations: expiringcache.New[string, volumeInitialization](cacheForgetDelay),
	}
	return c
}

func newDescribeInstancesOutput(nodeID string, volumeID ...string) *ec2.DescribeInstancesOutput {
	instance := types.Instance{
		InstanceId: aws.String(nodeID),
	}

	if len(volumeID) > 0 && volumeID[0] != "" {
		instance.BlockDeviceMappings = []types.InstanceBlockDeviceMapping{
			{
				DeviceName: aws.String(defaultPath),
				Ebs: &types.EbsInstanceBlockDevice{
					VolumeId: aws.String(volumeID[0]),
				},
			},
		}
	}

	return &ec2.DescribeInstancesOutput{
		Reservations: []types.Reservation{
			{
				Instances: []types.Instance{
					instance,
				},
			},
		},
	}
}

func newFakeInstance(instanceID, volumeID, devicePath string) types.Instance {
	return types.Instance{
		InstanceId: aws.String(instanceID),
		BlockDeviceMappings: []types.InstanceBlockDeviceMapping{
			{
				DeviceName: aws.String(devicePath),
				Ebs:        &types.EbsInstanceBlockDevice{VolumeId: &volumeID},
			},
		},
	}
}

func createVolumeRequest(volumeID string) *ec2.DescribeVolumesInput {
	return &ec2.DescribeVolumesInput{
		VolumeIds: []string{volumeID},
	}
}

func createInstanceRequest(nodeID string) *ec2.DescribeInstancesInput {
	return &ec2.DescribeInstancesInput{
		InstanceIds: []string{nodeID},
	}
}

func createAttachRequest(volumeID, nodeID, path string) *ec2.AttachVolumeInput {
	return &ec2.AttachVolumeInput{
		Device:     aws.String(path),
		InstanceId: aws.String(nodeID),
		VolumeId:   aws.String(volumeID),
	}
}

func createDetachRequest(volumeID, nodeID string) *ec2.DetachVolumeInput {
	return &ec2.DetachVolumeInput{
		VolumeId:   aws.String(volumeID),
		InstanceId: aws.String(nodeID),
	}
}

func createDescribeVolumesOutput(volumeIDs []*string, nodeID, path, state string) *ec2.DescribeVolumesOutput {
	volumes := make([]types.Volume, 0, len(volumeIDs))

	for _, volumeID := range volumeIDs {
		volumes = append(volumes, types.Volume{
			VolumeId: volumeID,
			Attachments: []types.VolumeAttachment{
				{
					Device:             aws.String(path),
					InstanceId:         aws.String(nodeID),
					State:              types.VolumeAttachmentState(state),
					AssociatedResource: aws.String("arn:aws:sagemaker:us-east-1:123456789012:cluster/cluster1-i-1234567890"),
				},
			},
		})
	}

	return &ec2.DescribeVolumesOutput{
		Volumes: volumes,
	}
}
