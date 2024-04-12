# Frequently Asked Questions

## 6-Minute Delays in Attaching Volumes

### What causes 6-minute delays in attaching volumes?

After a node is preempted, pods using persistent volumes may experience a 6+ minute restart delay. This occurs when a volume is not cleanly unmounted, as indicated by `Node.Status.VolumesInUse`. The Attach/Detach controller in `kube-controller-manager` waits for 6 minutes before issuing a force detach. This delay in pod restart can prevent potential data corruption due to unclean mounts.

### What behaviors have been observed that contribute to this issue?

- The EBS CSI Driver node pod on the node may get terminated before the workload's volumes are unmounted, leading to unmount failures.
- The operating system might shut down before volume unmount is completed, even with remaining time in the `shutdownGracePeriod`.

### What are the symptoms of this issue?

```
Warning  FailedAttachVolume      6m51s              attachdetach-controller  Multi-Attach error for volume "pvc-4a86c32c-fbce-11ea-b7d2-0ad358e4e824" Volume is already exclusively attached to one node and can't be attached to another
```

### What steps can be taken to mitigate this issue?

1. **Graceful Termination:** Ensure instances are gracefully terminated, which is a best practice in Kubernetes. This allows the CSI driver to clean up volumes before the node is terminated utilizing the preStop lifecycle hook.

2. **Configure Kubelet for Graceful Node Shutdown:** For unexpected shutdowns, it is highly recommended to configure the Kubelet for graceful node shutdown. Using the standard EKS-optimized AMI, you can configure the kubelet for graceful node shutdown with the following user data script:

```bash
  #!/bin/bash
  echo -e "InhibitDelayMaxSec=45\n" >> /etc/systemd/logind.conf
  systemctl restart systemd-logind
  echo "$(jq ".shutdownGracePeriod=\"45s\"" /etc/kubernetes/kubelet/kubelet-config.json)" > /etc/kubernetes/kubelet/kubelet-config.json
  echo "$(jq ".shutdownGracePeriodCriticalPods=\"15s\"" /etc/kubernetes/kubelet/kubelet-config.json)" > /etc/kubernetes/kubelet/kubelet-config.json
  systemctl restart kubelet
```

