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

package options

import (
	flag "github.com/spf13/pflag"

	"github.com/kubernetes-sigs/aws-ebs-csi-driver/pkg/driver"
)

// ServerOptions contains options and configuration settings for the driver server.
type ServerOptions struct {
	// Endpoint is the endpoint that the driver server should listen on.
	Endpoint string
	// HttpEndpoint is the endpoint that the HTTP server for metrics should listen on.
	HttpEndpoint string
}

func (s *ServerOptions) AddFlags(fs *flag.FlagSet) {
	fs.StringVar(&s.Endpoint, "endpoint", driver.DefaultCSIEndpoint, "Endpoint for the CSI driver server")
	fs.StringVar(&s.HttpEndpoint, "http-endpoint", "", "The TCP network address where the HTTP server for metrics will listen (example: `:8080`). The default is empty string, which means the server is disabled.")
}
