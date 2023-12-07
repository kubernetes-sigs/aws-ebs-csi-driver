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

# This script creates a cluster for use of running the e2e tests
# CLUSTER_NAME and CLUSTER_TYPE are expected to be specified by the caller
# All other environment variables have default values (see config.sh) but
# many can be overridden on demand if needed

set -euo pipefail

BASE_DIR="$(dirname "$(realpath "${BASH_SOURCE[0]}")")"
BIN="${BASE_DIR}/../../bin"

source "${BASE_DIR}/config.sh"
source "${BASE_DIR}/util.sh"
source "${BASE_DIR}/kops/kops.sh"
source "${BASE_DIR}/eksctl/eksctl.sh"

if [[ "${CLUSTER_TYPE}" == "kops" ]]; then
  BUCKET_CHECK=$("${BIN}/aws" s3api head-bucket --region us-east-1 --bucket "${KOPS_BUCKET}" 2>&1 || true)
  if grep -q "Forbidden" <<< "${BUCKET_CHECK}"; then
    echo "Kops requires a S3 bucket in order to store the state" >&2
    echo "This script is attempting to use a bucket called \`${KOPS_BUCKET}\`" >&2
    echo "That bucket already exists and you do not have access to it" >&2
    echo "You can change the bucket by setting the environment variable \$KOPS_BUCKET" >&2
    exit 1
  fi
  if grep -q "Not Found" <<< "${BUCKET_CHECK}"; then
    "${BIN}/aws" s3api create-bucket --region us-east-1 --bucket "${KOPS_BUCKET}" --acl private >/dev/null
  fi

  kops_create_cluster \
    "$CLUSTER_NAME" \
    "${BIN}/kops" \
    "$ZONES" \
    "$NODE_COUNT" \
    "$INSTANCE_TYPE" \
    "$AMI_ID" \
    "$K8S_VERSION_KOPS" \
    "$CLUSTER_FILE" \
    "$KUBECONFIG" \
    "${BASE_DIR}/kops/patch-cluster.yaml" \
    "${BASE_DIR}/kops/patch-node.yaml" \
    "s3://${KOPS_BUCKET}"
elif [[ "${CLUSTER_TYPE}" == "eksctl" ]]; then
  eksctl_create_cluster \
    "$CLUSTER_NAME" \
    "${BIN}/eksctl" \
    "$ZONES" \
    "$INSTANCE_TYPE" \
    "$K8S_VERSION_EKSCTL" \
    "$CLUSTER_FILE" \
    "$KUBECONFIG" \
    "${BASE_DIR}/eksctl/patch.yaml" \
    "$EKSCTL_ADMIN_ROLE" \
    "$WINDOWS" \
    "${BASE_DIR}/eksctl/vpc-resource-controller-configmap.yaml"
else
  echo "Cluster type ${CLUSTER_TYPE} is invalid, must be kops or eksctl" >&2
  exit 1
fi

