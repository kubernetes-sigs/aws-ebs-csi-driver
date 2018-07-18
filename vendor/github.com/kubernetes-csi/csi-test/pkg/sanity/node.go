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

func isNodeCapabilitySupported(c csi.NodeClient,
	capType csi.NodeServiceCapability_RPC_Type,
) bool {

	caps, err := c.NodeGetCapabilities(
		context.Background(),
		&csi.NodeGetCapabilitiesRequest{})
	Expect(err).NotTo(HaveOccurred())
	Expect(caps).NotTo(BeNil())

	for _, cap := range caps.GetCapabilities() {
		Expect(cap.GetRpc()).NotTo(BeNil())
		if cap.GetRpc().GetType() == capType {
			return true
		}
	}
	return false
}

func isPluginCapabilitySupported(c csi.IdentityClient,
	capType csi.PluginCapability_Service_Type,
) bool {

	caps, err := c.GetPluginCapabilities(
		context.Background(),
		&csi.GetPluginCapabilitiesRequest{})
	Expect(err).NotTo(HaveOccurred())
	Expect(caps).NotTo(BeNil())
	Expect(caps.GetCapabilities()).NotTo(BeNil())

	for _, cap := range caps.GetCapabilities() {
		Expect(cap.GetService()).NotTo(BeNil())
		if cap.GetService().GetType() == capType {
			return true
		}
	}
	return false
}

var _ = DescribeSanity("NodeGetCapabilities [Node Server]", func(sc *SanityContext) {
	var (
		c csi.NodeClient
	)

	BeforeEach(func() {
		c = csi.NewNodeClient(sc.Conn)
	})

	It("should return appropriate capabilities", func() {
		caps, err := c.NodeGetCapabilities(
			context.Background(),
			&csi.NodeGetCapabilitiesRequest{})

		By("checking successful response")
		Expect(err).NotTo(HaveOccurred())
		Expect(caps).NotTo(BeNil())

		for _, cap := range caps.GetCapabilities() {
			Expect(cap.GetRpc()).NotTo(BeNil())

			switch cap.GetRpc().GetType() {
			case csi.NodeServiceCapability_RPC_UNKNOWN:
			case csi.NodeServiceCapability_RPC_STAGE_UNSTAGE_VOLUME:
			default:
				Fail(fmt.Sprintf("Unknown capability: %v\n", cap.GetRpc().GetType()))
			}
		}
	})
})

var _ = DescribeSanity("NodeGetId [Node Server]", func(sc *SanityContext) {
	var (
		c csi.NodeClient
	)

	BeforeEach(func() {
		c = csi.NewNodeClient(sc.Conn)
	})

	It("should return appropriate values", func() {
		nid, err := c.NodeGetId(
			context.Background(),
			&csi.NodeGetIdRequest{})

		Expect(err).NotTo(HaveOccurred())
		Expect(nid).NotTo(BeNil())
		Expect(nid.GetNodeId()).NotTo(BeEmpty())
	})
})

var _ = DescribeSanity("NodeGetInfo [Node Server]", func(sc *SanityContext) {
	var (
		c                                csi.NodeClient
		i                                csi.IdentityClient
		accessibilityConstraintSupported bool
	)

	BeforeEach(func() {
		c = csi.NewNodeClient(sc.Conn)
		i = csi.NewIdentityClient(sc.Conn)
		accessibilityConstraintSupported = isPluginCapabilitySupported(i, csi.PluginCapability_Service_ACCESSIBILITY_CONSTRAINTS)
	})

	It("should return approproate values", func() {
		ninfo, err := c.NodeGetInfo(
			context.Background(),
			&csi.NodeGetInfoRequest{})

		Expect(err).NotTo(HaveOccurred())
		Expect(ninfo).NotTo(BeNil())
		Expect(ninfo.GetNodeId()).NotTo(BeEmpty())
		Expect(ninfo.GetMaxVolumesPerNode()).NotTo(BeNumerically("<", 0))

		if accessibilityConstraintSupported {
			Expect(ninfo.GetAccessibleTopology()).NotTo(BeNil())
		}
	})
})

