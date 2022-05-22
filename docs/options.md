# Driver Options
There are couple driver options that can be passed as arguments when starting driver container.

| Option argument             | value sample                                      | default                                             | Description         |
|-----------------------------|---------------------------------------------------|-----------------------------------------------------|---------------------|
| endpoint                    | tcp://127.0.0.1:10000/                            | unix:///var/lib/csi/sockets/pluginproxy/csi.sock    | added to all volumes, for checking if a given volume was already created so that ControllerPublish/CreateVolume is idempotent. |
| volume-attach-limit         | 1,2,3 ...                                         | -1                                                  | Value for the maximum number of volumes attachable per node. If specified, the limit applies to all nodes. If not specified, the value is approximated from the instance type.    |
| extra-tags                  | key1=value1,key2=value2                           |                                                     | Extra tags to attach to each dynamically provisioned resource.|
| k8s-tag-cluster-id          | aws-cluster-id-1                                  |                                                     | ID of the Kubernetes cluster used for tagging provisioned EBS volumes.|
| aws-sdk-debug-log           | true                                              | false                                               | if true, driver will enable the aws sdk debug log level|
