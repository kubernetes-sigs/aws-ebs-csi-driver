# Cluster Debugging Scripts

This folder contains a collection of scripts to help debug clusters.

## FAQ

### How can I validate that the aws-ebs-csi-driver correctly makes use of all available attachment slots for instance type X? 

Answer: Perform the following steps (Which will create a nodegroup, count the Block Device Mappings + ENI for the underlying instance, deploy pods EBS PVs until the script finds the maximum amount of volumes the aws-ebs-csi-driver can attach to instance)

```
export CLUSTER_NAME="devcluster"
export NODEGROUP_NAME="ng-attachment-limit-test"
export INSTANCE_TYPE="m7g.large"

eksctl create nodegroup -c "$CLUSTER_NAME" --nodes 1 -t "$INSTANCE_TYPE" -n "$NODEGROUP_NAME"

./get-attachment-breakdown "$INSTANCE_TYPE" 

eksctl delete nodegroup -c "$CLUSTER_NAME" -n "$INSTANCE_TYPE"
```

By the end of the script, you should see an output similar to this:
```
Attachments for ng-f3ecdf71
BlockDeviceMappings  ENIs  Available-Attachment-Slots(Validated-by-aws-ebs-csi-driver)
1                    2     25
```

## Scripts

get-attachment-breakdown: Find the maximum amount of volumes that can be attached to a specified nodegroup node. Additionally, log how many BlockDeviceMappings and ENIs are attached the instance of the specified nodegroup. 

Examples
```
./get_attachment_breakdown "ng-f3ecdf71"
MIN_VOLUME_GUESS=20 MAX_VOLUME_GUESS=40 POD_TIMEOUT_SECONDS=120 ./get_attachment_breakdown "ng-f3ecdf71"
```

find-attachment-limit: Find the maximum amount of volumes the aws-ebs-csi-driver can attach to a node. 

Examples:
```
./find-attachment-limit 'some.node.affinity.key:value'
./find-attachment-limit 'eks.amazonaws.com/nodegroup:test-nodegroup'
./find-attachment-limit 'node.kubernetes.io/instance-type:m5.large'
MIN_VOLUME_GUESS=12 MAX_VOLUME_GUESS=30 POD_TIMEOUT_SECONDS=60 ./find_attachment_limit 'node.kubernetes.io/instance-type:m5.large' 
```

generate_example_manifest.go: Generate a yaml file containing a pod associated with a specified amount of PVCs based off of the template file `device_slot_test.tmpl`

Example:
```
go run "generate_example_manifest.go" --node-affinity "some.label:value" --volume-count "22" --test-pod-name "test-pod"
```
