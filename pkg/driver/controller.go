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

package driver

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws/arn"
	"github.com/awslabs/volume-modifier-for-k8s/pkg/rpc"
	csi "github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/kubernetes-sigs/aws-ebs-csi-driver/pkg/cloud"
	"github.com/kubernetes-sigs/aws-ebs-csi-driver/pkg/coalescer"
	"github.com/kubernetes-sigs/aws-ebs-csi-driver/pkg/driver/internal"
	"github.com/kubernetes-sigs/aws-ebs-csi-driver/pkg/util"
	"github.com/kubernetes-sigs/aws-ebs-csi-driver/pkg/util/template"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
	"k8s.io/klog/v2"
)

// Supported access modes
const (
	SingleNodeWriter     = csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER
	MultiNodeMultiWriter = csi.VolumeCapability_AccessMode_MULTI_NODE_MULTI_WRITER
)

var (
	// controllerCaps represents the capability of controller service
	controllerCaps = []csi.ControllerServiceCapability_RPC_Type{
		csi.ControllerServiceCapability_RPC_CREATE_DELETE_VOLUME,
		csi.ControllerServiceCapability_RPC_PUBLISH_UNPUBLISH_VOLUME,
		csi.ControllerServiceCapability_RPC_CREATE_DELETE_SNAPSHOT,
		csi.ControllerServiceCapability_RPC_LIST_SNAPSHOTS,
		csi.ControllerServiceCapability_RPC_EXPAND_VOLUME,
		csi.ControllerServiceCapability_RPC_MODIFY_VOLUME,
	}
)

const isManagedByDriver = "true"

// ControllerService represents the controller service of CSI driver
type ControllerService struct {
	cloud                 cloud.Cloud
	inFlight              *internal.InFlight
	options               *Options
	modifyVolumeCoalescer coalescer.Coalescer[modifyVolumeRequest, int32]
	rpc.UnimplementedModifyServer
	csi.UnimplementedControllerServer
}

// NewControllerService creates a new controller service
func NewControllerService(c cloud.Cloud, o *Options) *ControllerService {
	return &ControllerService{
		cloud:                 c,
		options:               o,
		inFlight:              internal.NewInFlight(),
		modifyVolumeCoalescer: newModifyVolumeCoalescer(c, o),
	}
}

