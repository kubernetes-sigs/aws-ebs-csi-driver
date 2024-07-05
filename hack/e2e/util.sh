# Copyright 2024 The Kubernetes Authors.
#
# Licensed under the Apache License, Version 2.0 (the 'License');
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#    http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an 'AS IS' BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

#!/bin/bash

set -uo pipefail

function loudecho() {
  echo "###"
  echo "## ${1}"
  echo "#"
}

function install_driver() {
  if [[ ${DEPLOY_METHOD} == "helm" ]]; then
    HELM_ARGS=(upgrade --install aws-ebs-csi-driver
      "${BASE_DIR}/../../charts/aws-ebs-csi-driver"
      --namespace kube-system
      --set image.repository="${IMAGE_NAME}"
      --set image.tag="${IMAGE_TAG}"
      --set node.enableWindows="${WINDOWS}"
      --set node.windowsHostProcess="${WINDOWS_HOSTPROCESS}"
      --set=controller.k8sTagClusterId="${CLUSTER_NAME}"
      --timeout 10m0s
      --wait
      --kubeconfig "${KUBECONFIG}")
    if [ -n "${HELM_VALUES_FILE:-}" ]; then
      HELM_ARGS+=(-f "${HELM_VALUES_FILE}")
    fi
    eval "EXPANDED_HELM_EXTRA_FLAGS=$HELM_EXTRA_FLAGS"
    if [[ -n "$EXPANDED_HELM_EXTRA_FLAGS" ]]; then
      HELM_ARGS+=("${EXPANDED_HELM_EXTRA_FLAGS}")
    fi
    set -x
    "${BIN}/helm" "${HELM_ARGS[@]}"
    set +x
  elif [[ ${DEPLOY_METHOD} == "kustomize" ]]; then
    set -x
    kubectl --kubeconfig "${KUBECONFIG}" apply -k "${BASE_DIR}/../../deploy/kubernetes/overlays/stable"
    kubectl --kubeconfig "${KUBECONFIG}" --namespace kube-system wait --timeout 10m0s --for "condition=ready" pod -l "app.kubernetes.io/name=aws-ebs-csi-driver"
    set +x
  fi
}

function uninstall_driver() {
  if [[ ${DEPLOY_METHOD} == "helm" ]]; then
    ${BIN}/helm uninstall "aws-ebs-csi-driver" --namespace kube-system --kubeconfig "${KUBECONFIG}"
  elif [[ ${DEPLOY_METHOD} == "kustomize" ]]; then
    kubectl --kubeconfig "${KUBECONFIG}" delete -k "${BASE_DIR}/../../deploy/kubernetes/overlays/stable"
  fi
}
