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

package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/kubernetes-sigs/aws-ebs-csi-driver/pkg/driver"
	cliflag "k8s.io/component-base/cli/flag"
	"k8s.io/klog"
)

func main() {
	var (
		version         bool
		endpoint        string
		extraVolumeTags map[string]string
	)

	flag.BoolVar(&version, "version", false, "Print the version and exit.")
	flag.StringVar(&endpoint, "endpoint", driver.DefaultCSIEndpoint, "CSI Endpoint")
	flag.Var(cliflag.NewMapStringString(&extraVolumeTags), "extra-volume-tags", "Extra volume tags to attach to each dynamically provisioned volume. It is a comma separated list of key value pairs like '<key1>=<value1>,<key2>=<value2>'")

	klog.InitFlags(nil)
	flag.Parse()

	if version {
		info, err := driver.GetVersionJSON()
		if err != nil {
			klog.Fatalln(err)
		}
		fmt.Println(info)
		os.Exit(0)
	}

	drv, err := driver.NewDriver(
		driver.WithEndpoint(endpoint),
		driver.WithExtraVolumeTags(extraVolumeTags),
	)
	if err != nil {
		klog.Fatalln(err)
	}
	if err := drv.Run(); err != nil {
		klog.Fatalln(err)
	}
}
