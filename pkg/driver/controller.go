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
	"os"
	"strconv"
	"strings"

	"github.com/aws/aws-sdk-go/aws/arn"
	csi "github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/kubernetes-sigs/aws-ebs-csi-driver/pkg/cloud"
	"github.com/kubernetes-sigs/aws-ebs-csi-driver/pkg/driver/internal"
	"github.com/kubernetes-sigs/aws-ebs-csi-driver/pkg/util"
	"github.com/kubernetes-sigs/aws-ebs-csi-driver/pkg/util/template"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
	"k8s.io/klog/v2"
)

var (
	// volumeCaps represents how the volume could be accessed.
	// It is SINGLE_NODE_WRITER since EBS volume could only be
	// attached to a single node at any given time.
	volumeCaps = []csi.VolumeCapability_AccessMode{
		{
			Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER,
		},
	}

	// controllerCaps represents the capability of controller service
	controllerCaps = []csi.ControllerServiceCapability_RPC_Type{
		csi.ControllerServiceCapability_RPC_CREATE_DELETE_VOLUME,
		csi.ControllerServiceCapability_RPC_PUBLISH_UNPUBLISH_VOLUME,
		csi.ControllerServiceCapability_RPC_CREATE_DELETE_SNAPSHOT,
		csi.ControllerServiceCapability_RPC_LIST_SNAPSHOTS,
		csi.ControllerServiceCapability_RPC_EXPAND_VOLUME,
	}
)

const isManagedByDriver = "true"

// controllerService represents the controller service of CSI driver
type controllerService struct {
	cloud         cloud.Cloud
	inFlight      *internal.InFlight
	driverOptions *DriverOptions
}

var (
	// NewMetadataFunc is a variable for the cloud.NewMetadata function that can
	// be overwritten in unit tests.
	NewMetadataFunc = cloud.NewMetadataService
	// NewCloudFunc is a variable for the cloud.NewCloud function that can
	// be overwritten in unit tests.
	NewCloudFunc = cloud.NewCloud
)

// newControllerService creates a new controller service
// it panics if failed to create the service
func newControllerService(driverOptions *DriverOptions) controllerService {
	region := os.Getenv("AWS_REGION")
	if region == "" {
		klog.V(5).InfoS("[Debug] Retrieving region from metadata service")
		metadata, err := NewMetadataFunc(cloud.DefaultEC2MetadataClient, cloud.DefaultKubernetesAPIClient, region)
		if err != nil {
			klog.ErrorS(err, "Could not determine region from any metadata service. The region can be manually supplied via the AWS_REGION environment variable.")
			panic(err)
		}
		region = metadata.GetRegion()
	}

	cloudSrv, err := NewCloudFunc(region, driverOptions.awsSdkDebugLog)
	if err != nil {
		panic(err)
	}

	return controllerService{
		cloud:         cloudSrv,
		inFlight:      internal.NewInFlight(),
		driverOptions: driverOptions,
	}
}

