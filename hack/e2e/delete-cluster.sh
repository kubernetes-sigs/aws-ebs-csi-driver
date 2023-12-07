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

# This script deletes a cluster that was created by `create-cluster.sh`
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
  kops_delete_cluster \
    "${BIN}/kops" \
    "${CLUSTER_NAME}" \
    "s3://${KOPS_BUCKET}"
elif [[ "${CLUSTER_TYPE}" == "eksctl" ]]; then
  eksctl_delete_cluster \
    "${BIN}/eksctl" \
    "${CLUSTER_NAME}"
else
  echo "Cluster type ${CLUSTER_TYPE} is invalid, must be kops or eksctl" >&2
  exit 1
fi
