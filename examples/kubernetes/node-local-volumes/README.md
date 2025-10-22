# Node-Local Volumes Example

This example demonstrates how to use node-local volumes with the EBS CSI Driver.

## Prerequisites

1. Enable the feature in the driver:
   ```bash
   helm upgrade --install aws-ebs-csi-driver \
     --namespace kube-system \
     ./charts/aws-ebs-csi-driver \
     --set controller.enableNodeLocalVolumes=true
   ```

2. Pre-attach EBS volumes to each node using an EC2 Launch Template:
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
   Apply this launch template to your node group or managed node group.

## Deploy

```bash
kubectl apply -f manifests/
```

## Verify

```bash
# Check pods are running
kubectl get pods -l app=cache-app -o wide
```

## Cleanup

```bash
kubectl delete -f manifests/
```
