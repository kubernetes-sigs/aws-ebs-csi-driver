## Raw Block Volume
This example shows how to consume a dynamically-provisioned EBS volume as a raw block device.

### Edit [Persistence Volume Claim Spec](./specs/raw-claim.yaml)
Make sure the `volumeMode` is `Block`.

### Edit [Application Pod](./specs/pod.yaml)
Make sure the pod is consuming the PVC with the defined name and `volumeDevices` is used instead of `volumeMounts`.

### Deploy the Application
```sh
kubectl apply -f examples/kubernetes/block-volume/specs/storageclass.yaml
kubectl apply -f examples/kubernetes/block-volume/specs/raw-claim.yaml
kubectl apply -f examples/kubernetes/block-volume/specs/pod.yaml
```

### Access Block Device
After the objects are created, verify that pod is running:

```sh
$ kubectl get pods
NAME   READY   STATUS    RESTARTS   AGE
app    1/1     Running   0          16m
```
Verify the device node is mounted inside the container:

```sh
$ kubectl exec -it app -- ls -al /dev/xvda
brw-rw----    1 root     disk      202, 23296 Mar 12 04:23 /dev/xvda
```

Write to the device using:

```sh
dd if=/dev/zero of=/dev/xvda bs=1024k count=100
100+0 records in
100+0 records out
104857600 bytes (105 MB, 100 MiB) copied, 0.0492386 s, 2.1 GB/s
```
