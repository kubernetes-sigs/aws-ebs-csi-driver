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

# This script deploys the EBS CSI Driver and runs e2e tests
# CLUSTER_NAME and CLUSTER_TYPE are expected to be specified by the caller
# All other environment variables have default values (see config.sh) but
# many can be overridden on demand if needed

### Helper script for exporting EBS CSI Driver metrics to S3 bucket

set -euo pipefail

export_to_s3() {
  echo "Port-forwarding"
  controller_pod_name=$(kubectl get pod -n kube-system -l app=ebs-csi-controller -o jsonpath='{.items[0].metadata.name}')
  kubectl port-forward "$controller_pod_name" 3301:3301 -n kube-system &
  kubectl port-forward "$controller_pod_name" 8081:8081 -n kube-system &
  kubectl port-forward "$controller_pod_name" 8082:8082 -n kube-system &
  kubectl port-forward "$controller_pod_name" 8084:8084 -n kube-system &

  echo "Collecting metrics"
  for port in 3301 8081 8082 8084; do
    while true; do
      curl "http://localhost:${port}/metrics" >>"$EXPORT_DIR/metrics.txt" && break
      echo "Failed to collect metrics from port ${port}, retrying..."
      sleep 5
    done
  done

  echo "Collecting ebs-plugin logs"
  kubectl logs "$controller_pod_name" -n kube-system >"$EXPORT_DIR/ebs-plugin-logs.txt"

  echo "Collecting ebs-csi-controller Deployment and ebs-csi-node Daemonset yaml"
  kubectl get deployment ebs-csi-controller -n kube-system -o yaml >"$EXPORT_DIR/ebs-csi-controller.yaml"
  kubectl get daemonset ebs-csi-node -n kube-system -o yaml >"$EXPORT_DIR/ebs-csi-node.yaml"

  echo "Exporting everything in $EXPORT_DIR to S3"
  if ! aws s3 ls "s3://$S3_BUCKET"; then
    aws s3 mb "s3://$S3_BUCKET" --region "${AWS_REGION}"
  fi

  aws s3 sync "$EXPORT_DIR" "s3://$S3_BUCKET/$SCALABILITY_TEST_RUN_NAME"
  echo "Metrics exported to s3://$S3_BUCKET/$SCALABILITY_TEST_RUN_NAME/"
}

(return 0 2>/dev/null) || (
  echo "This script is not meant to be run directly, only sourced as a helper!"
  exit 1
)
