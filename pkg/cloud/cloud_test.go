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
	"sort"
	"strings"
	"sync"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/ec2/ec2iface"
	"github.com/golang/mock/gomock"
	dm "github.com/kubernetes-sigs/aws-ebs-csi-driver/pkg/cloud/devicemanager"
	"github.com/kubernetes-sigs/aws-ebs-csi-driver/pkg/util"
	"github.com/stretchr/testify/assert"
)

const (
	defaultZone     = "test-az"
	expZone         = "us-west-2b"
	snowZone        = "snow"
	defaultVolumeID = "vol-test-1234"
	defaultNodeID   = "node-1234"
	defaultPath     = "/dev/xvdaa"
)

func generateVolumes(volIdCount, volTagCount int) []*ec2.Volume {
	volumes := make([]*ec2.Volume, 0, volIdCount+volTagCount)

	for i := 0; i < volIdCount; i++ {
		volumeID := fmt.Sprintf("vol-%d", i)
		volumes = append(volumes, &ec2.Volume{VolumeId: aws.String(volumeID)})
	}

	for i := 0; i < volTagCount; i++ {
		volumeName := fmt.Sprintf("vol-name-%d", i)
		volumes = append(volumes, &ec2.Volume{Tags: []*ec2.Tag{{Key: aws.String(VolumeNameTagKey), Value: aws.String(volumeName)}}})
	}

	return volumes
}

