# EBS-backed Generic Ephemeral Volume

## Prerequisites

1. Kubernetes 1.23+
2. The [aws-ebs-csi-driver](https://github.com/kubernetes-sigs/aws-ebs-csi-driver) installed.

## Usage

This example shows you how to use EBS-backed [generic ephemeral volumes](https://kubernetes.io/docs/concepts/storage/ephemeral-volumes/#generic-ephemeral-volumes) in your cluster to have Kubernetes ensure volumes are created and deleted alongside their Pods. See [Kubernetes documentation on generic ephemeral volume lifecycle and PersistentVolumeClaim](https://kubernetes.io/docs/concepts/storage/ephemeral-volumes/#lifecycle-and-persistentvolumeclaim) for more information.

1. Deploy the provided pod on your cluster along with the `StorageClass`. Note that we do not create a `PersistentVolumeClaim`:
    ```sh
    $ kubectl apply -f manifests

    pod/app created
    storageclass.storage.k8s.io/ebs-ephemeral-demo created
    ```

2. Validate the `PersistentVolumeClaim` Kubernetes created on your behalf is bound to a `PersistentVolume`.
    ```sh
    $ kubectl get pvc app-persistent-storage

    NAME        STATUS   VOLUME                                     CAPACITY   ACCESS MODES   STORAGECLASS   
    ebs-claim   Bound    pvc-9124c6d0-382a-49c5-9494-bcb60f6c0c9c   1Gi        RWO            ebs-ephemeral-demo 
    ```

3. Validate the pod successfully wrote data to the volume:
    ```sh
    $ kubectl exec app -- cat /data/out.txt

    Fri Jan 24 17:15:42 UTC 2025
    ...
    ```

4. Cleanup resources:
    ```sh
    $ kubectl delete -f manifests

    pod "app" deleted
    storageclass.storage.k8s.io "ebs-ephemeral-demo" deleted
    ```
   
5. Validate that Kubernetes deleted the `PersistentVolumeClaim` and released the `PersistentVolume` on your behalf:
    ```sh
    $ kubectl get pvc app-persistent-storage

    Error from server (NotFound): persistentvolumeclaims "app-persistent-storage" not found

    $ kubectl get pv <PV_NAME>
   
    Error from server (NotFound): persistentvolumes "pvc-9124c6d0-382a-49c5-9494-bcb60f6c0c9c" not found
    ```
