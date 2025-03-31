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

# This script builds the EBS CSI Driver image for the e2e tests
# Environment variables have default values (see config.sh) but
# many can be overridden on demand if needed

set -euo pipefail

BASE_DIR="$(dirname "$(realpath "${BASH_SOURCE[0]}")")"
BIN="${BASE_DIR}/../../bin"

source "${BASE_DIR}/config.sh"
source "${BASE_DIR}/util.sh"

function build_and_push() {
  REGION=${1}
  AWS_ACCOUNT_ID=${2}
  IMAGE_NAME=${3}
  IMAGE_TAG=${4}
  IMAGE_ARCH=${5}

  # https://docs.aws.amazon.com/AmazonECR/latest/userguide/service-quotas.html
  MAX_IMAGES=10000
  IMAGE_COUNT=$(aws ecr list-images --repository-name "${IMAGE_NAME##*/}" --region "${REGION}" --query 'length(imageIds[])')

  if [ $IMAGE_COUNT -ge $MAX_IMAGES ]; then
    loudecho "Repository image limit reached. Unable to push new images."
  fi

  loudecho "Building and pushing test driver image to ${IMAGE_NAME}:${IMAGE_TAG}"
  # Ignore login failures, could be caused by having https://github.com/awslabs/amazon-ecr-credential-helper installed or other similar special cases
  # We're about to try to push the image, so any credential issue will be revealed at that time
  aws ecr get-login-password --region "${REGION}" | docker login --username AWS --password-stdin "${AWS_ACCOUNT_ID}.dkr.ecr.${REGION}.amazonaws.com" ||
    echo "Ignoring ECR login failure, image push may fail if this is unexpected"

  # Only setup buildx builder on Prow, allow local users to use docker cache
  if [ -n "${PROW_JOB_ID:-}" ]; then
    trap "docker buildx rm ebs-csi-multiarch-builder" EXIT
    docker buildx create --driver-opt=image=moby/buildkit:v0.12.5 --bootstrap --use --name ebs-csi-multiarch-builder
    # Ignore failures: Sometimes, this fails if run in parallel across multiple jobs
    # If it fails "for real" the build later will fail, so it is safe to proceed
    docker run --rm --privileged multiarch/qemu-user-static --reset -p yes || true
  fi

  export IMAGE="${IMAGE_NAME}"
  export TAG="${IMAGE_TAG}"
  if [[ "$WINDOWS" == true ]]; then
    export ALL_OS="linux windows"
    export ALL_OSVERSION_windows="ltsc2022"
    export ALL_ARCH_linux="amd64"
    export ALL_ARCH_windows="${IMAGE_ARCH}"
  else
    export ALL_OS="linux"
    export ALL_ARCH_linux="${IMAGE_ARCH}"
  fi

  PUSH_TYPE="sub-push"

  if [[ "$INSTANCE_TYPE" == "a1.large" ]]; then
    # In the case of a1compat image we need both controller image and a1 compat node image
    PUSH_TYPE="sub-push sub-push-a1compat"
  fi
  if [[ "${FIPS_TEST}" == "true" ]]; then
    PUSH_TYPE="sub-push-fips"
  fi

  make -j $(nproc) ${PUSH_TYPE}

  loudecho "Image pushed to ${IMAGE_NAME}:${IMAGE_TAG}"
}

if [[ "${CREATE_MISSING_ECR_REPO}" == true ]]; then
  REPO_CHECK=$(aws ecr describe-repositories --region "${AWS_REGION}")
  if [ $(jq ".repositories | map(.repositoryName) | index(\"${IMAGE_NAME##*/}\")" <<<"${REPO_CHECK}") == "null" ]; then
    aws ecr create-repository --region "${AWS_REGION}" --repository-name aws-ebs-csi-driver >/dev/null
  fi
fi

build_and_push "${AWS_REGION}" \
  "${AWS_ACCOUNT_ID}" \
  "${IMAGE_NAME}" \
  "${IMAGE_TAG}" \
  "${IMAGE_ARCH}"
