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

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/sts"
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

const (
	// LabelRefreshTime is the time in minutes that it takes for node labels to update volume and ENI count.
	LabelRefreshTime = 60
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
		clientset, _, clientErr := metadata.DefaultKubernetesAPIClient(options.Kubeconfig)()
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

	// TODO question: I think it makes sense to check if cmd == "patcher" here because this is where the other
	// cmd checks are. However, this would result in a lot of code duplication as the patcher container needs
	// metadata (region and instance ID) to run.
	case "patcher":
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

		cfg := metadata.MetadataServiceConfig{
			MetadataSources: options.MetadataSources,
			IMDSClient:      metadata.DefaultIMDSClient,
			K8sAPIClient:    metadata.DefaultKubernetesAPIClient(options.Kubeconfig),
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

		cloud, err := cloud.NewCloud(region, options.AwsSdkDebugLog, options.UserAgentExtra, options.Batching, options.DeprecatedMetrics)
		if err != nil {
			klog.ErrorS(err, "failed to create cloud service")
			klog.FlushAndExit(klog.ExitFlushTimeout, 1)
		}

		k8sClient, k8sConfig, err := cfg.K8sAPIClient()
		if err != nil {
			klog.V(2).InfoS("Failed to setup k8s client", "err", err)
		}
		metadata.ContinuousUpdateLabelsLeaderElection(k8sClient, k8sConfig, md.GetInstanceID(), cloud, LabelRefreshTime)
		klog.FlushAndExit(klog.ExitFlushTimeout, 0)
	default:
		klog.Errorf("Unknown driver mode %s: Expected %s, %s, %s, patcher, or pre-stop-hook", cmd, driver.ControllerMode, driver.NodeMode, driver.AllMode)
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
		//nolint:forbidigo // Print version info without klog/timestamp
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

	cfg := metadata.MetadataServiceConfig{
		MetadataSources: options.MetadataSources,
		IMDSClient:      metadata.DefaultIMDSClient,
		K8sAPIClient:    metadata.DefaultKubernetesAPIClient(options.Kubeconfig),
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

	if options.HTTPEndpoint != "" {
		r := metrics.InitializeRecorder(options.DeprecatedMetrics)
		r.InitializeMetricsHandler(options.HTTPEndpoint, "/metrics", options.MetricsCertFile, options.MetricsKeyFile)

		if options.Mode == driver.ControllerMode || options.Mode == driver.AllMode {
			// TODO inject metrics in cloud for clean unit tests
			r.InitializeAPIMetrics(options.DeprecatedMetrics)
			r.InitializeAsyncEC2Metrics(60 * time.Second /* Don't emit metrics for detaches that take < 60s */)
		}
		if options.Mode == driver.NodeMode || options.Mode == driver.AllMode {
			r.InitializeNVME(options.CsiMountPointPath, md.GetInstanceID())
		}
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

	var accountID string
	if options.Mode == driver.ControllerMode || options.Mode == driver.AllMode {
		cfg, err := config.LoadDefaultConfig(context.Background(), config.WithRegion(region))
		if err != nil {
			klog.ErrorS(err, "Failed to create AWS config for account ID retrieval")
			klog.FlushAndExit(klog.ExitFlushTimeout, 1)
		}

		stsClient := sts.NewFromConfig(cfg)
		resp, err := stsClient.GetCallerIdentity(context.Background(), &sts.GetCallerIdentityInput{})
		if err != nil {
			klog.ErrorS(err, "Failed to get AWS account ID, HyperPod functionality may not work")
			// Continue without account ID - existing functionality should still work
		} else {
			accountID = *resp.Account
			klog.V(5).InfoS("Retrieved AWS account ID for HyperPod operations", "accountID", accountID)
		}
	}

	cloud, err := cloud.NewCloud(region, accountID, options.AwsSdkDebugLog, options.UserAgentExtra, options.Batching, options.DeprecatedMetrics)
	if err != nil {
		klog.ErrorS(err, "failed to create cloud service")
		klog.FlushAndExit(klog.ExitFlushTimeout, 1)
	}

	m, err := mounter.NewNodeMounter(options.WindowsHostProcess)
	if err != nil {
		klog.ErrorS(err, "failed to create node mounter")
		klog.FlushAndExit(klog.ExitFlushTimeout, 1)
	}

	k8sClient, _, err := cfg.K8sAPIClient()
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
