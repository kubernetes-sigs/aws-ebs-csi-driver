# Multi-Attach

The multi-attach capability allows you to attach a single EBS volume to multiple EC2 instances located within the same Availability Zone (AZ). This shared volume can be utilized by several pods running on distinct nodes.

Multi-attach is enabled by specifying `ReadWriteMany` for the `PersistentVolumeClaim.spec.accessMode`.

## Important

- Application-level coordination (e.g., via I/O fencing) is required to use multi-attach safely. Failure to do so can result in data loss and silent data corruption. Refer to the AWS documentation on Multi-Attach for more information.
- Currently, the EBS CSI driver only supports multi-attach for `IO2` volumes in `Block` mode.

Refer to the official AWS documentation on [Multi-Attach](https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/ebs-volumes-multi.html) for more information, best practices, and limitations of this capability.

## Example

1. Create a `StorageClass` referencing an `IO2` volume type:
```
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: ebs-sc
provisioner: ebs.csi.aws.com
volumeBindingMode: WaitForFirstConsumer
parameters:
  type: io2
  iops: "1000"
```

2. Create a `PersistentVolumeClaim` referencing the `ReadWriteMany` access and `Block` device modes:
```
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: block-claim
spec:
  accessModes:
    - ReadWriteMany
  volumeMode: Block
  storageClassName: ebs-sc
  resources:
    requests:
      storage: 4Gi
```

3. Create a `DaemonSet` referencing the `PersistentVolumeClaim` created in the previous step:
```
apiVersion: apps/v1
kind: DaemonSet
metadata:
  name: app-daemon
spec:
  selector:
    matchLabels:
      name: app
  template:
    metadata:
      labels:
        name: app
    spec:
      containers:
      - name: app
        image: busybox
        command: ["/bin/sh", "-c"]
        args: ["tail -f /dev/null"]
        volumeDevices:
        - name: data
          devicePath: /dev/xvda
      volumes:
      - name: data
        persistentVolumeClaim:
          claimName: block-claim
```

4. Verify the `DaemonSet` is running:
```
$ kubectl get pods -A

NAMESPACE     NAME                                   READY   STATUS    RESTARTS       AGE
default       app-daemon-9hdgw                       1/1     Running   0              18s
default       app-daemon-xm8zr                       1/1     Running   0              18s
```
