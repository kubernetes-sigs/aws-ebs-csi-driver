#!/bin/bash

# Copyright 2023 The Kubernetes Authors.
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

# This script updates the kustomize templates in deploy/kubernetes/base/ by
# running `helm template` and stripping the namespace from the output

set -euo pipefail

function eksctl_create_cluster() {
  CLUSTER_NAME=${1}
  EKSCTL_BIN=${2}
  ZONES=${3}
  INSTANCE_TYPE=${4}
  K8S_VERSION=${5}
  CLUSTER_FILE=${6}
  KUBECONFIG=${7}
  EKSCTL_PATCH_FILE=${8}
  EKSCTL_ADMIN_ROLE=${9}
  WINDOWS=${10}
  VPC_CONFIGMAP_FILE=${11}

  CLUSTER_NAME="${CLUSTER_NAME//./-}"

  if eksctl_cluster_exists "${CLUSTER_NAME}" "${EKSCTL_BIN}"; then
    loudecho "Upgrading cluster $CLUSTER_NAME with $CLUSTER_FILE"
    ${EKSCTL_BIN} upgrade cluster -f "${CLUSTER_FILE}"
  else
    loudecho "Creating cluster $CLUSTER_NAME with $CLUSTER_FILE (dry run)"
    ${EKSCTL_BIN} create cluster \
      --managed \
      --ssh-access=false \
      --zones "${ZONES}" \
      --nodes=3 \
      --instance-types="${INSTANCE_TYPE}" \
      --version="${K8S_VERSION}" \
      --disable-pod-imds \
      --dry-run \
      "${CLUSTER_NAME}" > "${CLUSTER_FILE}"

    if test -f "$EKSCTL_PATCH_FILE"; then
      eksctl_patch_cluster_file "$CLUSTER_FILE" "$EKSCTL_PATCH_FILE"
    fi

    loudecho "Creating cluster $CLUSTER_NAME with $CLUSTER_FILE"
    ${EKSCTL_BIN} create cluster -f "${CLUSTER_FILE}" --kubeconfig "${KUBECONFIG}"
  fi

  loudecho "Cluster ${CLUSTER_NAME} kubecfg written to ${KUBECONFIG}"
  loudecho "Getting cluster ${CLUSTER_NAME}"
  ${EKSCTL_BIN} get cluster "${CLUSTER_NAME}"

  if [[ -n "$EKSCTL_ADMIN_ROLE" ]]; then
    AWS_ACCOUNT_ID=$(aws sts get-caller-identity --query Account --output text)
    ADMIN_ARN="arn:aws:iam::${AWS_ACCOUNT_ID}:role/${EKSCTL_ADMIN_ROLE}"
    loudecho "Granting ${ADMIN_ARN} admin access to the cluster"
    ${EKSCTL_BIN} create iamidentitymapping --cluster "${CLUSTER_NAME}" --arn "${ADMIN_ARN}" --group system:masters --username admin
  fi

  if [[ "$WINDOWS" == true ]]; then
    ${EKSCTL_BIN} create nodegroup \
      --managed=true \
      --ssh-access=false \
      --cluster="${CLUSTER_NAME}" \
      --node-ami-family=WindowsServer2022CoreContainer \
      --instance-types=m5.2xlarge \
      -n ng-windows \
      -m 3 \
      -M 3 \

    kubectl apply --kubeconfig "${KUBECONFIG}" -f "$VPC_CONFIGMAP_FILE"
  fi

  return $?
}

function eksctl_cluster_exists() {
  CLUSTER_NAME=${1}
  EKSCTL_BIN=${2}
  set +e
  if ${EKSCTL_BIN} get cluster "${CLUSTER_NAME}"; then
    set -e
    return 0
  else
    set -e
    return 1
  fi
}

function eksctl_delete_cluster() {
  EKSCTL_BIN=${1}
  CLUSTER_NAME=${2}

  CLUSTER_NAME="${CLUSTER_NAME//./-}"

  loudecho "Deleting cluster ${CLUSTER_NAME}"
  ${EKSCTL_BIN} delete cluster "${CLUSTER_NAME}"
}

function eksctl_patch_cluster_file() {
  CLUSTER_FILE=${1}      # input must be yaml
  EKSCTL_PATCH_FILE=${2} # input must be yaml

  loudecho "Patching cluster $CLUSTER_NAME with $EKSCTL_PATCH_FILE"

  # Temporary intermediate files for patching
  CLUSTER_FILE_0=$CLUSTER_FILE.0
  CLUSTER_FILE_1=$CLUSTER_FILE.1

  cp "$CLUSTER_FILE" "$CLUSTER_FILE_0"

  # Patch only the Cluster
  kubectl patch --kubeconfig "/dev/null" -f "$CLUSTER_FILE_0" --local --type merge --patch "$(cat "$EKSCTL_PATCH_FILE")" -o yaml > "$CLUSTER_FILE_1"
  mv "$CLUSTER_FILE_1" "$CLUSTER_FILE_0"

  # Done patching, overwrite original CLUSTER_FILE
  mv "$CLUSTER_FILE_0" "$CLUSTER_FILE" # output is yaml
}
