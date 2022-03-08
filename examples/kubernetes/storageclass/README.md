# Configuring StorageClass

## Prerequisites

1. Kubernetes 1.13+ (CSI 1.0).
2. The [aws-ebs-csi-driver](https://github.com/kubernetes-sigs/aws-ebs-csi-driver) installed.

## Usage

This example shows you how to write to a dynamically provisioned EBS volume with a specified configuration in the `StorageClass` resource.

1. Modify the `StorageClass` resource in [`storageclass.yaml`](./manifests/storageclass.yaml) as desired. For a list of supported parameters consult the Kubernetes documentation on [Storage Classes](https://kubernetes.io/docs/concepts/storage/storage-classes/#aws-ebs).

2. Deploy the provided pod on your cluster along with the `StorageClass` and `PersistentVolumeClaim`:
    ```sh
    $ kubectl apply -f manifests

    persistentvolumeclaim/ebs-claim created
    pod/app created
    storageclass.storage.k8s.io/ebs-sc created
    ```

3. Validate the `PersistentVolumeClaim` is bound to your `PersistentVolume`.
    ```sh
    $ kubectl get pvc ebs-claim

    NAME        STATUS   VOLUME                                     CAPACITY   ACCESS MODES   STORAGECLASS   AGE
    ebs-claim   Bound    pvc-1fb712f2-632d-4b63-92e4-3b773d698ae1   4Gi        RWO            ebs-sc         17s
    ```

4. Validate the pod successfully wrote data to the dynamically provisioned volume:
    ```sh
    $ kubectl exec app -- cat /data/out.txt

    Wed Feb 23 19:56:12 UTC 2022
    ...
    ```

5. Cleanup resources:
    ```sh
    $ kubectl delete -f manifests

    persistentvolumeclaim "ebs-claim" deleted
    pod "app" deleted
    storageclass.storage.k8s.io "ebs-sc" deleted
    ```