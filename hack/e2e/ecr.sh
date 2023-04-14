#!/bin/bash

set -uo pipefail

BASE_DIR=$(dirname "$(realpath "${BASH_SOURCE[0]}")")
source "${BASE_DIR}"/util.sh

function ecr_build_and_push() {
  REGION=${1}
  AWS_ACCOUNT_ID=${2}
  IMAGE_NAME=${3}
  IMAGE_TAG=${4}
  set +e
  if docker images --format "{{.Repository}}:{{.Tag}}" | grep "${IMAGE_NAME}:${IMAGE_TAG}"; then
    set -e
    loudecho "Assuming ${IMAGE_NAME}:${IMAGE_TAG} has been built and pushed"
  else
    set -e
    loudecho "Building and pushing test driver image to ${IMAGE_NAME}:${IMAGE_TAG}"
    aws ecr get-login-password --region "${REGION}" | docker login --username AWS --password-stdin "${AWS_ACCOUNT_ID}".dkr.ecr."${REGION}".amazonaws.com
    if [[ "$WINDOWS" == true ]]; then
      export DOCKER_CLI_EXPERIMENTAL=enabled
      export TAG=${IMAGE_TAG}
      export IMAGE=${IMAGE_NAME}
      trap "docker buildx rm multiarch-builder" EXIT
      docker buildx create --use --name multiarch-builder
      docker run --rm --privileged multiarch/qemu-user-static --reset -p yes
      make all-push
    else
      IMAGE=${IMAGE_NAME} TAG=${IMAGE_TAG} OS=linux ARCH=amd64 OSVERSION=amazon make image
      docker tag "${IMAGE_NAME}":"${IMAGE_TAG}"-linux-amd64-amazon "${IMAGE_NAME}":"${IMAGE_TAG}"
      docker push "${IMAGE_NAME}":"${IMAGE_TAG}"
    fi
  fi
}
