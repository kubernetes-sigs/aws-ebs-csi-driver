/*
Copyright 2017 Kubernetes Authors.

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

package sanity

import (
	"fmt"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/container-storage-interface/spec/lib/go/csi/v0"
	"golang.org/x/net/context"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"strconv"
)

const (
	// DefTestVolumeSize defines the base size of dynamically
	// provisioned volumes. 10GB by default, can be overridden by
	// setting Config.TestVolumeSize.
	DefTestVolumeSize int64 = 10 * 1024 * 1024 * 1024
)

func TestVolumeSize(sc *SanityContext) int64 {
	if sc.Config.TestVolumeSize > 0 {
		return sc.Config.TestVolumeSize
	}
	return DefTestVolumeSize
}

func verifyVolumeInfo(v *csi.Volume) {
	Expect(v).NotTo(BeNil())
	Expect(v.GetId()).NotTo(BeEmpty())
}

func verifySnapshotInfo(snapshot *csi.Snapshot) {
	Expect(snapshot).NotTo(BeNil())
	Expect(snapshot.GetId()).NotTo(BeEmpty())
	Expect(snapshot.GetSourceVolumeId()).NotTo(BeEmpty())
	Expect(snapshot.GetCreatedAt()).NotTo(BeZero())
}

func isControllerCapabilitySupported(
	c csi.ControllerClient,
	capType csi.ControllerServiceCapability_RPC_Type,
) bool {

	caps, err := c.ControllerGetCapabilities(
		context.Background(),
		&csi.ControllerGetCapabilitiesRequest{})
	Expect(err).NotTo(HaveOccurred())
	Expect(caps).NotTo(BeNil())
	Expect(caps.GetCapabilities()).NotTo(BeNil())

	for _, cap := range caps.GetCapabilities() {
		Expect(cap.GetRpc()).NotTo(BeNil())
		if cap.GetRpc().GetType() == capType {
			return true
		}
	}
	return false
}

var _ = DescribeSanity("ControllerGetCapabilities [Controller Server]", func(sc *SanityContext) {
	var (
		c csi.ControllerClient
	)

	BeforeEach(func() {
		c = csi.NewControllerClient(sc.Conn)
	})

	It("should return appropriate capabilities", func() {
		caps, err := c.ControllerGetCapabilities(
			context.Background(),
			&csi.ControllerGetCapabilitiesRequest{})

		By("checking successful response")
		Expect(err).NotTo(HaveOccurred())
		Expect(caps).NotTo(BeNil())
		Expect(caps.GetCapabilities()).NotTo(BeNil())

		for _, cap := range caps.GetCapabilities() {
			Expect(cap.GetRpc()).NotTo(BeNil())

			switch cap.GetRpc().GetType() {
			case csi.ControllerServiceCapability_RPC_CREATE_DELETE_VOLUME:
			case csi.ControllerServiceCapability_RPC_PUBLISH_UNPUBLISH_VOLUME:
			case csi.ControllerServiceCapability_RPC_LIST_VOLUMES:
			case csi.ControllerServiceCapability_RPC_GET_CAPACITY:
			case csi.ControllerServiceCapability_RPC_CREATE_DELETE_SNAPSHOT:
			case csi.ControllerServiceCapability_RPC_LIST_SNAPSHOTS:
			default:
				Fail(fmt.Sprintf("Unknown capability: %v\n", cap.GetRpc().GetType()))
			}
		}
	})
})

var _ = DescribeSanity("GetCapacity [Controller Server]", func(sc *SanityContext) {
	var (
		c csi.ControllerClient
	)

	BeforeEach(func() {
		c = csi.NewControllerClient(sc.Conn)

		if !isControllerCapabilitySupported(c, csi.ControllerServiceCapability_RPC_GET_CAPACITY) {
			Skip("GetCapacity not supported")
		}
	})

	It("should return capacity (no optional values added)", func() {
		_, err := c.GetCapacity(
			context.Background(),
			&csi.GetCapacityRequest{})
		Expect(err).NotTo(HaveOccurred())

		// Since capacity is int64 we will not be checking it
		// The value of zero is a possible value.
	})
})

var _ = DescribeSanity("ListVolumes [Controller Server]", func(sc *SanityContext) {
	var (
		c csi.ControllerClient
	)

	BeforeEach(func() {
		c = csi.NewControllerClient(sc.Conn)

		if !isControllerCapabilitySupported(c, csi.ControllerServiceCapability_RPC_LIST_VOLUMES) {
			Skip("ListVolumes not supported")
		}
	})

	It("should return appropriate values (no optional values added)", func() {
		vols, err := c.ListVolumes(
			context.Background(),
			&csi.ListVolumesRequest{})
		Expect(err).NotTo(HaveOccurred())
		Expect(vols).NotTo(BeNil())

		for _, vol := range vols.GetEntries() {
			verifyVolumeInfo(vol.GetVolume())
		}
	})

	// TODO: Add test to test for tokens

	// TODO: Add test which checks list of volume is there when created,
	//       and not there when deleted.
})

var _ = DescribeSanity("CreateVolume [Controller Server]", func(sc *SanityContext) {
	var (
		c csi.ControllerClient
	)

	BeforeEach(func() {
		c = csi.NewControllerClient(sc.Conn)

		if !isControllerCapabilitySupported(c, csi.ControllerServiceCapability_RPC_CREATE_DELETE_VOLUME) {
			Skip("CreateVolume not supported")
		}
	})

	It("should fail when no name is provided", func() {

		req := &csi.CreateVolumeRequest{}

		if sc.Secrets != nil {
			req.ControllerCreateSecrets = sc.Secrets.CreateVolumeSecret
		}

		_, err := c.CreateVolume(context.Background(), req)
		Expect(err).To(HaveOccurred())

		serverError, ok := status.FromError(err)
		Expect(ok).To(BeTrue())
		Expect(serverError.Code()).To(Equal(codes.InvalidArgument))
	})

	It("should fail when no volume capabilities are provided", func() {

		req := &csi.CreateVolumeRequest{
			Name: "name",
		}

		if sc.Secrets != nil {
			req.ControllerCreateSecrets = sc.Secrets.CreateVolumeSecret
		}

		_, err := c.CreateVolume(context.Background(), req)
		Expect(err).To(HaveOccurred())

		serverError, ok := status.FromError(err)
		Expect(ok).To(BeTrue())
		Expect(serverError.Code()).To(Equal(codes.InvalidArgument))
	})

	It("should return appropriate values SingleNodeWriter NoCapacity Type:Mount", func() {

		By("creating a volume")
		name := "sanity"

		req := &csi.CreateVolumeRequest{
			Name: name,
			VolumeCapabilities: []*csi.VolumeCapability{
				{
					AccessType: &csi.VolumeCapability_Mount{
						Mount: &csi.VolumeCapability_MountVolume{},
					},
					AccessMode: &csi.VolumeCapability_AccessMode{
						Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER,
					},
				},
			},
		}

		if sc.Secrets != nil {
			req.ControllerCreateSecrets = sc.Secrets.CreateVolumeSecret
		}

		vol, err := c.CreateVolume(context.Background(), req)
		Expect(err).NotTo(HaveOccurred())
		Expect(vol).NotTo(BeNil())
		Expect(vol.GetVolume()).NotTo(BeNil())
		Expect(vol.GetVolume().GetId()).NotTo(BeEmpty())

		By("cleaning up deleting the volume")

		delReq := &csi.DeleteVolumeRequest{
			VolumeId: vol.GetVolume().GetId(),
		}

		if sc.Secrets != nil {
			delReq.ControllerDeleteSecrets = sc.Secrets.DeleteVolumeSecret
		}

		_, err = c.DeleteVolume(context.Background(), delReq)
		Expect(err).NotTo(HaveOccurred())
	})

	It("should return appropriate values SingleNodeWriter WithCapacity 1Gi Type:Mount", func() {

		By("creating a volume")
		name := "sanity"

		req := &csi.CreateVolumeRequest{
			Name: name,
			VolumeCapabilities: []*csi.VolumeCapability{
				{
					AccessType: &csi.VolumeCapability_Mount{
						Mount: &csi.VolumeCapability_MountVolume{},
					},
					AccessMode: &csi.VolumeCapability_AccessMode{
						Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER,
					},
				},
			},
			CapacityRange: &csi.CapacityRange{
				RequiredBytes: TestVolumeSize(sc),
			},
		}

		if sc.Secrets != nil {
			req.ControllerCreateSecrets = sc.Secrets.CreateVolumeSecret
		}

		vol, err := c.CreateVolume(context.Background(), req)
		if serverError, ok := status.FromError(err); ok {
			if serverError.Code() == codes.OutOfRange || serverError.Code() == codes.Unimplemented {
				Skip("Required bytes not supported")
			} else {
				Expect(err).NotTo(HaveOccurred())
			}
		} else {

			Expect(err).NotTo(HaveOccurred())
			Expect(vol).NotTo(BeNil())
			Expect(vol.GetVolume()).NotTo(BeNil())
			Expect(vol.GetVolume().GetId()).NotTo(BeEmpty())
			Expect(vol.GetVolume().GetCapacityBytes()).To(BeNumerically(">=", TestVolumeSize(sc)))
		}
		By("cleaning up deleting the volume")

		delReq := &csi.DeleteVolumeRequest{
			VolumeId: vol.GetVolume().GetId(),
		}

		if sc.Secrets != nil {
			delReq.ControllerDeleteSecrets = sc.Secrets.DeleteVolumeSecret
		}

		_, err = c.DeleteVolume(context.Background(), delReq)
		Expect(err).NotTo(HaveOccurred())
	})
	It("should not fail when requesting to create a volume with already exisiting name and same capacity.", func() {

		By("creating a volume")
		name := "sanity"
		size := TestVolumeSize(sc)

		req := &csi.CreateVolumeRequest{
			Name: name,
			VolumeCapabilities: []*csi.VolumeCapability{
				{
					AccessType: &csi.VolumeCapability_Mount{
						Mount: &csi.VolumeCapability_MountVolume{},
					},
					AccessMode: &csi.VolumeCapability_AccessMode{
						Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER,
					},
				},
			},
			CapacityRange: &csi.CapacityRange{
				RequiredBytes: size,
			},
		}

		if sc.Secrets != nil {
			req.ControllerCreateSecrets = sc.Secrets.CreateVolumeSecret
		}

		vol1, err := c.CreateVolume(context.Background(), req)
		Expect(err).NotTo(HaveOccurred())
		Expect(vol1).NotTo(BeNil())
		Expect(vol1.GetVolume()).NotTo(BeNil())
		Expect(vol1.GetVolume().GetId()).NotTo(BeEmpty())
		Expect(vol1.GetVolume().GetCapacityBytes()).To(BeNumerically(">=", size))

		req2 := &csi.CreateVolumeRequest{
			Name: name,
			VolumeCapabilities: []*csi.VolumeCapability{
				{
					AccessType: &csi.VolumeCapability_Mount{
						Mount: &csi.VolumeCapability_MountVolume{},
					},
					AccessMode: &csi.VolumeCapability_AccessMode{
						Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER,
					},
				},
			},
			CapacityRange: &csi.CapacityRange{
				RequiredBytes: size,
			},
		}

		if sc.Secrets != nil {
			req2.ControllerCreateSecrets = sc.Secrets.CreateVolumeSecret
		}

		vol2, err := c.CreateVolume(context.Background(), req2)
		Expect(err).NotTo(HaveOccurred())
		Expect(vol2).NotTo(BeNil())
		Expect(vol2.GetVolume()).NotTo(BeNil())
		Expect(vol2.GetVolume().GetId()).NotTo(BeEmpty())
		Expect(vol2.GetVolume().GetCapacityBytes()).To(BeNumerically(">=", size))
		Expect(vol1.GetVolume().GetId()).To(Equal(vol2.GetVolume().GetId()))

		By("cleaning up deleting the volume")

		delReq := &csi.DeleteVolumeRequest{
			VolumeId: vol1.GetVolume().GetId(),
		}

		if sc.Secrets != nil {
			delReq.ControllerDeleteSecrets = sc.Secrets.DeleteVolumeSecret
		}

		_, err = c.DeleteVolume(context.Background(), delReq)
		Expect(err).NotTo(HaveOccurred())
	})
	It("should fail when requesting to create a volume with already exisiting name and different capacity.", func() {

		By("creating a volume")
		name := "sanity"
		size1 := TestVolumeSize(sc)

		req := &csi.CreateVolumeRequest{
			Name: name,
			VolumeCapabilities: []*csi.VolumeCapability{
				{
					AccessType: &csi.VolumeCapability_Mount{
						Mount: &csi.VolumeCapability_MountVolume{},
					},
					AccessMode: &csi.VolumeCapability_AccessMode{
						Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER,
					},
				},
			},
			CapacityRange: &csi.CapacityRange{
				RequiredBytes: size1,
				LimitBytes:    size1,
			},
		}

		if sc.Secrets != nil {
			req.ControllerCreateSecrets = sc.Secrets.CreateVolumeSecret
		}

		vol1, err := c.CreateVolume(context.Background(), req)
		Expect(err).ToNot(HaveOccurred())
		Expect(vol1).NotTo(BeNil())
		Expect(vol1.GetVolume()).NotTo(BeNil())
		Expect(vol1.GetVolume().GetId()).NotTo(BeEmpty())
		size2 := 2 * TestVolumeSize(sc)

		req2 := &csi.CreateVolumeRequest{
			Name: name,
			VolumeCapabilities: []*csi.VolumeCapability{
				{
					AccessType: &csi.VolumeCapability_Mount{
						Mount: &csi.VolumeCapability_MountVolume{},
					},
					AccessMode: &csi.VolumeCapability_AccessMode{
						Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER,
					},
				},
			},
			CapacityRange: &csi.CapacityRange{
				RequiredBytes: size2,
				LimitBytes:    size2,
			},
		}

		if sc.Secrets != nil {
			req2.ControllerCreateSecrets = sc.Secrets.CreateVolumeSecret
		}

		_, err = c.CreateVolume(context.Background(), req2)
		Expect(err).To(HaveOccurred())
		serverError, ok := status.FromError(err)
		Expect(ok).To(BeTrue())
		Expect(serverError.Code()).To(Equal(codes.AlreadyExists))

		By("cleaning up deleting the volume")

		delReq := &csi.DeleteVolumeRequest{
			VolumeId: vol1.GetVolume().GetId(),
		}

		if sc.Secrets != nil {
			delReq.ControllerDeleteSecrets = sc.Secrets.DeleteVolumeSecret
		}

		_, err = c.DeleteVolume(context.Background(), delReq)
		Expect(err).NotTo(HaveOccurred())
	})
})

var _ = DescribeSanity("DeleteVolume [Controller Server]", func(sc *SanityContext) {
	var (
		c csi.ControllerClient
	)

	BeforeEach(func() {
		c = csi.NewControllerClient(sc.Conn)

		if !isControllerCapabilitySupported(c, csi.ControllerServiceCapability_RPC_CREATE_DELETE_VOLUME) {
			Skip("DeleteVolume not supported")
		}
	})

	It("should fail when no volume id is provided", func() {

		req := &csi.DeleteVolumeRequest{}

		if sc.Secrets != nil {
			req.ControllerDeleteSecrets = sc.Secrets.DeleteVolumeSecret
		}

		_, err := c.DeleteVolume(context.Background(), req)
		Expect(err).To(HaveOccurred())

		serverError, ok := status.FromError(err)
		Expect(ok).To(BeTrue())
		Expect(serverError.Code()).To(Equal(codes.InvalidArgument))
	})

	It("should succeed when an invalid volume id is used", func() {

		req := &csi.DeleteVolumeRequest{
			VolumeId: "reallyfakevolumeid",
		}

		if sc.Secrets != nil {
			req.ControllerDeleteSecrets = sc.Secrets.DeleteVolumeSecret
		}

		_, err := c.DeleteVolume(context.Background(), req)
		Expect(err).NotTo(HaveOccurred())
	})

	It("should return appropriate values (no optional values added)", func() {

		// Create Volume First
		By("creating a volume")
		name := "sanity"

		createReq := &csi.CreateVolumeRequest{
			Name: name,
			VolumeCapabilities: []*csi.VolumeCapability{
				{
					AccessType: &csi.VolumeCapability_Mount{
						Mount: &csi.VolumeCapability_MountVolume{},
					},
					AccessMode: &csi.VolumeCapability_AccessMode{
						Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER,
					},
				},
			},
		}

		if sc.Secrets != nil {
			createReq.ControllerCreateSecrets = sc.Secrets.CreateVolumeSecret
		}

		vol, err := c.CreateVolume(context.Background(), createReq)

		Expect(err).NotTo(HaveOccurred())
		Expect(vol).NotTo(BeNil())
		Expect(vol.GetVolume()).NotTo(BeNil())
		Expect(vol.GetVolume().GetId()).NotTo(BeEmpty())

		// Delete Volume
		By("deleting a volume")

		req := &csi.DeleteVolumeRequest{
			VolumeId: vol.GetVolume().GetId(),
		}

		if sc.Secrets != nil {
			req.ControllerDeleteSecrets = sc.Secrets.DeleteVolumeSecret
		}

		_, err = c.DeleteVolume(context.Background(), req)
		Expect(err).NotTo(HaveOccurred())
	})
})

var _ = DescribeSanity("ValidateVolumeCapabilities [Controller Server]", func(sc *SanityContext) {
	var (
		c csi.ControllerClient
	)

	BeforeEach(func() {
		c = csi.NewControllerClient(sc.Conn)
	})

	It("should fail when no volume id is provided", func() {

		_, err := c.ValidateVolumeCapabilities(
			context.Background(),
			&csi.ValidateVolumeCapabilitiesRequest{})
		Expect(err).To(HaveOccurred())

		serverError, ok := status.FromError(err)
		Expect(ok).To(BeTrue())
		Expect(serverError.Code()).To(Equal(codes.InvalidArgument))
	})

	It("should fail when no volume capabilities are provided", func() {

		_, err := c.ValidateVolumeCapabilities(
			context.Background(),
			&csi.ValidateVolumeCapabilitiesRequest{
				VolumeId: "id",
			})
		Expect(err).To(HaveOccurred())

		serverError, ok := status.FromError(err)
		Expect(ok).To(BeTrue())
		Expect(serverError.Code()).To(Equal(codes.InvalidArgument))
	})

	It("should return appropriate values (no optional values added)", func() {

		// Create Volume First
		By("creating a single node writer volume")
		name := "sanity"

		req := &csi.CreateVolumeRequest{
			Name: name,
			VolumeCapabilities: []*csi.VolumeCapability{
				{
					AccessType: &csi.VolumeCapability_Mount{
						Mount: &csi.VolumeCapability_MountVolume{},
					},
					AccessMode: &csi.VolumeCapability_AccessMode{
						Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER,
					},
				},
			},
		}

		if sc.Secrets != nil {
			req.ControllerCreateSecrets = sc.Secrets.CreateVolumeSecret
		}

		vol, err := c.CreateVolume(context.Background(), req)
		Expect(err).NotTo(HaveOccurred())
		Expect(vol).NotTo(BeNil())
		Expect(vol.GetVolume()).NotTo(BeNil())
		Expect(vol.GetVolume().GetId()).NotTo(BeEmpty())

		// ValidateVolumeCapabilities
		By("validating volume capabilities")
		valivolcap, err := c.ValidateVolumeCapabilities(
			context.Background(),
			&csi.ValidateVolumeCapabilitiesRequest{
				VolumeId: vol.GetVolume().GetId(),
				VolumeCapabilities: []*csi.VolumeCapability{
					{
						AccessType: &csi.VolumeCapability_Mount{
							Mount: &csi.VolumeCapability_MountVolume{},
						},
						AccessMode: &csi.VolumeCapability_AccessMode{
							Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER,
						},
					},
				},
			})
		Expect(err).NotTo(HaveOccurred())
		Expect(valivolcap).NotTo(BeNil())
		Expect(valivolcap.GetSupported()).To(BeTrue())

		By("cleaning up deleting the volume")

		delReq := &csi.DeleteVolumeRequest{
			VolumeId: vol.GetVolume().GetId(),
		}

		if sc.Secrets != nil {
			delReq.ControllerDeleteSecrets = sc.Secrets.DeleteVolumeSecret
		}

		_, err = c.DeleteVolume(context.Background(), delReq)
		Expect(err).NotTo(HaveOccurred())
	})

	It("should fail when the requested volume does not exist", func() {

		_, err := c.ValidateVolumeCapabilities(
			context.Background(),
			&csi.ValidateVolumeCapabilitiesRequest{
				VolumeId: "some-vol-id",
				VolumeCapabilities: []*csi.VolumeCapability{
					{
						AccessType: &csi.VolumeCapability_Mount{
							Mount: &csi.VolumeCapability_MountVolume{},
						},
						AccessMode: &csi.VolumeCapability_AccessMode{
							Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER,
						},
					},
				},
			},
		)
		Expect(err).To(HaveOccurred())

		serverError, ok := status.FromError(err)
		Expect(ok).To(BeTrue())
		Expect(serverError.Code()).To(Equal(codes.NotFound))
	})
})

var _ = DescribeSanity("ControllerPublishVolume [Controller Server]", func(sc *SanityContext) {
	var (
		c csi.ControllerClient
		n csi.NodeClient
	)

	BeforeEach(func() {
		c = csi.NewControllerClient(sc.Conn)
		n = csi.NewNodeClient(sc.Conn)

		if !isControllerCapabilitySupported(c, csi.ControllerServiceCapability_RPC_PUBLISH_UNPUBLISH_VOLUME) {
			Skip("ControllerPublishVolume not supported")
		}
	})

	It("should fail when no volume id is provided", func() {

		req := &csi.ControllerPublishVolumeRequest{}

		if sc.Secrets != nil {
			req.ControllerPublishSecrets = sc.Secrets.ControllerPublishVolumeSecret
		}

		_, err := c.ControllerPublishVolume(context.Background(), req)
		Expect(err).To(HaveOccurred())

		serverError, ok := status.FromError(err)
		Expect(ok).To(BeTrue())
		Expect(serverError.Code()).To(Equal(codes.InvalidArgument))
	})

	It("should fail when no node id is provided", func() {

		req := &csi.ControllerPublishVolumeRequest{
			VolumeId: "id",
		}

		if sc.Secrets != nil {
			req.ControllerPublishSecrets = sc.Secrets.ControllerPublishVolumeSecret
		}

		_, err := c.ControllerPublishVolume(context.Background(), req)
		Expect(err).To(HaveOccurred())

		serverError, ok := status.FromError(err)
		Expect(ok).To(BeTrue())
		Expect(serverError.Code()).To(Equal(codes.InvalidArgument))
	})

	It("should fail when no volume capability is provided", func() {

		req := &csi.ControllerPublishVolumeRequest{
			VolumeId: "id",
			NodeId:   "fakenode",
		}

		if sc.Secrets != nil {
			req.ControllerPublishSecrets = sc.Secrets.ControllerPublishVolumeSecret
		}

		_, err := c.ControllerPublishVolume(context.Background(), req)
		Expect(err).To(HaveOccurred())

		serverError, ok := status.FromError(err)
		Expect(ok).To(BeTrue())
		Expect(serverError.Code()).To(Equal(codes.InvalidArgument))
	})

	It("should return appropriate values (no optional values added)", func() {

		// Create Volume First
		By("creating a single node writer volume")
		name := "sanity"
		req := &csi.CreateVolumeRequest{
			Name: name,
			VolumeCapabilities: []*csi.VolumeCapability{
				{
					AccessType: &csi.VolumeCapability_Mount{
						Mount: &csi.VolumeCapability_MountVolume{},
					},
					AccessMode: &csi.VolumeCapability_AccessMode{
						Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER,
					},
				},
			},
		}

		if sc.Secrets != nil {
			req.ControllerCreateSecrets = sc.Secrets.CreateVolumeSecret
		}

		vol, err := c.CreateVolume(context.Background(), req)
		Expect(err).NotTo(HaveOccurred())
		Expect(vol).NotTo(BeNil())
		Expect(vol.GetVolume()).NotTo(BeNil())
		Expect(vol.GetVolume().GetId()).NotTo(BeEmpty())

		By("getting a node id")
		nid, err := n.NodeGetId(
			context.Background(),
			&csi.NodeGetIdRequest{})
		Expect(err).NotTo(HaveOccurred())
		Expect(nid).NotTo(BeNil())
		Expect(nid.GetNodeId()).NotTo(BeEmpty())

		// ControllerPublishVolume
		By("calling controllerpublish on that volume")

		pubReq := &csi.ControllerPublishVolumeRequest{
			VolumeId: vol.GetVolume().GetId(),
			NodeId:   nid.GetNodeId(),
			VolumeCapability: &csi.VolumeCapability{
				AccessType: &csi.VolumeCapability_Mount{
					Mount: &csi.VolumeCapability_MountVolume{},
				},
				AccessMode: &csi.VolumeCapability_AccessMode{
					Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER,
				},
			},
			Readonly: false,
		}

		if sc.Secrets != nil {
			pubReq.ControllerPublishSecrets = sc.Secrets.ControllerPublishVolumeSecret
		}

		conpubvol, err := c.ControllerPublishVolume(context.Background(), pubReq)
		Expect(err).NotTo(HaveOccurred())
		Expect(conpubvol).NotTo(BeNil())

		By("cleaning up unpublishing the volume")

		unpubReq := &csi.ControllerUnpublishVolumeRequest{
			VolumeId: vol.GetVolume().GetId(),
			// NodeID is optional in ControllerUnpublishVolume
			NodeId: nid.GetNodeId(),
		}

		if sc.Secrets != nil {
			unpubReq.ControllerUnpublishSecrets = sc.Secrets.ControllerUnpublishVolumeSecret
		}

		conunpubvol, err := c.ControllerUnpublishVolume(context.Background(), unpubReq)
		Expect(err).NotTo(HaveOccurred())
		Expect(conunpubvol).NotTo(BeNil())

		By("cleaning up deleting the volume")

		delReq := &csi.DeleteVolumeRequest{
			VolumeId: vol.GetVolume().GetId(),
		}

		if sc.Secrets != nil {
			delReq.ControllerDeleteSecrets = sc.Secrets.DeleteVolumeSecret
		}

		_, err = c.DeleteVolume(context.Background(), delReq)
		Expect(err).NotTo(HaveOccurred())
	})

	It("should fail when the volume does not exist", func() {

		By("calling controller publish on a non-existent volume")

		pubReq := &csi.ControllerPublishVolumeRequest{
			VolumeId: "some-vol-id",
			NodeId:   "some-node-id",
			VolumeCapability: &csi.VolumeCapability{
				AccessType: &csi.VolumeCapability_Mount{
					Mount: &csi.VolumeCapability_MountVolume{},
				},
				AccessMode: &csi.VolumeCapability_AccessMode{
					Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER,
				},
			},
			Readonly: false,
		}

		if sc.Secrets != nil {
			pubReq.ControllerPublishSecrets = sc.Secrets.ControllerPublishVolumeSecret
		}

		conpubvol, err := c.ControllerPublishVolume(context.Background(), pubReq)
		Expect(err).To(HaveOccurred())
		Expect(conpubvol).To(BeNil())

		serverError, ok := status.FromError(err)
		Expect(ok).To(BeTrue())
		Expect(serverError.Code()).To(Equal(codes.NotFound))
	})

	It("should fail when the node does not exist", func() {

		// Create Volume First
		By("creating a single node writer volume")
		name := "sanity"
		req := &csi.CreateVolumeRequest{
			Name: name,
			VolumeCapabilities: []*csi.VolumeCapability{
				{
					AccessType: &csi.VolumeCapability_Mount{
						Mount: &csi.VolumeCapability_MountVolume{},
					},
					AccessMode: &csi.VolumeCapability_AccessMode{
						Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER,
					},
				},
			},
		}

		if sc.Secrets != nil {
			req.ControllerCreateSecrets = sc.Secrets.CreateVolumeSecret
		}

		vol, err := c.CreateVolume(context.Background(), req)
		Expect(err).NotTo(HaveOccurred())
		Expect(vol).NotTo(BeNil())
		Expect(vol.GetVolume()).NotTo(BeNil())
		Expect(vol.GetVolume().GetId()).NotTo(BeEmpty())

		// ControllerPublishVolume
		By("calling controllerpublish on that volume")

		pubReq := &csi.ControllerPublishVolumeRequest{
			VolumeId: vol.GetVolume().GetId(),
			NodeId:   "some-fake-node-id",
			VolumeCapability: &csi.VolumeCapability{
				AccessType: &csi.VolumeCapability_Mount{
					Mount: &csi.VolumeCapability_MountVolume{},
				},
				AccessMode: &csi.VolumeCapability_AccessMode{
					Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER,
				},
			},
			Readonly: false,
		}

		if sc.Secrets != nil {
			pubReq.ControllerPublishSecrets = sc.Secrets.ControllerPublishVolumeSecret
		}

		conpubvol, err := c.ControllerPublishVolume(context.Background(), pubReq)
		Expect(err).To(HaveOccurred())
		Expect(conpubvol).To(BeNil())

		serverError, ok := status.FromError(err)
		Expect(ok).To(BeTrue())
		Expect(serverError.Code()).To(Equal(codes.NotFound))

		By("cleaning up deleting the volume")

		delReq := &csi.DeleteVolumeRequest{
			VolumeId: vol.GetVolume().GetId(),
		}

		if sc.Secrets != nil {
			delReq.ControllerDeleteSecrets = sc.Secrets.DeleteVolumeSecret
		}

		_, err = c.DeleteVolume(context.Background(), delReq)
		Expect(err).NotTo(HaveOccurred())
	})

	It("should fail when the volume is already published but is incompatible", func() {

		// Create Volume First
		By("creating a single node writer volume")
		name := "sanity"
		req := &csi.CreateVolumeRequest{
			Name: name,
			VolumeCapabilities: []*csi.VolumeCapability{
				{
					AccessType: &csi.VolumeCapability_Mount{
						Mount: &csi.VolumeCapability_MountVolume{},
					},
					AccessMode: &csi.VolumeCapability_AccessMode{
						Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER,
					},
				},
			},
		}

		if sc.Secrets != nil {
			req.ControllerCreateSecrets = sc.Secrets.CreateVolumeSecret
		}

		vol, err := c.CreateVolume(context.Background(), req)
		Expect(err).NotTo(HaveOccurred())
		Expect(vol).NotTo(BeNil())
		Expect(vol.GetVolume()).NotTo(BeNil())
		Expect(vol.GetVolume().GetId()).NotTo(BeEmpty())

		By("getting a node id")
		nid, err := n.NodeGetId(
			context.Background(),
			&csi.NodeGetIdRequest{})
		Expect(err).NotTo(HaveOccurred())
		Expect(nid).NotTo(BeNil())
		Expect(nid.GetNodeId()).NotTo(BeEmpty())

		// ControllerPublishVolume
		By("calling controllerpublish on that volume")

		pubReq := &csi.ControllerPublishVolumeRequest{
			VolumeId: vol.GetVolume().GetId(),
			NodeId:   nid.GetNodeId(),
			VolumeCapability: &csi.VolumeCapability{
				AccessType: &csi.VolumeCapability_Mount{
					Mount: &csi.VolumeCapability_MountVolume{},
				},
				AccessMode: &csi.VolumeCapability_AccessMode{
					Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER,
				},
			},
			Readonly: false,
		}

		if sc.Secrets != nil {
			pubReq.ControllerPublishSecrets = sc.Secrets.ControllerPublishVolumeSecret
		}

		conpubvol, err := c.ControllerPublishVolume(context.Background(), pubReq)
		Expect(err).NotTo(HaveOccurred())
		Expect(conpubvol).NotTo(BeNil())

		// Publish again with different attributes.
		pubReq.Readonly = true

		conpubvol, err = c.ControllerPublishVolume(context.Background(), pubReq)
		Expect(err).To(HaveOccurred())
		Expect(conpubvol).To(BeNil())

		serverError, ok := status.FromError(err)
		Expect(ok).To(BeTrue())
		Expect(serverError.Code()).To(Equal(codes.AlreadyExists))

		By("cleaning up unpublishing the volume")

		unpubReq := &csi.ControllerUnpublishVolumeRequest{
			VolumeId: vol.GetVolume().GetId(),
			// NodeID is optional in ControllerUnpublishVolume
			NodeId: nid.GetNodeId(),
		}

		if sc.Secrets != nil {
			unpubReq.ControllerUnpublishSecrets = sc.Secrets.ControllerUnpublishVolumeSecret
		}

		conunpubvol, err := c.ControllerUnpublishVolume(context.Background(), unpubReq)
		Expect(err).NotTo(HaveOccurred())
		Expect(conunpubvol).NotTo(BeNil())

		By("cleaning up deleting the volume")

		delReq := &csi.DeleteVolumeRequest{
			VolumeId: vol.GetVolume().GetId(),
		}

		if sc.Secrets != nil {
			delReq.ControllerDeleteSecrets = sc.Secrets.DeleteVolumeSecret
		}

		_, err = c.DeleteVolume(context.Background(), delReq)
		Expect(err).NotTo(HaveOccurred())
	})
})

var _ = DescribeSanity("ControllerUnpublishVolume [Controller Server]", func(sc *SanityContext) {
	var (
		c csi.ControllerClient
		n csi.NodeClient
	)

	BeforeEach(func() {
		c = csi.NewControllerClient(sc.Conn)
		n = csi.NewNodeClient(sc.Conn)

		if !isControllerCapabilitySupported(c, csi.ControllerServiceCapability_RPC_PUBLISH_UNPUBLISH_VOLUME) {
			Skip("ControllerUnpublishVolume not supported")
		}
	})

	It("should fail when no volume id is provided", func() {

		req := &csi.ControllerUnpublishVolumeRequest{}

		if sc.Secrets != nil {
			req.ControllerUnpublishSecrets = sc.Secrets.ControllerUnpublishVolumeSecret
		}

		_, err := c.ControllerUnpublishVolume(context.Background(), req)
		Expect(err).To(HaveOccurred())

		serverError, ok := status.FromError(err)
		Expect(ok).To(BeTrue())
		Expect(serverError.Code()).To(Equal(codes.InvalidArgument))
	})

	It("should return appropriate values (no optional values added)", func() {

		// Create Volume First
		By("creating a single node writer volume")
		name := "sanity"

		req := &csi.CreateVolumeRequest{
			Name: name,
			VolumeCapabilities: []*csi.VolumeCapability{
				{
					AccessType: &csi.VolumeCapability_Mount{
						Mount: &csi.VolumeCapability_MountVolume{},
					},
					AccessMode: &csi.VolumeCapability_AccessMode{
						Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER,
					},
				},
			},
		}

		if sc.Secrets != nil {
			req.ControllerCreateSecrets = sc.Secrets.CreateVolumeSecret
		}

		vol, err := c.CreateVolume(context.Background(), req)
		Expect(err).NotTo(HaveOccurred())
		Expect(vol).NotTo(BeNil())
		Expect(vol.GetVolume()).NotTo(BeNil())
		Expect(vol.GetVolume().GetId()).NotTo(BeEmpty())

		By("getting a node id")
		nid, err := n.NodeGetId(
			context.Background(),
			&csi.NodeGetIdRequest{})
		Expect(err).NotTo(HaveOccurred())
		Expect(nid).NotTo(BeNil())
		Expect(nid.GetNodeId()).NotTo(BeEmpty())

		// ControllerPublishVolume
		By("calling controllerpublish on that volume")

		pubReq := &csi.ControllerPublishVolumeRequest{
			VolumeId: vol.GetVolume().GetId(),
			NodeId:   nid.GetNodeId(),
			VolumeCapability: &csi.VolumeCapability{
				AccessType: &csi.VolumeCapability_Mount{
					Mount: &csi.VolumeCapability_MountVolume{},
				},
				AccessMode: &csi.VolumeCapability_AccessMode{
					Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER,
				},
			},
			Readonly: false,
		}

		if sc.Secrets != nil {
			pubReq.ControllerPublishSecrets = sc.Secrets.ControllerPublishVolumeSecret
		}

		conpubvol, err := c.ControllerPublishVolume(context.Background(), pubReq)
		Expect(err).NotTo(HaveOccurred())
		Expect(conpubvol).NotTo(BeNil())

		// ControllerUnpublishVolume
		By("calling controllerunpublish on that volume")

		unpubReq := &csi.ControllerUnpublishVolumeRequest{
			VolumeId: vol.GetVolume().GetId(),
			// NodeID is optional in ControllerUnpublishVolume
			NodeId: nid.GetNodeId(),
		}

		if sc.Secrets != nil {
			unpubReq.ControllerUnpublishSecrets = sc.Secrets.ControllerUnpublishVolumeSecret
		}

		conunpubvol, err := c.ControllerUnpublishVolume(context.Background(), unpubReq)
		Expect(err).NotTo(HaveOccurred())
		Expect(conunpubvol).NotTo(BeNil())

		By("cleaning up deleting the volume")

		delReq := &csi.DeleteVolumeRequest{
			VolumeId: vol.GetVolume().GetId(),
		}

		if sc.Secrets != nil {
			delReq.ControllerDeleteSecrets = sc.Secrets.DeleteVolumeSecret
		}

		_, err = c.DeleteVolume(context.Background(), delReq)
		Expect(err).NotTo(HaveOccurred())
	})
})

var _ = DescribeSanity("ListSnapshots [Controller Server]", func(sc *SanityContext) {
	var (
		c csi.ControllerClient
	)

	BeforeEach(func() {
		c = csi.NewControllerClient(sc.Conn)

		if !isControllerCapabilitySupported(c, csi.ControllerServiceCapability_RPC_LIST_SNAPSHOTS) {
			Skip("ListSnapshots not supported")
		}
	})

	It("should return appropriate values (no optional values added)", func() {
		snapshots, err := c.ListSnapshots(
			context.Background(),
			&csi.ListSnapshotsRequest{})
		Expect(err).NotTo(HaveOccurred())
		Expect(snapshots).NotTo(BeNil())

		for _, snapshot := range snapshots.GetEntries() {
			verifySnapshotInfo(snapshot.GetSnapshot())
		}
	})

	It("should return snapshots that match the specify snapshot id", func() {

		By("creating a volume")
		volReq := MakeCreateVolumeReq(sc, "listSnapshots-volume-1")
		volume, err := c.CreateVolume(context.Background(), volReq)
		Expect(err).NotTo(HaveOccurred())

		By("creating a snapshot")
		snapshotReq := MakeCreateSnapshotReq(sc, "listSnapshots-snapshot-1", volume.GetVolume().GetId(), nil)
		snapshot, err := c.CreateSnapshot(context.Background(), snapshotReq)
		Expect(err).NotTo(HaveOccurred())

		snapshots, err := c.ListSnapshots(
			context.Background(),
			&csi.ListSnapshotsRequest{SnapshotId: snapshot.GetSnapshot().GetId()})
		Expect(err).NotTo(HaveOccurred())
		Expect(snapshots).NotTo(BeNil())
		Expect(len(snapshots.GetEntries())).To(BeNumerically("==", 1))
		verifySnapshotInfo(snapshots.GetEntries()[0].GetSnapshot())
		Expect(snapshots.GetEntries()[0].GetSnapshot().GetId()).To(Equal(snapshot.GetSnapshot().GetId()))

		By("cleaning up deleting the volume")
		delVolReq := MakeDeleteVolumeReq(sc, volume.GetVolume().GetId())
		_, err = c.DeleteVolume(context.Background(), delVolReq)
		Expect(err).NotTo(HaveOccurred())

		By("cleaning up deleting the snapshot")
		delSnapReq := MakeDeleteSnapshotReq(sc, snapshot.GetSnapshot().GetId())
		_, err = c.DeleteSnapshot(context.Background(), delSnapReq)
		Expect(err).NotTo(HaveOccurred())
	})

	It("should return empty when the specify snapshot id is not exist", func() {

		snapshots, err := c.ListSnapshots(
			context.Background(),
			&csi.ListSnapshotsRequest{SnapshotId: "none-exist-id"})
		Expect(err).NotTo(HaveOccurred())
		Expect(snapshots).NotTo(BeNil())
		Expect(snapshots.GetEntries()).To(BeEmpty())
	})

	It("should return snapshots that match the specify source volume id)", func() {

		By("creating a volume")
		volReq := MakeCreateVolumeReq(sc, "listSnapshots-volume-2")
		volume, err := c.CreateVolume(context.Background(), volReq)
		Expect(err).NotTo(HaveOccurred())

		By("creating a snapshot")
		snapshotReq := MakeCreateSnapshotReq(sc, "listSnapshots-snapshot-2", volume.GetVolume().GetId(), nil)
		snapshot, err := c.CreateSnapshot(context.Background(), snapshotReq)
		Expect(err).NotTo(HaveOccurred())

		snapshots, err := c.ListSnapshots(
			context.Background(),
			&csi.ListSnapshotsRequest{SourceVolumeId: snapshot.GetSnapshot().GetSourceVolumeId()})
		Expect(err).NotTo(HaveOccurred())
		Expect(snapshots).NotTo(BeNil())
		for _, snap := range snapshots.GetEntries() {
			verifySnapshotInfo(snap.GetSnapshot())
			Expect(snap.GetSnapshot().GetSourceVolumeId()).To(Equal(snapshot.GetSnapshot().GetSourceVolumeId()))
		}

		By("cleaning up deleting the snapshot")
		delSnapReq := MakeDeleteSnapshotReq(sc, snapshot.GetSnapshot().GetId())
		_, err = c.DeleteSnapshot(context.Background(), delSnapReq)
		Expect(err).NotTo(HaveOccurred())

		By("cleaning up deleting the volume")
		delVolReq := MakeDeleteVolumeReq(sc, volume.GetVolume().GetId())
		_, err = c.DeleteVolume(context.Background(), delVolReq)
		Expect(err).NotTo(HaveOccurred())
	})

	It("should return empty when the specify source volume id is not exist", func() {

		snapshots, err := c.ListSnapshots(
			context.Background(),
			&csi.ListSnapshotsRequest{SourceVolumeId: "none-exist-volume-id"})
		Expect(err).NotTo(HaveOccurred())
		Expect(snapshots).NotTo(BeNil())
		Expect(snapshots.GetEntries()).To(BeEmpty())
	})

	It("should fail when an invalid starting_token is passed", func() {
		vols, err := c.ListSnapshots(
			context.Background(),
			&csi.ListSnapshotsRequest{
				StartingToken: "invalid-token",
			},
		)
		Expect(err).To(HaveOccurred())
		Expect(vols).To(BeNil())

		serverError, ok := status.FromError(err)
		Expect(ok).To(BeTrue())
		Expect(serverError.Code()).To(Equal(codes.Aborted))
	})

	It("should fail when the starting_token is greater than total number of snapshots", func() {
		// Get total number of snapshots.
		snapshots, err := c.ListSnapshots(
			context.Background(),
			&csi.ListSnapshotsRequest{})
		Expect(err).NotTo(HaveOccurred())
		Expect(snapshots).NotTo(BeNil())

		totalSnapshots := len(snapshots.GetEntries())

		// Send starting_token that is greater than the total number of snapshots.
		snapshots, err = c.ListSnapshots(
			context.Background(),
			&csi.ListSnapshotsRequest{
				StartingToken: strconv.Itoa(totalSnapshots + 5),
			},
		)
		Expect(err).To(HaveOccurred())
		Expect(snapshots).To(BeNil())

		serverError, ok := status.FromError(err)
		Expect(ok).To(BeTrue())
		Expect(serverError.Code()).To(Equal(codes.Aborted))
	})

	It("check the presence of new snapshots in the snapshot list", func() {
		// List Snapshots before creating new snapshots.
		snapshots, err := c.ListSnapshots(
			context.Background(),
			&csi.ListSnapshotsRequest{})
		Expect(err).NotTo(HaveOccurred())
		Expect(snapshots).NotTo(BeNil())

		totalSnapshots := len(snapshots.GetEntries())

		By("creating a volume")
		volReq := MakeCreateVolumeReq(sc, "listSnapshots-volume-3")
		volume, err := c.CreateVolume(context.Background(), volReq)
		Expect(err).NotTo(HaveOccurred())

		By("creating a snapshot")
		snapReq := MakeCreateSnapshotReq(sc, "listSnapshots-snapshot-3", volume.GetVolume().GetId(), nil)
		snapshot, err := c.CreateSnapshot(context.Background(), snapReq)
		Expect(err).NotTo(HaveOccurred())
		Expect(snapshot).NotTo(BeNil())
		verifySnapshotInfo(snapshot.GetSnapshot())

		snapshots, err = c.ListSnapshots(
			context.Background(),
			&csi.ListSnapshotsRequest{})
		Expect(err).NotTo(HaveOccurred())
		Expect(snapshots).NotTo(BeNil())
		Expect(len(snapshots.GetEntries())).To(Equal(totalSnapshots + 1))

		By("cleaning up deleting the snapshot")
		delSnapReq := MakeDeleteSnapshotReq(sc, snapshot.GetSnapshot().GetId())
		_, err = c.DeleteSnapshot(context.Background(), delSnapReq)
		Expect(err).NotTo(HaveOccurred())

		By("cleaning up deleting the volume")
		delVolReq := MakeDeleteVolumeReq(sc, volume.GetVolume().GetId())
		_, err = c.DeleteVolume(context.Background(), delVolReq)
		Expect(err).NotTo(HaveOccurred())

		// List snapshots and check if the deleted snapshot exists in the snapshot list.
		snapshots, err = c.ListSnapshots(
			context.Background(),
			&csi.ListSnapshotsRequest{})
		Expect(err).NotTo(HaveOccurred())
		Expect(snapshots).NotTo(BeNil())
		Expect(len(snapshots.GetEntries())).To(Equal(totalSnapshots))
	})

	It("should return next token when a limited number of entries are requested", func() {
		// minSnapshotCount is the minimum number of snapshots expected to exist,
		// based on which paginated snapshot listing is performed.
		minSnapshotCount := 5
		// maxEntried is the maximum entries in list snapshot request.
		maxEntries := 2
		// currentTotalVols is the total number of volumes at a given time. It
		// is used to verify that all the snapshots have been listed.
		currentTotalSnapshots := 0

		// Get the number of existing volumes.
		snapshots, err := c.ListSnapshots(
			context.Background(),
			&csi.ListSnapshotsRequest{})
		Expect(err).NotTo(HaveOccurred())
		Expect(snapshots).NotTo(BeNil())

		initialTotalSnapshots := len(snapshots.GetEntries())
		currentTotalSnapshots = initialTotalSnapshots

		createVols := make([]*csi.Volume, 0)
		createSnapshots := make([]*csi.Snapshot, 0)

		// Ensure minimum minVolCount volumes exist.
		if initialTotalSnapshots < minSnapshotCount {

			By("creating required new volumes")
			requiredSnapshots := minSnapshotCount - initialTotalSnapshots

			for i := 1; i <= requiredSnapshots; i++ {
				volReq := MakeCreateVolumeReq(sc, "volume"+strconv.Itoa(i))
				volume, err := c.CreateVolume(context.Background(), volReq)
				Expect(err).NotTo(HaveOccurred())
				Expect(volume).NotTo(BeNil())
				createVols = append(createVols, volume.GetVolume())

				snapReq := MakeCreateSnapshotReq(sc, "snapshot"+strconv.Itoa(i), volume.GetVolume().GetId(), nil)
				snapshot, err := c.CreateSnapshot(context.Background(), snapReq)
				Expect(err).NotTo(HaveOccurred())
				Expect(snapshot).NotTo(BeNil())
				verifySnapshotInfo(snapshot.GetSnapshot())
				createSnapshots = append(createSnapshots, snapshot.GetSnapshot())
			}

			// Update the current total snapshots count.
			currentTotalSnapshots += requiredSnapshots
		}

		// Request list snapshots with max entries maxEntries.
		snapshots, err = c.ListSnapshots(
			context.Background(),
			&csi.ListSnapshotsRequest{
				MaxEntries: int32(maxEntries),
			})
		Expect(err).NotTo(HaveOccurred())
		Expect(snapshots).NotTo(BeNil())

		nextToken := snapshots.GetNextToken()

		Expect(nextToken).To(Equal(strconv.Itoa(maxEntries)))
		Expect(len(snapshots.GetEntries())).To(Equal(maxEntries))

		// Request list snapshots with starting_token and no max entries.
		snapshots, err = c.ListSnapshots(
			context.Background(),
			&csi.ListSnapshotsRequest{
				StartingToken: nextToken,
			})
		Expect(err).NotTo(HaveOccurred())
		Expect(snapshots).NotTo(BeNil())

		// Ensure that all the remaining entries are returned at once.
		Expect(len(snapshots.GetEntries())).To(Equal(currentTotalSnapshots - maxEntries))

		if initialTotalSnapshots < minSnapshotCount {

			By("cleaning up deleting the snapshots")

			for _, snap := range createSnapshots {
				delSnapReq := MakeDeleteSnapshotReq(sc, snap.GetId())
				_, err = c.DeleteSnapshot(context.Background(), delSnapReq)
				Expect(err).NotTo(HaveOccurred())
			}

			By("cleaning up deleting the volumes")

			for _, vol := range createVols {
				delVolReq := MakeDeleteVolumeReq(sc, vol.GetId())
				_, err = c.DeleteVolume(context.Background(), delVolReq)
				Expect(err).NotTo(HaveOccurred())
			}
		}
	})

})

var _ = DescribeSanity("DeleteSnapshot [Controller Server]", func(sc *SanityContext) {
	var (
		c csi.ControllerClient
	)

	BeforeEach(func() {
		c = csi.NewControllerClient(sc.Conn)

		if !isControllerCapabilitySupported(c, csi.ControllerServiceCapability_RPC_CREATE_DELETE_SNAPSHOT) {
			Skip("DeleteSnapshot not supported")
		}
	})

	It("should fail when no snapshot id is provided", func() {

		req := &csi.DeleteSnapshotRequest{}

		if sc.Secrets != nil {
			req.DeleteSnapshotSecrets = sc.Secrets.DeleteSnapshotSecret
		}

		_, err := c.DeleteSnapshot(context.Background(), req)
		Expect(err).To(HaveOccurred())

		serverError, ok := status.FromError(err)
		Expect(ok).To(BeTrue())
		Expect(serverError.Code()).To(Equal(codes.InvalidArgument))
	})

	It("should succeed when an invalid snapshot id is used", func() {

		req := MakeDeleteSnapshotReq(sc, "reallyfakesnapshotid")
		_, err := c.DeleteSnapshot(context.Background(), req)
		Expect(err).NotTo(HaveOccurred())
	})

	It("should return appropriate values (no optional values added)", func() {

		By("creating a volume")
		volReq := MakeCreateVolumeReq(sc, "DeleteSnapshot-volume-1")
		volume, err := c.CreateVolume(context.Background(), volReq)
		Expect(err).NotTo(HaveOccurred())

		// Create Snapshot First
		By("creating a snapshot")
		snapshotReq := MakeCreateSnapshotReq(sc, "DeleteSnapshot-snapshot-1", volume.GetVolume().GetId(), nil)
		snapshot, err := c.CreateSnapshot(context.Background(), snapshotReq)
		Expect(err).NotTo(HaveOccurred())
		Expect(snapshot).NotTo(BeNil())
		verifySnapshotInfo(snapshot.GetSnapshot())

		By("cleaning up deleting the snapshot")
		delSnapReq := MakeDeleteSnapshotReq(sc, snapshot.GetSnapshot().GetId())
		_, err = c.DeleteSnapshot(context.Background(), delSnapReq)
		Expect(err).NotTo(HaveOccurred())

		By("cleaning up deleting the volume")
		delVolReq := MakeDeleteVolumeReq(sc, volume.GetVolume().GetId())
		_, err = c.DeleteVolume(context.Background(), delVolReq)
		Expect(err).NotTo(HaveOccurred())
	})
})

var _ = DescribeSanity("CreateSnapshot [Controller Server]", func(sc *SanityContext) {
	var (
		c csi.ControllerClient
	)

	BeforeEach(func() {
		c = csi.NewControllerClient(sc.Conn)

		if !isControllerCapabilitySupported(c, csi.ControllerServiceCapability_RPC_CREATE_DELETE_SNAPSHOT) {
			Skip("CreateSnapshot not supported")
		}
	})

	It("should fail when no name is provided", func() {

		req := &csi.CreateSnapshotRequest{
			SourceVolumeId: "testId",
		}

		if sc.Secrets != nil {
			req.CreateSnapshotSecrets = sc.Secrets.CreateSnapshotSecret
		}

		_, err := c.CreateSnapshot(context.Background(), req)
		Expect(err).To(HaveOccurred())
		serverError, ok := status.FromError(err)
		Expect(ok).To(BeTrue())
		Expect(serverError.Code()).To(Equal(codes.InvalidArgument))
	})

	It("should fail when no source volume id is provided", func() {

		req := &csi.CreateSnapshotRequest{
			Name: "name",
		}

		if sc.Secrets != nil {
			req.CreateSnapshotSecrets = sc.Secrets.CreateSnapshotSecret
		}

		_, err := c.CreateSnapshot(context.Background(), req)
		Expect(err).To(HaveOccurred())
		serverError, ok := status.FromError(err)
		Expect(ok).To(BeTrue())
		Expect(serverError.Code()).To(Equal(codes.InvalidArgument))
	})

	It("should not fail when requesting to create a snapshot with already existing name and same SourceVolumeId.", func() {

		By("creating a volume")
		volReq := MakeCreateVolumeReq(sc, "CreateSnapshot-volume-1")
		volume, err := c.CreateVolume(context.Background(), volReq)
		Expect(err).NotTo(HaveOccurred())

		By("creating a snapshot")
		snapReq1 := MakeCreateSnapshotReq(sc, "CreateSnapshot-snapshot-1", volume.GetVolume().GetId(), nil)
		snap1, err := c.CreateSnapshot(context.Background(), snapReq1)
		Expect(err).NotTo(HaveOccurred())
		Expect(snap1).NotTo(BeNil())
		verifySnapshotInfo(snap1.GetSnapshot())

		snap2, err := c.CreateSnapshot(context.Background(), snapReq1)
		Expect(err).NotTo(HaveOccurred())
		Expect(snap2).NotTo(BeNil())
		verifySnapshotInfo(snap2.GetSnapshot())

		By("cleaning up deleting the snapshot")
		delSnapReq := MakeDeleteSnapshotReq(sc, snap1.GetSnapshot().GetId())
		_, err = c.DeleteSnapshot(context.Background(), delSnapReq)
		Expect(err).NotTo(HaveOccurred())

		By("cleaning up deleting the volume")
		delVolReq := MakeDeleteVolumeReq(sc, volume.GetVolume().GetId())
		_, err = c.DeleteVolume(context.Background(), delVolReq)
		Expect(err).NotTo(HaveOccurred())
	})

	It("should fail when requesting to create a snapshot with already existing name and different SourceVolumeId.", func() {

		By("creating a volume")
		volume, err := c.CreateVolume(context.Background(), MakeCreateVolumeReq(sc, "CreateSnapshot-volume-2"))
		Expect(err).ToNot(HaveOccurred())

		By("creating a snapshot with the created volume source id")
		req1 := MakeCreateSnapshotReq(sc, "CreateSnapshot-snapshot-2", volume.GetVolume().GetId(), nil)
		snap1, err := c.CreateSnapshot(context.Background(), req1)
		Expect(err).NotTo(HaveOccurred())
		Expect(snap1).NotTo(BeNil())
		verifySnapshotInfo(snap1.GetSnapshot())

		By("creating a snapshot with the same name but different volume source id")
		req2 := MakeCreateSnapshotReq(sc, "CreateSnapshot-snapshot-2", "test001", nil)
		_, err = c.CreateSnapshot(context.Background(), req2)
		Expect(err).To(HaveOccurred())
		serverError, ok := status.FromError(err)
		Expect(ok).To(BeTrue())
		Expect(serverError.Code()).To(Equal(codes.AlreadyExists))

		By("cleaning up deleting the snapshot")
		delSnapReq := MakeDeleteSnapshotReq(sc, snap1.GetSnapshot().GetId())
		_, err = c.DeleteSnapshot(context.Background(), delSnapReq)
		Expect(err).NotTo(HaveOccurred())

		By("cleaning up deleting the volume")
		delVolReq := MakeDeleteVolumeReq(sc, volume.GetVolume().GetId())
		_, err = c.DeleteVolume(context.Background(), delVolReq)
		Expect(err).NotTo(HaveOccurred())
	})
})

func MakeCreateVolumeReq(sc *SanityContext, name string) *csi.CreateVolumeRequest {
	size1 := TestVolumeSize(sc)

	req := &csi.CreateVolumeRequest{
		Name: name,
		VolumeCapabilities: []*csi.VolumeCapability{
			{
				AccessType: &csi.VolumeCapability_Mount{
					Mount: &csi.VolumeCapability_MountVolume{},
				},
				AccessMode: &csi.VolumeCapability_AccessMode{
					Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER,
				},
			},
		},
		CapacityRange: &csi.CapacityRange{
			RequiredBytes: size1,
			LimitBytes:    size1,
		},
	}

	if sc.Secrets != nil {
		req.ControllerCreateSecrets = sc.Secrets.CreateVolumeSecret
	}

	return req
}

func MakeCreateSnapshotReq(sc *SanityContext, name, sourceVolumeId string, parameters map[string]string) *csi.CreateSnapshotRequest {
	req := &csi.CreateSnapshotRequest{
		Name:           name,
		SourceVolumeId: sourceVolumeId,
		Parameters:     parameters,
	}

	if sc.Secrets != nil {
		req.CreateSnapshotSecrets = sc.Secrets.CreateSnapshotSecret
	}

	return req
}

func MakeDeleteSnapshotReq(sc *SanityContext, id string) *csi.DeleteSnapshotRequest {
	delSnapReq := &csi.DeleteSnapshotRequest{
		SnapshotId: id,
	}

	if sc.Secrets != nil {
		delSnapReq.DeleteSnapshotSecrets = sc.Secrets.DeleteSnapshotSecret
	}

	return delSnapReq
}

func MakeDeleteVolumeReq(sc *SanityContext, id string) *csi.DeleteVolumeRequest {
	delVolReq := &csi.DeleteVolumeRequest{
		VolumeId: id,
	}

	if sc.Secrets != nil {
		delVolReq.ControllerDeleteSecrets = sc.Secrets.DeleteVolumeSecret
	}

	return delVolReq
}
