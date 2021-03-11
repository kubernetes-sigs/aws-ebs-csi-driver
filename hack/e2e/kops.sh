#!/bin/bash

set -uo pipefail

OS_ARCH=$(go env GOOS)-amd64

BASE_DIR=$(dirname "$(realpath "${BASH_SOURCE[0]}")")
source "${BASE_DIR}"/util.sh

function kops_install() {
  INSTALL_PATH=${1}
  KOPS_VERSION=${2}
  if [[ ! -e ${INSTALL_PATH}/kops ]]; then
    KOPS_DOWNLOAD_URL=https://github.com/kubernetes/kops/releases/download/v${KOPS_VERSION}/kops-${OS_ARCH}
    curl -L -X GET "${KOPS_DOWNLOAD_URL}" -o "${INSTALL_PATH}"/kops
    chmod +x "${INSTALL_PATH}"/kops
  fi
}

function kops_create_cluster() {
  SSH_KEY_PATH=${1}
  KOPS_STATE_FILE=${2}
  CLUSTER_NAME=${3}
  KOPS_BIN=${4}
  ZONES=${5}
  INSTANCE_TYPE=${6}
  K8S_VERSION=${7}
  TEST_DIR=${8}
  KOPS_FEATURE_GATES_FILE=${10}
  KOPS_ADDITIONAL_POLICIES_FILE=${11}

  loudecho "Generating SSH key $SSH_KEY_PATH"
  if [[ ! -e ${SSH_KEY_PATH} ]]; then
    ssh-keygen -P csi-e2e -f "${SSH_KEY_PATH}"
  fi

  set +e
  if ${KOPS_BIN} get cluster --state "${KOPS_STATE_FILE}" "${CLUSTER_NAME}"; then
    set -e
    loudecho "Updating cluster $CLUSTER_NAME"
  else
    set -e
    loudecho "Creating cluster $CLUSTER_NAME"
    ${KOPS_BIN} create cluster --state "${KOPS_STATE_FILE}" \
      --zones "${ZONES}" \
      --node-count=3 \
      --node-size="${INSTANCE_TYPE}" \
      --kubernetes-version="${K8S_VERSION}" \
      --ssh-public-key="${SSH_KEY_PATH}".pub \
      "${CLUSTER_NAME}"
  fi

  CLUSTER_YAML_PATH=${TEST_DIR}/${CLUSTER_NAME}.yaml
  ${KOPS_BIN} get cluster --state "${KOPS_STATE_FILE}" "${CLUSTER_NAME}" -o yaml > "${CLUSTER_YAML_PATH}"
  [ -r "$KOPS_FEATURE_GATES_FILE" ] && cat "${KOPS_FEATURE_GATES_FILE}" >> "${CLUSTER_YAML_PATH}"
  [ -r "$KOPS_ADDITIONAL_POLICIES_FILE" ] && cat "${KOPS_ADDITIONAL_POLICIES_FILE}" >> "${CLUSTER_YAML_PATH}"
  ${KOPS_BIN} replace --state "${KOPS_STATE_FILE}" -f "${CLUSTER_YAML_PATH}"
  ${KOPS_BIN} update cluster --state "${KOPS_STATE_FILE}" "${CLUSTER_NAME}" --yes

  loudecho "Validating cluster $CLUSTER_NAME"
  ${KOPS_BIN} validate cluster --state "${KOPS_STATE_FILE}" --wait 10m
  return $?
}

function kops_delete_cluster() {
  KOPS_BIN=${1}
  CLUSTER_NAME=${2}
  KOPS_STATE_FILE=${3}
  loudecho "Deleting cluster ${CLUSTER_NAME}"
  ${KOPS_BIN} delete cluster --name "${CLUSTER_NAME}" --state "${KOPS_STATE_FILE}" --yes
}
