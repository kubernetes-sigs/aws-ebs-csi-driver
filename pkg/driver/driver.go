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
	"github.com/kubernetes-sigs/aws-ebs-csi-driver/pkg/cloud"
	"github.com/kubernetes-sigs/aws-ebs-csi-driver/pkg/cloud/metadata"
	"github.com/kubernetes-sigs/aws-ebs-csi-driver/pkg/mounter"
	"github.com/kubernetes-sigs/aws-ebs-csi-driver/pkg/util"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"google.golang.org/grpc"
	"k8s.io/client-go/kubernetes"
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
	DriverName               = "ebs.csi.aws.com"
	AwsPartitionKey          = "topology." + DriverName + "/partition"
	AwsAccountIDKey          = "topology." + DriverName + "/account-id"
	AwsRegionKey             = "topology." + DriverName + "/region"
	AwsOutpostIDKey          = "topology." + DriverName + "/outpost-id"
	WellKnownZoneTopologyKey = "topology.kubernetes.io/zone"
	// DEPRECATED Use the WellKnownZoneTopologyKey instead
	ZoneTopologyKey = "topology." + DriverName + "/zone"
	OSTopologyKey   = "kubernetes.io/os"
)

type Driver struct {
	controller *ControllerService
	node       *NodeService
	srv        *grpc.Server
	options    *Options
	csi.UnimplementedIdentityServer
}

func NewDriver(c cloud.Cloud, o *Options, m mounter.Mounter, md metadata.MetadataService, k kubernetes.Interface) (*Driver, error) {
	klog.InfoS("Driver Information", "Driver", DriverName, "Version", driverVersion)

	if err := ValidateDriverOptions(o); err != nil {
		return nil, fmt.Errorf("invalid driver options: %w", err)
	}

	driver := &Driver{
		options: o,
	}

	switch o.Mode {
	case ControllerMode:
		driver.controller = NewControllerService(c, o)
	case NodeMode:
		driver.node = NewNodeService(o, md, m, k)
	case AllMode:
		driver.controller = NewControllerService(c, o)
		driver.node = NewNodeService(o, md, m, k)
	default:
		return nil, fmt.Errorf("unknown mode: %s", o.Mode)
	}

	return driver, nil
}

func (d *Driver) Run() error {
	scheme, addr, err := util.ParseEndpoint(d.options.Endpoint, d.options.WindowsHostProcess)
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

	if d.options.EnableOtelTracing {
		opts = append(opts, grpc.StatsHandler(otelgrpc.NewServerHandler()))
	}

	d.srv = grpc.NewServer(opts...)
	csi.RegisterIdentityServer(d.srv, d)

	switch d.options.Mode {
	case ControllerMode:
		csi.RegisterControllerServer(d.srv, d.controller)
		rpc.RegisterModifyServer(d.srv, d.controller)
	case NodeMode:
		csi.RegisterNodeServer(d.srv, d.node)
	case AllMode:
		csi.RegisterControllerServer(d.srv, d.controller)
		csi.RegisterNodeServer(d.srv, d.node)
		rpc.RegisterModifyServer(d.srv, d.controller)
	default:
		return fmt.Errorf("unknown mode: %s", d.options.Mode)
	}

	klog.V(4).InfoS("Listening for connections", "address", listener.Addr())
	return d.srv.Serve(listener)
}

func (d *Driver) Stop() {
	d.srv.Stop()
}
