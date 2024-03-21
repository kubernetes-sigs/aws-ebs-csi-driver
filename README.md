# Amazon Elastic Block Store (EBS) CSI driver
[![GitHub release (latest SemVer)](https://img.shields.io/github/v/release/kubernetes-sigs/aws-ebs-csi-driver)](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/releases)
[![Go Report Card](https://goreportcard.com/badge/github.com/kubernetes-sigs/aws-ebs-csi-driver)](https://goreportcard.com/report/github.com/kubernetes-sigs/aws-ebs-csi-driver)

## Overview

The [Amazon Elastic Block Store](https://aws.amazon.com/ebs/) Container Storage Interface (CSI) Driver provides a [CSI](https://github.com/container-storage-interface/spec/blob/master/spec.md) interface used by Container Orchestrators to manage the lifecycle of Amazon EBS volumes.

## Features
* **Static Provisioning** - Associate an externally-created EBS volume with a [PersistentVolume](https://kubernetes.io/docs/concepts/storage/persistent-volumes/) (PV) for consumption within Kubernetes.
* **Dynamic Provisioning** - Automatically create EBS volumes and associated [PersistentVolumes](https://kubernetes.io/docs/concepts/storage/persistent-volumes/) (PV) from [PersistentVolumeClaims](https://kubernetes.io/docs/concepts/storage/persistent-volumes/#dynamic)) (PVC). Parameters can be passed via a [StorageClass](https://kubernetes.io/docs/concepts/storage/storage-classes/#the-storageclass-resource) for fine-grained control over volume creation.
* **Mount Options** - Mount options could be specified in the [PersistentVolume](https://kubernetes.io/docs/concepts/storage/persistent-volumes/) (PV) resource to define how the volume should be mounted.
* **NVMe Volumes** - Consume [NVMe](https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/nvme-ebs-volumes.html) volumes from EC2 [Nitro instances](https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/instance-types.html#ec2-nitro-instances).
* **Block Volumes** - Consume an EBS volume as a [raw block device](https://kubernetes-csi.github.io/docs/raw-block.html).
* **Volume Snapshots** - Create and restore [snapshots](https://kubernetes.io/docs/concepts/storage/volume-snapshots/) taken from a volume in Kubernetes.
* **Volume Resizing** - Expand the volume by specifying a new size in the [PersistentVolumeClaim](https://kubernetes.io/docs/concepts/storage/persistent-volumes/#expanding-persistent-volumes-claims) (PVC).
* **Volume Modification** - Change the properties (type, iops, or throughput) [via a `VolumeAttributesClass`](examples/kubernetes/modify-volume).

## Container Images

| Driver Version | [registry.k8s.io](https://kubernetes.io/blog/2022/11/28/registry-k8s-io-faster-cheaper-ga/) Image | [ECR Public](https://gallery.ecr.aws/ebs-csi-driver/aws-ebs-csi-driver) Image |
|----------------|---------------------------------------------------------------------------------------------------|-------------------------------------------------------------------------------|
| v1.29.0        | registry.k8s.io/provider-aws/aws-ebs-csi-driver:v1.29.0                                           | public.ecr.aws/ebs-csi-driver/aws-ebs-csi-driver:v1.29.0                      |
| v1.28.0        | registry.k8s.io/provider-aws/aws-ebs-csi-driver:v1.28.0                                           | public.ecr.aws/ebs-csi-driver/aws-ebs-csi-driver:v1.28.0                      |

## Releases

The EBS CSI Driver publishes monthly releases. Unscheduled releases may be published for patches to security vulnerabilities and other fixes deemed urgent.

The EBS CSI Driver follows [semantic versioning](https://semver.org/). The version will be bumped following the rules below:

* Significant breaking changes will be released as a `MAJOR` update.
* New features will be released as a `MINOR` update.
* Bug or vulnerability fixes will be released as a `PATCH` update.

Monthly releases will contain at minimum a `MINOR` version bump, even if the content would normally be treated as a `PATCH` version.

## Support

Support will be provided for the latest version and one prior version. Bugs or vulnerabilities found in the latest version will be backported to the previous release in a new minor version.

This policy is non-binding and subject to change.

## Compatibility

The EBS CSI Driver is compatible with all Kubernetes versions supported by [the Kubernetes project](https://kubernetes.io/releases/) and/or [Amazon EKS (including extended support versions)](https://docs.aws.amazon.com/eks/latest/userguide/kubernetes-versions.html).

The EBS CSI Driver implements the [Container Storage Interface specification](https://github.com/container-storage-interface/spec/blob/master/spec.md) version `v1.9.0`.

## Documentation

* [Driver Installation](docs/install.md)
* [Driver Launch Options](docs/options.md)
* [StorageClass Parameters](docs/parameters.md)
* [Frequently Asked Questions](docs/faq.md)
* [Volume Tagging](docs/tagging.md)
* [Volume Modification](docs/modify-volume.md)
* [Kubernetes Examples](/examples/kubernetes)
* [Driver Uninstallation](docs/install.md#uninstalling-the-ebs-csi-driver)
* [Development and Contributing](CONTRIBUTING.md)
