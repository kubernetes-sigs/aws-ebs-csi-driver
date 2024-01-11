# Frequently Asked Questions

## Driver performance for large-scale clusters

### Summary of v1.25 scalability improvements

[Version 1.25 of aws-ebs-csi-driver](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/blob/master/CHANGELOG.md#v1250) featured four improvements to better manage the EBS volume lifecycle for large-scale clusters. 

At a high-level:
1. Batching EC2 `DescribeVolumes` API Calls across CSI gRPC calls ([#1819](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/1819))
	- This greatly decreases the amount of EC2 `DescribeVolumes` calls made by the driver, which significantly reduces your risk of region-level throttling of the ['Describe*' EC2 API Action](https://docs.aws.amazon.com/AWSEC2/latest/APIReference/throttling.html#throttling-limits)
2. Increasing the default CSI sidecar `worker-threads` values (to 100 for all sidecars) (`workers` for external-resizer) ([#1834](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/1834))
	- E.g. the `external-provisioner` can be simultaneously running 100 `ControllerPublishVolume` operations, the `external-attacher` now has 100 goroutines for processing VolumeAttachments, etc. 
	- This increases the amount of in-flight EBS volume creations / attaches / modifications / deletions managed by the driver (which may increase your risk of region-level ['Mutating action' request throttling by the Amazon EC2 API](https://docs.aws.amazon.com/AWSEC2/latest/APIReference/throttling.html#throttling-limits) )
	- **Note: If you are running multiple clusters within a single AWS account and region and risk hitting your [EC2 API Throttling Limits](https://docs.aws.amazon.com/AWSEC2/latest/APIReference/throttling.html#throttling-increase)**. See [Request a limit increase](https://docs.aws.amazon.com/AWSEC2/latest/APIReference/throttling.html#throttling-limits-rate-based) and [Fine-tuning the CSI Sidecar worker-threads parameter]()
3. Increasing the default CSI sidecar `kube-api-qps` (to 20) and `kube-api-burst` (to 100) for all sidecars ([#1834](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/1834))
	- Each sidecar can now send a burst of up to 100 [queries](https://kubernetes.io/docs/reference/using-api/api-concepts/#api-verbs) to the Kubernetes API Server before throttling itself. It will then allow up to 20 more requests per second until it stops bursting.   
	- This keeps Kubernetes objects (`PersistentVolume`, `PersistentVolumeClaim`, and `VolumeAttachment`) more synchronized with the actual state of AWS resources, at the cost of increasing the load on the K8s API Server from `ebs-csi-controller` pods when many volume operations are happening at once. 
4. Increasing the default CSI sidecar `timeout` values (from 15s to 60s) for all sidecars ([#1824](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/1824))   
  - E.g. the [external-attacher](https://github.com/kubernetes-csi/external-attacher?tab=readme-ov-file#csi-error-and-timeout-handling) will now give the driver up to 60s to report an attachment success/failure before retrying a `ControllerPublishVolume` call. Now the external-attacher won't prematurely timeout a `ControllerPublishVolume` call that would've taken 20s before returning a success response.  
	- This decreases the amount of premature timeouts for CSI RPC calls, which reduces the amount of replay EC2 API requests made by and waited for by the driver (at the cost of a longer delay during a real driver timeout (e.g. network blip leads to lost `ControllerPublishVolume` response))
## EC2 and K8s CSI Sidecar Throttling Overview

Both the EC2 API and K8s CSI sidecars base their API throttling implementation off of the [token bucket algorithm](https://en.wikipedia.org/wiki/Token_bucket). Both the [Amazon EC2 API Reference](https://docs.aws.amazon.com/AWSEC2/latest/APIReference/throttling.html#throttling-limits-rate-based) and the [K8s documentation definitions of qps and burst](https://kubernetes.io/docs/reference/config-api/apiserver-eventratelimit.v1alpha1/#eventratelimit-admission-k8s-io-v1alpha1-Limit) describe their throttling systems.  

Let's look at an example from [Request throttling for the Amazon EC2 API](https://docs.aws.amazon.com/AWSEC2/latest/APIReference/throttling.html#throttling-limits-rate-based)

<details><summary>Expand for excerpt:</summary>

"With request rate limiting, you are throttled on the number of API requests you make. Each request that you make removes one token from the bucket. For example, the bucket size for _non-mutating_ (`Describe*`) API actions is 100 tokens, so you can make up to 100 `Describe*` requests in one second. If you exceed 100 requests in a second, you are throttled and the remaining requests within that second fail.

Buckets automatically refill at a set rate. If the bucket is below its maximum capacity, a set number of tokens is added back to it every second until it reaches its maximum capacity. If the bucket is full when refill tokens arrive, they are discarded. The bucket cannot hold more than its maximum number of tokens. For example, the bucket size for _non-mutating_ (`Describe*`) API actions is 100 tokens, and the refill rate is 20 tokens per second. If you make 100 `Describe*` API requests in a second, the bucket is immediately reduced to zero (0) tokens. The bucket is then refilled by 20 tokens every second, until it reaches its maximum capacity of 100 tokens. This means that the previously empty bucket reaches its maximum capacity after 5 seconds.

You do not need to wait for the bucket to be completely full before you can make API requests. You can use tokens as they are added to the bucket. If you immediately use the refill tokens, the bucket does not reach its maximum capacity. For example, the bucket size for _console non-mutating actions_ is 100 tokens, and the refill rate is 10 tokens per second. If you deplete the bucket by making 100 API requests in a second, you can continue to make 10 API requests per second. The bucket can refill to the maximum capacity only if you make fewer than 10 API requests per second."</details>

Analogous to the above example, cluster operators can set the CSI Sidecar `--kube-api-burst` (i.e. bucket size) and `--kube-api-qps` (i.e. refill rate) parameters in order to fine-tune how strictly these containers throttle their queries towards the K8s API Server. 

#### Further Reading
- AWS
	- [Managing and monitoring API throttling in your workloads](https://aws.amazon.com/blogs/mt/managing-monitoring-api-throttling-in-workloads/)
- K8s
	- [Reference: kube-apiserver | Kubernetes](https://kubernetes.io/docs/reference/command-line-tools-reference/kube-apiserver/)
	- [Rate Limiting in controller-runtime and client-go](https://danielmangum.com/posts/controller-runtime-client-go-rate-limiting/)
	- [API Priority and Fairness | Kubernetes](https://kubernetes.io/docs/concepts/cluster-administration/flow-control/)

### Fine-tuning CSI sidecar scalability parameters

In [aws-ebs-csi-driver v1.25.0](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/blob/76ae539ae3129b408cd184c5cdfd1ffe476dab1d/CHANGELOG.md?plain=1#L35-L36), we changed the following K8s CSI external sidecar parameters to more sensible defaults. See the summary section for an overview of how these parameters affect volume lifecycle management. 

- `worker-threads` (named `workers` for [external-resizer](https://github.com/kubernetes-csi/external-resizer))
- `kube-api-burst`
- `kube-api-qps`
- `timeout`

If your workloads need a higher throughput of volume operations, or you have higher AWS limits for any of the EC2 APIs in the `EC2 API Calls Made By Driver` column, you may increase consider increasing the associated sidecar's `worker-threads` value. 

| Sidecar | Configuration Name | Description | EC2 API Calls Made By Driver |
| ---- | ---- | ---- | ---- |
| [external-provisioner](https://github.com/kubernetes-csi/external-provisioner) | provisioner | Watches PersistentVolumeClaim objects and triggers CreateVolume/DeleteVolume | EC2 CreateVolume/DeleteVolume |
| [external-attacher](https://github.com/kubernetes-csi/external-attacher) | attacher | Watches VolumeAttachment objects and triggers ControllerPublish/Unpublish | EC2 AttachVolume/DetachVolume |
| [external-resizer](https://github.com/kubernetes-csi/external-resizer) | resizer | Watches PersistentVolumeClaims objects and triggers controller side expansion operation | EC2 ModifyVolume, EC2 DescribeVolumesModifications |
| [external-snapshotter](https://github.com/kubernetes-csi/external-snapshotter) | snapshotter | Watches Snapshot CRD objects and triggers CreateSnapshot/DeleteSnapshot | EC2 CreateSnapshot/DeleteSnapshot, EC2 DescribeSnapshots |

#### Sidecar Fine-tuning Examples

**Note: The external-resizer uses the `--workers` parameter instead of `--worker-threads`
##### Helm

Make the following diff in the helm chart's default [values.yaml](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/blob/master/charts/aws-ebs-csi-driver/values.yaml) file

```diff
sidecars:
  provisioner:
    ...
    # Additional parameters provided by external-provisioner.
-   additionalArgs: []
+   additionalArgs:
+   - "--worker-threads=101"
+   - "--kube-api-burst=200"
+   - "--kube-api-qps=40.0"
+   - "--timeout=61s"
...
  resizer:
-   additionalArgs: []
+   additionalArgs:
+   - "--workers=101"
```

##### EKS Managed Add-on

Create a file named `example-addon-config.yaml` with the following yaml:

```yaml
sidecars:
  attacher:
    additionalArgs:
    - "--worker-threads=101"
    - "--kube-api-burst=200"
    - "--kube-api-qps=40.0"
    - "--timeout=61s"
  resizer:
    additionalArgs:
    - "--workers=101"
```

Pass in the add-on configuration-values file when creating your add-on: 

```
ADDON-CONFIG-FILEPATH="./example-addon-config.yaml"

aws eks create-addon \
  --cluster-name "example-cluster" \
  --addon-name "aws-ebs-csi-driver" \
  --service-account-role-arn "arn:aws:iam::123456789012:role/EBSCSIDriverRole" \
  --configuration-values "file://$ADDON-CONFIG-FILEPATH"
```

You can confirm that these arguments were set by describing am `ebs-csi-controller` pod and observing the following args under the relevant sidecar container:

```yaml
Name: ebs-csi-controller-6d57fcdfb6-9wpjr
...
Containers:
  ...
  csi-attacher:
    ...
    Args:
    ...
      --worker-threads=101
      --kube-api-burst=200
      --kube-api-qps=40.0
      --timeout=61s
```

## CreateVolume (`StorageClass`) Parameters

### `ext4BigAlloc` and `ext4ClusterSize`

Warnings:
- Ext4's `bigalloc` is an experimental feature, under active development. Please pay particular attention to your node's kernel version. See the [ext4(5) man-page](https://man7.org/linux/man-pages/man5/ext4.5.html) for details.
- Linux kernel release 4.15 added support for resizing ext4 filesystems using clustered allocation. **Resizing volumes mounted to nodes running a Linux kernel version prior to 4.15 will fail.** 