var _ = DescribeSanity("NodePublishVolume [Node Server]", func(sc *SanityContext) {
	var (
		s                          csi.ControllerClient
		c                          csi.NodeClient
		controllerPublishSupported bool
		nodeStageSupported         bool
	)

	BeforeEach(func() {
		s = csi.NewControllerClient(sc.Conn)
		c = csi.NewNodeClient(sc.Conn)
		controllerPublishSupported = isControllerCapabilitySupported(
			s,
			csi.ControllerServiceCapability_RPC_PUBLISH_UNPUBLISH_VOLUME)
		nodeStageSupported = isNodeCapabilitySupported(c, csi.NodeServiceCapability_RPC_STAGE_UNSTAGE_VOLUME)
		if nodeStageSupported {
			err := createMountTargetLocation(sc.Config.StagingPath)
			Expect(err).NotTo(HaveOccurred())
		}
	})

	It("should fail when no volume id is provided", func() {

		req := &csi.NodePublishVolumeRequest{}

		if sc.Secrets != nil {
			req.NodePublishSecrets = sc.Secrets.NodePublishVolumeSecret
		}

		_, err := c.NodePublishVolume(context.Background(), req)
		Expect(err).To(HaveOccurred())

		serverError, ok := status.FromError(err)
		Expect(ok).To(BeTrue())
		Expect(serverError.Code()).To(Equal(codes.InvalidArgument))
	})

	It("should fail when no target path is provided", func() {

		req := &csi.NodePublishVolumeRequest{
			VolumeId: "id",
		}

		if sc.Secrets != nil {
			req.NodePublishSecrets = sc.Secrets.NodePublishVolumeSecret
		}

		_, err := c.NodePublishVolume(context.Background(), req)
		Expect(err).To(HaveOccurred())

		serverError, ok := status.FromError(err)
		Expect(ok).To(BeTrue())
		Expect(serverError.Code()).To(Equal(codes.InvalidArgument))
	})

	It("should fail when no volume capability is provided", func() {

		req := &csi.NodePublishVolumeRequest{
			VolumeId:   "id",
			TargetPath: sc.Config.TargetPath,
		}

		if sc.Secrets != nil {
			req.NodePublishSecrets = sc.Secrets.NodePublishVolumeSecret
		}

		_, err := c.NodePublishVolume(context.Background(), req)
		Expect(err).To(HaveOccurred())

		serverError, ok := status.FromError(err)
		Expect(ok).To(BeTrue())
		Expect(serverError.Code()).To(Equal(codes.InvalidArgument))
	})

	It("should return appropriate values (no optional values added)", func() {
		testFullWorkflowSuccess(sc, s, c, controllerPublishSupported, nodeStageSupported)
	})
})

var _ = DescribeSanity("NodeUnpublishVolume [Node Server]", func(sc *SanityContext) {
	var (
		s                          csi.ControllerClient
		c                          csi.NodeClient
		controllerPublishSupported bool
		nodeStageSupported         bool
	)

	BeforeEach(func() {
		s = csi.NewControllerClient(sc.Conn)
		c = csi.NewNodeClient(sc.Conn)
		controllerPublishSupported = isControllerCapabilitySupported(
			s,
			csi.ControllerServiceCapability_RPC_PUBLISH_UNPUBLISH_VOLUME)
		nodeStageSupported = isNodeCapabilitySupported(c, csi.NodeServiceCapability_RPC_STAGE_UNSTAGE_VOLUME)
		if nodeStageSupported {
			err := createMountTargetLocation(sc.Config.StagingPath)
			Expect(err).NotTo(HaveOccurred())
		}
	})

	It("should fail when no volume id is provided", func() {

		_, err := c.NodeUnpublishVolume(
			context.Background(),
			&csi.NodeUnpublishVolumeRequest{})
		Expect(err).To(HaveOccurred())

		serverError, ok := status.FromError(err)
		Expect(ok).To(BeTrue())
		Expect(serverError.Code()).To(Equal(codes.InvalidArgument))
	})

	It("should fail when no target path is provided", func() {

		_, err := c.NodeUnpublishVolume(
			context.Background(),
			&csi.NodeUnpublishVolumeRequest{
				VolumeId: "id",
			})
		Expect(err).To(HaveOccurred())

		serverError, ok := status.FromError(err)
		Expect(ok).To(BeTrue())
		Expect(serverError.Code()).To(Equal(codes.InvalidArgument))
	})

	It("should return appropriate values (no optional values added)", func() {
		testFullWorkflowSuccess(sc, s, c, controllerPublishSupported, nodeStageSupported)
	})
})

