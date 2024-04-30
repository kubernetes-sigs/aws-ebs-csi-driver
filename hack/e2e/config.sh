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

set -euo pipefail

BASE_DIR="$(dirname "$(realpath "${BASH_SOURCE[0]}")")"
TEST_DIR="${BASE_DIR}/csi-test-artifacts"
mkdir -p "${TEST_DIR}"
CLUSTER_FILE=${TEST_DIR}/${CLUSTER_NAME}.${CLUSTER_TYPE}.yaml
KUBECONFIG=${KUBECONFIG:-"${TEST_DIR}/${CLUSTER_NAME}.${CLUSTER_TYPE}.kubeconfig"}

export AWS_REGION=${AWS_REGION:-us-west-2}
ZONES=${AWS_AVAILABILITY_ZONES:-us-west-2a,us-west-2b,us-west-2c}
FIRST_ZONE=$(echo "${ZONES}" | cut -d, -f1)
NODE_COUNT=${NODE_COUNT:-3}
INSTANCE_TYPE=${INSTANCE_TYPE:-c5.large}
WINDOWS=${WINDOWS:-"false"}
WINDOWS_HOSTPROCESS=${WINDOWS_HOSTPROCESS:-"false"}

# kops: must include patch version (e.g. 1.19.1)
# eksctl: mustn't include patch version (e.g. 1.19)
K8S_VERSION_KOPS=${K8S_VERSION_KOPS:-1.29.3}
K8S_VERSION_EKSCTL=${K8S_VERSION_EKSCTL:-1.29}

EBS_INSTALL_SNAPSHOT=${EBS_INSTALL_SNAPSHOT:-"true"}
EBS_INSTALL_SNAPSHOT_VERSION=${EBS_INSTALL_SNAPSHOT_VERSION:-"v7.0.1"}

AWS_ACCOUNT_ID=$(aws sts get-caller-identity --query Account --output text)
KOPS_BUCKET=${KOPS_BUCKET:-${AWS_ACCOUNT_ID}-ebs-csi-e2e-kops}

AMI_PARAMETER=${AMI_PARAMETER:-/aws/service/ami-amazon-linux-latest/al2023-ami-kernel-default-x86_64}
AMI_ID=$(aws ssm get-parameters --names ${AMI_PARAMETER} --region ${AWS_REGION} --query 'Parameters[0].Value' --output text)

CREATE_MISSING_ECR_REPO=${CREATE_MISSING_ECR_REPO:-"true"}
IMAGE_NAME=${IMAGE_NAME:-${AWS_ACCOUNT_ID}.dkr.ecr.${AWS_REGION}.amazonaws.com/aws-ebs-csi-driver}
IMAGE_TAG=${IMAGE_TAG:-$(md5sum <<<"${CLUSTER_NAME}.${CLUSTER_TYPE}" | awk '{ print $1 }')}
IMAGE_ARCH=${IMAGE_ARCH:-amd64}

DEPLOY_METHOD=${DEPLOY_METHOD:-"helm"}
HELM_CT_TEST=${HELM_CT_TEST:-"false"}
HELM_EXTRA_FLAGS=${HELM_EXTRA_FLAGS:-}
COLLECT_METRICS=${COLLECT_METRICS:-"false"}

TEST_PATH=${TEST_PATH:-"./tests/e2e-kubernetes/..."}
GINKGO_FOCUS=${GINKGO_FOCUS:-"External.Storage"}
GINKGO_SKIP=${GINKGO_SKIP:-"\[Disruptive\]|\[Serial\]"}
GINKGO_PARALLEL=${GINKGO_PARALLEL:-25}

# TODO: Left in for now, but look into if this is still necessary and remove if not
EKSCTL_ADMIN_ROLE=${EKSCTL_ADMIN_ROLE:-"Infra-prod-KopsDeleteAllLambdaServiceRoleF1578477-1ELDFIB4KCMXV"}
