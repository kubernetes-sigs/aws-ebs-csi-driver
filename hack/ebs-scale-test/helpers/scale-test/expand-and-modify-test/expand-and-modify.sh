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

### Helper script for running EBS-backed PVC expansion and modification test

# We expect this helper script is sourced from hack/ebs-scale-test
path_to_resize_and_modify_test_dir="${BASE_DIR}/helpers/scale-test/expand-and-modify-test"

export EXPAND_ONLY=${EXPAND_ONLY:="false"}
export MODIFY_ONLY=${MODIFY_ONLY:="false"}

expand_and_modify_test() {
  manifest_path="$path_to_resize_and_modify_test_dir/expand-and-modify.yaml"
  export_manifest_path="$EXPORT_DIR/expand-and-modify.yaml"

  echo "Applying $manifest_path. Exported to $export_manifest_path"
  gomplate -f "$manifest_path" -o "$export_manifest_path"
  kubectl apply -f "$export_manifest_path"

  # Cleanup K8s resources upon script interruption
  trap 'echo "Test interrupted! Deleting test resources to prevent leak"; kubectl delete -f $export_manifest_path' EXIT

  echo "Waiting for all PVCs to be dynamically provisioned"
  wait_for_pvcs_to_bind

  case "$EXPAND_ONLY-$MODIFY_ONLY" in
  *false-false*) patches='[{"op": "replace", "path": "/spec/volumeAttributesClassName", "value": "ebs-scale-test-expand-and-modify"},{"op": "replace", "path": "/spec/resources/requests/storage", "value": "2Gi"}]' ;;
  *false-true*) patches='[{"op": "replace", "path": "/spec/volumeAttributesClassName", "value": "ebs-scale-test-expand-and-modify"}]' ;;
  *true-false* | *delete*) patches='[{"op": "replace", "path": "/spec/resources/requests/storage", "value": "2Gi"}]' ;;
  *) echo "Environment variables EXPAND_ONLY ('$EXPAND_ONLY') and MODIFY_ONLY ('$MODIFY_ONLY') are not properly set to either 'true' or 'false'" ;;
  esac

  echo "Patching PVCs with $patches"
  kubectl get pvc -o name | sed -e 's/.*\///g' | xargs -P 5 -I {} kubectl patch pvc {} --type=json -p="$patches"

  echo "Waiting until volumes modified and/or expanded"
  ensure_volumes_modified

  echo "Deleting resources"
  kubectl delete -f "$export_manifest_path"

  echo "Waiting for all PVs to be deleted"
  wait_for_pvs_to_delete

  trap - EXIT
}

ensure_volumes_modified() {
  if [[ "$EXPAND_ONLY" == "false" ]]; then
    while true; do
      modified_volumes_count=$(kubectl get pvc -o json | jq '.items | map(select(.status.currentVolumeAttributesClassName == "ebs-scale-test-expand-and-modify")) | length')
      echo "$modified_volumes_count/$REPLICAS volumes modified"
      if [[ "$modified_volumes_count" == "$REPLICAS" ]]; then
        echo "All volumes modified"
        break
      fi
      sleep 5
    done
  fi

  if [[ "$MODIFY_ONLY" == "false" ]]; then
    while true; do
      expanded_volumes_count=$(kubectl get pvc -o json | jq '.items | map(select(.status.capacity.storage == "2Gi")) | length')
      echo "$expanded_volumes_count/$REPLICAS volumes expanded"
      if [[ "$expanded_volumes_count" == "$REPLICAS" ]]; then
        echo "All volumes expanded"
        break
      fi
      sleep 5
    done
  fi
}

wait_for_pvcs_to_bind() {
  while true; do
    bound_pvc_count=$(kubectl get pvc -o json | jq '.items | map(select(.status.phase == "Bound")) | length')
    if [[ "$bound_pvc_count" -ge "$REPLICAS" ]]; then
      echo "All PVCs bound, proceeding..."
      break
    else
      echo "Only $bound_pvc_count PVCs are bound, waiting for a total of $REPLICAS..."
      sleep 5
    fi
  done
}

(return 0 2>/dev/null) || (
  echo "This script is not meant to be run directly, only sourced as a helper!"
  exit 1
)
