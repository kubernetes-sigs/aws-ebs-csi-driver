# Volume Health

**Note: This feature is currently in alpha and disabled by default.**

The EBS CSI Driver implements the CSI `ControllerGetVolumeHealth` and `ControllerListVolumeHealth` RPCs. They report the results of the [EC2 volume status checks](https://docs.aws.amazon.com/ebs/latest/userguide/monitoring-volume-checks.html) of each volume. On Kubernetes, the [`csi-external-health-monitor-controller`](https://github.com/kubernetes-csi/external-health-monitor) sidecar (v0.19.0 or later) writes them to the `status.healthStatus` field of the volume's `PersistentVolumeClaim`.

## Installation

To use this feature, it must be enabled in the following places:
- `CSIVolumeHealth` [feature gate](https://kubernetes.io/docs/reference/command-line-tools-reference/feature-gates/) on `kube-apiserver` (Kubernetes 1.37 and later). Without it, the API server silently drops `status.healthStatus`.
- The `csi-external-health-monitor-controller` sidecar, installed by the EBS CSI Driver Helm chart when `sidecars.healthMonitor.enabled` is set to `true`. The chart also creates the ClusterRole the sidecar needs to patch `persistentvolumeclaims/status`.

Amazon EKS doesn't support alpha features, so this feature can't be used on EKS clusters yet.

The driver uses `ec2:DescribeVolumeStatus` and `ec2:DescribeVolumes`, which the policies in [Set up driver permissions](install.md#set-up-driver-permissions) already grant, including the AWS managed policies, so no IAM change is needed.

A cluster where the feature is off looks the same as one where every volume is healthy. To check the setup:
- `kubectl get --raw /metrics` on kube-apiserver includes `kubernetes_feature_enabled{name="CSIVolumeHealth",stage="ALPHA"} 1`.
- With `controller.enableMetrics`, the sidecar serves its metrics on port 3308, and on the leader `csi_volume_health_probe_total{method="ControllerListVolumeHealth",result="success"}` grows every list interval.
- If `csi_volume_health_status_writes_dropped_total` grows, or the sidecar logs that the API server dropped the `healthStatus` field, the feature gate is off or Kubernetes is older than 1.37.

## Health conditions

| EC2 volume status check | `status` | `reason` |
|---|---|---|
| `io-enabled`: `failed` | `Inaccessible` | `IODisabled` |
| `io-performance`: `stalled` | `Inaccessible` | `IOPerformanceStalled` |
| `io-performance`: `severely-degraded` | `Degraded` | `IOPerformanceSeverelyDegraded` |
| `io-performance`: `degraded` | `Degraded` | `IOPerformanceDegraded` |

Any other result, including `insufficient-data`, adds no condition. A condition is removed when its check reports any other result, which is not always a recovery: for example, `io-performance` is not checked while the volume is detached.

For example, while I/O to a `gp3` volume is paused:

```
$ kubectl get pvc data -o jsonpath='{.status.healthStatus}' | jq
{
  "healthConditions": [
    {
      "message": "I/O to the volume is stalled (EC2 volume status check io-performance: stalled)",
      "reason": "IOPerformanceStalled",
      "status": "Inaccessible"
    }
  ],
  "lastTransitionTime": "2026-10-21T17:46:09Z"
}
```

## Considerations

- EC2 runs the `io-performance` check only for `io1`, `io2` and `gp3` volumes that are attached to an instance. For `gp2`, `st1`, `sc1` and `standard` volumes, `io-performance` is `not-applicable` and only `io-enabled` is checked, so the driver can't report a stall on them. On Nitro instances, the [`VolumeStalledIOCheck`](https://docs.aws.amazon.com/ebs/latest/userguide/using_cloudwatch_ebs.html) CloudWatch metric still reports stalls on these volumes.
- Conditions appear and clear a few minutes after the underlying problem starts and ends: EC2 updates the status checks periodically, and the sidecar lists volume health every `--list-volumes-interval` (5 minutes by default, set it with `sidecars.healthMonitor.additionalArgs`).
- Each list cycle makes one `ec2:DescribeVolumeStatus` call per 1,000 volumes in the account and Region, because EC2 can't filter volume status by tag and applies filters after paginating. In accounts with many volumes, consider a longer `--list-volumes-interval`.
- While `io1` and `io2` volumes created from snapshots are initializing, EC2 can report their I/O performance as degraded. [The EBS documentation](https://docs.aws.amazon.com/ebs/latest/userguide/initalize-volume.html) says this is expected and can be ignored.
- Volume status does not reflect the volume state, so a volume in the `error` state is not reported. A volume with [auto-enabled I/O](https://docs.aws.amazon.com/ebs/latest/userguide/volumeIO.html) keeps passing the `io-enabled` check when EBS finds its data potentially inconsistent, so it is not reported either.
- The sidecar only monitors bound PersistentVolumes with a `spec.csi` source. PersistentVolumes provisioned through a StorageClass with `provisioner: kubernetes.io/aws-ebs` (CSI migration) keep the in-tree `awsElasticBlockStore` source and are not monitored. Node-local volumes report no health.
- A volume deleted outside of Kubernetes is not reported: List doesn't include it, and for a PVC that already has conditions, `ControllerGetVolumeHealth` returns `NOT_FOUND` and the sidecar keeps them.
- Disabling the sidecar leaves existing conditions in place until it is enabled again or they are cleared by hand: `kubectl patch pvc <pvc> --subresource=status --type=merge -p '{"status":{"healthStatus":null}}'`.

## Testing with AWS Fault Injection Service

The [`aws:ebs:pause-volume-io`](https://docs.aws.amazon.com/ebs/latest/userguide/ebs-fis-pause-io.html) FIS action pauses the I/O of a volume attached to a Nitro instance. While a `gp3` volume is paused, `io-performance` becomes `stalled` after two to four minutes, and it returns to `normal` two to three minutes after the pause ends. The PVC condition follows within one `--list-volumes-interval`.

1. Create a cluster with the `CSIVolumeHealth` feature gate on `kube-apiserver` (for `make cluster/create`, add `CSIVolumeHealth: "true"` to `spec.kubeAPIServer.featureGates` in `hack/e2e/kops/patch-cluster.yaml` before the cluster is created), and install the driver with `sidecars.healthMonitor.enabled=true`.
2. Create a `gp3` PVC and a pod that keeps writing to it, for example with `dd if=/dev/zero of=/data/out bs=1M count=32 oflag=direct conv=fsync` in a loop.
3. Create an IAM role that `fis.amazonaws.com` can assume, with `ec2:DescribeVolumes`, `tag:GetResources` and `ec2:PauseVolumeIO`.
4. Create and start an experiment that pauses the volume for 10 minutes:
   ```
   aws fis create-experiment-template --cli-input-json '{
     "description": "Pause EBS volume I/O",
     "targets": {"Volumes": {"resourceType": "aws:ec2:ebs-volume", "resourceArns": ["arn:aws:ec2:<region>:<account>:volume/<volume-id>"], "selectionMode": "ALL"}},
     "actions": {"PauseIO": {"actionId": "aws:ebs:pause-volume-io", "parameters": {"duration": "PT10M"}, "targets": {"Volumes": "Volumes"}}},
     "stopConditions": [{"source": "none"}],
     "roleArn": "arn:aws:iam::<account>:role/<fis-role>"
   }'
   aws fis start-experiment --experiment-template-id <template-id>
   ```
5. Watch `kubectl get pvc <pvc> -o jsonpath='{.status.healthStatus}'` and `aws ec2 describe-volume-status --volume-ids <volume-id>`. The PVC gets the `IOPerformanceStalled` condition, which is removed again a few minutes after the experiment ends.
