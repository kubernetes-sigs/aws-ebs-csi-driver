[![Build Status](https://travis-ci.org/kubernetes-sigs/aws-ebs-csi-driver.svg?branch=master)](https://travis-ci.org/kubernetes-sigs/aws-ebs-csi-driver)
[![Coverage Status](https://coveralls.io/repos/github/kubernetes-sigs/aws-ebs-csi-driver/badge.svg?branch=master)](https://coveralls.io/github/kubernetes-sigs/aws-ebs-csi-driver?branch=master)
[![Go Report Card](https://goreportcard.com/badge/github.com/kubernetes-sigs/aws-ebs-csi-driver)](https://goreportcard.com/report/github.com/kubernetes-sigs/aws-ebs-csi-driver)

**WARNING**: This driver is currently in Beta release and should not be used in performance critical applications.

**DISCLAIMER**: This is not an officially supported Amazon product

# Amazon Elastic Block Store (EBS) CSI driver

## Overview

The [Amazon Elastic Block Store](https://aws.amazon.com/ebs/) Container Storage Interface (CSI) Driver provides a [CSI](https://github.com/container-storage-interface/spec/blob/master/spec.md) interface used by Container Orchestrators to manage the lifecycle of Amazon EBS volumes.

## CSI Specification Compability Matrix
| AWS EBS CSI Driver \ CSI Version       | v0.3.0| v1.0.0 | 
|----------------------------------------|-------|--------|
| master branch                          | no    | yes    |
| v0.3.0                                 | no    | yes    |
| v0.2.0                                 | no    | yes    |
| v0.1.0                                 | yes   | no     |

## Features
The following CSI gRPC calls are implemented:
* **Controller Service**: CreateVolume, DeleteVolume, ControllerPublishVolume, ControllerUnpublishVolume, ControllerGetCapabilities, ValidateVolumeCapabilities, CreateSnapshot, DeleteSnapshot
* **Node Service**: NodeStageVolume, NodeUnstageVolume, NodePublishVolume, NodeUnpublishVolume, NodeGetCapabilities, NodeGetInfo
* **Identity Service**: GetPluginInfo, GetPluginCapabilities, Probe

### CreateVolume Parameters
There are several optional parameters that could be passed into `CreateVolumeRequest.parameters` map:

| Parameters        | Values           | Default  | Description         |
|-------------------|------------------|----------|---------------------|
| "type"            |io1, gp2, sc1, st1| gp2      | EBS volume type     |
| "iopsPerGB"       |                  |          | I/O operations per second per GiB. Required when io1 volume type is specified |
| "fsType"          | ext2, ext3, ext4 | ext4     | File system type that will be formatted during volume creation |
| "encrypted"       |                  |          | Whether the volume should be encrypted or not. Valid values are "true" or "false" | 
| "kmsKeyId"        |                  |          | The full ARN of the key to use when encrypting the volume. When not specified, the default KMS key is used |

# EBS CSI Driver on Kubernetes
Following sections are Kubernetes specific. If you are Kubernetes user, use followings for driver features, installation steps and examples.

## Kubernetes Version Compability Matrix
| AWS EBS CSI Driver \ Kubernetes Version| v1.12 | v1.13 |
|----------------------------------------|-------|-------|
| master branch                          | no    | yes   |
| v0.3.0                                 | no    | yes   |
| v0.2.0                                 | no    | yes   |
| v0.1.0                                 | yes   | yes   |

## Container Images:
|AWS EBS CSI Driver Version | Image                               |
|---------------------------|-------------------------------------|
|master branch              |amazon/aws-ebs-csi-driver:latest     |
|v0.3.0                     |amazon/aws-ebs-csi-driver:0.3.0      |
|v0.2.0                     |amazon/aws-ebs-csi-driver:0.2.0      |
|v0.1.0                     |amazon/aws-ebs-csi-driver:0.1.0-alpha|

## Features
* **Static Provisioning** - create a new or migrating existing EBS volumes, then create persistence volume (PV) from the EBS volume and consume the PV from container using persistence volume claim (PVC).
* **Dynamic Provisioning** - uses persistence volume claim (PVC) to request the Kuberenetes to create the EBS volume on behalf of user and consumes the volume from inside container.
* **Mount Option** - mount options could be specified in persistence volume (PV) to define how the volume should be mounted.
* **Block Volume** - consumes the EBS volume as a raw block device for latency sensitive application eg. MySql
* **Volume Snapshot** - creating volume snapshots and restore volume from snapshot.
* **NVMe** - consume NVMe EBS volume from EC2 [Nitro instance](https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/instance-types.html#ec2-nitro-instances).

## Prerequisites
* Get yourself familiar with how to setup Kubernetes on AWS
* If you are managing EBS volumes using static provisioning, get yourself familiar with [EBS volume](https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/AmazonEBS.html).
* Have a working Kubernetes cluster:
  * Enable flag `--allow-privileged=true` for `kubelet` and `kube-apiserver`
  * Enable `kube-apiserver` feature gates `--feature-gates=CSINodeInfo=true,CSIDriverRegistry=true,CSIBlockVolume=true,VolumeSnapshotDataSource=true`
  * Enable `kubelet` feature gates `--feature-gates=CSINodeInfo=true,CSIDriverRegistry=true,CSIBlockVolume=true`

## Installation
Checkout the project:
```sh
git clone https://github.com/kubernetes-sigs/aws-ebs-csi-driver.git
cd aws-ebs-csi-driver
```

Edit the [secret manifest](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/blob/master/deploy/kubernetes/secret.yaml) using your favorite text editor. The secret should have [enough IAM permission](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/blob/master/docs/example-iam-policy.json) to create EBS volume. Then deploy the secret:
```sh
kubectl apply -f deploy/kubernetes/secret.yaml
```

If your cluster is v1.14+, you can skip this step. Install the `CSINodeInfo` CRD on the cluster:
```sh
kubectl create -f https://raw.githubusercontent.com/kubernetes/csi-api/release-1.13/pkg/crd/manifests/csinodeinfo.yaml
```

Then deploy the driver:
```sh
kubectl apply -f deploy/kubernetes/manifest.yaml
```

Verify driver is running:
```sh
kubectl get pods -n kube-system
```

## Examples
Make sure you follow the [Prerequisites](README.md#Prerequisites) before the examples:
* [Dynamic Provisioning](../examples/kubernetes/dynamic-provisioning)
* [Block Volume](../examples/kubernetes/block-volume)
* [Volume Snapshot](../examples/kubernetes/snapshot)

## Migrating from in-tree EBS plugin
Starting from Kubernetes 1.14, CSI migration is supported as alpha feature. If you have persistence volumes that are created with in-tree `kubernetes.io/aws-ebs` plugin, you could migrate to use EBS CSI driver. To turn on the migration, set `CSIMigration` and `CSIMigrationAWS` feature gates to `true` for `kube-controller-manager` and `kubelet`.

## Development
Please go through [CSI Spec](https://github.com/container-storage-interface/spec/blob/master/spec.md) and [General CSI driver development guideline](https://kubernetes-csi.github.io/docs/Development.html) to get some basic understanding of CSI driver before you start.

### Requirements
* Golang 1.11.4+
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
* EC2 instance is required to run integration test, since it is exercising the actual flow of creating EBS volume, attaching it and read/write on the disk. See [Ingetration Testing](../tests/integration/README.md) for more details.
* E22 tests exercises various driver functionalities in Kubernetes cluster. See [E2E Testing](../tests/e2e/README.md) for more details.

### Build and Publish Container Image
* Build image and push it with latest tag: `make image && make push`
* Build image and push it with release tag: `make image-release && make push-release`

## Milestone
[Milestones page](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/milestones)
