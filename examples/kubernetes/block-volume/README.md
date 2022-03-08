# Raw Block Volume

## Prerequisites

1. Kubernetes 1.13+ (CSI 1.0).
2. The [aws-ebs-csi-driver](https://github.com/kubernetes-sigs/aws-ebs-csi-driver) installed.

## Usage

This example shows you how to create and consume a dynamically-provisioned EBS volume as a raw block device.

1. Deploy the provided pod on your cluster along with the `StorageClass` and `PersistentVolumeClaim`:
    ```sh
    $ kubectl apply -f manifests

    pod/app created
    persistentvolumeclaim/block-claim created
    storageclass.storage.k8s.io/ebs-sc created
    ```

2. Validate the `PersistentVolumeClaim` is bound to your `PersistentVolume`.
    ```sh
    $ kubectl get pvc block-claim

    NAME          STATUS   VOLUME                                     CAPACITY   ACCESS MODES   STORAGECLASS   AGE
    block-claim   Bound    pvc-2074bf0a-4726-44f2-bb7a-eb4292d4f40a   10Gi       RWO            ebs-sc
    ```

3. Cleanup resources:
    ```sh
    $ kubectl delete -f manifests

    pod "app" deleted
    persistentvolumeclaim "block-claim" deleted
    storageclass.storage.k8s.io "ebs-sc" deleted
    ```