func (d *controllerService) CreateVolume(ctx context.Context, req *csi.CreateVolumeRequest) (*csi.CreateVolumeResponse, error) {
	klog.V(4).InfoS("CreateVolume: called", "args", *req)
	if err := validateCreateVolumeRequest(req); err != nil {
		return nil, err
	}
	volSizeBytes, err := getVolSizeBytes(req)
	if err != nil {
		return nil, err
	}
	volName := req.GetName()

	// check if a request is already in-flight
	if ok := d.inFlight.Insert(volName); !ok {
		msg := fmt.Sprintf("Create volume request for %s is already in progress", volName)
		return nil, status.Error(codes.Aborted, msg)
	}
	defer d.inFlight.Delete(volName)

	var (
		volumeType             string
		iopsPerGB              int
		allowIOPSPerGBIncrease bool
		iops                   int
		throughput             int
		isEncrypted            bool
		blockExpress           bool
		kmsKeyID               string
		scTags                 []string
		volumeTags             = map[string]string{
			cloud.VolumeNameTagKey:   volName,
			cloud.AwsEbsDriverTagKey: isManagedByDriver,
		}
		blockSize string
	)

	tProps := new(template.Props)

	for key, value := range req.GetParameters() {
		switch strings.ToLower(key) {
		case "fstype":
			klog.InfoS("\"fstype\" is deprecated, please use \"csi.storage.k8s.io/fstype\" instead")
		case VolumeTypeKey:
			volumeType = value
		case IopsPerGBKey:
			iopsPerGB, err = strconv.Atoi(value)
			if err != nil {
				return nil, status.Errorf(codes.InvalidArgument, "Could not parse invalid iopsPerGB: %v", err)
			}
		case AllowAutoIOPSPerGBIncreaseKey:
			allowIOPSPerGBIncrease = value == "true"
		case IopsKey:
			iops, err = strconv.Atoi(value)
			if err != nil {
				return nil, status.Errorf(codes.InvalidArgument, "Could not parse invalid iops: %v", err)
			}
		case ThroughputKey:
			throughput, err = strconv.Atoi(value)
			if err != nil {
				return nil, status.Errorf(codes.InvalidArgument, "Could not parse invalid throughput: %v", err)
			}
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
			_, err = strconv.Atoi(value)
			if err != nil {
				return nil, status.Errorf(codes.InvalidArgument, "Could not parse blockSize (%s): %v", value, err)
			}
			blockSize = value
		default:
			if strings.HasPrefix(key, TagKeyPrefix) {
				scTags = append(scTags, value)
			} else {
				return nil, status.Errorf(codes.InvalidArgument, "Invalid parameter key %s for CreateVolume", key)
			}
		}
	}

	if len(blockSize) > 0 {
		for _, volCap := range req.GetVolumeCapabilities() {
			switch volCap.GetAccessType().(type) {
			case *csi.VolumeCapability_Block:
				return nil, status.Error(codes.InvalidArgument, "Cannot use block size with block volume")
			}

			mountVolume := volCap.GetMount()
			if mountVolume == nil {
				return nil, status.Error(codes.InvalidArgument, "CreateVolume: mount is nil within volume capability")
			}

			fsType := mountVolume.GetFsType()

			if _, ok := BlockSizeExcludedFSTypes[fsType]; ok {
				return nil, status.Errorf(codes.InvalidArgument, "Cannot use block size with fstype %s", fsType)
			}
		}
	}

	if volumeType == cloud.VolumeTypeIO1 {
		if iopsPerGB == 0 {
			return nil, status.Errorf(codes.InvalidArgument, "The parameter IOPSPerGB must be specified for io1 volumes")
		}
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
	if d.driverOptions.kubernetesClusterID != "" {
		resourceLifecycleTag := ResourceLifecycleTagPrefix + d.driverOptions.kubernetesClusterID
		volumeTags[resourceLifecycleTag] = ResourceLifecycleOwned
		volumeTags[NameTag] = d.driverOptions.kubernetesClusterID + "-dynamic-" + volName
		volumeTags[KubernetesClusterTag] = d.driverOptions.kubernetesClusterID
	}
	for k, v := range d.driverOptions.extraTags {
		volumeTags[k] = v
	}

	addTags, err := template.Evaluate(scTags, tProps, d.driverOptions.warnOnInvalidTag)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "Error interpolating the tag value: %v", err)
	}

	if err = validateExtraTags(addTags, d.driverOptions.warnOnInvalidTag); err != nil {
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
	}

	disk, err := d.cloud.CreateDisk(ctx, volName, opts)
	if err != nil {
		errCode := codes.Internal
		if errors.Is(err, cloud.ErrNotFound) {
			errCode = codes.NotFound
		}
		if errors.Is(err, cloud.ErrIdempotentParameterMismatch) {
			errCode = codes.AlreadyExists
		}
		return nil, status.Errorf(errCode, "Could not create volume %q: %v", volName, err)
	}
	return newCreateVolumeResponse(disk, blockSize), nil
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
		modes := util.GetAccessModes(volCaps)
		stringModes := strings.Join(*modes, ", ")
		errString := "Volume capabilities " + stringModes + " not supported. Only AccessModes[ReadWriteOnce] supported."
		return status.Error(codes.InvalidArgument, errString)
	}
	return nil
}