func (d *ControllerService) CreateVolume(ctx context.Context, req *csi.CreateVolumeRequest) (*csi.CreateVolumeResponse, error) {
	klog.V(4).InfoS("CreateVolume: called", "args", util.SanitizeRequest(req))
	if err := validateCreateVolumeRequest(req); err != nil {
		return nil, err
	}
	volSizeBytes, err := getVolSizeBytes(req)
	if err != nil {
		return nil, err
	}
	volName := req.GetName()
	volCap := req.GetVolumeCapabilities()

	multiAttach := false
	for _, c := range volCap {
		if c.GetAccessMode().GetMode() == MultiNodeMultiWriter && isBlock(c) {
			klog.V(4).InfoS("CreateVolume: multi-attach is enabled", "volumeID", volName)
			multiAttach = true
		}
	}

	// check if a request is already in-flight
	if ok := d.inFlight.Insert(volName); !ok {
		msg := fmt.Sprintf("Create volume request for %s is already in progress", volName)
		return nil, status.Error(codes.Aborted, msg)
	}
	defer d.inFlight.Delete(volName)

	var (
		volumeType             string
		iopsPerGB              int32
		allowIOPSPerGBIncrease bool
		iops                   int32
		throughput             int32
		isEncrypted            bool
		blockExpress           bool
		kmsKeyID               string
		scTags                 []string
		volumeTags             = map[string]string{
			cloud.VolumeNameTagKey:   volName,
			cloud.AwsEbsDriverTagKey: isManagedByDriver,
		}
		blockSize       string
		inodeSize       string
		bytesPerInode   string
		numberOfInodes  string
		ext4BigAlloc    bool
		ext4ClusterSize string
	)

	tProps := new(template.PVProps)

	for key, value := range req.GetParameters() {
		switch strings.ToLower(key) {
		case "fstype":
			klog.InfoS("\"fstype\" is deprecated, please use \"csi.storage.k8s.io/fstype\" instead")
		case VolumeTypeKey:
			volumeType = value
		case IopsPerGBKey:
			parseIopsPerGBKey, parseIopsPerGBKeyErr := strconv.ParseInt(value, 10, 32)
			if parseIopsPerGBKeyErr != nil {
				return nil, status.Errorf(codes.InvalidArgument, "Could not parse invalid iopsPerGB: %v", err)
			}
			iopsPerGB = int32(parseIopsPerGBKey)
		case AllowAutoIOPSPerGBIncreaseKey:
			allowIOPSPerGBIncrease = value == "true"
		case IopsKey:
			parseIopsKey, parseIopsKeyErr := strconv.ParseInt(value, 10, 32)
			if parseIopsKeyErr != nil {
				return nil, status.Errorf(codes.InvalidArgument, "Could not parse invalid iops: %v", err)
			}
			iops = int32(parseIopsKey)
		case ThroughputKey:
			parseThroughput, parseThroughputErr := strconv.ParseInt(value, 10, 32)
			if parseThroughputErr != nil {
				return nil, status.Errorf(codes.InvalidArgument, "Could not parse invalid throughput: %v", err)
			}
			throughput = int32(parseThroughput)
		case EncryptedKey:
			if value == "true" {
				isEncrypted = true
			}
		case KmsKeyIDKey:
			kmsKeyID = value
		case PVCNameKey:
			volumeTags[PVCNameTag] = value
			tProps.PVCName = value
		case PVCNamespaceKey:
			volumeTags[PVCNamespaceTag] = value
			tProps.PVCNamespace = value
		case PVNameKey:
			volumeTags[PVNameTag] = value
			tProps.PVName = value
		case BlockExpressKey:
			if value == "true" {
				blockExpress = true
			}
		case BlockSizeKey:
			if isAlphanumeric := util.StringIsAlphanumeric(value); !isAlphanumeric {
				return nil, status.Errorf(codes.InvalidArgument, "Could not parse blockSize (%s): %v", value, err)
			}
			blockSize = value
		case InodeSizeKey:
			if isAlphanumeric := util.StringIsAlphanumeric(value); !isAlphanumeric {
				return nil, status.Errorf(codes.InvalidArgument, "Could not parse inodeSize (%s): %v", value, err)
			}
			inodeSize = value
		case BytesPerInodeKey:
			if isAlphanumeric := util.StringIsAlphanumeric(value); !isAlphanumeric {
				return nil, status.Errorf(codes.InvalidArgument, "Could not parse bytesPerInode (%s): %v", value, err)
			}
			bytesPerInode = value
		case NumberOfInodesKey:
			if isAlphanumeric := util.StringIsAlphanumeric(value); !isAlphanumeric {
				return nil, status.Errorf(codes.InvalidArgument, "Could not parse numberOfInodes (%s): %v", value, err)
			}
			numberOfInodes = value
		case Ext4BigAllocKey:
			if value == "true" {
				ext4BigAlloc = true
			}
		case Ext4ClusterSizeKey:
			if isAlphanumeric := util.StringIsAlphanumeric(value); !isAlphanumeric {
				return nil, status.Errorf(codes.InvalidArgument, "Could not parse ext4ClusterSize (%s): %v", value, err)
			}
			ext4ClusterSize = value
		default:
			if strings.HasPrefix(key, TagKeyPrefix) {
				scTags = append(scTags, value)
			} else {
				return nil, status.Errorf(codes.InvalidArgument, "Invalid parameter key %s for CreateVolume", key)
			}
		}
	}

	modifyOptions, err := parseModifyVolumeParameters(req.GetMutableParameters())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "Invalid mutable parameter: %v", err)
	}

	// "Values specified in mutable_parameters MUST take precedence over the values from parameters."
	// https://github.com/container-storage-interface/spec/blob/master/spec.md#createvolume
	if modifyOptions.modifyDiskOptions.VolumeType != "" {
		volumeType = modifyOptions.modifyDiskOptions.VolumeType
	}
	if modifyOptions.modifyDiskOptions.IOPS != 0 {
		iops = modifyOptions.modifyDiskOptions.IOPS
	}
	if modifyOptions.modifyDiskOptions.Throughput != 0 {
		throughput = modifyOptions.modifyDiskOptions.Throughput
	}

	responseCtx := map[string]string{}

	if len(blockSize) > 0 {
		responseCtx[BlockSizeKey] = blockSize
		if err = validateFormattingOption(volCap, BlockSizeKey, FileSystemConfigs); err != nil {
			return nil, err
		}
	}
	if len(inodeSize) > 0 {
		responseCtx[InodeSizeKey] = inodeSize
		if err = validateFormattingOption(volCap, InodeSizeKey, FileSystemConfigs); err != nil {
			return nil, err
		}
	}
	if len(bytesPerInode) > 0 {
		responseCtx[BytesPerInodeKey] = bytesPerInode
		if err = validateFormattingOption(volCap, BytesPerInodeKey, FileSystemConfigs); err != nil {
			return nil, err
		}
	}
	if len(numberOfInodes) > 0 {
		responseCtx[NumberOfInodesKey] = numberOfInodes
		if err = validateFormattingOption(volCap, NumberOfInodesKey, FileSystemConfigs); err != nil {
			return nil, err
		}
	}
	if ext4BigAlloc {
		responseCtx[Ext4BigAllocKey] = "true"
		if err = validateFormattingOption(volCap, Ext4BigAllocKey, FileSystemConfigs); err != nil {
			return nil, err
		}
	}
	if len(ext4ClusterSize) > 0 {
		responseCtx[Ext4ClusterSizeKey] = ext4ClusterSize
		if err = validateFormattingOption(volCap, Ext4ClusterSizeKey, FileSystemConfigs); err != nil {
			return nil, err
		}
	}

	if !ext4BigAlloc && len(ext4ClusterSize) > 0 {
		return nil, status.Errorf(codes.InvalidArgument, "Cannot set ext4BigAllocClusterSize when ext4BigAlloc is false")
	}

	if blockExpress && volumeType != cloud.VolumeTypeIO2 {
		return nil, status.Errorf(codes.InvalidArgument, "Block Express is only supported on io2 volumes")
	}

	snapshotID := ""
	volumeSource := req.GetVolumeContentSource()
	if volumeSource != nil {
		if _, ok := volumeSource.GetType().(*csi.VolumeContentSource_Snapshot); !ok {
			return nil, status.Error(codes.InvalidArgument, "Unsupported volumeContentSource type")
		}
		sourceSnapshot := volumeSource.GetSnapshot()
		if sourceSnapshot == nil {
			return nil, status.Error(codes.InvalidArgument, "Error retrieving snapshot from the volumeContentSource")
		}
		snapshotID = sourceSnapshot.GetSnapshotId()
	}

	// create a new volume
	zone := pickAvailabilityZone(req.GetAccessibilityRequirements())
	outpostArn := getOutpostArn(req.GetAccessibilityRequirements())

	// fill volume tags
	if d.options.KubernetesClusterID != "" {
		resourceLifecycleTag := ResourceLifecycleTagPrefix + d.options.KubernetesClusterID
		volumeTags[resourceLifecycleTag] = ResourceLifecycleOwned
		volumeTags[NameTag] = d.options.KubernetesClusterID + "-dynamic-" + volName
		volumeTags[KubernetesClusterTag] = d.options.KubernetesClusterID
	}
	for k, v := range d.options.ExtraTags {
		volumeTags[k] = v
	}

	addTags, err := template.Evaluate(scTags, tProps, d.options.WarnOnInvalidTag)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "Error interpolating the tag value: %v", err)
	}

	if err = validateExtraTags(addTags, d.options.WarnOnInvalidTag); err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "Invalid tag value: %v", err)
	}

	for k, v := range addTags {
		volumeTags[k] = v
	}

	opts := &cloud.DiskOptions{
		CapacityBytes:          volSizeBytes,
		Tags:                   volumeTags,
		VolumeType:             volumeType,
		IOPSPerGB:              iopsPerGB,
		AllowIOPSPerGBIncrease: allowIOPSPerGBIncrease,
		IOPS:                   iops,
		Throughput:             throughput,
		AvailabilityZone:       zone,
		OutpostArn:             outpostArn,
		Encrypted:              isEncrypted,
		BlockExpress:           blockExpress,
		KmsKeyID:               kmsKeyID,
		SnapshotID:             snapshotID,
		MultiAttachEnabled:     multiAttach,
	}

	disk, err := d.cloud.CreateDisk(ctx, volName, opts)
	if err != nil {
		var errCode codes.Code
		switch {
		case errors.Is(err, cloud.ErrNotFound):
			errCode = codes.NotFound
		case errors.Is(err, cloud.ErrIdempotentParameterMismatch), errors.Is(err, cloud.ErrAlreadyExists):
			errCode = codes.AlreadyExists
		default:
			errCode = codes.Internal
		}
		return nil, status.Errorf(errCode, "Could not create volume %q: %v", volName, err)
	}
	return newCreateVolumeResponse(disk, responseCtx), nil
}

