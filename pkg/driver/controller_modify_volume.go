package driver

import (
	"context"
	"strconv"

	"github.com/awslabs/volume-modifier-for-k8s/pkg/rpc"
	"github.com/kubernetes-sigs/aws-ebs-csi-driver/pkg/cloud"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"k8s.io/klog/v2"
)

const (
	ModificationKeyVolumeType = "volumeType"

	ModificationKeyIOPS = "iops"

	ModificationKeyThroughput = "throughput"
)

func (d *controllerService) GetCSIDriverModificationCapability(
	_ context.Context,
	_ *rpc.GetCSIDriverModificationCapabilityRequest,
) (*rpc.GetCSIDriverModificationCapabilityResponse, error) {
	return &rpc.GetCSIDriverModificationCapabilityResponse{}, nil
}

func (d *controllerService) ModifyVolumeProperties(
	ctx context.Context,
	req *rpc.ModifyVolumePropertiesRequest,
) (*rpc.ModifyVolumePropertiesResponse, error) {
	klog.V(4).InfoS("ModifyVolumeAttributes called", "req", req)
	if err := validateModifyVolumeAttributesRequest(req); err != nil {
		return nil, err
	}

	name := req.GetName()
	modifyOptions := cloud.ModifyDiskOptions{}
	for key, value := range req.GetParameters() {
		switch key {
		case ModificationKeyIOPS:
			iops, err := strconv.Atoi(value)
			if err != nil {
				return nil, status.Errorf(codes.InvalidArgument, "Could not parse IOPS: %q", value)
			}
			modifyOptions.IOPS = iops
		case ModificationKeyThroughput:
			throughput, err := strconv.Atoi(value)
			if err != nil {
				return nil, status.Errorf(codes.InvalidArgument, "Could not parse throughput: %q", value)
			}
			modifyOptions.Throughput = throughput
		case ModificationKeyVolumeType:
			modifyOptions.VolumeType = value
		}
	}
	if err := d.cloud.ModifyDisk(ctx, name, &modifyOptions); err != nil {
		return nil, status.Errorf(codes.Internal, "Could not modify volume %q: %v", name, err)
	}
	return &rpc.ModifyVolumePropertiesResponse{}, nil
}

func validateModifyVolumeAttributesRequest(req *rpc.ModifyVolumePropertiesRequest) error {
	name := req.GetName()
	if name == "" {
		return status.Error(codes.InvalidArgument, "Volume name not provided")
	}
	return nil
}
