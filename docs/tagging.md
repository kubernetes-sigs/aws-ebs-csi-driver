# Tagging
To help manage volumes in the aws account, CSI driver will automatically add tags to the volumes it manages.

| TagKey                 | TagValue                  | sample                                                              | Description         |
|------------------------|---------------------------|---------------------------------------------------------------------|---------------------|
| CSIVolumeName          | pvcName                   | CSIVolumeName = pvc-a3ab0567-3a48-4608-8cb6-4e3b1485c808            | add to all volumes, for recording associated pvc id and checking if a given volume was already created so that ControllerPublish/CreateVolume is idempotent. |
| CSIVolumeSnapshotName  | volumeSnapshotContentName | CSIVolumeSnapshotName = snapcontent-69477690-803b-4d3e-a61a-03c7b2592a76 | add to all snapshots, for recording associated VolumeSnapshot id and checking if a given snapshot was already created                                    |
| ebs.csi.aws.com/cluster| true                      | ebs.csi.aws.com/cluster = true                                      | add to all volumes and snapshots, for allowing users to use a policy to limit csi driver's permission to just the resources it manages.                      |
| kubernetes.io/cluster/X| owned                     | kubernetes.io/cluster/aws-cluster-id-1 = owned                      | add to all volumes and snapshots if k8s-tag-cluster-id argument is set to X.|
| extra-key              | extra-value               | extra-key = extra-value                                             | add to all volumes and snapshots if extraTags argument is set|

The CSI driver also supports passing tags through `StorageClass.parameters`. For more information, please refer to the [tagging doc](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/blob/master/docs/TAGGING.md).