func validateCreateVolumeRequest(req *csi.CreateVolumeRequest) error {
	volName := req.GetName()
	if len(volName) == 0 {
		return status.Error(codes.InvalidArgument, "Volume name not provided")
	}

	volCaps := req.GetVolumeCapabilities()
	if len(volCaps) == 0 {
		return status.Error(codes.InvalidArgument, "Volume capabilities not provided")
	}

	if !isValidVolumeCapabilities(volCaps) {
		return status.Error(codes.InvalidArgument, "Volume capabilities not supported")
	}
	return nil
}

func (d *ControllerService) DeleteVolume(ctx context.Context, req *csi.DeleteVolumeRequest) (*csi.DeleteVolumeResponse, error) {
	klog.V(4).InfoS("DeleteVolume: called", "args", util.SanitizeRequest(req))
	if err := validateDeleteVolumeRequest(req); err != nil {
		return nil, err
	}

	volumeID := req.GetVolumeId()
	// check if a request is already in-flight
	if ok := d.inFlight.Insert(volumeID); !ok {
		msg := fmt.Sprintf(internal.VolumeOperationAlreadyExistsErrorMsg, volumeID)
		return nil, status.Error(codes.Aborted, msg)
	}
	defer d.inFlight.Delete(volumeID)

	if _, err := d.cloud.DeleteDisk(ctx, volumeID); err != nil {
		if errors.Is(err, cloud.ErrNotFound) {
			klog.V(4).InfoS("DeleteVolume: volume not found, returning with success")
			return &csi.DeleteVolumeResponse{}, nil
		}
		return nil, status.Errorf(codes.Internal, "Could not delete volume ID %q: %v", volumeID, err)
	}

	return &csi.DeleteVolumeResponse{}, nil
}

func validateDeleteVolumeRequest(req *csi.DeleteVolumeRequest) error {
	if len(req.GetVolumeId()) == 0 {
		return status.Error(codes.InvalidArgument, "Volume ID not provided")
	}
	return nil
}

