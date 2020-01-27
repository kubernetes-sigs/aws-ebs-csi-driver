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
	"flag"
	"os"
	"reflect"
	"testing"

	"github.com/kubernetes-sigs/aws-ebs-csi-driver/pkg/driver"
)

func TestGetOptions(t *testing.T) {
	testFunc := func(
		t *testing.T,
		additionalArgs []string,
		withServerOptions bool,
		withControllerOptions bool,
		withNodeOptions bool,
	) *Options {
		flagSet := flag.NewFlagSet("test-flagset", flag.ContinueOnError)

		endpointFlagName := "endpoint"
		endpoint := "foo"

		extraVolumeTagsFlagName := "extra-volume-tags"
		extraVolumeTagKey := "bar"
		extraVolumeTagValue := "baz"
		extraVolumeTags := map[string]string{
			extraVolumeTagKey: extraVolumeTagValue,
		}

		args := append([]string{
			"aws-ebs-csi-driver",
		}, additionalArgs...)

		if withServerOptions {
			args = append(args, "-"+endpointFlagName+"="+endpoint)
		}
		if withControllerOptions {
			args = append(args, "-"+extraVolumeTagsFlagName+"="+extraVolumeTagKey+"="+extraVolumeTagValue)
		}

		oldArgs := os.Args
		defer func() { os.Args = oldArgs }()
		os.Args = args

		options := GetOptions(flagSet)

		if withServerOptions {
			endpointFlag := flagSet.Lookup(endpointFlagName)
			if endpointFlag == nil {
				t.Fatalf("expected %q flag to be added but it is not", endpointFlagName)
			}
			if options.ServerOptions.Endpoint != endpoint {
				t.Fatalf("expected endpoint to be %q but it is %q", endpoint, options.ServerOptions.Endpoint)
			}
		}

		if withControllerOptions {
			extraVolumeTagsFlag := flagSet.Lookup(extraVolumeTagsFlagName)
			if extraVolumeTagsFlag == nil {
				t.Fatalf("expected %q flag to be added but it is not", extraVolumeTagsFlagName)
			}
			if !reflect.DeepEqual(options.ControllerOptions.ExtraVolumeTags, extraVolumeTags) {
				t.Fatalf("expected extra volume tags to be %q but it is %q", extraVolumeTags, options.ControllerOptions.ExtraVolumeTags)
			}
		}

		return options
	}

	testCases := []struct {
		name     string
		testFunc func(t *testing.T)
	}{
		{
			name: "no controller mode given - expect all mode",
			testFunc: func(t *testing.T) {
				options := testFunc(t, nil, true, true, true)

				if options.DriverMode != driver.AllMode {
					t.Fatalf("expected driver mode to be %q but it is %q", driver.AllMode, options.DriverMode)
				}
			},
		},
		{
			name: "all mode given - expect all mode",
			testFunc: func(t *testing.T) {
				options := testFunc(t, []string{"all"}, true, true, true)

				if options.DriverMode != driver.AllMode {
					t.Fatalf("expected driver mode to be %q but it is %q", driver.AllMode, options.DriverMode)
				}
			},
		},
		{
			name: "controller mode given - expect controller mode",
			testFunc: func(t *testing.T) {
				options := testFunc(t, []string{"controller"}, true, true, false)

				if options.DriverMode != driver.ControllerMode {
					t.Fatalf("expected driver mode to be %q but it is %q", driver.ControllerMode, options.DriverMode)
				}
			},
		},
		{
			name: "node mode given - expect node mode",
			testFunc: func(t *testing.T) {
				options := testFunc(t, []string{"node"}, true, false, true)

				if options.DriverMode != driver.NodeMode {
					t.Fatalf("expected driver mode to be %q but it is %q", driver.NodeMode, options.DriverMode)
				}
			},
		},
		{
			name: "version flag specified",
			testFunc: func(t *testing.T) {
				oldOSExit := osExit
				defer func() { osExit = oldOSExit }()

				var exitCode int
				testExit := func(code int) {
					exitCode = code
				}
				osExit = testExit

				oldArgs := os.Args
				defer func() { os.Args = oldArgs }()
				os.Args = []string{
					"aws-ebs-csi-driver",
					"-version",
				}

				flagSet := flag.NewFlagSet("test-flagset", flag.ContinueOnError)
				_ = GetOptions(flagSet)

				if exitCode != 0 {
					t.Fatalf("expected exit code 0 but got %d", exitCode)
				}
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, tc.testFunc)
	}
}
