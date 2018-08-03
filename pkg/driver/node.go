package driver

import (
	"context"
	"fmt"
	"os"

	csi "github.com/container-storage-interface/spec/lib/go/csi/v0"
	"github.com/golang/glog"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func (d *Driver) NodeStageVolume(ctx context.Context, req *csi.NodeStageVolumeRequest) (*csi.NodeStageVolumeResponse, error) {
	glog.Infoln("NodeStageVolume call")
	volumeID := req.GetVolumeId()
	if len(volumeID) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Volume ID not provided")
	}

	target := req.GetStagingTargetPath()
	if len(target) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Staging target not provided")
	}

	volCap := req.GetVolumeCapability()
	if volCap == nil {
		return nil, status.Error(codes.InvalidArgument, "Volume capability not provided")
	}

	if !d.isValidVolumeCapabilities([]*csi.VolumeCapability{volCap}) {
		return nil, status.Error(codes.InvalidArgument, "Volume capability not supported")
	}

	// TODO: get device attached (source)
	source := "/host/dev/xvdbc"

	// TODO: consider replacing IsNotMountPoint by IsLikelyNotMountPoint
	notMnt, err := d.mounter.Interface.IsNotMountPoint(target)
	if err != nil {
		if !os.IsNotExist(err) {
			msg := fmt.Sprintf("could not determine if %q is valid mount point: %v", target, err)
			return nil, status.Error(codes.Internal, msg)
		}
		if errMkDir := d.mounter.Interface.MakeDir(target); errMkDir != nil {
			msg := fmt.Sprintf("could not create target dir %q: %v", target, errMkDir)
			return nil, status.Error(codes.Internal, msg)
		}
	}

	if !notMnt {
		msg := fmt.Sprintf("target %q is not a valid mount point", target)
		return nil, status.Error(codes.InvalidArgument, msg)
	}

	// FormatAndMount will format only if needed
	err = d.mounter.FormatAndMount(source, target, "ext4", nil)
	if err != err {
		msg := fmt.Sprintf("could not format %q and mount it at %q", source, target)
		return nil, status.Error(codes.Internal, msg)
	}

	glog.Infoln("FormatAndMount success call")
	return &csi.NodeStageVolumeResponse{}, nil
}

func (d *Driver) NodeUnstageVolume(ctx context.Context, req *csi.NodeUnstageVolumeRequest) (*csi.NodeUnstageVolumeResponse, error) {
	volumeID := req.GetVolumeId()
	if len(volumeID) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Volume ID not provided")
	}

	target := req.GetStagingTargetPath()
	if len(target) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Staging target not provided")
	}

	err := d.mounter.Interface.Unmount(target)
	if err != nil {
		msg := fmt.Sprintf("Could not unstage target %q: %v", target, err)
		return nil, status.Error(codes.Internal, msg)
	}

	return &csi.NodeUnstageVolumeResponse{}, nil
}

func (d *Driver) NodePublishVolume(ctx context.Context, req *csi.NodePublishVolumeRequest) (*csi.NodePublishVolumeResponse, error) {
	volumeID := req.GetVolumeId()
	if len(volumeID) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Volume ID not provided")
	}

	source := req.GetStagingTargetPath()
	if len(source) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Staging target not provided")
	}

	target := req.GetTargetPath()
	if len(target) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Target path not provided")
	}

	volCap := req.GetVolumeCapability()
	if volCap == nil {
		return nil, status.Error(codes.InvalidArgument, "Volume capability not provided")
	}

	if !d.isValidVolumeCapabilities([]*csi.VolumeCapability{volCap}) {
		return nil, status.Error(codes.InvalidArgument, "Volume capability not supported")
	}

	options := []string{"bind"}
	options = append(options, "rw")
	//if req.GetReadonly() {
	//options = append(options, "ro")
	//}

	if err := d.mounter.Interface.MakeDir(target); err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	if err := d.mounter.Interface.Mount(source, target, "ext4", options); err != nil {
		os.Remove(target)
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &csi.NodePublishVolumeResponse{}, nil
}

func (d *Driver) NodeUnpublishVolume(ctx context.Context, req *csi.NodeUnpublishVolumeRequest) (*csi.NodeUnpublishVolumeResponse, error) {
	volumeID := req.GetVolumeId()
	if len(volumeID) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Volume ID not provided")
	}

	target := req.GetTargetPath()
	if len(target) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Target path not provided")
	}

	err := d.mounter.Interface.Unmount(target)
	if err != nil {
		msg := fmt.Sprintf("Could not unpublish target %q: %v", target, err)
		return nil, status.Error(codes.Internal, msg)
	}

	return &csi.NodeUnpublishVolumeResponse{}, nil
}

func (d *Driver) NodeGetCapabilities(ctx context.Context, req *csi.NodeGetCapabilitiesRequest) (*csi.NodeGetCapabilitiesResponse, error) {
	var caps []*csi.NodeServiceCapability
	for _, cap := range d.nodeCaps {
		c := &csi.NodeServiceCapability{
			Type: &csi.NodeServiceCapability_Rpc{
				Rpc: &csi.NodeServiceCapability_RPC{
					Type: cap,
				},
			},
		}
		caps = append(caps, c)
	}
	return &csi.NodeGetCapabilitiesResponse{Capabilities: caps}, nil
}

func (d *Driver) NodeGetInfo(ctx context.Context, req *csi.NodeGetInfoRequest) (*csi.NodeGetInfoResponse, error) {
	m := d.cloud.GetMetadata()
	return &csi.NodeGetInfoResponse{
		NodeId: m.GetInstanceID(),
	}, nil
}

func (d *Driver) NodeGetId(ctx context.Context, req *csi.NodeGetIdRequest) (*csi.NodeGetIdResponse, error) {
	m := d.cloud.GetMetadata()
	return &csi.NodeGetIdResponse{
		NodeId: m.GetInstanceID(),
	}, nil
}
