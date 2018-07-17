package driver

import (
	"context"
	"fmt"

	"github.com/bertinatto/ebs-csi-driver/pkg/cloud"
	csi "github.com/container-storage-interface/spec/lib/go/csi/v0"
	"github.com/golang/glog"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	volumeutil "k8s.io/kubernetes/pkg/volume/util"
)

func (d *Driver) CreateVolume(ctx context.Context, req *csi.CreateVolumeRequest) (*csi.CreateVolumeResponse, error) {
	volName := req.GetName()
	if len(volName) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Volume name not provided")
	}

	volSizeBytes := cloud.DefaultVolumeSize
	if req.GetCapacityRange() != nil {
		volSizeBytes = req.GetCapacityRange().GetRequiredBytes()
	}
	// TODO: check for int overflow
	// TODO: check if this round up is really necessary
	roundSize := int(volumeutil.RoundUpSize(
		volSizeBytes,
		1024*1024*1024,
	))

	volCaps := req.GetVolumeCapabilities()
	if volCaps == nil || len(volCaps) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Volume capabilities not provided")
	}

	// FIXME: for some reason, AWS takes a while to tag the volume after it's created.
	// As a result, this call could be racy.
	volumes, err := d.cloud.GetVolumesByNameAndSize(cloud.VolumeNameTagKey, volName, roundSize)
	if err != nil {
		switch err {
		case cloud.ErrWrongDiskSize:
			return nil, status.Error(codes.AlreadyExists, err.Error())
		default:
			return nil, status.Error(codes.Internal, err.Error())
		}
	}

	var volID string
	if len(volumes) == 1 {
		volID = volumes[0]
	} else if len(volumes) > 1 {
		msg := fmt.Sprintf("multiple volumes with same name: %v", volumes)
		return nil, status.Error(codes.Internal, msg)
	} else {
		v, err := d.cloud.CreateDisk(volName, &cloud.DiskOptions{
			CapacityGB: roundSize,
			Tags:       map[string]string{cloud.VolumeNameTagKey: volName},
		})
		if err != nil {
			glog.V(3).Infof("Failed to create volume: %v", err)
			return nil, status.Error(codes.Internal, err.Error())
		}

		awsID, err := v.MapToAWSVolumeID()
		if err != nil {
			return nil, status.Error(codes.Internal, err.Error())
		}

		volID = string(awsID)
	}

	return &csi.CreateVolumeResponse{
		Volume: &csi.Volume{
			Id:            volID,
			CapacityBytes: int64(roundSize * 1000 * 1000 * 1000),
		},
	}, nil
}

func (d *Driver) DeleteVolume(ctx context.Context, req *csi.DeleteVolumeRequest) (*csi.DeleteVolumeResponse, error) {
	volID := req.GetVolumeId()
	if len(volID) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Volume ID not provided")
	}

	_, err := d.cloud.DeleteDisk(cloud.VolumeID(volID))
	if err != nil {
		return nil, err
	}

	return &csi.DeleteVolumeResponse{}, nil
}

func (d *Driver) ControllerPublishVolume(ctx context.Context, req *csi.ControllerPublishVolumeRequest) (*csi.ControllerPublishVolumeResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}

func (d *Driver) ControllerUnpublishVolume(ctx context.Context, req *csi.ControllerUnpublishVolumeRequest) (*csi.ControllerUnpublishVolumeResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}

func (d *Driver) ControllerGetCapabilities(ctx context.Context, req *csi.ControllerGetCapabilitiesRequest) (*csi.ControllerGetCapabilitiesResponse, error) {
	newCap := func(cap csi.ControllerServiceCapability_RPC_Type) *csi.ControllerServiceCapability {
		return &csi.ControllerServiceCapability{
			Type: &csi.ControllerServiceCapability_Rpc{
				Rpc: &csi.ControllerServiceCapability_RPC{
					Type: cap,
				},
			},
		}
	}

	var caps []*csi.ControllerServiceCapability
	for _, cap := range []csi.ControllerServiceCapability_RPC_Type{
		csi.ControllerServiceCapability_RPC_CREATE_DELETE_VOLUME,
		csi.ControllerServiceCapability_RPC_PUBLISH_UNPUBLISH_VOLUME,
		csi.ControllerServiceCapability_RPC_LIST_VOLUMES,
	} {
		caps = append(caps, newCap(cap))
	}

	resp := &csi.ControllerGetCapabilitiesResponse{
		Capabilities: caps,
	}

	return resp, nil
}

func (d *Driver) GetCapacity(ctx context.Context, req *csi.GetCapacityRequest) (*csi.GetCapacityResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}

func (d *Driver) ListVolumes(ctx context.Context, req *csi.ListVolumesRequest) (*csi.ListVolumesResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}

func (d *Driver) ValidateVolumeCapabilities(ctx context.Context, req *csi.ValidateVolumeCapabilitiesRequest) (*csi.ValidateVolumeCapabilitiesResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}

func (d *Driver) CreateSnapshot(ctx context.Context, req *csi.CreateSnapshotRequest) (*csi.CreateSnapshotResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}

func (d *Driver) DeleteSnapshot(ctx context.Context, req *csi.DeleteSnapshotRequest) (*csi.DeleteSnapshotResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}

func (d *Driver) ListSnapshots(ctx context.Context, req *csi.ListSnapshotsRequest) (*csi.ListSnapshotsResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}
