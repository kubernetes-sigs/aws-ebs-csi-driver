# Volume Modification

The EBS CSI Driver (starting from v1.19.0) supports volume modification through PVC annotations. This allows users to modify volume properties (e.g., volume type, IOPS, and throughput). 

## Installation
This feature is opt-in. 

To install this feature through the Helm chart, users must set `controller.volumeModificationFeature.enabled` in `values.yaml` to `true`.

This will install an additional sidecar (`volumemodifier`) that watches the Kubernetes API server for changes to PVC annotations and triggers an RPC call against the CSI driver.

## Usage

Users can specify the following PVC annotations:

- `ebs.csi.aws.com/volumeType`: to update the volume type
- `ebs.csi.aws.com/iops`: to update the IOPS
- `ebs.csi.aws.com/throughput`: to update the throughput

## Considerations

- Keep in mind the [6 hour cooldown period](https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_ModifyVolume.html) for EBS ModifyVolume. Multiple ModifyVolume calls for the same volume within a 6 hour period will fail. 
- It is not yet possible to update both the annotations and capacity of the PVC at the same time. This results in multiple RPC calls to the driver, and only one of them will succeed (due to the cooldown period). A future release of the driver will lift this restriction.
- Ensure that the desired volume properties are permissible. The driver does minimum client side validation. 

## Example

#### 1) Create a PVC.


```
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: ebs-claim
spec:
  accessModes:
    - ReadWriteOnce
  storageClassName: ebs-sc
  resources:
    requests:
      storage: 100Gi
---
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: ebs-sc
provisioner: ebs.csi.aws.com
volumeBindingMode: WaitForFirstConsumer
---
apiVersion: v1
kind: Pod
metadata:
  name: app
spec:
  containers:
  - name: app
    image: centos
    command: ["/bin/sh"]
    args: ["-c", "while true; do echo $(date -u) >> /data/out.txt; sleep 5; done"]
    volumeMounts:
    - name: persistent-storage
      mountPath: /data
  volumes:
  - name: persistent-storage
    persistentVolumeClaim:
      claimName: ebs-claim
```

#### 2) Verify the PVC is created.

```
$ k describe pvc ebs-claim
Name:          ebs-claim
Namespace:     default
StorageClass:  ebs-sc
Status:        Bound
Volume:        pvc-20c5ddec-5913-4b8d-a2fc-bfd0943b966d
Labels:        <none>
Annotations:   pv.kubernetes.io/bind-completed: yes
               pv.kubernetes.io/bound-by-controller: yes
               volume.beta.kubernetes.io/storage-provisioner: ebs.csi.aws.com
               volume.kubernetes.io/selected-node: ip-192-168-32-79.ec2.internal
               volume.kubernetes.io/storage-provisioner: ebs.csi.aws.com
Finalizers:    [kubernetes.io/pvc-protection]
Capacity:      100Gi
Access Modes:  RWO
VolumeMode:    Filesystem
Used By:       app
Events:
  Type     Reason                 Age              From                                                                                     Message
  ----     ------                 ----             ----                                                                                     -------
  Warning  ProvisioningFailed     6s               persistentvolume-controller                                                              storageclass.storage.k8s.io "ebs-sc" not found
  Normal   Provisioning           5s               ebs.csi.aws.com_ebs-csi-controller-b84fbbd88-5z66k_d2cb941a-aee8-41d6-aea5-8f4a40c7c82c  External provisioner is provisioning volume for claim "default/ebs-claim"
  Normal   ExternalProvisioning   4s (x2 over 5s)  persistentvolume-controller                                                              waiting for a volume to be created, either by external provisioner "ebs.csi.aws.com" or manually created by system administrator
  Normal   ProvisioningSucceeded  1s               ebs.csi.aws.com_ebs-csi-controller-b84fbbd88-5z66k_d2cb941a-aee8-41d6-aea5-8f4a40c7c82c  Successfully provisioned volume pvc-20c5ddec-5913-4b8d-a2fc-bfd0943b966d
```

#### 3) (Optional) Verify volume properties in EBS.

```
$ pv=$(k get -o json pvc ebs-claim | jq -r '.spec | .volumeName')
$ volumename=$(k get -o json pv $pv | jq -r '.spec | .csi | .volumeHandle')
$ aws ec2 describe-volumes —volume-id $volumename | jq '.Volumes[] | "\(.VolumeType) \(.Iops) \(.Throughput)"'
"gp3 3000 125"
```

#### 4) Edit the PVC with annotations.

