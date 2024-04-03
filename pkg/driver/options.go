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

package driver

import (
	"fmt"
	"time"

	flag "github.com/spf13/pflag"
	cliflag "k8s.io/component-base/cli/flag"
)

type Options struct {
	Mode                              Mode
	Endpoint                          string
	HttpEndpoint                      string
	EnableOtelTracing                 bool
	ExtraTags                         map[string]string
	ExtraVolumeTags                   map[string]string
	KubernetesClusterID               string
	AwsSdkDebugLog                    bool
	WarnOnInvalidTag                  bool
	UserAgentExtra                    string
	Batching                          bool
	ModifyVolumeRequestHandlerTimeout time.Duration
	VolumeAttachLimit                 int64
	ReservedVolumeAttachments         int
}

func (o *Options) AddFlags(fs *flag.FlagSet) {
	fs.StringVar(&o.Endpoint, "endpoint", DefaultCSIEndpoint, "Endpoint for the CSI driver server")
	fs.StringVar(&o.HttpEndpoint, "http-endpoint", "", "The TCP network address where the HTTP server for metrics will listen (example: `:8080`). The default is empty string, which means the server is disabled.")
	fs.BoolVar(&o.EnableOtelTracing, "enable-otel-tracing", false, "To enable opentelemetry tracing for the driver. The tracing is disabled by default. Configure the exporter endpoint with OTEL_EXPORTER_OTLP_ENDPOINT and other env variables, see https://opentelemetry.io/docs/specs/otel/configuration/sdk-environment-variables/#general-sdk-configuration.")
	fs.Var(cliflag.NewMapStringString(&o.ExtraTags), "extra-tags", "Extra tags to attach to each dynamically provisioned resource. It is a comma separated list of key value pairs like '<key1>=<value1>,<key2>=<value2>'")
	fs.Var(cliflag.NewMapStringString(&o.ExtraVolumeTags), "extra-volume-tags", "DEPRECATED: Please use --extra-tags instead. Extra volume tags to attach to each dynamically provisioned volume. It is a comma separated list of key value pairs like '<key1>=<value1>,<key2>=<value2>'")
	fs.StringVar(&o.KubernetesClusterID, "k8s-tag-cluster-id", "", "ID of the Kubernetes cluster used for tagging provisioned EBS volumes (optional).")
	fs.BoolVar(&o.AwsSdkDebugLog, "aws-sdk-debug-log", false, "To enable the aws sdk debug log level (default to false).")
	fs.BoolVar(&o.WarnOnInvalidTag, "warn-on-invalid-tag", false, "To warn on invalid tags, instead of returning an error")
	fs.StringVar(&o.UserAgentExtra, "user-agent-extra", "", "Extra string appended to user agent.")
	fs.BoolVar(&o.Batching, "batching", false, "To enable batching of API calls. This is especially helpful for improving performance in workloads that are sensitive to EC2 rate limits.")
	fs.DurationVar(&o.ModifyVolumeRequestHandlerTimeout, "modify-volume-request-handler-timeout", DefaultModifyVolumeRequestHandlerTimeout, "Timeout for the window in which volume modification calls must be received in order for them to coalesce into a single volume modification call to AWS. This must be lower than the csi-resizer and volumemodifier timeouts")
	fs.Int64Var(&o.VolumeAttachLimit, "volume-attach-limit", -1, "Value for the maximum number of volumes attachable per node. If specified, the limit applies to all nodes and overrides --reserved-volume-attachments. If not specified, the value is approximated from the instance type.")
	fs.IntVar(&o.ReservedVolumeAttachments, "reserved-volume-attachments", -1, "Number of volume attachments reserved for system use. Not used when --volume-attach-limit is specified. The total amount of volume attachments for a node is computed as: <nr. of attachments for corresponding instance type> - <number of NICs, if relevant to the instance type> - <reserved-volume-attachments value>. When -1, the amount of reserved attachments is loaded from instance metadata that captured state at node boot and may include not only system disks but also CSI volumes.")
}

func (o *Options) Validate() error {
	if o.VolumeAttachLimit != -1 && o.ReservedVolumeAttachments != -1 {
		return fmt.Errorf("only one of --volume-attach-limit and --reserved-volume-attachments may be specified")
	}
	return nil
}
