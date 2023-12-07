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

case ${1} in
test-e2e-single-az)
  TEST="single-az"
  export AWS_AVAILABILITY_ZONES="us-west-2a"
  ;;
test-e2e-multi-az)
  TEST="multi-az"
  ;;
test-e2e-external)
  TEST="external"
  ;;
test-e2e-external-arm64)
  TEST="external-arm64"
  export INSTANCE_TYPE="m7g.medium"
  export AMI_PARAMETER="/aws/service/ami-amazon-linux-latest/al2023-ami-kernel-default-arm64"
  ;;
test-e2e-external-eks)
  TEST="external"
  export CLUSTER_TYPE="eksctl"
  ;;
test-e2e-external-eks-windows)
  TEST="external-windows"
  export CLUSTER_TYPE="eksctl"
  export WINDOWS="true"
  ;;
test-e2e-external-kustomize)
  TEST="external-kustomize"
  ;;
test-helm-chart)
  TEST="helm-ct"
  ;;
*)
  echo "Unknown e2e test ${1}" >&2
  exit 1
  ;;
esac

export CLUSTER_NAME="ebs-csi-e2e-${RANDOM}.k8s.local"
# Use S3 bucket created for CI
export KOPS_BUCKET=${KOPS_BUCKET:-"k8s-kops-csi-shared-e2e"}
# Always use us-west-2 in CI, no matter where the local client is
export AWS_REGION=us-west-2

make cluster/create || exit 1
make e2e/${TEST}
E2E_PASSED=$?
make cluster/delete

echo "E2E_PASSED: ${E2E_PASSED}"
if [[ $E2E_PASSED -ne 0 ]]; then
  echo "FAIL!"
  exit 1
else
  echo "SUCCESS!"
fi
