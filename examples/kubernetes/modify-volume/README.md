# Volume Modification via `VolumeAttributesClass`

## Prerequisites

This example will only work on a cluster with the `VolumeAttributesClass` feature enabled. For more information see the [installation instructions in the EBS CSI `ModifyVolume` documentation](/docs/modify-volume.md).

## Usage

1. Deploy the example `Pod`, `PersistentVolumeClaim`, and `StorageClass` to your cluster
    ```sh
    $ kubectl apply -f manifests/pod-with-volume.yaml

    storageclass.storage.k8s.io/ebs-sc created
    persistentvolumeclaim/ebs-claim created
    pod/app created
    ```

2. Wait for the `PersistentVolumeClaim` to bind and the pod to reach the `Running` state
    ```sh
    $ kubectl get pvc ebs-claim
    NAME        STATUS   VOLUME                                     CAPACITY   ACCESS MODES   STORAGECLASS   VOLUMEATTRIBUTESCLASS   AGEebs-claim   Bound    pvc-076b2d14-b643-47d4-a2ce-fbf9cd36572b   100Gi      RWO            ebs-sc         <unset>                 2m51s
    
    $ kubectl get pod app
    NAME   READY   STATUS    RESTARTS   AGE
    app    1/1     Running   0          3m24
    ```

3. Watch the logs of the pod
    ```sh
    $ kubectl logs -f app
    Mon Feb 26 22:28:19 UTC 2024
    Mon Feb 26 22:28:24 UTC 2024
    Mon Feb 26 22:28:29 UTC 2024
    Mon Feb 26 22:28:34 UTC 2024
    Mon Feb 26 22:28:39 UTC 2024
    ...
    ```
4. Deploy the `VolumeAttributesClass`
    ```sh
    $ kubectl apply -f manifests/volumeattributesclass.yaml
    ```

5. Edit the `PersistentVolumeClaim` to point to this class
    ```sh
    $ kubectl patch pvc ebs-claim --patch '{"spec": {"volumeAttributesClassName": "io2-class"}}'
    persistentvolumeclaim/ebs-claim patched
    ```

6. Wait for the `VolumeAttributesClass` to apply to the volume
    ```sh
    $ kubectl get pvc ebs-claim
    NAME        STATUS   VOLUME                                     CAPACITY   ACCESS MODES   STORAGECLASS   VOLUMEATTRIBUTESCLASS   AGE
    ebs-claim   Bound    pvc-076b2d14-b643-47d4-a2ce-fbf9cd36572b   100Gi      RWO            ebs-sc         io2-class               5m54s
    ```

7. (Optional) Delete example resources
    ```sh
    $ kubectl delete -f manifests 
    storageclass.storage.k8s.io "ebs-sc" deleted
    persistentvolumeclaim "ebs-claim" deleted
    pod "app" deleted
    volumeattributesclass.storage.k8s.io "io2-class" deleted
    ```
