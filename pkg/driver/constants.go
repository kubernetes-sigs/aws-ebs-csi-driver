/*
Copyright 2019 The Kubernetes Authors.

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

// constants of keys in PublishContext
const (
	// devicePathKey represents key for device path in PublishContext
	// devicePath is the device path where the volume is attached to
	DevicePathKey = "devicePath"
)

// constants of keys in VolumeContext
const (
	// VolumeAttributePartition represents key for partition config in VolumeContext
	// this represents the partition number on a device used to mount
	VolumeAttributePartition = "partition"
)

// constants of disk partition suffix
const (
	diskPartitionSuffix     = ""
	nvmeDiskPartitionSuffix = "p"
)

// constants of keys in volume parameters
const (
	// VolumeTypeKey represents key for volume type
	VolumeTypeKey = "type"

	// IopsPerGBKey represents key for IOPS per GB
	IopsPerGBKey = "iopspergb"

	// AllowAutoIOPSPerGBIncreaseKey represents key for allowing automatic increase of IOPS
	AllowAutoIOPSPerGBIncreaseKey = "allowautoiopspergbincrease"

	// Iops represents key for IOPS for volume
	IopsKey = "iops"

	// ThroughputKey represents key for throughput
	ThroughputKey = "throughput"

	// EncryptedKey represents key for whether filesystem is encrypted
	EncryptedKey = "encrypted"

	// KmsKeyId represents key for KMS encryption key
	KmsKeyIDKey = "kmskeyid"

	// PVCNameKey contains name of the PVC for which is a volume provisioned.
	PVCNameKey = "csi.storage.k8s.io/pvc/name"

	// PVCNamespaceKey contains namespace of the PVC for which is a volume provisioned.
	PVCNamespaceKey = "csi.storage.k8s.io/pvc/namespace"

	// PVNameKey contains name of the final PV that will be used for the dynamically
	// provisioned volume
	PVNameKey = "csi.storage.k8s.io/pv/name"

	// BlockExpressKey increases the iops limit for io2 volumes to the block express limit
	BlockExpressKey = "blockexpress"

	// BlockSizeKey configures the block size when formatting a volume
	BlockSizeKey = "blocksize"

	// TagKeyPrefix contains the prefix of a volume parameter that designates it as
	// a tag to be attached to the resource
	TagKeyPrefix = "tagSpecification"
)

// constants for volume tags and their values
const (
	// ResourceLifecycleTagPrefix is prefix of tag for provisioned EBS volume that
	// marks them as owned by the cluster. Used only when --cluster-id is set.
	ResourceLifecycleTagPrefix = "kubernetes.io/cluster/"

	// ResourceLifecycleOwned is the value we use when tagging resources to indicate
	// that the resource is considered owned and managed by the cluster,
	// and in particular that the lifecycle is tied to the lifecycle of the cluster.
	// From k8s.io/legacy-cloud-providers/aws/tags.go.
	ResourceLifecycleOwned = "owned"

	// NameTag is tag applied to provisioned EBS volume for backward compatibility with
	// in-tree volume plugin. Used only when --cluster-id is set.
	NameTag = "Name"

	// KubernetesClusterTag is tag applied to provisioned EBS volume for backward compatibility with
	// in-tree volume plugin. Used only when --cluster-id is set.
	// See https://github.com/kubernetes/cloud-provider-aws/blob/release-1.20/pkg/providers/v1/tags.go#L38-L41.
	KubernetesClusterTag = "KubernetesCluster"

	// PVCNameTag is tag applied to provisioned EBS volume for backward compatibility
	// with in-tree volume plugin. Value of the tag is PVC name. It is applied only when
	// the external provisioner sidecar is started with --extra-create-metadata=true and
	// thus provides such metadata to the CSI driver.
	PVCNameTag = "kubernetes.io/created-for/pvc/name"

	// PVCNamespaceTag is tag applied to provisioned EBS volume for backward compatibility
	// with in-tree volume plugin. Value of the tag is PVC namespace. It is applied only when
	// the external provisioner sidecar is started with --extra-create-metadata=true and
	// thus provides such metadata to the CSI driver.
	PVCNamespaceTag = "kubernetes.io/created-for/pvc/namespace"

	// PVNameTag is tag applied to provisioned EBS volume for backward compatibility
	// with in-tree volume plugin. Value of the tag is PV name. It is applied only when
	// the external provisioner sidecar is started with --extra-create-metadata=true and
	// thus provides such metadata to the CSI driver.
	PVNameTag = "kubernetes.io/created-for/pv/name"
)

// constants for default command line flag values
const (
	DefaultCSIEndpoint = "unix://tmp/csi.sock"
)

// constants for disk block size
const (
	//DefaultBlockSize represents the default block size (4KB)
	DefaultBlockSize = 4096
)

// constants for fstypes
const (
	// FSTypeExt2 represents the ext2 filesystem type
	FSTypeExt2 = "ext2"
	// FSTypeExt3 represents the ext3 filesystem type
	FSTypeExt3 = "ext3"
	// FSTypeExt4 represents the ext4 filesystem type
	FSTypeExt4 = "ext4"
	// FSTypeXfs represents the xfs filesystem type
	FSTypeXfs = "xfs"
	// FSTypeNtfs represents the ntfs filesystem type
	FSTypeNtfs = "ntfs"
)

// BlockSizeExcludedFSTypes contains the filesystems that a custom block size is *NOT* supported on
var (
	BlockSizeExcludedFSTypes = map[string]struct{}{
		FSTypeNtfs: {},
	}
)