func extractVolumeIdentifiers(volumes []*ec2.Volume) (volumeIDs []string, volumeNames []string) {
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

func TestBatchDescribeVolumes(t *testing.T) {
	testCases := []struct {
		name     string
		volumes  []*ec2.Volume
		expErr   error
		mockFunc func(mockEC2 *MockEC2API, expErr error, volumes []*ec2.Volume)
	}{
		{
			name:    "TestBatchDescribeVolumes: volume by ID",
			volumes: generateVolumes(10, 0),
			mockFunc: func(mockEC2 *MockEC2API, expErr error, volumes []*ec2.Volume) {
				volumeOutput := &ec2.DescribeVolumesOutput{Volumes: volumes}
				mockEC2.EXPECT().DescribeVolumesWithContext(gomock.Any(), gomock.Any()).Return(volumeOutput, expErr).Times(1)
			},
			expErr: nil,
		},
		{
			name:    "TestBatchDescribeVolumes: volume by tag",
			volumes: generateVolumes(0, 10),
			mockFunc: func(mockEC2 *MockEC2API, expErr error, volumes []*ec2.Volume) {
				volumeOutput := &ec2.DescribeVolumesOutput{Volumes: volumes}
				mockEC2.EXPECT().DescribeVolumesWithContext(gomock.Any(), gomock.Any()).Return(volumeOutput, expErr).Times(1)
			},
			expErr: nil,
		},
		{
			name:    "TestBatchDescribeVolumes: volume by ID and tag",
			volumes: generateVolumes(10, 10),
			mockFunc: func(mockEC2 *MockEC2API, expErr error, volumes []*ec2.Volume) {
				volumeOutput := &ec2.DescribeVolumesOutput{Volumes: volumes}
				mockEC2.EXPECT().DescribeVolumesWithContext(gomock.Any(), gomock.Any()).Return(volumeOutput, expErr).Times(2)
			},
			expErr: nil,
		},
		{
			name:    "TestBatchDescribeVolumes: max capacity",
			volumes: generateVolumes(500, 0),
			mockFunc: func(mockEC2 *MockEC2API, expErr error, volumes []*ec2.Volume) {
				volumeOutput := &ec2.DescribeVolumesOutput{Volumes: volumes}
				mockEC2.EXPECT().DescribeVolumesWithContext(gomock.Any(), gomock.Any()).Return(volumeOutput, expErr).Times(1)
			},
			expErr: nil,
		},
		{
			name:    "TestBatchDescribeVolumes: capacity exceeded",
			volumes: generateVolumes(550, 0),
			mockFunc: func(mockEC2 *MockEC2API, expErr error, volumes []*ec2.Volume) {
				volumeOutput := &ec2.DescribeVolumesOutput{Volumes: volumes}
				mockEC2.EXPECT().DescribeVolumesWithContext(gomock.Any(), gomock.Any()).Return(volumeOutput, expErr).Times(2)
			},
			expErr: nil,
		},
		{
			name:    "TestBatchDescribeVolumes: EC2 API generic error",
			volumes: generateVolumes(4, 0),
			mockFunc: func(mockEC2 *MockEC2API, expErr error, volumes []*ec2.Volume) {
				mockEC2.EXPECT().DescribeVolumesWithContext(gomock.Any(), gomock.Any()).Return(nil, expErr).Times(1)
			},
			expErr: fmt.Errorf("Generic EC2 API error"),
		},
		{
			name:    "TestBatchDescribeVolumes: volume not found",
			volumes: generateVolumes(1, 0),
			mockFunc: func(mockEC2 *MockEC2API, expErr error, volumes []*ec2.Volume) {
				mockEC2.EXPECT().DescribeVolumesWithContext(gomock.Any(), gomock.Any()).Return(nil, expErr).Times(1)
			},
			expErr: fmt.Errorf("volume not found"),
		},
		{
			name: "TestBatchDescribeVolumes: invalid tag",
			volumes: []*ec2.Volume{
				{
					Tags: []*ec2.Tag{
						{Key: aws.String("InvalidKey"), Value: aws.String("InvalidValue")},
					},
				},
			},
			mockFunc: func(mockEC2 *MockEC2API, expErr error, volumes []*ec2.Volume) {

				volumeOutput := &ec2.DescribeVolumesOutput{Volumes: volumes}
				mockEC2.EXPECT().DescribeVolumesWithContext(gomock.Any(), gomock.Any()).Return(volumeOutput, expErr).Times(0)
			},
			expErr: fmt.Errorf("invalid tag"),
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			mockCtrl := gomock.NewController(t)
			defer mockCtrl.Finish()

			mockEC2 := NewMockEC2API(mockCtrl)
			c := newCloud(mockEC2)
			cloudInstance := c.(*cloud)
			cloudInstance.bm = newBatcherManager(cloudInstance.ec2)

			tc.mockFunc(mockEC2, tc.expErr, tc.volumes)
			volumeIDs, volumeNames := extractVolumeIdentifiers(tc.volumes)
			executeDescribeVolumesTest(t, cloudInstance, volumeIDs, volumeNames, tc.expErr)
		})
	}
}
func executeDescribeVolumesTest(t *testing.T, c *cloud, volumeIDs, volumeNames []string, expErr error) {
	var wg sync.WaitGroup

	getRequestForID := func(id string) *ec2.DescribeVolumesInput {
		return &ec2.DescribeVolumesInput{VolumeIds: []*string{&id}}
	}

	getRequestForTag := func(volName string) *ec2.DescribeVolumesInput {
		return &ec2.DescribeVolumesInput{
			Filters: []*ec2.Filter{
				{
					Name:   aws.String("tag:" + VolumeNameTagKey),
					Values: []*string{&volName},
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

	r := make([]chan *ec2.Volume, len(requests))
	e := make([]chan error, len(requests))

	for i, request := range requests {
		wg.Add(1)
		r[i] = make(chan *ec2.Volume, 1)
		e[i] = make(chan error, 1)

		go func(req *ec2.DescribeVolumesInput, resultCh chan *ec2.Volume, errCh chan error) {
			defer wg.Done()
			volume, err := c.batchDescribeVolumes(req)
			if err != nil {
				errCh <- err
				return
			}
			resultCh <- volume
			// passing `request` as a parameter to create a copy
			// TODO remove after https://github.com/golang/go/discussions/56010 is implemented
		}(request, r[i], e[i])
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

func TestCreateDisk(t *testing.T) {
	testCases := []struct {
		name                 string
		volumeName           string
		volState             string
		diskOptions          *DiskOptions
		expDisk              *Disk
		cleanUpFailedVolume  bool
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
				Iops: aws.Int64(6000),
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
				Iops: aws.Int64(100),
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
				Iops: aws.Int64(100),
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
				Iops: aws.Int64(100),
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
				Iops:       aws.Int64(3000),
				Throughput: aws.Int64(125),
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
			expErr:               fmt.Errorf("could not create volume in EC2: CreateVolume generic error"),
			expCreateVolumeErr:   fmt.Errorf("CreateVolume generic error"),
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
			expCreateVolumeErr:   awserr.New("InvalidSnapshot.NotFound", "Snapshot not found", fmt.Errorf("not able to find source snapshot")),
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
			expErr:               ErrIdempotentParameterMismatch,
			expCreateVolumeErr:   awserr.New("IdempotentParameterMismatch", "Another request is in-flight", fmt.Errorf("another request is in-flight")),
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
			expErr:               fmt.Errorf("failed to get an available volume in EC2: DescribeVolumes generic error"),
			expDescVolumeErr:     fmt.Errorf("DescribeVolumes generic error"),
			cleanUpFailedVolume:  true,
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
			cleanUpFailedVolume:  true,
			expCreateVolumeInput: &ec2.CreateVolumeInput{},
			expErr:               fmt.Errorf("failed to get an available volume in EC2: context deadline exceeded"),
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
				Iops: aws.Int64(100),
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
			expErr:               fmt.Errorf("invalid StorageClass parameters; specify either IOPS or IOPSPerGb, not both"),
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
				Iops: aws.Int64(200),
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
				Iops: aws.Int64(64000),
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
				Iops: aws.Int64(100),
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
				Iops: aws.Int64(2000),
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
				Iops: aws.Int64(64000),
			},
			expErr: nil,
		},
		{
			name:       "success: large io2 Block Express with too high iopsPerGB",
			volumeName: "vol-test-name",
			diskOptions: &DiskOptions{
				CapacityBytes: util.GiBToBytes(3333),
				Tags:          map[string]string{VolumeNameTagKey: "vol-test", AwsEbsDriverTagKey: "true"},
				VolumeType:    VolumeTypeIO2,
				IOPSPerGB:     100000,
				BlockExpress:  true,
			},
			expDisk: &Disk{
				VolumeID:         "vol-test",
				CapacityGiB:      3333,
				AvailabilityZone: defaultZone,
			},
			expCreateVolumeInput: &ec2.CreateVolumeInput{
				Iops: aws.Int64(256000),
			},
			expErr: nil,
		},
		{
			name:       "success: create volume when zone is snow and add tags",
			volumeName: "vol-test-name",
			diskOptions: &DiskOptions{
				CapacityBytes:    util.GiBToBytes(1),
				Tags:             map[string]string{VolumeNameTagKey: "vol-test", AwsEbsDriverTagKey: "true"},
				AvailabilityZone: snowZone,
				VolumeType:       "sbp1",
			},
			expCreateVolumeInput: &ec2.CreateVolumeInput{},
			expDisk: &Disk{
				VolumeID:         "vol-test",
				CapacityGiB:      1,
				AvailabilityZone: snowZone,
			},
			expErr: nil,
		},
		{
			name:       "fail: zone is snow and add tags throws error",
			volumeName: "vol-test-name",
			diskOptions: &DiskOptions{
				CapacityBytes:    util.GiBToBytes(1),
				Tags:             map[string]string{VolumeNameTagKey: "vol-test", AwsEbsDriverTagKey: "true"},
				AvailabilityZone: snowZone,
				VolumeType:       "sbg1",
			},
			expCreateVolumeInput: &ec2.CreateVolumeInput{},
			expCreateTagsErr:     fmt.Errorf("CreateTags generic error"),
			expDisk: &Disk{
				VolumeID:         "vol-test",
				CapacityGiB:      1,
				AvailabilityZone: snowZone,
			},
			expErr: fmt.Errorf("could not attach tags to volume: vol-test. CreateTags generic error"),
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
				Throughput: aws.Int64(250),
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
				Iops: aws.Int64(2000),
			},
			expErr: nil,
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
			expErr: fmt.Errorf("CreateDisk: multi-attach is only supported for io2 volumes"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockCtrl := gomock.NewController(t)
			mockEC2 := NewMockEC2API(mockCtrl)
			c := newCloud(mockEC2)

			volState := tc.volState
			if volState == "" {
				volState = "available"
			}

			vol := &ec2.Volume{
				VolumeId:         aws.String(tc.diskOptions.Tags[VolumeNameTagKey]),
				Size:             aws.Int64(util.BytesToGiB(tc.diskOptions.CapacityBytes)),
				State:            aws.String(volState),
				AvailabilityZone: aws.String(tc.diskOptions.AvailabilityZone),
				OutpostArn:       aws.String(tc.diskOptions.OutpostArn),
			}
			snapshot := &ec2.Snapshot{
				SnapshotId: aws.String(tc.diskOptions.SnapshotID),
				VolumeId:   aws.String("snap-test-volume"),
				State:      aws.String("completed"),
			}
			ctx := context.Background()

			if tc.expCreateVolumeInput != nil {
				matcher := eqCreateVolume(tc.expCreateVolumeInput)
				mockEC2.EXPECT().CreateVolumeWithContext(gomock.Any(), matcher).Return(vol, tc.expCreateVolumeErr)
				mockEC2.EXPECT().DescribeVolumesWithContext(gomock.Any(), gomock.Any()).Return(&ec2.DescribeVolumesOutput{Volumes: []*ec2.Volume{vol}}, tc.expDescVolumeErr).AnyTimes()
				if tc.diskOptions.AvailabilityZone == "snow" {
					mockEC2.EXPECT().CreateTagsWithContext(gomock.Any(), gomock.Any()).Return(&ec2.CreateTagsOutput{}, tc.expCreateTagsErr)
					mockEC2.EXPECT().DeleteVolumeWithContext(gomock.Any(), gomock.Any()).Return(&ec2.DeleteVolumeOutput{}, nil).AnyTimes()
				}
				if len(tc.diskOptions.SnapshotID) > 0 {
					mockEC2.EXPECT().DescribeSnapshotsWithContext(gomock.Any(), gomock.Any()).Return(&ec2.DescribeSnapshotsOutput{Snapshots: []*ec2.Snapshot{snapshot}}, nil).AnyTimes()
				}
				if tc.cleanUpFailedVolume == true {
					mockEC2.EXPECT().DeleteVolumeWithContext(gomock.Any(), gomock.Any()).Return(&ec2.DeleteVolumeOutput{}, nil)
				}
				if len(tc.diskOptions.AvailabilityZone) == 0 {
					mockEC2.EXPECT().DescribeAvailabilityZonesWithContext(gomock.Any(), gomock.Any()).Return(&ec2.DescribeAvailabilityZonesOutput{
						AvailabilityZones: []*ec2.AvailabilityZone{
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
			expErr:   fmt.Errorf("DeleteVolume generic error"),
		},
		{
			name:     "fail: DeleteVolume returned not found error",
			volumeID: "vol-test-1234",
			expResp:  false,
			expErr:   awserr.New("InvalidVolume.NotFound", "", nil),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockCtrl := gomock.NewController(t)
			mockEC2 := NewMockEC2API(mockCtrl)
			c := newCloud(mockEC2)

			ctx := context.Background()
			mockEC2.EXPECT().DeleteVolumeWithContext(gomock.Any(), gomock.Any()).Return(&ec2.DeleteVolumeOutput{}, tc.expErr)

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
					mockEC2.EXPECT().DescribeInstancesWithContext(gomock.Any(), instanceRequest).Return(newDescribeInstancesOutput(nodeID), nil),
					mockEC2.EXPECT().AttachVolumeWithContext(gomock.Any(), attachRequest).Return(createAttachVolumeOutput(volumeID, nodeID, path), nil),
					mockEC2.EXPECT().DescribeVolumesWithContext(gomock.Any(), volumeRequest).Return(createDescribeVolumesOutput([]*string{&volumeID}, nodeID, path, "attached"), nil),
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
				_, err := dm.NewDevice(fakeInstance, volumeID)
				assert.NoError(t, err)

				gomock.InOrder(
					mockEC2.EXPECT().DescribeInstancesWithContext(gomock.Any(), instanceRequest).Return(newDescribeInstancesOutput(nodeID, volumeID), nil),
					mockEC2.EXPECT().DescribeVolumesWithContext(gomock.Any(), volumeRequest).Return(createDescribeVolumesOutput([]*string{&volumeID}, nodeID, path, "attached"), nil))
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
					mockEC2.EXPECT().DescribeInstancesWithContext(gomock.Any(), instanceRequest).Return(newDescribeInstancesOutput(nodeID), nil),
					mockEC2.EXPECT().AttachVolumeWithContext(gomock.Any(), attachRequest).Return(nil, errors.New("AttachVolume error")),
				)
			},
		},
		{
			name:     "fail: AttachVolume returned error volumeInUse",
			volumeID: defaultVolumeID,
			nodeID:   defaultNodeID,
			path:     defaultPath,
			expErr:   fmt.Errorf("could not attach volume %q to node %q: %w", defaultVolumeID, defaultNodeID, ErrVolumeInUse),
			mockFunc: func(mockEC2 *MockEC2API, ctx context.Context, volumeID, nodeID, nodeID2, path string, dm dm.DeviceManager) {
				instanceRequest := createInstanceRequest(nodeID)
				attachRequest := createAttachRequest(volumeID, nodeID, path)

				gomock.InOrder(
					mockEC2.EXPECT().DescribeInstancesWithContext(ctx, instanceRequest).Return(newDescribeInstancesOutput(nodeID), nil),
					mockEC2.EXPECT().AttachVolumeWithContext(ctx, attachRequest).Return(nil, ErrVolumeInUse),
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
					Volumes: []*ec2.Volume{
						{
							VolumeId: aws.String(volumeID),
							Attachments: []*ec2.VolumeAttachment{
								{
									Device:     aws.String(path),
									InstanceId: aws.String(nodeID),
									State:      aws.String("attached"),
								},
								{
									Device:     aws.String(path),
									InstanceId: aws.String(nodeID2),
									State:      aws.String("attached"),
								},
							},
						},
					},
				}

				gomock.InOrder(
					mockEC2.EXPECT().DescribeInstancesWithContext(ctx, instanceRequest).Return(newDescribeInstancesOutput(nodeID), nil),
					mockEC2.EXPECT().AttachVolumeWithContext(ctx, attachRequest).Return(createAttachVolumeOutput(volumeID, nodeID, path), nil),
					mockEC2.EXPECT().DescribeVolumesWithContext(ctx, volumeRequest).Return(createDescribeVolumesOutput([]*string{&volumeID}, nodeID, path, "attached"), nil),

					mockEC2.EXPECT().DescribeInstancesWithContext(ctx, createInstanceRequest2).Return(newDescribeInstancesOutput(nodeID2), nil),
					mockEC2.EXPECT().AttachVolumeWithContext(ctx, attachRequest2).Return(createAttachVolumeOutput(volumeID, nodeID2, path), nil),
					mockEC2.EXPECT().DescribeVolumesWithContext(ctx, volumeRequest).Return(dvOutput, nil),
				)
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockCtrl := gomock.NewController(t)
			mockEC2 := NewMockEC2API(mockCtrl)
			c := newCloud(mockEC2)

			ctx := context.Background()
			dm := c.(*cloud).dm

			tc.mockFunc(mockEC2, ctx, tc.volumeID, tc.nodeID, tc.nodeID2, tc.path, dm)

			devicePath, err := c.AttachDisk(ctx, tc.volumeID, tc.nodeID)

			if tc.expErr != nil {
				assert.Error(t, err)
				assert.Equal(t, tc.expErr, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tc.path, devicePath)
			}

			if tc.nodeID2 != "" {
				devicePath, err := c.AttachDisk(ctx, tc.volumeID, tc.nodeID2)
				assert.NoError(t, err)
				assert.Equal(t, tc.path, devicePath)
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
					mockEC2.EXPECT().DescribeInstancesWithContext(gomock.Any(), instanceRequest).Return(newDescribeInstancesOutput(nodeID), nil),
					mockEC2.EXPECT().DetachVolumeWithContext(gomock.Any(), detachRequest).Return(nil, nil),
					mockEC2.EXPECT().DescribeVolumesWithContext(gomock.Any(), volumeRequest).Return(createDescribeVolumesOutput([]*string{&volumeID}, nodeID, "", "detached"), nil),
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
					mockEC2.EXPECT().DescribeInstancesWithContext(gomock.Any(), instanceRequest).Return(newDescribeInstancesOutput(nodeID), nil),
					mockEC2.EXPECT().DetachVolumeWithContext(gomock.Any(), detachRequest).Return(nil, errors.New("DetachVolume error")),
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
					mockEC2.EXPECT().DescribeInstancesWithContext(gomock.Any(), instanceRequest).Return(newDescribeInstancesOutput(nodeID), nil),
					mockEC2.EXPECT().DetachVolumeWithContext(gomock.Any(), detachRequest).Return(nil, ErrNotFound),
				)
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockCtrl := gomock.NewController(t)
			mockEC2 := NewMockEC2API(mockCtrl)
			c := newCloud(mockEC2)

			ctx := context.Background()
			tc.mockFunc(mockEC2, ctx, tc.volumeID, tc.nodeID)

			err := c.DetachDisk(ctx, tc.volumeID, tc.nodeID)

			if tc.expErr != nil {
				assert.Error(t, err)
				assert.Equal(t, tc.expErr, err)
			} else {
				assert.NoError(t, err)
			}

			mockCtrl.Finish()
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
			expErr:         fmt.Errorf("DescribeVolumes generic error"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockCtrl := gomock.NewController(t)
			mockEC2 := NewMockEC2API(mockCtrl)
			c := newCloud(mockEC2)

			vol := &ec2.Volume{
				VolumeId:         aws.String(tc.volumeName),
				Size:             aws.Int64(util.BytesToGiB(tc.volumeCapacity)),
				AvailabilityZone: aws.String(tc.availabilityZone),
				OutpostArn:       aws.String(tc.outpostArn),
				Tags: []*ec2.Tag{
					{
						Key:   aws.String(VolumeNameTagKey),
						Value: aws.String(tc.volumeName),
					},
				},
			}

			ctx := context.Background()
			mockEC2.EXPECT().DescribeVolumesWithContext(gomock.Any(), gomock.Any()).Return(&ec2.DescribeVolumesOutput{Volumes: []*ec2.Volume{vol}}, tc.expErr)

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
		attachments      *ec2.VolumeAttachment
		expErr           error
	}{
		{
			name:             "success: normal",
			volumeID:         "vol-test-1234",
			availabilityZone: expZone,
			attachments:      &ec2.VolumeAttachment{},
			expErr:           nil,
		},
		{
			name:             "success: outpost volume",
			volumeID:         "vol-test-1234",
			availabilityZone: expZone,
			outpostArn:       "arn:aws:outposts:us-west-2:111111111111:outpost/op-0aaa000a0aaaa00a0",
			attachments:      &ec2.VolumeAttachment{},
			expErr:           nil,
		},
		{
			name:             "success: attached instance list",
			volumeID:         "vol-test-1234",
			availabilityZone: expZone,
			outpostArn:       "arn:aws:outposts:us-west-2:111111111111:outpost/op-0aaa000a0aaaa00a0",
			attachments: &ec2.VolumeAttachment{
				InstanceId: aws.String("test-instance"),
				State:      aws.String("attached")},
			expErr: nil,
		},
		{
			name:     "fail: DescribeVolumes returned generic error",
			volumeID: "vol-test-1234",
			expErr:   fmt.Errorf("DescribeVolumes generic error"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockCtrl := gomock.NewController(t)
			mockEC2 := NewMockEC2API(mockCtrl)
			c := newCloud(mockEC2)

			ctx := context.Background()
			mockEC2.EXPECT().DescribeVolumesWithContext(gomock.Any(), gomock.Any()).Return(
				&ec2.DescribeVolumesOutput{
					Volumes: []*ec2.Volume{
						{
							VolumeId:         aws.String(tc.volumeID),
							AvailabilityZone: aws.String(tc.availabilityZone),
							OutpostArn:       aws.String(tc.outpostArn),
							Attachments:      []*ec2.VolumeAttachment{tc.attachments},
						},
					},
				},
				tc.expErr,
			)

			disk, err := c.GetDiskByID(ctx, tc.volumeID)
			if err != nil {
				if tc.expErr == nil {
					t.Fatalf("GetDisk() failed: expected no error, got: %v", err)
				}
			} else {
				if tc.expErr != nil {
					t.Fatal("GetDisk() failed: expected error, got nothing")
				}
				if disk.VolumeID != tc.volumeID {
					t.Fatalf("GetDisk() failed: expected ID %q, got %q", tc.volumeID, disk.VolumeID)
				}
				if tc.availabilityZone != disk.AvailabilityZone {
					t.Fatalf("GetDiskByName() failed: expected availabilityZone %q, got %q", tc.availabilityZone, disk.AvailabilityZone)
				}
				if disk.OutpostArn != tc.outpostArn {
					t.Fatalf("GetDisk() failed: expected outpostArn %q, got %q", tc.outpostArn, disk.OutpostArn)
				}
				if len(disk.Attachments) > 0 && disk.Attachments[0] != aws.StringValue(tc.attachments.InstanceId) {
					t.Fatalf("GetDisk() failed: expected attachment instance %q, got %q", aws.StringValue(tc.attachments.InstanceId), disk.Attachments[0])
				}
			}

			mockCtrl.Finish()
		})
	}
}

func TestCreateSnapshot(t *testing.T) {
	testCases := []struct {
		name            string
		snapshotName    string
		snapshotOptions *SnapshotOptions
		expInput        *ec2.CreateSnapshotInput
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
			expInput: &ec2.CreateSnapshotInput{
				VolumeId: aws.String("snap-test-volume"),
				DryRun:   aws.Bool(false),
				TagSpecifications: []*ec2.TagSpecification{
					{
						ResourceType: aws.String("snapshot"),
						Tags: []*ec2.Tag{
							{
								Key:   aws.String(SnapshotNameTagKey),
								Value: aws.String("snap-test-name"),
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
					},
				},
				Description: aws.String("Created by AWS EBS CSI driver for volume snap-test-volume"),
			},
			expSnapshot: &Snapshot{
				SourceVolumeID: "snap-test-volume",
			},
			expErr: nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockCtrl := gomock.NewController(t)
			mockEC2 := NewMockEC2API(mockCtrl)
			c := newCloud(mockEC2)

			ec2snapshot := &ec2.Snapshot{
				SnapshotId: aws.String(tc.snapshotOptions.Tags[SnapshotNameTagKey]),
				VolumeId:   aws.String("snap-test-volume"),
				State:      aws.String("completed"),
			}

			ctx := context.Background()
			mockEC2.EXPECT().CreateSnapshotWithContext(gomock.Any(), eqCreateSnapshotInput(tc.expInput)).Return(ec2snapshot, tc.expErr)
			mockEC2.EXPECT().DescribeSnapshotsWithContext(gomock.Any(), gomock.Any()).Return(&ec2.DescribeSnapshotsOutput{Snapshots: []*ec2.Snapshot{ec2snapshot}}, nil).AnyTimes()

			snapshot, err := c.CreateSnapshot(ctx, tc.expSnapshot.SourceVolumeID, tc.snapshotOptions)
			if err != nil {
				if tc.expErr == nil {
					t.Fatalf("CreateSnapshot() failed: expected no error, got: %v", err)
				}
			} else {
				if tc.expErr != nil {
					t.Fatal("CreateSnapshot() failed: expected error, got nothing")
				} else {
					if snapshot.SourceVolumeID != tc.expSnapshot.SourceVolumeID {
						t.Fatalf("CreateSnapshot() failed: expected source volume ID %s, got %v", tc.expSnapshot.SourceVolumeID, snapshot.SourceVolumeID)
					}
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
				Successful: []*ec2.EnableFastSnapshotRestoreSuccessItem{{
					AvailabilityZone: aws.String("us-west-2a,us-west-2b"),
					SnapshotId:       aws.String("snap-test-id")}},
				Unsuccessful: []*ec2.EnableFastSnapshotRestoreErrorItem{},
			},
			expErr: nil,
		},
		{
			name:              "fail: unsuccessful response",
			snapshotID:        "snap-test-id",
			availabilityZones: []string{"us-west-2a", "invalid-zone"},
			expOutput: &ec2.EnableFastSnapshotRestoresOutput{
				Unsuccessful: []*ec2.EnableFastSnapshotRestoreErrorItem{{
					SnapshotId: aws.String("snap-test-id"),
					FastSnapshotRestoreStateErrors: []*ec2.EnableFastSnapshotRestoreStateErrorItem{
						{AvailabilityZone: aws.String("us-west-2a,invalid-zone"),
							Error: &ec2.EnableFastSnapshotRestoreStateError{
								Message: aws.String("failed to create fast snapshot restore")}},
					},
				}},
			},
			expErr: fmt.Errorf("failed to create fast snapshot restores for snapshot"),
		},
		{
			name:              "fail: error",
			snapshotID:        "",
			availabilityZones: nil,
			expOutput:         nil,
			expErr:            fmt.Errorf("EnableFastSnapshotRestores error"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockCtrl := gomock.NewController(t)
			mockEC2 := NewMockEC2API(mockCtrl)
			c := newCloud(mockEC2)

			ctx := context.Background()
			mockEC2.EXPECT().EnableFastSnapshotRestoresWithContext(gomock.Any(), gomock.Any()).Return(tc.expOutput, tc.expErr).AnyTimes()

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
				AvailabilityZones: []*ec2.AvailabilityZone{
					{ZoneName: aws.String(expZone)},
				}},
			expErr: nil,
		},
		{
			name:             "fail: error",
			availabilityZone: "",
			expOutput:        nil,
			expErr:           fmt.Errorf("TestAvailabilityZones error"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockCtrl := gomock.NewController(t)
			mockEC2 := NewMockEC2API(mockCtrl)
			c := newCloud(mockEC2)

			ctx := context.Background()
			mockEC2.EXPECT().DescribeAvailabilityZonesWithContext(gomock.Any(), gomock.Any()).Return(tc.expOutput, tc.expErr).AnyTimes()

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
			expErr:       fmt.Errorf("DeleteSnapshot generic error"),
		},
		{
			name:         "fail: delete snapshot return not found error",
			snapshotName: "snap-test-name",
			expErr:       awserr.New("InvalidSnapshot.NotFound", "", nil),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockCtrl := gomock.NewController(t)
			mockEC2 := NewMockEC2API(mockCtrl)
			c := newCloud(mockEC2)

			ctx := context.Background()
			mockEC2.EXPECT().DeleteSnapshotWithContext(gomock.Any(), gomock.Any()).Return(&ec2.DeleteSnapshotOutput{}, tc.expErr)

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
		existingVolume      *ec2.Volume
		existingVolumeError awserr.Error
		modifiedVolume      *ec2.ModifyVolumeOutput
		modifiedVolumeError awserr.Error
		descModVolume       *ec2.DescribeVolumesModificationsOutput
		reqSizeGiB          int64
		modifyDiskOptions   *ModifyDiskOptions
		expErr              error
		shouldCallDescribe  bool
	}{
		{
			name:     "success: normal resize",
			volumeID: "vol-test",
			existingVolume: &ec2.Volume{
				VolumeId:         aws.String("vol-test"),
				Size:             aws.Int64(1),
				AvailabilityZone: aws.String(defaultZone),
			},
			modifiedVolume: &ec2.ModifyVolumeOutput{
				VolumeModification: &ec2.VolumeModification{
					VolumeId:          aws.String("vol-test"),
					TargetSize:        aws.Int64(2),
					ModificationState: aws.String(ec2.VolumeModificationStateCompleted),
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
			existingVolume: &ec2.Volume{
				VolumeId:         aws.String("vol-test"),
				Size:             aws.Int64(1),
				AvailabilityZone: aws.String(defaultZone),
			},
			modifiedVolume: &ec2.ModifyVolumeOutput{
				VolumeModification: &ec2.VolumeModification{
					VolumeId:          aws.String("vol-test"),
					TargetSize:        aws.Int64(2),
					ModificationState: aws.String(ec2.VolumeModificationStateModifying),
				},
			},
			descModVolume: &ec2.DescribeVolumesModificationsOutput{
				VolumesModifications: []*ec2.VolumeModification{
					{
						VolumeId:          aws.String("vol-test"),
						TargetSize:        aws.Int64(2),
						ModificationState: aws.String(ec2.VolumeModificationStateCompleted),
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
			existingVolume: &ec2.Volume{
				VolumeId:         aws.String("vol-test"),
				Size:             aws.Int64(2),
				AvailabilityZone: aws.String(defaultZone),
			},
			descModVolume: &ec2.DescribeVolumesModificationsOutput{
				VolumesModifications: []*ec2.VolumeModification{
					{
						VolumeId:          aws.String("vol-test"),
						TargetSize:        aws.Int64(2),
						ModificationState: aws.String(ec2.VolumeModificationStateCompleted),
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
			existingVolume: &ec2.Volume{
				VolumeId:   aws.String("vol-test"),
				VolumeType: aws.String("gp2"),
			},
			modifyDiskOptions: &ModifyDiskOptions{
				VolumeType: "GP3",
				IOPS:       3000,
				Throughput: 1000,
			},
			modifiedVolume: &ec2.ModifyVolumeOutput{
				VolumeModification: &ec2.VolumeModification{
					VolumeId:          aws.String("vol-test"),
					TargetVolumeType:  aws.String("GP3"),
					TargetIops:        aws.Int64(3000),
					TargetThroughput:  aws.Int64(1000),
					ModificationState: aws.String(ec2.VolumeModificationStateCompleted),
				},
			},
			expErr:             nil,
			shouldCallDescribe: true,
		},
		{
			name:     "success: modify size, IOPS, throughput and volume type",
			volumeID: "vol-test",
			existingVolume: &ec2.Volume{
				VolumeId:         aws.String("vol-test"),
				Size:             aws.Int64(1),
				AvailabilityZone: aws.String(defaultZone),
				VolumeType:       aws.String("gp2"),
				Iops:             aws.Int64(2000),
			},
			modifyDiskOptions: &ModifyDiskOptions{
				VolumeType: "GP3",
				IOPS:       3000,
				Throughput: 1000,
			},
			reqSizeGiB: 2,
			modifiedVolume: &ec2.ModifyVolumeOutput{
				VolumeModification: &ec2.VolumeModification{
					VolumeId:          aws.String("vol-test"),
					TargetSize:        aws.Int64(2),
					TargetVolumeType:  aws.String("GP3"),
					TargetIops:        aws.Int64(3000),
					TargetThroughput:  aws.Int64(1000),
					ModificationState: aws.String(ec2.VolumeModificationStateCompleted),
				},
			},
			expErr:             nil,
			shouldCallDescribe: true,
		},
		{
			name:                "fail: volume doesn't exist",
			volumeID:            "vol-test",
			existingVolumeError: awserr.New("InvalidVolume.NotFound", "", nil),
			reqSizeGiB:          2,
			expErr:              fmt.Errorf("ResizeDisk generic error"),
		},
		{
			name:     "failure: volume in modifying state",
			volumeID: "vol-test",
			existingVolume: &ec2.Volume{
				VolumeId:         aws.String("vol-test"),
				Size:             aws.Int64(1),
				AvailabilityZone: aws.String(defaultZone),
			},
			descModVolume: &ec2.DescribeVolumesModificationsOutput{
				VolumesModifications: []*ec2.VolumeModification{
					{
						VolumeId:          aws.String("vol-test"),
						TargetSize:        aws.Int64(2),
						ModificationState: aws.String(ec2.VolumeModificationStateModifying),
					},
				},
			},
			reqSizeGiB: 2,
			expErr:     fmt.Errorf("ResizeDisk generic error"),
		},
		{
			name:     "failure: ModifyVolume returned generic error",
			volumeID: "vol-test",
			modifyDiskOptions: &ModifyDiskOptions{
				VolumeType: "GP2",
				IOPS:       3000,
			},
			existingVolume: &ec2.Volume{
				VolumeId:         aws.String("vol-test"),
				AvailabilityZone: aws.String(defaultZone),
				VolumeType:       aws.String("gp2"),
			},
			modifiedVolumeError: awserr.New("InvalidParameterCombination", "The parameter iops is not supported for gp2 volumes", nil),
			expErr:              awserr.New("InvalidParameterCombination", "The parameter iops is not supported for gp2 volumes", nil),
		},
		{
			name:     "success: does not call ModifyVolume when no modification required",
			volumeID: "vol-test",
			existingVolume: &ec2.Volume{
				VolumeId:         aws.String("vol-test"),
				AvailabilityZone: aws.String(defaultZone),
				VolumeType:       aws.String("gp3"),
				Iops:             aws.Int64(3000),
			},
			modifyDiskOptions: &ModifyDiskOptions{
				VolumeType: "GP3",
				IOPS:       3000,
			},
			shouldCallDescribe: true,
		},
		{
			name:     "success: does not call ModifyVolume when no modification required (with size)",
			volumeID: "vol-test",
			existingVolume: &ec2.Volume{
				VolumeId:         aws.String("vol-test"),
				AvailabilityZone: aws.String(defaultZone),
				Size:             aws.Int64(13),
				Iops:             aws.Int64(3000),
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
			// reduce number of steps to reduce test time
			volumeModificationWaitSteps = 3
			c := newCloud(mockEC2)

			ctx := context.Background()
			if tc.existingVolume != nil || tc.existingVolumeError != nil {
				mockEC2.EXPECT().DescribeVolumesWithContext(gomock.Any(), gomock.Any()).Return(
					&ec2.DescribeVolumesOutput{
						Volumes: []*ec2.Volume{
							tc.existingVolume,
						},
					}, tc.existingVolumeError)

				if tc.shouldCallDescribe {
					newVolume := tc.existingVolume
					if tc.reqSizeGiB != 0 {
						newVolume.Size = aws.Int64(tc.reqSizeGiB)
					}
					if tc.modifyDiskOptions != nil {
						if tc.modifyDiskOptions.IOPS != 0 {
							newVolume.Iops = aws.Int64(int64(tc.modifyDiskOptions.IOPS))
						}
						if tc.modifyDiskOptions.Throughput != 0 {
							newVolume.Throughput = aws.Int64(int64(tc.modifyDiskOptions.Throughput))
						}
						if tc.modifyDiskOptions.VolumeType != "" {
							newVolume.VolumeType = aws.String(tc.modifyDiskOptions.VolumeType)
						}
					}
					mockEC2.EXPECT().DescribeVolumesWithContext(gomock.Any(), gomock.Any()).Return(
						&ec2.DescribeVolumesOutput{
							Volumes: []*ec2.Volume{
								newVolume,
							},
						}, tc.existingVolumeError)
				}
			}
			if tc.modifiedVolume != nil || tc.modifiedVolumeError != nil {
				mockEC2.EXPECT().ModifyVolumeWithContext(gomock.Any(), gomock.Any()).Return(tc.modifiedVolume, tc.modifiedVolumeError).AnyTimes()
			}
			if tc.descModVolume != nil {
				mockEC2.EXPECT().DescribeVolumesModificationsWithContext(gomock.Any(), gomock.Any()).Return(tc.descModVolume, nil).AnyTimes()
			} else {
				emptyOutput := &ec2.DescribeVolumesModificationsOutput{}
				mockEC2.EXPECT().DescribeVolumesModificationsWithContext(gomock.Any(), gomock.Any()).Return(emptyOutput, nil).AnyTimes()
			}

			newSize, err := c.ResizeOrModifyDisk(ctx, tc.volumeID, util.GiBToBytes(tc.reqSizeGiB), tc.modifyDiskOptions)
			if err != nil {
				if tc.expErr == nil {
					t.Fatalf("ResizeOrModifyDisk() failed: expected no error, got: %v", err)
				}
			} else {
				if tc.expErr != nil {
					t.Fatal("ResizeOrModifyDisk() failed: expected error, got nothing")
				} else {
					if tc.reqSizeGiB != newSize {
						t.Fatalf("ResizeOrModifyDisk() failed: expected capacity %d, got %d", tc.reqSizeGiB, newSize)
					}
				}
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
				SourceVolumeID: "snap-test-volume",
			},
			expErr: nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockCtrl := gomock.NewController(t)
			mockEC2 := NewMockEC2API(mockCtrl)
			c := newCloud(mockEC2)

			ec2snapshot := &ec2.Snapshot{
				SnapshotId: aws.String(tc.snapshotOptions.Tags[SnapshotNameTagKey]),
				VolumeId:   aws.String("snap-test-volume"),
				State:      aws.String("completed"),
			}

			ctx := context.Background()
			mockEC2.EXPECT().DescribeSnapshotsWithContext(gomock.Any(), gomock.Any()).Return(&ec2.DescribeSnapshotsOutput{Snapshots: []*ec2.Snapshot{ec2snapshot}}, nil)

			_, err := c.GetSnapshotByName(ctx, tc.snapshotOptions.Tags[SnapshotNameTagKey])
			if err != nil {
				if tc.expErr == nil {
					t.Fatalf("GetSnapshotByName() failed: expected no error, got: %v", err)
				}
			} else {
				if tc.expErr != nil {
					t.Fatal("GetSnapshotByName() failed: expected error, got nothing")
				}
			}

			mockCtrl.Finish()
		})
	}
}

func TestGetSnapshotByID(t *testing.T) {
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
				SourceVolumeID: "snap-test-volume",
			},
			expErr: nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockCtrl := gomock.NewController(t)
			mockEC2 := NewMockEC2API(mockCtrl)
			c := newCloud(mockEC2)

			ec2snapshot := &ec2.Snapshot{
				SnapshotId: aws.String(tc.snapshotOptions.Tags[SnapshotNameTagKey]),
				VolumeId:   aws.String("snap-test-volume"),
				State:      aws.String("completed"),
			}

			ctx := context.Background()
			mockEC2.EXPECT().DescribeSnapshotsWithContext(gomock.Any(), gomock.Any()).Return(&ec2.DescribeSnapshotsOutput{Snapshots: []*ec2.Snapshot{ec2snapshot}}, nil)

			_, err := c.GetSnapshotByID(ctx, tc.snapshotOptions.Tags[SnapshotNameTagKey])
			if err != nil {
				if tc.expErr == nil {
					t.Fatalf("GetSnapshotByName() failed: expected no error, got: %v", err)
				}
			} else {
				if tc.expErr != nil {
					t.Fatal("GetSnapshotByName() failed: expected error, got nothing")
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
				expSnapshots := []*Snapshot{
					{
						SourceVolumeID: "snap-test-volume1",
						SnapshotID:     "snap-test-name1",
					},
					{
						SourceVolumeID: "snap-test-volume2",
						SnapshotID:     "snap-test-name2",
					},
				}
				ec2Snapshots := []*ec2.Snapshot{
					{
						SnapshotId: aws.String(expSnapshots[0].SnapshotID),
						VolumeId:   aws.String("snap-test-volume1"),
						State:      aws.String("completed"),
					},
					{
						SnapshotId: aws.String(expSnapshots[1].SnapshotID),
						VolumeId:   aws.String("snap-test-volume2"),
						State:      aws.String("completed"),
					},
				}

				mockCtl := gomock.NewController(t)
				defer mockCtl.Finish()
				mockEC2 := NewMockEC2API(mockCtl)
				c := newCloud(mockEC2)

				ctx := context.Background()

				mockEC2.EXPECT().DescribeSnapshotsWithContext(gomock.Any(), gomock.Any()).Return(&ec2.DescribeSnapshotsOutput{Snapshots: ec2Snapshots}, nil)

				_, err := c.ListSnapshots(ctx, "", 0, "")
				if err != nil {
					t.Fatalf("ListSnapshots() failed: expected no error, got: %v", err)
				}
			},
		},
		{
			name: "success: with volume ID",
			testFunc: func(t *testing.T) {
				sourceVolumeID := "snap-test-volume"
				expSnapshots := []*Snapshot{
					{
						SourceVolumeID: sourceVolumeID,
						SnapshotID:     "snap-test-name1",
					},
					{
						SourceVolumeID: sourceVolumeID,
						SnapshotID:     "snap-test-name2",
					},
				}
				ec2Snapshots := []*ec2.Snapshot{
					{
						SnapshotId: aws.String(expSnapshots[0].SnapshotID),
						VolumeId:   aws.String(sourceVolumeID),
						State:      aws.String("completed"),
					},
					{
						SnapshotId: aws.String(expSnapshots[1].SnapshotID),
						VolumeId:   aws.String(sourceVolumeID),
						State:      aws.String("completed"),
					},
				}

				mockCtl := gomock.NewController(t)
				defer mockCtl.Finish()
				mockEC2 := NewMockEC2API(mockCtl)
				c := newCloud(mockEC2)

				ctx := context.Background()

				mockEC2.EXPECT().DescribeSnapshotsWithContext(gomock.Any(), gomock.Any()).Return(&ec2.DescribeSnapshotsOutput{Snapshots: ec2Snapshots}, nil)

				resp, err := c.ListSnapshots(ctx, sourceVolumeID, 0, "")
				if err != nil {
					t.Fatalf("ListSnapshots() failed: expected no error, got: %v", err)
				}

				if len(resp.Snapshots) != len(expSnapshots) {
					t.Fatalf("Expected %d snapshots, got %d", len(expSnapshots), len(resp.Snapshots))
				}

				for _, snap := range resp.Snapshots {
					if snap.SourceVolumeID != sourceVolumeID {
						t.Fatalf("Unexpected source volume.  Expected %s, got %s", sourceVolumeID, snap.SourceVolumeID)
					}
				}
			},
		},
		{
			name: "success: max results, next token",
			testFunc: func(t *testing.T) {
				maxResults := 5
				nextTokenValue := "nextTokenValue"
				var expSnapshots []*Snapshot
				for i := 0; i < maxResults*2; i++ {
					expSnapshots = append(expSnapshots, &Snapshot{
						SourceVolumeID: "snap-test-volume1",
						SnapshotID:     fmt.Sprintf("snap-test-name%d", i),
					})
				}

				var ec2Snapshots []*ec2.Snapshot
				for i := 0; i < maxResults*2; i++ {
					ec2Snapshots = append(ec2Snapshots, &ec2.Snapshot{
						SnapshotId: aws.String(expSnapshots[i].SnapshotID),
						VolumeId:   aws.String(fmt.Sprintf("snap-test-volume%d", i)),
						State:      aws.String("completed"),
					})
				}

				mockCtl := gomock.NewController(t)
				defer mockCtl.Finish()
				mockEC2 := NewMockEC2API(mockCtl)
				c := newCloud(mockEC2)

				ctx := context.Background()

				firstCall := mockEC2.EXPECT().DescribeSnapshotsWithContext(gomock.Any(), gomock.Any()).Return(&ec2.DescribeSnapshotsOutput{
					Snapshots: ec2Snapshots[:maxResults],
					NextToken: aws.String(nextTokenValue),
				}, nil)
				secondCall := mockEC2.EXPECT().DescribeSnapshotsWithContext(gomock.Any(), gomock.Any()).Return(&ec2.DescribeSnapshotsOutput{
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
			name: "fail: AWS DescribeSnapshotsWithContext error",
			testFunc: func(t *testing.T) {
				mockCtl := gomock.NewController(t)
				defer mockCtl.Finish()
				mockEC2 := NewMockEC2API(mockCtl)
				c := newCloud(mockEC2)

				ctx := context.Background()

				mockEC2.EXPECT().DescribeSnapshotsWithContext(gomock.Any(), gomock.Any()).Return(nil, errors.New("test error"))

				if _, err := c.ListSnapshots(ctx, "", 0, ""); err == nil {
					t.Fatalf("ListSnapshots() failed: expected an error, got none")
				}
			},
		},
		{
			name: "fail: no snapshots ErrNotFound",
			testFunc: func(t *testing.T) {
				mockCtl := gomock.NewController(t)
				defer mockCtl.Finish()
				mockEC2 := NewMockEC2API(mockCtl)
				c := newCloud(mockEC2)

				ctx := context.Background()

				mockEC2.EXPECT().DescribeSnapshotsWithContext(gomock.Any(), gomock.Any()).Return(&ec2.DescribeSnapshotsOutput{}, nil)

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
		name             string
		volumeID         string
		expectedState    string
		expectedInstance string
		expectedDevice   string
		alreadyAssigned  bool
		expectError      bool
	}{
		{
			name:             "success: attached",
			volumeID:         "vol-test-1234",
			expectedState:    volumeAttachedState,
			expectedInstance: "1234",
			expectedDevice:   defaultPath,
			alreadyAssigned:  false,
			expectError:      false,
		},
		{
			name:             "success: detached",
			volumeID:         "vol-test-1234",
			expectedState:    volumeDetachedState,
			expectedInstance: "1234",
			expectedDevice:   defaultPath,
			alreadyAssigned:  false,
			expectError:      false,
		},
		{
			name:             "success: disk not found, assumed detached",
			volumeID:         "vol-test-1234",
			expectedState:    volumeDetachedState,
			expectedInstance: "1234",
			expectedDevice:   defaultPath,
			alreadyAssigned:  false,
			expectError:      false,
		},
		{
			name:             "success: multiple attachments with Multi-Attach enabled",
			volumeID:         "vol-test-1234",
			expectedState:    volumeAttachedState,
			expectedInstance: "1234",
			expectedDevice:   defaultPath,
			alreadyAssigned:  false,
			expectError:      false,
		},
		{
			name:             "failure: disk not found, expected attached",
			volumeID:         "vol-test-1234",
			expectedState:    volumeAttachedState,
			expectedInstance: "1234",
			expectedDevice:   defaultPath,
			alreadyAssigned:  false,
			expectError:      true,
		},
		{
			name:             "failure: unexpected device",
			volumeID:         "vol-test-1234",
			expectedState:    volumeAttachedState,
			expectedInstance: "1234",
			expectedDevice:   "/dev/xvdab",
			alreadyAssigned:  false,
			expectError:      true,
		},
		{
			name:             "failure: unexpected instance",
			volumeID:         "vol-test-1234",
			expectedState:    volumeAttachedState,
			expectedInstance: "1235",
			expectedDevice:   defaultPath,
			alreadyAssigned:  false,
			expectError:      true,
		},
		{
			name:             "failure: already assigned but wrong state",
			volumeID:         "vol-test-1234",
			expectedState:    volumeAttachedState,
			expectedInstance: "1234",
			expectedDevice:   defaultPath,
			alreadyAssigned:  true,
			expectError:      true,
		},
		{
			name:             "failure: multiple attachments with Multi-Attach disabled",
			volumeID:         "vol-test-1234",
			expectedState:    volumeAttachedState,
			expectedInstance: "1234",
			expectedDevice:   defaultPath,
			alreadyAssigned:  false,
			expectError:      true,
		},
	}

	volumeAttachmentStatePollSteps = 1

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockCtrl := gomock.NewController(t)
			mockEC2 := NewMockEC2API(mockCtrl)
			c := newCloud(mockEC2)

			attachedVol := &ec2.Volume{
				VolumeId:    aws.String(tc.volumeID),
				Attachments: []*ec2.VolumeAttachment{{Device: aws.String(defaultPath), InstanceId: aws.String("1234"), State: aws.String("attached")}},
			}

			attachingVol := &ec2.Volume{
				VolumeId:    aws.String(tc.volumeID),
				Attachments: []*ec2.VolumeAttachment{{Device: aws.String(defaultPath), InstanceId: aws.String("1234"), State: aws.String("attaching")}},
			}

			detachedVol := &ec2.Volume{
				VolumeId:    aws.String(tc.volumeID),
				Attachments: []*ec2.VolumeAttachment{{Device: aws.String(defaultPath), InstanceId: aws.String("1234"), State: aws.String("detached")}},
			}

			multipleAttachmentsVol := &ec2.Volume{
				VolumeId:           aws.String(tc.volumeID),
				Attachments:        []*ec2.VolumeAttachment{{Device: aws.String(defaultPath), InstanceId: aws.String("1235"), State: aws.String("attached")}, {Device: aws.String(defaultPath), InstanceId: aws.String("1234"), State: aws.String("attached")}},
				MultiAttachEnabled: aws.Bool(false),
			}

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			switch tc.name {
			case "success: detached", "failure: already assigned but wrong state":
				mockEC2.EXPECT().DescribeVolumesWithContext(gomock.Any(), gomock.Any()).Return(&ec2.DescribeVolumesOutput{Volumes: []*ec2.Volume{detachedVol}}, nil).AnyTimes()
			case "success: disk not found, assumed detached", "failure: disk not found, expected attached":
				mockEC2.EXPECT().DescribeVolumesWithContext(gomock.Any(), gomock.Any()).Return(nil, awserr.New("InvalidVolume.NotFound", "foo", fmt.Errorf(""))).AnyTimes()
			case "success: multiple attachments with Multi-Attach enabled":
				multipleAttachmentsVol.MultiAttachEnabled = aws.Bool(true)
				mockEC2.EXPECT().DescribeVolumesWithContext(gomock.Any(), gomock.Any()).Return(&ec2.DescribeVolumesOutput{Volumes: []*ec2.Volume{multipleAttachmentsVol}}, nil).AnyTimes()
			case "failure: multiple attachments with Multi-Attach disabled":
				mockEC2.EXPECT().DescribeVolumesWithContext(gomock.Any(), gomock.Any()).Return(&ec2.DescribeVolumesOutput{Volumes: []*ec2.Volume{multipleAttachmentsVol}}, nil).AnyTimes()
			case "failure: disk still attaching":
				mockEC2.EXPECT().DescribeVolumesWithContext(gomock.Any(), gomock.Any()).Return(&ec2.DescribeVolumesOutput{Volumes: []*ec2.Volume{attachingVol}}, nil).AnyTimes()
			case "failure: context cancelled":
				mockEC2.EXPECT().DescribeVolumesWithContext(ctx, gomock.Any()).Return(&ec2.DescribeVolumesOutput{Volumes: []*ec2.Volume{attachingVol}}, nil).AnyTimes()
				cancel()
			default:
				mockEC2.EXPECT().DescribeVolumesWithContext(gomock.Any(), gomock.Any()).Return(&ec2.DescribeVolumesOutput{Volumes: []*ec2.Volume{attachedVol}}, nil).AnyTimes()
			}

			attachment, err := c.WaitForAttachmentState(ctx, tc.volumeID, tc.expectedState, tc.expectedInstance, tc.expectedDevice, tc.alreadyAssigned)

			if tc.expectError {
				if err == nil {
					t.Fatal("WaitForAttachmentState() failed: expected error, got nothing")
				}
			} else {
				if err != nil {
					t.Fatalf("WaitForAttachmentState() failed: expected no error, got %v", err)
				}

				if tc.expectedState == volumeAttachedState {
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

func newCloud(mockEC2 ec2iface.EC2API) Cloud {
	c := &cloud{
		region: "test-region",
		dm:     dm.NewDeviceManager(),
		ec2:    mockEC2,
	}
	return c
}

func newDescribeInstancesOutput(nodeID string, volumeID ...string) *ec2.DescribeInstancesOutput {
	instance := &ec2.Instance{
		InstanceId: aws.String(nodeID),
	}

	if len(volumeID) > 0 && volumeID[0] != "" {
		instance.BlockDeviceMappings = []*ec2.InstanceBlockDeviceMapping{
			{
				DeviceName: aws.String(defaultPath),
				Ebs: &ec2.EbsInstanceBlockDevice{
					VolumeId: aws.String(volumeID[0]),
				},
			},
		}
	}

	return &ec2.DescribeInstancesOutput{
		Reservations: []*ec2.Reservation{
			{
				Instances: []*ec2.Instance{
					instance,
				},
			},
		},
	}
}

func newFakeInstance(instanceID, volumeID, devicePath string) *ec2.Instance {
	return &ec2.Instance{
		InstanceId: aws.String(instanceID),
		BlockDeviceMappings: []*ec2.InstanceBlockDeviceMapping{
			{
				DeviceName: aws.String(devicePath),
				Ebs:        &ec2.EbsInstanceBlockDevice{VolumeId: aws.String(volumeID)},
			},
		},
	}
}

func createVolumeRequest(volumeID string) *ec2.DescribeVolumesInput {
	return &ec2.DescribeVolumesInput{
		VolumeIds: []*string{
			aws.String(volumeID),
		},
	}
}

func createInstanceRequest(nodeID string) *ec2.DescribeInstancesInput {
	return &ec2.DescribeInstancesInput{
		InstanceIds: []*string{
			aws.String(nodeID),
		},
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
	volumes := make([]*ec2.Volume, 0, len(volumeIDs))

	for _, volumeID := range volumeIDs {
		volumes = append(volumes, &ec2.Volume{
			VolumeId: volumeID,
			Attachments: []*ec2.VolumeAttachment{
				{
					Device:     aws.String(path),
					InstanceId: aws.String(nodeID),
					State:      aws.String(state),
				},
			},
		})
	}

	return &ec2.DescribeVolumesOutput{
		Volumes: volumes,
	}
}

func createAttachVolumeOutput(volumeID, nodeID, path string) *ec2.VolumeAttachment {
	return &ec2.VolumeAttachment{
		VolumeId:   aws.String(volumeID),
		Device:     aws.String(path),
		InstanceId: aws.String(nodeID),
		State:      aws.String("attached"),
	}
}

type eqCreateSnapshotInputMatcher struct {
	expected *ec2.CreateSnapshotInput
}

func eqCreateSnapshotInput(expected *ec2.CreateSnapshotInput) gomock.Matcher {
	return &eqCreateSnapshotInputMatcher{expected}
}

func (m *eqCreateSnapshotInputMatcher) Matches(x interface{}) bool {
	input, ok := x.(*ec2.CreateSnapshotInput)
	if !ok {
		return false
	}

	if input != nil {
		for _, ts := range input.TagSpecifications {
			// Because these tags are generated from a map
			// which has a random order.
			sort.SliceStable(ts.Tags, func(i, j int) bool {
				return *ts.Tags[i].Key < *ts.Tags[j].Key
			})
		}
	}

	return reflect.DeepEqual(m.expected, input)
}

func (m *eqCreateSnapshotInputMatcher) String() string {
	return m.expected.String()
}

type eqCreateVolumeMatcher struct {
	expected *ec2.CreateVolumeInput
}

func eqCreateVolume(expected *ec2.CreateVolumeInput) gomock.Matcher {
	return &eqCreateVolumeMatcher{expected}
}

func (m *eqCreateVolumeMatcher) Matches(x interface{}) bool {
	input, ok := x.(*ec2.CreateVolumeInput)
	if !ok {
		return false
	}

	if input == nil {
		return false
	}
	// TODO: Check all inputs
	ret := reflect.DeepEqual(m.expected.Iops, input.Iops) && reflect.DeepEqual(m.expected.Throughput, input.Throughput)
	return ret
}

func (m *eqCreateVolumeMatcher) String() string {
	return m.expected.String()
}
