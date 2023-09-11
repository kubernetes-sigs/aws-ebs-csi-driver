# Additional Node DaemonSets Feature

In some situations, it is desirable to create multiple node `DaemonSet`s of the EBS CSI Driver. For example, when specifying `.node.volumeAttachLimit`, the limit may differ by node instance type or role.

The EBS CSI Driver Helm chart supports the creation of additional `DaemonSet`s via the `.additionalDaemonSets` parameter. Node configuration from the values supplied to `.node` are taken as a default, with the values supplied in the `.additionalDaemonSets` configuration as overrides. An additional Linux (and Windows, if enabled) `DaemonSet` will be rendered for each entry in `additionalDaemonSets`.

**WARNING: The EBS CSI Driver does not support running multiple node pods on the same node. If you use this feature, ensure that all nodes are targeted by no more than one `DaemonSet`s.**

## Example

For example, the following configuration would produce three `DaemonSet`s:

```yaml
node:
  nodeSelector:
    node.kubernetes.io/instance-type: c5.large
  volumeAttachLimit: 25
  resources:
    limits:
      memory: 512Mi
  
additionalNodeDaemonSets:
  big:
    nodeSelector:
      node.kubernetes.io/instance-type: m7i.48xlarge
    volumeAttachLimit: 100
  small:
    nodeSelector:
      node.kubernetes.io/instance-type: t3.medium
    volumeAttachLimit: 5
    resources:
      limits:
        memory: 128Mi
```

The `DaemonSet`s would be configured as follows:

- `ebs-csi-node` (the default `DaemonSet`)  
Runs on `c5.large` instances with a volume limit of 25 and 512Mi memory limit.
- `ebs-csi-node-big`  
Runs on `m7i.48xlarge` instances with a volume limit of 100 and 512Mi memory limit.  
Note how the volume limit is inherited from the `.node` configuration because this config does not specify them. This way, `.node` can be used to set defaults for all the `DaemonSet`s.
- `ebs-csi-node-small`  
Runs on `t3.medium` instances with a volume limit of 5 and 128Mi memory limit.
