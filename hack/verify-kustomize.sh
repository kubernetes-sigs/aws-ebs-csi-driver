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

set -euo pipefail

readonly ROOT="$(git rev-parse --show-toplevel)"
readonly KUSTOMIZE="${ROOT}/bin/kustomize"

verify_registry() {
  local overlay="${1}"
  local expected_registry="${2}"
  local rendered
  local images
  local image
  local failed=0

  rendered="$("${KUSTOMIZE}" build "${ROOT}/${overlay}")"
  images="$(
    printf '%s\n' "${rendered}" |
      sed -nE 's/^[[:space:]]*(-[[:space:]]+)?image:[[:space:]]*"?([^"[:space:]]+)"?[[:space:]]*$/\2/p'
  )"

  if [[ -z "${images}" ]]; then
    echo "No images found in rendered overlay ${overlay}" >&2
    return 1
  fi

  while IFS= read -r image; do
    if [[ "${image}" != "${expected_registry}"* ]]; then
      echo "${overlay}: unexpected image registry: ${image}" >&2
      failed=1
    fi
  done <<<"${images}"

  return "${failed}"
}

verify_registry "deploy/kubernetes/overlays/stable" "public.ecr.aws/"
verify_registry "deploy/kubernetes/overlays/stable/ecr-public" "public.ecr.aws/"
verify_registry "deploy/kubernetes/overlays/stable/ecr" "602401143452.dkr.ecr.us-west-2.amazonaws.com/"
verify_registry "deploy/kubernetes/overlays/stable/gcr" "registry.k8s.io/"
