#!/bin/bash
# Copyright 2025 The Kubernetes Authors.
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

# Checks/creates $S3_BUCKET and that there hasn't been a run with $SCALABILITY_TEST_RUN_NAME
pre_test_validation() {
  if ! aws s3 ls "s3://$S3_BUCKET"; then
    aws s3 mb "s3://$S3_BUCKET" --region "${AWS_REGION}"
  fi

  result=$(aws s3api list-objects-v2 --bucket "$S3_BUCKET" --prefix "$SCALABILITY_TEST_RUN_NAME" --query 'Contents[]')
  if [[ "$result" != "null" ]]; then
    echo "ERROR: Your S3 bucket already contains directory with name \$SCALABILITY_TEST_RUN_NAME: 's3://$S3_BUCKET/$SCALABILITY_TEST_RUN_NAME'. Please pick a unique SCALABILITY_TEST_RUN_NAME."
    exit 1
  fi

  echo "Updating kubeconfig and restarting ebs-csi-controller Deployment"
  aws eks update-kubeconfig --name "$CLUSTER_NAME"
  kubectl rollout restart deployment/ebs-csi-controller -n kube-system
  kubectl rollout status deployment/ebs-csi-controller -n kube-system --timeout=30s
}
