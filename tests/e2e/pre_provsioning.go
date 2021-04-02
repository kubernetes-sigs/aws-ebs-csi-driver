/*
Copyright 2018 The Kubernetes Authors.

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
	"math/rand"
	"os"
	"strings"

	ebscsidriver "github.com/kubernetes-sigs/aws-ebs-csi-driver/pkg/driver"
	k8srestclient "k8s.io/client-go/rest"

	awscloud "github.com/kubernetes-sigs/aws-ebs-csi-driver/pkg/cloud"
	"github.com/kubernetes-sigs/aws-ebs-csi-driver/tests/e2e/driver"
	"github.com/kubernetes-sigs/aws-ebs-csi-driver/tests/e2e/testsuites"
	. "github.com/onsi/ginkgo"
	v1 "k8s.io/api/core/v1"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/kubernetes/test/e2e/framework"
)

const (
	defaultDiskSize   = 4
	defaultVoluemType = awscloud.VolumeTypeGP3

	awsAvailabilityZonesEnv = "AWS_AVAILABILITY_ZONES"

	dummyVolumeName   = "pre-provisioned"
	dummySnapshotName = "pre-provisioned-snapshot"
)

var (
	defaultDiskSizeBytes int64 = defaultDiskSize * 1024 * 1024 * 1024
)

// Requires env AWS_AVAILABILITY_ZONES a comma separated list of AZs to be set
var _ = Describe("[ebs-csi-e2e] [single-az] Pre-Provisioned", func() {
	f := framework.NewDefaultFramework("ebs")

	var (
		cs           clientset.Interface
		ns           *v1.Namespace
		ebsDriver    driver.PreProvisionedVolumeTestDriver
		pvTestDriver driver.PVTestDriver
		snapshotrcs  k8srestclient.Interface
		cloud        awscloud.Cloud
		volumeID     string
		snapshotID   string
		diskSize     string
		// Set to true if the volume should be deleted automatically after test
		skipManuallyDeletingVolume bool
	)

	BeforeEach(func() {
		cs = f.ClientSet
		ns = f.Namespace
		ebsDriver = driver.InitEbsCSIDriver()

		// setup EBS volume
		if os.Getenv(awsAvailabilityZonesEnv) == "" {
			Skip(fmt.Sprintf("env %q not set", awsAvailabilityZonesEnv))
		}
		availabilityZones := strings.Split(os.Getenv(awsAvailabilityZonesEnv), ",")
		availabilityZone := availabilityZones[rand.Intn(len(availabilityZones))]
		region := availabilityZone[0 : len(availabilityZone)-1]

		diskOptions := &awscloud.DiskOptions{
			CapacityBytes:    defaultDiskSizeBytes,
			VolumeType:       defaultVoluemType,
			AvailabilityZone: availabilityZone,
			Tags:             map[string]string{awscloud.VolumeNameTagKey: dummyVolumeName},
		}
		var err error
		cloud, err = awscloud.NewCloud(region)
		if err != nil {
			Fail(fmt.Sprintf("could not get NewCloud: %v", err))
		}
		disk, err := cloud.CreateDisk(context.Background(), "", diskOptions)
		if err != nil {
			Fail(fmt.Sprintf("could not provision a volume: %v", err))
		}
		volumeID = disk.VolumeID
		diskSize = fmt.Sprintf("%dGi", defaultDiskSize)
		snapshotrcs, err = restClient(testsuites.SnapshotAPIGroup, testsuites.APIVersionv1beta1)
		if err != nil {
			Fail(fmt.Sprintf("could not get rest clientset: %v", err))
		}
		pvTestDriver = driver.InitEbsCSIDriver()
		By(fmt.Sprintf("Successfully provisioned EBS volume: %q\n", volumeID))
		snapshotOptions := &awscloud.SnapshotOptions{
			Tags: map[string]string{awscloud.SnapshotNameTagKey: dummySnapshotName},
		}
		snapshot, err := cloud.CreateSnapshot(context.Background(), volumeID, snapshotOptions)
		if err != nil {
			Fail(fmt.Sprintf("could not provision a snapshot from volume: %v", volumeID))
		}
		snapshotID = snapshot.SnapshotID
		By(fmt.Sprintf("Successfully provisioned EBS volume snapshot: %q\n", snapshotID))
	})

	AfterEach(func() {
		if !skipManuallyDeletingVolume {
			_, err := cloud.WaitForAttachmentState(context.Background(), volumeID, "detached", "", "", false)
			if err != nil {
				Fail(fmt.Sprintf("could not detach volume %q: %v", volumeID, err))
			}
			ok, err := cloud.DeleteDisk(context.Background(), volumeID)
			if err != nil || !ok {
				Fail(fmt.Sprintf("could not delete volume %q: %v", volumeID, err))
			}
		}
	})

	It("[env] should write and read to a pre-provisioned volume", func() {
		pods := []testsuites.PodDetails{
			{
				Cmd: "echo 'hello world' > /mnt/test-1/data && grep 'hello world' /mnt/test-1/data",
				Volumes: []testsuites.VolumeDetails{
					{
						VolumeID:  volumeID,
						FSType:    ebscsidriver.FSTypeExt4,
						ClaimSize: diskSize,
						VolumeMount: testsuites.VolumeMountDetails{
							NameGenerate:      "test-volume-",
							MountPathGenerate: "/mnt/test-",
						},
					},
				},
			},
		}
		test := testsuites.PreProvisionedVolumeTest{
			CSIDriver: ebsDriver,
			Pods:      pods,
		}
		test.Run(cs, ns)
	})

	It("[env] should use a pre-defined snapshot and create pv from that", func() {
		pod := testsuites.PodDetails{
			Cmd: "echo 'hello world' >> /mnt/test-1/data && grep 'hello world' /mnt/test-1/data && sync",
			Volumes: []testsuites.VolumeDetails{
				{
					ClaimSize: diskSize,
					VolumeMount: testsuites.VolumeMountDetails{
						NameGenerate:      "test-volume-",
						MountPathGenerate: "/mnt/test-",
					},
				},
			},
		}
		test := testsuites.PreProvisionedVolumeSnapshotTest{
			CSIDriver: pvTestDriver,
			Pod:       pod,
		}
		test.Run(cs, snapshotrcs, ns, snapshotID)
	})

	It("[env] should use a pre-provisioned volume and mount it as readOnly in a pod", func() {
		pods := []testsuites.PodDetails{
			{
				Cmd: "echo 'hello world' > /mnt/test-1/data && grep 'hello world' /mnt/test-1/data",
				Volumes: []testsuites.VolumeDetails{
					{
						VolumeID:  volumeID,
						FSType:    ebscsidriver.FSTypeExt4,
						ClaimSize: diskSize,
						VolumeMount: testsuites.VolumeMountDetails{
							NameGenerate:      "test-volume-",
							MountPathGenerate: "/mnt/test-",
							ReadOnly:          true,
						},
					},
				},
			},
		}
		test := testsuites.PreProvisionedReadOnlyVolumeTest{
			CSIDriver: ebsDriver,
			Pods:      pods,
		}
		test.Run(cs, ns)
	})

	It(fmt.Sprintf("[env] should use a pre-provisioned volume and retain PV with reclaimPolicy %q", v1.PersistentVolumeReclaimRetain), func() {
		reclaimPolicy := v1.PersistentVolumeReclaimRetain
		volumes := []testsuites.VolumeDetails{
			{
				VolumeID:      volumeID,
				FSType:        ebscsidriver.FSTypeExt4,
				ClaimSize:     diskSize,
				ReclaimPolicy: &reclaimPolicy,
			},
		}
		test := testsuites.PreProvisionedReclaimPolicyTest{
			CSIDriver: ebsDriver,
			Volumes:   volumes,
		}
		test.Run(cs, ns)
	})

	It(fmt.Sprintf("[env] should use a pre-provisioned volume and delete PV with reclaimPolicy %q", v1.PersistentVolumeReclaimDelete), func() {
		reclaimPolicy := v1.PersistentVolumeReclaimDelete
		skipManuallyDeletingVolume = true
		volumes := []testsuites.VolumeDetails{
			{
				VolumeID:      volumeID,
				FSType:        ebscsidriver.FSTypeExt4,
				ClaimSize:     diskSize,
				ReclaimPolicy: &reclaimPolicy,
			},
		}
		test := testsuites.PreProvisionedReclaimPolicyTest{
			CSIDriver: ebsDriver,
			Volumes:   volumes,
		}
		test.Run(cs, ns)
	})
})