3. **Spot Instances and Karpenter Best Practices:** When using Spot Instances with Karpenter, enable [interruption handling](https://aws.github.io/aws-eks-best-practices/karpenter/#enable-interruption-handling-when-using-spot) to manage involuntary interruptions gracefully. Karpenter supports native interruption handling, which cordons, drains, and terminates nodes ahead of interruption events, maximizing workload cleanup time.

### What is the PreStop lifecycle hook?

The [PreStop](https://kubernetes.io/docs/concepts/containers/container-lifecycle-hooks/#container-hooks) hook executes to check for remaining `VolumeAttachment` objects when a node is being drained for termination. If the node is not being drained, the check is skipped. The hook uses Kubernetes informers to monitor the deletion of `VolumeAttachment` objects, waiting until all attachments associated with the node are removed before proceeding.

### How do I enable the PreStop lifecycle hook?

The PreStop lifecycle hook is enabled by default in the AWS EBS CSI driver. It will execute when a node is being drained for termination.

## Driver performance for large-scale clusters

### Summary of scalability-related changes in v1.25

[Version 1.25 of aws-ebs-csi-driver](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/blob/master/CHANGELOG.md#v1250) featured four improvements to better manage the EBS volume lifecycle for large-scale clusters. 

At a high-level:
1. Batching EC2 `DescribeVolumes` API Calls across CSI gRPC calls ([#1819](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/1819))
	- This greatly decreases the number of EC2 `DescribeVolumes` calls made by the driver, which significantly reduces your risk of region-level throttling of the ['Describe*' EC2 API Action](https://docs.aws.amazon.com/AWSEC2/latest/APIReference/throttling.html#throttling-limits)
2. Increasing the default CSI sidecar `worker-threads` values (to 100 for all sidecars) ([#1834](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/1834))
	- E.g. the `external-provisioner` can be simultaneously running 100 `ControllerPublishVolume` operations, the `external-attacher` now has 100 goroutines for processing VolumeAttachments, etc. 
	- This increases the number of in-flight EBS volume creations / attaches / modifications / deletions managed by the driver (which may increase your risk of region-level ['Mutating action' request throttling by the Amazon EC2 API](https://docs.aws.amazon.com/AWSEC2/latest/APIReference/throttling.html#throttling-limits) )
	- **Note: If you are running multiple clusters within a single AWS account and region and risk hitting your [EC2 API Throttling Limits](https://docs.aws.amazon.com/AWSEC2/latest/APIReference/throttling.html#throttling-increase)**. See [Request a limit increase](https://docs.aws.amazon.com/AWSEC2/latest/APIReference/throttling.html#throttling-limits-rate-based) and the below [Fine-tuning the CSI Sidecar worker-threads parameter](faq.md) section
3. Increasing the default CSI sidecar `kube-api-qps` (to 20) and `kube-api-burst` (to 100) for all sidecars ([#1834](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/1834))
	- Each sidecar can now send a burst of up to 100 [queries](https://kubernetes.io/docs/reference/using-api/api-concepts/#api-verbs) to the Kubernetes API Server before throttling itself. It will then allow up to 20 more requests per second until it stops bursting.   
	- This keeps Kubernetes objects (`PersistentVolume`, `PersistentVolumeClaim`, and `VolumeAttachment`) more synchronized with the actual state of AWS resources, at the cost of increasing the load on the K8s API Server from `ebs-csi-controller` pods when many volume operations are happening at once. 
4. Increasing the default CSI sidecar `timeout` values (from 15s to 60s) for all sidecars ([#1824](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/1824))   
    - E.g. the [external-attacher](https://github.com/kubernetes-csi/external-attacher?tab=readme-ov-file#csi-error-and-timeout-handling) will now give the driver up to 60s to report an attachment success/failure before retrying a `ControllerPublishVolume` call. Now the external-attacher won't prematurely time out a `ControllerPublishVolume` call that would've taken 20s before returning a success response.  
	- This decreases the number of premature timeouts for CSI RPC calls, which reduces the number of replay EC2 API requests made by and waited for by the driver (at the cost of a longer delay during a real driver timeout (e.g. network blip leads to lost `ControllerPublishVolume` response))

### EC2 and K8s CSI Sidecar Throttling Overview

Both the EC2 API and K8s CSI sidecars base their API throttling implementation off of the [token bucket algorithm](https://en.wikipedia.org/wiki/Token_bucket). The Amazon EC2 API Reference provides a thorough explanation and example of how this algorithm is applied: [Request throttling for the Amazon EC2 API](https://docs.aws.amazon.com/AWSEC2/latest/APIReference/throttling.html#throttling-limits-rate-based)

Cluster operators can set the CSI Sidecar `--kube-api-burst` (i.e. bucket size) and `--kube-api-qps` (i.e. bucket refill rate) parameters in order to fine-tune how strictly these containers throttle their queries towards the K8s API Server. 

#### Further Reading
- [Request throttling for the Amazon EC2 API | Amazon Web Services](https://docs.aws.amazon.com/AWSEC2/latest/APIReference/throttling.html#throttling-limits-rate-based)
- [Managing and monitoring API throttling in your workloads | Amazon Web Services](https://aws.amazon.com/blogs/mt/managing-monitoring-api-throttling-in-workloads/)
- [Reference: kube-apiserver | Kubernetes](https://kubernetes.io/docs/reference/command-line-tools-reference/kube-apiserver/)
- [API Priority and Fairness | Kubernetes](https://kubernetes.io/docs/concepts/cluster-administration/flow-control/)

### Fine-tuning CSI sidecar scalability parameters

In [aws-ebs-csi-driver v1.25.0](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/blob/master/CHANGELOG.md#v1250), we changed the following K8s CSI external sidecar parameters to more sensible defaults. See the summary section for an overview of how these parameters affect volume lifecycle management. 

- `--worker-threads` (named `--workers` in [external-resizer](https://github.com/kubernetes-csi/external-resizer))
- `--kube-api-burst`
- `--kube-api-qps`
- `--timeout`

The AWS EBS CSI Driver provides a set of default values intended to balance performance while reducing the risk of reaching the default EC2 rate limits for API calls. 

Cluster operators can increase the `--kube-api-qps` and `--kube-api-burst` of each sidecar to keep the state of Kubernetes objects more in sync with their associated AWS resources. 

Cluster operators that need a greater throughput of volume operations should increase the associated sidecar's `worker-threads` value. When increased, the AWS Account's EC2 API limits may need to be raised to account for the increased rate of API calls.

| Sidecar | Configuration Name | Description | EC2 API Calls Made By Driver                             |
| ---- | ---- | ---- |----------------------------------------------------------|
| [external-provisioner](https://github.com/kubernetes-csi/external-provisioner) | provisioner | Watches PersistentVolumeClaim objects and triggers CreateVolume/DeleteVolume | EC2 CreateVolume/DeleteVolume                            |
| [external-attacher](https://github.com/kubernetes-csi/external-attacher) | attacher | Watches VolumeAttachment objects and triggers ControllerPublish/Unpublish | EC2 AttachVolume/DetachVolume, EC2 DescribeInstances     |
| [external-resizer](https://github.com/kubernetes-csi/external-resizer) | resizer | Watches PersistentVolumeClaims objects and triggers controller side expansion operation | EC2 ModifyVolume, EC2 DescribeVolumesModifications       |
| [external-snapshotter](https://github.com/kubernetes-csi/external-snapshotter) | snapshotter | Watches Snapshot CRD objects and triggers CreateSnapshot/DeleteSnapshot | EC2 CreateSnapshot/DeleteSnapshot, EC2 DescribeSnapshots |

#### Sidecar Fine-tuning Examples

Create a file named `example-ebs-csi-config-values.yaml` with the following yaml:

```yaml
sidecars:
  provisioner:
    additionalArgs:
    - "--worker-threads=101"
    - "--kube-api-burst=200"
    - "--kube-api-qps=40.0"
    - "--timeout=61s"
  resizer:
    additionalArgs:
    - "--workers=101"
```

**Note: The external-resizer uses the `--workers` parameter instead of `--worker-threads`

<details><summary>Self-managed Helm instructions</summary> 

Pass in the configuration-values file when installing/upgrading `aws-ebs-csi-driver`

```sh
helm upgrade --install aws-ebs-csi-driver \
--namespace kube-system \
--values example-ebs-csi-config-values.yaml
aws-ebs-csi-driver/aws-ebs-csi-driver
```
</details>


<details><summary>EKS-managed add-on instructions</summary>

Pass in the add-on configuration-values file when: 

Creating your add-on:
```sh
ADDON_CONFIG_FILEPATH="./example-addon-config.yaml"

aws eks create-addon \
  --cluster-name "example-cluster" \
  --addon-name "aws-ebs-csi-driver" \
  --service-account-role-arn "arn:aws:iam::123456789012:role/EBSCSIDriverRole" \
  --configuration-values "file://$ADDON_CONFIG_FILEPATH"
```

Updating your add-on:

```sh
ADDON_CONFIG_FILEPATH="./example-addon-config.yaml"

aws eks update-addon \
  --cluster-name "example-cluster" \
  --addon-name "aws-ebs-csi-driver" \
  --configuration-values "file://$ADDON_CONFIG_FILEPATH"
```
</details>

Confirm that these arguments were set by describing a `ebs-csi-controller` pod and observing the following args under the relevant sidecar container:

```yaml
Name: ebs-csi-controller-...
...
Containers:
  ...
  csi-provisioner:
    ...
    Args:
    ...
      --worker-threads=101
      --kube-api-burst=200
      --kube-api-qps=40.0
      --timeout=61s
```

## IMDSv2 Support

The driver supports the use of [IMDSv2](https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/configuring-instance-metadata-service.html) (Instance Metadata Service Version 2).

To use IMDSv2 with the driver in a containerized environment like Amazon EKS, please ensure that the hop limit for IMDSv2 responses is set to 2 or greater. This is because the default hop limit of 1 is incompatible with containerized applications on Kubernetes that run in a separate network namespace from the instance.

## CreateVolume (`StorageClass`) Parameters

### `ext4BigAlloc` and `ext4ClusterSize`

Warnings:
- Ext4's `bigalloc` is an experimental feature, under active development. Please pay particular attention to your node's kernel version. See the [ext4(5) man-page](https://man7.org/linux/man-pages/man5/ext4.5.html) for details.
- Linux kernel release 4.15 added support for resizing ext4 filesystems using clustered allocation. **Resizing volumes mounted to nodes running a Linux kernel version prior to 4.15 will fail.** 
