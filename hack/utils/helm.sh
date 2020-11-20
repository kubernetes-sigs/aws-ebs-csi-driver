#!/bin/bash

set -uo pipefail

helm::install() {
  INSTALL_PATH=${1}
  if [[ ! -e ${INSTALL_PATH}/helm ]]; then
    curl -fsSL -o get_helm.sh https://raw.githubusercontent.com/helm/helm/master/scripts/get-helm-3
    chmod 700 get_helm.sh
    export USE_SUDO=false
    export HELM_INSTALL_DIR=${INSTALL_PATH}
    ./get_helm.sh
    rm get_helm.sh
  fi
}
