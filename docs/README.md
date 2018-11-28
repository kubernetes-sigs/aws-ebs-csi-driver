[![Build Status](https://travis-ci.org/kubernetes-sigs/aws-ebs-csi-driver.svg?branch=master)](https://travis-ci.org/kubernetes-sigs/aws-ebs-csi-driver)
[![Coverage Status](https://coveralls.io/repos/github/kubernetes-sigs/aws-ebs-csi-driver/badge.svg?branch=master)](https://coveralls.io/github/kubernetes-sigs/aws-ebs-csi-driver?branch=master)
[![Go Report Card](https://goreportcard.com/badge/github.com/kubernetes-sigs/aws-ebs-csi-driver)](https://goreportcard.com/report/github.com/kubernetes-sigs/aws-ebs-csi-driver)
[![FOSSA Status](https://app.fossa.io/api/projects/git%2Bgithub.com%2Fd-nishi%2Faws-ebs-csi-driver.svg?type=shield)](https://app.fossa.io/projects/git%2Bgithub.com%2Fd-nishi%2Faws-ebs-csi-driver?ref=badge_shield)

**WARNING**: This driver is in ALPHA currently. This means that there may be potentially backwards compatibility breaking changes moving forward. Do NOT use this driver in a production environment in its current state.

**WARNING**: The ALPHA driver is NOT compatible with Kubernetes versions <1.12.

**DISCLAIMER**: This is not an officially supported Amazon product

# Amazon Elastic Block Store CSI driver

## Overview

The [Amazon Elastic Block Store](https://aws.amazon.com/ebs/) CSI Driver provides a [CSI](https://github.com/container-storage-interface/spec/blob/master/spec.md) interface used by Container Orchestrators to manage the lifecycle of EBS volumes.

This driver is in alpha stage and basic volume operations are already working including CreateVolume/DeleteVolume, ControllerPublishVolume/ControllerUnpublishVolume, NodeStageVolume/NodeUnstageVolume,  NodePublishVolume/NodeUnpublishVolume and [Volume Scheduling](https://kubernetes.io/docs/concepts/storage/storage-classes/#volume-binding-mode).

This driver is compatiable with CSI version [v0.3.0](https://github.com/container-storage-interface/spec/blob/v0.3.0/spec.md).

Stable alpha image: [amazon/aws-ebs-csi-driver:0.1.0-alpha](https://hub.docker.com/r/amazon/aws-ebs-csi-driver/)

To check our current development efforts, visit our [Milestones page](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/milestones).

## Requirements
### Kubernetes
* Kubernetes 1.12+ is required. Although this driver should work with any other container orchestration system that implements the CSI specification, so far it has only been tested in Kubernetes.

* Kube-apiserver and kubelet should run with the flag`--allow-privileged` set.

* For general CSI driver setup on kubernetes, please refer to [kubernetes CSI docs](https://kubernetes-csi.github.io/docs/Home.html).

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

### Topology
`topology.ebs.csi.aws.com/zone` is the only topology key that represents the availability zone of which a volume is accessible.

To enable topology support on kuberetes, make sure `CSINodeInfo` and `CSIDriverRegistry` feature flags are enabled on both kubelet and kube-apiserver and `CSINodeInfo` CRD is installed on the cluster following [Enabling CSINodeInfo](https://kubernetes-csi.github.io/docs/Setup.html#enabling-csinodeinfo).

And *external-provisioner* must have the togology feature gate enabled with `--feature-gates=CSINodeInfo=true`

## Installation
### Kubernetes
Under the directory [deploy/kubernetes](./deploy/kubernetes), there are a few manifest files that are needed to deploy the CSI driver along with sidecar containers. If you are using Kubernetes v1.12+, use the manifest files under [deploy/kubernetes/v1.12+](deploy/kubernetes/v1.12+); for kubernetes v1.10 and v1.11, use the files under [deploy/kubernetes/v1.[10,11]](deploy/kubernetes/v1.[10,11]).

In this example we'll use Kubernetes v1.12. First of all, edit the `deploy/kubernetes/v1.12+/secrets.yaml` file and add AWS credentials of the IAM user. It's a best practice to only grant required permission to the driver.

The file will look like this:

```
apiVersion: v1
kind: Secret
metadata:
  name: aws-secret
stringData:
  key_id: my_key_id
  access_key: my_access_key
```

Now, with one command we will create the secret and deploy the sidecar containers and the CSI driver:

```
kubectl create -f deploy/kubernetes/v1.12+
```

From now on we can start creating EBS volumes using the CSI driver. Under `deploy/kubernetes/v1.12+/sample_app` you will find a sample app deployment that uses the recently deployed driver:

```
kubectl create -f deploy/kubernetes/v1.12+/sample_app
```

## Development
Please go through [CSI Spec](https://github.com/container-storage-interface/spec/blob/master/spec.md) and [General CSI driver development guideline](https://kubernetes-csi.github.io/docs/Development.html) to get some basic understanding of CSI driver before you start.

### Requirements
* Golang 1.11.1+
* [Ginkgo](https://github.com/onsi/ginkgo) for integration and end-to-end testing
* Docker 17.05+ for releasing

### Testing

In order to make sure that the driver complies with the CSI specification, run the command:

```
make test-sanity
```

To execute all unit tests, run:

```
make test
```

To execute integration tests, run:

```
make test-integration
```

**Note**: EC2 instance is required to run integration test, since it is exercising the actual flow of creating EBS volume, attaching it and read/write on the disk.

### Build and Publish Container Image

Build and publish container image of the driver is as simple as building the image and pushing it to the container registry with the command:

```
make image && make push
```
