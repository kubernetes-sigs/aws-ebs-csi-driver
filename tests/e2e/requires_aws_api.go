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

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/google/uuid"
	volumesnapshotv1 "github.com/kubernetes-csi/external-snapshotter/client/v4/apis/volumesnapshot/v1"
	awscloud "github.com/kubernetes-sigs/aws-ebs-csi-driver/pkg/cloud"
	ebscsidriver "github.com/kubernetes-sigs/aws-ebs-csi-driver/pkg/driver"
	"github.com/kubernetes-sigs/aws-ebs-csi-driver/tests/e2e/driver"
	"github.com/kubernetes-sigs/aws-ebs-csi-driver/tests/e2e/testsuites"
	. "github.com/onsi/ginkgo/v2"
	v1 "k8s.io/api/core/v1"
	clientset "k8s.io/client-go/kubernetes"
	restclientset "k8s.io/client-go/rest"
	"k8s.io/kubernetes/test/e2e/framework"
	admissionapi "k8s.io/pod-security-admission/api"
)

const testTagNamePrefix = "testTag"
const testTagValue = "3.1415926"

// generateTagName appends a random uuid to tag name to prevent clashes on parallel e2e test runs on shared cluster.
func generateTagName() string {
	return testTagNamePrefix + uuid.NewString()[:8]
}

func validateEc2Snapshot(ctx context.Context, ec2Client *ec2.Client, input *ec2.DescribeSnapshotsInput) *ec2.DescribeSnapshotsOutput {
	describeResult, err := ec2Client.DescribeSnapshots(ctx, input)
	if err != nil {
		Fail(fmt.Sprintf("failed to describe snapshot: %v", err))
	}

	if len(describeResult.Snapshots) != 1 {
		Fail(fmt.Sprintf("expected 1 snapshot, got %d", len(describeResult.Snapshots)))
	}

	return describeResult
}