// TODO: Tests for NodeStageVolume/NodeUnstageVolume
func testFullWorkflowSuccess(sc *SanityContext, s csi.ControllerClient, c csi.NodeClient, controllerPublishSupported, nodeStageSupported bool) {
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

	vol, err := s.CreateVolume(context.Background(), req)
	Expect(err).NotTo(HaveOccurred())
	Expect(vol).NotTo(BeNil())
	Expect(vol.GetVolume()).NotTo(BeNil())
	Expect(vol.GetVolume().GetId()).NotTo(BeEmpty())

	By("getting a node id")
	nid, err := c.NodeGetId(
		context.Background(),
		&csi.NodeGetIdRequest{})
	Expect(err).NotTo(HaveOccurred())
	Expect(nid).NotTo(BeNil())
	Expect(nid.GetNodeId()).NotTo(BeEmpty())
	var conpubvol *csi.ControllerPublishVolumeResponse
	if controllerPublishSupported {
		By("controller publishing volume")

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
			VolumeAttributes: vol.GetVolume().GetAttributes(),
			Readonly:         false,
		}

		if sc.Secrets != nil {
			pubReq.ControllerPublishSecrets = sc.Secrets.ControllerPublishVolumeSecret
		}

		conpubvol, err = s.ControllerPublishVolume(context.Background(), pubReq)
		Expect(err).NotTo(HaveOccurred())
		Expect(conpubvol).NotTo(BeNil())
	}
	// NodeStageVolume
	if nodeStageSupported {
		By("node staging volume")
		nodeStageVolReq := &csi.NodeStageVolumeRequest{
			VolumeId: vol.GetVolume().GetId(),
			VolumeCapability: &csi.VolumeCapability{
				AccessType: &csi.VolumeCapability_Mount{
					Mount: &csi.VolumeCapability_MountVolume{},
				},
				AccessMode: &csi.VolumeCapability_AccessMode{
					Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER,
				},
			},
			StagingTargetPath: sc.Config.StagingPath,
			VolumeAttributes:  vol.GetVolume().GetAttributes(),
		}
		if controllerPublishSupported {
			nodeStageVolReq.PublishInfo = conpubvol.GetPublishInfo()
		}
		if sc.Secrets != nil {
			nodeStageVolReq.NodeStageSecrets = sc.Secrets.NodeStageVolumeSecret
		}
		nodestagevol, err := c.NodeStageVolume(
			context.Background(), nodeStageVolReq)
		Expect(err).NotTo(HaveOccurred())
		Expect(nodestagevol).NotTo(BeNil())
	}
	// NodePublishVolume
	By("publishing the volume on a node")
	nodepubvolRequest := &csi.NodePublishVolumeRequest{
		VolumeId:   vol.GetVolume().GetId(),
		TargetPath: sc.Config.TargetPath,
		VolumeCapability: &csi.VolumeCapability{
			AccessType: &csi.VolumeCapability_Mount{
				Mount: &csi.VolumeCapability_MountVolume{},
			},
			AccessMode: &csi.VolumeCapability_AccessMode{
				Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER,
			},
		},
		VolumeAttributes: vol.GetVolume().GetAttributes(),
	}
	if nodeStageSupported {
		nodepubvolRequest.StagingTargetPath = sc.Config.StagingPath
	}
	if controllerPublishSupported {
		nodepubvolRequest.PublishInfo = conpubvol.GetPublishInfo()
	}
	if sc.Secrets != nil {
		nodepubvolRequest.NodePublishSecrets = sc.Secrets.NodePublishVolumeSecret
	}
	nodepubvol, err := c.NodePublishVolume(context.Background(), nodepubvolRequest)
	Expect(err).NotTo(HaveOccurred())
	Expect(nodepubvol).NotTo(BeNil())

	// NodeUnpublishVolume
	By("cleaning up calling nodeunpublish")
	nodeunpubvol, err := c.NodeUnpublishVolume(
		context.Background(),
		&csi.NodeUnpublishVolumeRequest{
			VolumeId:   vol.GetVolume().GetId(),
			TargetPath: sc.Config.TargetPath,
		})
	Expect(err).NotTo(HaveOccurred())
	Expect(nodeunpubvol).NotTo(BeNil())

	if nodeStageSupported {
		By("cleaning up calling nodeunstage")
		nodeunstagevol, err := c.NodeUnstageVolume(
			context.Background(),
			&csi.NodeUnstageVolumeRequest{
				VolumeId:          vol.GetVolume().GetId(),
				StagingTargetPath: sc.Config.StagingPath,
			},
		)
		Expect(err).NotTo(HaveOccurred())
		Expect(nodeunstagevol).NotTo(BeNil())
	}

	if controllerPublishSupported {
		By("cleaning up calling controllerunpublishing")

		unpubReq := &csi.ControllerUnpublishVolumeRequest{
			VolumeId: vol.GetVolume().GetId(),
			NodeId:   nid.GetNodeId(),
		}

		if sc.Secrets != nil {
			unpubReq.ControllerUnpublishSecrets = sc.Secrets.ControllerUnpublishVolumeSecret
		}

		controllerunpubvol, err := s.ControllerUnpublishVolume(context.Background(), unpubReq)
		Expect(err).NotTo(HaveOccurred())
		Expect(controllerunpubvol).NotTo(BeNil())
	}

	By("cleaning up deleting the volume")

	delReq := &csi.DeleteVolumeRequest{
		VolumeId: vol.GetVolume().GetId(),
	}

	if sc.Secrets != nil {
		delReq.ControllerDeleteSecrets = sc.Secrets.DeleteVolumeSecret
	}

	_, err = s.DeleteVolume(context.Background(), delReq)
	Expect(err).NotTo(HaveOccurred())
}

