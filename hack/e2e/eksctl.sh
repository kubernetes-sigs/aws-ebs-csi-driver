#!/bin/bash

set -euo pipefail

function eksctl_install() {
  INSTALL_PATH=${1}
  if [[ ! -e ${INSTALL_PATH}/eksctl ]]; then
    EKSCTL_DOWNLOAD_URL="https://github.com/weaveworks/eksctl/releases/latest/download/eksctl_$(uname -s)_amd64.tar.gz"
    curl --silent --location "${EKSCTL_DOWNLOAD_URL}" | tar xz -C "${INSTALL_PATH}"
    chmod +x "${INSTALL_PATH}"/eksctl
  fi
}

function eksctl_create_cluster() {
  SSH_KEY_PATH=${1}
  CLUSTER_NAME=${2}
  BIN=${3}
  ZONES=${4}
  INSTANCE_TYPE=${5}
  K8S_VERSION=${6}
  CLUSTER_FILE=${7}
  KUBECONFIG=${8}
  EKSCTL_PATCH_FILE=${9}

  generate_ssh_key "${SSH_KEY_PATH}"

  CLUSTER_NAME="${CLUSTER_NAME//./-}"

  if eksctl_cluster_exists "${CLUSTER_NAME}" "${BIN}"; then
    loudecho "Upgrading cluster $CLUSTER_NAME with $CLUSTER_FILE"
    ${BIN} upgrade cluster -f "${CLUSTER_FILE}"
  else
    loudecho "Creating cluster $CLUSTER_NAME with $CLUSTER_FILE (dry run)"
    ${BIN} create cluster \
      --managed \
      --ssh-access \
      --ssh-public-key "${SSH_KEY_PATH}".pub \
      --zones "${ZONES}" \
      --nodes=3 \
      --instance-types="${INSTANCE_TYPE}" \
      --version="${K8S_VERSION}" \
      --disable-pod-imds \
      --dry-run \
      "${CLUSTER_NAME}" > "${CLUSTER_FILE}"

    eksctl_patch_cluster_file "$CLUSTER_FILE" "$EKSCTL_PATCH_FILE"

    loudecho "Creating cluster $CLUSTER_NAME with $CLUSTER_FILE"
    ${BIN} create cluster -f "${CLUSTER_FILE}" --kubeconfig "${KUBECONFIG}"
  fi

  loudecho "Cluster ${CLUSTER_NAME} kubecfg written to ${KUBECONFIG}"

  loudecho "Getting cluster ${CLUSTER_NAME}"
  ${BIN} get cluster "${CLUSTER_NAME}"
  return $?
}

function eksctl_cluster_exists() {
  CLUSTER_NAME=${1}
  BIN=${2}
  set +e
  if ${BIN} get cluster "${CLUSTER_NAME}"; then
    set -e
    return 0
  else
    set -e
    return 1
  fi
}

function eksctl_delete_cluster() {
  BIN=${1}
  CLUSTER_NAME=${2}
  loudecho "Deleting cluster ${CLUSTER_NAME}"
  ${BIN} delete cluster "${CLUSTER_NAME}"
}

function eksctl_patch_cluster_file() {
  CLUSTER_FILE=${1}      # input must be yaml
  EKSCTL_PATCH_FILE=${2} # input must be yaml

  loudecho "Patching cluster $CLUSTER_NAME with $EKSCTL_PATCH_FILE"

  # Temporary intermediate files for patching
  CLUSTER_FILE_0=$CLUSTER_FILE.0
  CLUSTER_FILE_1=$CLUSTER_FILE.1

  cp "$CLUSTER_FILE" "$CLUSTER_FILE_0"

  # Patch only the Cluster
  kubectl patch -f "$CLUSTER_FILE_0" --local --type merge --patch "$(cat "$EKSCTL_PATCH_FILE")" -o yaml > "$CLUSTER_FILE_1"
  mv "$CLUSTER_FILE_1" "$CLUSTER_FILE_0"

  # Done patching, overwrite original CLUSTER_FILE
  mv "$CLUSTER_FILE_0" "$CLUSTER_FILE" # output is yaml
}
