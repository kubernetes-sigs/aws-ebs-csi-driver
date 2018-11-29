/*
Copyright 2018 The Kubernetes Authors.

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
	"fmt"
	"os"

	csi "github.com/container-storage-interface/spec/lib/go/csi/v0"
	"github.com/golang/glog"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	// default file system type to be used when it is not provided
	defaultFsType = "ext4"
)

func (d *Driver) NodeStageVolume(ctx context.Context, req *csi.NodeStageVolumeRequest) (*csi.NodeStageVolumeResponse, error) {
	glog.V(4).Infof("NodeStageVolume: called with args %#v", req)
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

	source, ok := req.PublishInfo["devicePath"]
	if !ok {
		return nil, status.Error(codes.InvalidArgument, "Device path not provided")
	}

	// TODO: consider replacing IsLikelyNotMountPoint by IsNotMountPoint
	notMnt, err := d.mounter.Interface.IsLikelyNotMountPoint(target)
	if err != nil {
		if os.IsNotExist(err) {
			if errMkDir := d.mounter.Interface.MakeDir(target); errMkDir != nil {
				msg := fmt.Sprintf("could not create target dir %q: %v", target, errMkDir)
				return nil, status.Error(codes.Internal, msg)
			}
			notMnt = true
		} else {
			msg := fmt.Sprintf("could not determine if %q is valid mount point: %v", target, err)
			return nil, status.Error(codes.Internal, msg)
		}
	}

	if !notMnt {
		msg := fmt.Sprintf("target %q is not a valid mount point", target)
		return nil, status.Error(codes.InvalidArgument, msg)
	}
	// Get fs type that the volume will be formatted with
	attributes := req.GetVolumeAttributes()
	fsType, exists := attributes["fsType"]
	if !exists || fsType == "" {
		fsType = defaultFsType
	}

	// FormatAndMount will format only if needed
	glog.V(5).Infof("NodeStageVolume: formatting %s and mounting at %s", source, target)
	err = d.mounter.FormatAndMount(source, target, fsType, nil)
	if err != nil {
		msg := fmt.Sprintf("could not format %q and mount it at %q", source, target)
		return nil, status.Error(codes.Internal, msg)
	}

	return &csi.NodeStageVolumeResponse{}, nil
}

func (d *Driver) NodeUnstageVolume(ctx context.Context, req *csi.NodeUnstageVolumeRequest) (*csi.NodeUnstageVolumeResponse, error) {
	glog.V(4).Infof("NodeUnstageVolume: called with args %#v", req)
	volumeID := req.GetVolumeId()
	if len(volumeID) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Volume ID not provided")
	}

	target := req.GetStagingTargetPath()
	if len(target) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Staging target not provided")
	}

	glog.V(5).Infof("NodeUnstageVolume: unmounting %s", target)
	err := d.mounter.Interface.Unmount(target)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Could not unmount target %q: %v", target, err)
	}

	return &csi.NodeUnstageVolumeResponse{}, nil
}

func (d *Driver) NodePublishVolume(ctx context.Context, req *csi.NodePublishVolumeRequest) (*csi.NodePublishVolumeResponse, error) {
	glog.V(4).Infof("NodePublishVolume: called with args %#v", req)
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

	mountOptions := []string{"bind"}
	if req.GetReadonly() {
		mountOptions = append(mountOptions, "ro")
	}

	if m := volCap.GetMount(); m != nil {
		hasOption := func(options []string, opt string) bool {
			for _, o := range options {
				if o == opt {
					return true
				}
			}
			return false
		}
		for _, f := range m.MountFlags {
			if !hasOption(mountOptions, f) {
				mountOptions = append(mountOptions, f)
			}
		}
	}

	glog.V(5).Infof("NodePublishVolume: creating dir %s", target)
	if err := d.mounter.Interface.MakeDir(target); err != nil {
		return nil, status.Errorf(codes.Internal, "Could not create dir %q: %v", target, err)
	}

	glog.V(5).Infof("NodePublishVolume: mounting %s at %s", source, target)
	if err := d.mounter.Interface.Mount(source, target, "ext4", mountOptions); err != nil {
		os.Remove(target)
		return nil, status.Errorf(codes.Internal, "Could not mount %q at %q: %v", source, target, err)
	}

	return &csi.NodePublishVolumeResponse{}, nil
}

func (d *Driver) NodeUnpublishVolume(ctx context.Context, req *csi.NodeUnpublishVolumeRequest) (*csi.NodeUnpublishVolumeResponse, error) {
	glog.V(4).Infof("NodeUnpublishVolume: called with args %#v", req)
	volumeID := req.GetVolumeId()
	if len(volumeID) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Volume ID not provided")
	}

	target := req.GetTargetPath()
	if len(target) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Target path not provided")
	}

	glog.V(5).Infof("NodeUnpublishVolume: unmounting %s", target)
	err := d.mounter.Interface.Unmount(target)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Could not unmount %q: %v", target, err)
	}

	return &csi.NodeUnpublishVolumeResponse{}, nil
}

func (d *Driver) NodeGetCapabilities(ctx context.Context, req *csi.NodeGetCapabilitiesRequest) (*csi.NodeGetCapabilitiesResponse, error) {
	glog.V(4).Infof("NodeGetCapabilities: called with args %#v", req)
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
	glog.V(4).Infof("NodeGetInfo: called with args %#v", req)
	m := d.cloud.GetMetadata()

	topology := &csi.Topology{
		Segments: map[string]string{topologyKey: m.GetAvailabilityZone()},
	}

	return &csi.NodeGetInfoResponse{
		NodeId:             m.GetInstanceID(),
		AccessibleTopology: topology,
	}, nil
}

func (d *Driver) NodeGetId(ctx context.Context, req *csi.NodeGetIdRequest) (*csi.NodeGetIdResponse, error) {
	glog.V(4).Infof("NodeGetId: called with args %#v", req)
	m := d.cloud.GetMetadata()
	return &csi.NodeGetIdResponse{
		NodeId: m.GetInstanceID(),
	}, nil
}
