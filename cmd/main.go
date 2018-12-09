/*
Copyright 2018 The Kubernetes Authors.

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

package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/golang/glog"
	"github.com/kubernetes-sigs/aws-ebs-csi-driver/pkg/driver"
)

func main() {
	var (
		endpoint = flag.String("endpoint", "unix://tmp/csi.sock", "CSI Endpoint")
		version  = flag.Bool("version", false, "Print the version and exit.")
	)
	flag.Parse()

	if *version {
		info, err := driver.GetVersionJSON()
		if err != nil {
			glog.Fatalln(err)
		}
		fmt.Println(info)
		os.Exit(0)
	}

	drv, err := driver.NewDriver(*endpoint)
	if err != nil {
		glog.Fatalln(err)
	}
	if err := drv.Run(); err != nil {
		glog.Fatalln(err)
	}
}
