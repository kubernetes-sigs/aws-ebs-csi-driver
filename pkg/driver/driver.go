package driver

import (
	"context"
	"net"

	"github.com/bertinatto/ebs-csi-driver/pkg/cloud"
	"github.com/bertinatto/ebs-csi-driver/pkg/util"
	csi "github.com/container-storage-interface/spec/lib/go/csi/v0"
	"github.com/golang/glog"
	"google.golang.org/grpc"
)

const (
	driverName    = "com.amazon.aws.csi.ebs"
	vendorVersion = "0.0.1" // FIXME
)

type Driver struct {
	endpoint string
	nodeID   string

	cloud cloud.CloudProvider
	srv   *grpc.Server
}

func NewDriver(cloud cloud.CloudProvider, endpoint, nodeID string) *Driver {
	glog.Infof("Driver: %v version: %v", driverName)
	return &Driver{
		endpoint: endpoint,
		nodeID:   nodeID,
		cloud:    cloud,
	}
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
			glog.Errorf("GRPC error: %v", err)
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

	glog.Infof("Listening for connections on address: %#v", listener.Addr())
	return d.srv.Serve(listener)
}

func (d *Driver) Stop() {
	glog.Infof("Stopping server")
	d.srv.Stop()
}