```
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: ebs-claim
  annotations:
	ebs.csi.aws.com/volumeType: "io2"
	ebs.csi.aws.com/iops: "4000"
spec:
  accessModes:
    - ReadWriteOnce
  storageClassName: ebs-sc
  resources:
    requests:
      storage: 100Gi
```

#### 5) Verify the volume has been updated successfully.


```
$ k describe pvc ebs-claim
Name:          ebs-claim
Namespace:     default
StorageClass:  ebs-sc
Status:        Bound
Volume:        pvc-6bff47f9-1843-4576-ba15-158c73491e8c
Labels:        <none>
Annotations:   ebs.csi.aws.com/iops: 4000
               ebs.csi.aws.com/volumeType: io2
               pv.kubernetes.io/bind-completed: yes
               pv.kubernetes.io/bound-by-controller: yes
               volume.beta.kubernetes.io/storage-provisioner: ebs.csi.aws.com
               volume.kubernetes.io/selected-node: ip-192-168-88-208.us-east-2.compute.internal
               volume.kubernetes.io/storage-provisioner: ebs.csi.aws.com
Finalizers:    [kubernetes.io/pvc-protection]
Capacity:      100Gi
Access Modes:  RWO
VolumeMode:    Filesystem
Used By:       app
Events:
  Type     Reason                        Age                    From                                                                                      Message
  ----     ------                        ----                   ----                                                                                      -------
  Warning  ProvisioningFailed            7m51s                  persistentvolume-controller                                                               storageclass.storage.k8s.io "ebs-sc" not found
  Normal   WaitForPodScheduled           7m49s                  persistentvolume-controller                                                               waiting for pod app to be scheduled
  Normal   ExternalProvisioning          7m49s (x2 over 7m49s)  persistentvolume-controller                                                               waiting for a volume to be created, either by external provisioner "ebs.csi.aws.com" or manually created by system administrator
  Normal   Provisioning                  7m49s                  ebs.csi.aws.com_ebs-csi-controller-6bb6b754f7-kqpzm_27524e89-857b-4197-b801-e48576823e89  External provisioner is provisioning volume for claim "default/ebs-claim"
  Normal   ProvisioningSucceeded         7m46s                  ebs.csi.aws.com_ebs-csi-controller-6bb6b754f7-kqpzm_27524e89-857b-4197-b801-e48576823e89  Successfully provisioned volume pvc-6bff47f9-1843-4576-ba15-158c73491e8c
  Normal   VolumeModificationStarted     4s                     volume-modifier-for-k8s-ebs.csi.aws.com                                                   External modifier is modifying volume pvc-6bff47f9-1843-4576-ba15-158c73491e8c
  Normal   VolumeModificationSuccessful  1s                     volume-modifier-for-k8s-ebs.csi.aws.com                                                   External modifier has successfully modified volume pvc-6bff47f9-1843-4576-ba15-158c73491e8c
```

You will notice that the annotations have been applied to the PV as well.

```
$ k describe pv           
Name:              pvc-6bff47f9-1843-4576-ba15-158c73491e8c
Labels:            <none>
Annotations:       ebs.csi.aws.com/iops: 4000
                   ebs.csi.aws.com/volumeType: io2
                   pv.kubernetes.io/provisioned-by: ebs.csi.aws.com
                   volume.kubernetes.io/provisioner-deletion-secret-name: 
                   volume.kubernetes.io/provisioner-deletion-secret-namespace: 
Finalizers:        [kubernetes.io/pv-protection external-attacher/ebs-csi-aws-com]
StorageClass:      ebs-sc
Status:            Bound
Claim:             default/ebs-claim
Reclaim Policy:    Delete
Access Modes:      RWO
VolumeMode:        Filesystem
Capacity:          100Gi
Node Affinity:     
  Required Terms:  
    Term 0:        topology.ebs.csi.aws.com/zone in [us-east-2b]
Message:           
Source:
    Type:              CSI (a Container Storage Interface (CSI) volume source)
    Driver:            ebs.csi.aws.com
    FSType:            ext4
    VolumeHandle:      vol-02cb54d01c685b919
    ReadOnly:          false
    VolumeAttributes:      storage.kubernetes.io/csiProvisionerIdentity=1684161810754-8081-ebs.csi.aws.com
Events:                <none>
```

Do **NOT** delete these annotations. These annotations are used by the sidecar to reconcile the PVC's state when modifying annotations.

#### 6) (Optional) Validate the volume has been modified in EBS.
```
$ aws ec2 describe-volumes --volume-id $volumename | jq '.Volumes[] | "\(.VolumeType) \(.Iops) \(.Throughput)"'
"io2 4000 null"
```

