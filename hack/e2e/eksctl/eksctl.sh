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
  GOMPLATE_BIN=${3}
  REGION=${4}
  ZONES=${5}
  INSTANCE_TYPE=${6}
  K8S_VERSION=${7}
  CLUSTER_FILE=${8}
  KUBECONFIG=${9}
  WINDOWS=${10}
  VPC_CONFIGMAP_FILE=${11}
  TEMPLATE_FILE=${12}

  CLUSTER_NAME="${CLUSTER_NAME//./-}"

  loudecho "Templating $CLUSTER_NAME to $CLUSTER_FILE"
  CLUSTER_NAME="${CLUSTER_NAME}" \
  REGION="${REGION}" \
  K8S_VERSION="${K8S_VERSION}" \
  ZONES="${ZONES}" \
  INSTANCE_TYPE="${INSTANCE_TYPE}" \
  ${GOMPLATE_BIN} -f "${TEMPLATE_FILE}" -o "${CLUSTER_FILE}"

  if eksctl_cluster_exists "${CLUSTER_NAME}" "${EKSCTL_BIN}"; then
    loudecho "Upgrading cluster $CLUSTER_NAME with $CLUSTER_FILE"
    ${EKSCTL_BIN} upgrade cluster -f "${CLUSTER_FILE}"
  else
    loudecho "Creating cluster $CLUSTER_NAME with $CLUSTER_FILE"
    ${EKSCTL_BIN} create cluster -f "${CLUSTER_FILE}" --kubeconfig "${KUBECONFIG}"
  fi

  if [[ "$WINDOWS" == true ]]; then
    loudecho "Applying VPC ConfigMap (Windows only)"
    kubectl apply --kubeconfig "${KUBECONFIG}" -f "${VPC_CONFIGMAP_FILE}"
  fi
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
