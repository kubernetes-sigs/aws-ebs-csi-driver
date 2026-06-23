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

# Packages the Helm chart and pushes it to the staging OCI registry. Invoked by
# the `helm-chart-push` Makefile target, which runs from hack/cloudbuild.sh on
# release branches.

set -euo pipefail

HELM=${HELM:-./bin/helm}
YQ=${YQ:-./bin/yq}
CHART_DIR=${CHART_DIR:-charts/aws-ebs-csi-driver}
DEST_CHART_DIR=${DEST_CHART_DIR:-artifacts}
HELM_CHART_REPO=${HELM_CHART_REPO:-gcr.io/k8s-staging-provider-aws/charts}

# The chart version is decoupled from the app version (chart 2.X.Y tracks app
# 1.X.Y), so read it from Chart.yaml rather than deriving it from the git tag.
chart_name=$(${YQ} '.name' "${CHART_DIR}/Chart.yaml")
chart_version=$(${YQ} '.version' "${CHART_DIR}/Chart.yaml")

if ${HELM} show chart "oci://${HELM_CHART_REPO}/${chart_name}" --version "${chart_version}" >/dev/null 2>&1; then
  echo "Chart ${chart_name} ${chart_version} already present in oci://${HELM_CHART_REPO}; skipping push"
  exit 0
fi

${HELM} package "${CHART_DIR}" -d "${DEST_CHART_DIR}"
${HELM} push "${DEST_CHART_DIR}/${chart_name}-${chart_version}.tgz" "oci://${HELM_CHART_REPO}"
