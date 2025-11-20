## Supported Parameters
| Parameter                          | Description of value                                      |
|------------------------------------|-----------------------------------------------------------|
| fastSnapshotRestoreAvailabilityZones | Comma separated list of availability zones                |
| outpostArn                         | Arn of the outpost you wish to have the snapshot saved to |
| lockMode                   | Lock mode (governance/compliance)                         |
| lockDuration               | Lock duration in days                                     |
| lockExpirationDate         | Lock expiration date (RFC3339 format)                    |
| lockCoolOffPeriod          | Cool-off period in hours (compliance mode only)          | 

The AWS EBS CSI Driver supports [tagging](tagging.md) through `VolumeSnapshotClass.parameters` (in v1.6.0 and later). 
## Prerequisites

- Install the [Kubernetes Volume Snapshot CRDs](https://github.com/kubernetes-csi/external-snapshotter/tree/master/client/config/crd) and external-snapshotter sidecar. For installation instructions, see [CSI Snapshotter Usage](https://github.com/kubernetes-csi/external-snapshotter#usage).

# Fast Snapshot Restores

The EBS CSI Driver provides support for [Fast Snapshot Restores(FSR)](https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/ebs-fast-snapshot-restore.html) via `VolumeSnapshotClass.parameters.fastSnapshotRestoreAvailabilityZones`.


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

# Snapshot Lock

The EBS CSI Driver supports [EBS Snapshot Lock](https://docs.aws.amazon.com/ebs/latest/userguide/ebs-snapshot-lock.html) via `VolumeSnapshotClass.parameters`. Snapshot locking protects snapshots from accidental or malicious deletion. A locked snapshot can't be deleted.

**Example - Lock in Governance Mode with Specified Duration**
```yaml
apiVersion: snapshot.storage.k8s.io/v1
kind: VolumeSnapshotClass
metadata:
  name: csi-aws-vsc-locked
driver: ebs.csi.aws.com
deletionPolicy: Delete
parameters:
  lockMode: "governance"
  lockDuration: "7"
```

**Example - Lock in Compliance Mode with Expiration Date and Cool Off Period**
```yaml
apiVersion: snapshot.storage.k8s.io/v1
kind: VolumeSnapshotClass
metadata:
  name: csi-aws-vsc-compliance
driver: ebs.csi.aws.com
deletionPolicy: Delete
parameters:
  lockMode: "compliance"
  lockExpirationDate: "2030-12-31T23:59:59Z"
  lockCoolOffPeriod: "24"
```

## Failure Mode

If the `LockSnapshot` API call fails, the driver will hard-fail the request and delete the snapshot. This ensures that the snapshot is not left in an unlocked state when locking was explicitly requested.


# Amazon EBS Local Snapshots on Outposts

The EBS CSI Driver provides support for [Amazon EBS local snapshots on Outposts](https://docs.aws.amazon.com/ebs/latest/userguide/snapshots-outposts.html) via `VolumeSnapshotClass.parameters.outpostArn`.



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
