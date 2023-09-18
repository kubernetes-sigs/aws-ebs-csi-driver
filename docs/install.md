# Installation

## Prerequisites

* Kubernetes Version >= 1.20 

* If you are using a self managed cluster, ensure the flag `--allow-privileged=true` for `kube-apiserver`.

* Important: If you intend to use the Volume Snapshot feature, the [Kubernetes Volume Snapshot CRDs](https://github.com/kubernetes-csi/external-snapshotter/tree/master/client/config/crd) must be installed **before** the EBS CSI driver. For installation instructions, see [CSI Snapshotter Usage](https://github.com/kubernetes-csi/external-snapshotter#usage).

## Installation
### Set up driver permissions
The driver requires IAM permissions to talk to Amazon EBS to manage the volume on user's behalf. [The example policy here](./example-iam-policy.json) defines these permissions. AWS maintains a managed policy, available at ARN `arn:aws:iam::aws:policy/service-role/AmazonEBSCSIDriverPolicy`. 

Note: Add the below statement to the example policy if you want to encrypt the EBS drives. 
```
{
  "Effect": "Allow",
  "Action": [
      "kms:Decrypt",
      "kms:GenerateDataKeyWithoutPlaintext",
      "kms:CreateGrant"
  ],
  "Resource": "*"
}
```

For more information, review ["Creating the Amazon EBS CSI driver IAM role for service accounts" from the EKS User Guide.](https://docs.aws.amazon.com/eks/latest/userguide/csi-iam-role.html) 

There are several methods to grant the driver IAM permissions:
* Using IAM [instance profile](https://docs.aws.amazon.com/IAM/latest/UserGuide/id_roles_use_switch-role-ec2_instance-profiles.html) - attach the policy to the instance profile IAM role and turn on access to [instance metadata](https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/ec2-instance-metadata.html) for the instance(s) on which the driver Deployment will run
* EKS only: Using [IAM roles for ServiceAccounts](https://docs.aws.amazon.com/eks/latest/userguide/iam-roles-for-service-accounts.html) - create an IAM role, attach the policy to it, then follow the IRSA documentation to associate the IAM role with the driver Deployment service account, which if you are installing via Helm is determined by value `controller.serviceAccount.name`, `ebs-csi-controller-sa` by default. If you are using k8s 1.24 or higher, the ServiceAccountToken is not mounted because the `LegacyServiceAccountTokenNoAutoGeneration` feature gate is enabled.
Therefore, if you are using k8s 1.24 or higher, you need to set `true` to `controller.serviceAccount.autoMountServiceAccountToken`.
* Using secret object - create an IAM user, attach the policy to it, then create a generic secret in the `kube-system` namespace with the user's credentials. The snippet below creates the generic secret named `aws-secret` that the driver accepts by default. You can customize the default secret and key names via the Helm parameters `awsAccessSecret.name`, `awsAccessSecret.keyId`, and `awsAccessSecret.accessKey` in the chart's [values.yaml](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/blob/master/charts/aws-ebs-csi-driver/values.yaml). 
```sh
kubectl create secret generic aws-secret \
    --namespace kube-system \
    --from-literal "key_id=${AWS_ACCESS_KEY_ID}" \
    --from-literal "access_key=${AWS_SECRET_ACCESS_KEY}"
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
kubectl apply -k "github.com/kubernetes-sigs/aws-ebs-csi-driver/deploy/kubernetes/overlays/stable/?ref=release-1.23"
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
