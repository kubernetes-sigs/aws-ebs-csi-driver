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

// Options contains options and configuration settings for the driver.
type Options struct {
	Mode Mode

	// #### Server options ####

	//Endpoint is the endpoint for the CSI driver server
	Endpoint string
	// HttpEndpoint is the TCP network address where the HTTP server for metrics will listen
	HttpEndpoint string
	// EnableOtelTracing is a flag to enable opentelemetry tracing for the driver
	EnableOtelTracing bool

	// #### Controller options ####

	// ExtraTags is a map of tags that will be attached to each dynamically provisioned
	// resource.
	ExtraTags map[string]string
	// ExtraVolumeTags is a map of tags that will be attached to each dynamically provisioned
	// volume.
	// DEPRECATED: Use ExtraTags instead.
	ExtraVolumeTags map[string]string
	// ID of the kubernetes cluster.
	KubernetesClusterID string
	// flag to enable sdk debug log
	AwsSdkDebugLog bool
	// flag to warn on invalid tag, instead of returning an error
	WarnOnInvalidTag bool
	// flag to set user agent
	UserAgentExtra string
	// flag to enable batching of API calls
	Batching bool
	// flag to set the timeout for volume modification requests to be coalesced into a single
	// volume modification call to AWS.
	ModifyVolumeRequestHandlerTimeout time.Duration

	// #### Node options #####

	// VolumeAttachLimit specifies the value that shall be reported as "maximum number of attachable volumes"
	// in CSINode objects. It is similar to https://kubernetes.io/docs/concepts/storage/storage-limits/#custom-limits
	// which allowed administrators to specify custom volume limits by configuring the kube-scheduler. Also, each AWS
	// machine type has different volume limits. By default, the EBS CSI driver parses the machine type name and then
	// decides the volume limit. However, this is only a rough approximation and not good enough in most cases.
	// Specifying the volume attach limit via command line is the alternative until a more sophisticated solution presents
	// itself (dynamically discovering the maximum number of attachable volume per EC2 machine type, see also
	// https://github.com/kubernetes-sigs/aws-ebs-csi-driver/issues/347).
	VolumeAttachLimit int64
	// ReservedVolumeAttachments specifies number of volume attachments reserved for system use.
	// Typically 1 for the root disk, but may be larger when more system disks are attached to nodes.
	// This option is not used when --volume-attach-limit is specified.
	// When -1, the amount of reserved attachments is loaded from instance metadata that captured state at node boot
	// and may include not only system disks but also CSI volumes (and therefore it may be wrong).
	ReservedVolumeAttachments int
}

func (o *Options) AddFlags(f *flag.FlagSet) {
	// Server options
	f.StringVar(&o.Endpoint, "endpoint", DefaultCSIEndpoint, "Endpoint for the CSI driver server")
	f.StringVar(&o.HttpEndpoint, "http-endpoint", "", "The TCP network address where the HTTP server for metrics will listen (example: `:8080`). The default is empty string, which means the server is disabled.")
	f.BoolVar(&o.EnableOtelTracing, "enable-otel-tracing", false, "To enable opentelemetry tracing for the driver. The tracing is disabled by default. Configure the exporter endpoint with OTEL_EXPORTER_OTLP_ENDPOINT and other env variables, see https://opentelemetry.io/docs/specs/otel/configuration/sdk-environment-variables/#general-sdk-configuration.")

	// Controller options
	if o.Mode == AllMode || o.Mode == ControllerMode {
		f.Var(cliflag.NewMapStringString(&o.ExtraTags), "extra-tags", "Extra tags to attach to each dynamically provisioned resource. It is a comma separated list of key value pairs like '<key1>=<value1>,<key2>=<value2>'")
		f.Var(cliflag.NewMapStringString(&o.ExtraVolumeTags), "extra-volume-tags", "DEPRECATED: Please use --extra-tags instead. Extra volume tags to attach to each dynamically provisioned volume. It is a comma separated list of key value pairs like '<key1>=<value1>,<key2>=<value2>'")
		f.StringVar(&o.KubernetesClusterID, "k8s-tag-cluster-id", "", "ID of the Kubernetes cluster used for tagging provisioned EBS volumes (optional).")
		f.BoolVar(&o.AwsSdkDebugLog, "aws-sdk-debug-log", false, "To enable the aws sdk debug log level (default to false).")
		f.BoolVar(&o.WarnOnInvalidTag, "warn-on-invalid-tag", false, "To warn on invalid tags, instead of returning an error")
		f.StringVar(&o.UserAgentExtra, "user-agent-extra", "", "Extra string appended to user agent.")
		f.BoolVar(&o.Batching, "batching", false, "To enable batching of API calls. This is especially helpful for improving performance in workloads that are sensitive to EC2 rate limits.")
		f.DurationVar(&o.ModifyVolumeRequestHandlerTimeout, "modify-volume-request-handler-timeout", DefaultModifyVolumeRequestHandlerTimeout, "Timeout for the window in which volume modification calls must be received in order for them to coalesce into a single volume modification call to AWS. This must be lower than the csi-resizer and volumemodifier timeouts")
	}
	// Node options
	if o.Mode == AllMode || o.Mode == NodeMode {
		f.Int64Var(&o.VolumeAttachLimit, "volume-attach-limit", -1, "Value for the maximum number of volumes attachable per node. If specified, the limit applies to all nodes and overrides --reserved-volume-attachments. If not specified, the value is approximated from the instance type.")
		f.IntVar(&o.ReservedVolumeAttachments, "reserved-volume-attachments", -1, "Number of volume attachments reserved for system use. Not used when --volume-attach-limit is specified. The total amount of volume attachments for a node is computed as: <nr. of attachments for corresponding instance type> - <number of NICs, if relevant to the instance type> - <reserved-volume-attachments value>. When -1, the amount of reserved attachments is loaded from instance metadata that captured state at node boot and may include not only system disks but also CSI volumes.")
	}
}

func (o *Options) Validate() error {
	if o.VolumeAttachLimit != -1 && o.ReservedVolumeAttachments != -1 {
		return fmt.Errorf("only one of --volume-attach-limit and --reserved-volume-attachments may be specified")
	}
	return nil
}
