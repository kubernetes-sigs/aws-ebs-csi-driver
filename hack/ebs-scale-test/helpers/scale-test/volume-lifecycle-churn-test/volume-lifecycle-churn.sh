#!/bin/bash
# Copyright 2026 The Kubernetes Authors.
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

### Helper script for running volume lifecycle churn scale test.
###
### Workloads like Spark pipelines produce overlapping waves of short-lived
### Jobs. While one wave's volumes are being detached/deleted, the next
### wave's volumes are being created/attached. This creates concurrent
### create+attach / detach+delete pressure on the driver which is the key
### characteristic we want to benchmark.
###
### Parameters:
###   REPLICAS - Jobs per wave. Each Job gets one PVC.
###   WAVES    - Number of sequential waves to run.

# We expect this helper script is sourced from hack/ebs-scale-test
path_to_churn_test_dir="${BASE_DIR}/helpers/scale-test/volume-lifecycle-churn-test"

export WAVES=${WAVES:=3}

volume_lifecycle_churn_test() {
  manifest_path="$path_to_churn_test_dir/volume-lifecycle-churn.yaml"
  export_manifest_path="$EXPORT_DIR/volume-lifecycle-churn-manifest.yaml"

  echo "Applying StorageClass from $manifest_path. Exported to $export_manifest_path"
  gomplate -f "$manifest_path" -o "$export_manifest_path"
  kubectl apply -f "$export_manifest_path"

  trap 'echo "Test interrupted! Cleaning up"; cleanup_churn_resources' EXIT

  echo "Starting volume-lifecycle-churn-test: WAVES=$WAVES, REPLICAS=$REPLICAS"

  for ((wave = 0; wave < WAVES; wave++)); do
    echo "=== Wave $wave: submitting $REPLICAS Jobs ==="
    submit_wave "$wave"

    echo "=== Wave $wave: waiting for Jobs to complete ==="
    wait_for_wave_jobs_complete "$wave"

    echo "=== Wave $wave: deleting Jobs and PVCs ==="
    kubectl delete jobs -l "churn-wave=$wave" --wait=false
    kubectl delete pvc -l "churn-wave=$wave" --wait=false
  done

  echo "Waiting for all PVCs to be deleted"
  wait_for_churn_pvcs_to_delete

  echo "Waiting for all PVs to be deleted"
  wait_for_churn_pvs_to_delete

  echo "Cleaning up StorageClass"
  kubectl delete -f "$export_manifest_path"

  trap - EXIT
}

submit_wave() {
  local wave=$1
  for ((i = 0; i < REPLICAS; i++)); do
    create_churn_job "$wave" "$i"
  done
}

create_churn_job() {
  local wave=$1
  local index=$2
  local job_name="churn-w${wave}-${index}"
  local pvc_name="churn-vol-w${wave}-${index}"

  cat <<EOF | kubectl apply -f -
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: ${pvc_name}
  labels:
    app: volume-lifecycle-churn-test
    churn-wave: "${wave}"
spec:
  accessModes: ["ReadWriteOnce"]
  storageClassName: ebs-volume-lifecycle-churn-test
  resources:
    requests:
      storage: 1Gi
---
apiVersion: batch/v1
kind: Job
metadata:
  name: ${job_name}
  labels:
    app: volume-lifecycle-churn-test
    churn-wave: "${wave}"
spec:
  backoffLimit: 0
  template:
    metadata:
      labels:
        app: volume-lifecycle-churn-test
        churn-wave: "${wave}"
    spec:
      restartPolicy: Never
      containers:
        - name: worker
          image: public.ecr.aws/docker/library/busybox:latest
          command: ["sh", "-c", "dd if=/dev/urandom of=/mnt/data/scratch.dat bs=1M count=64 && sync"]
          volumeMounts:
            - name: scratch
              mountPath: /mnt/data
          resources:
            requests:
              memory: "256Mi"
              cpu: "250m"
            limits:
              memory: "256Mi"
      volumes:
        - name: scratch
          persistentVolumeClaim:
            claimName: ${pvc_name}
$(if [[ "${CLUSTER_TYPE}" == "karpenter" ]]; then
    echo "      nodeSelector:"
    echo "        karpenter.sh/nodepool: ebs-scale-test"
  fi)
EOF
}

wait_for_wave_jobs_complete() {
  local wave=$1
  while true; do
    local complete
    complete=$(kubectl get jobs -l "churn-wave=$wave" -o json | jq '[.items[] | select(.status.succeeded == 1)] | length')
    if ((complete >= REPLICAS)); then
      echo "Wave $wave: $complete/$REPLICAS Jobs complete"
      break
    fi
    echo "Wave $wave: $complete/$REPLICAS Jobs complete, waiting..."
    sleep 5
  done
}

wait_for_churn_pvcs_to_delete() {
  while true; do
    local pvc_count
    pvc_count=$(kubectl get pvc -l app=volume-lifecycle-churn-test --no-headers 2>/dev/null | wc -l)
    if ((pvc_count == 0)); then
      echo "All volume-lifecycle-churn-test PVCs deleted"
      break
    fi
    echo "$pvc_count PVCs still exist, waiting..."
    sleep 5
  done
}

wait_for_churn_pvs_to_delete() {
  while true; do
    local pv_count
    pv_count=$(kubectl get pv -o json | jq '[.items[] | select(.spec.storageClassName == "ebs-volume-lifecycle-churn-test")] | length')
    if ((pv_count == 0)); then
      echo "All volume-lifecycle-churn-test PVs deleted"
      break
    fi
    echo "$pv_count PVs still exist, waiting..."
    sleep 5
  done
}

cleanup_churn_resources() {
  echo "Cleaning up all volume-lifecycle-churn-test resources"
  kubectl delete jobs -l app=volume-lifecycle-churn-test --wait=false --ignore-not-found=true
  kubectl delete pvc -l app=volume-lifecycle-churn-test --wait=false --ignore-not-found=true
  kubectl delete sc ebs-volume-lifecycle-churn-test --ignore-not-found=true
  wait_for_churn_pvs_to_delete
}

(return 0 2>/dev/null) || (
  echo "This script is not meant to be run directly, only sourced as a helper!"
  exit 1
)
