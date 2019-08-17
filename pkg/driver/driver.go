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

	srv      *grpc.Server
	endpoint string
}

func NewDriver(endpoint string) (*Driver, error) {
	klog.Infof("Driver: %v Version: %v", DriverName, driverVersion)

	return &Driver{
		endpoint:          endpoint,
		controllerService: newControllerService(),
		nodeService:       newNodeService(),
	}, nil
}

func (d *Driver) Run() error {
	scheme, addr, err := util.ParseEndpoint(d.endpoint)
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
