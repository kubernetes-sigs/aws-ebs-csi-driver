/*
Copyright 2020 The Kubernetes Authors.

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
	"fmt"
	"os"
	"strings"

	flag "github.com/spf13/pflag"

	"github.com/kubernetes-sigs/aws-ebs-csi-driver/cmd/hooks"
	"github.com/kubernetes-sigs/aws-ebs-csi-driver/cmd/options"
	"github.com/kubernetes-sigs/aws-ebs-csi-driver/pkg/cloud"
	"github.com/kubernetes-sigs/aws-ebs-csi-driver/pkg/driver"

	"k8s.io/component-base/featuregate"
	logsapi "k8s.io/component-base/logs/api/v1"
	"k8s.io/klog/v2"
)

// Options is the combined set of options for all operating modes.
type Options struct {
	DriverMode driver.Mode

	*options.ServerOptions
	*options.ControllerOptions
	*options.NodeOptions
}

// used for testing
var osExit = os.Exit

var featureGate = featuregate.NewFeatureGate()

// GetOptions parses the command line options and returns a struct that contains
// the parsed options.
func GetOptions(fs *flag.FlagSet) *Options {
	var (
		version  = fs.Bool("version", false, "Print the version and exit.")
		toStderr = fs.Bool("logtostderr", false, "log to standard error instead of files. DEPRECATED: will be removed in a future release.")

		args = os.Args[1:]
		cmd  = string(driver.AllMode)

		serverOptions     = options.ServerOptions{}
		controllerOptions = options.ControllerOptions{}
		nodeOptions       = options.NodeOptions{}
	)

	serverOptions.AddFlags(fs)

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
		clientset, clientErr := cloud.DefaultKubernetesAPIClient()
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

	case string(driver.ControllerMode):
		controllerOptions.AddFlags(fs)

	case string(driver.NodeMode):
		nodeOptions.AddFlags(fs)

	case string(driver.AllMode):
		controllerOptions.AddFlags(fs)
		nodeOptions.AddFlags(fs)

	default:
		klog.Errorf("Unknown driver mode %s: Expected %s, %s, %s, or pre-stop-hook", cmd, driver.ControllerMode, driver.NodeMode, driver.AllMode)
		klog.FlushAndExit(klog.ExitFlushTimeout, 0)
	}

	if err = fs.Parse(args); err != nil {
		panic(err)
	}

	err = logsapi.ValidateAndApply(c, featureGate)
	if err != nil {
		klog.ErrorS(err, "failed to validate and apply logging configuration")
	}

	if cmd != string(driver.ControllerMode) {
		// nodeOptions must have been populated from the cmdline, validate them.
		if err := nodeOptions.Validate(); err != nil {
			klog.Error(err.Error())
			klog.FlushAndExit(klog.ExitFlushTimeout, 1)
		}
	}

	if *version {
		versionInfo, err := driver.GetVersionJSON()
		if err != nil {
			klog.ErrorS(err, "failed to get version")
			klog.FlushAndExit(klog.ExitFlushTimeout, 1)
		}
		fmt.Println(versionInfo)
		osExit(0)
	}

	if *toStderr {
		klog.SetOutput(os.Stderr)
	}

	return &Options{
		DriverMode: driver.Mode(cmd),

		ServerOptions:     &serverOptions,
		ControllerOptions: &controllerOptions,
		NodeOptions:       &nodeOptions,
	}
}
