# Volume Health Monitoring

The EBS CSI Driver provides experimental support for [Volume Health Monitoring](https://kubernetes.io/docs/concepts/storage/volume-health-monitoring/). Because Volume Health Monitoring is an alpha feature, this support is subject to change as the feature evolves.

When this feature is enabled, the EBS CSI Driver will use [`DescribeVolumeStatus`](https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_DescribeVolumeStatus.html) to determine the status of volumes and report abnormal volumes to the Container Orchestrator.

## Prerequisites

If using Kubernetes, Volume Health Monitoring requires Kubernetes 1.21 or later. For other Container Orchestrators, refer to their respective documentation for Volume Health Monitoring support.

The EBS CSI Driver must be given permission to access the [`DescribeVolumeStatus` EC2 API](https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_DescribeVolumeStatus.html) for the volumes in the cluster. This example snippet can be used in an IAM policy to grant access to `DescribeVolumeStatus` on all volumes:

```json
{
  "Effect": "Allow",
  "Action": [
    "ec2:DescribeVolumeStatus"
  ],
  "Resource": "*"
}

```

The controller must be passed the CLI flag `--volume-health-monitoring=true` (exposed via the Helm parameter `controller.volumeHealthMonitoring`).

Additionally, the controller needs to run with the [External Health Monitor sidecar](https://github.com/kubernetes-csi/external-health-monitor) (this will be performed automatically when using Helm with `controller.volumeHealthMonitoring`).

## `ListVolumes` Support

In the default configuration, the EBS CSI Driver supports Volume Health Monitoring only using `ControllerGetVolume`. This will result in 2 AWS API calls per volume (for `DescribeVolumes` and `DescribeVolumeStatus`). The [`ListVolumes`](https://github.com/container-storage-interface/spec/blob/master/spec.md#listvolumes) CSI RPC call can be used to batch up to 500 volumes with the same 2 AWS API calls.

The EBS CSI Driver can only support `ListVolumes` if the volumes are marked on a per-cluster basis. `ListVolumes` support will be automatically enabled if the EBS CSI Driver is launched using the CLI parameter `--k8s-tag-cluster-id`. This parameter is available using the Helm parameter `controller.k8sTagClusterId`, and is automatically set based on the EKS cluster name when installing the driver as an EKS-managed addon.

## Operation

The EBS CSI Driver will report any volumes with `impaired` status as abnormal. This will record a `VolumeConditionAbnormal` event on the `PersistenVolumeClaim` associated with the abnormal volume. This event will include the most recent message from the AWS API about the volume's status.

At this time, no other action will be taken on abnormal volumes. The cluster administrator and/or users should monitor `PersistentVolumeClaim`s for `VolumeConditionAbnormal` and take manual steps to rectify volumes.
