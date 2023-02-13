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

	"github.com/kubernetes-sigs/aws-ebs-csi-driver/cmd/options"
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
		mode = driver.AllMode

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

	if len(os.Args) > 1 {
		cmd := os.Args[1]

		switch {
		case cmd == string(driver.ControllerMode):
			controllerOptions.AddFlags(fs)
			args = os.Args[2:]
			mode = driver.ControllerMode

		case cmd == string(driver.NodeMode):
			nodeOptions.AddFlags(fs)
			args = os.Args[2:]
			mode = driver.NodeMode

		case cmd == string(driver.AllMode):
			controllerOptions.AddFlags(fs)
			nodeOptions.AddFlags(fs)
			args = os.Args[2:]

		case strings.HasPrefix(cmd, "-"):
			controllerOptions.AddFlags(fs)
			nodeOptions.AddFlags(fs)
			args = os.Args[1:]

		default:
			fmt.Printf("unknown command: %s: expected %q, %q or %q", cmd, driver.ControllerMode, driver.NodeMode, driver.AllMode)
			os.Exit(1)
		}
	}

	if err = fs.Parse(args); err != nil {
		panic(err)
	}

	err = logsapi.ValidateAndApply(c, featureGate)
	if err != nil {
		klog.ErrorS(err, "failed to validate and apply logging configuration")
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
		DriverMode: mode,

		ServerOptions:     &serverOptions,
		ControllerOptions: &controllerOptions,
		NodeOptions:       &nodeOptions,
	}
}
