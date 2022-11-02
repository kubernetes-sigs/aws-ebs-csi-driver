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

package integration

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/ec2"
	csi "github.com/container-storage-interface/spec/lib/go/csi"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/util/wait"
)

var (
	stdVolCap = []*csi.VolumeCapability{
		{
			AccessType: &csi.VolumeCapability_Mount{
				Mount: &csi.VolumeCapability_MountVolume{},
			},
			AccessMode: &csi.VolumeCapability_AccessMode{
				Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER,
			},
		},
	}
	stdVolSize  = int64(1 * 1024 * 1024 * 1024)
	stdCapRange = &csi.CapacityRange{RequiredBytes: stdVolSize}
)

var _ = Describe("EBS CSI Driver", func() {

	It("Should create, attach, stage and mount volume, check if it's writable, unmount, unstage, detach, delete, and check if it's deleted", func() {

		r1 := rand.New(rand.NewSource(time.Now().UnixNano()))
		req := &csi.CreateVolumeRequest{
			Name:               fmt.Sprintf("volume-name-integration-test-%d", r1.Uint64()),
			CapacityRange:      stdCapRange,
			VolumeCapabilities: stdVolCap,
			Parameters:         nil,
		}

		logf("Creating volume with name %q", req.GetName())
		resp, err := csiClient.ctrl.CreateVolume(context.Background(), req)
		Expect(err).To(BeNil(), "Could not create volume")

		volume := resp.GetVolume()
		Expect(volume).NotTo(BeNil(), "Expected valid volume, got nil")
		waitForVolume(volume.VolumeId, 1 /* number of expected volumes */)

		defer func() {
			logf("Deleting volume %q", volume.VolumeId)
			_, err = csiClient.ctrl.DeleteVolume(context.Background(), &csi.DeleteVolumeRequest{VolumeId: volume.VolumeId})
			Expect(err).To(BeNil(), "Could not delete volume")
			waitForVolume(volume.VolumeId, 0 /* number of expected volumes */)

			logf("Deleting volume %q twice", volume.VolumeId)
			_, err = csiClient.ctrl.DeleteVolume(context.Background(), &csi.DeleteVolumeRequest{VolumeId: volume.VolumeId})
			Expect(err).To(BeNil(), "Error when trying to delete volume twice")
		}()

		// Attach, stage, publish, unpublish, unstage, detach
		metadata, err := newMetadata()
		Expect(err).To(BeNil())
		nodeID := metadata.GetInstanceID()
		testAttachWriteReadDetach(volume.VolumeId, req.GetName(), nodeID, false)

	})
})