func (d *ControllerService) ControllerPublishVolume(ctx context.Context, req *csi.ControllerPublishVolumeRequest) (*csi.ControllerPublishVolumeResponse, error) {
	klog.V(4).InfoS("ControllerPublishVolume: called", "args", util.SanitizeRequest(req))
	if err := validateControllerPublishVolumeRequest(req); err != nil {
		return nil, err
	}

	volumeID := req.GetVolumeId()
	nodeID := req.GetNodeId()

	if !d.inFlight.Insert(volumeID + nodeID) {
		return nil, status.Error(codes.Aborted, fmt.Sprintf(internal.VolumeOperationAlreadyExistsErrorMsg, volumeID))
	}
	defer d.inFlight.Delete(volumeID + nodeID)

	klog.V(2).InfoS("ControllerPublishVolume: attaching", "volumeID", volumeID, "nodeID", nodeID)
	devicePath, err := d.cloud.AttachDisk(ctx, volumeID, nodeID)
	if err != nil {
		if errors.Is(err, cloud.ErrNotFound) {
			klog.InfoS("ControllerPublishVolume: volume not found", "volumeID", volumeID, "nodeID", nodeID)
			return nil, status.Errorf(codes.NotFound, "Volume %q not found", volumeID)
		}
		return nil, status.Errorf(codes.Internal, "Could not attach volume %q to node %q: %v", volumeID, nodeID, err)
	}
	klog.InfoS("ControllerPublishVolume: attached", "volumeID", volumeID, "nodeID", nodeID, "devicePath", devicePath)

	pvInfo := map[string]string{DevicePathKey: devicePath}
	return &csi.ControllerPublishVolumeResponse{PublishContext: pvInfo}, nil
}

func validateControllerPublishVolumeRequest(req *csi.ControllerPublishVolumeRequest) error {
	if len(req.GetVolumeId()) == 0 {
		return status.Error(codes.InvalidArgument, "Volume ID not provided")
	}

	if len(req.GetNodeId()) == 0 {
		return status.Error(codes.InvalidArgument, "Node ID not provided")
	}

	volCap := req.GetVolumeCapability()
	if volCap == nil {
		return status.Error(codes.InvalidArgument, "Volume capability not provided")
	}

	if !isValidCapability(volCap) {
		return status.Error(codes.InvalidArgument, "Volume capability not supported")
	}
	return nil
}

func (d *ControllerService) ControllerUnpublishVolume(ctx context.Context, req *csi.ControllerUnpublishVolumeRequest) (*csi.ControllerUnpublishVolumeResponse, error) {
	klog.V(4).InfoS("ControllerUnpublishVolume: called", "args", util.SanitizeRequest(req))

	if err := validateControllerUnpublishVolumeRequest(req); err != nil {
		return nil, err
	}

	volumeID := req.GetVolumeId()
	nodeID := req.GetNodeId()

	if !d.inFlight.Insert(volumeID + nodeID) {
		return nil, status.Error(codes.Aborted, fmt.Sprintf(internal.VolumeOperationAlreadyExistsErrorMsg, volumeID))
	}
	defer d.inFlight.Delete(volumeID + nodeID)

	klog.V(2).InfoS("ControllerUnpublishVolume: detaching", "volumeID", volumeID, "nodeID", nodeID)
	if err := d.cloud.DetachDisk(ctx, volumeID, nodeID); err != nil {
		if errors.Is(err, cloud.ErrNotFound) {
			klog.InfoS("ControllerUnpublishVolume: attachment not found", "volumeID", volumeID, "nodeID", nodeID)
			return &csi.ControllerUnpublishVolumeResponse{}, nil
		}
		return nil, status.Errorf(codes.Internal, "Could not detach volume %q from node %q: %v", volumeID, nodeID, err)
	}
	klog.InfoS("ControllerUnpublishVolume: detached", "volumeID", volumeID, "nodeID", nodeID)

	return &csi.ControllerUnpublishVolumeResponse{}, nil
}

func validateControllerUnpublishVolumeRequest(req *csi.ControllerUnpublishVolumeRequest) error {
	if len(req.GetVolumeId()) == 0 {
		return status.Error(codes.InvalidArgument, "Volume ID not provided")
	}

	if len(req.GetNodeId()) == 0 {
		return status.Error(codes.InvalidArgument, "Node ID not provided")
	}

	return nil
}

func (d *ControllerService) ControllerGetCapabilities(ctx context.Context, req *csi.ControllerGetCapabilitiesRequest) (*csi.ControllerGetCapabilitiesResponse, error) {
	klog.V(4).InfoS("ControllerGetCapabilities: called", "args", req)
	var caps []*csi.ControllerServiceCapability
	for _, cap := range controllerCaps {
		c := &csi.ControllerServiceCapability{
			Type: &csi.ControllerServiceCapability_Rpc{
				Rpc: &csi.ControllerServiceCapability_RPC{
					Type: cap,
				},
			},
		}
		caps = append(caps, c)
	}
	return &csi.ControllerGetCapabilitiesResponse{Capabilities: caps}, nil
}

func (d *ControllerService) GetCapacity(ctx context.Context, req *csi.GetCapacityRequest) (*csi.GetCapacityResponse, error) {
	klog.V(4).InfoS("GetCapacity: called", "args", req)
	return nil, status.Error(codes.Unimplemented, "")
}

func (d *ControllerService) ListVolumes(ctx context.Context, req *csi.ListVolumesRequest) (*csi.ListVolumesResponse, error) {
	klog.V(4).InfoS("ListVolumes: called", "args", req)
	return nil, status.Error(codes.Unimplemented, "")
}

