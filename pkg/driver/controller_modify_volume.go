/*
Copyright 2024 The Kubernetes Authors.

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
	"time"

	"github.com/awslabs/volume-modifier-for-k8s/pkg/rpc"
	"github.com/kubernetes-sigs/aws-ebs-csi-driver/pkg/cloud"
	"github.com/kubernetes-sigs/aws-ebs-csi-driver/pkg/coalescer"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"k8s.io/klog/v2"
)

const (
	ModificationKeyVolumeType = "type"
	// Retained for backwards compatibility, but not recommended
	DeprecatedModificationKeyVolumeType = "volumeType"

	ModificationKeyIOPS = "iops"

	ModificationKeyThroughput = "throughput"
)

type modifyVolumeRequest struct {
	newSize           int64
	modifyDiskOptions cloud.ModifyDiskOptions
}

func (d *ControllerService) GetCSIDriverModificationCapability(
	_ context.Context,
	_ *rpc.GetCSIDriverModificationCapabilityRequest,
) (*rpc.GetCSIDriverModificationCapabilityResponse, error) {
	return &rpc.GetCSIDriverModificationCapabilityResponse{}, nil
}

func (d *ControllerService) ModifyVolumeProperties(
	ctx context.Context,
	req *rpc.ModifyVolumePropertiesRequest,
) (*rpc.ModifyVolumePropertiesResponse, error) {
	klog.V(4).InfoS("ModifyVolumeProperties called", "req", req)
	name := req.GetName()
	if name == "" {
		return nil, status.Error(codes.InvalidArgument, "Volume name not provided")
	}

	options, err := parseModifyVolumeParameters(req.GetParameters())
	if err != nil {
		return nil, err
	}

	_, err = d.modifyVolumeCoalescer.Coalesce(name, modifyVolumeRequest{
		modifyDiskOptions: *options,
	})
	if err != nil {
		return nil, err
	}

	return &rpc.ModifyVolumePropertiesResponse{}, nil
}

func newModifyVolumeCoalescer(c cloud.Cloud, o *Options) coalescer.Coalescer[modifyVolumeRequest, int32] {
	return coalescer.New[modifyVolumeRequest, int32](o.ModifyVolumeRequestHandlerTimeout, mergeModifyVolumeRequest, executeModifyVolumeRequest(c))
}

func mergeModifyVolumeRequest(input modifyVolumeRequest, existing modifyVolumeRequest) (modifyVolumeRequest, error) {
	if input.newSize != 0 {
		if existing.newSize != 0 && input.newSize != existing.newSize {
			return existing, fmt.Errorf("Different size was requested by a previous request. Current: %d, Requested: %d", existing.newSize, input.newSize)
		}
		existing.newSize = input.newSize
	}
	if input.modifyDiskOptions.IOPS != 0 {
		if existing.modifyDiskOptions.IOPS != 0 && input.modifyDiskOptions.IOPS != existing.modifyDiskOptions.IOPS {
			return existing, fmt.Errorf("Different IOPS was requested by a previous request. Current: %d, Requested: %d", existing.modifyDiskOptions.IOPS, input.modifyDiskOptions.IOPS)
		}
		existing.modifyDiskOptions.IOPS = input.modifyDiskOptions.IOPS
	}
	if input.modifyDiskOptions.Throughput != 0 {
		if existing.modifyDiskOptions.Throughput != 0 && input.modifyDiskOptions.Throughput != existing.modifyDiskOptions.Throughput {
			return existing, fmt.Errorf("Different throughput was requested by a previous request. Current: %d, Requested: %d", existing.modifyDiskOptions.Throughput, input.modifyDiskOptions.Throughput)
		}
		existing.modifyDiskOptions.Throughput = input.modifyDiskOptions.Throughput
	}
	if input.modifyDiskOptions.VolumeType != "" {
		if existing.modifyDiskOptions.VolumeType != "" && input.modifyDiskOptions.VolumeType != existing.modifyDiskOptions.VolumeType {
			return existing, fmt.Errorf("Different volume type was requested by a previous request. Current: %s, Requested: %s", existing.modifyDiskOptions.VolumeType, input.modifyDiskOptions.VolumeType)
		}
		existing.modifyDiskOptions.VolumeType = input.modifyDiskOptions.VolumeType
	}

	return existing, nil
}

func executeModifyVolumeRequest(c cloud.Cloud) func(string, modifyVolumeRequest) (int32, error) {
	return func(volumeID string, req modifyVolumeRequest) (int32, error) {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		actualSizeGiB, err := c.ResizeOrModifyDisk(ctx, volumeID, req.newSize, &req.modifyDiskOptions)
		if err != nil {
			// Kubernetes sidecars treats "Invalid Argument" errors as infeasible and retries less aggressively
			if errors.Is(err, cloud.ErrInvalidArgument) {
				return 0, status.Errorf(codes.InvalidArgument, "Could not modify volume (invalid argument) %q: %v", volumeID, err)
			}
			return 0, status.Errorf(codes.Internal, "Could not modify volume %q: %v", volumeID, err)
		} else {
			return actualSizeGiB, nil
		}
	}
}

func parseModifyVolumeParameters(params map[string]string) (*cloud.ModifyDiskOptions, error) {
	options := cloud.ModifyDiskOptions{}

	for key, value := range params {
		switch key {
		case ModificationKeyIOPS:
			iops, err := strconv.Atoi(value)
			if err != nil {
				return nil, status.Errorf(codes.InvalidArgument, "Could not parse IOPS: %q", value)
			}
			options.IOPS = int32(iops)
		case ModificationKeyThroughput:
			throughput, err := strconv.Atoi(value)
			if err != nil {
				return nil, status.Errorf(codes.InvalidArgument, "Could not parse throughput: %q", value)
			}
			options.Throughput = int32(throughput)
		case DeprecatedModificationKeyVolumeType:
			if _, ok := params[ModificationKeyVolumeType]; ok {
				klog.Infof("Ignoring deprecated key `volumeType` because preferred key `type` is present")
				continue
			}
			klog.InfoS("Key `volumeType` is deprecated, please use `type` instead")
			options.VolumeType = value
		case ModificationKeyVolumeType:
			options.VolumeType = value
		}
	}

	return &options, nil
}
