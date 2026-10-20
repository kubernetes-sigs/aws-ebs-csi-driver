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

# This script runs tests in CI by creating a cluster, running the tests,
# cleaning up (regardless of test success/failure), and passing out the result

# Prevent race conditions by frontloading tool download
# TODO: Find a way to lock pip installs to prevent pip concurrency bugs from hurting us
make bin/aws

case ${1} in
test-e2e-functional)
  echo "===BEGIN=== RUNNING CLEANUP, SKIPPING E2E TESTS"
  PROTECTED_STACKS=$(bin/aws cloudformation describe-stacks --region us-west-2 \
    --query 'Stacks[?contains(StackName, `eksctl`) && EnableTerminationProtection == `true`].[StackId, RootId]' \
    --output text) || exit 1
  while read -r STACK_ID ROOT_ID; do
    [[ -n "${STACK_ID}" ]] || continue
    [[ "${ROOT_ID}" == "None" ]] || STACK_ID="${ROOT_ID}"
    echo "Disabling termination protection for ${STACK_ID}"
    bin/aws cloudformation update-termination-protection --region us-west-2 \
      --stack-name "${STACK_ID}" --no-enable-termination-protection
  done <<<"${PROTECTED_STACKS}"

  make bin/eksctl || exit 1
  EKS_CLUSTER_NAMES=$(bin/aws eks list-clusters --region us-west-2 --query clusters --output text) || exit 1
  CLEANUP_CUTOFF=$(date -u -d '6 hours ago' +%s) || exit 1
  for EKS_CLUSTER_NAME in ${EKS_CLUSTER_NAMES}; do
    [[ "${EKS_CLUSTER_NAME}" =~ ^e2e-[0-9]+-k8s-local$ ]] || continue
    CLUSTER_METADATA=$(bin/aws eks describe-cluster --region us-west-2 --name "${EKS_CLUSTER_NAME}" \
      --query 'cluster.[createdAt, status, tags.Shared]' --output text) || continue
    read -r CREATED_AT CLUSTER_STATUS SHARED_TAG <<<"${CLUSTER_METADATA}"
    [[ "${CLUSTER_STATUS}" != "DELETING" && "${SHARED_TAG}" != "Ignore" ]] || continue
    CREATED_AT_EPOCH=$(date -d "${CREATED_AT}" +%s) || continue
    ((CREATED_AT_EPOCH < CLEANUP_CUTOFF)) || continue
    echo "Starting deletion of EKS cluster ${EKS_CLUSTER_NAME}, created ${CREATED_AT}"
    bin/eksctl delete cluster --region us-west-2 --name "${EKS_CLUSTER_NAME}" --force --wait=false
  done

  EKSCTL_STACK_IDS=$(bin/aws cloudformation describe-stacks --region us-west-2 \
    --query 'Stacks[?contains(StackName, `eksctl`) && StackStatus != `DELETE_IN_PROGRESS`].StackId' \
    --output text) || exit 1
  for STACK_ID in ${EKSCTL_STACK_IDS}; do
    echo "Starting deletion of CloudFormation stack ${STACK_ID}"
    bin/aws cloudformation delete-stack --region us-west-2 --stack-name "${STACK_ID}"
  done

  EIP_ALLOCATION_IDS=$(bin/aws ec2 describe-addresses --region us-west-2 \
    --query 'Addresses[?AssociationId == `null` && NetworkInterfaceId == `null` && ServiceManaged == `null` && !Tags[?Key == `Shared` && Value == `Ignore`]].AllocationId' \
    --output text) || exit 1
  for ALLOCATION_ID in ${EIP_ALLOCATION_IDS}; do
    echo "Releasing unused Elastic IP ${ALLOCATION_ID}"
    bin/aws ec2 release-address --region us-west-2 --allocation-id "${ALLOCATION_ID}"
  done
  echo "===END=== RUNNING CLEANUP, SKIPPING E2E TESTS"
  TEST="functional"
  ;;
