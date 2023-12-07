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

BIN="$(dirname "$(realpath "${BASH_SOURCE[0]}")")/../bin"
TEMP_DIR=$(mktemp -d)
trap "rm -rf \"${TEMP_DIR}\"" EXIT
cp "deploy/kubernetes/base/kustomization.yaml" "${TEMP_DIR}/kustomization.yaml"

"${BIN}/helm" template --output-dir "${TEMP_DIR}" --skip-tests --api-versions 'snapshot.storage.k8s.io/v1' --api-versions 'policy/v1/PodDisruptionBudget' --set 'controller.userAgentExtra=kustomize' kustomize charts/aws-ebs-csi-driver > /dev/null
rm -rf "deploy/kubernetes/base"
mv "${TEMP_DIR}/aws-ebs-csi-driver/templates" "deploy/kubernetes/base"

sed -i '/namespace:/d' deploy/kubernetes/base/*
cp "${TEMP_DIR}/kustomization.yaml" "deploy/kubernetes/base/kustomization.yaml"
