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

# This script builds and deploys the EBS CSI Driver and runs e2e tests
# CLUSTER_NAME and CLUSTER_TYPE are expected to be specified by the caller
# All other environment variables have default values (see config.sh) but
# many can be overridden on demand if needed

set -euo pipefail

BASE_DIR="$(dirname "$(realpath "${BASH_SOURCE[0]}")")"
BIN="${BASE_DIR}/../../bin"

source "${BASE_DIR}/config.sh"
source "${BASE_DIR}/util.sh"
source "${BASE_DIR}/ecr.sh"
source "${BASE_DIR}/metrics/metrics.sh"

## Setup

if [[ "${CLUSTER_TYPE}" == "kops" ]]; then
  HELM_VALUES_FILE="${BASE_DIR}/kops/values.yaml"
  K8S_VERSION="${K8S_VERSION_KOPS}"
elif [[ "${CLUSTER_TYPE}" == "eksctl" ]]; then
  HELM_VALUES_FILE="${BASE_DIR}/eksctl/values.yaml"
  K8S_VERSION="${K8S_VERSION_EKSCTL}"
else
  echo "Cluster type ${CLUSTER_TYPE} is invalid, must be kops or eksctl" >&2
  exit 1
fi

if [[ "$WINDOWS" == true ]]; then
  NODE_OS_DISTRO="windows"
else
  NODE_OS_DISTRO="linux"
fi

## Build image

if [[ "${CREATE_MISSING_ECR_REPO}" == true ]]; then
  REPO_CHECK=$(aws ecr describe-repositories --region "${AWS_REGION}")
  if [ $(jq '.repositories | map(.repositoryName) | index("aws-ebs-csi-driver")' <<<"${REPO_CHECK}") == "null" ]; then
    aws ecr create-repository --region "${AWS_REGION}" --repository-name aws-ebs-csi-driver >/dev/null
  fi
fi

ecr_build_and_push "${AWS_REGION}" \
  "${AWS_ACCOUNT_ID}" \
  "${IMAGE_NAME}" \
  "${IMAGE_TAG}" \
  "${IMAGE_ARCH}"

## Deploy

if [[ "${EBS_INSTALL_SNAPSHOT}" == true ]]; then
  loudecho "Applying snapshot controller and CRDs"
  kubectl apply --kubeconfig "${KUBECONFIG}" -f https://raw.githubusercontent.com/kubernetes-csi/external-snapshotter/"${EBS_INSTALL_SNAPSHOT_VERSION}"/deploy/kubernetes/snapshot-controller/rbac-snapshot-controller.yaml
  kubectl apply --kubeconfig "${KUBECONFIG}" -f https://raw.githubusercontent.com/kubernetes-csi/external-snapshotter/"${EBS_INSTALL_SNAPSHOT_VERSION}"/deploy/kubernetes/snapshot-controller/setup-snapshot-controller.yaml
  kubectl apply --kubeconfig "${KUBECONFIG}" -f https://raw.githubusercontent.com/kubernetes-csi/external-snapshotter/"${EBS_INSTALL_SNAPSHOT_VERSION}"/client/config/crd/snapshot.storage.k8s.io_volumesnapshotclasses.yaml
  kubectl apply --kubeconfig "${KUBECONFIG}" -f https://raw.githubusercontent.com/kubernetes-csi/external-snapshotter/"${EBS_INSTALL_SNAPSHOT_VERSION}"/client/config/crd/snapshot.storage.k8s.io_volumesnapshotcontents.yaml
  kubectl apply --kubeconfig "${KUBECONFIG}" -f https://raw.githubusercontent.com/kubernetes-csi/external-snapshotter/"${EBS_INSTALL_SNAPSHOT_VERSION}"/client/config/crd/snapshot.storage.k8s.io_volumesnapshots.yaml
fi

