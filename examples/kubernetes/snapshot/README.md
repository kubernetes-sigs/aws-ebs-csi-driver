# Volume Snapshots with AWS EBS CSI Driver

## Overview

This driver implements basic volume snapshotting functionality, i.e. it is possible to use it along with the [external
snapshotter](https://github.com/kubernetes-csi/external-snapshotter) sidecar and create snapshots of EBS volumes using
the `VolumeSnapshot` custom resources.

## Prerequisites

1. Kubernetes 1.13+ (CSI 1.0) is required

2. The `VolumeSnapshotDataSource` feature gate of Kubernetes API server and controller manager must be turned on.

## Usage

This directory contains example YAML files to test the feature. First, see the [deployment example](../../../deploy/kubernetes) and [volume scheduling example](../volume_scheduling)
to set up the external provisioner:

### Set up

1. Create the RBAC rules

2. Start the contoller `StatefulSet`

3. Start the node `DaemonSet`

4. Create a `StorageClass` for dynamic provisioning of the AWS CSI volumes

5. Create a `SnapshotClass` to create `VolumeSnapshot`s using the AWS CSI external controller

6. Create a `PersistentVolumeClaim` and a pod using it

### Taking and restoring volume snapshot

7. Create a `VolumeSnapshot` referencing the `PersistentVolumeClaim`; the snapshot creation may take time to finish:
   check the `ReadyToUse` attribute of the `VolumeSnapshot` object to find out when a new `PersistentVolume` can be
   created from the snapshot

8. To restore a volume from a snapshot use a `PersistentVolumeClaim` referencing the `VolumeSnapshot` in its `dataSource`; see the
   [Kubernetes Persistent Volumes documentation](https://kubernetes.io/docs/concepts/storage/persistent-volumes/#volume-snapshot-and-restore-volume-from-snapshot-support)
   and the example [restore claim](./restore-claim.yaml)
