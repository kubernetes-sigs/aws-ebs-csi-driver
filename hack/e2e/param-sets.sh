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

# Parameter set definitions for e2e parameter tests that require a live cluster.
# Tests that only assert on rendered Kubernetes object specs run via
# `go test ./tests/helm-template/...` and do not need a cluster.
#
# Each set only needs to set GINKGO_FOCUS. By default, Helm values come from
# tests/helm-template/testdata/e2e-<set>.yaml (or a file pointed at by
# VALUES_FILE), shared with the helm-template Go tests as a single source
# of truth. A set may override HELM_EXTRA_FLAGS directly when the set only
# needs one or two flags and a values file isn't worth it.
#
# Sets in PARAM_SETS_ALL:
#   standard            - Volume tagging (EC2 API) and defaultFsType (mount check)
#   miscellaneous       - Metadata labeler node labels and additional DaemonSet scheduling
#   node-component-only - Deploys only node DaemonSet without controller
#   fips                - Builds FIPS image then validates it is deployed
#   legacy-compat       - legacyXFS behavior

set -euo pipefail

BASE_DIR="$(dirname "$(realpath "${BASH_SOURCE[0]}")")"
VALUES_DIR="${BASE_DIR}/../../tests/helm-template/testdata"

PARAM_SETS_ALL="standard miscellaneous node-component-only fips legacy-compat"

param_set_standard() {
  GINKGO_FOCUS="\[param:(extraCreateMetadata|k8sTagClusterId|extraVolumeTags|defaultFsType)\]"
  # Opt out of install_driver()'s hardcoded --set controller.k8sTagClusterId=$CLUSTER_NAME
  # so the value from e2e-standard.yaml takes effect (otherwise --set would beat --values).
  HELM_OVERRIDE_K8S_TAG_CLUSTER_ID=true
}

param_set_miscellaneous() {
  GINKGO_FOCUS="\[param:(metadataLabeler|additionalDaemonSets)\]"
}

param_set_node-component-only() {
  GINKGO_FOCUS="\[param:nodeComponentOnly\]"
  # Shares values with the helm-template TestNodeComponentOnly (identical content).
  VALUES_FILE="${VALUES_DIR}/node-component-only.yaml"
}

param_set_legacy-compat() {
  GINKGO_FOCUS="\[param:legacyXFS\]"
  HELM_EXTRA_FLAGS="--set=node.legacyXFS=true"
}

param_set_fips() {
  GINKGO_FOCUS="\[param:fips\]"
  # Single flag; inlined instead of maintaining a one-line values file.
  HELM_EXTRA_FLAGS="--set=fips=true"
  FIPS_TEST=true
}

# Load a parameter set by name, exporting GINKGO_FOCUS and HELM_EXTRA_FLAGS.
# By default HELM_EXTRA_FLAGS is derived from a values file convention
# (e2e-<name>.yaml). A per-set function may override HELM_EXTRA_FLAGS or
# VALUES_FILE directly when the values-file convention doesn't fit.
load_param_set() {
  local name="$1"
  local func="param_set_${name}"
  if ! declare -f "$func" >/dev/null 2>&1; then
    echo "Unknown parameter set: ${name}" >&2
    echo "Available sets: ${PARAM_SETS_ALL}" >&2
    exit 1
  fi
  HELM_EXTRA_FLAGS=""
  VALUES_FILE="${VALUES_DIR}/e2e-${name}.yaml"
  # Clear state that a previous set's param_set_* function may have set,
  # so sets are independent of run order.
  unset HELM_OVERRIDE_K8S_TAG_CLUSTER_ID
  unset FIPS_TEST
  "$func"
  if [[ -z "$HELM_EXTRA_FLAGS" ]]; then
    HELM_EXTRA_FLAGS="--values=${VALUES_FILE}"
  fi
  export GINKGO_FOCUS HELM_EXTRA_FLAGS
  export GINKGO_PARALLEL="${GINKGO_PARALLEL:-5}"
  export AWS_AVAILABILITY_ZONES="${AWS_AVAILABILITY_ZONES:-us-west-2a}"
  export TEST_PATH="${TEST_PATH:-./tests/e2e/...}"
  export JUNIT_REPORT="${REPORT_DIR:-/logs/artifacts}/junit-params-${name}.xml"
  if [[ -n "${EBS_INSTALL_SNAPSHOT+x}" ]]; then export EBS_INSTALL_SNAPSHOT; fi
  if [[ -n "${FIPS_TEST+x}" ]]; then export FIPS_TEST; fi
  if [[ -n "${HELM_OVERRIDE_K8S_TAG_CLUSTER_ID+x}" ]]; then export HELM_OVERRIDE_K8S_TAG_CLUSTER_ID; fi
}

# Run a single parameter set
run_param_set() {
  load_param_set "$1"
  if [[ "${FIPS_TEST:-}" == "true" ]]; then
    echo "### Building FIPS image for param set: $1"
    FIPS_TEST=true make cluster/image || {
      echo "FIPS image build failed!" >&2
      return 1
    }
  fi
  echo "### Running parameter set: $1"
  ./hack/e2e/run.sh
}

# Run all standard parameter sets sequentially
run_all_param_sets() {
  echo "Running all parameter sets sequentially..."
  local failed_sets=()
  for set in $PARAM_SETS_ALL; do
    run_param_set "$set" || failed_sets+=("$set")
  done
  if [[ ${#failed_sets[@]} -gt 0 ]]; then
    echo "Failed parameter sets: ${failed_sets[*]}" >&2
    return 1
  fi
  echo "All parameter sets completed successfully!"
}

# Allow direct invocation: ./hack/e2e/param-sets.sh run <name> or ./hack/e2e/param-sets.sh run-all
if [[ "${BASH_SOURCE[0]}" == "${0}" ]]; then
  case "${1:-}" in
  run)
    [[ -z "${2:-}" ]] && {
      echo "Usage: $0 run <param-set-name>" >&2
      exit 1
    }
    run_param_set "$2"
    ;;
  run-all)
    run_all_param_sets
    ;;
  *)
    echo "Usage: $0 {run <param-set-name>|run-all}" >&2
    exit 1
    ;;
  esac
fi