if [[ "${HELM_CT_TEST}" != true ]]; then
  startSec=$(date +'%s')

  if [[ ${DEPLOY_METHOD} == "helm" ]]; then
    HELM_ARGS=(upgrade --install "aws-ebs-csi-driver"
      --namespace kube-system
      --set image.repository="${IMAGE_NAME}"
      --set image.tag="${IMAGE_TAG}"
      --set node.enableWindows="${WINDOWS}"
      --set node.windowsHostProcess="${WINDOWS_HOSTPROCESS}"
      --set=controller.k8sTagClusterId="${CLUSTER_NAME}"
      --timeout 10m0s
      --wait
      --kubeconfig "${KUBECONFIG}"
      ./charts/aws-ebs-csi-driver)
    if [[ -f "$HELM_VALUES_FILE" ]]; then
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
    kubectl --kubeconfig "${KUBECONFIG}" apply -k "./deploy/kubernetes/overlays/stable"
    kubectl --kubeconfig "${KUBECONFIG}" --namespace kube-system wait --timeout 10m0s --for "condition=ready" pod -l "app.kubernetes.io/name=aws-ebs-csi-driver"
    set +x
  fi

  endSec=$(date +'%s')
  deployTimeSeconds=$(((endSec - startSec) / 1))
  loudecho "Driver deployment complete, time used: $deployTimeSeconds seconds"
fi

## Run tests

if [[ "${HELM_CT_TEST}" == true ]]; then
  loudecho "Test and lint Helm chart with chart-testing"
  if [ -n "${PROW_JOB_ID:-}" ]; then
    # Prow-specific setup
    # Required becuase chart_testing ALWAYS needs a remote
    git remote add ct https://github.com/kubernetes-sigs/aws-ebs-csi-driver.git
    git fetch ct "${PULL_BASE_REF}"
    export CT_REMOTE="ct"
    export CT_TARGET_BRANCH="${PULL_BASE_REF}"
  fi
  set -x
  set +e

  (
    while true; do
      if kubectl get pod ebs-csi-driver-test -n kube-system --kubeconfig "${KUBECONFIG}" &>/dev/null; then
        echo "Pod found, waiting for it to become ready..."
        if kubectl wait --for=condition=ready pod ebs-csi-driver-test -n kube-system --timeout=300s --kubeconfig "${KUBECONFIG}"; then
          echo "Pod is ready, fetching logs..."
          kubectl logs -f ebs-csi-driver-test -n kube-system -c kubetest2 --kubeconfig "${KUBECONFIG}"
        fi
      fi
      sleep 30
    done
  ) &
  LOG_STREAM_PID=$!

  KUBECONFIG="$KUBECONFIG" PATH="${BIN}:${PATH}" "${BIN}/ct" lint-and-install \
    --config="${BASE_DIR}/../../tests/ct-config.yaml" \
    --helm-extra-set-args="--set=image.repository=${IMAGE_NAME},image.tag=${IMAGE_TAG},node.tolerateAllTaints=false"
  TEST_PASSED=$?

  if kill -0 $LOG_STREAM_PID 2>/dev/null; then
    kill $LOG_STREAM_PID
  fi

  set -e
  set +x
