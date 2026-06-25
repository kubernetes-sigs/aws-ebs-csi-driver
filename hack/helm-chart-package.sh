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
# the `helm-chart-push` Makefile target, which runs from hack/cloudbuild.sh when
# triggered by a helm-chart tag (e.g. helm-chart-aws-ebs-csi-driver-2.62.0).

set -euo pipefail

BIN="$(dirname "$(realpath "${BASH_SOURCE[0]}")")/../bin"

HELM=${HELM:-"${BIN}/helm"}
CHART_DIR=${CHART_DIR:-charts/aws-ebs-csi-driver}
DEST_CHART_DIR=${DEST_CHART_DIR:-artifacts}
HELM_CHART_REPO=${HELM_CHART_REPO:-gcr.io/k8s-staging-provider-aws/charts}

# The chart version is extracted from the helm-chart tag that triggered this
# build (e.g. helm-chart-aws-ebs-csi-driver-2.62.0 -> 2.62.0). The tag is
# created by chart-releaser-action.
chart_version="${PULL_BASE_REF##helm-chart-aws-ebs-csi-driver-}"

${HELM} package --version "${chart_version}" "${CHART_DIR}" -d "${DEST_CHART_DIR}"
${HELM} push "${DEST_CHART_DIR}/aws-ebs-csi-driver-${chart_version}.tgz" "oci://${HELM_CHART_REPO}"
