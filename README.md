# Amazon Elastic Block Store (EBS) CSI driver
[![GitHub release (latest SemVer)](https://img.shields.io/github/v/release/kubernetes-sigs/aws-ebs-csi-driver)](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/releases)
[![Coverage Status](https://coveralls.io/repos/github/kubernetes-sigs/aws-ebs-csi-driver/badge.svg?branch=master)](https://coveralls.io/github/kubernetes-sigs/aws-ebs-csi-driver?branch=master)
[![Go Report Card](https://goreportcard.com/badge/github.com/kubernetes-sigs/aws-ebs-csi-driver)](https://goreportcard.com/report/github.com/kubernetes-sigs/aws-ebs-csi-driver)

## Overview

The [Amazon Elastic Block Store](https://aws.amazon.com/ebs/) Container Storage Interface (CSI) Driver provides a [CSI](https://github.com/container-storage-interface/spec/blob/master/spec.md) interface used by Container Orchestrators to manage the lifecycle of Amazon EBS volumes.

## Features
* **Static Provisioning** - Associate an externally-created EBS volume with a [PersistentVolume](https://kubernetes.io/docs/concepts/storage/persistent-volumes/) (PV) for consumption within Kubernetes.
* **Dynamic Provisioning** - Automatically create EBS volumes and associated [PersistentVolumes](https://kubernetes.io/docs/concepts/storage/persistent-volumes/) (PV) from [PersistentVolumeClaims](https://kubernetes.io/docs/concepts/storage/persistent-volumes/#PersistentVolumeClaim:~:text=PersistentVolumeClaim%20(PVC)) (PVC). Parameters can be passed via a [StorageClass](https://kubernetes.io/docs/concepts/storage/storage-classes/#the-storageclass-resource) for fine-grained control over volume creation.
* **Mount Options** - Mount options could be specified in the [PersistentVolume](https://kubernetes.io/docs/concepts/storage/persistent-volumes/) (PV) resource to define how the volume should be mounted.
* **NVMe Volumes** - Consume [NVMe](https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/nvme-ebs-volumes.html) volumes from EC2 [Nitro instances](https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/instance-types.html#ec2-nitro-instances).
* **Block Volumes** - Consume an EBS volume as a [raw block device](https://kubernetes-csi.github.io/docs/raw-block.html).
* **Volume Snapshots** - Create and restore [snapshots](https://kubernetes.io/docs/concepts/storage/volume-snapshots/) taken from a volume in Kubernetes.
* **Volume Resizing** - Expand the volume size by specifying a new size in the [PersistentVolumeClaim](https://kubernetes.io/docs/concepts/storage/persistent-volumes/#PersistentVolumeClaim:~:text=PersistentVolumeClaim%20(PVC)) (PVC).

## Container Images:

|Driver Version | [GCR](https://us.gcr.io/k8s-artifacts-prod/provider-aws/aws-ebs-csi-driver ) Image | [ECR](https://gallery.ecr.aws/ebs-csi-driver/aws-ebs-csi-driver) Image |
|---------------------------|--------------------------------------------------|-----------------------------------------------------------------------------|
|v1.11.4                    |k8s.gcr.io/provider-aws/aws-ebs-csi-driver:v1.11.4| public.ecr.aws/ebs-csi-driver/aws-ebs-csi-driver:v1.11.4                    |

<details>
<summary>Previous Images</summary>

|Driver Version | [GCR](https://us.gcr.io/k8s-artifacts-prod/provider-aws/aws-ebs-csi-driver ) Image | [ECR](https://gallery.ecr.aws/ebs-csi-driver/aws-ebs-csi-driver) Image |
|---------------------------|--------------------------------------------------|-----------------------------------------------------------------------------|
|v1.11.3                    |k8s.gcr.io/provider-aws/aws-ebs-csi-driver:v1.11.3| public.ecr.aws/ebs-csi-driver/aws-ebs-csi-driver:v1.11.3                    |
|v1.11.2                    |k8s.gcr.io/provider-aws/aws-ebs-csi-driver:v1.11.2| public.ecr.aws/ebs-csi-driver/aws-ebs-csi-driver:v1.11.2                    |
|v1.10.0                    |k8s.gcr.io/provider-aws/aws-ebs-csi-driver:v1.10.0| public.ecr.aws/ebs-csi-driver/aws-ebs-csi-driver:v1.10.0                    |
|v1.9.0                     |k8s.gcr.io/provider-aws/aws-ebs-csi-driver:v1.9.0 | public.ecr.aws/ebs-csi-driver/aws-ebs-csi-driver:v1.9.0                     |
|v1.8.0                     |k8s.gcr.io/provider-aws/aws-ebs-csi-driver:v1.8.0 | public.ecr.aws/ebs-csi-driver/aws-ebs-csi-driver:v1.8.0                     |
|v1.7.0                     |k8s.gcr.io/provider-aws/aws-ebs-csi-driver:v1.7.0 | public.ecr.aws/ebs-csi-driver/aws-ebs-csi-driver:v1.7.0                     |
|v1.6.2                     |k8s.gcr.io/provider-aws/aws-ebs-csi-driver:v1.6.2 | public.ecr.aws/ebs-csi-driver/aws-ebs-csi-driver:v1.6.2                     |
|v1.6.1                     |k8s.gcr.io/provider-aws/aws-ebs-csi-driver:v1.6.1 | public.ecr.aws/ebs-csi-driver/aws-ebs-csi-driver:v1.6.1                     |
|v1.6.0                     |k8s.gcr.io/provider-aws/aws-ebs-csi-driver:v1.6.0 | public.ecr.aws/ebs-csi-driver/aws-ebs-csi-driver:v1.6.0                     |
|v1.5.3                     |k8s.gcr.io/provider-aws/aws-ebs-csi-driver:v1.5.3 | public.ecr.aws/ebs-csi-driver/aws-ebs-csi-driver:v1.5.3                     |
|v1.5.2                     |                                                  | public.ecr.aws/ebs-csi-driver/aws-ebs-csi-driver:v1.5.2                     |
|v1.5.1                     |k8s.gcr.io/provider-aws/aws-ebs-csi-driver:v1.5.1 | public.ecr.aws/ebs-csi-driver/aws-ebs-csi-driver:v1.5.1                     |
|v1.5.0                     |k8s.gcr.io/provider-aws/aws-ebs-csi-driver:v1.5.0 | public.ecr.aws/ebs-csi-driver/aws-ebs-csi-driver:v1.5.0                     |
|v1.4.0                     |k8s.gcr.io/provider-aws/aws-ebs-csi-driver:v1.4.0 | public.ecr.aws/ebs-csi-driver/aws-ebs-csi-driver:v1.4.0                     |
|v1.3.1                     |k8s.gcr.io/provider-aws/aws-ebs-csi-driver:v1.3.1 | public.ecr.aws/ebs-csi-driver/aws-ebs-csi-driver:v1.3.1                     |
|v1.3.0                     |k8s.gcr.io/provider-aws/aws-ebs-csi-driver:v1.3.0 | 602401143452.dkr.ecr.us-west-2.amazonaws.com/eks/aws-ebs-csi-driver:v1.3.0  |
|v1.2.1                     |k8s.gcr.io/provider-aws/aws-ebs-csi-driver:v1.2.1 | 602401143452.dkr.ecr.us-west-2.amazonaws.com/eks/aws-ebs-csi-driver:v1.2.1  |
|v1.2.0                     |k8s.gcr.io/provider-aws/aws-ebs-csi-driver:v1.2.0 | 602401143452.dkr.ecr.us-west-2.amazonaws.com/eks/aws-ebs-csi-driver:v1.2.0  |
|v1.1.4                     |k8s.gcr.io/provider-aws/aws-ebs-csi-driver:v1.1.4 | 602401143452.dkr.ecr.us-west-2.amazonaws.com/eks/aws-ebs-csi-driver:v1.1.4  |
|v1.1.3                     |k8s.gcr.io/provider-aws/aws-ebs-csi-driver:v1.1.3 | 602401143452.dkr.ecr.us-west-2.amazonaws.com/eks/aws-ebs-csi-driver:v1.1.3  |
|v1.1.2                     |k8s.gcr.io/provider-aws/aws-ebs-csi-driver:v1.1.2 | 602401143452.dkr.ecr.us-west-2.amazonaws.com/eks/aws-ebs-csi-driver:v1.1.2  |
|v1.1.1                     |k8s.gcr.io/provider-aws/aws-ebs-csi-driver:v1.1.1 | 602401143452.dkr.ecr.us-west-2.amazonaws.com/eks/aws-ebs-csi-driver:v1.1.1  |
|v1.1.0                     |k8s.gcr.io/provider-aws/aws-ebs-csi-driver:v1.1.0 | 602401143452.dkr.ecr.us-west-2.amazonaws.com/eks/aws-ebs-csi-driver:v1.1.0  |
|v1.0.0                     |k8s.gcr.io/provider-aws/aws-ebs-csi-driver:v1.0.0 |                                                                             |
|v0.10.1                    |k8s.gcr.io/provider-aws/aws-ebs-csi-driver:v0.10.1| 602401143452.dkr.ecr.us-west-2.amazonaws.com/eks/aws-ebs-csi-driver:v0.10.1 |
|v0.10.0                    |k8s.gcr.io/provider-aws/aws-ebs-csi-driver:v0.10.0|                                                                             |
|v0.9.1                     |k8s.gcr.io/provider-aws/aws-ebs-csi-driver:v0.9.1 |                                                                             |
|v0.9.0                     |k8s.gcr.io/provider-aws/aws-ebs-csi-driver:v0.9.0 | 602401143452.dkr.ecr.us-west-2.amazonaws.com/eks/aws-ebs-csi-driver:v0.9.0  |
|v0.8.1                     |k8s.gcr.io/provider-aws/aws-ebs-csi-driver:v0.8.1 |                                                                             |
|v0.7.1                     |                                                  | amazon/aws-ebs-csi-driver:v0.7.1                                            |
|v0.6.0                     |                                                  | amazon/aws-ebs-csi-driver:v0.6.0                                            |
|v0.5.0                     |                                                  | amazon/aws-ebs-csi-driver:v0.5.0                                            |
|v0.4.0                     |                                                  | amazon/aws-ebs-csi-driver:v0.4.0                                            |
|v0.3.0                     |                                                  | amazon/aws-ebs-csi-driver:v0.3.0                                            |
|v0.2.0                     |                                                  | amazon/aws-ebs-csi-driver:0.2.0                                             |
|v0.1.0                     |                                                  | amazon/aws-ebs-csi-driver:0.1.0-alpha                                       |

**Note**: If your cluster isn't in the `us-west-2` Region, please change `602401143452.dkr.ecr.us-west-2.amazonaws.com` to the [address](https://github.com/awsdocs/amazon-eks-user-guide/blob/master/doc_source/add-ons-images.md) that corresponds to your Region.
</details>

## Kubernetes Compatibility Matrix

| AWS EBS CSI Driver / Kubernetes Version| v1.12 | v1.13 | v1.14 | v1.15 | v1.16 | v1.17 | v1.18+|
|----------------------------------------|-------|-------|-------|-------|-------|-------|-------|
| master branch                          | no    | no    | no    | no    | no    | yes   | yes   |
| v0.9.x-v1.11.x                          | no    | no    | no    | no    | no    | yes   | yes   |
| v0.5.0-v0.8.x                          | no    | no    | yes   | yes   | yes   | yes   | yes   |
| v0.4.0                                 | no    | no    | yes   | yes   | no    | no    | no    |
| v0.3.0                                 | no    | no    | yes   | no    | no    | no    | no    |
| v0.2.0                                 | no    | yes   | yes   | no    | no    | no    | no    |
| v0.1.0                                 | yes   | yes   | yes   | no    | no    | no    | no    |
 
## CSI Specification Compatibility Matrix
| AWS EBS CSI Driver / CSI Version       | v0.3.0| v1.0.0 | v1.1.0 |
|----------------------------------------|-------|--------|--------|
| master branch                          | no    | no     | yes    |
| v0.4.0-v1.11.x                          | no    | no     | yes    |
| v0.2.0-v0.3.0                          | no    | yes    | no     |
| v0.1.0                                 | yes   | no     | no     |

## Documentation

* [Driver Installation](docs/install.md)
* [Driver Launch Options](docs/options.md)
* [StorageClass Parameters](docs/parameters.md)
* [Volume Tagging](docs/tagging.md)
* [Kubernetes Examples](/examples/kubernetes)
* [Development and Contributing](CONTRIBUTING.md)
