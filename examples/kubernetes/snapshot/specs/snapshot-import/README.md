# Volume Snapshots

## Overview

This driver implements basic volume snapshotting functionality using the [external snapshotter](https://github.com/kubernetes-csi/external-snapshotter) sidecar and creates snapshots of EBS volumes using the `VolumeSnapshot` custom resources.

## Prerequisites

1. Kubernetes 1.17+ (CSI 1.0).

1. The `VolumeSnapshotDataSource` must be set in `--feature-gates=` in the `kube-apiserver`. This feature is enabled by default from Kubernetes v1.17+. 

1. Install Snapshot Beta CRDs, Common Snapshot Controller, & CSI Driver (with alpha features) per CSI Snapshotter [Doc](https://github.com/kubernetes-csi/external-snapshotter#usage)




### Usage

1. Edit the PersistentVolume spec in [example manifest](./static-snapshot/volume-snapshot-content.yaml). Update `snapshotHandle` with EBS snapshot ID that you are going to use. In this example, I have a pre-created EBS snapshot in us-west-2b availability zone.

```
apiVersion: snapshot.storage.k8s.io/v1beta1
kind: VolumeSnapshotContent
metadata:
  name: static-snapshot-content
spec:
  volumeSnapshotRef:
    kind: VolumeSnapshot
    name: static-snapshot-demo
    namespace: default 
  source:
    snapshotHandle: snap-0fba4d7649d765c50
  driver: ebs.csi.aws.com
  deletionPolicy: Delete
  volumeSnapshotClassName: csi-aws-vsc
```

2. Create the `StorageClass` and `VolumeSnapshotClass`:
```
kubectl apply -f specs/classes/
```

3. Create the `VolumeSnapshotContent` and `VolumeSnapshot`: 
```
kubectl apply -f specs/snapshot-import/static-snapshot/
```

4. Validate the VolumeSnapshot was created and `snapshotHandle` contains an EBS snapshotID: 
```
kubectl describe VolumeSnapshotContent
kubectl describe VolumeSnapshot
```

5. Create the `PersistentVolumeClaim` and `Pod`:
```
kubectl apply -f specs/snapshot-import/app/
```

6. Validate the pod successfully wrote data to the volume:
```
kubectl exec -it app cat /data/out.txt
```

7. Cleanup resources:
```
kubectl delete -f specs/snapshot-import/app
kubectl delete -f specs/snapshot-import/static-snapshot
kubectl delete -f specs/classes
```