var _ = DescribeSanity("NodeStageVolume [Node Server]", func(sc *SanityContext) {
	var (
		s                          csi.ControllerClient
		c                          csi.NodeClient
		controllerPublishSupported bool
		nodeStageSupported         bool
		device                     string
	)

	BeforeEach(func() {
		s = csi.NewControllerClient(sc.Conn)
		c = csi.NewNodeClient(sc.Conn)
		device = "/dev/mock"
		controllerPublishSupported = isControllerCapabilitySupported(
			s,
			csi.ControllerServiceCapability_RPC_PUBLISH_UNPUBLISH_VOLUME)
		nodeStageSupported = isNodeCapabilitySupported(c, csi.NodeServiceCapability_RPC_STAGE_UNSTAGE_VOLUME)
		if nodeStageSupported {
			err := createMountTargetLocation(sc.Config.StagingPath)
			Expect(err).NotTo(HaveOccurred())
		} else {
			Skip("NodeStageVolume not supported")
		}
	})

	It("should fail when no volume id is provided", func() {

		req := &csi.NodeStageVolumeRequest{
			StagingTargetPath: sc.Config.StagingPath,
			VolumeCapability: &csi.VolumeCapability{
				AccessType: &csi.VolumeCapability_Mount{
					Mount: &csi.VolumeCapability_MountVolume{},
				},
				AccessMode: &csi.VolumeCapability_AccessMode{
					Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER,
				},
			},
			PublishInfo: map[string]string{
				"device": device,
			},
		}

		if sc.Secrets != nil {
			req.NodeStageSecrets = sc.Secrets.NodeStageVolumeSecret
		}

		_, err := c.NodeStageVolume(context.Background(), req)
		Expect(err).To(HaveOccurred())

		serverError, ok := status.FromError(err)
		Expect(ok).To(BeTrue())
		Expect(serverError.Code()).To(Equal(codes.InvalidArgument))
	})

	It("should fail when no staging target path is provided", func() {

		req := &csi.NodeStageVolumeRequest{
			VolumeId: "id",
			VolumeCapability: &csi.VolumeCapability{
				AccessType: &csi.VolumeCapability_Mount{
					Mount: &csi.VolumeCapability_MountVolume{},
				},
				AccessMode: &csi.VolumeCapability_AccessMode{
					Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER,
				},
			},
			PublishInfo: map[string]string{
				"device": device,
			},
		}

		if sc.Secrets != nil {
			req.NodeStageSecrets = sc.Secrets.NodeStageVolumeSecret
		}

		_, err := c.NodeStageVolume(context.Background(), req)
		Expect(err).To(HaveOccurred())

		serverError, ok := status.FromError(err)
		Expect(ok).To(BeTrue())
		Expect(serverError.Code()).To(Equal(codes.InvalidArgument))
	})

	It("should fail when no volume capability is provided", func() {

		req := &csi.NodeStageVolumeRequest{
			VolumeId:          "id",
			StagingTargetPath: sc.Config.StagingPath,
			PublishInfo: map[string]string{
				"device": device,
			},
		}

		if sc.Secrets != nil {
			req.NodeStageSecrets = sc.Secrets.NodeStageVolumeSecret
		}

		_, err := c.NodeStageVolume(context.Background(), req)
		Expect(err).To(HaveOccurred())

		serverError, ok := status.FromError(err)
		Expect(ok).To(BeTrue())
		Expect(serverError.Code()).To(Equal(codes.InvalidArgument))
	})

	It("should return appropriate values (no optional values added)", func() {
		testFullWorkflowSuccess(sc, s, c, controllerPublishSupported, nodeStageSupported)
	})
})

