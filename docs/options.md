# Driver Options
There are a couple of driver options that can be passed as arguments when starting the driver container.

| Option argument             | value sample                                      | default                                             | Description         |
|-----------------------------|---------------------------------------------------|-----------------------------------------------------|---------------------|
| endpoint                    | tcp://127.0.0.1:10000/                            | unix:///var/lib/csi/sockets/pluginproxy/csi.sock    | The socket on which the driver will listen for CSI RPCs|
| volume-attach-limit         | 1,2,3 ...                                         | -1                                                  | Value for the maximum number of volumes attachable per node. If specified, the limit applies to all nodes. If not specified, the value is approximated from the instance type|
| extra-tags                  | key1=value1,key2=value2                           |                                                     | Tags attached to each dynamically provisioned resource|
| k8s-tag-cluster-id          | aws-cluster-id-1                                  |                                                     | ID of the Kubernetes cluster used for tagging provisioned EBS volumes|
| aws-sdk-debug-log           | true                                              | false                                               | If set to true, the driver will enable the aws sdk debug log level|
| logging-format              | json                                              | text                                                | Sets the log format. Permitted formats: text, json|
| user-agent-extra            | csi-ebs                                           | helm                                                | Extra string appended to user agent|
| batching                    | true                                              | false                                                | If set to true, the driver will enable batching of API calls. This is especially helpful for improving performance in workloads that are sensitive to EC2 rate limits|
