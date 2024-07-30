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

# This script echos the KUBECONFIG back to the caller
# CLUSTER_NAME and CLUSTER_TYPE are expected to be specified by the caller

set -euo pipefail

BASE_DIR="$(dirname "$(realpath "${BASH_SOURCE[0]}")")"
KUBECONFIG="${BASE_DIR}/csi-test-artifacts/${CLUSTER_NAME}.${CLUSTER_TYPE}.kubeconfig"

echo "# Makefiles cannot export environment variables directly"
echo "# Run eval \"\$(make cluster/kubeconfig)\""
echo "export KUBECONFIG=\"${KUBECONFIG}\""
