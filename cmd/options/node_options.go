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
	"fmt"

	flag "github.com/spf13/pflag"
)

// NodeOptions contains options and configuration settings for the node service.
type NodeOptions struct {
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

func (o *NodeOptions) AddFlags(fs *flag.FlagSet) {
	fs.Int64Var(&o.VolumeAttachLimit, "volume-attach-limit", -1, "Value for the maximum number of volumes attachable per node. If specified, the limit applies to all nodes and overrides --reserved-volume-attachments. If not specified, the value is approximated from the instance type.")
	fs.IntVar(&o.ReservedVolumeAttachments, "reserved-volume-attachments", -1, "Number of volume attachments reserved for system use. Not used when --volume-attach-limit is specified. The total amount of volume attachments for a node is computed as: <nr. of attachments for corresponding instance type> - <number of NICs, if relevant to the instance type> - <reserved-volume-attachments value>. When -1, the amount of reserved attachments is loaded from instance metadata that captured state at node boot and may include not only system disks but also CSI volumes.")
}

func (o *NodeOptions) Validate() error {
	if o.VolumeAttachLimit != -1 && o.ReservedVolumeAttachments != -1 {
		return fmt.Errorf("only one of --volume-attach-limit and --reserved-volume-attachments may be specified")
	}
	return nil
}
