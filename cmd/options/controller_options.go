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
	"time"

	flag "github.com/spf13/pflag"

	"github.com/kubernetes-sigs/aws-ebs-csi-driver/pkg/driver"
	cliflag "k8s.io/component-base/cli/flag"
)

// ControllerOptions contains options and configuration settings for the controller service.
type ControllerOptions struct {
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
}

func (s *ControllerOptions) AddFlags(fs *flag.FlagSet) {
	fs.Var(cliflag.NewMapStringString(&s.ExtraTags), "extra-tags", "Extra tags to attach to each dynamically provisioned resource. It is a comma separated list of key value pairs like '<key1>=<value1>,<key2>=<value2>'")
	fs.Var(cliflag.NewMapStringString(&s.ExtraVolumeTags), "extra-volume-tags", "DEPRECATED: Please use --extra-tags instead. Extra volume tags to attach to each dynamically provisioned volume. It is a comma separated list of key value pairs like '<key1>=<value1>,<key2>=<value2>'")
	fs.StringVar(&s.KubernetesClusterID, "k8s-tag-cluster-id", "", "ID of the Kubernetes cluster used for tagging provisioned EBS volumes (optional).")
	fs.BoolVar(&s.AwsSdkDebugLog, "aws-sdk-debug-log", false, "To enable the aws sdk debug log level (default to false).")
	fs.BoolVar(&s.WarnOnInvalidTag, "warn-on-invalid-tag", false, "To warn on invalid tags, instead of returning an error")
	fs.StringVar(&s.UserAgentExtra, "user-agent-extra", "", "Extra string appended to user agent.")
	fs.BoolVar(&s.Batching, "batching", false, "To enable batching of API calls. This is especially helpful for improving performance in workloads that are sensitive to EC2 rate limits.")
	fs.DurationVar(&s.ModifyVolumeRequestHandlerTimeout, "modify-volume-request-handler-timeout", driver.DefaultModifyVolumeRequestHandlerTimeout, "Timeout for the window in which volume modification calls must be received in order for them to coalesce into a single volume modification call to AWS. This must be lower than the csi-resizer and volumemodifier timeouts")
}