func testAttachWriteReadDetach(volumeID, volName, nodeID string, readOnly bool) {
	logf("Attaching volume %q to node %q", volumeID, nodeID)
	respAttach, err := csiClient.ctrl.ControllerPublishVolume(
		context.Background(),
		&csi.ControllerPublishVolumeRequest{
			VolumeId:         volumeID,
			NodeId:           nodeID,
			VolumeCapability: stdVolCap[0],
		},
	)
	Expect(err).To(BeNil(), "ControllerPublishVolume failed attaching volume %q to node %q", volumeID, nodeID)
	assertAttachmentState(volumeID, "attached")

	defer func() {
		logf("Detaching volume %q from node %q", volumeID, nodeID)
		_, err = csiClient.ctrl.ControllerUnpublishVolume(
			context.Background(),
			&csi.ControllerUnpublishVolumeRequest{
				VolumeId: volumeID,
				NodeId:   nodeID,
			},
		)
		Expect(err).To(BeNil(), "ControllerUnpublishVolume failed with error")
		assertAttachmentState(volumeID, "detached")
	}()

	volDir := filepath.Join("/tmp/", volName)
	stageDir := filepath.Join(volDir, "stage")
	logf("Staging volume %q to path %q", volumeID, stageDir)
	_, err = csiClient.node.NodeStageVolume(
		context.Background(),
		&csi.NodeStageVolumeRequest{
			VolumeId:          volumeID,
			StagingTargetPath: stageDir,
			VolumeCapability:  stdVolCap[0],
			PublishContext:    map[string]string{"devicePath": respAttach.PublishContext["devicePath"]},
		})
	Expect(err).To(BeNil(), "NodeStageVolume failed with error")

	defer func() {
		logf("Unstaging volume %q from path %q", volumeID, stageDir)
		_, err = csiClient.node.NodeUnstageVolume(context.Background(), &csi.NodeUnstageVolumeRequest{VolumeId: volumeID, StagingTargetPath: stageDir})
		Expect(err).To(BeNil(), "NodeUnstageVolume failed with error")
		err = os.RemoveAll(volDir)
		Expect(err).To(BeNil(), "Failed to remove temp directory")
	}()

	publishDir := filepath.Join("/tmp/", volName, "mount")
	logf("Publishing volume %q to path %q", volumeID, publishDir)
	_, err = csiClient.node.NodePublishVolume(context.Background(), &csi.NodePublishVolumeRequest{
		VolumeId:          volumeID,
		StagingTargetPath: stageDir,
		TargetPath:        publishDir,
		VolumeCapability:  stdVolCap[0],
	})
	Expect(err).To(BeNil(), "NodePublishVolume failed with error")

	defer func() {
		logf("Unpublishing volume %q from path %q", volumeID, publishDir)
		_, err = csiClient.node.NodeUnpublishVolume(context.Background(), &csi.NodeUnpublishVolumeRequest{
			VolumeId:   volumeID,
			TargetPath: publishDir,
		})
		Expect(err).To(BeNil(), "NodeUnpublishVolume failed with error")
	}()

	if !readOnly {
		logf("Writing and reading a file")
		// Write a file
		testFileContents := []byte("sample content")
		testFile := filepath.Join(publishDir, "testfile")
		err := os.WriteFile(testFile, testFileContents, 0644)
		Expect(err).To(BeNil(), "Failed to write file")
		// Read the file and check if content is correct
		data, err := os.ReadFile(testFile)
		Expect(err).To(BeNil(), "Failed to read file")
		Expect(data).To(Equal(testFileContents), "File content is incorrect")
	}
}

func waitForVolume(volumeID string, nVolumes int) {
	backoff := wait.Backoff{
		Duration: 1 * time.Second,
		Factor:   1.8,
		Steps:    13,
	}
	verifyVolumeFunc := func() (bool, error) {
		logf("Waiting for %d volumes with ID %q", nVolumes, volumeID)
		params := &ec2.DescribeVolumesInput{
			VolumeIds: []*string{aws.String(volumeID)},
		}
		volumes, err := describeVolumes(params)
		if err != nil {
			if nVolumes == 0 {
				var awsErr awserr.Error
				if errors.As(err, &awsErr) {
					if awsErr.Code() == "InvalidVolume.NotFound" {
						return true, nil
					}
				}
			}
			return false, err
		}
		if len(volumes) != nVolumes {
			return false, nil
		}
		if nVolumes == 1 {
			if aws.StringValue(volumes[0].State) != "available" {
				return false, nil
			}
		}
		return true, nil
	}
	waitErr := wait.ExponentialBackoff(backoff, verifyVolumeFunc)
	Expect(waitErr).To(BeNil(), "Timeout error when looking for volume %q: %v", volumeID, waitErr)
}

func assertAttachmentState(volumeID, state string) {
	logf("Checking if attachment state of volume %q is %q", volumeID, state)
	volumes, err := describeVolumes(&ec2.DescribeVolumesInput{
		VolumeIds: []*string{aws.String(volumeID)},
	})
	Expect(err).To(BeNil(), "Error describing volumes: %v", err)

	nVolumes := len(volumes)
	Expect(nVolumes).To(BeNumerically("==", 1), "Expected 1 volume, got %d", nVolumes)

	// Detached volumes have 0 attachments
	if state == "detached" {
		nAttachments := len(volumes[0].Attachments)
		Expect(nAttachments).To(BeNumerically("==", 0), "Expected 0 attachments, got %d", nAttachments)
		return
	}

	aState := aws.StringValue(volumes[0].Attachments[0].State)
	Expect(aState).To(Equal(state), "Expected state %s, got %s", state, aState)
}

func describeVolumes(params *ec2.DescribeVolumesInput) ([]*ec2.Volume, error) {
	var volumes []*ec2.Volume
	var nextToken *string
	for {
		response, err := ec2Client.DescribeVolumes(params)
		if err != nil {
			return nil, err
		}
		volumes = append(volumes, response.Volumes...)
		nextToken = response.NextToken
		if aws.StringValue(nextToken) == "" {
			break
		}
		params.NextToken = nextToken
	}
	return volumes, nil
}