var _ = Describe("[ebs-csi-e2e] [single-az] [requires-aws-api] Dynamic Provisioning", func() {
	f := framework.NewDefaultFramework("ebs")
	f.NamespacePodSecurityEnforceLevel = admissionapi.LevelPrivileged

	var (
		cs          clientset.Interface
		snapshotrcs restclientset.Interface
		ns          *v1.Namespace
		ebsDriver   driver.PVTestDriver
	)

	BeforeEach(func() {
		cs = f.ClientSet
		var err error
		snapshotrcs, err = restClient(testsuites.SnapshotAPIGroup, testsuites.APIVersionv1)
		if err != nil {
			Fail(fmt.Sprintf("could not get rest clientset: %v", err))
		}
		ns = f.Namespace
		ebsDriver = driver.InitEbsCSIDriver()
	})

	// Tests that require that the e2e runner has access to the AWS API
	cfg, err := config.LoadDefaultConfig(context.Background())
	if err != nil {
		Fail(fmt.Sprintf("failed to load AWS config: %v", err))
	}
	ec2Client := ec2.NewFromConfig(cfg)

	It("should create a volume with additional tags", func() {
		testTag := generateTagName()
		pods := []testsuites.PodDetails{
			{
				Cmd: testsuites.PodCmdWriteToVolume("/mnt/test-1"),
				Volumes: []testsuites.VolumeDetails{
					{
						CreateVolumeParameters: map[string]string{
							ebscsidriver.VolumeTypeKey: awscloud.VolumeTypeGP3,
							ebscsidriver.FSTypeKey:     ebscsidriver.FSTypeExt4,
							ebscsidriver.TagKeyPrefix:  fmt.Sprintf("%s=%s", testTag, testTagValue),
						},
						ClaimSize:   driver.MinimumSizeForVolumeType(awscloud.VolumeTypeGP3),
						VolumeMount: testsuites.DefaultGeneratedVolumeMount,
					},
				},
			},
		}
		test := testsuites.DynamicallyProvisionedCmdVolumeTest{
			CSIDriver: ebsDriver,
			Pods:      pods,
			ValidateFunc: func() {
				result, err := ec2Client.DescribeVolumes(context.Background(), &ec2.DescribeVolumesInput{
					Filters: []types.Filter{
						{
							Name:   aws.String("tag:" + testTag),
							Values: []string{(testTagValue)},
						},
					},
				})
				if err != nil {
					Fail(fmt.Sprintf("failed to describe volume: %v", err))
				}

				if len(result.Volumes) != 1 {
					Fail(fmt.Sprintf("expected 1 volume, got %d", len(result.Volumes)))
				}
			},
		}
		test.Run(cs, ns)
	})

	It("should create a snapshot with additional tags", func() {
		testTag := generateTagName()
		pod := testsuites.PodDetails{
			Cmd: testsuites.PodCmdWriteToVolume("/mnt/test-1"),
			Volumes: []testsuites.VolumeDetails{
				{
					CreateVolumeParameters: map[string]string{
						ebscsidriver.VolumeTypeKey: awscloud.VolumeTypeGP3,
						ebscsidriver.FSTypeKey:     ebscsidriver.FSTypeExt4,
					},
					ClaimSize:   driver.MinimumSizeForVolumeType(awscloud.VolumeTypeGP3),
					VolumeMount: testsuites.DefaultGeneratedVolumeMount,
				},
			},
		}
		restoredPod := testsuites.PodDetails{
			Cmd: testsuites.PodCmdGrepVolumeData("/mnt/test-1"),
			Volumes: []testsuites.VolumeDetails{
				{
					CreateVolumeParameters: map[string]string{
						ebscsidriver.VolumeTypeKey: awscloud.VolumeTypeGP3,
						ebscsidriver.FSTypeKey:     ebscsidriver.FSTypeExt4,
					},
					ClaimSize:   driver.MinimumSizeForVolumeType(awscloud.VolumeTypeGP3),
					VolumeMount: testsuites.DefaultGeneratedVolumeMount,
				},
			},
		}
		test := testsuites.DynamicallyProvisionedVolumeSnapshotTest{
			CSIDriver:   ebsDriver,
			Pod:         pod,
			RestoredPod: restoredPod,
			Parameters: map[string]string{
				ebscsidriver.TagKeyPrefix: fmt.Sprintf("%s=%s", testTag, testTagValue),
			},
			ValidateFunc: func(_ *volumesnapshotv1.VolumeSnapshot) {
				validateEc2Snapshot(context.Background(), ec2Client, &ec2.DescribeSnapshotsInput{
					Filters: []types.Filter{
						{
							Name:   aws.String("tag:" + testTag),
							Values: []string{testTagValue},
						},
					},
				})
			},
		}
		test.Run(cs, snapshotrcs, ns)
	})

	It("should create a volume from a snapshot with initialization rate and blockAttachUntilInitialized then wait to attach volume until it is initialized", func() {
		restoredTagKey := generateTagName()
		restoredTagVal := "volume-init-rate-and-block-attach-test"
		initRate := "200"

		originalPod := testsuites.PodDetails{
			Cmd: testsuites.PodCmdWriteToVolume("/mnt/test-1"),
			Volumes: []testsuites.VolumeDetails{
				{
					CreateVolumeParameters: map[string]string{
						ebscsidriver.VolumeTypeKey: awscloud.VolumeTypeGP3,
						ebscsidriver.FSTypeKey:     ebscsidriver.FSTypeExt4,
					},
					ClaimSize:   driver.MinimumSizeForVolumeType(awscloud.VolumeTypeGP3),
					VolumeMount: testsuites.DefaultGeneratedVolumeMount,
				},
			},
		}

		restoredPod := testsuites.PodDetails{
			Cmd: testsuites.PodCmdGrepVolumeData("/mnt/test-1"),
			Volumes: []testsuites.VolumeDetails{
				{
					CreateVolumeParameters: map[string]string{
						ebscsidriver.VolumeTypeKey:                  awscloud.VolumeTypeGP3,
						ebscsidriver.FSTypeKey:                      ebscsidriver.FSTypeExt4,
						ebscsidriver.VolumeInitializationRateKey:    initRate,
						ebscsidriver.TagKeyPrefix:                   fmt.Sprintf("%s=%s", restoredTagKey, restoredTagVal),
						ebscsidriver.BlockAttachUntilInitializedKey: "true",
					},
					ClaimSize:   driver.MinimumSizeForVolumeType(awscloud.VolumeTypeGP3),
					VolumeMount: testsuites.DefaultGeneratedVolumeMount,
				},
			},
		}

		test := testsuites.DynamicallyProvisionedVolumeSnapshotTest{
			CSIDriver:   ebsDriver,
			Pod:         originalPod,
			RestoredPod: restoredPod,

			ValidateFunc: func(snapshot *volumesnapshotv1.VolumeSnapshot) {
				intInitRate, err := strconv.Atoi(initRate)
				if err != nil {
					Fail(fmt.Sprintf("failed to parse initRate: %v", err))
				}

				result, err := ec2Client.DescribeVolumes(context.Background(), &ec2.DescribeVolumesInput{
					Filters: []types.Filter{
						{
							Name:   aws.String("tag:" + restoredTagKey),
							Values: []string{restoredTagVal},
						},
					},
				})
				if err != nil {
					Fail(fmt.Sprintf("failed to describe volume: %v", err))
				}
				if len(result.Volumes) != 1 {
					Fail(fmt.Sprintf("expected 1 volume, got %d", len(result.Volumes)))
				}

				vol := result.Volumes[0]
				if vol.VolumeInitializationRate == nil {
					Fail("VolumeInitializationRate is unexpectedly nil")
				}
				if *vol.VolumeInitializationRate != int32(intInitRate) {
					Fail(fmt.Sprintf("VolumeInitializationRate mismatch: got %d, expected %d", *vol.VolumeInitializationRate, intInitRate))
				}

				// Need volID from DescribeVolumes because we can't filter DescribeVolumeStatus results by volume tag
				volID := aws.ToString(vol.VolumeId)

				resultDVS, err := ec2Client.DescribeVolumeStatus(context.Background(), &ec2.DescribeVolumeStatusInput{
					VolumeIds: []string{volID},
				})
				if err != nil {
					Fail(fmt.Sprintf("failed to describe volume status: %v", err))
				}

				statuses := resultDVS.VolumeStatuses
				if len(statuses) != 1 {
					Fail(fmt.Sprintf("expected 1 volume status, got %d", len(statuses)))
				}

				// Purposefully don't rely on helper function from cloud pkg
				volumeInitialized := false
				for _, detail := range statuses[0].VolumeStatus.Details {
					if detail.Name == types.VolumeStatusNameInitializationState && detail.Status != nil && *detail.Status == awscloud.VolumeStatusInitializedState {
						volumeInitialized = true
					}
				}
				if !volumeInitialized {
					Fail("expected volume status item 'initialization-state' to be " + awscloud.VolumeStatusInitializedState)
				}
			},
		}
		test.Run(cs, snapshotrcs, ns)
	})

	It("should create a snapshot with FSR enabled", func() {
		azList, err := ec2Client.DescribeAvailabilityZones(context.Background(), &ec2.DescribeAvailabilityZonesInput{})
		if err != nil {
			Fail(fmt.Sprintf("failed to list AZs: %v", err))
		}
		fsrAvailabilityZone := *azList.AvailabilityZones[0].ZoneName

		pod := testsuites.PodDetails{
			Cmd: testsuites.PodCmdWriteToVolume("/mnt/test-1"),
			Volumes: []testsuites.VolumeDetails{
				{
					CreateVolumeParameters: map[string]string{
						ebscsidriver.VolumeTypeKey: awscloud.VolumeTypeGP3,
						ebscsidriver.FSTypeKey:     ebscsidriver.FSTypeExt4,
					},
					ClaimSize:   driver.MinimumSizeForVolumeType(awscloud.VolumeTypeGP3),
					VolumeMount: testsuites.DefaultGeneratedVolumeMount,
				},
			},
		}
		restoredPod := testsuites.PodDetails{
			Cmd: testsuites.PodCmdGrepVolumeData("/mnt/test-1"),
			Volumes: []testsuites.VolumeDetails{
				{
					CreateVolumeParameters: map[string]string{
						ebscsidriver.VolumeTypeKey: awscloud.VolumeTypeGP3,
						ebscsidriver.FSTypeKey:     ebscsidriver.FSTypeExt4,
					},
					ClaimSize:   driver.MinimumSizeForVolumeType(awscloud.VolumeTypeGP3),
					VolumeMount: testsuites.DefaultGeneratedVolumeMount,
				},
			},
		}
		test := testsuites.DynamicallyProvisionedVolumeSnapshotTest{
			CSIDriver:   ebsDriver,
			Pod:         pod,
			RestoredPod: restoredPod,
			Parameters: map[string]string{
				ebscsidriver.FastSnapshotRestoreAvailabilityZones: fsrAvailabilityZone,
			},
			ValidateFunc: func(snapshot *volumesnapshotv1.VolumeSnapshot) {
				describeResult := validateEc2Snapshot(context.Background(), ec2Client, &ec2.DescribeSnapshotsInput{
					Filters: []types.Filter{
						{
							Name:   aws.String("tag:" + awscloud.SnapshotNameTagKey),
							Values: []string{"snapshot-" + string(snapshot.UID)},
						},
					},
				})

				result, err := ec2Client.DescribeFastSnapshotRestores(context.Background(), &ec2.DescribeFastSnapshotRestoresInput{
					Filters: []types.Filter{
						{
							Name:   aws.String("snapshot-id"),
							Values: []string{*describeResult.Snapshots[0].SnapshotId},
						},
					},
				})
				if err != nil {
					Fail(fmt.Sprintf("failed to list AZs: %v", err))
				}

				if len(result.FastSnapshotRestores) != 1 {
					Fail(fmt.Sprintf("expected 1 FSR, got %d", len(result.FastSnapshotRestores)))
				}

				if *result.FastSnapshotRestores[0].AvailabilityZone != fsrAvailabilityZone {
					Fail(fmt.Sprintf("expected FSR to be enabled for %s, got %s", fsrAvailabilityZone, *result.FastSnapshotRestores[0].AvailabilityZone))
				}
			},
		}
		test.Run(cs, snapshotrcs, ns)
	})
	It("should copy a volume with different volume parameters", func() {
		pod := testsuites.PodDetails{
			Cmd: testsuites.PodCmdWriteToVolume("/mnt/test-1"),
			Volumes: []testsuites.VolumeDetails{
				{
					CreateVolumeParameters: map[string]string{
						ebscsidriver.EncryptedKey:  "true",
						ebscsidriver.VolumeTypeKey: awscloud.VolumeTypeGP2,
						ebscsidriver.FSTypeKey:     ebscsidriver.FSTypeExt4,
					},
					ClaimSize:   driver.MinimumSizeForVolumeType(awscloud.VolumeTypeIO2),
					VolumeMount: testsuites.DefaultGeneratedVolumeMount,
				},
			},
		}
		clonedPod := testsuites.PodDetails{
			Cmd: testsuites.PodCmdWriteToVolume("/mnt/test-1"),
			Volumes: []testsuites.VolumeDetails{
				{
					CreateVolumeParameters: map[string]string{
						ebscsidriver.EncryptedKey:  "true",
						ebscsidriver.VolumeTypeKey: awscloud.VolumeTypeIO2,
						ebscsidriver.IopsKey:       testsuites.DefaultIopsIoVolumes,
					},
					ClaimSize:   driver.MinimumSizeForVolumeType(awscloud.VolumeTypeIO2),
					VolumeMount: testsuites.DefaultGeneratedVolumeMount,
				},
			},
		}
		test := testsuites.DynamicallyProvisionedCopyVolumeTest{
			CSIDriver: ebsDriver,
			Pod:       pod,
			ClonedPod: clonedPod,
			ValidateFunc: func() {
				result, err := ec2Client.DescribeVolumes(context.Background(), &ec2.DescribeVolumesInput{
					VolumeIds: []string{clonedPod.Volumes[0].VolumeID},
				})
				if err != nil {
					Fail(fmt.Sprintf("failed to describe volume: %v", err))
				}

				if len(result.Volumes) != 1 {
					Fail(fmt.Sprintf("expected 1 volume, got %d", len(result.Volumes)))
				}

				if result.Volumes[0].VolumeType != awscloud.VolumeTypeIO1 {
					Fail(fmt.Sprintf("expected volume type %s, got %s", awscloud.VolumeTypeIO1, result.Volumes[0].VolumeType))
				}
			},
		}
		test.Run(cs, ns)
	})
	// This is a test to ensure driver logic is handled correctly when all params are omitted.
	It("should copy a volume with all omitted params and not get same IOPS as source ", func() {
		sourceIops := "3012"
		pod := testsuites.PodDetails{
			Cmd: testsuites.PodCmdWriteToVolume("/mnt/test-1"),
			Volumes: []testsuites.VolumeDetails{
				{
					CreateVolumeParameters: map[string]string{
						ebscsidriver.EncryptedKey:  "true",
						ebscsidriver.VolumeTypeKey: awscloud.VolumeTypeGP3,
						ebscsidriver.IopsKey:       sourceIops,
						ebscsidriver.FSTypeKey:     ebscsidriver.FSTypeExt4,
					},
					ClaimSize:   "10Gi", // Must be higher than minimum to be within max IOPS ratio.
					VolumeMount: testsuites.DefaultGeneratedVolumeMount,
				},
			},
		}
		clonedPod := testsuites.PodDetails{
			Cmd: testsuites.PodCmdWriteToVolume("/mnt/test-1"),
			Volumes: []testsuites.VolumeDetails{
				{
					CreateVolumeParameters: map[string]string{
						ebscsidriver.VolumeTypeKey: awscloud.VolumeTypeGP3,
					},
					ClaimSize:   "10Gi",
					VolumeMount: testsuites.DefaultGeneratedVolumeMount,
				},
			},
		}
		test := testsuites.DynamicallyProvisionedCopyVolumeTest{
			CSIDriver: ebsDriver,
			Pod:       pod,
			ClonedPod: clonedPod,
			ValidateFunc: func() {
				result, err := ec2Client.DescribeVolumes(context.Background(), &ec2.DescribeVolumesInput{
					VolumeIds: []string{clonedPod.Volumes[0].VolumeID},
				})
				if err != nil {
					Fail(fmt.Sprintf("failed to describe volume: %v", err))
				}

				if len(result.Volumes) != 1 {
					Fail(fmt.Sprintf("expected 1 volume, got %d", len(result.Volumes)))
				}

				if fmt.Sprintf("%d", result.Volumes[0].Iops) == sourceIops {
					Fail(fmt.Sprintf("expected volume iops %d, to not be same as source %s", result.Volumes[0].Iops, sourceIops))
				}
			},
		}
		test.Run(cs, ns)
	})
	It("should copy a volume to a bigger volume", func() {
		cloneClaimSize := "3Gi"
		pod := testsuites.PodDetails{
			Cmd: testsuites.PodCmdWriteToVolume("/mnt/test-1"),
			Volumes: []testsuites.VolumeDetails{
				{
					CreateVolumeParameters: map[string]string{
						ebscsidriver.EncryptedKey:  "true",
						ebscsidriver.VolumeTypeKey: awscloud.VolumeTypeGP2,
						ebscsidriver.FSTypeKey:     ebscsidriver.FSTypeExt4,
					},
					ClaimSize:   driver.MinimumSizeForVolumeType(awscloud.VolumeTypeGP2),
					VolumeMount: testsuites.DefaultGeneratedVolumeMount,
				},
			},
		}
		clonedPod := testsuites.PodDetails{
			Cmd: testsuites.PodCmdWriteToVolume("/mnt/test-1"),
			Volumes: []testsuites.VolumeDetails{
				{
					CreateVolumeParameters: map[string]string{
						ebscsidriver.VolumeTypeKey: awscloud.VolumeTypeGP2,
						ebscsidriver.FSTypeKey:     ebscsidriver.FSTypeExt4,
					},
					ClaimSize:   cloneClaimSize,
					VolumeMount: testsuites.DefaultGeneratedVolumeMount,
				},
			},
		}
		test := testsuites.DynamicallyProvisionedCopyVolumeTest{
			CSIDriver: ebsDriver,
			Pod:       pod,
			ClonedPod: clonedPod,
			ValidateFunc: func() {
				result, err := ec2Client.DescribeVolumes(context.Background(), &ec2.DescribeVolumesInput{
					VolumeIds: []string{clonedPod.Volumes[0].VolumeID},
				})
				if err != nil {
					Fail(fmt.Sprintf("failed to describe volume: %v", err))
				}

				if len(result.Volumes) != 1 {
					Fail(fmt.Sprintf("expected 1 volume, got %d", len(result.Volumes)))
				}

				if fmt.Sprintf("%dGi", result.Volumes[0].Size) != cloneClaimSize {
					Fail(fmt.Sprintf("expected volume size to be %s, got %d", cloneClaimSize, result.Volumes[0].Size))
				}
			},
		}
		test.Run(cs, ns)
	})

	It("should validate GP3 volume IOPS are not capped at 16000 when requesting 16001", func() {
		testTag := generateTagName()

		pod := testsuites.PodDetails{
			Cmd: testsuites.PodCmdWriteToVolume("/mnt/test-1"),
			Volumes: []testsuites.VolumeDetails{
				{
					CreateVolumeParameters: map[string]string{
						ebscsidriver.VolumeTypeKey: awscloud.VolumeTypeGP3,
						ebscsidriver.IopsKey:       "16001",
						ebscsidriver.FSTypeKey:     ebscsidriver.FSTypeExt4,
						ebscsidriver.TagKeyPrefix:  fmt.Sprintf("%s=%s", testTag, testTagValue),
					},
					ClaimSize:   "34Gi", // To make sure we don't hit IOPS:Gb max error
					VolumeMount: testsuites.DefaultGeneratedVolumeMount,
				},
			},
		}

		test := testsuites.DynamicallyProvisionedCmdVolumeTest{
			CSIDriver: ebsDriver,
			Pods:      []testsuites.PodDetails{pod},
			ValidateFunc: func() {
				result, err := ec2Client.DescribeVolumes(context.Background(), &ec2.DescribeVolumesInput{
					Filters: []types.Filter{
						{
							Name:   aws.String("tag:" + testTag),
							Values: []string{testTagValue},
						},
					},
				})
				if err != nil {
					Fail(fmt.Sprintf("failed to describe volume: %v", err))
				}

				if len(result.Volumes) != 1 {
					Fail(fmt.Sprintf("expected 1 volume, got %d", len(result.Volumes)))
				}

				vol := result.Volumes[0]
				if *vol.Iops == 16000 {
					Fail(fmt.Sprintf("IOPS was capped at 16000, indicating error message may have changed"))
				}
			},
		}
		test.Run(cs, ns)
	})
})
