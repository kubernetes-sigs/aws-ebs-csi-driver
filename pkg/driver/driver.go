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

	csi "github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/kubernetes-sigs/aws-ebs-csi-driver/pkg/util"
	"google.golang.org/grpc"
	"k8s.io/klog"
)

const (
	DriverName  = "ebs.csi.aws.com"
	TopologyKey = "topology." + DriverName + "/zone"
)

type Driver struct {
	controllerService
	nodeService

	srv     *grpc.Server
	options *DriverOptions
}

type DriverOptions struct {
	endpoint        string
	extraVolumeTags map[string]string
}

func NewDriver(options ...func(*DriverOptions)) (*Driver, error) {
	klog.Infof("Driver: %v Version: %v", DriverName, driverVersion)

	driverOptions := DriverOptions{
		endpoint: DefaultCSIEndpoint,
	}
	for _, option := range options {
		option(&driverOptions)
	}

	if err := ValidateDriverOptions(&driverOptions); err != nil {
		return nil, fmt.Errorf("Invalid driver options: %v", err)
	}

	driver := Driver{
		controllerService: newControllerService(&driverOptions),
		nodeService:       newNodeService(),
		options:           &driverOptions,
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
			klog.Errorf("GRPC error: %v", err)
		}
		return resp, err
	}
	opts := []grpc.ServerOption{
		grpc.UnaryInterceptor(logErr),
	}
	d.srv = grpc.NewServer(opts...)

	csi.RegisterIdentityServer(d.srv, d)
	csi.RegisterControllerServer(d.srv, d)
	csi.RegisterNodeServer(d.srv, d)

	klog.Infof("Listening for connections on address: %#v", listener.Addr())
	return d.srv.Serve(listener)
}

func (d *Driver) Stop() {
	klog.Infof("Stopping server")
	d.srv.Stop()
}

func WithEndpoint(endpoint string) func(*DriverOptions) {
	return func(o *DriverOptions) {
		o.endpoint = endpoint
	}
}

func WithExtraVolumeTags(extraVolumeTags map[string]string) func(*DriverOptions) {
	return func(o *DriverOptions) {
		o.extraVolumeTags = extraVolumeTags
	}
}
