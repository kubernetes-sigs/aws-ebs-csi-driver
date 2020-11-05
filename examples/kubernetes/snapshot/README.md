# Volume Snapshots

## Overview

This driver implements basic volume snapshotting functionality using the [external snapshotter](https://github.com/kubernetes-csi/external-snapshotter) sidecar and creates snapshots of EBS volumes using the `VolumeSnapshot` custom resources.

## Prerequisites

1. Kubernetes 1.13+ (CSI 1.0).

1. The `VolumeSnapshotDataSource` must be set in `--feature-gates=` in the `kube-apiserver`.

1. The [aws-ebs-csi-driver driver](https://github.com/kubernetes-sigs/aws-ebs-csi-driver) with alpha features is installed.
1. If your cluster does not come pre-installed with Snapshot Beta CRDs, install the [Snapshot Beta CRDs](https://github.com/kubernetes-csi/external-snapshotter/tree/master/client/config/crd)

### Usage

1. Create the `StorageClass` and `VolumeSnapshotClass`:
```
kubectl apply -f specs/classes/
```

2. Create a sample app and the `PersistentVolumeClaim`: 
```
kubectl apply -f specs/app/
```

3. Validate the volume was created and `volumeHandle` contains an EBS volumeID: 
```
kubectl describe pv
```

4. Validate the pod successfully wrote data to the volume, taking note of the timestamp of the first entry:
```
kubectl exec -it app cat /data/out.txt
```

5. Create a `VolumeSnapshot` referencing the `PersistentVolumeClaim` name:
```
kubectl apply -f specs/snapshot/
```

6. Wait for the `Ready To Use:  true` attribute of the `VolumeSnapshot`: 
```
kubectl describe volumesnapshot.snapshot.storage.k8s.io ebs-volume-snapshot
```

7. Delete the existing app:
```
kubectl delete -f specs/app/
```

8. Restore a volume from the snapshot with a `PersistentVolumeClaim` referencing the `VolumeSnapshot` in its `dataSource`:
```
kubectl apply -f specs/snapshot-restore/
```

9. Validate the new pod has the restored data by comparing the timestamp of the first entry to that of in step 4:
```
kubectl exec -it app cat /data/out.txt
```

10. Cleanup resources:
```
kubectl delete -f specs/snapshot-restore
kubectl delete -f specs/snapshot
kubectl delete -f specs/classes
```
