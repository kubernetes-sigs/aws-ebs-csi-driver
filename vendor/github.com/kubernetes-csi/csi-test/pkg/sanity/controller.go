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
	"context"
	"fmt"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/container-storage-interface/spec/lib/go/csi"

	"strconv"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

const (
	// DefTestVolumeSize defines the base size of dynamically
	// provisioned volumes. 10GB by default, can be overridden by
	// setting Config.TestVolumeSize.
	DefTestVolumeSize int64 = 10 * 1024 * 1024 * 1024

	MaxNameLength int = 128
)

func TestVolumeSize(sc *SanityContext) int64 {
	if sc.Config.TestVolumeSize > 0 {
		return sc.Config.TestVolumeSize
	}
	return DefTestVolumeSize
}

func verifyVolumeInfo(v *csi.Volume) {
	Expect(v).NotTo(BeNil())
	Expect(v.GetVolumeId()).NotTo(BeEmpty())
}

func verifySnapshotInfo(snapshot *csi.Snapshot) {
	Expect(snapshot).NotTo(BeNil())
	Expect(snapshot.GetSnapshotId()).NotTo(BeEmpty())
	Expect(snapshot.GetSourceVolumeId()).NotTo(BeEmpty())
	Expect(snapshot.GetCreationTime()).NotTo(BeZero())
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

var _ = DescribeSanity("Controller Service", func(sc *SanityContext) {
	var (
		c csi.ControllerClient
		n csi.NodeClient

		cl *Cleanup
	)

	BeforeEach(func() {
		c = csi.NewControllerClient(sc.Conn)
		n = csi.NewNodeClient(sc.Conn)

		cl = &Cleanup{
			NodeClient:       n,
			ControllerClient: c,
			Context:          sc,
		}
	})

	AfterEach(func() {
		cl.DeleteVolumes()
	})

	Describe("ControllerGetCapabilities", func() {
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
				case csi.ControllerServiceCapability_RPC_PUBLISH_READONLY:
				default:
					Fail(fmt.Sprintf("Unknown capability: %v\n", cap.GetRpc().GetType()))
				}
			}
		})
	})

	Describe("GetCapacity", func() {
		BeforeEach(func() {
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

	Describe("ListVolumes", func() {
		BeforeEach(func() {
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

	Describe("CreateVolume", func() {
		BeforeEach(func() {
			if !isControllerCapabilitySupported(c, csi.ControllerServiceCapability_RPC_CREATE_DELETE_VOLUME) {
				Skip("CreateVolume not supported")
			}
		})

		It("should fail when no name is provided", func() {
			vol, err := c.CreateVolume(
				context.Background(),
				&csi.CreateVolumeRequest{
					Secrets:    sc.Secrets.CreateVolumeSecret,
					Parameters: sc.Config.TestVolumeParameters,
				},
			)
			cl.MaybeRegisterVolume("", vol, err)
			Expect(err).To(HaveOccurred())

			serverError, ok := status.FromError(err)
			Expect(ok).To(BeTrue())
			Expect(serverError.Code()).To(Equal(codes.InvalidArgument))
		})

		It("should fail when no volume capabilities are provided", func() {
			name := uniqueString("sanity-controller-create-no-volume-capabilities")
			vol, err := c.CreateVolume(
				context.Background(),
				&csi.CreateVolumeRequest{
					Name:       name,
					Secrets:    sc.Secrets.CreateVolumeSecret,
					Parameters: sc.Config.TestVolumeParameters,
				},
			)
			cl.MaybeRegisterVolume(name, vol, err)
			Expect(err).To(HaveOccurred())

			serverError, ok := status.FromError(err)
			Expect(ok).To(BeTrue())
			Expect(serverError.Code()).To(Equal(codes.InvalidArgument))
		})

		It("should return appropriate values SingleNodeWriter NoCapacity Type:Mount", func() {

			By("creating a volume")
			name := uniqueString("sanity-controller-create-single-no-capacity")

			vol, err := c.CreateVolume(
				context.Background(),
				&csi.CreateVolumeRequest{
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
					Secrets:    sc.Secrets.CreateVolumeSecret,
					Parameters: sc.Config.TestVolumeParameters,
				},
			)
			Expect(err).NotTo(HaveOccurred())
			Expect(vol).NotTo(BeNil())
			Expect(vol.GetVolume()).NotTo(BeNil())
			Expect(vol.GetVolume().GetVolumeId()).NotTo(BeEmpty())
			cl.RegisterVolume(name, VolumeInfo{VolumeID: vol.GetVolume().GetVolumeId()})

			By("cleaning up deleting the volume")

			_, err = c.DeleteVolume(
				context.Background(),
				&csi.DeleteVolumeRequest{
					VolumeId: vol.GetVolume().GetVolumeId(),
					Secrets:  sc.Secrets.DeleteVolumeSecret,
				},
			)
			Expect(err).NotTo(HaveOccurred())
			cl.UnregisterVolume(name)
		})

		It("should return appropriate values SingleNodeWriter WithCapacity 1Gi Type:Mount", func() {

			By("creating a volume")
			name := uniqueString("sanity-controller-create-single-with-capacity")

			vol, err := c.CreateVolume(
				context.Background(),
				&csi.CreateVolumeRequest{
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
					Secrets:    sc.Secrets.CreateVolumeSecret,
					Parameters: sc.Config.TestVolumeParameters,
				},
			)
			if serverError, ok := status.FromError(err); ok &&
				(serverError.Code() == codes.OutOfRange || serverError.Code() == codes.Unimplemented) {
				Skip("Required bytes not supported")
			}
			Expect(err).NotTo(HaveOccurred())
			Expect(vol).NotTo(BeNil())
			Expect(vol.GetVolume()).NotTo(BeNil())
			Expect(vol.GetVolume().GetVolumeId()).NotTo(BeEmpty())
			cl.RegisterVolume(name, VolumeInfo{VolumeID: vol.GetVolume().GetVolumeId()})
			Expect(vol.GetVolume().GetCapacityBytes()).To(BeNumerically(">=", TestVolumeSize(sc)))

			By("cleaning up deleting the volume")

			_, err = c.DeleteVolume(
				context.Background(),
				&csi.DeleteVolumeRequest{
					VolumeId: vol.GetVolume().GetVolumeId(),
					Secrets:  sc.Secrets.DeleteVolumeSecret,
				},
			)
			Expect(err).NotTo(HaveOccurred())
			cl.UnregisterVolume(name)
		})
		It("should not fail when requesting to create a volume with already existing name and same capacity.", func() {

			By("creating a volume")
			name := uniqueString("sanity-controller-create-twice")
			size := TestVolumeSize(sc)

			vol1, err := c.CreateVolume(
				context.Background(),
				&csi.CreateVolumeRequest{
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
					Secrets:    sc.Secrets.CreateVolumeSecret,
					Parameters: sc.Config.TestVolumeParameters,
				},
			)
			Expect(err).NotTo(HaveOccurred())
			Expect(vol1).NotTo(BeNil())
			Expect(vol1.GetVolume()).NotTo(BeNil())
			Expect(vol1.GetVolume().GetVolumeId()).NotTo(BeEmpty())
			cl.RegisterVolume(name, VolumeInfo{VolumeID: vol1.GetVolume().GetVolumeId()})
			Expect(vol1.GetVolume().GetCapacityBytes()).To(BeNumerically(">=", size))

			vol2, err := c.CreateVolume(
				context.Background(),
				&csi.CreateVolumeRequest{
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
					Secrets:    sc.Secrets.CreateVolumeSecret,
					Parameters: sc.Config.TestVolumeParameters,
				},
			)
			Expect(err).NotTo(HaveOccurred())
			Expect(vol2).NotTo(BeNil())
			Expect(vol2.GetVolume()).NotTo(BeNil())
			Expect(vol2.GetVolume().GetVolumeId()).NotTo(BeEmpty())
			Expect(vol2.GetVolume().GetCapacityBytes()).To(BeNumerically(">=", size))
			Expect(vol1.GetVolume().GetVolumeId()).To(Equal(vol2.GetVolume().GetVolumeId()))

			By("cleaning up deleting the volume")

			_, err = c.DeleteVolume(
				context.Background(),
				&csi.DeleteVolumeRequest{
					VolumeId: vol1.GetVolume().GetVolumeId(),
					Secrets:  sc.Secrets.DeleteVolumeSecret,
				},
			)
			Expect(err).NotTo(HaveOccurred())
			cl.UnregisterVolume(name)
		})
		It("should fail when requesting to create a volume with already existing name and different capacity.", func() {

			By("creating a volume")
			name := uniqueString("sanity-controller-create-twice-different")
			size1 := TestVolumeSize(sc)

			vol1, err := c.CreateVolume(
				context.Background(),
				&csi.CreateVolumeRequest{
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
					Secrets:    sc.Secrets.CreateVolumeSecret,
					Parameters: sc.Config.TestVolumeParameters,
				},
			)
			Expect(err).ToNot(HaveOccurred())
			Expect(vol1).NotTo(BeNil())
			Expect(vol1.GetVolume()).NotTo(BeNil())
			Expect(vol1.GetVolume().GetVolumeId()).NotTo(BeEmpty())
			cl.RegisterVolume(name, VolumeInfo{VolumeID: vol1.GetVolume().GetVolumeId()})
			size2 := 2 * TestVolumeSize(sc)

			_, err = c.CreateVolume(
				context.Background(),
				&csi.CreateVolumeRequest{
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
					Secrets:    sc.Secrets.CreateVolumeSecret,
					Parameters: sc.Config.TestVolumeParameters,
				},
			)
			Expect(err).To(HaveOccurred())
			serverError, ok := status.FromError(err)
			Expect(ok).To(BeTrue())
			Expect(serverError.Code()).To(Equal(codes.AlreadyExists))

			By("cleaning up deleting the volume")

			_, err = c.DeleteVolume(
				context.Background(),
				&csi.DeleteVolumeRequest{
					VolumeId: vol1.GetVolume().GetVolumeId(),
					Secrets:  sc.Secrets.DeleteVolumeSecret,
				},
			)
			Expect(err).NotTo(HaveOccurred())
			cl.UnregisterVolume(name)
		})

		It("should not fail when creating volume with maximum-length name", func() {

			nameBytes := make([]byte, MaxNameLength)
			for i := 0; i < MaxNameLength; i++ {
				nameBytes[i] = 'a'
			}
			name := string(nameBytes)
			By("creating a volume")
			size := TestVolumeSize(sc)

			vol, err := c.CreateVolume(
				context.Background(),
				&csi.CreateVolumeRequest{
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
					Secrets:    sc.Secrets.CreateVolumeSecret,
					Parameters: sc.Config.TestVolumeParameters,
				},
			)
			Expect(err).NotTo(HaveOccurred())
			Expect(vol).NotTo(BeNil())
			Expect(vol.GetVolume()).NotTo(BeNil())
			Expect(vol.GetVolume().GetVolumeId()).NotTo(BeEmpty())
			cl.RegisterVolume(name, VolumeInfo{VolumeID: vol.GetVolume().GetVolumeId()})
			Expect(vol.GetVolume().GetCapacityBytes()).To(BeNumerically(">=", size))

			By("cleaning up deleting the volume")

			_, err = c.DeleteVolume(
				context.Background(),
				&csi.DeleteVolumeRequest{
					VolumeId: vol.GetVolume().GetVolumeId(),
					Secrets:  sc.Secrets.DeleteVolumeSecret,
				},
			)
			Expect(err).NotTo(HaveOccurred())
			cl.UnregisterVolume(name)
		})
	})

	Describe("DeleteVolume", func() {
		BeforeEach(func() {
			if !isControllerCapabilitySupported(c, csi.ControllerServiceCapability_RPC_CREATE_DELETE_VOLUME) {
				Skip("DeleteVolume not supported")
			}
		})

		It("should fail when no volume id is provided", func() {

			_, err := c.DeleteVolume(
				context.Background(),
				&csi.DeleteVolumeRequest{
					Secrets: sc.Secrets.DeleteVolumeSecret,
				},
			)
			Expect(err).To(HaveOccurred())

			serverError, ok := status.FromError(err)
			Expect(ok).To(BeTrue())
			Expect(serverError.Code()).To(Equal(codes.InvalidArgument))
		})

		It("should succeed when an invalid volume id is used", func() {

			_, err := c.DeleteVolume(
				context.Background(),
				&csi.DeleteVolumeRequest{
					VolumeId: "reallyfakevolumeid",
					Secrets:  sc.Secrets.DeleteVolumeSecret,
				},
			)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should return appropriate values (no optional values added)", func() {

			// Create Volume First
			By("creating a volume")
			name := uniqueString("sanity-controller-create-appropriate")

			vol, err := c.CreateVolume(
				context.Background(),
				&csi.CreateVolumeRequest{
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
					Secrets:    sc.Secrets.CreateVolumeSecret,
					Parameters: sc.Config.TestVolumeParameters,
				},
			)
			Expect(err).NotTo(HaveOccurred())
			Expect(vol).NotTo(BeNil())
			Expect(vol.GetVolume()).NotTo(BeNil())
			Expect(vol.GetVolume().GetVolumeId()).NotTo(BeEmpty())
			cl.RegisterVolume(name, VolumeInfo{VolumeID: vol.GetVolume().GetVolumeId()})

			// Delete Volume
			By("deleting a volume")

			_, err = c.DeleteVolume(
				context.Background(),
				&csi.DeleteVolumeRequest{
					VolumeId: vol.GetVolume().GetVolumeId(),
					Secrets:  sc.Secrets.DeleteVolumeSecret,
				},
			)
			Expect(err).NotTo(HaveOccurred())
			cl.UnregisterVolume(name)
		})
	})

	Describe("ValidateVolumeCapabilities", func() {
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
			name := uniqueString("sanity-controller-validate")

			vol, err := c.CreateVolume(
				context.Background(),
				&csi.CreateVolumeRequest{
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
					Secrets:    sc.Secrets.CreateVolumeSecret,
					Parameters: sc.Config.TestVolumeParameters,
				},
			)
			Expect(err).NotTo(HaveOccurred())
			Expect(vol).NotTo(BeNil())
			Expect(vol.GetVolume()).NotTo(BeNil())
			Expect(vol.GetVolume().GetVolumeId()).NotTo(BeEmpty())
			cl.RegisterVolume(name, VolumeInfo{VolumeID: vol.GetVolume().GetVolumeId()})

			// ValidateVolumeCapabilities
			By("validating volume capabilities")
			valivolcap, err := c.ValidateVolumeCapabilities(
				context.Background(),
				&csi.ValidateVolumeCapabilitiesRequest{
					VolumeId: vol.GetVolume().GetVolumeId(),
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

			// If confirmation is provided then it is REQUIRED to provide
			// the volume capabilities
			if valivolcap.GetConfirmed() != nil {
				Expect(valivolcap.GetConfirmed().GetVolumeCapabilities()).NotTo(BeEmpty())
			}

			By("cleaning up deleting the volume")

			_, err = c.DeleteVolume(
				context.Background(),
				&csi.DeleteVolumeRequest{
					VolumeId: vol.GetVolume().GetVolumeId(),
					Secrets:  sc.Secrets.DeleteVolumeSecret,
				},
			)
			Expect(err).NotTo(HaveOccurred())
			cl.UnregisterVolume(name)
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

	Describe("ControllerPublishVolume", func() {
		BeforeEach(func() {
			if !isControllerCapabilitySupported(c, csi.ControllerServiceCapability_RPC_PUBLISH_UNPUBLISH_VOLUME) {
				Skip("ControllerPublishVolume not supported")
			}
		})

		It("should fail when no volume id is provided", func() {

			_, err := c.ControllerPublishVolume(
				context.Background(),
				&csi.ControllerPublishVolumeRequest{
					Secrets: sc.Secrets.ControllerPublishVolumeSecret,
				},
			)
			Expect(err).To(HaveOccurred())

			serverError, ok := status.FromError(err)
			Expect(ok).To(BeTrue())
			Expect(serverError.Code()).To(Equal(codes.InvalidArgument))
		})

		It("should fail when no node id is provided", func() {

			_, err := c.ControllerPublishVolume(
				context.Background(),
				&csi.ControllerPublishVolumeRequest{
					VolumeId: "id",
					Secrets:  sc.Secrets.ControllerPublishVolumeSecret,
				},
			)
			Expect(err).To(HaveOccurred())

			serverError, ok := status.FromError(err)
			Expect(ok).To(BeTrue())
			Expect(serverError.Code()).To(Equal(codes.InvalidArgument))
		})

		It("should fail when no volume capability is provided", func() {

			_, err := c.ControllerPublishVolume(
				context.Background(),
				&csi.ControllerPublishVolumeRequest{
					VolumeId: "id",
					NodeId:   "fakenode",
					Secrets:  sc.Secrets.ControllerPublishVolumeSecret,
				},
			)
			Expect(err).To(HaveOccurred())

			serverError, ok := status.FromError(err)
			Expect(ok).To(BeTrue())
			Expect(serverError.Code()).To(Equal(codes.InvalidArgument))
		})

		It("should return appropriate values (no optional values added)", func() {

			// Create Volume First
			By("creating a single node writer volume")
			name := uniqueString("sanity-controller-publish")

			vol, err := c.CreateVolume(
				context.Background(),
				&csi.CreateVolumeRequest{
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
					Secrets:    sc.Secrets.CreateVolumeSecret,
					Parameters: sc.Config.TestVolumeParameters,
				},
			)
			Expect(err).NotTo(HaveOccurred())
			Expect(vol).NotTo(BeNil())
			Expect(vol.GetVolume()).NotTo(BeNil())
			Expect(vol.GetVolume().GetVolumeId()).NotTo(BeEmpty())
			cl.RegisterVolume(name, VolumeInfo{VolumeID: vol.GetVolume().GetVolumeId()})

			By("getting a node id")
			nid, err := n.NodeGetInfo(
				context.Background(),
				&csi.NodeGetInfoRequest{})
			Expect(err).NotTo(HaveOccurred())
			Expect(nid).NotTo(BeNil())
			Expect(nid.GetNodeId()).NotTo(BeEmpty())

			// ControllerPublishVolume
			By("calling controllerpublish on that volume")

			conpubvol, err := c.ControllerPublishVolume(
				context.Background(),
				&csi.ControllerPublishVolumeRequest{
					VolumeId: vol.GetVolume().GetVolumeId(),
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
					Secrets:  sc.Secrets.ControllerPublishVolumeSecret,
				},
			)
			Expect(err).NotTo(HaveOccurred())
			cl.RegisterVolume(name, VolumeInfo{VolumeID: vol.GetVolume().GetVolumeId(), NodeID: nid.GetNodeId()})
			Expect(conpubvol).NotTo(BeNil())

			By("cleaning up unpublishing the volume")

			conunpubvol, err := c.ControllerUnpublishVolume(
				context.Background(),
				&csi.ControllerUnpublishVolumeRequest{
					VolumeId: vol.GetVolume().GetVolumeId(),
					// NodeID is optional in ControllerUnpublishVolume
					NodeId:  nid.GetNodeId(),
					Secrets: sc.Secrets.ControllerUnpublishVolumeSecret,
				},
			)
			Expect(err).NotTo(HaveOccurred())
			Expect(conunpubvol).NotTo(BeNil())

			By("cleaning up deleting the volume")

			_, err = c.DeleteVolume(
				context.Background(),
				&csi.DeleteVolumeRequest{
					VolumeId: vol.GetVolume().GetVolumeId(),
					Secrets:  sc.Secrets.DeleteVolumeSecret,
				},
			)
			Expect(err).NotTo(HaveOccurred())
			cl.UnregisterVolume(name)
		})

		It("should fail when the volume does not exist", func() {

			By("calling controller publish on a non-existent volume")

			conpubvol, err := c.ControllerPublishVolume(
				context.Background(),
				&csi.ControllerPublishVolumeRequest{
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
					Secrets:  sc.Secrets.ControllerPublishVolumeSecret,
				},
			)
			Expect(err).To(HaveOccurred())
			Expect(conpubvol).To(BeNil())

			serverError, ok := status.FromError(err)
			Expect(ok).To(BeTrue())
			Expect(serverError.Code()).To(Equal(codes.NotFound))
		})

		It("should fail when the node does not exist", func() {

			// Create Volume First
			By("creating a single node writer volume")
			name := uniqueString("sanity-controller-wrong-node")

			vol, err := c.CreateVolume(
				context.Background(),
				&csi.CreateVolumeRequest{
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
					Secrets:    sc.Secrets.CreateVolumeSecret,
					Parameters: sc.Config.TestVolumeParameters,
				},
			)
			Expect(err).NotTo(HaveOccurred())
			Expect(vol).NotTo(BeNil())
			Expect(vol.GetVolume()).NotTo(BeNil())
			Expect(vol.GetVolume().GetVolumeId()).NotTo(BeEmpty())
			cl.RegisterVolume(name, VolumeInfo{VolumeID: vol.GetVolume().GetVolumeId()})

			// ControllerPublishVolume
			By("calling controllerpublish on that volume")

			conpubvol, err := c.ControllerPublishVolume(
				context.Background(),
				&csi.ControllerPublishVolumeRequest{
					VolumeId: vol.GetVolume().GetVolumeId(),
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
					Secrets:  sc.Secrets.ControllerPublishVolumeSecret,
				},
			)
			Expect(err).To(HaveOccurred())
			Expect(conpubvol).To(BeNil())

			serverError, ok := status.FromError(err)
			Expect(ok).To(BeTrue())
			Expect(serverError.Code()).To(Equal(codes.NotFound))

			By("cleaning up deleting the volume")

			_, err = c.DeleteVolume(
				context.Background(),
				&csi.DeleteVolumeRequest{
					VolumeId: vol.GetVolume().GetVolumeId(),
					Secrets:  sc.Secrets.DeleteVolumeSecret,
				},
			)
			Expect(err).NotTo(HaveOccurred())
			cl.UnregisterVolume(name)
		})

		It("should fail when the volume is already published but is incompatible", func() {

			// Create Volume First
			By("creating a single node writer volume")
			name := uniqueString("sanity-controller-published-incompatible")

			vol, err := c.CreateVolume(
				context.Background(),
				&csi.CreateVolumeRequest{
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
					Secrets:    sc.Secrets.CreateVolumeSecret,
					Parameters: sc.Config.TestVolumeParameters,
				},
			)
			Expect(err).NotTo(HaveOccurred())
			Expect(vol).NotTo(BeNil())
			Expect(vol.GetVolume()).NotTo(BeNil())
			Expect(vol.GetVolume().GetVolumeId()).NotTo(BeEmpty())
			cl.RegisterVolume(name, VolumeInfo{VolumeID: vol.GetVolume().GetVolumeId()})

			By("getting a node id")
			nid, err := n.NodeGetInfo(
				context.Background(),
				&csi.NodeGetInfoRequest{})
			Expect(err).NotTo(HaveOccurred())
			Expect(nid).NotTo(BeNil())
			Expect(nid.GetNodeId()).NotTo(BeEmpty())

			// ControllerPublishVolume
			By("calling controllerpublish on that volume")

			pubReq := &csi.ControllerPublishVolumeRequest{
				VolumeId: vol.GetVolume().GetVolumeId(),
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
				Secrets:  sc.Secrets.ControllerPublishVolumeSecret,
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

			conunpubvol, err := c.ControllerUnpublishVolume(
				context.Background(),
				&csi.ControllerUnpublishVolumeRequest{
					VolumeId: vol.GetVolume().GetVolumeId(),
					// NodeID is optional in ControllerUnpublishVolume
					NodeId:  nid.GetNodeId(),
					Secrets: sc.Secrets.ControllerUnpublishVolumeSecret,
				},
			)

			Expect(err).NotTo(HaveOccurred())
			Expect(conunpubvol).NotTo(BeNil())

			By("cleaning up deleting the volume")

			_, err = c.DeleteVolume(
				context.Background(),
				&csi.DeleteVolumeRequest{
					VolumeId: vol.GetVolume().GetVolumeId(),
					Secrets:  sc.Secrets.DeleteVolumeSecret,
				},
			)
			Expect(err).NotTo(HaveOccurred())
			cl.UnregisterVolume(name)
		})
	})

	Describe("ControllerUnpublishVolume", func() {
		BeforeEach(func() {
			if !isControllerCapabilitySupported(c, csi.ControllerServiceCapability_RPC_PUBLISH_UNPUBLISH_VOLUME) {
				Skip("ControllerUnpublishVolume not supported")
			}
		})

		It("should fail when no volume id is provided", func() {

			_, err := c.ControllerUnpublishVolume(
				context.Background(),
				&csi.ControllerUnpublishVolumeRequest{
					Secrets: sc.Secrets.ControllerUnpublishVolumeSecret,
				},
			)
			Expect(err).To(HaveOccurred())

			serverError, ok := status.FromError(err)
			Expect(ok).To(BeTrue())
			Expect(serverError.Code()).To(Equal(codes.InvalidArgument))
		})

		It("should return appropriate values (no optional values added)", func() {

			// Create Volume First
			By("creating a single node writer volume")
			name := uniqueString("sanity-controller-unpublish")

			vol, err := c.CreateVolume(
				context.Background(),
				&csi.CreateVolumeRequest{
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
					Secrets:    sc.Secrets.CreateVolumeSecret,
					Parameters: sc.Config.TestVolumeParameters,
				},
			)
			Expect(err).NotTo(HaveOccurred())
			Expect(vol).NotTo(BeNil())
			Expect(vol.GetVolume()).NotTo(BeNil())
			Expect(vol.GetVolume().GetVolumeId()).NotTo(BeEmpty())
			cl.RegisterVolume(name, VolumeInfo{VolumeID: vol.GetVolume().GetVolumeId()})

			By("getting a node id")
			nid, err := n.NodeGetInfo(
				context.Background(),
				&csi.NodeGetInfoRequest{})
			Expect(err).NotTo(HaveOccurred())
			Expect(nid).NotTo(BeNil())
			Expect(nid.GetNodeId()).NotTo(BeEmpty())

			// ControllerPublishVolume
			By("calling controllerpublish on that volume")

			conpubvol, err := c.ControllerPublishVolume(
				context.Background(),
				&csi.ControllerPublishVolumeRequest{
					VolumeId: vol.GetVolume().GetVolumeId(),
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
					Secrets:  sc.Secrets.ControllerPublishVolumeSecret,
				},
			)
			Expect(err).NotTo(HaveOccurred())
			cl.RegisterVolume(name, VolumeInfo{VolumeID: vol.GetVolume().GetVolumeId(), NodeID: nid.GetNodeId()})
			Expect(conpubvol).NotTo(BeNil())

			// ControllerUnpublishVolume
			By("calling controllerunpublish on that volume")

			conunpubvol, err := c.ControllerUnpublishVolume(
				context.Background(),
				&csi.ControllerUnpublishVolumeRequest{
					VolumeId: vol.GetVolume().GetVolumeId(),
					// NodeID is optional in ControllerUnpublishVolume
					NodeId:  nid.GetNodeId(),
					Secrets: sc.Secrets.ControllerUnpublishVolumeSecret,
				},
			)
			Expect(err).NotTo(HaveOccurred())
			Expect(conunpubvol).NotTo(BeNil())

			By("cleaning up deleting the volume")

			_, err = c.DeleteVolume(
				context.Background(),
				&csi.DeleteVolumeRequest{
					VolumeId: vol.GetVolume().GetVolumeId(),
					Secrets:  sc.Secrets.DeleteVolumeSecret,
				},
			)
			Expect(err).NotTo(HaveOccurred())
			cl.UnregisterVolume(name)
		})
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
		snapshotReq := MakeCreateSnapshotReq(sc, "listSnapshots-snapshot-1", volume.GetVolume().GetVolumeId(), nil)
		snapshot, err := c.CreateSnapshot(context.Background(), snapshotReq)
		Expect(err).NotTo(HaveOccurred())

		snapshots, err := c.ListSnapshots(
			context.Background(),
			&csi.ListSnapshotsRequest{SnapshotId: snapshot.GetSnapshot().GetSnapshotId()})
		Expect(err).NotTo(HaveOccurred())
		Expect(snapshots).NotTo(BeNil())
		Expect(len(snapshots.GetEntries())).To(BeNumerically("==", 1))
		verifySnapshotInfo(snapshots.GetEntries()[0].GetSnapshot())
		Expect(snapshots.GetEntries()[0].GetSnapshot().GetSnapshotId()).To(Equal(snapshot.GetSnapshot().GetSnapshotId()))

		By("cleaning up deleting the snapshot")
		delSnapReq := MakeDeleteSnapshotReq(sc, snapshot.GetSnapshot().GetSnapshotId())
		_, err = c.DeleteSnapshot(context.Background(), delSnapReq)
		Expect(err).NotTo(HaveOccurred())

		By("cleaning up deleting the volume")
		delVolReq := MakeDeleteVolumeReq(sc, volume.GetVolume().GetVolumeId())
		_, err = c.DeleteVolume(context.Background(), delVolReq)
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
		snapshotReq := MakeCreateSnapshotReq(sc, "listSnapshots-snapshot-2", volume.GetVolume().GetVolumeId(), nil)
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
		delSnapReq := MakeDeleteSnapshotReq(sc, snapshot.GetSnapshot().GetSnapshotId())
		_, err = c.DeleteSnapshot(context.Background(), delSnapReq)
		Expect(err).NotTo(HaveOccurred())

		By("cleaning up deleting the volume")
		delVolReq := MakeDeleteVolumeReq(sc, volume.GetVolume().GetVolumeId())
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
		snapReq := MakeCreateSnapshotReq(sc, "listSnapshots-snapshot-3", volume.GetVolume().GetVolumeId(), nil)
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
		delSnapReq := MakeDeleteSnapshotReq(sc, snapshot.GetSnapshot().GetSnapshotId())
		_, err = c.DeleteSnapshot(context.Background(), delSnapReq)
		Expect(err).NotTo(HaveOccurred())

		By("cleaning up deleting the volume")
		delVolReq := MakeDeleteVolumeReq(sc, volume.GetVolume().GetVolumeId())
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

				snapReq := MakeCreateSnapshotReq(sc, "snapshot"+strconv.Itoa(i), volume.GetVolume().GetVolumeId(), nil)
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
				delSnapReq := MakeDeleteSnapshotReq(sc, snap.GetSnapshotId())
				_, err = c.DeleteSnapshot(context.Background(), delSnapReq)
				Expect(err).NotTo(HaveOccurred())
			}

			By("cleaning up deleting the volumes")

			for _, vol := range createVols {
				delVolReq := MakeDeleteVolumeReq(sc, vol.GetVolumeId())
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
			req.Secrets = sc.Secrets.DeleteSnapshotSecret
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
		snapshotReq := MakeCreateSnapshotReq(sc, "DeleteSnapshot-snapshot-1", volume.GetVolume().GetVolumeId(), nil)
		snapshot, err := c.CreateSnapshot(context.Background(), snapshotReq)
		Expect(err).NotTo(HaveOccurred())
		Expect(snapshot).NotTo(BeNil())
		verifySnapshotInfo(snapshot.GetSnapshot())

		By("cleaning up deleting the snapshot")
		delSnapReq := MakeDeleteSnapshotReq(sc, snapshot.GetSnapshot().GetSnapshotId())
		_, err = c.DeleteSnapshot(context.Background(), delSnapReq)
		Expect(err).NotTo(HaveOccurred())

		By("cleaning up deleting the volume")
		delVolReq := MakeDeleteVolumeReq(sc, volume.GetVolume().GetVolumeId())
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
			req.Secrets = sc.Secrets.CreateSnapshotSecret
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
			req.Secrets = sc.Secrets.CreateSnapshotSecret
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
		snapReq1 := MakeCreateSnapshotReq(sc, "CreateSnapshot-snapshot-1", volume.GetVolume().GetVolumeId(), nil)
		snap1, err := c.CreateSnapshot(context.Background(), snapReq1)
		Expect(err).NotTo(HaveOccurred())
		Expect(snap1).NotTo(BeNil())
		verifySnapshotInfo(snap1.GetSnapshot())

		snap2, err := c.CreateSnapshot(context.Background(), snapReq1)
		Expect(err).NotTo(HaveOccurred())
		Expect(snap2).NotTo(BeNil())
		verifySnapshotInfo(snap2.GetSnapshot())

		By("cleaning up deleting the snapshot")
		delSnapReq := MakeDeleteSnapshotReq(sc, snap1.GetSnapshot().GetSnapshotId())
		_, err = c.DeleteSnapshot(context.Background(), delSnapReq)
		Expect(err).NotTo(HaveOccurred())

		By("cleaning up deleting the volume")
		delVolReq := MakeDeleteVolumeReq(sc, volume.GetVolume().GetVolumeId())
		_, err = c.DeleteVolume(context.Background(), delVolReq)
		Expect(err).NotTo(HaveOccurred())
	})

	It("should fail when requesting to create a snapshot with already existing name and different SourceVolumeId.", func() {

		By("creating a volume")
		volume, err := c.CreateVolume(context.Background(), MakeCreateVolumeReq(sc, "CreateSnapshot-volume-2"))
		Expect(err).ToNot(HaveOccurred())

		By("creating a snapshot with the created volume source id")
		req1 := MakeCreateSnapshotReq(sc, "CreateSnapshot-snapshot-2", volume.GetVolume().GetVolumeId(), nil)
		snap1, err := c.CreateSnapshot(context.Background(), req1)
		Expect(err).NotTo(HaveOccurred())
		Expect(snap1).NotTo(BeNil())
		verifySnapshotInfo(snap1.GetSnapshot())

		volume2, err := c.CreateVolume(context.Background(), MakeCreateVolumeReq(sc, "CreateSnapshot-volume-3"))
		Expect(err).ToNot(HaveOccurred())

		By("creating a snapshot with the same name but different volume source id")
		req2 := MakeCreateSnapshotReq(sc, "CreateSnapshot-snapshot-2", volume2.GetVolume().GetVolumeId(), nil)
		_, err = c.CreateSnapshot(context.Background(), req2)
		Expect(err).To(HaveOccurred())
		serverError, ok := status.FromError(err)
		Expect(ok).To(BeTrue())
		Expect(serverError.Code()).To(Equal(codes.AlreadyExists))

		By("cleaning up deleting the snapshot")
		delSnapReq := MakeDeleteSnapshotReq(sc, snap1.GetSnapshot().GetSnapshotId())
		_, err = c.DeleteSnapshot(context.Background(), delSnapReq)
		Expect(err).NotTo(HaveOccurred())

		By("cleaning up deleting the volume")
		delVolReq := MakeDeleteVolumeReq(sc, volume.GetVolume().GetVolumeId())
		_, err = c.DeleteVolume(context.Background(), delVolReq)
		Expect(err).NotTo(HaveOccurred())
	})

	It("should not fail when creating snapshot with maximum-length name", func() {

		By("creating a volume")
		volReq := MakeCreateVolumeReq(sc, "CreateSnapshot-volume-3")
		volume, err := c.CreateVolume(context.Background(), volReq)
		Expect(err).NotTo(HaveOccurred())

		nameBytes := make([]byte, MaxNameLength)
		for i := 0; i < MaxNameLength; i++ {
			nameBytes[i] = 'a'
		}
		name := string(nameBytes)

		By("creating a snapshot")
		snapReq1 := MakeCreateSnapshotReq(sc, name, volume.GetVolume().GetVolumeId(), nil)
		snap1, err := c.CreateSnapshot(context.Background(), snapReq1)
		Expect(err).NotTo(HaveOccurred())
		Expect(snap1).NotTo(BeNil())
		verifySnapshotInfo(snap1.GetSnapshot())

		snap2, err := c.CreateSnapshot(context.Background(), snapReq1)
		Expect(err).NotTo(HaveOccurred())
		Expect(snap2).NotTo(BeNil())
		verifySnapshotInfo(snap2.GetSnapshot())

		By("cleaning up deleting the snapshot")
		delSnapReq := MakeDeleteSnapshotReq(sc, snap1.GetSnapshot().GetSnapshotId())
		_, err = c.DeleteSnapshot(context.Background(), delSnapReq)
		Expect(err).NotTo(HaveOccurred())

		By("cleaning up deleting the volume")
		delVolReq := MakeDeleteVolumeReq(sc, volume.GetVolume().GetVolumeId())
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
		Parameters: sc.Config.TestVolumeParameters,
	}

	if sc.Secrets != nil {
		req.Secrets = sc.Secrets.CreateVolumeSecret
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
		req.Secrets = sc.Secrets.CreateSnapshotSecret
	}

	return req
}

func MakeDeleteSnapshotReq(sc *SanityContext, id string) *csi.DeleteSnapshotRequest {
	delSnapReq := &csi.DeleteSnapshotRequest{
		SnapshotId: id,
	}

	if sc.Secrets != nil {
		delSnapReq.Secrets = sc.Secrets.DeleteSnapshotSecret
	}

	return delSnapReq
}

func MakeDeleteVolumeReq(sc *SanityContext, id string) *csi.DeleteVolumeRequest {
	delVolReq := &csi.DeleteVolumeRequest{
		VolumeId: id,
	}

	if sc.Secrets != nil {
		delVolReq.Secrets = sc.Secrets.DeleteVolumeSecret
	}

	return delVolReq
}
