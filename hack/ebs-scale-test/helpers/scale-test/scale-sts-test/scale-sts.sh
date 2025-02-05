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

### Helper script for running EBS-backed StatefulSet scaling test

# We expect this helper script is sourced from hack/ebs-scale-test
path_to_scale_test_dir="${BASE_DIR}/helpers/scale-test/scale-sts-test"

sts_scale_test() {
  manifest_path="$path_to_scale_test_dir/scale-sts.yaml"
  export_manifest_path="$EXPORT_DIR/scale-manifest.yaml"

  echo "Applying $manifest_path. Exported to $export_manifest_path"
  gomplate -f "$manifest_path" -o "$export_manifest_path"
  kubectl apply -f "$export_manifest_path"

  # Cleanup K8s resources upon script interruption
  trap 'echo "Test interrupted! Deleting test resources to prevent leak"; kubectl delete -f $export_manifest_path' EXIT

  echo "Scaling StatefulSet $REPLICAS replicas"
  kubectl scale sts --replicas "$REPLICAS" ebs-scale-test
  kubectl rollout status statefulset ebs-scale-test

  echo "Deleting StatefulSet"
  kubectl delete -f "$export_manifest_path"

  echo "Waiting for all PVs to be deleted"
  wait_for_pvs_to_delete

  trap - EXIT
}

wait_for_pvs_to_delete() {
  while true; do
    pv_count=$(kubectl get pv -o json | jq '.items | length')
    if [ "$pv_count" -eq 0 ]; then
      echo "No PVs exist in the cluster, proceeding..."
      break
    else
      echo "$pv_count PVs still exist, waiting..."
      sleep 5
    fi
  done
}

(return 0 2>/dev/null) || (
  echo "This script is not meant to be run directly, only sourced as a helper!"
  exit 1
)
