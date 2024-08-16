/*
Copyright 2024 The Kubernetes Authors.

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
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/kubernetes-sigs/aws-ebs-csi-driver/cmd/hooks"
	"github.com/kubernetes-sigs/aws-ebs-csi-driver/pkg/cloud"
	"github.com/kubernetes-sigs/aws-ebs-csi-driver/pkg/cloud/metadata"
	"github.com/kubernetes-sigs/aws-ebs-csi-driver/pkg/driver"
	"github.com/kubernetes-sigs/aws-ebs-csi-driver/pkg/metrics"
	"github.com/kubernetes-sigs/aws-ebs-csi-driver/pkg/mounter"
	flag "github.com/spf13/pflag"
	"k8s.io/component-base/featuregate"
	logsapi "k8s.io/component-base/logs/api/v1"
	json "k8s.io/component-base/logs/json"
	"k8s.io/klog/v2"
)

var (
	featureGate = featuregate.NewFeatureGate()
)

func main() {
	fs := flag.NewFlagSet("aws-ebs-csi-driver", flag.ExitOnError)
	if err := logsapi.RegisterLogFormat(logsapi.JSONLogFormat, json.Factory{}, logsapi.LoggingBetaOptions); err != nil {
		klog.ErrorS(err, "failed to register JSON log format")
	}

	var (
		version  = fs.Bool("version", false, "Print the version and exit.")
		toStderr = fs.Bool("logtostderr", false, "log to standard error instead of files. DEPRECATED: will be removed in a future release.")
		args     = os.Args[1:]
		cmd      = string(driver.AllMode)
		options  = driver.Options{}
	)

	c := logsapi.NewLoggingConfiguration()
	err := logsapi.AddFeatureGates(featureGate)
	if err != nil {
		klog.ErrorS(err, "failed to add feature gates")
	}
	logsapi.AddFlags(c, fs)

	if len(os.Args) > 1 && !strings.HasPrefix(os.Args[1], "-") {
		cmd = os.Args[1]
		args = os.Args[2:]
	}

	switch cmd {
	case "pre-stop-hook":
		clientset, clientErr := metadata.DefaultKubernetesAPIClient(options.Kubeconfig)()
		if clientErr != nil {
			klog.ErrorS(err, "unable to communicate with k8s API")
		} else {
			err = hooks.PreStop(clientset)
			if err != nil {
				klog.ErrorS(err, "failed to execute PreStop lifecycle hook")
				klog.FlushAndExit(klog.ExitFlushTimeout, 1)
			}
		}
		klog.FlushAndExit(klog.ExitFlushTimeout, 0)
	case string(driver.ControllerMode), string(driver.NodeMode), string(driver.AllMode):
		options.Mode = driver.Mode(cmd)
	default:
		klog.Errorf("Unknown driver mode %s: Expected %s, %s, %s, or pre-stop-hook", cmd, driver.ControllerMode, driver.NodeMode, driver.AllMode)
		klog.FlushAndExit(klog.ExitFlushTimeout, 0)
	}

	options.AddFlags(fs)

	if err = fs.Parse(args); err != nil {
		klog.ErrorS(err, "Failed to parse options")
		klog.FlushAndExit(klog.ExitFlushTimeout, 0)
	}
	if err = options.Validate(); err != nil {
		klog.ErrorS(err, "Invalid options")
		klog.FlushAndExit(klog.ExitFlushTimeout, 0)
	}

	err = logsapi.ValidateAndApply(c, featureGate)
	if err != nil {
		klog.ErrorS(err, "failed to validate and apply logging configuration")
	}

	if *version {
		versionInfo, versionErr := driver.GetVersionJSON()
		if versionErr != nil {
			klog.ErrorS(err, "failed to get version")
			klog.FlushAndExit(klog.ExitFlushTimeout, 1)
		}
		fmt.Println(versionInfo)
		os.Exit(0)
	}

	if *toStderr {
		klog.SetOutput(os.Stderr)
	}

	// Start tracing as soon as possible
	if options.EnableOtelTracing {
		exporter, exporterErr := driver.InitOtelTracing()
		if exporterErr != nil {
			klog.ErrorS(err, "failed to initialize otel tracing")
			klog.FlushAndExit(klog.ExitFlushTimeout, 1)
		}
		// Exporter will flush traces on shutdown
		defer func() {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			if shutdownErr := exporter.Shutdown(ctx); shutdownErr != nil {
				klog.ErrorS(exporterErr, "could not shutdown otel exporter")
			}
		}()
	}

	if options.HttpEndpoint != "" {
		r := metrics.InitializeRecorder()
		r.InitializeMetricsHandler(options.HttpEndpoint, "/metrics", options.MetricsCertFile, options.MetricsKeyFile)
	}

	cfg := metadata.MetadataServiceConfig{
		EC2MetadataClient: metadata.DefaultEC2MetadataClient,
		K8sAPIClient:      metadata.DefaultKubernetesAPIClient(options.Kubeconfig),
	}

	region := os.Getenv("AWS_REGION")
	var md metadata.MetadataService
	var metadataErr error

	if region != "" {
		klog.InfoS("Region provided via AWS_REGION environment variable", "region", region)
		if options.Mode != driver.ControllerMode {
			klog.InfoS("Node service requires metadata even if AWS_REGION provided, initializing metadata")
			md, metadataErr = metadata.NewMetadataService(cfg, region)
		}
	} else {
		klog.InfoS("Initializing metadata")
		md, metadataErr = metadata.NewMetadataService(cfg, region)
	}

	if metadataErr != nil {
		klog.ErrorS(metadataErr, "Failed to initialize metadata when it is required")
		if options.Mode == driver.ControllerMode {
			klog.InfoS("The region can be manually supplied via the AWS_REGION environment variable")
		}
		klog.FlushAndExit(klog.ExitFlushTimeout, 1)
	} else if region == "" {
		region = md.GetRegion()
	}

	cloud, err := cloud.NewCloud(region, options.AwsSdkDebugLog, options.UserAgentExtra, options.Batching, options.RoleARN)
	if err != nil {
		klog.ErrorS(err, "failed to create cloud service")
		klog.FlushAndExit(klog.ExitFlushTimeout, 1)
	}

	m, err := mounter.NewNodeMounter(options.WindowsHostProcess)
	if err != nil {
		klog.ErrorS(err, "failed to create node mounter")
		klog.FlushAndExit(klog.ExitFlushTimeout, 1)
	}

	k8sClient, err := cfg.K8sAPIClient()
	if err != nil {
		klog.V(2).InfoS("Failed to setup k8s client", "err", err)
	}

	drv, err := driver.NewDriver(cloud, &options, m, md, k8sClient)
	if err != nil {
		klog.ErrorS(err, "failed to create driver")
		klog.FlushAndExit(klog.ExitFlushTimeout, 1)
	}
	if err := drv.Run(); err != nil {
		klog.ErrorS(err, "failed to run driver")
		klog.FlushAndExit(klog.ExitFlushTimeout, 1)
	}
}
