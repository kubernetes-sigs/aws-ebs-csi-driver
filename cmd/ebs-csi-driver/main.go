package main

import (
	"flag"
	"log"

	"github.com/bertinatto/ebs-csi-driver/pkg/cloud"
	"github.com/bertinatto/ebs-csi-driver/pkg/driver"
)

func main() {
	var (
		endpoint = flag.String("endpoint", "unix://tmp/csi.sock", "CSI Endpoint")
		nodeID   = flag.String("node", "CSINode", "Node ID")
	)
	flag.Parse()

	cloudProvider, err := cloud.NewCloudProvider()
	if err != nil {
		log.Fatalln(err)
	}

	drv := driver.NewDriver(cloudProvider, *endpoint, *nodeID)
	if err := drv.Run(); err != nil {
		log.Fatalln(err)
	}
}