func (d *controllerService) DeleteVolume(ctx context.Context, req *csi.DeleteVolumeRequest) (*csi.DeleteVolumeResponse, error) {
	klog.V(4).InfoS("DeleteVolume: called", "args", *req)
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

func (d *controllerService) ControllerPublishVolume(ctx context.Context, req *csi.ControllerPublishVolumeRequest) (*csi.ControllerPublishVolumeResponse, error) {
	klog.V(4).InfoS("ControllerPublishVolume: called", "args", *req)
	if err := validateControllerPublishVolumeRequest(req); err != nil {
		return nil, err
	}

	volumeID := req.GetVolumeId()
	nodeID := req.GetNodeId()

	if !d.cloud.IsExistInstance(ctx, nodeID) {
		return nil, status.Errorf(codes.NotFound, "Instance %q not found", nodeID)
	}
	disk, err := d.cloud.GetDiskByID(ctx, volumeID)
	if err != nil {
		if errors.Is(err, cloud.ErrNotFound) {
			return nil, status.Error(codes.NotFound, "Volume not found")
		}
		return nil, status.Errorf(codes.Internal, "Could not get volume with ID %q: %v", volumeID, err)
	}

	// If given volumeId already assigned to given node, will directly return current device path
	devicePath, err := d.cloud.AttachDisk(ctx, volumeID, nodeID)
	if err != nil {
		if errors.Is(err, cloud.ErrVolumeInUse) {
			return nil, status.Error(codes.FailedPrecondition, strings.Join(disk.Attachments, ","))
		}
		// TODO: Check volume capability matches for ALREADY_EXISTS
		return nil, status.Errorf(codes.Internal, "Could not attach volume %q to node %q: %v", volumeID, nodeID, err)
	}
	klog.V(5).InfoS("[Debug] ControllerPublishVolume: attached to node", "volumeID", volumeID, "nodeID", nodeID, "devicePath", devicePath)

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

	caps := []*csi.VolumeCapability{volCap}
	if !isValidVolumeCapabilities(caps) {
		modes := util.GetAccessModes(caps)
		stringModes := strings.Join(*modes, ", ")
		errString := "Volume capabilities " + stringModes + " not supported. Only AccessModes[ReadWriteOnce] supported."
		return status.Error(codes.InvalidArgument, errString)
	}
	return nil
}

func (d *controllerService) ControllerUnpublishVolume(ctx context.Context, req *csi.ControllerUnpublishVolumeRequest) (*csi.ControllerUnpublishVolumeResponse, error) {
	klog.V(4).InfoS("ControllerUnpublishVolume: called", "args", *req)
	if err := validateControllerUnpublishVolumeRequest(req); err != nil {
		return nil, err
	}

	volumeID := req.GetVolumeId()
	nodeID := req.GetNodeId()

	if err := d.cloud.DetachDisk(ctx, volumeID, nodeID); err != nil {
		if errors.Is(err, cloud.ErrNotFound) {
			return &csi.ControllerUnpublishVolumeResponse{}, nil
		}
		return nil, status.Errorf(codes.Internal, "Could not detach volume %q from node %q: %v", volumeID, nodeID, err)
	}
	klog.V(5).InfoS("[Debug] ControllerUnpublishVolume: detached from node", "volumeID", volumeID, "nodeID", nodeID)

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

func (d *controllerService) ControllerGetCapabilities(ctx context.Context, req *csi.ControllerGetCapabilitiesRequest) (*csi.ControllerGetCapabilitiesResponse, error) {
	klog.V(4).InfoS("ControllerGetCapabilities: called", "args", *req)
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

func (d *controllerService) GetCapacity(ctx context.Context, req *csi.GetCapacityRequest) (*csi.GetCapacityResponse, error) {
	klog.V(4).InfoS("GetCapacity: called", "args", *req)
	return nil, status.Error(codes.Unimplemented, "")
}

func (d *controllerService) ListVolumes(ctx context.Context, req *csi.ListVolumesRequest) (*csi.ListVolumesResponse, error) {
	klog.V(4).InfoS("ListVolumes: called", "args", *req)
	return nil, status.Error(codes.Unimplemented, "")
}

func (d *controllerService) ValidateVolumeCapabilities(ctx context.Context, req *csi.ValidateVolumeCapabilitiesRequest) (*csi.ValidateVolumeCapabilitiesResponse, error) {
	klog.V(4).InfoS("ValidateVolumeCapabilities: called", "args", *req)
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

func (d *controllerService) ControllerExpandVolume(ctx context.Context, req *csi.ControllerExpandVolumeRequest) (*csi.ControllerExpandVolumeResponse, error) {
	klog.V(4).InfoS("ControllerExpandVolume: called", "args", *req)
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

	actualSizeGiB, err := d.cloud.ResizeDisk(ctx, volumeID, newSize)
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

func (d *controllerService) ControllerGetVolume(ctx context.Context, req *csi.ControllerGetVolumeRequest) (*csi.ControllerGetVolumeResponse, error) {
	klog.V(4).InfoS("ControllerGetVolume: called", "args", *req)
	return nil, status.Error(codes.Unimplemented, "")
}

func isValidVolumeCapabilities(volCaps []*csi.VolumeCapability) bool {
	hasSupport := func(cap *csi.VolumeCapability) bool {
		for _, c := range volumeCaps {
			if c.GetMode() == cap.AccessMode.GetMode() {
				return true
			}
		}
		return false
	}

	foundAll := true
	for _, c := range volCaps {
		if !hasSupport(c) {
			foundAll = false
		}
	}
	return foundAll
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

func (d *controllerService) CreateSnapshot(ctx context.Context, req *csi.CreateSnapshotRequest) (*csi.CreateSnapshotResponse, error) {
	klog.V(4).InfoS("CreateSnapshot: called", "args", req)
	if err := validateCreateSnapshotRequest(req); err != nil {
		return nil, err
	}

	snapshotName := req.GetName()
	volumeID := req.GetSourceVolumeId()

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
		return newCreateSnapshotResponse(snapshot)
	}

	snapshotTags := map[string]string{
		cloud.SnapshotNameTagKey: snapshotName,
		cloud.AwsEbsDriverTagKey: isManagedByDriver,
	}

	var vscTags []string
	for key, value := range req.GetParameters() {
		if strings.HasPrefix(key, TagKeyPrefix) {
			vscTags = append(vscTags, value)
		} else {
			return nil, status.Errorf(codes.InvalidArgument, "Invalid parameter key %s for CreateSnapshot", key)
		}
	}

	addTags, err := template.Evaluate(vscTags, nil, d.driverOptions.warnOnInvalidTag)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "Error interpolating the tag value: %v", err)
	}

	if err = validateExtraTags(addTags, d.driverOptions.warnOnInvalidTag); err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "Invalid tag value: %v", err)
	}

	for k, v := range addTags {
		snapshotTags[k] = v
	}

	if d.driverOptions.kubernetesClusterID != "" {
		resourceLifecycleTag := ResourceLifecycleTagPrefix + d.driverOptions.kubernetesClusterID
		snapshotTags[resourceLifecycleTag] = ResourceLifecycleOwned
		snapshotTags[NameTag] = d.driverOptions.kubernetesClusterID + "-dynamic-" + snapshotName
	}
	for k, v := range d.driverOptions.extraTags {
		snapshotTags[k] = v
	}
	opts := &cloud.SnapshotOptions{
		Tags: snapshotTags,
	}

	snapshot, err = d.cloud.CreateSnapshot(ctx, volumeID, opts)

	if err != nil {
		return nil, status.Errorf(codes.Internal, "Could not create snapshot %q: %v", snapshotName, err)
	}
	return newCreateSnapshotResponse(snapshot)
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

func (d *controllerService) DeleteSnapshot(ctx context.Context, req *csi.DeleteSnapshotRequest) (*csi.DeleteSnapshotResponse, error) {
	klog.V(4).InfoS("DeleteSnapshot: called", "args", req)
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

func (d *controllerService) ListSnapshots(ctx context.Context, req *csi.ListSnapshotsRequest) (*csi.ListSnapshotsResponse, error) {
	klog.V(4).InfoS("ListSnapshots: called", "args", req)
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
	maxEntries := int64(req.GetMaxEntries())

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
		zone, exists := topology.GetSegments()[WellKnownTopologyKey]
		if exists {
			return zone
		}

		zone, exists = topology.GetSegments()[TopologyKey]
		if exists {
			return zone
		}
	}
	for _, topology := range requirement.GetRequisite() {
		zone, exists := topology.GetSegments()[WellKnownTopologyKey]
		if exists {
			return zone
		}
		zone, exists = topology.GetSegments()[TopologyKey]
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

func newCreateVolumeResponse(disk *cloud.Disk, blockSize string) *csi.CreateVolumeResponse {
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

	segments := map[string]string{TopologyKey: disk.AvailabilityZone}

	arn, err := arn.Parse(disk.OutpostArn)

	if err == nil {
		segments[AwsRegionKey] = arn.Region
		segments[AwsPartitionKey] = arn.Partition
		segments[AwsAccountIDKey] = arn.AccountID
		segments[AwsOutpostIDKey] = strings.ReplaceAll(arn.Resource, "outpost/", "")
	}

	context := map[string]string{}
	if len(blockSize) > 0 {
		context[BlockSizeKey] = blockSize
	}

	return &csi.CreateVolumeResponse{
		Volume: &csi.Volume{
			VolumeId:      disk.VolumeID,
			CapacityBytes: util.GiBToBytes(disk.CapacityGiB),
			VolumeContext: context,
			AccessibleTopology: []*csi.Topology{
				{
					Segments: segments,
				},
			},
			ContentSource: src,
		},
	}
}

func newCreateSnapshotResponse(snapshot *cloud.Snapshot) (*csi.CreateSnapshotResponse, error) {
	ts := timestamppb.New(snapshot.CreationTime)

	return &csi.CreateSnapshotResponse{
		Snapshot: &csi.Snapshot{
			SnapshotId:     snapshot.SnapshotID,
			SourceVolumeId: snapshot.SourceVolumeID,
			SizeBytes:      snapshot.Size,
			CreationTime:   ts,
			ReadyToUse:     snapshot.ReadyToUse,
		},
	}, nil
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
			SizeBytes:      snapshot.Size,
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
