# Using EBS CSI Driver on AWS Outposts

## Overview

When using the EBS CSI Driver on AWS Outposts, there are several important considerations to be aware of.

### IMDS Metadata Access
The EBS CSI driver requires access to Instance Metadata Service (IMDS) to function properly on Outposts. Ensure that:
- IMDS is not disabled on your EC2 instances
- Ensure the hop limit is set to 2 

### Volume Type 
- You must explicitly specify a supported volume type (e.g., `gp2`). Check [AWS documentation](https://aws.amazon.com/outposts/rack/features/#topic-0) for the latest supported EBS volume types on your specific Outpost configuration.

### Topology Considerations

- You can use "WaitForFirstConsumer" which will ensure the volume will wait until a stateful pod is specified to 
provision the volume to allow topology aware provisioning without specifying the Outpost ARN. 

- Otherwise if you would like to specify the Outpost explicitly use `topology.ebs.csi.aws.com/outpost-id` as the topology key along with  `volumeBindingMode:` Immediate.

### StorageClass Examples
Example Outpost with `volumeBindingMode:` WaitForFirstConsumer

```yaml
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: outpostExampleOne
provisioner: ebs.csi.aws.com
volumeBindingMode: WaitForFirstConsumer
```

Example with `volumeBindingMode:` Immediate

```yaml
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: outpostExampleTwo
provisioner: ebs.csi.aws.com
parameters:
  type: gp2  # Specify volume type supported on your outpost
volumeBindingMode: Immediate
allowedTopologies:
- matchLabelExpressions:
  - key: topology.ebs.csi.aws.com/outpost-id
    values:
    -  "arn:aws:outposts:xxxx:xxxxx:outpost/op-xxxxx"  # Replace with your Outpost ARN
```

