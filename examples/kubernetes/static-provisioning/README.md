# Static Provisioning

## Prerequisites

1. Kubernetes 1.13+ (CSI 1.0).
2. The [aws-ebs-csi-driver](https://github.com/kubernetes-sigs/aws-ebs-csi-driver) installed.
3. Created an [Amazon EBS volume](https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/ebs-volume-types.html).

## Usage

This example shows you how to create and consume a `PersistentVolume` from an existing EBS volume with static provisioning.

1. Edit the `PersistentVolume` manifest in [pv.yaml](./manifests/pv.yaml) to include your `volumeHandle` EBS volume ID and `nodeSelectorTerms` zone value.

    ```
    apiVersion: v1
    kind: PersistentVolume
    metadata:
      name: test-pv
    spec:
      accessModes:
      - ReadWriteOnce
      capacity:
        storage: 5Gi
      csi:
        driver: ebs.csi.aws.com
        fsType: ext4
        volumeHandle: {EBS volume ID}
      nodeAffinity:
        required:
          nodeSelectorTerms:
            - matchExpressions:
                - key: topology.ebs.csi.aws.com/zone
                  operator: In
                  values:
                    - {availability zone}
    ```

2. Deploy the provided pod on your cluster along with the `PersistentVolume` and `PersistentVolumeClaim`:
    ```sh
    $ kubectl apply -f manifests

    persistentvolumeclaim/ebs-claim created
    pod/app created
    persistentvolume/test-pv created
    ```

3. Validate the `PersistentVolumeClaim` is bound to your `PersistentVolume`.
    ```sh
    $ kubectl get pvc ebs-claim

    NAME        STATUS   VOLUME    CAPACITY   ACCESS MODES   STORAGECLASS   AGE
    ebs-claim   Bound    test-pv   5Gi        RWO                           53s
    ```

4. Validate the pod successfully wrote data to the statically provisioned volume:
    ```sh
    $ kubectl exec app -- cat /data/out.txt

    Tue Feb 22 20:51:37 UTC 2022
    ...
    ```

5. Cleanup resources:
    ```sh
    $ kubectl delete -f manifests

    persistentvolumeclaim "ebs-claim" deleted
    pod "app" deleted
    persistentvolume "test-pv" deleted
    ```