func (d *ControllerService) ValidateVolumeCapabilities(ctx context.Context, req *csi.ValidateVolumeCapabilitiesRequest) (*csi.ValidateVolumeCapabilitiesResponse, error) {
	klog.V(4).InfoS("ValidateVolumeCapabilities: called", "args", req)
	volumeID := req.GetVolumeId()
	if len(volumeID) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Volume ID not provided")
	}

	volCaps := req.GetVolumeCapabilities()
	if len(volCaps) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Volume capabilities not provided")
	}

	if _, err := d.cloud.GetDiskByID(ctx, volumeID); err != nil {
		if errors.Is(err, cloud.ErrNotFound) {
			return nil, status.Error(codes.NotFound, "Volume not found")
		}
		return nil, status.Errorf(codes.Internal, "Could not get volume with ID %q: %v", volumeID, err)
	}

	var confirmed *csi.ValidateVolumeCapabilitiesResponse_Confirmed
	if isValidVolumeCapabilities(volCaps) {
		confirmed = &csi.ValidateVolumeCapabilitiesResponse_Confirmed{VolumeCapabilities: volCaps}
	}
	return &csi.ValidateVolumeCapabilitiesResponse{
		Confirmed: confirmed,
	}, nil
}

func (d *ControllerService) ControllerExpandVolume(ctx context.Context, req *csi.ControllerExpandVolumeRequest) (*csi.ControllerExpandVolumeResponse, error) {
	klog.V(4).InfoS("ControllerExpandVolume: called", "args", util.SanitizeRequest(req))
	volumeID := req.GetVolumeId()
	if len(volumeID) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Volume ID not provided")
	}

	capRange := req.GetCapacityRange()
	if capRange == nil {
		return nil, status.Error(codes.InvalidArgument, "Capacity range not provided")
	}

	newSize := util.RoundUpBytes(capRange.GetRequiredBytes())
	maxVolSize := capRange.GetLimitBytes()
	if maxVolSize > 0 && maxVolSize < newSize {
		return nil, status.Error(codes.InvalidArgument, "After round-up, volume size exceeds the limit specified")
	}

	actualSizeGiB, err := d.modifyVolumeCoalescer.Coalesce(volumeID, modifyVolumeRequest{
		newSize: newSize,
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Could not resize volume %q: %v", volumeID, err)
	}

	nodeExpansionRequired := true
	// if this is a raw block device, no expansion should be necessary on the node
	cap := req.GetVolumeCapability()
	if cap != nil && cap.GetBlock() != nil {
		nodeExpansionRequired = false
	}

	return &csi.ControllerExpandVolumeResponse{
		CapacityBytes:         util.GiBToBytes(actualSizeGiB),
		NodeExpansionRequired: nodeExpansionRequired,
	}, nil
}

func (d *ControllerService) ControllerModifyVolume(ctx context.Context, req *csi.ControllerModifyVolumeRequest) (*csi.ControllerModifyVolumeResponse, error) {
	klog.V(4).InfoS("ControllerModifyVolume: called", "args", util.SanitizeRequest(req))

	volumeID := req.GetVolumeId()
	if len(volumeID) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Volume ID not provided")
	}

	options, err := parseModifyVolumeParameters(req.GetMutableParameters())
	if err != nil {
		return nil, err
	}

	_, err = d.modifyVolumeCoalescer.Coalesce(volumeID, modifyVolumeRequest{
		modifyDiskOptions: options.modifyDiskOptions,
		modifyTagsOptions: options.modifyTagsOptions,
	})
	if err != nil {
		return nil, err
	}

	return &csi.ControllerModifyVolumeResponse{}, nil
}

func (d *ControllerService) ControllerGetVolume(ctx context.Context, req *csi.ControllerGetVolumeRequest) (*csi.ControllerGetVolumeResponse, error) {
	klog.V(4).InfoS("ControllerGetVolume: called", "args", req)
	return nil, status.Error(codes.Unimplemented, "")
}

func isValidVolumeCapabilities(v []*csi.VolumeCapability) bool {
	for _, c := range v {
		if !isValidCapability(c) {
			return false
		}
	}
	return true
}

func isValidCapability(c *csi.VolumeCapability) bool {
	accessMode := c.GetAccessMode().GetMode()

	//nolint:exhaustive
	switch accessMode {
	case SingleNodeWriter:
		return true

	case MultiNodeMultiWriter:
		if isBlock(c) {
			return true
		} else {
			klog.InfoS("isValidCapability: access mode is only supported for block devices", "accessMode", accessMode)
			return false
		}

	default:
		klog.InfoS("isValidCapability: access mode is not supported", "accessMode", accessMode)
		return false
	}
}

func isBlock(cap *csi.VolumeCapability) bool {
	_, isBlock := cap.GetAccessType().(*csi.VolumeCapability_Block)
	return isBlock
}

func isValidVolumeContext(volContext map[string]string) bool {
	//There could be multiple volume attributes in the volumeContext map
	//Validate here case by case
	if partition, ok := volContext[VolumeAttributePartition]; ok {
		partitionInt, err := strconv.ParseInt(partition, 10, 64)
		if err != nil {
			klog.ErrorS(err, "failed to parse partition as int", "partition", partition)
			return false
		}
		if partitionInt < 0 {
			klog.ErrorS(err, "invalid partition config", "partition", partition)
			return false
		}
	}
	return true
}

