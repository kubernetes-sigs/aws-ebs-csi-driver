#!/bin/bash

# Copyright 2019 The Kubernetes Authors.
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

set -euo pipefail

BASE_DIR=$(dirname "$(realpath "${BASH_SOURCE[0]}")")
source "${BASE_DIR}"/ecr.sh
source "${BASE_DIR}"/eksctl.sh
source "${BASE_DIR}"/helm.sh
source "${BASE_DIR}"/kops.sh
source "${BASE_DIR}"/util.sh
source "${BASE_DIR}"/chart-testing.sh
source "${BASE_DIR}"/metrics/metrics.sh

DRIVER_NAME=${DRIVER_NAME:-aws-ebs-csi-driver}
CONTAINER_NAME=${CONTAINER_NAME:-ebs-plugin}

TEST_ID=${TEST_ID:-$RANDOM}
CLUSTER_NAME=test-cluster-${TEST_ID}.k8s.local
CLUSTER_TYPE=${CLUSTER_TYPE:-kops}

TEST_DIR=${BASE_DIR}/csi-test-artifacts
BIN_DIR=${TEST_DIR}/bin
CLUSTER_FILE=${TEST_DIR}/${CLUSTER_NAME}.${CLUSTER_TYPE}.yaml
KUBECONFIG=${KUBECONFIG:-"${TEST_DIR}/${CLUSTER_NAME}.${CLUSTER_TYPE}.kubeconfig"}

REGION=${AWS_REGION:-us-west-2}
ZONES=${AWS_AVAILABILITY_ZONES:-us-west-2a,us-west-2b,us-west-2c}
FIRST_ZONE=$(echo "${ZONES}" | cut -d, -f1)
NODE_COUNT=${NODE_COUNT:-3}
INSTANCE_TYPE=${INSTANCE_TYPE:-c5.large}

AWS_ACCOUNT_ID=$(aws sts get-caller-identity --query Account --output text)
IMAGE_NAME=${IMAGE_NAME:-${AWS_ACCOUNT_ID}.dkr.ecr.${REGION}.amazonaws.com/${DRIVER_NAME}}
IMAGE_TAG=${IMAGE_TAG:-${TEST_ID}}
IMAGE_ARCH=${IMAGE_ARCH:-amd64}

# kops: must include patch version (e.g. 1.19.1)
# eksctl: mustn't include patch version (e.g. 1.19)
K8S_VERSION_KOPS=${K8S_VERSION_KOPS:-${K8S_VERSION:-1.28.3}}
K8S_VERSION_EKSCTL=${K8S_VERSION_EKSCTL:-${K8S_VERSION:-1.28}}

