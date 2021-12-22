# Configuring StorageClass
This example shows how to configure Kubernetes storageclass to provision EBS volumes with various configuration parameters. EBS CSI driver is compatiable with in-tree EBS plugin on StorageClass parameters. For the full list of in-tree EBS plugin parameters, please refer to Kubernetes documentation of [StorageClass Parameter](https://kubernetes.io/docs/concepts/storage/storage-classes/#aws-ebs).

## Usage
1. Edit the StorageClass spec in [example manifest](./specs/example.yaml) and update storageclass parameters to desired value. In this example, a `io1` EBS volume will be created and formatted to `xfs` filesystem with encryption enabled using the default KMS key.

2. Deploy the example:
```sh
kubectl apply -f specs/
```

3. Verify the volume is created:
```sh
kubectl describe pv
```

4. Validate the pod successfully wrote data to the volume:
```sh
kubectl exec -it app -- cat /data/out.txt
```

5. Cleanup resources:
```sh
kubectl delete -f specs/
```
