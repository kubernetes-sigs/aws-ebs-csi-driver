# Volume Resizing

## Prerequisites

1. Kubernetes 1.13+ (CSI 1.0).
2. The [aws-ebs-csi-driver](https://github.com/kubernetes-sigs/aws-ebs-csi-driver) installed.

## Usage

In this example, an EBS volume of `4Gi` is resized to `8Gi` using the volume resizing feature.

1. Deploy the provided pod on your cluster along with the `StorageClass` and `PersistentVolumeClaim`:
    ```sh
    $ kubectl apply -f manifests

    persistentvolumeclaim/ebs-claim created
    pod/app created
    storageclass.storage.k8s.io/resize-sc created
    ```

2. Validate the `PersistentVolumeClaim` is bound to your `PersistentVolume`.
    ```sh
    $ kubectl get pvc ebs-claim

    NAME        STATUS   VOLUME                                     CAPACITY   ACCESS MODES   STORAGECLASS   AGE
    ebs-claim   Bound    pvc-b0f6d590-f4b3-4329-a118-49cd09f6993c   4Gi        RWO            resize-sc      28s
    ```

3. Expand the volume size by increasing the `capacity` specification in the `PersistentVolumeClaim`.
    ```sh
    $ export KUBE_EDITOR="nano" && kubectl edit pvc ebs-claim
    ```

4. Verify that both the persistence volume and persistence volume claim have been appropriately resized:
    ```sh
    $ kubectl get pv && kubectl get pvc

    NAME                                       CAPACITY   ACCESS MODES   RECLAIM POLICY   STATUS   CLAIM               STORAGECLASS   
    pvc-b0f6d590-f4b3-4329-a118-49cd09f6993c   8Gi        RWO            Delete           Bound    default/ebs-claim   resize-sc   

    NAME        STATUS   VOLUME                                     CAPACITY   ACCESS MODES   STORAGECLASS   AGE
    ebs-claim   Bound    pvc-b0f6d590-f4b3-4329-a118-49cd09f6993c   8Gi        RWO            resize-sc      23m
    ```

5. Cleanup resources:
    ```sh
    $ kubectl delete -f manifests

    persistentvolumeclaim "ebs-claim" deleted
    pod "app" deleted
    storageclass.storage.k8s.io "resize-sc" deleted
    ```
