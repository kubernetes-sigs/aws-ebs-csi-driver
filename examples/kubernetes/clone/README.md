# Volume Cloning

## Prerequisites

1. The [aws-ebs-csi-driver](https://github.com/kubernetes-sigs/aws-ebs-csi-driver) on at least v1.51.0 installed.
2. Default managed policy will need to be adjusted see [install.md](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/blob/master/docs/install.md)  

## Usage

This example shows you how to clone an existing EBS `PersistentVolume` to create a new volume with the same data.

### Create Source Volume

1. Create the `StorageClass`:
    ```sh
    $ kubectl apply -f manifests/classes/

    storageclass.storage.k8s.io/ebs-sc created
    ```

2. Deploy the provided pod on your cluster along with the `PersistentVolumeClaim`:
    ```sh
    $ kubectl apply -f manifests/app/

    persistentvolumeclaim/ebs-claim created
    pod/app created
    ```

3. Validate the `PersistentVolumeClaim` is bound to your `PersistentVolume`:
    ```sh
    $ kubectl get pvc ebs-claim

    NAME        STATUS   VOLUME                                     CAPACITY   ACCESS MODES   STORAGECLASS   AGE
    ebs-claim   Bound    pvc-c2e5476c-d9f5-4a49-bbb2-9dfdb27db4c8   4Gi        RWO            ebs-sc         43s
    ```

4. Validate the pod successfully wrote data to the volume:
    ```sh
    $ kubectl exec app -- cat /data/out.txt

    Thu Feb 24 04:07:57 UTC 2022
    ...
    ```

### Clone Volume

5. Create a clone of the existing volume with a `PersistentVolumeClaim` referencing the source PVC in its `dataSource`:
    ```sh
    $ kubectl apply -f manifests/clone/

    persistentvolumeclaim/ebs-clone-claim created
    pod/clone-app created
    ```

6. Validate the cloned volume contains the same data:
    ```sh
    $ kubectl exec clone-app -- cat /data/out.txt

    Thu Feb 24 04:07:57 UTC 2022
    ...
    ```

7. Cleanup resources:
    ```sh
    $ kubectl delete -f manifests/clone/

    persistentvolumeclaim "ebs-clone-claim" deleted
    pod "clone-app" deleted
    ```

    ```sh
    $ kubectl delete -f manifests/app/

    persistentvolumeclaim "ebs-claim" deleted
    pod "app" deleted
    ```

    ```sh
    $ kubectl delete -f manifests/classes/

    storageclass.storage.k8s.io "ebs-sc" deleted
    ```
