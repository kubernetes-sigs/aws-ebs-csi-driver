# Installation

## Prerequisites
* If you are managing EBS volumes using static provisioning, get yourself familiar with [EBS volume](https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/AmazonEBS.html).
* Get yourself familiar with how to setup Kubernetes on AWS and have a working Kubernetes cluster:
  * Enable flag `--allow-privileged=true` for `kubelet` and `kube-apiserver`
  * Enable `kube-apiserver` feature gates `--feature-gates=CSINodeInfo=true,CSIDriverRegistry=true,CSIBlockVolume=true,VolumeSnapshotDataSource=true`
  * Enable `kubelet` feature gates `--feature-gates=CSINodeInfo=true,CSIDriverRegistry=true,CSIBlockVolume=true`
* If you intend to use the csi-snapshotter functionality you will need to first install the [CSI Snapshotter](https://github.com/kubernetes-csi/external-snapshotter).

## Installation
#### Set up driver permission
The driver requires IAM permission to talk to Amazon EBS to manage the volume on user's behalf. [The example policy here](./example-iam-policy.json) defines these permissions. There are several methods to grant the driver IAM permission:
* Using IAM [instance profile](https://docs.aws.amazon.com/IAM/latest/UserGuide/id_roles_use_switch-role-ec2_instance-profiles.html) - attach the policy to the instance profile IAM role and turn on access to [instance metadata](https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/ec2-instance-metadata.html) for the instance(s) on which the driver Deployment will run
* EKS only: Using [IAM roles for ServiceAccounts](https://docs.aws.amazon.com/eks/latest/userguide/iam-roles-for-service-accounts.html) - create an IAM role, attach the policy to it, then follow the IRSA documentation to associate the IAM role with the driver Deployment service account, which if you are installing via Helm is determined by value `controller.serviceAccount.name`, `ebs-csi-controller-sa` by default
* Using secret object - create an IAM user, attach the policy to it, put that user's credentials in [secret manifest](../deploy/kubernetes/secret.yaml), then deploy the secret
```sh
curl https://raw.githubusercontent.com/kubernetes-sigs/aws-ebs-csi-driver/master/deploy/kubernetes/secret.yaml > secret.yaml
# Edit the secret with user credentials
kubectl apply -f secret.yaml
```

#### Config node toleration settings
By default, driver tolerates taint `CriticalAddonsOnly` and has `tolerationSeconds` configured as `300`, to deploy the driver on all nodes, please set Helm `Value.node.tolerateAllTaints` to true before deployment

#### Deploy driver
Please see the compatibility matrix before you deploy the driver

To deploy the CSI driver:
```sh
kubectl apply -k "github.com/kubernetes-sigs/aws-ebs-csi-driver/deploy/kubernetes/overlays/stable/?ref=release-1.6"
```

Verify driver is running:
```sh
kubectl get pods -n kube-system
```

Alternatively, you could also install the driver using Helm:

Add the aws-ebs-csi-driver Helm repository:
```sh
helm repo add aws-ebs-csi-driver https://kubernetes-sigs.github.io/aws-ebs-csi-driver
helm repo update
```

Then install a release of the driver using the chart
```sh
helm upgrade --install aws-ebs-csi-driver \
    --namespace kube-system \
    aws-ebs-csi-driver/aws-ebs-csi-driver
```

#### Upgrading from version 1.X to 2.X of the Helm chart
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

To make sure that the CSI driver has permission to Attach, Detach, and Delete volumes that were dynamically provisioned and tagged by the in-tree plugin prior to migration being turned on, the IAM policy has to grant permission to operate on volumes with tag `kubernetes.io/cluster/<ID of the Kubernetes cluster>": "owned"` like in [the example policy](./example-iam-policy.json#L85).

**Warning**:
* kubelet *must* be drained of all pods with mounted EBS volumes ***before*** changing its CSI migration feature flags.  Failure to do this will cause deleted pods to get stuck in `Terminating`, requiring a forced delete which can cause filesystem corruption. See [#679](../../../issues/679) for more details.