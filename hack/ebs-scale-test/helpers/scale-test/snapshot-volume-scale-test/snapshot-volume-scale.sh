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

# We expect this helper script is sourced from hack/ebs-scale-test
path_to_snapshot_test_dir="${BASE_DIR}/helpers/scale-test/snapshot-volume-scale-test"

export SNAPSHOTS_PER_VOLUME=${SNAPSHOTS_PER_VOLUME:=1}

snapshot_scale_test() {
  manifest_path="$path_to_snapshot_test_dir/snapshot-volume-scale.yaml"
  export_manifest_path="$EXPORT_DIR/snapshot-manifest.yaml"

  echo "Applying $manifest_path. Exported to $export_manifest_path"
  gomplate -f "$manifest_path" -o "$export_manifest_path"
  kubectl apply -f "$export_manifest_path"

  trap 'echo "Test interrupted! Deleting test resources to prevent leak"; cleanup_snapshot_resources' EXIT

  echo "Waiting for all PVCs to be bound"
  wait_for_pvcs_bound

  echo "Creating $REPLICAS volume snapshots ($SNAPSHOTS_PER_VOLUME per volume)"
  create_snapshots

  echo "Waiting for snapshots to be ready"
  wait_for_snapshots_ready

  echo "Restoring volumes from snapshots"
  restore_volumes_from_snapshots

  echo "Waiting for restored PVCs to be bound"
  wait_for_restored_pvcs_bound

  echo "Deleting snapshots"
  delete_snapshots

  echo "Waiting for VolumeSnapshotContents to be deleted"
  wait_for_volume_snapshot_contents_deleted

  echo "Deleting PVCs"
  delete_pvcs

  echo "Waiting for PVs to be deleted"
  wait_for_pvs_deleted

  echo "Cleaning up snapshot resources"
  cleanup_snapshot_resources

  trap - EXIT
}

wait_for_pvcs_bound() {
  while true; do
    bound_pvc_count=$(kubectl get pvc -l app=snapshot-scale-test -o json | jq '.items | map(select(.status.phase == "Bound")) | length')
    if [[ "$bound_pvc_count" -ge "$REPLICAS" ]]; then
      echo "All PVCs bound, proceeding..."
      break
    else
      echo "Only $bound_pvc_count PVCs are bound, waiting for a total of $REPLICAS..."
      sleep 5
    fi
  done
}

create_snapshots() {
  for ((i = 0; i < REPLICAS; i++)); do
    for ((j = 1; j <= SNAPSHOTS_PER_VOLUME; j++)); do
      cat <<EOF | kubectl apply -f -
apiVersion: snapshot.storage.k8s.io/v1
kind: VolumeSnapshot
metadata:
  name: snapshot-volume-$i-$j
  labels:
    app: snapshot-scale-test
spec:
  volumeSnapshotClassName: csi-aws-vsc
  source:
    persistentVolumeClaimName: snapshot-pvc-$i
EOF
    done
  done
}

wait_for_snapshots_ready() {
  expected_REPLICAS=$((REPLICAS * SNAPSHOTS_PER_VOLUME))
  while true; do
    ready_REPLICAS=$(kubectl get volumesnapshot -l app=snapshot-scale-test -o json | jq '.items | map(select(.status.readyToUse == true)) | length')
    if [[ "$ready_REPLICAS" -ge "$expected_REPLICAS" ]]; then
      echo "All snapshots ready to use, proceeding..."
      break
    else
      echo "$ready_REPLICAS/$expected_REPLICAS snapshots ready to use, waiting..."
      sleep 5
    fi
  done
}

restore_volumes_from_snapshots() {
  for i in $(seq 0 $(($REPLICAS - 1))); do
    cat <<EOF | kubectl apply -f -
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: restored-pvc-$i
  labels:
    app: snapshot-scale-test
    type: restored
spec:
  accessModes:
    - ReadWriteOnce
  storageClassName: ebs-snapshot-test
  resources:
    requests:
      storage: 1Gi
  dataSource:
    name: snapshot-volume-$i-1
    kind: VolumeSnapshot
    apiGroup: snapshot.storage.k8s.io
EOF
  done
}

wait_for_restored_pvcs_bound() {
  while true; do
    bound_pvc_count=$(kubectl get pvc -l app=snapshot-scale-test,type=restored -o json | jq '.items | map(select(.status.phase == "Bound")) | length')
    if [[ "$bound_pvc_count" -ge "$REPLICAS" ]]; then
      echo "All restored PVCs bound, proceeding..."
      break
    else
      echo "Only $bound_pvc_count restored PVCs are bound, waiting for a total of $REPLICAS..."
      sleep 5
    fi
  done
}

delete_snapshots() {
  kubectl delete volumesnapshot -l app=snapshot-scale-test
}

wait_for_volume_snapshot_contents_deleted() {
  while true; do
    snapshot_content_count=$(kubectl get volumesnapshotcontent -o json | jq '.items | map(select(.spec.volumeSnapshotRef.name | startswith("snapshot-volume-"))) | length')
    if [[ "$snapshot_content_count" -eq 0 ]]; then
      echo "All VolumeSnapshotContents deleted, proceeding..."
      break
    else
      echo "$snapshot_content_count VolumeSnapshotContents still exist, waiting..."
      sleep 5
    fi
  done
}

delete_pvcs() {
  kubectl delete pvc -l app=snapshot-scale-test
}

wait_for_pvs_deleted() {
  while true; do
    pv_count=$(kubectl get pv -o json | jq '.items | map(select(.spec.claimRef.name | startswith("snapshot-pvc-") or startswith("restored-pvc-"))) | length')
    if [[ "$pv_count" -eq 0 ]]; then
      echo "All PVs deleted, proceeding..."
      break
    else
      echo "$pv_count PVs still exist, waiting..."
      sleep 5
    fi
  done
}

cleanup_snapshot_resources() {
  kubectl delete volumesnapshotclass csi-aws-vsc --ignore-not-found=true
  kubectl delete sc ebs-snapshot-test --ignore-not-found=true
  kubectl delete volumesnapshot -l app=snapshot-scale-test --ignore-not-found=true
  kubectl delete pvc -l app=snapshot-scale-test --ignore-not-found=true
}

(return 0 2>/dev/null) || (
  echo "This script is not meant to be run directly, only sourced as a helper!"
  exit 1
)