KOPS_VERSION=${KOPS_VERSION:-1.28.0}
KOPS_STATE_FILE=${KOPS_STATE_FILE:-s3://k8s-kops-csi-shared-e2e}
KOPS_PATCH_FILE=${KOPS_PATCH_FILE:-./hack/kops-patch.yaml}
KOPS_PATCH_NODE_FILE=${KOPS_PATCH_NODE_FILE:-./hack/kops-patch-node.yaml}
AMI_PARAMETER=${AMI_PARAMETER:-/aws/service/ami-amazon-linux-latest/al2023-ami-kernel-default-x86_64}
AMI_ID=$(aws ssm get-parameters --names ${AMI_PARAMETER} --region ${REGION} --query 'Parameters[0].Value' --output text)

EKSCTL_VERSION=${EKSCTL_VERSION:-0.163.0}
EKSCTL_PATCH_FILE=${EKSCTL_PATCH_FILE:-./hack/eksctl-patch.yaml}
VPC_CONFIGMAP_FILE=${VPC_CONFIGMAP_FILE:-./hack/vpc-resource-controller-configmap.yaml}
EKSCTL_ADMIN_ROLE=${EKSCTL_ADMIN_ROLE:-}
# Creates a windows node group.
WINDOWS=${WINDOWS:-"false"}

# Valid deploy methods: "helm" (default), "kustomize"
DEPLOY_METHOD=${DEPLOY_METHOD:-"helm"}

HELM_VALUES_FILE=${HELM_VALUES_FILE:-./hack/values.yaml}
HELM_EXTRA_FLAGS=${HELM_EXTRA_FLAGS:-}

TEST_PATH=${TEST_PATH:-"./tests/e2e/..."}
ARTIFACTS=${ARTIFACTS:-"${TEST_DIR}/artifacts"}
GINKGO_FOCUS=${GINKGO_FOCUS:-"\[ebs-csi-e2e\]"}
GINKGO_SKIP=${GINKGO_SKIP:-"\[Disruptive\]"}
GINKGO_NODES=${GINKGO_NODES:-4}
GINKGO_PARALLEL=${GINKGO_PARALLEL:-25}
NODE_OS_DISTRO=${NODE_OS_DISTRO:-"linux"}
TEST_EXTRA_FLAGS=${TEST_EXTRA_FLAGS:-}

EBS_INSTALL_SNAPSHOT=${EBS_INSTALL_SNAPSHOT:-"false"}
# https://github.com/kubernetes-csi/external-snapshotter
EBS_INSTALL_SNAPSHOT_VERSION=${EBS_INSTALL_SNAPSHOT_VERSION:-"v6.3.1"}

HELM_CT_TEST=${HELM_CT_TEST:-"false"}
# https://github.com/helm/chart-testing
CHART_TESTING_VERSION=${CHART_TESTING_VERSION:-3.8.0}
CLEAN=${CLEAN:-"true"}

COLLECT_METRICS=${COLLECT_METRICS:-"false"}

loudecho "Testing in region ${REGION} and zones ${ZONES}"
mkdir -p "${BIN_DIR}"
export PATH=${PATH}:${BIN_DIR}

if [[ "${CLUSTER_TYPE}" == "kops" ]]; then
  loudecho "Installing kops ${KOPS_VERSION} to ${BIN_DIR}"
  kops_install "${BIN_DIR}" "${KOPS_VERSION}"
  KOPS_BIN=${BIN_DIR}/kops
elif [[ "${CLUSTER_TYPE}" == "eksctl" ]]; then
  loudecho "Installing eksctl ${EKSCTL_VERSION} to ${BIN_DIR}"
  eksctl_install "${BIN_DIR}" "${EKSCTL_VERSION}"
  EKSCTL_BIN=${BIN_DIR}/eksctl
else
  loudecho "${CLUSTER_TYPE} must be kops or eksctl!"
  exit 1
fi

loudecho "Installing helm to ${BIN_DIR}"
helm_install "${BIN_DIR}"
HELM_BIN=${BIN_DIR}/helm

if [[ "${HELM_CT_TEST}" == true ]]; then
  loudecho "Installing chart-testing ${CHART_TESTING_VERSION} to ${BIN_DIR}"
  ct_install "${BIN_DIR}" "${CHART_TESTING_VERSION}"
  CHART_TESTING_BIN=${BIN_DIR}/ct
else
  loudecho "Installing ginkgo to ${BIN_DIR}"
  GINKGO_BIN=${BIN_DIR}/ginkgo
  if [[ ! -e ${GINKGO_BIN} ]]; then
    pushd /tmp
    GOPATH=${TEST_DIR} GOBIN=${BIN_DIR} go install github.com/onsi/ginkgo/v2/ginkgo@v2.11.0
    popd
    ginkgo version
  fi
  loudecho "Installing kubetest2 to ${BIN_DIR}"
  KUBETEST2_BIN=${BIN_DIR}/kubetest2
  if [[ ! -e ${KUBETEST2_BIN} ]]; then
    pushd /tmp
    GOPATH=${TEST_DIR} GOBIN=${BIN_DIR} go install sigs.k8s.io/kubetest2/...@latest
    popd
  fi
fi

ecr_build_and_push "${REGION}" \
  "${AWS_ACCOUNT_ID}" \
  "${IMAGE_NAME}" \
  "${IMAGE_TAG}" \
  "${IMAGE_ARCH}"

if [[ "${CLUSTER_TYPE}" == "kops" ]]; then
  kops_create_cluster \
    "$CLUSTER_NAME" \
    "$KOPS_BIN" \
    "$ZONES" \
    "$NODE_COUNT" \
    "$INSTANCE_TYPE" \
    "$AMI_ID" \
    "$K8S_VERSION_KOPS" \
    "$CLUSTER_FILE" \
    "$KUBECONFIG" \
    "$KOPS_PATCH_FILE" \
    "$KOPS_PATCH_NODE_FILE" \
    "$KOPS_STATE_FILE"
  if [[ $? -ne 0 ]]; then
    exit 1
  fi
elif [[ "${CLUSTER_TYPE}" == "eksctl" ]]; then
  eksctl_create_cluster \
    "$CLUSTER_NAME" \
    "$EKSCTL_BIN" \
    "$ZONES" \
    "$INSTANCE_TYPE" \
    "$K8S_VERSION_EKSCTL" \
    "$CLUSTER_FILE" \
    "$KUBECONFIG" \
    "$EKSCTL_PATCH_FILE" \
    "$EKSCTL_ADMIN_ROLE" \
    "$WINDOWS" \
    "$VPC_CONFIGMAP_FILE"
  if [[ $? -ne 0 ]]; then
    exit 1
  fi
fi

if [[ "${EBS_INSTALL_SNAPSHOT}" == true ]]; then
  loudecho "Installing snapshot controller and CRDs"
  kubectl apply --kubeconfig "${KUBECONFIG}" -f https://raw.githubusercontent.com/kubernetes-csi/external-snapshotter/"${EBS_INSTALL_SNAPSHOT_VERSION}"/deploy/kubernetes/snapshot-controller/rbac-snapshot-controller.yaml
  kubectl apply --kubeconfig "${KUBECONFIG}" -f https://raw.githubusercontent.com/kubernetes-csi/external-snapshotter/"${EBS_INSTALL_SNAPSHOT_VERSION}"/deploy/kubernetes/snapshot-controller/setup-snapshot-controller.yaml
  kubectl apply --kubeconfig "${KUBECONFIG}" -f https://raw.githubusercontent.com/kubernetes-csi/external-snapshotter/"${EBS_INSTALL_SNAPSHOT_VERSION}"/client/config/crd/snapshot.storage.k8s.io_volumesnapshotclasses.yaml
  kubectl apply --kubeconfig "${KUBECONFIG}" -f https://raw.githubusercontent.com/kubernetes-csi/external-snapshotter/"${EBS_INSTALL_SNAPSHOT_VERSION}"/client/config/crd/snapshot.storage.k8s.io_volumesnapshotcontents.yaml
  kubectl apply --kubeconfig "${KUBECONFIG}" -f https://raw.githubusercontent.com/kubernetes-csi/external-snapshotter/"${EBS_INSTALL_SNAPSHOT_VERSION}"/client/config/crd/snapshot.storage.k8s.io_volumesnapshots.yaml
fi

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
  export KUBECONFIG="${KUBECONFIG}"
  ${CHART_TESTING_BIN} lint-and-install --config ${PWD}/tests/ct-config.yaml --helm-extra-set-args="--set=image.repository=${IMAGE_NAME},image.tag=${IMAGE_TAG},node.tolerateAllTaints=false"
  TEST_PASSED=$?
  set -e
  set +x
else
  loudecho "Deploying driver via ${DEPLOY_METHOD}"
  startSec=$(date +'%s')

  if [[ ${DEPLOY_METHOD} == "helm" ]]; then
    HELM_ARGS=(upgrade --install "${DRIVER_NAME}"
      --namespace kube-system
      --set image.repository="${IMAGE_NAME}"
      --set image.tag="${IMAGE_TAG}"
      --set node.enableWindows="${WINDOWS}"
      --timeout 10m0s
      --wait
      --kubeconfig "${KUBECONFIG}"
      ./charts/"${DRIVER_NAME}")
    if [[ -f "$HELM_VALUES_FILE" ]]; then
      HELM_ARGS+=(-f "${HELM_VALUES_FILE}")
    fi
    eval "EXPANDED_HELM_EXTRA_FLAGS=$HELM_EXTRA_FLAGS"
    if [[ -n "$EXPANDED_HELM_EXTRA_FLAGS" ]]; then
      HELM_ARGS+=("${EXPANDED_HELM_EXTRA_FLAGS}")
    fi
    set -x
    "${HELM_BIN}" "${HELM_ARGS[@]}"
    set +x
  elif [[ ${DEPLOY_METHOD} == "kustomize" ]]; then
    kubectl --kubeconfig "${KUBECONFIG}" apply -k "./deploy/kubernetes/overlays/stable"
    kubectl --kubeconfig "${KUBECONFIG}" --namespace kube-system wait --timeout 10m0s --for "condition=ready" pod -l "app.kubernetes.io/name=aws-ebs-csi-driver"
  fi

  endSec=$(date +'%s')
  deployTimeSeconds=$(((endSec - startSec) / 1))
  loudecho "Driver deployment complete, time used: $deployTimeSeconds seconds"
  loudecho "Testing focus ${GINKGO_FOCUS}"

  if [[ $TEST_PATH == "./tests/e2e-kubernetes/..." ]]; then
    pushd ${PWD}/tests/e2e-kubernetes
    packageVersion=$(echo $(cut -d '.' -f 1,2 <<< $K8S_VERSION))

    set -x
    set +e
    kubetest2 noop \
      --run-id="e2e-kubernetes" \
      --test=ginkgo \
      -- \
      --skip-regex="${GINKGO_SKIP}" \
      --focus-regex="${GINKGO_FOCUS}" \
      --test-package-version=$(curl -L https://dl.k8s.io/release/stable-$packageVersion.txt) \
      --parallel=${GINKGO_PARALLEL} \
      --test-args="-storage.testdriver=${PWD}/manifests.yaml -kubeconfig=$KUBECONFIG -node-os-distro=${NODE_OS_DISTRO}"

    TEST_PASSED=$?
    set -e
    set +x
    popd
  fi

  if [[ $TEST_PATH == "./tests/e2e/..." ]]; then
    eval "EXPANDED_TEST_EXTRA_FLAGS=$TEST_EXTRA_FLAGS"
    set -x
    set +e
    ${GINKGO_BIN} -p -nodes="${GINKGO_NODES}" -v --focus="${GINKGO_FOCUS}" --skip="${GINKGO_SKIP}" "${TEST_PATH}" -- -kubeconfig="${KUBECONFIG}" -report-dir="${ARTIFACTS}" -gce-zone="${FIRST_ZONE}" "${EXPANDED_TEST_EXTRA_FLAGS}"
    TEST_PASSED=$?
    set -e
    set +x
  fi

  PODS=$(kubectl get pod -n kube-system -l "app.kubernetes.io/name=${DRIVER_NAME},app.kubernetes.io/instance=${DRIVER_NAME}" -o json --kubeconfig "${KUBECONFIG}" | jq -r .items[].metadata.name)

  while IFS= read -r POD; do
    loudecho "Printing pod ${POD} ${CONTAINER_NAME} container logs"
    set +e
    kubectl logs "${POD}" -n kube-system "${CONTAINER_NAME}" \
      --kubeconfig "${KUBECONFIG}"
    set -e
  done <<< "${PODS}"
fi

OVERALL_TEST_PASSED="${TEST_PASSED}"

if [[ "${COLLECT_METRICS}" == true ]]; then
  metrics_collector "$KUBECONFIG" \
    "$AWS_ACCOUNT_ID" \
    "$AWS_REGION" \
    "$NODE_OS_DISTRO" \
    "$deployTimeSeconds" \
    "$DRIVER_NAME" \
    "$VERSION"
fi

if [[ "${CLEAN}" == true ]]; then
  loudecho "Cleaning"

  if [[ "${HELM_CT_TEST}" != true ]]; then
    loudecho "Removing driver via ${DEPLOY_METHOD}"
    if [[ ${DEPLOY_METHOD} == "helm" ]]; then
      ${HELM_BIN} del "${DRIVER_NAME}" \
        --namespace kube-system \
        --kubeconfig "${KUBECONFIG}"
    elif [[ ${DEPLOY_METHOD} == "kustomize" ]]; then
      kubectl --kubeconfig "${KUBECONFIG}" delete -k "./deploy/kubernetes/overlays/stable"
    fi
  fi

  if [[ "${CLUSTER_TYPE}" == "kops" ]]; then
    kops_delete_cluster \
      "${KOPS_BIN}" \
      "${CLUSTER_NAME}" \
      "${KOPS_STATE_FILE}"
  elif [[ "${CLUSTER_TYPE}" == "eksctl" ]]; then
    eksctl_delete_cluster \
      "${EKSCTL_BIN}" \
      "${CLUSTER_NAME}"
  fi
else
  loudecho "Not cleaning"
fi

loudecho "OVERALL_TEST_PASSED: ${OVERALL_TEST_PASSED}"
if [[ $OVERALL_TEST_PASSED -ne 0 ]]; then
  loudecho "FAIL!"
  exit 1
else
  loudecho "SUCCESS!"
fi
