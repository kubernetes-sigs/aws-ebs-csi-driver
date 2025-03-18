#!/bin/bash

# Copyright 2025 The Kubernetes Authors.
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

set -euo pipefail

BASE_DIR="$(dirname "$(realpath "${BASH_SOURCE[0]}")")"
BIN="${BASE_DIR}/../../bin"

source "${BASE_DIR}/config.sh"
source "${BASE_DIR}/util.sh"

export IMAGE="${IMAGE_NAME}"
export TAG="${IMAGE_TAG}"

function build_and_push() {
  REGION=${1}
  AWS_ACCOUNT_ID=${2}
  IMAGE_NAME=${3}
  IMAGE_TAG=${4}
  IMAGE_ARCH=${5}

  # https://docs.aws.amazon.com/AmazonECR/latest/userguide/service-quotas.html
  MAX_IMAGES=10000
  IMAGE_COUNT=$(aws ecr list-images --repository-name "${IMAGE##*/}" --region "${REGION}" --query 'length(imageIds[])')

  if [ $IMAGE_COUNT -ge $MAX_IMAGES ]; then
    loudecho "Repository image limit reached. Unable to push new images."
    exit 1
  fi

  loudecho "Building and pushing test driver images to ${IMAGE}:${IMAGE_TAG}"
  aws ecr get-login-password --region "${REGION}" | docker login --username AWS --password-stdin "${AWS_ACCOUNT_ID}.dkr.ecr.${REGION}.amazonaws.com"
  trap "docker buildx rm ebs-csi-multiarch-builder" EXIT
  docker buildx create --driver-opt=image=moby/buildkit:v0.12.5 --bootstrap --use --name ebs-csi-multiarch-builder
  # Ignore failures: Sometimes, this fails if run in parallel across multiple jobs
  # If it fails "for real" the build later will fail, so it is safe to proceed
  docker run --rm --privileged multiarch/qemu-user-static --reset -p yes || true

  make -j $(nproc) all-push

  loudecho "Images pushed to ${IMAGE_NAME}:${IMAGE_TAG}"
}

REPO_CHECK=$(aws ecr describe-repositories --region "${AWS_REGION}")
if [ $(jq ".repositories | map(.repositoryName) | index(\"${IMAGE_NAME##*/}\")" <<<"${REPO_CHECK}") == "null" ]; then
  aws ecr create-repository --region "${AWS_REGION}" --repository-name aws-ebs-csi-driver >/dev/null
fi

build_and_push "${AWS_REGION}" \
  "${AWS_ACCOUNT_ID}" \
  "${IMAGE_NAME}" \
  "${IMAGE_TAG}" \
  "${IMAGE_ARCH}"

imageSuffixes=("a1compat fips-windows-amd64-ltsc2022 fips-windows-amd64-ltsc2019 windows-amd64-ltsc2019 windows-amd64-ltsc2022 fips-linux-amd64-al2023 fips-linux-arm64-al2023 linux-amd64-al2023 linux-arm64-al2023 linux-arm64-al2")

loudecho "Ensuring all images are present"

for suffix in ${imageSuffixes[@]}; do
  if [ ! "$(docker manifest inspect "${IMAGE}":"${TAG}"-"${suffix}")" ]; then
    loudecho "$suffix image not found"
    exit 1
  fi
done

loudecho "Ensuring image indexes have all images"
if [ ! "$(docker manifest inspect ${IMAGE}:${TAG} | jq ".manifests[3].platform")" ]; then
  loudecho "Error index image is missing images"
  exit 1
fi
if [ ! "$(docker manifest inspect ${IMAGE}:${TAG}-fips | jq ".manifests[3].platform")" ]; then
  loudecho "Error fips index image is missing images"
  exit 1
fi

loudecho "Success"