test-e2e-disruptive)
  TEST="disruptive"
  ;;
test-e2e-external)
  TEST="external"
  ;;
test-e2e-external-fips)
  TEST="external-fips"
  ;;
test-e2e-external-arm64)
  TEST="external"
  export IMAGE_ARCH="arm64"
  export INSTANCE_TYPE="m7g.medium"
  export AMI_PARAMETER="/aws/service/ami-amazon-linux-latest/al2023-ami-kernel-default-arm64"
  ;;
test-e2e-external-eks)
  TEST="external"
  export CLUSTER_TYPE="eksctl"
  ;;
test-e2e-external-eks-oldest)
  TEST="external"
  export CLUSTER_TYPE="eksctl"
  OLDEST_EKS_VERSION=$(aws eks describe-cluster-versions \
    --region "${AWS_REGION:-us-west-2}" \
    --cluster-type eks \
    --output json |
    jq -r '[.clusterVersions[]
            | select(.versionStatus == "STANDARD_SUPPORT" or .versionStatus == "EXTENDED_SUPPORT")
            | .clusterVersion]
           | sort_by(split(".") | map(tonumber))
           | .[0] // empty')
  if [[ -z "${OLDEST_EKS_VERSION}" ]]; then
    echo "Failed to resolve the oldest supported EKS Kubernetes version from the EKS API" >&2
    exit 1
  fi
  export K8S_VERSION_EKSCTL="${OLDEST_EKS_VERSION}"
  ;;
test-e2e-external-eks-bottlerocket)
  TEST="external-eks-bottlerocket"
  export CLUSTER_TYPE="eksctl"
  export AMI_FAMILY="Bottlerocket"
  ;;
test-e2e-external-eks-windows)
  TEST="external-windows"
  export CLUSTER_TYPE="eksctl"
  export WINDOWS="true"
  ;;
test-e2e-external-eks-windows-fips)
  TEST="external-windows-fips"
  export CLUSTER_TYPE="eksctl"
  export WINDOWS="true"
  ;;
test-e2e-external-windows-hostprocess)
  TEST="external-windows-hostprocess"
  export CLUSTER_TYPE="eksctl"
  export WINDOWS="true"
  export WINDOWS_HOSTPROCESS="true"
  ;;
test-e2e-external-kustomize)
  TEST="external-kustomize"
  ;;
test-helm-chart)
  TEST="helm-ct"
  export INSTANCE_TYPE="c5.2xlarge"
  ;;
test-e2e-parameters)
  TEST="parameters-all"
  ;;
*)
  echo "Unknown e2e test ${1}" >&2
  exit 1
  ;;
esac

echo "SKIPPING ALL E2E TESTS, ABORTING"
exit 1

export CLUSTER_NAME="e2e-${BUILD_ID:-${RANDOM}}.k8s.local"
# Use S3 bucket created for CI
export KOPS_BUCKET=${KOPS_BUCKET:-"k8s-kops-csi-shared-e2e"}
# Always use us-west-2 in CI, no matter where the local client is
export AWS_REGION=us-west-2

make cluster/create &
PIDS[1]=$!
make cluster/image &
PIDS[2]=$!
# Installing kubetest2 takes surprisingly long and blocks make e2e/${TEST}
# Thus, to speed up CI, preinstall it while creating the cluster/image
make bin/kubetest2 &
PIDS[3]=$!

for PID in "${PIDS[@]}"; do
  wait $PID || E2E_PASSED=1
done

if [[ $E2E_PASSED -eq 0 ]]; then
  make e2e/${TEST}
  E2E_PASSED=$?
fi
make cluster/delete

echo "E2E_PASSED: ${E2E_PASSED}"
if [[ $E2E_PASSED -ne 0 ]]; then
  echo "FAIL!"
  exit 1
else
  echo "SUCCESS!"
fi
