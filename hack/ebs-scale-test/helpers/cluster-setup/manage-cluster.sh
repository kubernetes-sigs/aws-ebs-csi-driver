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

### Helper script to create/delete eks ebs-scale-test clusters and install add-ons.

set -euo pipefail

# We expect this helper script is sourced from hack/ebs-scale-test
path_to_cluster_setup_dir="${BASE_DIR}/helpers/cluster-setup/"

## Cluster

create_cluster() {
  if eksctl get cluster --name "$CLUSTER_NAME" --region "$AWS_REGION" >/dev/null 2>&1; then
    echo "EKS cluster '$CLUSTER_NAME' already up in $AWS_REGION."
    aws eks update-kubeconfig --name "$CLUSTER_NAME" --region "$AWS_REGION"
  else
    echo "Deploying EKS cluster. See configuration in $EXPORT_DIR/cluster-config.yaml"
    gomplate -f "$path_to_cluster_setup_dir/scale-cluster-config.yaml" -o "$EXPORT_DIR/cluster-config.yaml"
    eksctl create cluster -f "$EXPORT_DIR/cluster-config.yaml"
  fi
}

cleanup_cluster() {
  eksctl delete cluster "$CLUSTER_NAME"
}

## Misc

check_lingering_volumes() {
  lingering_vol_count=$(aws ec2 describe-volumes \
    --filters "Name=tag:ebs-scale-test,Values=${SCALABILITY_TEST_RUN_NAME}" \
    --query 'length(Volumes[*])' \
    --output text)

  [[ lingering_vol_count -ne 0 ]] && echo "WARNING: detected $lingering_vol_count lingering ebs-scale-test EBS volumes in $AWS_ACCOUNT_ID. Please run \`aws ec2 describe-volumes --filters 'Name=tag-key,Values=ebs-scale-test'\` and audit their AWS resource tags. Note these volumes may belong to a different scalability run than $SCALABILITY_TEST_RUN_NAME"
}

check_lingering_snapshots() {
  lingering_snap_count=$(aws ec2 describe-snapshots \
    --filters "Name=tag:ebs-scale-test,Values=${SCALABILITY_TEST_RUN_NAME}" \
    --query 'length(Snapshots[*])' \
    --output text)

  [[ lingering_snap_count -ne 0 ]] && echo "WARNING: detected $lingering_snap_count lingering ebs-scale-test EBS snapshots from run ${SCALABILITY_TEST_RUN_NAME} in $AWS_ACCOUNT_ID. Please run \`aws ec2 describe-snapshots --filters 'Name=tag:ebs-scale-test,Values=${SCALABILITY_TEST_RUN_NAME}'\` and audit their AWS resource tags."
}

## EBS CSI Driver

deploy_ebs_csi_driver() {
  path_to_chart="${BASE_DIR}/../../charts/aws-ebs-csi-driver"
  echo "Deploying EBS CSI driver from chart $path_to_chart"

  # We use helm install instead of upgrade to ensure the release does not already exist
  helm install aws-ebs-csi-driver \
    --namespace kube-system \
    --values "$DRIVER_VALUES_FILEPATH" \
    --wait \
    --timeout 15m \
    "$path_to_chart"
}

(return 0 2>/dev/null) || (
  echo "This script is not meant to be run directly, only sourced as a helper!"
  exit 1
)

deploy_snapshot_controller() {
  echo "Deploying snapshot controller and CRDs"
  kubectl apply -f https://raw.githubusercontent.com/kubernetes-csi/external-snapshotter/"${EBS_INSTALL_SNAPSHOT_VERSION}"/deploy/kubernetes/snapshot-controller/rbac-snapshot-controller.yaml
  kubectl apply -f https://raw.githubusercontent.com/kubernetes-csi/external-snapshotter/"${EBS_INSTALL_SNAPSHOT_VERSION}"/deploy/kubernetes/snapshot-controller/setup-snapshot-controller.yaml
  kubectl apply -f https://raw.githubusercontent.com/kubernetes-csi/external-snapshotter/"${EBS_INSTALL_SNAPSHOT_VERSION}"/client/config/crd/snapshot.storage.k8s.io_volumesnapshotclasses.yaml
  kubectl apply -f https://raw.githubusercontent.com/kubernetes-csi/external-snapshotter/"${EBS_INSTALL_SNAPSHOT_VERSION}"/client/config/crd/snapshot.storage.k8s.io_volumesnapshotcontents.yaml
  kubectl apply -f https://raw.githubusercontent.com/kubernetes-csi/external-snapshotter/"${EBS_INSTALL_SNAPSHOT_VERSION}"/client/config/crd/snapshot.storage.k8s.io_volumesnapshots.yaml
}
