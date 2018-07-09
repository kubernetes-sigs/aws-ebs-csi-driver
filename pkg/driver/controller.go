package driver

import (
	"context"

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

	volSizeBytes := int64(4000000000)
	//if req.GetCapacityRange() == nil {
	//return nil, status.Error(codes.InvalidArgument, "Volume size not provided")
	//}
	roundSize := volumeutil.RoundUpSize(
		volSizeBytes,
		1024*1024*1024,
	)

	const volNameTagKey = "VolumeName"
	volumes, err := d.cloud.GetVolumesByTagName(volNameTagKey, volName)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	var volID string
	if len(volumes) == 1 {
		volID = volumes[0]
	} else if len(volumes) > 1 {
		return nil, status.Error(codes.Internal, "multiple volumes with same name")
	} else {
		// TODO check for int overflow
		v, err := d.cloud.CreateDisk(&cloud.DiskOptions{
			CapacityGB: int(roundSize),
			Tags:       map[string]string{volNameTagKey: volName},
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
			CapacityBytes: roundSize * 1024 * 1024 * 1024,
		},
	}, nil
}

func (d *Driver) DeleteVolume(ctx context.Context, req *csi.DeleteVolumeRequest) (*csi.DeleteVolumeResponse, error) {
	//volID := req.GetVolumeId()
	//if len(volID) == 0 {
	//return nil, status.Error(codes.InvalidArgument, "Volume ID not provided")
	//}

	//_, err := d.cloud.DeleteDisk(aws.KubernetesVolumeID(volID))
	//if err != nil {
	//glog.V(3).Infof("Failed to delete volume: %v", err)
	//}

	return &csi.DeleteVolumeResponse{}, nil
}

func (d *Driver) ControllerPublishVolume(ctx context.Context, req *csi.ControllerPublishVolumeRequest) (*csi.ControllerPublishVolumeResponse, error) {
	return &csi.ControllerPublishVolumeResponse{}, nil
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
