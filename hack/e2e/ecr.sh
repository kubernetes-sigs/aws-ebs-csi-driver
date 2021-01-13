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
  if docker images | grep "${IMAGE_NAME}" | grep "${IMAGE_TAG}"; then
    set -e
    loudecho "Assuming ${IMAGE_NAME}:${IMAGE_TAG} has been built and pushed"
  else
    set -e
    loudecho "Building and pushing test driver image to ${IMAGE_NAME}:${IMAGE_TAG}"
    aws ecr get-login-password --region "${REGION}" | docker login --username AWS --password-stdin "${AWS_ACCOUNT_ID}".dkr.ecr."${REGION}".amazonaws.com
    docker build -t "${IMAGE_NAME}":"${IMAGE_TAG}" .
    docker push "${IMAGE_NAME}":"${IMAGE_TAG}"
  fi
}
