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
	"fmt"
	"net"

	"github.com/awslabs/volume-modifier-for-k8s/pkg/rpc"
	csi "github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/kubernetes-sigs/aws-ebs-csi-driver/pkg/util"
	"google.golang.org/grpc"
	"k8s.io/klog/v2"
)

// Mode is the operating mode of the CSI driver.
type Mode string

const (
	// ControllerMode is the mode that only starts the controller service.
	ControllerMode Mode = "controller"
	// NodeMode is the mode that only starts the node service.
	NodeMode Mode = "node"
	// AllMode is the mode that only starts both the controller and the node service.
	AllMode Mode = "all"
)

const (
	DriverName      = "ebs.csi.aws.com"
	AwsPartitionKey = "topology." + DriverName + "/partition"
	AwsAccountIDKey = "topology." + DriverName + "/account-id"
	AwsRegionKey    = "topology." + DriverName + "/region"
	AwsOutpostIDKey = "topology." + DriverName + "/outpost-id"

	WellKnownTopologyKey = "topology.kubernetes.io/zone"
	// DEPRECATED Use the WellKnownTopologyKey instead
	TopologyKey = "topology." + DriverName + "/zone"
)

type Driver struct {
	controllerService
	nodeService

	srv     *grpc.Server
	options *DriverOptions
}

type DriverOptions struct {
	endpoint            string
	extraTags           map[string]string
	mode                Mode
	volumeAttachLimit   int64
	kubernetesClusterID string
	awsSdkDebugLog      bool
	batching            bool
	warnOnInvalidTag    bool
	userAgentExtra      string
}

func NewDriver(options ...func(*DriverOptions)) (*Driver, error) {
	klog.InfoS("Driver Information", "Driver", DriverName, "Version", driverVersion)

	driverOptions := DriverOptions{
		endpoint: DefaultCSIEndpoint,
		mode:     AllMode,
	}
	for _, option := range options {
		option(&driverOptions)
	}

	if err := ValidateDriverOptions(&driverOptions); err != nil {
		return nil, fmt.Errorf("Invalid driver options: %w", err)
	}

	driver := Driver{
		options: &driverOptions,
	}

	switch driverOptions.mode {
	case ControllerMode:
		driver.controllerService = newControllerService(&driverOptions)
	case NodeMode:
		driver.nodeService = newNodeService(&driverOptions)
	case AllMode:
		driver.controllerService = newControllerService(&driverOptions)
		driver.nodeService = newNodeService(&driverOptions)
	default:
		return nil, fmt.Errorf("unknown mode: %s", driverOptions.mode)
	}

	return &driver, nil
}

func (d *Driver) Run() error {
	scheme, addr, err := util.ParseEndpoint(d.options.endpoint)
	if err != nil {
		return err
	}

	listener, err := net.Listen(scheme, addr)
	if err != nil {
		return err
	}

	logErr := func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		resp, err := handler(ctx, req)
		if err != nil {
			klog.ErrorS(err, "GRPC error")
		}
		return resp, err
	}
	opts := []grpc.ServerOption{
		grpc.UnaryInterceptor(logErr),
	}
	d.srv = grpc.NewServer(opts...)

	csi.RegisterIdentityServer(d.srv, d)

	switch d.options.mode {
	case ControllerMode:
		csi.RegisterControllerServer(d.srv, d)
		rpc.RegisterModifyServer(d.srv, d)
	case NodeMode:
		csi.RegisterNodeServer(d.srv, d)
	case AllMode:
		csi.RegisterControllerServer(d.srv, d)
		csi.RegisterNodeServer(d.srv, d)
		rpc.RegisterModifyServer(d.srv, d)
	default:
		return fmt.Errorf("unknown mode: %s", d.options.mode)
	}

	klog.V(4).InfoS("Listening for connections", "address", listener.Addr())
	return d.srv.Serve(listener)
}

func (d *Driver) Stop() {
	d.srv.Stop()
}

func WithEndpoint(endpoint string) func(*DriverOptions) {
	return func(o *DriverOptions) {
		o.endpoint = endpoint
	}
}

func WithExtraTags(extraTags map[string]string) func(*DriverOptions) {
	return func(o *DriverOptions) {
		o.extraTags = extraTags
	}
}

func WithExtraVolumeTags(extraVolumeTags map[string]string) func(*DriverOptions) {
	return func(o *DriverOptions) {
		if o.extraTags == nil && extraVolumeTags != nil {
			klog.InfoS("DEPRECATION WARNING: --extra-volume-tags is deprecated, please use --extra-tags instead")
			o.extraTags = extraVolumeTags
		}
	}
}

func WithMode(mode Mode) func(*DriverOptions) {
	return func(o *DriverOptions) {
		o.mode = mode
	}
}

func WithVolumeAttachLimit(volumeAttachLimit int64) func(*DriverOptions) {
	return func(o *DriverOptions) {
		o.volumeAttachLimit = volumeAttachLimit
	}
}

func WithBatching(enableBatching bool) func(*DriverOptions) {
	return func(o *DriverOptions) {
		o.batching = enableBatching
	}
}

func WithKubernetesClusterID(clusterID string) func(*DriverOptions) {
	return func(o *DriverOptions) {
		o.kubernetesClusterID = clusterID
	}
}

func WithAwsSdkDebugLog(enableSdkDebugLog bool) func(*DriverOptions) {
	return func(o *DriverOptions) {
		o.awsSdkDebugLog = enableSdkDebugLog
	}
}

func WithWarnOnInvalidTag(warnOnInvalidTag bool) func(*DriverOptions) {
	return func(o *DriverOptions) {
		o.warnOnInvalidTag = warnOnInvalidTag
	}
}

func WithUserAgentExtra(userAgentExtra string) func(*DriverOptions) {
	return func(o *DriverOptions) {
		o.userAgentExtra = userAgentExtra
	}
}
