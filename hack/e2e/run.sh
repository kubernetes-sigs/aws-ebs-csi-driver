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
source "${BASE_DIR}"/ebs.sh
source "${BASE_DIR}"/ecr.sh
source "${BASE_DIR}"/helm.sh
source "${BASE_DIR}"/kops.sh
source "${BASE_DIR}"/util.sh

DRIVER_NAME=${DRIVER_NAME:-aws-ebs-csi-driver}

TEST_ID=${TEST_ID:-$RANDOM}
CLUSTER_NAME=test-cluster-${TEST_ID}.k8s.local

TEST_DIR=${BASE_DIR}/csi-test-artifacts
BIN_DIR=${TEST_DIR}/bin
SSH_KEY_PATH=${TEST_DIR}/id_rsa

REGION=${AWS_REGION:-us-west-2}
ZONES=${AWS_AVAILABILITY_ZONES:-us-west-2a,us-west-2b,us-west-2c}
INSTANCE_TYPE=${INSTANCE_TYPE:-c4.large}

AWS_ACCOUNT_ID=$(aws sts get-caller-identity --query Account --output text)
IMAGE_NAME=${IMAGE_NAME:-${AWS_ACCOUNT_ID}.dkr.ecr.${REGION}.amazonaws.com/${DRIVER_NAME}}
IMAGE_TAG=${IMAGE_TAG:-${TEST_ID}}

K8S_VERSION=${K8S_VERSION:-1.18.10}
KOPS_VERSION=${KOPS_VERSION:-1.18.2}
KOPS_STATE_FILE=${KOPS_STATE_FILE:-s3://k8s-kops-csi-e2e}
KOPS_FEATURE_GATES_FILE=${KOPS_FEATURE_GATES_FILE:-./hack/feature-gates.yaml}
KOPS_ADDITIONAL_POLICIES_FILE=${KOPS_ADDITIONAL_POLICIES_FILE:-./hack/additional-policies.yaml}

TEST_PATH=${TEST_PATH:-"./tests/e2e/..."}
KUBECONFIG=${KUBECONFIG:-"${HOME}/.kube/config"}
ARTIFACTS=${ARTIFACTS:-"${TEST_DIR}/artifacts"}
GINKGO_FOCUS=${GINKGO_FOCUS:-"\[ebs-csi-e2e\]"}
GINKGO_SKIP=${GINKGO_SKIP:-"\[Disruptive\]"}
GINKGO_NODES=${GINKGO_NODES:-4}
TEST_EXTRA_FLAGS=${TEST_EXTRA_FLAGS:-}

EBS_CHECK_MIGRATION=${EBS_CHECK_MIGRATION:-"false"}

CLEAN=${CLEAN:-"true"}

loudecho "Testing in region ${REGION} and zones ${ZONES}"
mkdir -p "${BIN_DIR}"
export PATH=${PATH}:${BIN_DIR}

loudecho "Installing kops ${KOPS_VERSION} to ${BIN_DIR}"
kops_install "${BIN_DIR}" "${KOPS_VERSION}"
KOPS_BIN=${BIN_DIR}/kops

loudecho "Installing helm to ${BIN_DIR}"
helm_install "${BIN_DIR}"
HELM_BIN=${BIN_DIR}/helm

loudecho "Installing ginkgo to ${BIN_DIR}"
GINKGO_BIN=${BIN_DIR}/ginkgo
if [[ ! -e ${GINKGO_BIN} ]]; then
  pushd /tmp
  GOPATH=${TEST_DIR} GOBIN=${BIN_DIR} GO111MODULE=on go get github.com/onsi/ginkgo/ginkgo@v1.12.0
  popd
fi

ecr_build_and_push "${REGION}" \
  "${AWS_ACCOUNT_ID}" \
  "${IMAGE_NAME}" \
  "${IMAGE_TAG}"

kops_create_cluster \
  "$SSH_KEY_PATH" \
  "$KOPS_STATE_FILE" \
  "$CLUSTER_NAME" \
  "$KOPS_BIN" \
  "$ZONES" \
  "$INSTANCE_TYPE" \
  "$K8S_VERSION" \
  "$TEST_DIR" \
  "$BASE_DIR" \
  "$KOPS_FEATURE_GATES_FILE" \
  "$KOPS_ADDITIONAL_POLICIES_FILE"
if [[ $? -ne 0 ]]; then
  exit 1
fi

loudecho "Deploying driver"
${HELM_BIN} upgrade --install ${DRIVER_NAME} \
  --namespace kube-system \
  --set enableVolumeScheduling=true \
  --set enableVolumeResizing=true \
  --set enableVolumeSnapshot=true \
  --set image.repository="${IMAGE_NAME}" \
  --set image.tag="${IMAGE_TAG}" \
  ./charts/${DRIVER_NAME}

loudecho "Testing focus ${GINKGO_FOCUS}"
eval "EXPANDED_TEST_EXTRA_FLAGS=$TEST_EXTRA_FLAGS"
set -x
${GINKGO_BIN} -p -nodes="${GINKGO_NODES}" -v --focus="${GINKGO_FOCUS}" --skip="${GINKGO_SKIP}" "${TEST_PATH}" -- -kubeconfig="${KUBECONFIG}" -report-dir="${ARTIFACTS}" -gce-zone="${ZONES%,*}" "${EXPANDED_TEST_EXTRA_FLAGS}"
TEST_PASSED=$?
set +x
loudecho "TEST_PASSED: ${TEST_PASSED}"

OVERALL_TEST_PASSED="${TEST_PASSED}"
if [[ "${EBS_CHECK_MIGRATION}" == true ]]; then
  OUTPUT=$(ebs_check_migration)
  echo "${OUTPUT}"
  MIGRATION_PASSED=$(echo "${OUTPUT}" | tail -1)
  loudecho "MIGRATION_PASSED: ${MIGRATION_PASSED}"
  if [ "${TEST_PASSED}" == 0 ] && [ "${MIGRATION_PASSED}" == 0 ]; then
    loudecho "Both test and migration passed"
    OVERALL_TEST_PASSED=0
  else
    loudecho "One of test or migration failed"
    OVERALL_TEST_PASSED=1
  fi
fi

if [[ "${CLEAN}" == true ]]; then
  loudecho "Cleaning"

  loudecho "Removing driver"
  ${HELM_BIN} del "${DRIVER_NAME}" \
    --namespace kube-system

  kops_delete_cluster \
    "${KOPS_BIN}" \
    "${CLUSTER_NAME}" \
    "${KOPS_STATE_FILE}"
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
