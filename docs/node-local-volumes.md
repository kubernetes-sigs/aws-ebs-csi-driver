# Node-Local Volumes

## Overview

Node-local volumes enable a single cluster-wide PersistentVolume (PV) and PersistentVolumeClaim (PVC) to mount pre-attached, node-specific EBS volumes. When pods reference this PVC, each node independently mounts its own local EBS device, and all pods on that node share the mount.

This feature is useful for scenarios where:
- Multiple co-located pods need access to a shared dataset (e.g., cached files from S3).
- You want to avoid using `hostPath` volumes for security reasons.
- You need to scale workloads across nodes while maintaining node-local caching.

## Prerequisites

- EBS volumes must be pre-attached to each node at a consistent device name (e.g., `/dev/xvdbz`).
- The driver must be deployed with `controller.enableNodeLocalVolumes=true`.

## Enabling the Feature

### Helm Installation

```bash
helm upgrade --install aws-ebs-csi-driver \
  --namespace kube-system \
  ./charts/aws-ebs-csi-driver \
  --set controller.enableNodeLocalVolumes=true
```

## Usage

### 1. Pre-attach EBS Volumes to Nodes

Each node must have an EBS volume attached at the same device name. For example:

**EC2 Launch Template:**
```json
{
  "BlockDeviceMappings": [{
    "DeviceName": "/dev/xvdbz",
    "Ebs": {
      "VolumeSize": 100,
      "VolumeType": "gp3",
      "DeleteOnTermination": true
    }
  }]
}
```

### 2. Create cluster-wide PV and PVC

Create a PersistentVolume with `volumeHandle` using the `local-ebs://` prefix:

```yaml
apiVersion: v1
kind: PersistentVolume
metadata:
  name: node-local-cache-pv
spec:
  capacity:
    storage: 100Gi
  volumeMode: Filesystem
  accessModes:
    - ReadWriteMany
  persistentVolumeReclaimPolicy: Retain
  csi:
    driver: ebs.csi.aws.com
    volumeHandle: local-ebs://dev/xvdbz
    volumeAttributes:
      ebs.csi.aws.com/fsType: "xfs"
---
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: node-local-cache-pvc
spec:
  accessModes:
    - ReadWriteMany
  resources:
    requests:
      storage: 100Gi
  volumeName: node-local-cache-pv
```

**Important:** The `volumeHandle` format is `local-ebs://<device-name>` where `<device-name>` is the device path without the leading slash (e.g., `dev/xvdbz` for `/dev/xvdbz`).

### 3. Use in Workloads

Reference the PVC in your pod or deployment:

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: my-app
spec:
  replicas: 10
  selector:
    matchLabels:
      app: my-app
  template:
    metadata:
      labels:
        app: my-app
    spec:
      containers:
      - name: app
        image: my-app:latest
        volumeMounts:
        - name: cache
          mountPath: /cache
      volumes:
      - name: cache
        persistentVolumeClaim:
          claimName: node-local-cache-pvc
```

## Access Mode Requirements

Node-local volumes may use `ReadWriteMany` (RWX) access mode. This tells the Kubernetes scheduler it's safe to place pods on multiple nodes. Each node uses its own physical EBS volume, so there's no actual cross-node sharing.

## Limitations

- Volumes must be statically provisioned and pre-attached at the specified device path.
- Cross-node data sharing is not supported (each node has its own volume).
- Volume snapshots and modifications for local cache volumes are not supported.
- The root device cannot be used as a node-local volume.

## Examples

See [examples/kubernetes/node-local-volumes](../examples/kubernetes/node-local-volumes) for complete working examples.
