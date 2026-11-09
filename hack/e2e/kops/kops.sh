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
  GOMPLATE_BIN=${13}
  KOPS_BPA_API_ACCESS=${14}
  AWS_BIN=${15}
  REGION=${16}
  declare -x KOPS_TOO_NEW_VERSION=true

  # The BPA exclusion makes the API load balancer reachable from the internet, so it is always paired with restricted access
  local network_flags=()
  if [[ -n "${KOPS_BPA_API_ACCESS}" ]]; then
    KOPS_BPA_API_ACCESS=${KOPS_BPA_API_ACCESS// /}
    local cidr
    for cidr in ${KOPS_BPA_API_ACCESS//,/ }; do
      if [[ "${cidr}" == "0.0.0.0/0" || "${cidr}" == "::/0" ]]; then
        echo "KOPS_BPA_API_ACCESS must not contain ${cidr}; it would expose the Kubernetes API and SSH to the internet" >&2
        return 1
      fi
    done
    network_flags+=(--topology private --admin-access "${KOPS_BPA_API_ACCESS}" --ssh-access "${KOPS_BPA_API_ACCESS}")
  fi

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
      ${network_flags[@]+"${network_flags[@]}"} \
      --dry-run \
      -o yaml \
      "${CLUSTER_NAME}" >"${CLUSTER_FILE}"

    if test -f "$KOPS_PATCH_FILE"; then
      loudecho "Templating $KOPS_PATCH_FILE to $KOPS_PATCH_FILE.processed"
      INSTANCE_TYPE="$INSTANCE_TYPE" \
        ${GOMPLATE_BIN} -f "$KOPS_PATCH_FILE" -o "$KOPS_PATCH_FILE.processed"
      kops_patch_cluster_file "$CLUSTER_FILE" "$KOPS_PATCH_FILE.processed" "Cluster" ""
    fi
    if test -f "$KOPS_PATCH_NODE_FILE"; then
      kops_patch_cluster_file "$CLUSTER_FILE" "$KOPS_PATCH_NODE_FILE" "InstanceGroup" "Node"
    fi

    loudecho "Creating cluster $CLUSTER_NAME with $CLUSTER_FILE"
    ${KOPS_BIN} create --state "${KOPS_STATE_FILE}" -f "${CLUSTER_FILE}"
  fi

  loudecho "Updating cluster $CLUSTER_NAME with $CLUSTER_FILE"
  ${KOPS_BIN} update cluster --state "${KOPS_STATE_FILE}" "${CLUSTER_NAME}" --yes

  if [[ -n "${KOPS_BPA_API_ACCESS}" ]]; then
    kops_create_bpa_exclusions "${CLUSTER_NAME}" "${AWS_BIN}" "${REGION}"
  fi

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
  KOPS_BPA_API_ACCESS=${4}
  AWS_BIN=${5}
  REGION=${6}

  # A failed exclusion cleanup must not leave the whole cluster running, so always delete the cluster
  local exclusion_cleanup_failed=0
  if [[ -n "${KOPS_BPA_API_ACCESS}" ]]; then
    if ! kops_delete_bpa_exclusions "${CLUSTER_NAME}" "${AWS_BIN}" "${REGION}"; then
      echo "WARNING: failed to remove VPC BPA exclusions for cluster ${CLUSTER_NAME}, deleting the cluster anyway" >&2
      exclusion_cleanup_failed=1
    fi
  fi
  loudecho "Deleting cluster ${CLUSTER_NAME}"
  ${KOPS_BIN} delete cluster --name "${CLUSTER_NAME}" --state "${KOPS_STATE_FILE}" --yes
  return ${exclusion_cleanup_failed}
}

# These helpers can run inside an `if` condition, where `set -e` does not apply, so every AWS call checks its own result

function kops_create_bpa_exclusions() {
  CLUSTER_NAME=${1}
  AWS_BIN=${2}
  REGION=${3}
  local subnets exclusions subnet
  subnets=$(${AWS_BIN} ec2 describe-subnets --region "${REGION}" \
    --filters "Name=tag:KubernetesCluster,Values=${CLUSTER_NAME}" "Name=tag:SubnetType,Values=Utility" \
    --query 'Subnets[].SubnetId' --output text) || return 1
  if [[ -z "${subnets}" ]]; then
    echo "No utility subnets found for cluster ${CLUSTER_NAME}" >&2
    return 1
  fi
  exclusions=$(kops_list_bpa_exclusions "${CLUSTER_NAME}" "${AWS_BIN}" "${REGION}") || return 1

  for subnet in ${subnets}; do
    if awk -v arn_suffix="subnet/${subnet}" '$2 !~ /^delete-/ && $3 ~ arn_suffix "$"' <<<"${exclusions}" | grep -q .; then
      loudecho "VPC BPA exclusion already exists for utility subnet ${subnet}"
      continue
    fi
    loudecho "Creating VPC BPA exclusion for utility subnet ${subnet}"
    ${AWS_BIN} ec2 create-vpc-block-public-access-exclusion --region "${REGION}" \
      --subnet-id "${subnet}" \
      --internet-gateway-exclusion-mode allow-bidirectional \
      --tag-specifications "ResourceType=vpc-block-public-access-exclusion,Tags=[{Key=KubernetesCluster,Value=${CLUSTER_NAME}}]" >/dev/null || return 1
  done

  kops_wait_bpa_exclusions "${CLUSTER_NAME}" "${AWS_BIN}" "${REGION}" "create" "$(wc -w <<<"${subnets}")"
}

function kops_delete_bpa_exclusions() {
  CLUSTER_NAME=${1}
  AWS_BIN=${2}
  REGION=${3}
  local exclusions exclusion
  exclusions=$(kops_list_bpa_exclusions "${CLUSTER_NAME}" "${AWS_BIN}" "${REGION}") || return 1
  # Skip exclusions that are already being deleted or are mid-transition, which the API refuses to delete
  for exclusion in $(awk '$2 !~ /^delete-/ && $2 !~ /-in-progress$/ {print $1}' <<<"${exclusions}"); do
    loudecho "Deleting VPC BPA exclusion ${exclusion}"
    ${AWS_BIN} ec2 delete-vpc-block-public-access-exclusion --region "${REGION}" --exclusion-id "${exclusion}" >/dev/null || return 1
  done

  kops_wait_bpa_exclusions "${CLUSTER_NAME}" "${AWS_BIN}" "${REGION}" "delete" 0
}

# Prints one "<exclusion-id> <state> <resource-arn>" line per exclusion tagged with the cluster name
function kops_list_bpa_exclusions() {
  CLUSTER_NAME=${1}
  AWS_BIN=${2}
  REGION=${3}
  ${AWS_BIN} ec2 describe-vpc-block-public-access-exclusions --region "${REGION}" --max-results 100 \
    --filters "Name=tag:KubernetesCluster,Values=${CLUSTER_NAME}" \
    --query 'VpcBlockPublicAccessExclusions[].[ExclusionId,State,ResourceArn]' --output text
}

# Waits until the expected number of exclusions are complete ("create") or none remain ("delete")
function kops_wait_bpa_exclusions() {
  CLUSTER_NAME=${1}
  AWS_BIN=${2}
  REGION=${3}
  OPERATION=${4}
  EXPECTED=${5}
  local exclusions
  for _ in $(seq 60); do
    exclusions=$(kops_list_bpa_exclusions "${CLUSTER_NAME}" "${AWS_BIN}" "${REGION}") || return 1
    if awk '$2 ~ /-failed$/' <<<"${exclusions}" | grep -q .; then
      echo "VPC BPA exclusion for cluster ${CLUSTER_NAME} failed: ${exclusions}" >&2
      return 1
    fi
    if [[ "${OPERATION}" == "create" ]]; then
      if (($(awk '$2 == "create-complete" || $2 == "update-complete"' <<<"${exclusions}" | grep -c .) >= EXPECTED)); then
        return 0
      fi
    elif ! awk '$2 != "delete-complete"' <<<"${exclusions}" | grep -q .; then
      return 0
    fi
    sleep 5
  done
  echo "Timed out waiting for VPC BPA exclusions for cluster ${CLUSTER_NAME} to ${OPERATION}" >&2
  return 1
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
  kubectl patch --kubeconfig /dev/null -f "$CLUSTER_FILE_0" --local --type merge --patch "$(cat "$KOPS_PATCH_FILE")" -o json >"$CLUSTER_FILE_1"
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
  kubectl patch --kubeconfig /dev/null -f "$IN" --local -p "{}" --type merge -o json | jq '.' -s >"$OUT"
}

function json_to_yaml() {
  IN=${1}
  OUT=${2}
  for ((i = 0; i < $(jq length "$IN"); i++)); do
    echo "---" >>"$OUT"
    jq ".[$i]" "$IN" | kubectl patch --kubeconfig /dev/null -f - --local -p "{}" --type merge -o yaml >>"$OUT"
  done
}