func (d *ControllerService) CreateSnapshot(ctx context.Context, req *csi.CreateSnapshotRequest) (*csi.CreateSnapshotResponse, error) {
	klog.V(4).InfoS("CreateSnapshot: called", "args", util.SanitizeRequest(req))
	if err := validateCreateSnapshotRequest(req); err != nil {
		return nil, err
	}

	snapshotName := req.GetName()
	volumeID := req.GetSourceVolumeId()
	var outpostArn string

	// check if a request is already in-flight
	if ok := d.inFlight.Insert(snapshotName); !ok {
		msg := fmt.Sprintf(internal.VolumeOperationAlreadyExistsErrorMsg, snapshotName)
		return nil, status.Error(codes.Aborted, msg)
	}
	defer d.inFlight.Delete(snapshotName)

	snapshot, err := d.cloud.GetSnapshotByName(ctx, snapshotName)
	if err != nil && !errors.Is(err, cloud.ErrNotFound) {
		klog.ErrorS(err, "Error looking for the snapshot", "snapshotName", snapshotName)
		return nil, err
	}
	if snapshot != nil {
		if snapshot.SourceVolumeID != volumeID {
			return nil, status.Errorf(codes.AlreadyExists, "Snapshot %s already exists for different volume (%s)", snapshotName, snapshot.SourceVolumeID)
		}
		klog.V(4).InfoS("Snapshot of volume already exists; nothing to do", "snapshotName", snapshotName, "volumeId", volumeID)
		return newCreateSnapshotResponse(snapshot), nil
	}

	snapshotTags := map[string]string{
		cloud.SnapshotNameTagKey: snapshotName,
		cloud.AwsEbsDriverTagKey: isManagedByDriver,
	}

	var vscTags []string
	var fsrAvailabilityZones []string
	vsProps := new(template.VolumeSnapshotProps)
	for key, value := range req.GetParameters() {
		switch strings.ToLower(key) {
		case VolumeSnapshotNameKey:
			vsProps.VolumeSnapshotName = value
		case VolumeSnapshotNamespaceKey:
			vsProps.VolumeSnapshotNamespace = value
		case VolumeSnapshotContentNameKey:
			vsProps.VolumeSnapshotContentName = value
		case FastSnapshotRestoreAvailabilityZones:
			f := strings.ReplaceAll(value, " ", "")
			fsrAvailabilityZones = strings.Split(f, ",")
		case OutpostArnKey:
			outpostArn = value
		default:
			if strings.HasPrefix(key, TagKeyPrefix) {
				vscTags = append(vscTags, value)
			} else {
				return nil, status.Errorf(codes.InvalidArgument, "Invalid parameter key %s for CreateSnapshot", key)
			}
		}
	}

	addTags, err := template.Evaluate(vscTags, vsProps, d.options.WarnOnInvalidTag)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "Error interpolating the tag value: %v", err)
	}

	if err = validateExtraTags(addTags, d.options.WarnOnInvalidTag); err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "Invalid tag value: %v", err)
	}

	if d.options.KubernetesClusterID != "" {
		resourceLifecycleTag := ResourceLifecycleTagPrefix + d.options.KubernetesClusterID
		snapshotTags[resourceLifecycleTag] = ResourceLifecycleOwned
		snapshotTags[NameTag] = d.options.KubernetesClusterID + "-dynamic-" + snapshotName
	}
	for k, v := range d.options.ExtraTags {
		snapshotTags[k] = v
	}

	for k, v := range addTags {
		snapshotTags[k] = v
	}

	opts := &cloud.SnapshotOptions{
		Tags:       snapshotTags,
		OutpostArn: outpostArn,
	}

	// Check if the availability zone is supported for fast snapshot restore
	if len(fsrAvailabilityZones) > 0 {
		zones, error := d.cloud.AvailabilityZones(ctx)
		if error != nil {
			klog.ErrorS(error, "failed to get availability zones")
		} else {
			klog.V(4).InfoS("Availability Zones", "zone", zones)
			for _, az := range fsrAvailabilityZones {
				if _, ok := zones[az]; !ok {
					return nil, status.Errorf(codes.InvalidArgument, "Availability zone %s is not supported for fast snapshot restore", az)
				}
			}
		}
	}

	snapshot, err = d.cloud.CreateSnapshot(ctx, volumeID, opts)
	if err != nil {
		if errors.Is(err, cloud.ErrAlreadyExists) {
			return nil, status.Errorf(codes.AlreadyExists, "Snapshot %q already exists", snapshotName)
		}
		return nil, status.Errorf(codes.Internal, "Could not create snapshot %q: %v", snapshotName, err)
	}

	if len(fsrAvailabilityZones) > 0 {
		_, err := d.cloud.EnableFastSnapshotRestores(ctx, fsrAvailabilityZones, snapshot.SnapshotID)
		if err != nil {
			if _, deleteErr := d.cloud.DeleteSnapshot(ctx, snapshot.SnapshotID); deleteErr != nil {
				return nil, status.Errorf(codes.Internal, "Could not delete snapshot ID %q: %v", snapshotName, deleteErr)
			}
			return nil, status.Errorf(codes.Internal, "Failed to create Fast Snapshot Restores for snapshot ID %q: %v", snapshotName, err)
		}
	}
	return newCreateSnapshotResponse(snapshot), nil
}

