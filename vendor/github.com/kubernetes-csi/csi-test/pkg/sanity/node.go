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
	Expect(caps.GetCapabilities()).NotTo(BeNil())

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

var _ = Describe("NodeGetCapabilities [Node Server]", func() {
	var (
		c csi.NodeClient
	)

	BeforeEach(func() {
		c = csi.NewNodeClient(conn)
	})

	It("should return appropriate capabilities", func() {
		caps, err := c.NodeGetCapabilities(
			context.Background(),
			&csi.NodeGetCapabilitiesRequest{})

		By("checking successful response")
		Expect(err).NotTo(HaveOccurred())
		Expect(caps).NotTo(BeNil())
		Expect(caps.GetCapabilities()).NotTo(BeNil())

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

var _ = Describe("NodeGetId [Node Server]", func() {
	var (
		c csi.NodeClient
	)

	BeforeEach(func() {
		c = csi.NewNodeClient(conn)
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

var _ = Describe("NodeGetInfo [Node Server]", func() {
	var (
		c                                csi.NodeClient
		i                                csi.IdentityClient
		accessibilityConstraintSupported bool
	)

	BeforeEach(func() {
		c = csi.NewNodeClient(conn)
		i = csi.NewIdentityClient(conn)
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

var _ = Describe("NodePublishVolume [Node Server]", func() {
	var (
		s                          csi.ControllerClient
		c                          csi.NodeClient
		controllerPublishSupported bool
		nodeStageSupported         bool
	)

	BeforeEach(func() {
		s = csi.NewControllerClient(conn)
		c = csi.NewNodeClient(conn)
		controllerPublishSupported = isControllerCapabilitySupported(
			s,
			csi.ControllerServiceCapability_RPC_PUBLISH_UNPUBLISH_VOLUME)
		nodeStageSupported = isNodeCapabilitySupported(c, csi.NodeServiceCapability_RPC_STAGE_UNSTAGE_VOLUME)
		if nodeStageSupported {
			err := createMountTargetLocation(config.StagingPath)
			Expect(err).NotTo(HaveOccurred())
		}
	})

	It("should fail when no volume id is provided", func() {

		req := &csi.NodePublishVolumeRequest{}

		if secrets != nil {
			req.NodePublishSecrets = secrets.NodePublishVolumeSecret
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

		if secrets != nil {
			req.NodePublishSecrets = secrets.NodePublishVolumeSecret
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
			TargetPath: config.TargetPath,
		}

		if secrets != nil {
			req.NodePublishSecrets = secrets.NodePublishVolumeSecret
		}

		_, err := c.NodePublishVolume(context.Background(), req)
		Expect(err).To(HaveOccurred())

		serverError, ok := status.FromError(err)
		Expect(ok).To(BeTrue())
		Expect(serverError.Code()).To(Equal(codes.InvalidArgument))
	})

	It("should return appropriate values (no optional values added)", func() {
		testFullWorkflowSuccess(s, c, controllerPublishSupported, nodeStageSupported)
	})
})

var _ = Describe("NodeUnpublishVolume [Node Server]", func() {
	var (
		s                          csi.ControllerClient
		c                          csi.NodeClient
		controllerPublishSupported bool
		nodeStageSupported         bool
	)

	BeforeEach(func() {
		s = csi.NewControllerClient(conn)
		c = csi.NewNodeClient(conn)
		controllerPublishSupported = isControllerCapabilitySupported(
			s,
			csi.ControllerServiceCapability_RPC_PUBLISH_UNPUBLISH_VOLUME)
		nodeStageSupported = isNodeCapabilitySupported(c, csi.NodeServiceCapability_RPC_STAGE_UNSTAGE_VOLUME)
		if nodeStageSupported {
			err := createMountTargetLocation(config.StagingPath)
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
		testFullWorkflowSuccess(s, c, controllerPublishSupported, nodeStageSupported)
	})
})

// TODO: Tests for NodeStageVolume/NodeUnstageVolume
func testFullWorkflowSuccess(s csi.ControllerClient, c csi.NodeClient, controllerPublishSupported, nodeStageSupported bool) {
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

	if secrets != nil {
		req.ControllerCreateSecrets = secrets.CreateVolumeSecret
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
			Readonly: false,
		}

		if secrets != nil {
			pubReq.ControllerPublishSecrets = secrets.ControllerPublishVolumeSecret
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
			StagingTargetPath: config.StagingPath,
		}
		if controllerPublishSupported {
			nodeStageVolReq.PublishInfo = conpubvol.GetPublishInfo()
		}
		if secrets != nil {
			nodeStageVolReq.NodeStageSecrets = secrets.NodeStageVolumeSecret
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
		TargetPath: config.TargetPath,
		VolumeCapability: &csi.VolumeCapability{
			AccessType: &csi.VolumeCapability_Mount{
				Mount: &csi.VolumeCapability_MountVolume{},
			},
			AccessMode: &csi.VolumeCapability_AccessMode{
				Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER,
			},
		},
	}
	if nodeStageSupported {
		nodepubvolRequest.StagingTargetPath = config.StagingPath
	}
	if controllerPublishSupported {
		nodepubvolRequest.PublishInfo = conpubvol.GetPublishInfo()
	}
	if secrets != nil {
		nodepubvolRequest.NodePublishSecrets = secrets.NodePublishVolumeSecret
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
			TargetPath: config.TargetPath,
		})
	Expect(err).NotTo(HaveOccurred())
	Expect(nodeunpubvol).NotTo(BeNil())

	if nodeStageSupported {
		By("cleaning up calling nodeunstage")
		nodeunstagevol, err := c.NodeUnstageVolume(
			context.Background(),
			&csi.NodeUnstageVolumeRequest{
				VolumeId:          vol.GetVolume().GetId(),
				StagingTargetPath: config.StagingPath,
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

		if secrets != nil {
			unpubReq.ControllerUnpublishSecrets = secrets.ControllerUnpublishVolumeSecret
		}

		controllerunpubvol, err := s.ControllerUnpublishVolume(context.Background(), unpubReq)
		Expect(err).NotTo(HaveOccurred())
		Expect(controllerunpubvol).NotTo(BeNil())
	}

	By("cleaning up deleting the volume")

	delReq := &csi.DeleteVolumeRequest{
		VolumeId: vol.GetVolume().GetId(),
	}

	if secrets != nil {
		delReq.ControllerDeleteSecrets = secrets.DeleteVolumeSecret
	}

	_, err = s.DeleteVolume(context.Background(), delReq)
	Expect(err).NotTo(HaveOccurred())
}

var _ = Describe("NodeStageVolume [Node Server]", func() {
	var (
		s                          csi.ControllerClient
		c                          csi.NodeClient
		controllerPublishSupported bool
		nodeStageSupported         bool
		device                     string
	)

	BeforeEach(func() {
		s = csi.NewControllerClient(conn)
		c = csi.NewNodeClient(conn)
		device = "/dev/mock"
		controllerPublishSupported = isControllerCapabilitySupported(
			s,
			csi.ControllerServiceCapability_RPC_PUBLISH_UNPUBLISH_VOLUME)
		nodeStageSupported = isNodeCapabilitySupported(c, csi.NodeServiceCapability_RPC_STAGE_UNSTAGE_VOLUME)
		if nodeStageSupported {
			err := createMountTargetLocation(config.StagingPath)
			Expect(err).NotTo(HaveOccurred())
		} else {
			Skip("NodeStageVolume not supported")
		}
	})

	It("should fail when no volume id is provided", func() {

		req := &csi.NodeStageVolumeRequest{
			StagingTargetPath: config.StagingPath,
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

		if secrets != nil {
			req.NodeStageSecrets = secrets.NodeStageVolumeSecret
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

		if secrets != nil {
			req.NodeStageSecrets = secrets.NodeStageVolumeSecret
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
			StagingTargetPath: config.StagingPath,
			PublishInfo: map[string]string{
				"device": device,
			},
		}

		if secrets != nil {
			req.NodeStageSecrets = secrets.NodeStageVolumeSecret
		}

		_, err := c.NodeStageVolume(context.Background(), req)
		Expect(err).To(HaveOccurred())

		serverError, ok := status.FromError(err)
		Expect(ok).To(BeTrue())
		Expect(serverError.Code()).To(Equal(codes.InvalidArgument))
	})

	It("should return appropriate values (no optional values added)", func() {
		testFullWorkflowSuccess(s, c, controllerPublishSupported, nodeStageSupported)
	})
})

var _ = Describe("NodeUnstageVolume [Node Server]", func() {
	var (
		s                          csi.ControllerClient
		c                          csi.NodeClient
		controllerPublishSupported bool
		nodeStageSupported         bool
	)

	BeforeEach(func() {
		s = csi.NewControllerClient(conn)
		c = csi.NewNodeClient(conn)
		controllerPublishSupported = isControllerCapabilitySupported(
			s,
			csi.ControllerServiceCapability_RPC_PUBLISH_UNPUBLISH_VOLUME)
		nodeStageSupported = isNodeCapabilitySupported(c, csi.NodeServiceCapability_RPC_STAGE_UNSTAGE_VOLUME)
		if nodeStageSupported {
			err := createMountTargetLocation(config.StagingPath)
			Expect(err).NotTo(HaveOccurred())
		} else {
			Skip("NodeUnstageVolume not supported")
		}
	})

	It("should fail when no volume id is provided", func() {

		_, err := c.NodeUnstageVolume(
			context.Background(),
			&csi.NodeUnstageVolumeRequest{
				StagingTargetPath: config.StagingPath,
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
		testFullWorkflowSuccess(s, c, controllerPublishSupported, nodeStageSupported)
	})
})
