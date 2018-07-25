package main

import (
	"flag"

	"github.com/bertinatto/ebs-csi-driver/pkg/cloud"
	"github.com/bertinatto/ebs-csi-driver/pkg/driver"
	"github.com/golang/glog"
)

func main() {
	var endpoint = flag.String("endpoint", "unix://tmp/csi.sock", "CSI Endpoint")
	flag.Parse()

	cloud, err := cloud.NewCloud()
	if err != nil {
		glog.Fatalln(err)
	}

	m := cloud.GetMetadata()

	drv := driver.NewDriver(cloud, *endpoint, m.InstanceID)
	if err := drv.Run(); err != nil {
		glog.Fatalln(err)
	}
}