func validateCreateSnapshotRequest(req *csi.CreateSnapshotRequest) error {
	if len(req.GetName()) == 0 {
		return status.Error(codes.InvalidArgument, "Snapshot name not provided")
	}

	if len(req.GetSourceVolumeId()) == 0 {
		return status.Error(codes.InvalidArgument, "Snapshot volume source ID not provided")
	}
	return nil
}

func (d *ControllerService) DeleteSnapshot(ctx context.Context, req *csi.DeleteSnapshotRequest) (*csi.DeleteSnapshotResponse, error) {
	klog.V(4).InfoS("DeleteSnapshot: called", "args", util.SanitizeRequest(req))
	if err := validateDeleteSnapshotRequest(req); err != nil {
		return nil, err
	}

	snapshotID := req.GetSnapshotId()

	// check if a request is already in-flight
	if ok := d.inFlight.Insert(snapshotID); !ok {
		msg := fmt.Sprintf("DeleteSnapshot for Snapshot %s is already in progress", snapshotID)
		return nil, status.Error(codes.Aborted, msg)
	}
	defer d.inFlight.Delete(snapshotID)

	if _, err := d.cloud.DeleteSnapshot(ctx, snapshotID); err != nil {
		if errors.Is(err, cloud.ErrNotFound) {
			klog.V(4).InfoS("DeleteSnapshot: snapshot not found, returning with success")
			return &csi.DeleteSnapshotResponse{}, nil
		}
		return nil, status.Errorf(codes.Internal, "Could not delete snapshot ID %q: %v", snapshotID, err)
	}

	return &csi.DeleteSnapshotResponse{}, nil
}

func validateDeleteSnapshotRequest(req *csi.DeleteSnapshotRequest) error {
	if len(req.GetSnapshotId()) == 0 {
		return status.Error(codes.InvalidArgument, "Snapshot ID not provided")
	}
	return nil
}

func (d *ControllerService) ListSnapshots(ctx context.Context, req *csi.ListSnapshotsRequest) (*csi.ListSnapshotsResponse, error) {
	klog.V(4).InfoS("ListSnapshots: called", "args", util.SanitizeRequest(req))
	var snapshots []*cloud.Snapshot

	snapshotID := req.GetSnapshotId()
	if len(snapshotID) != 0 {
		snapshot, err := d.cloud.GetSnapshotByID(ctx, snapshotID)
		if err != nil {
			if errors.Is(err, cloud.ErrNotFound) {
				klog.V(4).InfoS("ListSnapshots: snapshot not found, returning with success")
				return &csi.ListSnapshotsResponse{}, nil
			}
			return nil, status.Errorf(codes.Internal, "Could not get snapshot ID %q: %v", snapshotID, err)
		}
		snapshots = append(snapshots, snapshot)
		response := newListSnapshotsResponse(&cloud.ListSnapshotsResponse{
			Snapshots: snapshots,
		})
		return response, nil
	}

	volumeID := req.GetSourceVolumeId()
	nextToken := req.GetStartingToken()
	maxEntries := req.GetMaxEntries()

	cloudSnapshots, err := d.cloud.ListSnapshots(ctx, volumeID, maxEntries, nextToken)
	if err != nil {
		if errors.Is(err, cloud.ErrNotFound) {
			klog.V(4).InfoS("ListSnapshots: snapshot not found, returning with success")
			return &csi.ListSnapshotsResponse{}, nil
		}
		if errors.Is(err, cloud.ErrInvalidMaxResults) {
			return nil, status.Errorf(codes.InvalidArgument, "Error mapping MaxEntries to AWS MaxResults: %v", err)
		}
		return nil, status.Errorf(codes.Internal, "Could not list snapshots: %v", err)
	}

	response := newListSnapshotsResponse(cloudSnapshots)
	return response, nil
}

// pickAvailabilityZone selects 1 zone given topology requirement.
// if not found, empty string is returned.
func pickAvailabilityZone(requirement *csi.TopologyRequirement) string {
	if requirement == nil {
		return ""
	}
	for _, topology := range requirement.GetPreferred() {
		zone, exists := topology.GetSegments()[WellKnownZoneTopologyKey]
		if exists {
			return zone
		}

		zone, exists = topology.GetSegments()[ZoneTopologyKey]
		if exists {
			return zone
		}
	}
	for _, topology := range requirement.GetRequisite() {
		zone, exists := topology.GetSegments()[WellKnownZoneTopologyKey]
		if exists {
			return zone
		}
		zone, exists = topology.GetSegments()[ZoneTopologyKey]
		if exists {
			return zone
		}
	}
	return ""
}

func getOutpostArn(requirement *csi.TopologyRequirement) string {
	if requirement == nil {
		return ""
	}
	for _, topology := range requirement.GetPreferred() {
		_, exists := topology.GetSegments()[AwsOutpostIDKey]
		if exists {
			return BuildOutpostArn(topology.GetSegments())
		}
	}
	for _, topology := range requirement.GetRequisite() {
		_, exists := topology.GetSegments()[AwsOutpostIDKey]
		if exists {
			return BuildOutpostArn(topology.GetSegments())
		}
	}

	return ""
}

