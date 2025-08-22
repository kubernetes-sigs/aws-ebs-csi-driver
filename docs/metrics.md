# Driver Metrics

## Overview

The EBS CSI Driver supports emitting metrics via an HTTP endpoint from both its controller and node pods in the standard [Prometheus exposition format](https://prometheus.io/docs/instrumenting/exposition_formats/). Most metrics systems support ingesting the Prometheus format, including [Prometheus](https://prometheus.io/), [CloudWatch](https://docs.aws.amazon.com/AmazonCloudWatch/latest/monitoring/ContainerInsights-Prometheus-metrics.html), [InfluxDB](https://docs.influxdata.com/influxdb/v1/supported_protocols/prometheus/), and many others.

When installing via Helm, the metrics can be configured via Helm parameters:
- The metrics may be enabled by setting `controller.enableMetrics` and/or `node.enableMetrics` to `true`.
  - This will deploy a `Service` object for each metrics-supporting container in the respective pod.
  - `Service` objects come with the annotations `prometheus.io/scrape` and `prometheus.io/port` by default. This behavior can be controlled via the parameters `controller.enablePrometheusAnnotations` and `node.enablePrometheusAnnotations`.
- If the [Prometheus Operator](https://github.com/prometheus-operator/prometheus-operator) CRDs are detected as installed, a `ServiceMonitor` object will be deployed for each `Service`.
  - The `ServiceMonitor` can be configured via `controller.serviceMonitor` and `node.serviceMonitor`.
  - If deploying in an environment where the CRDs cannot be detected, `controller.serviceMonitor.forceEnable` and `node.serviceMonitor.forceEnable` will forcefully render the `ServiceMonitor`.

## AWS API Metrics (`ebs-csi-controller`)

The EBS CSI Driver will emit [AWS API](https://docs.aws.amazon.com/AWSEC2/latest/APIReference/OperationList-query.html) metrics to the following TCP endpoint: `0.0.0.0:3301/metrics` if `controller.enableMetrics: true` has been configured in the Helm chart.

The following metrics are currently supported:

| Metric name | Metric type | Description | Labels                                                                                                                                                                     |
|-------------|-------------|-------------|----------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
|aws_ebs_csi_api_request_duration_seconds|Histogram|Duration by request type in seconds| request=\<AWS SDK API Request Type\> <br/> le=\<Time In Seconds\>                                                                                                          | 
|aws_ebs_csi_api_request_errors_total|Counter|Total number of errors by error code and request type| request=\<AWS SDK API Request Type\> <br/> error=\<Error Code\>                                                                                                            | 
|aws_ebs_csi_api_request_throttles_total|Counter|Total number of throttled requests per request type| request=\<AWS SDK API Request Type\>                                                                                                                                       |
|aws_ebs_csi_ec2_detach_pending_seconds|Counter|Number of seconds csi driver has been waiting for volume to be detached from instance| attachment_state=<Last observed attachment state\><br/>volume_id=<EBS Volume ID of associated volume\><br/>instance_id=<EC2 Instance ID associated with detaching volume\> |

## CSI Sidecar Metrics (`ebs-csi-controller`)

When controller metrics are enabled, metrics are also automatically enabled for the [CSI Sidecars](https://kubernetes-csi.github.io/docs/sidecar-containers.html) present in the controller deployment. The CSI Sidecars record metrics about the number of errors and duration of CSI RPC calls via the [`csi-lib-utils` library](https://github.com/kubernetes-csi/csi-lib-utils/blob/master/metrics/metrics.go).

## EBS NVMe Metrics (`ebs-csi-node`)

The EBS CSI Driver will emit data from the [EBS detailed performance stats](https://docs.aws.amazon.com/ebs/latest/userguide/nvme-detailed-performance-stats.html) for EBS CSI managed volumes. All NVMe metrics (except the `nvme_collector` metrics which have no labels) support the `instance_id` and `volume_id` labels.

The following metrics are currently supported:

| Metric name | Metric type | Description |
|-------------|-------------|-------------|
|aws_ebs_csi_read_ops_total|Counter|The total number of completed read operations|
|aws_ebs_csi_write_ops_total|Counter|The total number of completed write operations|
|aws_ebs_csi_read_bytes_total|Counter|The total number of read bytes transferred|
|aws_ebs_csi_write_bytes_total|Counter|The total number of write bytes transferred|
|aws_ebs_csi_read_seconds_total|Counter|The total time spent, in seconds, by all completed read operations|
|aws_ebs_csi_write_seconds_total|Counter|The total time spent, in seconds, by all completed write operations|
|aws_ebs_csi_exceeded_iops_seconds_total|Counter|The total time, in seconds, that IOPS demand exceeded the volume's provisioned IOPS performance|
|aws_ebs_csi_exceeded_tp_seconds_total|Counter|The total time, in seconds, that throughput demand exceeded the volume's provisioned throughput performance|
|aws_ebs_csi_ec2_exceeded_iops_seconds_total|Counter|The total time, in seconds, that the EBS volume exceeded the attached Amazon EC2 instance's maximum IOPS performance|
|aws_ebs_csi_ec2_exceeded_tp_seconds_total|Counter|The total time, in seconds, that the EBS volume exceeded the attached Amazon EC2 instance's maximum throughput performance|
|aws_ebs_csi_nvme_collector_scrapes_total|Counter|Total number of NVMe collector scrapes|
|aws_ebs_csi_nvme_collector_errors_total|Counter|Total number of NVMe collector scrape errors|
|aws_ebs_csi_volume_queue_length|Gauge|The number of read and write operations waiting to be completed|
|aws_ebs_csi_read_io_latency_seconds|Histogram|The number of read operations completed within each latency bin, in seconds|
|aws_ebs_csi_write_io_latency_seconds|Histogram|The number of write operations completed within each latency bin, in seconds|
|aws_ebs_csi_nvme_collector_duration_seconds|Histogram|NVMe collector scrape duration in seconds|


## Volume Stats Metrics (`kubelet`)

The EBS CSI Driver implements the CSI [NodeGetVolumeStats](https://github.com/container-storage-interface/spec/blob/master/spec.md#nodegetvolumestats) RPC, which allows the `kubelet` to collect information about volumes attached to running pods. Note that the EBS CSI Driver Helm Chart does not deploy monitoring configuration for the `kubelet` - see the documentation of your monitoring system for information of how to configure collection of `kubelet` metrics.

If metrics support is enabled on the `kubelet`, the EBS CSI Driver provides the information to vend:
- `kubelet_volume_stats_capacity_bytes`
- `kubelet_volume_stats_available_bytes`
- `kubelet_volume_stats_used_bytes`
- `kubelet_volume_stats_inodes`
- `kubelet_volume_stats_inodes_free`
- `kubelet_volume_stats_inodes_used`
- `csi_operations_seconds`

For more details, see the [Metrics For Kubernetes System Components](https://kubernetes.io/docs/concepts/cluster-administration/system-metrics/#metrics-in-kubernetes) documentation.