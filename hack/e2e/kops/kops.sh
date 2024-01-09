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

function kops_create_cluster() {
  CLUSTER_NAME=${1}
  KOPS_BIN=${2}
  ZONES=${3}
  NODE_COUNT=${4}
  INSTANCE_TYPE=${5}
  AMI_ID=${6}
  K8S_VERSION=${7}
  CLUSTER_FILE=${8}
  KUBECONFIG=${9}
  KOPS_PATCH_FILE=${10}
  KOPS_PATCH_NODE_FILE=${11}
  KOPS_STATE_FILE=${12}

  if kops_cluster_exists "${CLUSTER_NAME}" "${KOPS_BIN}" "${KOPS_STATE_FILE}"; then
    loudecho "Replacing cluster $CLUSTER_NAME with $CLUSTER_FILE"
    ${KOPS_BIN} replace --state "${KOPS_STATE_FILE}" -f "${CLUSTER_FILE}"
  else
    loudecho "Creating cluster $CLUSTER_NAME with $CLUSTER_FILE (dry run)"
    ${KOPS_BIN} create cluster --state "${KOPS_STATE_FILE}" \
      --zones "${ZONES}" \
      --node-count="${NODE_COUNT}" \
      --node-size="${INSTANCE_TYPE}" \
      --image="${AMI_ID}" \
      --kubernetes-version="${K8S_VERSION}" \
      --dry-run \
      -o yaml \
      "${CLUSTER_NAME}" >"${CLUSTER_FILE}"

    if test -f "$KOPS_PATCH_FILE"; then
      kops_patch_cluster_file "$CLUSTER_FILE" "$KOPS_PATCH_FILE" "Cluster" ""
    fi
    if test -f "$KOPS_PATCH_NODE_FILE"; then
      kops_patch_cluster_file "$CLUSTER_FILE" "$KOPS_PATCH_NODE_FILE" "InstanceGroup" "Node"
    fi

    loudecho "Creating cluster $CLUSTER_NAME with $CLUSTER_FILE"
    ${KOPS_BIN} create --state "${KOPS_STATE_FILE}" -f "${CLUSTER_FILE}"
  fi

  loudecho "Updating cluster $CLUSTER_NAME with $CLUSTER_FILE"
  ${KOPS_BIN} update cluster --state "${KOPS_STATE_FILE}" "${CLUSTER_NAME}" --yes

  loudecho "Exporting cluster ${CLUSTER_NAME} kubecfg to ${KUBECONFIG}"
  ${KOPS_BIN} export kubecfg --state "${KOPS_STATE_FILE}" "${CLUSTER_NAME}" --admin --kubeconfig "${KUBECONFIG}"

  loudecho "Validating cluster ${CLUSTER_NAME}"
  ${KOPS_BIN} validate cluster --state "${KOPS_STATE_FILE}" --wait 10m --kubeconfig "${KUBECONFIG}"
  return $?
}

function kops_cluster_exists() {
  CLUSTER_NAME=${1}
  KOPS_BIN=${2}
  KOPS_STATE_FILE=${3}
  set +e
  if ${KOPS_BIN} get cluster --state "${KOPS_STATE_FILE}" "${CLUSTER_NAME}"; then
    set -e
    return 0
  else
    set -e
    return 1
  fi
}

function kops_delete_cluster() {
  KOPS_BIN=${1}
  CLUSTER_NAME=${2}
  KOPS_STATE_FILE=${3}
  loudecho "Deleting cluster ${CLUSTER_NAME}"
  ${KOPS_BIN} delete cluster --name "${CLUSTER_NAME}" --state "${KOPS_STATE_FILE}" --yes
}

# TODO switch this to python, work exclusively with yaml, use kops toolbox
# template/kops set?, all this hacking with jq stinks!
function kops_patch_cluster_file() {
  CLUSTER_FILE=${1}    # input must be yaml
  KOPS_PATCH_FILE=${2} # input must be yaml
  KIND=${3}            # must be either Cluster or InstanceGroup
  ROLE=${4}            # must be either Master or Node

  loudecho "Patching cluster $CLUSTER_NAME with $KOPS_PATCH_FILE"

  # Temporary intermediate files for patching, don't mutate CLUSTER_FILE until
  # the end
  CLUSTER_FILE_JSON=$CLUSTER_FILE.json
  CLUSTER_FILE_0=$CLUSTER_FILE.0
  CLUSTER_FILE_1=$CLUSTER_FILE.1

  # HACK convert the multiple yaml documents to an array of json objects
  yaml_to_json "$CLUSTER_FILE" "$CLUSTER_FILE_JSON"

  # Find the json objects to patch
  FILTER=".[] | select(.kind==\"$KIND\")"
  if [ -n "$ROLE" ]; then
    FILTER="$FILTER | select(.spec.role==\"$ROLE\")"
  fi
  jq "$FILTER" "$CLUSTER_FILE_JSON" >"$CLUSTER_FILE_0"

  # Patch only the json objects
  kubectl patch -f "$CLUSTER_FILE_0" --local --type merge --patch "$(cat "$KOPS_PATCH_FILE")" -o json >"$CLUSTER_FILE_1"
  mv "$CLUSTER_FILE_1" "$CLUSTER_FILE_0"

  # Delete the original json objects, add the patched
  # TODO Cluster must always be first?
  jq "del($FILTER)" "$CLUSTER_FILE_JSON" | jq ". + \$patched | sort" --slurpfile patched "$CLUSTER_FILE_0" >"$CLUSTER_FILE_1"
  mv "$CLUSTER_FILE_1" "$CLUSTER_FILE_0"

  # HACK convert the array of json objects to multiple yaml documents
  json_to_yaml "$CLUSTER_FILE_0" "$CLUSTER_FILE_1"
  mv "$CLUSTER_FILE_1" "$CLUSTER_FILE_0"

  # Done patching, overwrite original yaml CLUSTER_FILE
  mv "$CLUSTER_FILE_0" "$CLUSTER_FILE" # output is yaml

  # Clean up
  rm "$CLUSTER_FILE_JSON"
}

function yaml_to_json() {
  IN=${1}
  OUT=${2}
  kubectl patch -f "$IN" --local -p "{}" --type merge -o json | jq '.' -s >"$OUT"
}

function json_to_yaml() {
  IN=${1}
  OUT=${2}
  for ((i = 0; i < $(jq length "$IN"); i++)); do
    echo "---" >>"$OUT"
    jq ".[$i]" "$IN" | kubectl patch -f - --local -p "{}" --type merge -o yaml >>"$OUT"
  done
}
