#!/bin/bash
# Copyright 2025 The Kubernetes Authors.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

### Helper script for exporting EBS CSI Driver metrics to S3 bucket

set -euo pipefail

collect-and-export-metrics() {
  export CONTROLLER_POD_NAME
  CONTROLLER_POD_NAME=$(kubectl get pod -n kube-system -l app=ebs-csi-controller -o jsonpath='{.items[0].metadata.name}')
  export METRICS_FILEPATH="$EXPORT_DIR/metrics.txt"

  collect_metrics
  clean_metrics

  echo "Collecting ebs-plugin logs"
  kubectl logs "$CONTROLLER_POD_NAME" -n kube-system >"$EXPORT_DIR/ebs-plugin-logs.txt"

  echo "Collecting ebs-csi-controller Deployment and ebs-csi-node Daemonset yaml"
  kubectl get deployment ebs-csi-controller -n kube-system -o yaml >"$EXPORT_DIR/ebs-csi-controller.yaml"
  kubectl get daemonset ebs-csi-node -n kube-system -o yaml >"$EXPORT_DIR/ebs-csi-node.yaml"

  echo "Exporting everything in $EXPORT_DIR to S3 bucket s3://$S3_BUCKET/$SCALABILITY_TEST_RUN_NAME"

  aws s3 sync "$EXPORT_DIR" "s3://$S3_BUCKET/$SCALABILITY_TEST_RUN_NAME"
  echo "Metrics exported to s3://$S3_BUCKET/$SCALABILITY_TEST_RUN_NAME/"
}

collect_metrics() {
  echo "Port-forwarding ebs-csi-controller containers"
  kubectl port-forward "$CONTROLLER_POD_NAME" 3301:3301 -n kube-system &
  PID_3301=$!
  kubectl port-forward "$CONTROLLER_POD_NAME" 8081:8081 -n kube-system &
  PID_8081=$!
  kubectl port-forward "$CONTROLLER_POD_NAME" 8082:8082 -n kube-system &
  PID_8082=$!
  kubectl port-forward "$CONTROLLER_POD_NAME" 8084:8084 -n kube-system &
  PID_8084=$!
  kubectl port-forward "$CONTROLLER_POD_NAME" 8085:8085 -n kube-system &
  PID_8085=$!

  echo "Collecting metrics"
  for port in 3301 8081 8082 8084 8085; do
    curl "http://localhost:${port}/metrics" >>"$METRICS_FILEPATH" && continue
    echo "Failed to collect metrics from port ${port}, retrying after 5s..."
    sleep 5
    curl "http://localhost:${port}/metrics" >>"$METRICS_FILEPATH" && continue
    echo "Failed to collect metrics from port ${port} AGAIN, retrying after 10s..."
    sleep 10
    curl "http://localhost:${port}/metrics" >>"$METRICS_FILEPATH" && continue
    echo "Failed to collect metrics from port ${port} THRICE, retrying one more time after 20s..."
    sleep 20
    echo "WARNING: Could not collect metrics from port ${port}. Something may be wrong in cluster."
  done
  # Stop forwarding ports after metrics collected.
  kill $PID_3301 $PID_8081 $PID_8082 $PID_8084 $PID_8085
}

clean_metrics() {
  echo "Generating clean version of exported data at $EXPORT_DIR/cleaned_data.txt"
  cat "$METRICS_FILEPATH" |
    grep -e "+Inf" -e "total" |
    grep -v "workqueue" |
    grep -v "go_" |
    grep -v "Identity" |
    grep -v "Capabili" |
    grep -v "TYPE" |
    grep -v "HELP" |
    grep -v "cloudprovider" |
    grep -v "promhttp" |
    grep -v "registered_metrics" >"$EXPORT_DIR/cleaned_data.txt"
}

(return 0 2>/dev/null) || (
  echo "This script is not meant to be run directly, only sourced as a helper!"
  exit 1
)
