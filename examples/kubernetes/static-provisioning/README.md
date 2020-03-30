# Static Provisioning 
This example shows how to create and consume persistence volume from exising EBS using static provisioning. 

## Usage
1. Edit the PersistentVolume spec in [example manifest](./specs/example.yaml). Update `volumeHandle` with EBS volume ID that you are going to use, and update the `fsType` with the filesystem type of the volume. In this example, I have a pre-created EBS  volume in us-east-1c availability zone and it is formatted with xfs filesystem.

```
apiVersion: v1
kind: PersistentVolume
metadata:
  name: test-pv
spec:
  capacity:
    storage: 50Gi
  volumeMode: Filesystem
  accessModes:
    - ReadWriteOnce
  storageClassName: ebs-sc
  csi:
    driver: ebs.csi.aws.com
    volumeHandle: {volumeId} 
    fsType: xfs
  nodeAffinity:
    required:
      nodeSelectorTerms:
      - matchExpressions:
        - key: topology.ebs.csi.aws.com/zone
          operator: In
          values:
          - us-east-1c 
```
Note that node affinity is used here since EBS volume is created in us-east-1c, hence only node in the same AZ can consume this persisence volume. 

2. Deploy the example:
```sh
kubectl apply -f specs/
```

3. Verify application pod is running:
```sh
kubectl describe po app
```

4. Validate the pod successfully wrote data to the volume:
```sh
kubectl exec -it app cat /data/out.txt
```

5. Cleanup resources:
```sh
kubectl delete -f specs/
```
