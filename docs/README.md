[![Build Status](https://travis-ci.org/kubernetes-sigs/aws-ebs-csi-driver.svg?branch=master)](https://travis-ci.org/kubernetes-sigs/aws-ebs-csi-driver)
[![Coverage Status](https://coveralls.io/repos/github/kubernetes-sigs/aws-ebs-csi-driver/badge.svg?branch=master)](https://coveralls.io/github/kubernetes-sigs/aws-ebs-csi-driver?branch=master)
[![Go Report Card](https://goreportcard.com/badge/github.com/kubernetes-sigs/aws-ebs-csi-driver)](https://goreportcard.com/report/github.com/kubernetes-sigs/aws-ebs-csi-driver)

# Amazon Elastic Block Store (EBS) CSI driver

## Overview

The [Amazon Elastic Block Store](https://aws.amazon.com/ebs/) Container Storage Interface (CSI) Driver provides a [CSI](https://github.com/container-storage-interface/spec/blob/master/spec.md) interface used by Container Orchestrators to manage the lifecycle of Amazon EBS volumes.

## CSI Specification Compatibility Matrix
| AWS EBS CSI Driver \ CSI Version       | v0.3.0| v1.0.0 | v1.1.0 |
|----------------------------------------|-------|--------|--------|
| master branch                          | no    | no     | yes    |
| v0.9.x                                 | no    | no     | yes    |
| v0.8.x                                 | no    | no     | yes    |
| v0.7.1                                 | no    | no     | yes    |
| v0.6.0                                 | no    | no     | yes    |
| v0.5.0                                 | no    | no     | yes    |
| v0.4.0                                 | no    | no     | yes    |
| v0.3.0                                 | no    | yes    | no     |
| v0.2.0                                 | no    | yes    | no     |
| v0.1.0                                 | yes   | no     | no     |

## Features
The following CSI gRPC calls are implemented:
* **Controller Service**: CreateVolume, DeleteVolume, ControllerPublishVolume, ControllerUnpublishVolume, ControllerGetCapabilities, ValidateVolumeCapabilities, CreateSnapshot, DeleteSnapshot, ListSnapshots
* **Node Service**: NodeStageVolume, NodeUnstageVolume, NodePublishVolume, NodeUnpublishVolume, NodeGetCapabilities, NodeGetInfo
* **Identity Service**: GetPluginInfo, GetPluginCapabilities, Probe

### CreateVolume Parameters
There are several optional parameters that could be passed into `CreateVolumeRequest.parameters` map:

| Parameters                  | Values                                 | Default  | Description         |
|-----------------------------|----------------------------------------|----------|---------------------|
| "csi.storage.k8s.io/fsType" | xfs, ext2, ext3, ext4                  | ext4     | File system type that will be formatted during volume creation |
| "type"                      | io1, io2, gp2, gp3, sc1, st1,standard  | gp3*     | EBS volume type     |
| "iopsPerGB"                 |                                        |          | I/O operations per second per GiB. Required when io1 or io2 volume type is specified. If this value multiplied by the size of a requested volume produces a value below the minimum or above the maximum IOPs allowed for the volume type, as documented [here](https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/ebs-volume-types.html), AWS will return an error and volume creation will fail |
| "iops"                      |                                        | 3000     | I/O operations per second. Only effetive when gp3 volume type is specified. If empty, it will set to 3000 as documented [here](https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/ebs-volume-types.html). |
| "throughput"                |                                        | 125      | Throughput in MiB/s. Only effective when gp3 volume type is specified. If empty, it will set to 125MiB/s as documented [here](https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/ebs-volume-types.html). |
| "encrypted"                 |                                        |          | Whether the volume should be encrypted or not. Valid values are "true" or "false" |
| "kmsKeyId"                  |                                        |          | The full ARN of the key to use when encrypting the volume. When not specified, the default KMS key is used |

**Notes**:
* `gp3` is currently not supported on outposts. Outpost customers need to use a different type for their volumes.
* The parameters are case insensitive.

# EBS CSI Driver on Kubernetes
Following sections are Kubernetes specific. If you are Kubernetes user, use followings for driver features, installation steps and examples.

## Kubernetes Version Compatibility Matrix
| AWS EBS CSI Driver \ Kubernetes Version| v1.12 | v1.13 | v1.14 | v1.15 | v1.16 | v1.17 | v1.18+ |
|----------------------------------------|-------|-------|-------|-------|-------|-------|-------|
| master branch                          | no    | no+   | no    | no    | no    | yes   | yes   |
| v0.9.x                                 | no    | no+   | no    | no    | no    | yes   | yes   |
| v0.8.x                                 | no    | no+   | yes   | yes   | yes   | yes   | yes   |
| v0.7.1                                 | no    | no+   | yes   | yes   | yes   | yes   | yes   |
| v0.6.0                                 | no    | no+   | yes   | yes   | yes   | yes   | yes   |
| v0.5.0                                 | no    | no+   | yes   | yes   | yes   | yes   | yes   |
| v0.4.0                                 | no    | no+   | yes   | yes   | no    | no    | no    |
| v0.3.0                                 | no    | no+   | yes   | no    | no    | no    | no    |
| v0.2.0                                 | no    | yes   | yes   | no    | no    | no    | no    |
| v0.1.0                                 | yes   | yes   | yes   | no    | no    | no    | no    |

**Note**: for the entry with `+` sign, it means the driver's default released manifest doesn't work with corresponding Kubernetes version, but the driver container image is compatiable with the Kubernetes version if an older version's manifest is used.

## Container Images:
|AWS EBS CSI Driver Version | Image                                            |
|---------------------------|--------------------------------------------------|
|master branch              |amazon/aws-ebs-csi-driver:latest                  |
|v0.9.1                     |k8s.gcr.io/provider-aws/aws-ebs-csi-driver:v0.9.1 |
|v0.9.0                     |k8s.gcr.io/provider-aws/aws-ebs-csi-driver:v0.9.0 |
|v0.8.1                     |k8s.gcr.io/provider-aws/aws-ebs-csi-driver:v0.8.1 |
|v0.7.1                     |amazon/aws-ebs-csi-driver:v0.7.1                  |
|v0.6.0                     |amazon/aws-ebs-csi-driver:v0.6.0                  |
|v0.5.0                     |amazon/aws-ebs-csi-driver:v0.5.0                  |
|v0.4.0                     |amazon/aws-ebs-csi-driver:v0.4.0                  |
|v0.3.0                     |amazon/aws-ebs-csi-driver:v0.3.0                  |
|v0.2.0                     |amazon/aws-ebs-csi-driver:0.2.0                   |
|v0.1.0                     |amazon/aws-ebs-csi-driver:0.1.0-alpha             |

## Features
* **Static Provisioning** - create a new or migrating existing EBS volumes, then create persistence volume (PV) from the EBS volume and consume the PV from container using persistence volume claim (PVC).
* **Dynamic Provisioning** - uses persistence volume claim (PVC) to request the Kuberenetes to create the EBS volume on behalf of user and consumes the volume from inside container. Storage class's **allowedTopologies** could be used to restrict which AZ the volume should be provisioned in. The topology key should be **topology.ebs.csi.aws.com/zone**.
* **Mount Option** - mount options could be specified in persistence volume (PV) to define how the volume should be mounted.
* **NVMe** - consume NVMe EBS volume from EC2 [Nitro instance](https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/instance-types.html#ec2-nitro-instances).
* **[Block Volume](https://kubernetes-csi.github.io/docs/raw-block.html)** - consumes the EBS volume as a raw block device for latency sensitive application eg. MySql. The corresponding CSI feature (`CSIBlockVolume`) is GA since Kubernetes 1.18.
* **[Volume Snapshot](https://kubernetes-csi.github.io/docs/snapshot-restore-feature.html)** - creating volume snapshots and restore volume from snapshot. The corresponding CSI feature (`VolumeSnapshotDataSource`) is beta since Kubernetes 1.17.
* **[Volume Resizing](https://kubernetes-csi.github.io/docs/volume-expansion.html)** - expand the volume size. The corresponding CSI feature (`ExpandCSIVolumes`) is beta since Kubernetes 1.16.

## Prerequisites
* If you are managing EBS volumes using static provisioning, get yourself familiar with [EBS volume](https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/AmazonEBS.html).
* Get yourself familiar with how to setup Kubernetes on AWS and have a working Kubernetes cluster:
  * Enable flag `--allow-privileged=true` for `kubelet` and `kube-apiserver`
  * Enable `kube-apiserver` feature gates `--feature-gates=CSINodeInfo=true,CSIDriverRegistry=true,CSIBlockVolume=true,VolumeSnapshotDataSource=true`
  * Enable `kubelet` feature gates `--feature-gates=CSINodeInfo=true,CSIDriverRegistry=true,CSIBlockVolume=true`

## Installation
#### Set up driver permission
The driver requires IAM permission to talk to Amazon EBS to manage the volume on user's behalf. There are several methods to grant driver IAM permission:
* Using secret object - create an IAM user with proper permission, put that user's credentials in [secret manifest](../deploy/kubernetes/secret.yaml) then deploy the secret.
```sh
curl https://raw.githubusercontent.com/kubernetes-sigs/aws-ebs-csi-driver/master/deploy/kubernetes/secret.yaml > secret.yaml
# Edit the secret with user credentials
kubectl apply -f secret.yaml
```
* Using IAM [instance profile](https://docs.aws.amazon.com/IAM/latest/UserGuide/id_roles_use_switch-role-ec2_instance-profiles.html) - grant all the worker nodes with [proper permission](./example-iam-policy.json) by attaching policy to the instance profile of the worker.

#### Deploy CRD (optional)
If your cluster is v1.14+, you can skip this step. Install the `CSINodeInfo` CRD on the cluster:
```sh
kubectl create -f https://raw.githubusercontent.com/kubernetes/csi-api/release-1.13/pkg/crd/manifests/csinodeinfo.yaml
```

#### Config node toleration settings
By default, driver tolerates taint `CriticalAddonsOnly` and has `tolerationSeconds` configured as `300`, to deploy the driver on any nodes, please set helm `Value.node.tolerateAllTaints` and `Value.tolerateAllTaints` to true before deployment

#### Deploy driver
Please see the compatibility matrix above before you deploy the driver

If you want to deploy the stable driver without alpha features:
```sh
kubectl apply -k "github.com/kubernetes-sigs/aws-ebs-csi-driver/deploy/kubernetes/overlays/stable/?ref=release-0.8"
```

If you want to deploy the driver with alpha features:
```sh
kubectl apply -k "github.com/kubernetes-sigs/aws-ebs-csi-driver/deploy/kubernetes/overlays/alpha/?ref=master"
```

Verify driver is running:
```sh
kubectl get pods -n kube-system
```

Alternatively, you could also install the driver using helm:

Add the aws-ebs-csi-driver Helm repository:
```sh
helm repo add aws-ebs-csi-driver https://kubernetes-sigs.github.io/aws-ebs-csi-driver
helm repo update
```

Then install a release of the driver using the chart
```sh
helm upgrade --install aws-ebs-csi-driver \
    --namespace kube-system \
    --set enableVolumeScheduling=true \
    --set enableVolumeResizing=true \
    --set enableVolumeSnapshot=true \
    aws-ebs-csi-driver/aws-ebs-csi-driver
```
## Examples
Make sure you follow the [Prerequisites](README.md#Prerequisites) before the examples:
* [Dynamic Provisioning](../examples/kubernetes/dynamic-provisioning)
* [Block Volume](../examples/kubernetes/block-volume)
* [Volume Snapshot](../examples/kubernetes/snapshot)
* [Configure StorageClass](../examples/kubernetes/storageclass)
* [Volume Resizing](../examples/kubernetes/resizing)

## Migrating from in-tree EBS plugin
Starting from Kubernetes 1.17, CSI migration is supported as beta feature (alpha since 1.14). If you have persistence volumes that are created with in-tree `kubernetes.io/aws-ebs` plugin, you could migrate to use EBS CSI driver. To turn on the migration, set `CSIMigration` and `CSIMigrationAWS` feature gates to `true` for `kube-controller-manager` and `kubelet`.

To make sure dynamically provisioned EBS volumes have all tags that the in-tree volume plugin used:
* Run the external-provisioner sidecar with `--extra-create-metadata=true` cmdline option. External-provisioner v1.6 or newer is required.
* Run the CSI driver with `--k8s-tag-cluster-id=<ID of the Kubernetes cluster>` command line option.


## Development
Please go through [CSI Spec](https://github.com/container-storage-interface/spec/blob/master/spec.md) and [General CSI driver development guideline](https://kubernetes-csi.github.io/docs/developing.html) to get some basic understanding of CSI driver before you start.

### Requirements
* Golang 1.15.+
* [Ginkgo](https://github.com/onsi/ginkgo) in your PATH for integration testing and end-to-end testing
* Docker 17.05+ for releasing

### Dependency
Dependencies are managed through go module. To build the project, first turn on go mod using `export GO111MODULE=on`, then build the project using: `make`

### Testing
* To execute all unit tests, run: `make test`
* To execute sanity test run: `make test-sanity`
* To execute integration tests, run: `make test-integration`
* To execute e2e tests, run: `make test-e2e-single-az` and `make test-e2e-multi-az`

**Notes**:
* Sanity tests make sure the driver complies with the CSI specification
* EC2 instance is required to run integration test, since it is exercising the actual flow of creating EBS volume, attaching it and read/write on the disk. See [Integration Testing](../tests/integration/README.md) for more details.
* E2E tests exercises various driver functionalities in Kubernetes cluster. See [E2E Testing](../tests/e2e/README.md) for more details.

### Build and Publish Container Image
* Build image and push it with latest tag: `make image && make push`
* Build image and push it with release tag: `make image-release && make push-release`

### Helm and manifests
The helm chart for this project is in the `charts/aws-ebs-csi-driver` directory.  The manifests for this project are in the `deploy/kubernetes` directory.  All of the manifests except kustomize patches are generated by running `helm template`.  This keeps the helm chart and the manifests in sync.

When updating the helm chart:
* Generate manifests: `make generate-kustomize`
* There are values files in `deploy/kubernetes/values` used for generating some of the manifests
* When adding a new resource template to the helm chart please update the `generate-kustomize` make target, the `deploy/kubernetes/values` files, and the appropriate kustomization.yaml file(s).

## Milestone
[Milestones page](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/milestones)
