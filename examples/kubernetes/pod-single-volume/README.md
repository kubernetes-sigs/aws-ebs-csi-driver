# Dynamic Volume Provisioning with AWS EBS CSI Driver

## Prerequisites

1. Kubernetes 1.13+ (CSI 1.0).

1. The [aws-ebs-csi-driver driver](https://github.com/kubernetes-sigs/aws-ebs-csi-driver) is installed.

## Usage

1. Create a sample app along with the StorageClass and the PersistentVolumeClaim:
```
kubectl apply -f specs/
```

2. Validate the volume was created and `volumeHandle` contains an EBS volumeID:
```
kubectl describe pv
```

3. Validate the pod successfully wrote data to the volume:
```
kubectl exec -it app cat /data/out.txt
```

4. Cleanup resources:
```
kubectl delete -f specs/
```