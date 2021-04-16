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
  KOPS_PATCH_FILE=${10}

  if [[ ! -e ${SSH_KEY_PATH} ]]; then
    loudecho "Generating SSH key $SSH_KEY_PATH"
    ssh-keygen -P csi-e2e -f "${SSH_KEY_PATH}"
  else
    loudecho "Reusing SSH key $SSH_KEY_PATH"
  fi

  CLUSTER_FILE=${TEST_DIR}/${CLUSTER_NAME}.json

  set +e
  if ${KOPS_BIN} get cluster --state "${KOPS_STATE_FILE}" "${CLUSTER_NAME}"; then
    set -e
    loudecho "Replacing cluster $CLUSTER_NAME with $CLUSTER_FILE"
    ${KOPS_BIN} replace --state "${KOPS_STATE_FILE}" -f "${CLUSTER_FILE}"
  else
    set -e
    loudecho "Creating cluster $CLUSTER_NAME with $CLUSTER_FILE (dry run)"
    ${KOPS_BIN} create cluster --state "${KOPS_STATE_FILE}" \
      --zones "${ZONES}" \
      --node-count=3 \
      --node-size="${INSTANCE_TYPE}" \
      --kubernetes-version="${K8S_VERSION}" \
      --dry-run \
      -o json \
      "${CLUSTER_NAME}" > "${CLUSTER_FILE}"

    kops_patch_cluster_file "$CLUSTER_FILE" "$KOPS_PATCH_FILE"

    loudecho "Creating cluster $CLUSTER_NAME with $CLUSTER_FILE"
    ${KOPS_BIN} create --state "${KOPS_STATE_FILE}" -f "${CLUSTER_FILE}"
  fi

  loudecho "Updating cluster $CLUSTER_NAME with $CLUSTER_FILE"
  ${KOPS_BIN} update cluster --state "${KOPS_STATE_FILE}" "${CLUSTER_NAME}" \
    --ssh-public-key="${SSH_KEY_PATH}".pub --yes

  ${KOPS_BIN} export kubecfg --state "${KOPS_STATE_FILE}" "${CLUSTER_NAME}" --admin

  loudecho "Validating cluster ${CLUSTER_NAME}"
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

# TODO switch this to python, all this hacking with jq stinks!
function kops_patch_cluster_file() {
  CLUSTER_FILE=${1}
  KOPS_PATCH_FILE=${2}

  loudecho "Patching cluster $CLUSTER_NAME with $KOPS_PATCH_FILE"

  # Temporary intermediate files for patching
  CLUSTER_FILE_0=$CLUSTER_FILE.0
  CLUSTER_FILE_1=$CLUSTER_FILE.1

  # Output is an array of Cluster and InstanceGroups
  jq '.[] | select(.kind=="Cluster")' "$CLUSTER_FILE" > "$CLUSTER_FILE_0"

  # Patch only the Cluster
  kubectl patch -f "$CLUSTER_FILE_0" --local --type merge --patch "$(cat "$KOPS_PATCH_FILE")" -o json > "$CLUSTER_FILE_1"
  mv "$CLUSTER_FILE_1" "$CLUSTER_FILE_0"

  # Write the patched Cluster back to the array
  jq '(.[] | select(.kind=="Cluster")) = $cluster[0]' "$CLUSTER_FILE" --slurpfile cluster "$CLUSTER_FILE_0" > "$CLUSTER_FILE_1"
  mv "$CLUSTER_FILE_1" "$CLUSTER_FILE_0"

  # HACK convert the json array to multiple yaml documents
  for ((i = 0; i < $(jq length "$CLUSTER_FILE_0"); i++)); do
    echo "---" >> "$CLUSTER_FILE_1"
    jq ".[$i]" "$CLUSTER_FILE_0" | kubectl patch -f - --local -p "{}" --type merge -o yaml >> "$CLUSTER_FILE_1"
  done
  mv "$CLUSTER_FILE_1" "$CLUSTER_FILE_0"

  # Done patching, overwrite original CLUSTER_FILE
  mv "$CLUSTER_FILE_0" "$CLUSTER_FILE"
}