else
  loudecho "Testing focus ${GINKGO_FOCUS}"

  if [[ $TEST_PATH == "./tests/e2e-kubernetes/..." ]]; then
    pushd "${BASE_DIR}/../../tests/e2e-kubernetes"
    packageVersion=$(echo $(cut -d '.' -f 1,2 <<<$K8S_VERSION))

    set -x
    set +e
    "${BIN}/kubetest2" noop \
      --run-id="e2e-kubernetes" \
      --test=ginkgo \
      -- \
      --skip-regex="${GINKGO_SKIP}" \
      --focus-regex="${GINKGO_FOCUS}" \
      --test-package-version=$(curl -L https://dl.k8s.io/release/stable-${packageVersion}.txt) \
      --parallel=${GINKGO_PARALLEL} \
      --test-args="-storage.testdriver=${PWD}/manifests.yaml -kubeconfig=${KUBECONFIG} -node-os-distro=${NODE_OS_DISTRO}"
    TEST_PASSED=$?
    set -e
    set +x
    popd
  fi

  if [[ $TEST_PATH == "./tests/e2e/..." ]]; then
    set -x
    set +e
    "${BIN}/ginkgo" -p -nodes="${GINKGO_PARALLEL}" -v \
      --focus="${GINKGO_FOCUS}" \
      --skip="${GINKGO_SKIP}" \
      "${BASE_DIR}/../../tests/e2e/..." \
      -- \
      -kubeconfig="${KUBECONFIG}" \
      -report-dir="${TEST_DIR}/artifacts" \
      -gce-zone="${FIRST_ZONE}"
    TEST_PASSED=$?
    set -e
    set +x
  fi

  PODS=$(kubectl get pod -n kube-system -l "app.kubernetes.io/name=aws-ebs-csi-driver,app.kubernetes.io/instance=aws-ebs-csi-driver" -o json --kubeconfig "${KUBECONFIG}" | jq -r .items[].metadata.name)

  while IFS= read -r POD; do
    loudecho "Printing pod ${POD} container logs"
    set +e
    kubectl logs "${POD}" -n kube-system --all-containers --ignore-errors --kubeconfig "${KUBECONFIG}"
    set -e
  done <<<"${PODS}"
fi

if [[ "${COLLECT_METRICS}" == true ]]; then
  metrics_collector "$KUBECONFIG" \
    "$AWS_ACCOUNT_ID" \
    "$AWS_REGION" \
    "$NODE_OS_DISTRO" \
    "$deployTimeSeconds" \
    "aws-ebs-csi-driver" \
    "$VERSION"
fi

## Cleanup

if [[ "${HELM_CT_TEST}" != true ]]; then
  loudecho "Removing driver via ${DEPLOY_METHOD}"
  if [[ ${DEPLOY_METHOD} == "helm" ]]; then
    ${BIN}/helm del "aws-ebs-csi-driver" \
      --namespace kube-system \
      --kubeconfig "${KUBECONFIG}"
  elif [[ ${DEPLOY_METHOD} == "kustomize" ]]; then
    kubectl --kubeconfig "${KUBECONFIG}" delete -k "${BASE_DIR}/../../deploy/kubernetes/overlays/stable"
  fi
fi

if [[ "${EBS_INSTALL_SNAPSHOT}" == true ]]; then
  loudecho "Removing snapshot controller and CRDs"
  kubectl delete --kubeconfig "${KUBECONFIG}" -f https://raw.githubusercontent.com/kubernetes-csi/external-snapshotter/"${EBS_INSTALL_SNAPSHOT_VERSION}"/deploy/kubernetes/snapshot-controller/rbac-snapshot-controller.yaml
  kubectl delete --kubeconfig "${KUBECONFIG}" -f https://raw.githubusercontent.com/kubernetes-csi/external-snapshotter/"${EBS_INSTALL_SNAPSHOT_VERSION}"/deploy/kubernetes/snapshot-controller/setup-snapshot-controller.yaml
  kubectl delete --kubeconfig "${KUBECONFIG}" -f https://raw.githubusercontent.com/kubernetes-csi/external-snapshotter/"${EBS_INSTALL_SNAPSHOT_VERSION}"/client/config/crd/snapshot.storage.k8s.io_volumesnapshotclasses.yaml
  kubectl delete --kubeconfig "${KUBECONFIG}" -f https://raw.githubusercontent.com/kubernetes-csi/external-snapshotter/"${EBS_INSTALL_SNAPSHOT_VERSION}"/client/config/crd/snapshot.storage.k8s.io_volumesnapshotcontents.yaml
  kubectl delete --kubeconfig "${KUBECONFIG}" -f https://raw.githubusercontent.com/kubernetes-csi/external-snapshotter/"${EBS_INSTALL_SNAPSHOT_VERSION}"/client/config/crd/snapshot.storage.k8s.io_volumesnapshots.yaml
fi

## Output result

loudecho "TEST_PASSED: ${TEST_PASSED}"
if [[ $TEST_PASSED -ne 0 ]]; then
  loudecho "FAIL!"
  exit 1
else
  loudecho "SUCCESS!"
fi
