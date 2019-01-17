[![Build Status](https://travis-ci.org/kubernetes-sigs/aws-ebs-csi-driver.svg?branch=master)](https://travis-ci.org/kubernetes-sigs/aws-ebs-csi-driver)
[![Coverage Status](https://coveralls.io/repos/github/kubernetes-sigs/aws-ebs-csi-driver/badge.svg?branch=master)](https://coveralls.io/github/kubernetes-sigs/aws-ebs-csi-driver?branch=master)
[![Go Report Card](https://goreportcard.com/badge/github.com/kubernetes-sigs/aws-ebs-csi-driver)](https://goreportcard.com/report/github.com/kubernetes-sigs/aws-ebs-csi-driver)
[![FOSSA Status](https://app.fossa.io/api/projects/git%2Bgithub.com%2Fd-nishi%2Faws-ebs-csi-driver.svg?type=shield)](https://app.fossa.io/projects/git%2Bgithub.com%2Fd-nishi%2Faws-ebs-csi-driver?ref=badge_shield)

**WARNING**: This driver is in ALPHA currently. This means that there may potentially be backwards compatible breaking changes moving forward. Do NOT use this driver in a production environment in its current state.

**DISCLAIMER**: This is not an officially supported Amazon product

# Amazon Elastic Block Store CSI driver

## Overview

The [Amazon Elastic Block Store](https://aws.amazon.com/ebs/) Container Storage Interface (CSI) Driver provides a [CSI](https://github.com/container-storage-interface/spec/blob/master/spec.md) interface used by Container Orchestrators to manage the lifecycle of Amazon EBS volumes.

This driver is in alpha stage. Basic volume operations that are functional include CreateVolume/DeleteVolume, ControllerPublishVolume/ControllerUnpublishVolume, NodeStageVolume/NodeUnstageVolume, NodePublishVolume/NodeUnpublishVolume and [Volume Scheduling](https://kubernetes.io/docs/concepts/storage/storage-classes/#volume-binding-mode).

## Container Images:

|AWS EBS CSI Driver Version | Image                               |
|---------------------------|-------------------------------------|
|v0.1.0                     |amazon/aws-ebs-csi-driver:0.1.0-alpha|
|v0.2.0                     |amazon/aws-ebs-csi-driver:0.2.0      |
|master branch              |amazon/aws-ebs-csi-driver:latest     |

## CSI Specification Compability Matrix
| AWS EBS CSI Driver \ CSI Version       | v0.3.0| v1.0.0 | 
|----------------------------------------|-------|--------|
| v0.1.0                                 | yes   | no     |
| v0.2.0                                 | no    | yes    |
| master branch                          | no    | yes    |

## Kubernetes Version Compability Matrix
| AWS EBS CSI Driver \ Kubernetes Version| v1.12 | v1.13 | 
|----------------------------------------|-------|-------|
| v0.1.0                                 | yes   | yes   |
| v0.2.0                                 | no    | yes   |
| master branch                          | no    | yes   |

## Features
### Capabilities
The list of supported driver capabilities:
* Identity Service: **CONTROLLER_SERVICE** and **ACCESSIBILITY_CONSTRAINTS**
* Controller Service: **CREATE_DELETE_VOLUME** and **PUBLISH_UNPUBLISH_VOLUME**
* Node Service: **STAGE_UNSTAGE_VOLUME**

### CreateVolume Parameters
There are several optional parameters that could be passed into `CreateVolumeRequest.parameters` map:

| Parameters        | Values           | Default  | Description         |
|-------------------|------------------|----------|---------------------|
| "type"            |io1, gp2, sc1, st1| gp2      | EBS volume type     |
| "iopsPerGB"       |                  |          | I/O operations per second per GiB. Required when io1 volume type is specified |
| "fsType"          | ext2, ext3, ext4 | ext4     | File system type that will be formatted during volume creation |
| "encrypted"       |                  |          | Whether the volume should be encrypted or not. Valid values are "true" or "false" | 
| "kmsKeyId"        |                  |          | The full ARN of the key to use when encrypting the volume. When not specified, the default KMS key is used |
| "additionalTags"  |                  |          | Comma separated key=value list of tags to set on the EBS volume |

## Prerequisites
### Kubernetes
1. Kubernetes 1.12+ is required. Although this driver should work with any other container orchestration system that implements the CSI specification, so far it has only been tested in Kubernetes.

2. Enable the flag `--allow-privileged=true` in the manifest entries of kubelet and kube-apiserver.

3. Add `--feature-gates=CSINodeInfo=true,CSIDriverRegistry=true,VolumeSnapshotDataSource=true` in the manifest entries of kubelet and kube-apiserver. This is required to enable topology support of EBS volumes in Kubernetes and restoring volumes from snapshots.

4. Install the `CSINodeInfo` CRD on the cluster using the instructions provided here: [Enabling CSINodeInfo](https://kubernetes-csi.github.io/docs/csi-node-info-object.html#enabling-csinodeinfo).

5. Please refer to [kubernetes CSI docs](https://kubernetes-csi.github.io/docs/) for general CSI driver setup instructions on kubernetes.

## Setup
### Kubernetes
1. Use the manifest files under the directory [deploy/kubernetes](../deploy/kubernetes), needed to deploy the CSI driver and sidecar containers.

2. The driver can use the EC2 instance roles, otherwise add AWS credentials of the IAM user to the [deploy/kubernetes/secret.yaml](../deploy/kubernetes/secret.yaml) file.

```
apiVersion: v1
kind: Secret
metadata:
  name: aws-secret
stringData:
  key_id: [aws_access_key_id]
  access_key: [aws_secret_access_key]
```

3. Apply the secret using `kubectl apply -f deploy/kubernetes/secret.yaml` if required.

4. Grant only required permissions to the CSI driver. Use this sample [IAM policy](example-iam-policy.json) and add it to the worker nodes in the cluster.

5. Deploy the csi-provisioner, csi-attacher and csi-node manifests to the cluster in one step:

```
kubectl apply -f deploy/kubernetes
```

Now any user can start creating and using EBS volumes with the CSI driver. 

6. Apply `examples/kubernetes/volume_scheduling` that uses the recently deployed driver:

```
kubectl apply -f examples/kubernetes/volume_scheduling
```

## Development
Please go through [CSI Spec](https://github.com/container-storage-interface/spec/blob/master/spec.md) and [General CSI driver development guideline](https://kubernetes-csi.github.io/docs/Development.html) to get some basic understanding of CSI driver before you start.

### Requirements
* Golang 1.11.4+
* [Ginkgo](https://github.com/onsi/ginkgo) in your PATH for integration testing and end-to-end testing
* Docker 17.05+ for releasing

### Testing

To execute all unit tests, run: `make test`

To execute sanity test run: `make test-sanity`

**Notes**:
* Sanity tests make sure that the driver complies with the CSI specification

To execute integration tests, run: `make test-integration`

**Notes**:
* EC2 instance is required to run integration test, since it is exercising the actual flow of creating EBS volume, attaching it and read/write on the disk.
* See [Ingetration Testing](../tests/integration/README.md) for more details.

To execute e2e tests, run:

```
make test-e2e-single-az // executes single az test suite
make test-e2e-multi-az // executes multi az test suite
```

**Notes**:
* See [E2E Testing](../tests/e2e/README.md) for more details.

### Build and Publish Container Image

Build image and push it with latest tag:

```
make image && make push
```

Build image and push it with release tag:

```
make image-release && make push-release
```

## Milestone
[Milestones page](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/milestones)
