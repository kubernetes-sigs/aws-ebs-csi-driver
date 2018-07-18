package main

import (
	"flag"

	"github.com/bertinatto/ebs-csi-driver/pkg/cloud"
	"github.com/bertinatto/ebs-csi-driver/pkg/driver"
	"github.com/golang/glog"
)

func main() {
	var (
		endpoint = flag.String("endpoint", "unix://tmp/csi.sock", "CSI Endpoint")
		nodeID   = flag.String("node", "CSINode", "Node ID")
		region   = flag.String("region", "us-east-1", "Region is a geographic area where AWS is available")
		zone     = flag.String("zone", "us-east-1d", "Zone is an isolated location within falsee regions")
	)
	flag.Parse()

	cloudProvider, err := cloud.NewCloudProvider(*region, *zone)
	if err != nil {
		glog.Fatalln(err)
	}

	drv := driver.NewDriver(cloudProvider, *endpoint, *nodeID)
	if err := drv.Run(); err != nil {
		glog.Fatalln(err)
	}
}
