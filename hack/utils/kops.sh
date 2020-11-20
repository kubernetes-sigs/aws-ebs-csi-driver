#!/bin/bash

set -uo pipefail

OS_ARCH=$(go env GOOS)-amd64

kops::install() {
  INSTALL_PATH=${1}
  KOPS_VERSION=${2}
  if [[ ! -e ${INSTALL_PATH}/kops ]]; then
    KOPS_DOWNLOAD_URL=https://github.com/kubernetes/kops/releases/download/v${KOPS_VERSION}/kops-${OS_ARCH}
    curl -L -X GET "${KOPS_DOWNLOAD_URL}" -o "${INSTALL_PATH}"/kops
    chmod +x "${INSTALL_PATH}"/kops
  fi
}