var _ = DescribeSanity("NodeUnstageVolume [Node Server]", func(sc *SanityContext) {
	var (
		s                          csi.ControllerClient
		c                          csi.NodeClient
		controllerPublishSupported bool
		nodeStageSupported         bool
	)

	BeforeEach(func() {
		s = csi.NewControllerClient(sc.Conn)
		c = csi.NewNodeClient(sc.Conn)
		controllerPublishSupported = isControllerCapabilitySupported(
			s,
			csi.ControllerServiceCapability_RPC_PUBLISH_UNPUBLISH_VOLUME)
		nodeStageSupported = isNodeCapabilitySupported(c, csi.NodeServiceCapability_RPC_STAGE_UNSTAGE_VOLUME)
		if nodeStageSupported {
			err := createMountTargetLocation(sc.Config.StagingPath)
			Expect(err).NotTo(HaveOccurred())
		} else {
			Skip("NodeUnstageVolume not supported")
		}
	})

	It("should fail when no volume id is provided", func() {

		_, err := c.NodeUnstageVolume(
			context.Background(),
			&csi.NodeUnstageVolumeRequest{
				StagingTargetPath: sc.Config.StagingPath,
			})
		Expect(err).To(HaveOccurred())

		serverError, ok := status.FromError(err)
		Expect(ok).To(BeTrue())
		Expect(serverError.Code()).To(Equal(codes.InvalidArgument))
	})

	It("should fail when no staging target path is provided", func() {

		_, err := c.NodeUnstageVolume(
			context.Background(),
			&csi.NodeUnstageVolumeRequest{
				VolumeId: "id",
			})
		Expect(err).To(HaveOccurred())

		serverError, ok := status.FromError(err)
		Expect(ok).To(BeTrue())
		Expect(serverError.Code()).To(Equal(codes.InvalidArgument))
	})

	It("should return appropriate values (no optional values added)", func() {
		testFullWorkflowSuccess(sc, s, c, controllerPublishSupported, nodeStageSupported)
	})
})