func newCreateVolumeResponse(disk *cloud.Disk, ctx map[string]string) *csi.CreateVolumeResponse {
	var src *csi.VolumeContentSource
	if disk.SnapshotID != "" {
		src = &csi.VolumeContentSource{
			Type: &csi.VolumeContentSource_Snapshot{
				Snapshot: &csi.VolumeContentSource_SnapshotSource{
					SnapshotId: disk.SnapshotID,
				},
			},
		}
	}

	segments := map[string]string{WellKnownZoneTopologyKey: disk.AvailabilityZone}

	arn, err := arn.Parse(disk.OutpostArn)

	if err == nil {
		segments[AwsRegionKey] = arn.Region
		segments[AwsPartitionKey] = arn.Partition
		segments[AwsAccountIDKey] = arn.AccountID
		segments[AwsOutpostIDKey] = strings.ReplaceAll(arn.Resource, "outpost/", "")
	}

	return &csi.CreateVolumeResponse{
		Volume: &csi.Volume{
			VolumeId:      disk.VolumeID,
			CapacityBytes: util.GiBToBytes(disk.CapacityGiB),
			VolumeContext: ctx,
			AccessibleTopology: []*csi.Topology{
				{
					Segments: segments,
				},
			},
			ContentSource: src,
		},
	}
}

func newCreateSnapshotResponse(snapshot *cloud.Snapshot) *csi.CreateSnapshotResponse {
	ts := timestamppb.New(snapshot.CreationTime)

	return &csi.CreateSnapshotResponse{
		Snapshot: &csi.Snapshot{
			SnapshotId:     snapshot.SnapshotID,
			SourceVolumeId: snapshot.SourceVolumeID,
			SizeBytes:      util.GiBToBytes(snapshot.Size),
			CreationTime:   ts,
			ReadyToUse:     snapshot.ReadyToUse,
		},
	}
}

func newListSnapshotsResponse(cloudResponse *cloud.ListSnapshotsResponse) *csi.ListSnapshotsResponse {

	var entries []*csi.ListSnapshotsResponse_Entry
	for _, snapshot := range cloudResponse.Snapshots {
		snapshotResponseEntry := newListSnapshotsResponseEntry(snapshot)
		entries = append(entries, snapshotResponseEntry)
	}
	return &csi.ListSnapshotsResponse{
		Entries:   entries,
		NextToken: cloudResponse.NextToken,
	}
}

func newListSnapshotsResponseEntry(snapshot *cloud.Snapshot) *csi.ListSnapshotsResponse_Entry {
	ts := timestamppb.New(snapshot.CreationTime)

	return &csi.ListSnapshotsResponse_Entry{
		Snapshot: &csi.Snapshot{
			SnapshotId:     snapshot.SnapshotID,
			SourceVolumeId: snapshot.SourceVolumeID,
			SizeBytes:      util.GiBToBytes(snapshot.Size),
			CreationTime:   ts,
			ReadyToUse:     snapshot.ReadyToUse,
		},
	}
}

func getVolSizeBytes(req *csi.CreateVolumeRequest) (int64, error) {
	var volSizeBytes int64
	capRange := req.GetCapacityRange()
	if capRange == nil {
		volSizeBytes = cloud.DefaultVolumeSize
	} else {
		volSizeBytes = util.RoundUpBytes(capRange.GetRequiredBytes())
		maxVolSize := capRange.GetLimitBytes()
		if maxVolSize > 0 && maxVolSize < volSizeBytes {
			return 0, status.Error(codes.InvalidArgument, "After round-up, volume size exceeds the limit specified")
		}
	}
	return volSizeBytes, nil
}

// BuildOutpostArn returns the string representation of the outpost ARN from the given csi.TopologyRequirement.segments
func BuildOutpostArn(segments map[string]string) string {

	if len(segments[AwsPartitionKey]) <= 0 {
		return ""
	}

	if len(segments[AwsRegionKey]) <= 0 {
		return ""
	}
	if len(segments[AwsOutpostIDKey]) <= 0 {
		return ""
	}
	if len(segments[AwsAccountIDKey]) <= 0 {
		return ""
	}

	return fmt.Sprintf("arn:%s:outposts:%s:%s:outpost/%s",
		segments[AwsPartitionKey],
		segments[AwsRegionKey],
		segments[AwsAccountIDKey],
		segments[AwsOutpostIDKey],
	)
}

func validateFormattingOption(volumeCapabilities []*csi.VolumeCapability, paramName string, fsConfigs map[string]fileSystemConfig) error {
	for _, volCap := range volumeCapabilities {
		switch volCap.GetAccessType().(type) {
		case *csi.VolumeCapability_Block:
			return status.Error(codes.InvalidArgument, fmt.Sprintf("Cannot use %s with block volume", paramName))
		}

		mountVolume := volCap.GetMount()
		if mountVolume == nil {
			return status.Error(codes.InvalidArgument, "CreateVolume: mount is nil within volume capability")
		}

		fsType := mountVolume.GetFsType()
		if supported := fsConfigs[fsType].isParameterSupported(paramName); !supported {
			return status.Errorf(codes.InvalidArgument, "Cannot use %s with fstype %s", paramName, fsType)
		}
	}

	return nil
}
