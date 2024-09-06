## Supported Parameters
| Parameter                          | Description of value                                      |
|------------------------------------|-----------------------------------------------------------|
| fastSnapshotRestoreAvailabilityZones | Comma separated list of availability zones                |
| outpostArn                         | Arn of the outpost you wish to have the snapshot saved to | 

The AWS EBS CSI Driver supports [tagging](tagging.md) through `VolumeSnapshotClass.parameters` (in v1.6.0 and later). 
## Prerequisites

- Install the [Kubernetes Volume Snapshot CRDs](https://github.com/kubernetes-csi/external-snapshotter/tree/master/client/config/crd) and external-snapshotter sidecar. For installation instructions, see [CSI Snapshotter Usage](https://github.com/kubernetes-csi/external-snapshotter#usage).

# Fast Snapshot Restores

The EBS CSI Driver provides support for [Fast Snapshot Restores(FSR)](https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/ebs-fast-snapshot-restore.html) via `VolumeSnapshotClass.parameters.fastSnapshotRestoreAvailabilityZones`.

Amazon EBS fast snapshot restore (FSR) enables you to create a volume from a snapshot that is fully initialized at creation. This eliminates the latency of I/O operations on a block when it is accessed for the first time. Volumes that are created using fast snapshot restore instantly deliver all of their provisioned performance.

Availability zones are specified as a comma separated list.

- The EBS CSI Driver must be given permission to access the [`EnableFastSnapshotRestores` EC2 API](https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_EnableFastSnapshotRestores.html). This example snippet can be used in an IAM policy to grant access to `EnableFastSnapshotRestores`:

**Example**
```
apiVersion: snapshot.storage.k8s.io/v1
kind: VolumeSnapshotClass
metadata:
  name: csi-aws-vsc
driver: ebs.csi.aws.com
deletionPolicy: Delete
parameters:
  fastSnapshotRestoreAvailabilityZones: "us-east-1a, us-east-1b"
```

```json
{
  "Effect": "Allow",
  "Action": [
    "ec2:EnableFastSnapshotRestores"
  ],
  "Resource": "*"
}
```

## Failure Mode 

The driver will attempt to check if the availability zones provided are supported for fast snapshot restore before attempting to create the snapshot. If the `EnableFastSnapshotRestores` API call fails, the driver will hard-fail the request and delete the snapshot. This is to ensure that the snapshot is not left in an inconsistent state.


# Amazon EBS Local Snapshots on Outposts

The EBS CSI Driver provides support for [Amazon EBS local snapshots on Outposts](https://docs.aws.amazon.com/ebs/latest/userguide/snapshots-outposts.html) via `VolumeSnapshotClass.parameters.outpostArn`.

By default, snapshots of EBS volumes on an Outpost are stored in Amazon S3 in the Region of the Outpost. You can also use Amazon EBS local snapshots on Outposts to store snapshots of volumes on an Outpost locally in Amazon S3 on the Outpost itself. This ensures that the snapshot data resides on the Outpost, and on your premises. In addition, you can use AWS Identity and Access Management (IAM) policies and permissions to set up data residency enforcement policies to ensue that snapshot data does not leave the Outpost. This is especially useful if you reside in a country or region that is not yet served by an AWS Region and that has data residency requirements.


**Example**
```
apiVersion: snapshot.storage.k8s.io/v1
kind: VolumeSnapshotClass
metadata:
  name: csi-aws-vsc
driver: ebs.csi.aws.com
deletionPolicy: Delete
parameters:
  outpostarn: {arn of your outpost}
```
