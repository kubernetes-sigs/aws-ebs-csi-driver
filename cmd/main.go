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
	"net/http"

	"github.com/kubernetes-sigs/aws-ebs-csi-driver/pkg/cloud"
	"github.com/kubernetes-sigs/aws-ebs-csi-driver/pkg/driver"
	"k8s.io/component-base/metrics/legacyregistry"

	"k8s.io/klog"
)

func main() {
	fs := flag.NewFlagSet("aws-ebs-csi-driver", flag.ExitOnError)
	options := GetOptions(fs)

	cloud.RegisterMetrics()
	if options.ServerOptions.HttpEndpoint != "" {
		mux := http.NewServeMux()
		mux.Handle("/metrics", legacyregistry.HandlerWithReset())
		go func() {
			err := http.ListenAndServe(options.ServerOptions.HttpEndpoint, mux)
			if err != nil {
				klog.Fatalf("failed to listen & serve metrics from %q: %v", options.ServerOptions.HttpEndpoint, err)
			}
		}()
	}

	drv, err := driver.NewDriver(
		driver.WithEndpoint(options.ServerOptions.Endpoint),
		driver.WithExtraTags(options.ControllerOptions.ExtraTags),
		driver.WithExtraVolumeTags(options.ControllerOptions.ExtraVolumeTags),
		driver.WithMode(options.DriverMode),
		driver.WithVolumeAttachLimit(options.NodeOptions.VolumeAttachLimit),
		driver.WithKubernetesClusterID(options.ControllerOptions.KubernetesClusterID),
		driver.WithAwsSdkDebugLog(options.ControllerOptions.AwsSdkDebugLog),
	)
	if err != nil {
		klog.Fatalln(err)
	}
	if err := drv.Run(); err != nil {
		klog.Fatalln(err)
	}
}
