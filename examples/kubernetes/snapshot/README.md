# Volume Snapshots

## Prerequisites

1. Kubernetes 1.13+ (CSI 1.0).
2. The [aws-ebs-csi-driver](https://github.com/kubernetes-sigs/aws-ebs-csi-driver) installed.
3. The [external snapshotter](https://github.com/kubernetes-csi/external-snapshotter) installed.
4. The `VolumeSnapshotDataSource` is set in `--feature-gates=` in the `kube-apiserver` specification. This feature is enabled by default from Kubernetes v1.17+. 

## Usage

This example shows you how to create a snapshot and restore an EBS `PersistentVolume`.

### Create Snapshot

1. Create the `StorageClass` and `VolumeSnapshotClass`:
    ```sh
    $ kubectl apply -f manifests/classes/

    volumesnapshotclass.snapshot.storage.k8s.io/csi-aws-vsc created
    storageclass.storage.k8s.io/ebs-sc created
    ```

2. Deploy the provided pod on your cluster along with the `PersistentVolumeClaim`:
    ```sh
    $ kubectl apply -f manifests/app/

    persistentvolumeclaim/ebs-claim created
    pod/app created
    ```

3. Validate the `PersistentVolumeClaim` is bound to your `PersistentVolume`.
    ```sh
    $ kubectl get pvc ebs-claim

    NAME        STATUS   VOLUME                                     CAPACITY   ACCESS MODES   STORAGECLASS   AGE
    ebs-claim   Bound    pvc-c2e5476c-d9f5-4a49-bbb2-9dfdb27db4c8   4Gi        RWO            ebs-sc         43s
    ```

4. Validate the pod successfully wrote data to the volume, taking note of the timestamp of the first entry:
    ```sh
    $ kubectl exec app -- cat /data/out.txt

    Thu Feb 24 04:07:57 UTC 2022
    ...
    ```

5. Create a `VolumeSnapshot` referencing the `PersistentVolumeClaim` name:
    ```sh
    $ kubectl apply -f manifests/snapshot/

    volumesnapshot.snapshot.storage.k8s.io/ebs-volume-snapshot created
    ```

6. Wait for the `Ready To Use:  true` attribute of the `VolumeSnapshot`: 
    ```sh
    $ kubectl describe volumesnapshot.snapshot.storage.k8s.io ebs-volume-snapshot

    ...
    Status:
    Bound Volume Snapshot Content Name:  snapcontent-333215f5-ab85-42b8-b4fc-27a6cba0cc19
    Creation Time:                       2022-02-24T04:09:51Z
    Ready To Use:                        true
    Restore Size:                        4Gi
    ```

7. Delete the existing app:
    ```sh
    $ kubectl delete -f manifests/app/

    persistentvolumeclaim "ebs-claim" deleted
    pod "app" deleted
    ```
### Restore Volume

8. Restore a volume from the snapshot with a `PersistentVolumeClaim` referencing the `VolumeSnapshot` in its `dataSource`:
    ```sh
    $ kubectl apply -f manifests/snapshot-restore/

    persistentvolumeclaim/ebs-snapshot-restored-claim created
    pod/app created
    ```

9. Validate the new pod has the restored data by comparing the timestamp of the first entry to that of in step 4:
    ```sh
    $ kubectl exec app -- cat /data/out.txt
    
    Thu Feb 24 04:07:57 UTC 2022
    ...
    ```

10. Cleanup resources:
    ```sh
    $ kubectl delete -f manifests/snapshot-restore

    persistentvolumeclaim "ebs-snapshot-restored-claim" deleted
    pod "app" deleted
    ```

    ```sh
    $ kubectl delete -f manifests/snapshot
    
    volumesnapshot.snapshot.storage.k8s.io "ebs-volume-snapshot" deleted
    ```

    ```sh
    $ kubectl delete -f manifests/classes

    volumesnapshotclass.snapshot.storage.k8s.io "csi-aws-vsc" deleted
    storageclass.storage.k8s.io "ebs-sc" deleted
    ```
