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
BIN="${BASE_DIR}/../../bin"
TEST_DIR="${BASE_DIR}/csi-test-artifacts"
# On Prow, $ARTIFACTS indicates where to put the artifacts for skylens upload
REPORT_DIR="${ARTIFACTS:-${TEST_DIR}/artifacts}"
mkdir -p "${TEST_DIR}"
CLUSTER_FILE=${TEST_DIR}/${CLUSTER_NAME}.${CLUSTER_TYPE}.yaml
KUBECONFIG=${KUBECONFIG:-"${TEST_DIR}/${CLUSTER_NAME}.${CLUSTER_TYPE}.kubeconfig"}

# Use AWS_REGION as priority, fallback to region from awscli config, fallback to us-west-2
REGION_FROM_CONFIG="$(${BIN}/aws configure get region || echo '')"
export AWS_REGION=${AWS_REGION:-${REGION_FROM_CONFIG:-us-west-2}}
# If zones are not provided, auto-detect the first 3 AZs that are not opt in
ZONES=${AWS_AVAILABILITY_ZONES:-$(${BIN}/aws ec2 describe-availability-zones --region "${AWS_REGION}" | jq -r '[.AvailabilityZones[] | select(.OptInStatus == "opt-in-not-required") | .ZoneName][:3] | join(",")')}
FIRST_ZONE=$(echo "${ZONES}" | cut -d, -f1)
NODE_COUNT=${NODE_COUNT:-3}
INSTANCE_TYPE=${INSTANCE_TYPE:-c5.large}
WINDOWS=${WINDOWS:-"false"}
AMI_FAMILY=${AMI_FAMILY:-"AmazonLinux2023"}
WINDOWS_HOSTPROCESS=${WINDOWS_HOSTPROCESS:-"false"}
OUTPOST_ARN=${OUTPOST_ARN:-}
OUTPOST_INSTANCE_TYPE=${OUTPOST_INSTANCE_TYPE:-${INSTANCE_TYPE}}
FIPS_TEST=${FIPS_TEST:-"false"}

# kops: must include patch version (e.g. 1.19.1)
# eksctl: mustn't include patch version (e.g. 1.19)
K8S_VERSION_KOPS=${K8S_VERSION_KOPS:-1.33.3}
K8S_VERSION_EKSCTL=${K8S_VERSION_EKSCTL:-1.33}

EBS_INSTALL_SNAPSHOT=${EBS_INSTALL_SNAPSHOT:-"true"}
EBS_INSTALL_SNAPSHOT_VERSION=${EBS_INSTALL_SNAPSHOT_VERSION:-"v8.3.0"}
EBS_INSTALL_SNAPSHOT_CUSTOM_IMAGE=${EBS_INSTALL_SNAPSHOT_CUSTOM_IMAGE:-}

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
GINKGO_SKIP=${GINKGO_SKIP:-"\[Disruptive\]|\[Serial\]|\[Flaky\]"}
GINKGO_PARALLEL=${GINKGO_PARALLEL:-25}
