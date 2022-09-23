# Driver Metrics

## Prerequisites

1. Install [Prometheus Operator](https://github.com/prometheus-operator/prometheus-operator) in your cluster:
```sh
$ helm repo add prometheus-community https://prometheus-community.github.io/helm-charts
$ helm repo update
$ helm install prometheus prometheus-community/kube-prometheus-stack
```
2. Enable metrics by setting `enableMetrics: true` in [values.yaml](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/blob/master/charts/aws-ebs-csi-driver/values.yaml).

3. Deploy EBS CSI Driver:
```sh
$ helm upgrade --install aws-ebs-csi-driver --namespace kube-system ./charts/aws-ebs-csi-driver --values ./charts/aws-ebs-csi-driver/values.yaml
```

## Overview

Installing the Prometheus Operator and enabling metrics will deploy a [Service](https://kubernetes.io/docs/concepts/services-networking/service/) object that exposes the EBS CSI Driver's controller metric port through a `ClusterIP`. Additionally, a [ServiceMonitor](https://github.com/prometheus-operator/prometheus-operator/blob/main/Documentation/user-guides/getting-started.md#:~:text=Alertmanager-,ServiceMonitor,-See%20the%20Alerting) object is deployed which updates the Prometheus scrape configuration and allows scraping metrics from the endpoint defined. For more information, see the manifest [metrics.yaml](/charts/aws-ebs-csi-driver/templates/metrics.yaml)

## AWS API Metrics

The EBS CSI Driver will emit [AWS API](https://docs.aws.amazon.com/AWSEC2/latest/APIReference/OperationList-query.html) metrics to the following TCP endpoint: `0.0.0.0:3301/metrics` if `enableMetrics: true` has been configured in the Helm chart.

The metrics will appear in the following format: 
```sh
# HELP cloudprovider_aws_api_request_duration_seconds [ALPHA] Latency of AWS API calls
# TYPE cloudprovider_aws_api_request_duration_seconds histogram
cloudprovider_aws_api_request_duration_seconds_bucket{request="AttachVolume",le="0.005"} 0
cloudprovider_aws_api_request_duration_seconds_bucket{request="AttachVolume",le="0.01"} 0
cloudprovider_aws_api_request_duration_seconds_bucket{request="AttachVolume",le="0.025"} 0
cloudprovider_aws_api_request_duration_seconds_bucket{request="AttachVolume",le="0.05"} 0
cloudprovider_aws_api_request_duration_seconds_bucket{request="AttachVolume",le="0.1"} 0
cloudprovider_aws_api_request_duration_seconds_bucket{request="AttachVolume",le="0.25"} 0
cloudprovider_aws_api_request_duration_seconds_bucket{request="AttachVolume",le="0.5"} 0
cloudprovider_aws_api_request_duration_seconds_bucket{request="AttachVolume",le="1"} 1
cloudprovider_aws_api_request_duration_seconds_bucket{request="AttachVolume",le="2.5"} 1
cloudprovider_aws_api_request_duration_seconds_bucket{request="AttachVolume",le="5"} 1
cloudprovider_aws_api_request_duration_seconds_bucket{request="AttachVolume",le="10"} 1
cloudprovider_aws_api_request_duration_seconds_bucket{request="AttachVolume",le="+Inf"} 1
cloudprovider_aws_api_request_duration_seconds_sum{request="AttachVolume"} 0.547694574
cloudprovider_aws_api_request_duration_seconds_count{request="AttachVolume"} 1
...
```

To manually scrape AWS metrics: 
```sh
$ export ebs_csi_controller=$(kubectl get lease -n kube-system ebs-csi-aws-com -o=jsonpath="{.spec.holderIdentity}")
$ kubectl port-forward $ebs_csi_controller 3301:3301 -n kube-system
$ curl 127.0.0.1:3301/metrics
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
