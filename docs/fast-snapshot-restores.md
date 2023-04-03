# Fast Snapshot Restores

The EBS CSI Driver provides support for [Fast Snapshot Restores(FSR)](https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/ebs-fast-snapshot-restore.html) via `VolumeSnapshotClass.parameters.fastSnapshotRestoreAvailabilityZones`.

Amazon EBS fast snapshot restore (FSR) enables you to create a volume from a snapshot that is fully initialized at creation. This eliminates the latency of I/O operations on a block when it is accessed for the first time. Volumes that are created using fast snapshot restore instantly deliver all of their provisioned performance.

Availability zones are specified as a comma separated list.

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

## Prerequisites

- Install the [Kubernetes Volume Snapshot CRDs](https://github.com/kubernetes-csi/external-snapshotter/tree/master/client/config/crd) and external-snapshotter sidecar. For installation instructions, see [CSI Snapshotter Usage](https://github.com/kubernetes-csi/external-snapshotter#usage).

- The EBS CSI Driver must be given permission to access the [`EnableFastSnapshotRestores` EC2 API](https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_EnableFastSnapshotRestores.html). This example snippet can be used in an IAM policy to grant access to `EnableFastSnapshotRestores`:

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
