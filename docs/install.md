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
* EKS only: Using [IAM roles for ServiceAccounts](https://docs.aws.amazon.com/eks/latest/userguide/iam-roles-for-service-accounts.html) - create an IAM role, attach the policy to it, then follow the IRSA documentation to associate the IAM role with the driver Deployment service account, which if you are installing via Helm is determined by value `controller.serviceAccount.name`, `ebs-csi-controller-sa` by default
* Using secret object - create an IAM user, attach the policy to it, then create a generic secret called `aws-secret` in the `kube-system` namespace with the user's credentials
```sh
kubectl create secret generic aws-secret \
    --namespace kube-system \
    --from-literal "key_id=${AWS_ACCESS_KEY_ID}" \
    --from-literal "access_key=${AWS_SECRET_ACCESS_KEY}"
```

### Configure driver toleration settings
By default, the driver controller tolerates taint `CriticalAddonsOnly` and has `tolerationSeconds` configured as `300`; and the driver node tolerates all taints. If you don't want to deploy the driver node on all nodes, please set Helm `Value.node.tolerateAllTaints` to false before deployment. Add policies to `Value.node.tolerations` to configure customized toleration for nodes.

### Configure node taint and driver start-up taint
In some cases when new node frequesntly join the cluster, workload pods can be scheduled to a new node ahead of the csi-node start-up and ready on that node. This race condition between workload pod and csi-node pod will cause the workload pod fails to mount PVC at first place.

To help overcome this situation, CSI Driver node can manipulate Kubernetes’s taints on a given node to help preventing pods from starting before CSI Driver's node pod runs on this node.

To configure start-up taint, the cluster administrator can places a taint with key `node.ebs.csi.aws.com/agent-not-ready` on a given uninitialized node(or node group). This prevents pods that don’t have a matching toleration from either being scheduled or altogether running on the node until the taint is removed. If use Helm to install CSI Driver, set `.Value.node.startUpTaint` to `true`. Once the CSI Driver pod runs on the node, initializes and ready, it will removes the aforementioned taint. After that, workload pods will start being scheduled and running on the node, with CSI Driver full functional on that node.

The taint effect can be set to `NoSchedule` or `NoExecute`,

* If NoSchedule is used, pods won’t be scheduled to a node until CSI Driver remove the taint. However, one practical effect of this is that if some external process (such as a reboot) resets the configuration on the node, pods that were already scheduled will be allowed to start concurrently with csi-node when the node next reboots
* If NoExecute is used (recommended), pods won’t be executed (nor scheduled) on a node until CSI Driver removes the taint. One practical effect of this is that whenever the taint is added back to the node by some external process (such as during an cluster upgrade), pods will be evicted from the node until CSI Driver remove the taint again after startup.


### Deploy driver
You may deploy the EBS CSI driver via Kustomize, Helm, or as an [Amazon EKS managed add-on](https://docs.aws.amazon.com/eks/latest/userguide/managing-ebs-csi.html).

#### Kustomize
```sh
kubectl apply -k "github.com/kubernetes-sigs/aws-ebs-csi-driver/deploy/kubernetes/overlays/stable/?ref=release-1.18"
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
