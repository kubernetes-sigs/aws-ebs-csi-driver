# CreateVolume (`StorageClass`) Parameters

## Supported Parameters
There are several optional parameters that may be passed into `CreateVolumeRequest.parameters` map, these parameters can be configured in StorageClass, see [example](../examples/kubernetes/storageclass). Unless explicitly noted, all parameters are case insensitive (e.g. "kmsKeyId", "kmskeyid" and any other combination of upper/lowercase characters can be used).

The AWS EBS CSI Driver supports [tagging](tagging.md) through `StorageClass.parameters` (in v1.6.0 and later). 

| Parameters                   | Values                                             | Default | Description                                                                                                                                                                                                                                                                                                                                                                                    |
|------------------------------|----------------------------------------------------|---------|------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| "csi.storage.k8s.io/fstype"  | xfs, ext2, ext3, ext4                              | ext4    | File system type that will be formatted during volume creation. This parameter is case sensitive!                                                                                                                                                                                                                                                                                              |
| "type"                       | io1, io2, gp2, gp3, sc1, st1, standard, sbp1, sbg1 | gp3*    | EBS volume type.                                                                                                                                                                                                                                                                                                                                                                               |
| "iopsPerGB"                  |                                                    |         | I/O operations per second per GiB. Can be specified for IO1, IO2, and GP3 volumes.                                                                                                                                                                                                                                                                                                             |
| "allowAutoIOPSPerGBIncrease" | true, false                                        | false   | When `"true"`, the CSI driver increases IOPS for a volume when `iopsPerGB * <volume size>` is too low to fit into IOPS range supported by AWS. This allows dynamic provisioning to always succeed, even when user specifies too small PVC capacity or `iopsPerGB` value. On the other hand, it may introduce additional costs, as such volumes have higher IOPS than requested in `iopsPerGB`. |
| "iops"                       |                                                    |         | I/O operations per second. Can be specified for IO1, IO2, and GP3 volumes.                                                                                                                                                                                                                                                                                                                     |
| "throughput"                 |                                                    | 125     | Throughput in MiB/s. Only effective when gp3 volume type is specified. If empty, it will set to 125MiB/s as documented [here](https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/ebs-volume-types.html).                                                                                                                                                                                      |
| "encrypted"                  | true, false                                        | false   | Whether the volume should be encrypted or not. Valid values are "true" or "false".                                                                                                                                                                                                                                                                                                             |
| "blockExpress"               | true, false                                        | false   | Enables the creation of [io2 Block Express volumes](https://aws.amazon.com/ebs/provisioned-iops/#Introducing_io2_Block_Express) by increasing the IOPS limit for io2 volumes to 256000. Volumes created with more than 64000 IOPS will fail to mount on instances that do not support io2 Block Express.                                                                                       |
| "kmsKeyId"                   |                                                    |         | The full ARN of the key to use when encrypting the volume. If not specified, AWS will use the default KMS key for the region the volume is in. This will be an auto-generated key called `/aws/ebs` if not changed.                                                                                                                                                                            |
| "blockSize"                  |                                                    |         | The block size to use when formatting the underlying filesystem. Only supported on linux nodes and with fstype `ext2`, `ext3`, `ext4`, or `xfs`.                                                                                                                                                                                                                                               |
| "inodeSize"                  |                                                    |         | The inode size to use when formatting the underlying filesystem. Only supported on linux nodes and with fstype `ext2`, `ext3`, `ext4`, or `xfs`.                                                                                                                                                                                                                                               |
| "bytesPerInode"              |                                                    |         | The `bytes-per-inode` to use when formatting the underlying filesystem. Only supported on linux nodes and with fstype `ext2`, `ext3`, `ext4`.                                                                                                                                                                                                                                                  |
| "numberOfInodes"             |                                                    |         | The `number-of-inodes` to use when formatting the underlying filesystem. Only supported on linux nodes and with fstype `ext2`, `ext3`, `ext4`.                                                                                                                                                                                                                                                 |
| "ext4BigAlloc"               | true, false                                        | false   | Changes the `ext4` filesystem to use clustered block allocation by enabling the `bigalloc` formatting option. Warning: `bigalloc` may not be fully supported with your node's Linux kernel. Please see our [FAQ](/docs/faq.md).                                                                                                                                                                |
| "ext4ClusterSize"            |                                                    |         | The cluster size to use when formatting an `ext4` filesystem when the `bigalloc` feature is enabled. Note: The `ext4BigAlloc` parameter must be set to true. See our [FAQ](/docs/faq.md).                                                                                                                                                                                                      |

## Restrictions
* `gp3` is currently not supported on outposts. Outpost customers need to use a different type for their volumes.
* If the requested IOPS (either directly from `iops` or from `iopsPerGB` multiplied by the volume's capacity) produces a value above the maximum IOPS allowed for the [volume type](https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/ebs-volume-types.html), the IOPS will be capped at the maximum value allowed. If the value is lower than the minimal supported IOPS value per volume, either an error is returned (the default behavior), or the value is increased to fit into the supported range when `allowautoiopspergbincrease` is `"true"`.
* You may specify either the "iops" or "iopsPerGb" parameters, not both. Specifying both parameters will result in an invalid StorageClass.

| Volume Type                | Min total IOPS | Max total IOPS | Max IOPS per GB   |
|----------------------------|----------------|---------------|-------------------|
| io1                        | 100            | 64000         | 50                |
| io2 (blockExpress = false) | 100            | 64000         | 500               |
| io2 (blockExpress = true)  | 100            | 256000        | 500               |
| gp3                        | 3000           | 16000         | 500               |

## Volume Availability Zone and Topologies

The EBS CSI Driver supports the [`WaitForFirstConsumer` volume binding mode in Kubernetes](https://kubernetes.io/docs/concepts/storage/storage-classes/#volume-binding-mode). When using `WaitForFirstConsumer` binding mode the volume will automatically be created in the appropriate Availability Zone and with the appropriate topology. The `WaitForFirstConsumer` binding mode is recommended whenever possible for dynamic provisioning.

When using static provisioning, or if `WaitForFirstConsumer` is not suitable for a specific usecase, the Availability Zone can be specified via the standard CSI topology mechanisms. The EBS CSI Driver supports specifying the Availability Zone via either the key `topology.kubernetes.io/zone` or the key `topology.ebs.csi.aws.com/zone`.

On Kubernetes, the Availability Zone of dynamically provisioned volumes can be restricted with the [`StorageClass`'s `availableToplogies` parameter](https://kubernetes.io/docs/concepts/storage/storage-classes/#allowed-topologies), for example:

```
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: ebs-sc
provisioner: ebs.csi.aws.com
allowedTopologies:
- matchLabelExpressions:
  - key: topology.kubernetes.io/zone
    values:
    - us-east-1
```

Additionally, statically provisioned volumes can be restricted to pods in the appropriate Availability Zone, see the [static provisioning example](../examples/kubernetes/static-provisioning/).
