# Installation

## Prerequisites

* Kubernetes Version >= 1.20 

* If you are using a self-managed cluster, ensure the flag `--allow-privileged=true` for `kube-apiserver`.

* Important: If you intend to use the Volume Snapshot feature, the [Kubernetes Volume Snapshot CRDs](https://github.com/kubernetes-csi/external-snapshotter/tree/master/client/config/crd) must be installed **before** the EBS CSI driver. For installation instructions, see [CSI Snapshotter Usage](https://github.com/kubernetes-csi/external-snapshotter#usage).

### Metadata

The EBS CSI Driver uses a metadata source in order to gather necessary information about the environment to function. The driver currently supports two metadata sources: [IMDS](https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/ec2-instance-metadata.html) or Kubernetes.

The controller `Deployment` can skip metadata if the region is provided via the `AWS_REGION` environment variable (Helm parameter `controller.region`). The node `DaemonSet` requires metadata and will not function without access to one of the sources.

You may override the default metadata behavior of attempting IMDS, then falling back to Kubernetes, through the `--metadata-sources` flag.

#### IMDS (EC2) Metadata

If the driver is able to access IMDS, it will utilize that as a preferred source of metadata. The EBS CSI Driver supports IMDSv1 and IMDSv2 (and will prefer IMDSv2 if both are available). However, by default, [IMDSv2 uses a hop limit of 1](https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/configuring-instance-metadata-service.html#instance-metadata-v2-how-it-works). That will prevent the driver from accessing IMDSv2 if run inside a container with the default IMDSv2 configuration.

In order for the driver to access IMDS, it either must be run in host networking mode, or with a [hop limit of at least 2](https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/configuring-IMDS-existing-instances.html#modify-PUT-response-hop-limit).

#### Kubernetes Metadata

By default, if the driver is unable to reach IMDS, it will fall back to using the Kubernetes API. For this metadata source to work, the driver pods must have access to the Kubernetes API server. Additionally, the Kubernetes node objects must include the following information:

- Instance ID (in the `Node`'s `ProviderID`)
- Instance Type (in the label `node.kubernetes.io/instance-type`)
- Instance Region (in the label `topology.kubernetes.io/region`)
- Instance AZ (in the label `topology.kubernetes.io/zone`)

These values are typically set by the [AWS CCM](https://github.com/kubernetes/cloud-provider-aws). You must have the AWS CCM or a similar tool installed in your cluster providing these values for Kubernetes metadata to function.

Kubernetes metadata does not provide information about the number of ENIs or EBS volumes attached to an instance. Thus, when performing volume limit calculations, node pods using Kubernetes metadata will assume one ENI and one EBS volume (the root volume) is attached.

## Installation
### Set up driver permissions

> [!NOTE]  
> The example policy and documentation below use the [`aws` partition in ARNs](https://docs.aws.amazon.com/IAM/latest/UserGuide/reference-arns.html). When installing the EBS CSI Driver on other partitions, replace instances of `arn:aws:` with the local partition, such as `arn:aws-us-gov:` for AWS GovCloud.

The driver requires IAM permissions to talk to Amazon EBS to manage the volume on user's behalf. [The example policy here](./example-iam-policy.json) defines these permissions. AWS maintains a [managed policy version of the example policy](https://docs.aws.amazon.com/aws-managed-policy/latest/reference/AmazonEBSCSIDriverPolicy.html), available at ARN `arn:aws:iam::aws:policy/service-role/AmazonEBSCSIDriverPolicy`.

The baseline example policy excludes permissions for some rarer and potentially dangerous usecases. For these usecases, additional statements are necessary:

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
Modification of tags of existing volumes can, in some configurations, allow the driver to bypass tag-based policies and restrictions, so it is not included in the default policy. Below is an example statement that will grant the EBS CSI Driver the ability to modify tags of any volume or snapshot:
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

#### IAM Roles for ServiceAccounts (i.e. IRSA)

[IAM roles for ServiceAccounts](https://docs.aws.amazon.com/eks/latest/userguide/iam-roles-for-service-accounts.html) is a method of enabling pods using a Kubernetes ServiceAccount to exchange the service account token for IAM credentials. Using IRSA requires a specially configured trust policy on the role, as well as setup of an OIDC endpoint for the cluster. On EKS, [refer to the EKS docs for setting up IRSA](https://docs.aws.amazon.com/eks/latest/userguide/iam-roles-for-service-accounts.html#:~:text=Enable%20IAM%20roles%20for%20service%20accounts%20by%20completing%20the%20following%20procedures:). On other cluster providers, refer to their documentation for steps to setup IRSA.

When deploying via Helm, the parameter `controller.serviceAccount.annotations` can be used to add the necessary annotation for IRSA, for example with the following values:
```yaml
controller:
  serviceAccount:
    annotations:
      eks.amazonaws.com/role-arn: arn:aws:iam::123412341234:role/ebs-csi-role
```

#### Secret Object

When deplying via Helm, the chart can be configured to pass IAM credentials stored in a Kubernetes `Secret` to the EBS CSI Driver. This option may be useful in confunction with third party software that stores credentials in a secret. This is configured using the `awsAccessSecret` Helm parameter:
```yaml
awsAccessSecret:
  name: aws-secret # This should be the name of the secret (must be in the same namespace as the driver deployment, typically kube-system)
  keyId: key_id # This is the name of the key on the secret that holds the AWS Key ID
  accessKey: access_key # This is the name of the key on the secret that holds the AWS Secret Access Key
```

#### (Not Recommended) IAM Instance Profile

[EC2 IAM Instance Profiles](https://docs.aws.amazon.com/IAM/latest/UserGuide/id_roles_use_switch-role-ec2_instance-profiles.html) enable sharing IAM credentials with software running on EC2 instances. The policy must be attached to the instance IAM role, and the EBS CSI Driver must be able to reach IMDS in order to retrieve the credentials. In order for the driver to access IMDS, it either must be run in host networking mode, or with a [hop limit of at least 2](https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/configuring-IMDS-existing-instances.html#modify-PUT-response-hop-limit).

This method is not recommended in production environments because any pod or software running on the node with access to IMDS could assume the role and access the wide permissions of the EBS CSI Driver, violating the best practice of [restricting pod access to the instance role](https://aws.github.io/aws-eks-best-practices/security/docs/iam/#restrict-access-to-the-instance-profile-assigned-to-the-worker-node).

### Configure driver toleration settings
By default, the driver controller tolerates taint `CriticalAddonsOnly` and has `tolerationSeconds` configured as `300`; and the driver node tolerates all taints. If you don't want to deploy the driver node on all nodes, please set Helm `Value.node.tolerateAllTaints` to false before deployment. Add policies to `Value.node.tolerations` to configure customized toleration for nodes.

### Configure node startup taint
There are potential race conditions on node startup (especially when a node is first joining the cluster) where pods/processes that rely on the EBS CSI Driver can act on a node before the EBS CSI Driver is able to startup up and become fully ready. To combat this, the EBS CSI Driver contains a feature to automatically remove a taint from the node on startup. Users can taint their nodes when they join the cluster and/or on startup, to prevent other pods from running and/or being scheduled on the node prior to the EBS CSI Driver becoming ready.

This feature is activated by default, and cluster administrators should use the taint `ebs.csi.aws.com/agent-not-ready:NoExecute` (any effect will work, but `NoExecute` is recommended). For example, EKS Managed Node Groups [support automatically tainting nodes](https://docs.aws.amazon.com/eks/latest/userguide/node-taints-managed-node-groups.html).

### Deploy driver
You may deploy the EBS CSI driver via Kustomize, Helm, or as an [Amazon EKS managed add-on](https://docs.aws.amazon.com/eks/latest/userguide/managing-ebs-csi.html).

#### Kustomize
```sh
kubectl apply -k "github.com/kubernetes-sigs/aws-ebs-csi-driver/deploy/kubernetes/overlays/stable/?ref=release-1.45"
```

*Note: Using the master branch to deploy the driver is not supported as the master branch may contain upcoming features incompatible with the currently released stable version of the driver.*

#### Helm
- Add the `aws-ebs-csi-driver` Helm repository.
```sh
helm repo add aws-ebs-csi-driver https://kubernetes-sigs.github.io/aws-ebs-csi-driver
helm repo update
```

- Install the latest release of the driver.
```sh
helm upgrade --install aws-ebs-csi-driver \
    --namespace kube-system \
    aws-ebs-csi-driver/aws-ebs-csi-driver
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
