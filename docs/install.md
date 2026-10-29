# Installation

## Prerequisites

* Kubernetes Version >= 1.20 

* If you are using a self-managed cluster, ensure the flag `--allow-privileged=true` for `kube-apiserver`.

* Important: If you intend to use the Volume Snapshot feature, the [Kubernetes Volume Snapshot CRDs](https://github.com/kubernetes-csi/external-snapshotter/tree/master/client/config/crd) must be installed **before** the EBS CSI driver. For installation instructions, see [CSI Snapshotter Usage](https://github.com/kubernetes-csi/external-snapshotter#usage).

### Instance type, zone, and attachment limit metadata sources

The EBS CSI Driver needs to know which instance it runs on (instance ID and type), its region and Availability Zone, and how many EBS volumes the node can still attach, which it reports to Kubernetes for scheduling. It reads this from a metadata source. The driver currently supports three metadata sources: [IMDS](https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/ec2-instance-metadata.html), Kubernetes, and the (alpha) metadata labeler.

The controller `Deployment` can skip metadata if the region is provided via the `AWS_REGION` environment variable (Helm parameter `controller.region`). The node `DaemonSet` requires metadata and will not function without access to one of the sources.

You may override the default metadata behavior of attempting IMDS, then falling back to Kubernetes, through the `--metadata-sources` flag.

#### IMDS (EC2) Metadata

If the driver is able to access IMDS, it will utilize that as a preferred source of metadata. IMDS is the only source that reports the ENIs and EBS volumes attached to the instance, which the node needs to compute its volume attachment limit (see [Kubernetes Metadata](#kubernetes-metadata)).

The EBS CSI Driver supports IMDSv1 and IMDSv2 (and will prefer IMDSv2 if both are available). With IMDSv2's [default hop limit of 1](https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/configuring-instance-metadata-service.html#instance-metadata-v2-how-it-works), a pod in its own network namespace cannot reach IMDS. To give the node pods access to IMDS, run them with host networking (Helm parameter `node.hostNetwork: true`), so that the hop limit of 1 still works.

Don't raise the hop limit to 2 instead: it is set per instance, so it exposes IMDS, and the node's IAM instance role credentials, to every pod on the node. Only the node pods need IMDS; set `controller.region` so that the controller does not.

#### Kubernetes Metadata

With this source, the driver reads its metadata from the Kubernetes API. For this metadata source to work, the driver pods must have access to the Kubernetes API server. Additionally, the Kubernetes node objects must include the following information:

- Instance ID (in the `Node`'s `ProviderID`)
- Instance Type (in the label `node.kubernetes.io/instance-type`)
- Instance Region (in the label `topology.kubernetes.io/region`)
- Instance AZ (in the label `topology.kubernetes.io/zone`)

These values are typically set by the [AWS CCM](https://github.com/kubernetes/cloud-provider-aws). You must have the AWS CCM or a similar tool installed in your cluster providing these values for Kubernetes metadata to function.

Kubernetes metadata does not provide information about the number of ENIs or EBS volumes attached to an instance. Thus, when performing volume limit calculations, node pods using Kubernetes metadata will assume one ENI and one EBS volume (the root volume) is attached. On instance types with a [shared volume attachment limit](https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/volume_limits.html), where ENIs and NVMe instance store volumes count against the same limit as EBS volumes, the node can over-report how many volumes it can attach (see [#2543](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/issues/2543)). The metadata labeler below provides these counts without IMDS access. A future Kubernetes feature, `CSIControllerGetNodeInfo` ([KEP-6011](https://github.com/kubernetes/enhancements/issues/6011), targeting alpha in Kubernetes v1.38), will let the driver's controller report each node's attachment limit, so that nodes can use Kubernetes metadata without IMDS.

#### Metadata Labeler

**Note: This metadata source is currently in alpha and disabled by default.**

The `metadata-labeler` sidecar (and corresponding metadata source) allow passing information from the EC2 API via labels on the sidecars, similar to the labels applied by the AWS CCM used by the `kubernetes` source.

For the Instance ID, type, region, and AZ, this metadata source uses the same logic as the `kubernetes` metadata source and has the same requirements. In addition, this metadata source uses labels applied by the EBS CSI `metadata-labeler` sidecar for the number of ENIs and extra EBS volumes attached to an instance.

To enable this metadata source:
- Set `sidecars.metadataLabeler.enabled` to `true`
- Include `metadata-labeler` in `node.metadataSources` list. E.g. setting `node.metadataSources` to `"metadata-labeler,kubernetes"` will first attempt to use this new metadata source, then fallback to Kubernetes metadata.
- EBS CSI Controller Pods must hold Kubernetes RBAC permission to patch Node objects (this is automatically enabled in the EBS CSI Helm chart via `sidecars.metadataLabeler.enabled`).

## Installation
### Set up driver permissions

> [!NOTE]  
> The example policy and documentation below use the [`aws` partition in ARNs](https://docs.aws.amazon.com/IAM/latest/UserGuide/reference-arns.html). When installing the EBS CSI Driver on other partitions, replace instances of `arn:aws:` with the local partition, such as `arn:aws-us-gov:` for AWS GovCloud.

The driver requires IAM permissions to talk to Amazon EBS to manage volumes on the user's behalf. Only the controller (`ebs-csi-controller-sa`) needs AWS credentials. There are two policies to choose from:

* **`AmazonEBSCSIDriverEKSClusterScopedPolicy` (recommended for new EKS clusters)**: [The example policy here](./AmazonEBSCSIDriverEKSClusterScopedPolicy.json) matches the [AWS managed policy](https://docs.aws.amazon.com/aws-managed-policy/latest/reference/AmazonEBSCSIDriverEKSClusterScopedPolicy.html), available at ARN `arn:aws:iam::aws:policy/AmazonEBSCSIDriverEKSClusterScopedPolicy`. It limits each cluster's driver to the volumes and snapshots tagged `ebs.csi.aws.com/cluster-name=<cluster name>`, so clusters that share an AWS account are isolated from each other. It requires [EKS Pod Identity](#eks-only-eks-pod-identity), which supplies the cluster name, and `controller.k8sTagClusterId` (`--k8s-tag-cluster-id`) set to the EKS cluster name, so that the driver adds that tag to the volumes and snapshots it creates. A volume or snapshot carries one cluster name, so this policy cannot share resources between clusters.
* **`AmazonEBSCSIDriverPolicyV2`**: [The example policy here](./AmazonEBSCSIDriverPolicyV2.json) defines these permissions. AWS maintains a [managed policy version of the example policy](https://docs.aws.amazon.com/aws-managed-policy/latest/reference/AmazonEBSCSIDriverPolicyV2.html), available at ARN `arn:aws:iam::aws:policy/service-role/AmazonEBSCSIDriverPolicyV2`. It scopes the driver to volumes and snapshots tagged `ebs.csi.aws.com/cluster=true`, a tag every cluster's driver writes. Use it when volumes or snapshots move between clusters in one account, for example to statically provision a volume or restore a snapshot that another cluster created. It does not isolate clusters from each other. Use it also with IRSA, a `Secret`, or anywhere EKS Pod Identity is not available.

> [!IMPORTANT]
> The `AmazonEBSCSIDriverEKSClusterScopedPolicy` steps below are for new clusters. In an existing cluster, volumes and snapshots created before `controller.k8sTagClusterId` was set have no `ebs.csi.aws.com/cluster-name` tag, so the policy denies attaching, resizing, snapshotting, restoring or deleting them. Set `controller.k8sTagClusterId` first, then tag the existing volumes and snapshots (and any self-managed node instances) `ebs.csi.aws.com/cluster-name=<cluster name>` before you replace `AmazonEBSCSIDriverPolicyV2` with this policy.

Both policies scope permissions by tag to the volumes and snapshots the EBS CSI Driver created. Review the following if this affects your workflow:

<details>
<summary>Static provisioning (importing externally-created volumes or snapshots)</summary>
<br>
<code>AmazonEBSCSIDriverPolicyV2</code> scopes permissions to volumes and snapshots tagged with <code>ebs.csi.aws.com/cluster: true</code>, and <code>AmazonEBSCSIDriverEKSClusterScopedPolicy</code> to those tagged with <code>ebs.csi.aws.com/cluster-name: &lt;cluster name&gt;</code>. The driver automatically applies both tags to dynamically provisioned resources (the second one when <code>controller.k8sTagClusterId</code> is set). If you use <a href="https://github.com/kubernetes-sigs/aws-ebs-csi-driver/tree/master/examples/kubernetes/static-provisioning">static provisioning</a> (i.e. importing externally-created EBS volumes or snapshots), you must manually tag those resources with the tag your policy checks for the driver to manage them. For more details, see <a href="https://github.com/kubernetes-sigs/aws-ebs-csi-driver/issues/2918">the announcement</a>.
</details>

Both policies also exclude permissions for some rarer and potentially dangerous usecases. For these usecases, additional statements are necessary:

<details>
<summary>Encrypted EBS Volumes via KMS</summary>
<br>
To create and manage encrypted EBS volumes, the EBS CSI Driver requires access to the KMS key(s) used for encryption/decryption of the volume(s). The below example grants the EBS CSI Driver access to all KMS keys in the account, but it is best practice to restrict the resource to only the keys the EBS CSI Driver needs access to.
<pre>
{
  "Effect": "Allow",
  "Action": [
    "kms:Decrypt",
    "kms:GenerateDataKeyWithoutPlaintext",
    "kms:CreateGrant"
  ],
  "Resource": "arn:aws:kms:*:*:key/*"
}
</pre>
</details>

<details>
<summary>Modifying tags of existing volumes</summary>
<br>
Both policies let the driver modify tags (for VolumeAttributesClass tag changes) only on volumes it manages, and never the tags the policies check. Modification of tags of other volumes and snapshots can allow the driver to bypass tag-based policies and restrictions, so it is not included in either policy. Do not add it alongside <code>AmazonEBSCSIDriverEKSClusterScopedPolicy</code>: the driver could then tag any volume or snapshot for its own cluster, which defeats cluster isolation. Below is an example statement that will grant the EBS CSI Driver the ability to modify tags of any volume or snapshot:
<pre>
{ 
  "Effect": "Allow",
  "Action": [
    "ec2:CreateTags"
  ],
  "Resource": [
    "arn:aws:ec2:*:*:volume/*",
    "arn:aws:ec2:*:*:snapshot/*"
  ]
}
</pre>
</details>

There are several options to pass credentials to the EBS CSI Driver, each documented below:

#### (EKS Only) EKS Pod Identity

[EKS Pod Identity](https://docs.aws.amazon.com/eks/latest/userguide/pod-identities.html) is the recommended method to provide IAM credentials to pods running on EKS clusters. 

When using EKS pod identity with the EBS CSI Driver, [configure the role's trust policy and assign it](https://docs.aws.amazon.com/eks/latest/userguide/pod-id-association.html) to the `ebs-csi-controller-sa` service account in the namespace the EBS CSI Driver is deployed (typically `kube-system`). Using EKS Pod Identity requires [installation of the EKS Pod Identity agent](https://docs.aws.amazon.com/eks/latest/userguide/pod-id-agent-setup.html) if it is not already installed on the cluster.

To use `AmazonEBSCSIDriverEKSClusterScopedPolicy` with EKS Pod Identity, follow these steps. One IAM role and policy can be shared by every cluster in the account, and each cluster's driver is still limited to its own volumes and snapshots: the trust policy names no cluster, and EKS adds the cluster name to the credentials as the `eks-cluster-name` session tag. Create the role once, and repeat steps 1, 3 and 4 for each cluster.

1. Install the EKS Pod Identity Agent add-on, if it is not already installed. The agent must be installed as an EKS add-on; the driver itself can be installed with Helm, Kustomize or as the EKS add-on.
   ```sh
   aws eks create-addon --cluster-name my-cluster --addon-name eks-pod-identity-agent
   ```

2. Create a role with the standard [EKS Pod Identity trust policy](https://docs.aws.amazon.com/eks/latest/userguide/pod-id-role.html), and attach the managed policy:
   ```sh
   aws iam create-role --role-name ebs-csi-driver-cluster-scoped \
     --assume-role-policy-document file://pod-identity-trust-policy.json
   aws iam attach-role-policy --role-name ebs-csi-driver-cluster-scoped \
     --policy-arn arn:aws:iam::aws:policy/AmazonEBSCSIDriverEKSClusterScopedPolicy
   ```
   Optionally, also attach a policy that denies EC2 calls outside the cluster's region; see [below](#optional-limit-the-driver-to-its-clusters-region).

3. Associate the role with the controller service account. Do this before you install the driver, so that the first controller pod gets credentials:
   ```sh
   aws eks create-pod-identity-association --cluster-name my-cluster --namespace kube-system \
     --service-account ebs-csi-controller-sa \
     --role-arn arn:aws:iam::111122223333:role/ebs-csi-driver-cluster-scoped
   ```

4. When you [deploy the driver](#deploy-driver), set the cluster ID to the EKS cluster name, because the policy compares the `ebs.csi.aws.com/cluster-name` tag with the `eks-cluster-name` session tag:
   ```sh
   helm upgrade --install aws-ebs-csi-driver aws-ebs-csi-driver/aws-ebs-csi-driver --namespace kube-system \
     --set controller.k8sTagClusterId=my-cluster
   ```
   Without Helm, pass `--k8s-tag-cluster-id=my-cluster` to the `controller` command.

##### Optional: limit the driver to its cluster's region

`AmazonEBSCSIDriverEKSClusterScopedPolicy` allows every region. The driver makes no cross-region calls, so you can also attach this deny-only policy to the role. It denies every EC2 call outside the region of the cluster named by the `eks-cluster-arn` session tag:
```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Sid": "DenyEC2OutsideClusterRegion",
      "Effect": "Deny",
      "Action": "ec2:*",
      "Resource": "*",
      "Condition": {
        "StringNotLike": {
          "aws:PrincipalTag/eks-cluster-arn": "arn:*:eks:${aws:RequestedRegion}:*:cluster/*"
        }
      }
    }
  ]
}
```
A deny-only policy grants nothing, so it is safe to attach alongside any allow policy. It fails closed: credentials without the `eks-cluster-arn` session tag (anything other than EKS Pod Identity) are denied every EC2 call. It covers only `ec2:*`, so the KMS calls EBS makes on the driver's behalf for encrypted volumes are not affected.

#### IAM Roles for ServiceAccounts (i.e. IRSA)

[IAM roles for ServiceAccounts](https://docs.aws.amazon.com/eks/latest/userguide/iam-roles-for-service-accounts.html) is a method of enabling pods using a Kubernetes ServiceAccount to exchange the service account token for IAM credentials. Using IRSA requires a specially configured trust policy on the role, as well as setup of an OIDC endpoint for the cluster. On EKS, [refer to the EKS docs for setting up IRSA](https://docs.aws.amazon.com/eks/latest/userguide/iam-roles-for-service-accounts.html#:~:text=Enable%20IAM%20roles%20for%20service%20accounts%20by%20completing%20the%20following%20procedures:). On self-managed clusters, IRSA requires a public OIDC issuer for the cluster, an IAM OIDC identity provider for that issuer, and the [`amazon-eks-pod-identity-webhook`](https://github.com/aws/amazon-eks-pod-identity-webhook) running in the cluster.

When deploying via Helm, the parameter `controller.serviceAccount.annotations` can be used to add the necessary annotation for IRSA, for example with the following values:
```yaml
controller:
  serviceAccount:
    annotations:
      eks.amazonaws.com/role-arn: arn:aws:iam::123412341234:role/ebs-csi-role
```

`AmazonEBSCSIDriverEKSClusterScopedPolicy` does not work with IRSA, because IRSA credentials carry no cluster name. Use `AmazonEBSCSIDriverPolicyV2` with IRSA.

#### Secret Object

When deplying via Helm, the chart can be configured to pass IAM credentials stored in a Kubernetes `Secret` to the EBS CSI Driver. This option may be useful in confunction with third party software that stores credentials in a secret. This is configured using the `awsAccessSecret` Helm parameter:
```yaml
awsAccessSecret:
  name: aws-secret # This should be the name of the secret (must be in the same namespace as the driver deployment, typically kube-system)
  keyId: key_id # This is the name of the key on the secret that holds the AWS Key ID
  accessKey: access_key # This is the name of the key on the secret that holds the AWS Secret Access Key
```

### Configure driver toleration settings
By default, the driver controller tolerates taint `CriticalAddonsOnly` and has `tolerationSeconds` configured as `300`; and the driver node tolerates all taints. If you don't want to deploy the driver node on all nodes, please set Helm `Value.node.tolerateAllTaints` to false before deployment. Add policies to `Value.node.tolerations` to configure customized toleration for nodes.

### Configure node startup taint
There are potential race conditions on node startup (especially when a node is first joining the cluster) where pods/processes that rely on the EBS CSI Driver can act on a node before the EBS CSI Driver is able to startup up and become fully ready. To combat this, the EBS CSI Driver contains a feature to automatically remove a taint from the node on startup. Users can taint their nodes when they join the cluster and/or on startup, to prevent other pods from running and/or being scheduled on the node prior to the EBS CSI Driver becoming ready.

This feature is activated by default, and cluster administrators should use the taint `ebs.csi.aws.com/agent-not-ready:NoExecute` (any effect will work, but `NoExecute` is recommended). For example, EKS Managed Node Groups [support automatically tainting nodes](https://docs.aws.amazon.com/eks/latest/userguide/node-taints-managed-node-groups.html).

### Deploy driver
You may deploy the EBS CSI driver via Kustomize, Helm, or as an [Amazon EKS managed add-on](https://docs.aws.amazon.com/eks/latest/userguide/managing-ebs-csi.html).

#### Kustomize
```sh
kubectl apply -k "github.com/kubernetes-sigs/aws-ebs-csi-driver/deploy/kubernetes/overlays/stable/?ref=release-1.67"
```

*Note: Using the master branch to deploy the driver is not supported as the master branch may contain upcoming features incompatible with the currently released stable version of the driver.*

#### Helm
Install from the GitHub Pages Helm repository:
```sh
helm repo add aws-ebs-csi-driver https://kubernetes-sigs.github.io/aws-ebs-csi-driver
helm repo update

helm upgrade --install aws-ebs-csi-driver \
    --namespace kube-system \
    aws-ebs-csi-driver/aws-ebs-csi-driver
```

Install from registry.k8s.io:
```sh
helm upgrade --install aws-ebs-csi-driver \
    --namespace kube-system \
    oci://registry.k8s.io/provider-aws/charts/aws-ebs-csi-driver \
    --version 2.67.2
```

Install from ECR Public:
```sh
helm upgrade --install aws-ebs-csi-driver \
    --namespace kube-system \
    oci://public.ecr.aws/ebs-csi-driver/charts/aws-ebs-csi-driver \
    --version 2.67.2
```

Review the [configuration values](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/blob/master/charts/aws-ebs-csi-driver/values.yaml) for the Helm chart.
For each container (including the controller, node, and sidecars), there is an `additionalArgs` that accepts arguments that are not explicitly specified, such as `--retry-interval-start`, `--retry-interval-max` and
`--timeout` that provisioner and attacher provides, or `--kube-api-burst`, `--kube-api-qps` etc.

#### Once the driver has been deployed, verify the pods are running:
```sh
kubectl get pods -n kube-system -l app.kubernetes.io/name=aws-ebs-csi-driver
```

### Upgrading from version 1.X to 2.X of the Helm chart
Version 2.0.0 removed support for Helm v2 and now requires Helm v3 or above.

The [CSI Snapshotter](https://github.com/kubernetes-csi/external-snapshotter) controller and CRDs will no longer be installed as part of this chart and moving forward will be a prerequisite of using the snap shotting functionality.

The following deprecated values have been removed and users upgrading from version 1.x must now use their counterparts under the `controller` and `node` maps.
* affinity
* extraCreateMetadata
* extraVolumeTags
* k8sTagClusterId
* nodeSelector
* podAnnotations
* priorityClassName
* region
* replicaCount
* resources
* tolerations
* topologySpreadConstraints
* volumeAttachLimit

The values under `serviceAccount.controller` have been relocated to `controller.serviceAccount`
The values under `serviceAccount.node` have been relocated to `node.serviceAccount`

The following `sidecars` values have been reorganized from
```yaml
sidecars:
  provisionerImage:
  attacherImage:
  snapshotterImage:
  livenessProbeImage:
  resizerImage:
  nodeDriverRegistrarImage:
```
to
```yaml
sidecars:
  provisioner:
    image:
  attacher:
    image:
  snapshotter:
    image:
  livenessProbe:
    image:
  resizer:
    image:
  nodeDriverRegistrar:
    image:
```

With the above reorganization `controller.containerResources`, `controller.env`, `node.containerResources`, and `node.env` were also moved into the sidecars structure as follows
```yaml
sidecars:
  provisioner:
    env: []
    resources: {}
  attacher:
    env: []
    resources: {}
  snapshotter:
    env: []
    resources: {}
  livenessProbe:
    resources: {}
  resizer:
    env: []
    resources: {}
  nodeDriverRegistrar:
    env: []
    resources: {}
```

## Migrating from in-tree EBS plugin
Starting from Kubernetes 1.17, CSI migration is supported as beta feature (alpha since 1.14). If you have persistent volumes that are created with in-tree `kubernetes.io/aws-ebs` plugin, you can migrate to use EBS CSI driver. To turn on the migration, set `CSIMigration` and `CSIMigrationAWS` feature gates to `true` for `kube-controller-manager`. Then drain Nodes and set the same feature gates to `true` for `kubelet`.

To make sure dynamically provisioned EBS volumes have all tags that the in-tree volume plugin used:
* Run the external-provisioner sidecar with `--extra-create-metadata=true` cmdline option. The Helm chart sets this option true by default.
* Run the CSI driver with `--k8s-tag-cluster-id=<ID of the Kubernetes cluster>` command line option.

**Warning**:
* kubelet *must* be drained of all pods with mounted EBS volumes ***before*** changing its CSI migration feature flags.  Failure to do this will cause deleted pods to get stuck in `Terminating`, requiring a forced delete which can cause filesystem corruption. See [#679](../../../issues/679) for more details.

## Uninstalling the EBS CSI Driver

Note: If your cluster is using EBS volumes, there should be no impact to running workloads. However, while the ebs-csi-driver daemonsets and controller are deleted from the cluster, no new EBS PVCs will be able to be created, and new pods that are created which use an EBS PV volume will not function (because the PV will not mount) until the driver is successfully re-installed (either manually, or through the [EKS add-on system](https://docs.aws.amazon.com/eks/latest/userguide/managing-ebs-csi.html)).

Uninstall the self-managed EBS CSI Driver with either Helm or Kustomize, depending on your installation method. If you are using the driver as a managed EKS add-on, see the [EKS Documentation](https://docs.aws.amazon.com/eks/latest/userguide/managing-ebs-csi.html).

**Helm**

```
helm uninstall aws-ebs-csi-driver --namespace kube-system
```

**Kustomize**

```
kubectl delete -k "github.com/kubernetes-sigs/aws-ebs-csi-driver/deploy/kubernetes/overlays/stable/?ref=release-<YOUR-CSI-DRIVER-VERION-NUMBER>"
```
