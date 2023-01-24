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
	"net/http"

	flag "github.com/spf13/pflag"

	"github.com/kubernetes-sigs/aws-ebs-csi-driver/pkg/cloud"
	"github.com/kubernetes-sigs/aws-ebs-csi-driver/pkg/driver"
	logsapi "k8s.io/component-base/logs/api/v1"
	json "k8s.io/component-base/logs/json"
	"k8s.io/component-base/metrics/legacyregistry"

	"k8s.io/klog/v2"
)

func main() {
	fs := flag.NewFlagSet("aws-ebs-csi-driver", flag.ExitOnError)

	if err := logsapi.RegisterLogFormat(logsapi.JSONLogFormat, json.Factory{}, logsapi.LoggingBetaOptions); err != nil {
		klog.ErrorS(err, "failed to register JSON log format")
	}

	options := GetOptions(fs)

	cloud.RegisterMetrics()
	if options.ServerOptions.HttpEndpoint != "" {
		mux := http.NewServeMux()
		mux.Handle("/metrics", legacyregistry.HandlerWithReset())
		go func() {
			err := http.ListenAndServe(options.ServerOptions.HttpEndpoint, mux)
			if err != nil {
				klog.ErrorS(err, "failed to listen & serve metrics", "endpoint", options.ServerOptions.HttpEndpoint)
				klog.FlushAndExit(klog.ExitFlushTimeout, 1)
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
		driver.WithWarnOnInvalidTag(options.ControllerOptions.WarnOnInvalidTag),
		driver.WithVolumeHealthMonitoring(options.ControllerOptions.VolumeHealthMonitoring),
	)
	if err != nil {
		klog.ErrorS(err, "failed to create driver")
		klog.FlushAndExit(klog.ExitFlushTimeout, 1)
	}
	if err := drv.Run(); err != nil {
		klog.ErrorS(err, "failed to run driver")
		klog.FlushAndExit(klog.ExitFlushTimeout, 1)
	}
}
