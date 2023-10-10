#!/bin/bash

set -uo pipefail

function ct_install() {
  INSTALL_PATH=${1}
  CHART_TESTING_VERSION=${2}
  if [[ ! -e ${INSTALL_PATH}/chart-testing ]]; then
    CHART_TESTING_DOWNLOAD_URL="https://github.com/helm/chart-testing/releases/download/v${CHART_TESTING_VERSION}/chart-testing_${CHART_TESTING_VERSION}_linux_amd64.tar.gz"
    curl --silent --location "${CHART_TESTING_DOWNLOAD_URL}" | tar xz -C "${INSTALL_PATH}"
    chmod +x "${INSTALL_PATH}"/ct
  fi
  apt-get update && apt-get install -y yamllint
}
