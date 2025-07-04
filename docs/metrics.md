# Driver Metrics

## Prerequisites

1. Install [Prometheus Operator](https://github.com/prometheus-operator/prometheus-operator) in your cluster:
```sh
$ helm repo add prometheus-community https://prometheus-community.github.io/helm-charts
$ helm repo update
$ helm install prometheus prometheus-community/kube-prometheus-stack
```
2. Enable metrics by configuring `controller.enableMetrics` and `node.enableMetrics`.

3. Deploy EBS CSI Driver:
```sh
$ helm upgrade --install aws-ebs-csi-driver --namespace kube-system ./charts/aws-ebs-csi-driver --values ./charts/aws-ebs-csi-driver/values.yaml
```

## Overview

Installing the Prometheus Operator and enabling metrics will deploy a [Service](https://kubernetes.io/docs/concepts/services-networking/service/) object that exposes the EBS CSI Driver's controller metric port through a `ClusterIP`. Additionally, a [ServiceMonitor](https://github.com/prometheus-operator/prometheus-operator/blob/main/Documentation/user-guides/getting-started.md#:~:text=Alertmanager-,ServiceMonitor,-See%20the%20Alerting) object is deployed which updates the Prometheus scrape configuration and allows scraping metrics from the endpoint defined. For more information, see the manifest [servicemonitor.yaml](/charts/aws-ebs-csi-driver/templates/servicemonitor.yaml)

## AWS API Metrics

The EBS CSI Driver will emit [AWS API](https://docs.aws.amazon.com/AWSEC2/latest/APIReference/OperationList-query.html) metrics to the following TCP endpoint: `0.0.0.0:3301/metrics` if `controller.enableMetrics: true` has been configured in the Helm chart.

The following metrics are currently supported:

| Metric name | Metric type | Description | Labels                                                                                                                                                                     |
|-------------|-------------|-------------|----------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
|aws_ebs_csi_api_request_duration_seconds|Histogram|Duration by request type in seconds| request=\<AWS SDK API Request Type\> <br/> le=\<Time In Seconds\>                                                                                                          | 
|aws_ebs_csi_api_request_errors_total|Counter|Total number of errors by error code and request type| request=\<AWS SDK API Request Type\> <br/> error=\<Error Code\>                                                                                                            | 
|aws_ebs_csi_api_request_throttles_total|Counter|Total number of throttled requests per request type| request=\<AWS SDK API Request Type\>                                                                                                                                       |
|aws_ebs_csi_ec2_detach_pending_seconds|Counter|Number of seconds csi driver has been waiting for volume to be detached from instance| attachment_state=<Last observed attachment state\><br/>volume_id=<EBS Volume ID of associated volume\><br/>instance_id=<EC2 Instance ID associated with detaching volume\> |

The metrics will appear in the following format: 
```sh
aws_ebs_csi_api_request_duration_seconds_bucket{request="AttachVolume",le="0.005"} 0
aws_ebs_csi_api_request_duration_seconds_bucket{request="AttachVolume",le="0.01"} 0
aws_ebs_csi_api_request_duration_seconds_bucket{request="AttachVolume",le="0.025"} 0
aws_ebs_csi_api_request_duration_seconds_bucket{request="AttachVolume",le="0.05"} 0
aws_ebs_csi_api_request_duration_seconds_bucket{request="AttachVolume",le="0.1"} 0
aws_ebs_csi_api_request_duration_seconds_bucket{request="AttachVolume",le="0.25"} 0
aws_ebs_csi_api_request_duration_seconds_bucket{request="AttachVolume",le="0.5"} 0
aws_ebs_csi_api_request_duration_seconds_bucket{request="AttachVolume",le="1"} 1
aws_ebs_csi_api_request_duration_seconds_bucket{request="AttachVolume",le="2.5"} 1
aws_ebs_csi_api_request_duration_seconds_bucket{request="AttachVolume",le="5"} 1
aws_ebs_csi_api_request_duration_seconds_bucket{request="AttachVolume",le="10"} 1
aws_ebs_csi_api_request_duration_seconds_bucket{request="AttachVolume",le="+Inf"} 1
aws_ebs_csi_api_request_duration_seconds_sum{request="AttachVolume"} 0.547694574
aws_ebs_csi_api_request_duration_seconds_count{request="AttachVolume"} 1
...
```
By default, the driver deploys 2 replicas of the controller pod. However, each CSI sidecar (such as the attacher and resizer) uses a leader election mechanism to designate one leader pod per sidecar.

To manually scrape metrics for specific operations, you must identify and target the leader pod for the relevant sidecar. As an example, to manually scrape metrics for AttachVolume operations (handled by the external attacher), follow these steps:
```sh
$ export ebs_csi_attacher_leader=$(kubectl get lease external-attacher-leader-ebs-csi-aws-com -n kube-system -o=jsonpath='{.spec.holderIdentity}')
$ kubectl port-forward $ebs_csi_attacher_leader 3301:3301 -n kube-system &
$ curl 127.0.0.1:3301/metrics
```

## EBS Node Metrics

The EBS CSI Driver will emit [container storage Interface managed devices metrics](https://docs.aws.amazon.com/ebs/latest/userguide/nvme-detailed-performance-stats.html) to the following TCP endpoint: `0.0.0.0:3302/metrics` if `node.enableMetrics: true` has been configured in the Helm chart.

The metrics will appear in the following format: 
```sh
aws_ebs_csi_nvme_collector_duration_seconds_bucket{instance_id="{instance-id}",le="0.001"} 0
aws_ebs_csi_nvme_collector_duration_seconds_bucket{instance_id="{instance-id}",le="0.0025"} 0
aws_ebs_csi_nvme_collector_duration_seconds_bucket{instance_id="{instance-id}",le="0.005"} 1
aws_ebs_csi_nvme_collector_duration_seconds_bucket{instance_id="{instance-id}",le="0.01"} 1
aws_ebs_csi_nvme_collector_duration_seconds_bucket{instance_id="{instance-id}",le="0.025"} 1
aws_ebs_csi_nvme_collector_duration_seconds_bucket{instance_id="instance-id}",le="0.05"} 1
aws_ebs_csi_nvme_collector_duration_seconds_bucket{instance_id="{instance-id}",le="0.1"} 1
aws_ebs_csi_nvme_collector_duration_seconds_bucket{instance_id="{instance-id}",le="0.25"} 1
aws_ebs_csi_nvme_collector_duration_seconds_bucket{instance_id="{instance-id}",le="0.5"} 1
aws_ebs_csi_nvme_collector_duration_seconds_bucket{instance_id="{instance-id}",le="1"} 1
aws_ebs_csi_nvme_collector_duration_seconds_bucket{instance_id="instance-id}",le="2.5"} 1
aws_ebs_csi_nvme_collector_duration_seconds_bucket{instance_id="instance-id}",le="5"} 1
aws_ebs_csi_nvme_collector_duration_seconds_bucket{instance_id="{instance-id}",le="10"} 1
aws_ebs_csi_nvme_collector_duration_seconds_bucket{instance_id="{instance-id}",le="+Inf"} 1
...
```

To manually scrape AWS metrics: 
```sh
$ kubectl port-forward $ebs_csi_node_pod_name 3302:3302 -n kube-system
$ curl 127.0.0.1:3302/metrics
```

## Volume Stats Metrics

The EBS CSI Driver emits Kubelet mounted volume metrics for volumes created with the driver. 

The following metrics are currently supported:

| Metric name | Metric type | Description | Labels |
|-------------|-------------|-------------|-------------|
|kubelet_volume_stats_capacity_bytes|Gauge|The capacity in bytes of the volume|namespace=\<persistentvolumeclaim-namespace\> <br/> persistentvolumeclaim=\<persistentvolumeclaim-name\>| 
|kubelet_volume_stats_available_bytes|Gauge|The number of available bytes in the volume|namespace=\<persistentvolumeclaim-namespace\> <br/> persistentvolumeclaim=\<persistentvolumeclaim-name\>| 
|kubelet_volume_stats_used_bytes|Gauge|The number of used bytes in the volume|namespace=\<persistentvolumeclaim-namespace\> <br/> persistentvolumeclaim=\<persistentvolumeclaim-name\>| 
|kubelet_volume_stats_inodes|Gauge|The maximum number of inodes in the volume|namespace=\<persistentvolumeclaim-namespace\> <br/> persistentvolumeclaim=\<persistentvolumeclaim-name\>| 
|kubelet_volume_stats_inodes_free|Gauge|The number of free inodes in the volume|namespace=\<persistentvolumeclaim-namespace\> <br/> persistentvolumeclaim=\<persistentvolumeclaim-name\>| 
|kubelet_volume_stats_inodes_used|Gauge|The number of used inodes in the volume|namespace=\<persistentvolumeclaim-namespace\> <br/> persistentvolumeclaim=\<persistentvolumeclaim-name\>| 

For more information about the supported metrics, see `VolumeUsage` within the CSI spec documentation for the [NodeGetVolumeStats](https://github.com/container-storage-interface/spec/blob/master/spec.md#nodegetvolumestats) RPC call.

For more information about metrics in Kubernetes, see the [Metrics For Kubernetes System Components](https://kubernetes.io/docs/concepts/cluster-administration/system-metrics/#metrics-in-kubernetes) documentation.

## CSI Operations Metrics

The `csi_operations_seconds metrics` reports a latency histogram of kubelet-initiated CSI gRPC calls by gRPC status code.

To manually scrape Kubelet metrics: 
```sh
$ kubectl proxy
$ kubectl get --raw /api/v1/nodes/<insert_node_name>/proxy/metrics
```
