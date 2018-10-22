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

This driver has been under heavy development, however, basic volume operations like creation, deletion, attaching and detaching are already working.

To check our current development efforts, visit our [Milestones page](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/milestones).

## Requirements
### Kubernetes
* Kubernetes 1.12+ is required. Although this driver should work with any other container orchestration system that implements the CSI specification, so far it has only been tested in Kubernetes.

* API server and Kubelet should run with the flag`--allow-privileged` set.

## Features
### Capabilities
* Identity Service
  - CONTROLLER_SERVICE
  - ACCESSIBILITY_CONSTRAINTS
* Controller Service
  - CREATE_DELETE_VOLUME
  - PUBLISH_UNPUBLISH_VOLUME
* Node Service
  - STAGE_UNSTAGE_VOLUME

### CreateVolume Parameters
| Parameters      | Values           | Default  | Description         |
|-----------------|------------------|----------|---------------------|
| type            |io1, gp2, sc1, st1| gp2      | EBS volume type     |
| iopsPerGB       |                  |          | Only for io1. I/O operations per second per GiB. |
| fsType          | ext2, ext3, ext4 | ext4     | File system type that will be formatted during volume createion |

### Topology
Topology key is `com.amazon.aws.csi.ebs/zone` that represents the search key of availability zone of which a volume is accessible.

## Installation
### Kubernetes
User the directory `deploy/kubernetes` you will find a few manifest files that can be used to deploy the CSI driver. If you are using Kubernetes v1.12 onwards, use the manifest files under `deploy/kubernetes/v1.12+`; for Kubernetes v1.10 and v1.11, use the files under `deploy/kubernetes/v1.[10,11]`.

In this example we'll use Kubernetes v1.12. First of all, edit the `deploy/kubernetes/v1.12+/secrets.yaml` file and add your AWS credentials. The file will look like this:

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
$ kubectl create -f deploy/kubernetes/v.12+
```

From now on we can start creating EBS volumes using the CSI driver. Under `deploy/kubernetes/v1.12+/sample_app` you will find a sample app deployment that uses the recently deployed driver:

```
$ kubectl create -f deploy/kubernetes/v.12+/sample_app
```

## Development

### Requirements

* golang 1.11+
* [ginkgo](https://github.com/onsi/ginkgo) for end-to-end testing
* docker 17.05+ for releasing

### Testing

In order to make sure that the driver complies with the CSI specification, run the command:

```
make test-sanity
```

To execute all unit tests, run:

```
make test
```

End-to-end tests are run through the command:

```
make test-e2e
```

### Releasing

Releasing a new driver version is as simple as building the image with the command:

```
make image
```

And then pushing it to the container registry:

```
make push
```
