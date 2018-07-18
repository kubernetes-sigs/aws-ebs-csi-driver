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

	csi "github.com/container-storage-interface/spec/lib/go/csi/v0"
	context "golang.org/x/net/context"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
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
