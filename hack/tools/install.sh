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

# https://pypi.org/project/awscli/
AWSCLI_VERSION="1.31.7"
# https://github.com/helm/chart-testing
CT_VERSION="v3.10.1"
# https://github.com/eksctl-io/eksctl
EKSCTL_VERSION="v0.165.0"
# https://github.com/onsi/ginkgo
GINKGO_VERSION="v2.13.2"
# https://github.com/golangci/golangci-lint
GOLANGCI_LINT_VERSION="v1.54.0"
# https://github.com/helm/helm
HELM_VERSION="v3.13.2"
# https://github.com/kubernetes/kops
KOPS_VERSION="v1.28.1"
# https://github.com/golang/mock
MOCKGEN_VERSION="v1.6.0"
# https://pypi.org/project/yamale/
YAMALE_VERSION="4.0.4"
# https://pypi.org/project/yamllint/
YAMLLINT_VERSION="1.32.0"

OS="$(go env GOHOSTOS)"
ARCH="$(go env GOHOSTARCH)"

# Installation helpers

function install_binary() {
	INSTALL_PATH="${1}"
	DOWNLOAD_URL="${2}"
	BINARY_NAME="${3}"

	curl --location "${DOWNLOAD_URL}" --output "${INSTALL_PATH}/${BINARY_NAME}"
	chmod +x "${INSTALL_PATH}/${BINARY_NAME}"
}

function install_go() {
	INSTALL_PATH="${1}"
	PACKAGE="${2}"

	export GOBIN="${INSTALL_PATH}"
	go install "${PACKAGE}"
}

function install_pip() {
	INSTALL_PATH="${1}"
	PACKAGE="${2}"
	COMMAND="${3}"

	source "${INSTALL_PATH}/venv/bin/activate"
	python3 -m pip install "${PACKAGE}"
	cp "$(dirname "${0}")/python-runner.sh" "${INSTALL_PATH}/${COMMAND}"
}

function install_tar_binary() {
	INSTALL_PATH="${1}"
	DOWNLOAD_URL="${2}"
	BINARY_PATH="${3}"

	BINARY_NAME="$(basename "${BINARY_PATH}")"

	if [ "${DOWNLOAD_URL##*.}" = "gz" ]; then
		TAR_EXTRA_FLAGS="-z"
	elif [ "${DOWNLOAD_URL##*.}" = "xz" ]; then
		TAR_EXTRA_FLAGS="-J"
	else
		TAR_EXTRA_FLAGS=""
	fi

	curl --location "${DOWNLOAD_URL}" | tar "$TAR_EXTRA_FLAGS" --extract --touch --transform "s/.*/${BINARY_NAME}/" -C "${INSTALL_PATH}" "${BINARY_PATH}"
	chmod +x "${INSTALL_PATH}/${BINARY_NAME}"
}

# Tool-specific installers

function install_aws() {
	INSTALL_PATH="${1}"

	install_pip "${INSTALL_PATH}" "awscli==${AWSCLI_VERSION}" "aws"
}

function install_ct() {
	INSTALL_PATH="${1}"

	install_tar_binary "${INSTALL_PATH}" "https://github.com/helm/chart-testing/releases/download/${CT_VERSION}/chart-testing_${CT_VERSION:1}_${OS}_${ARCH}.tar.gz" "ct"
	install_pip "${INSTALL_PATH}" "yamale==${YAMALE_VERSION}" "yamale"
	install_pip "${INSTALL_PATH}" "yamllint==${YAMLLINT_VERSION}" "yamllint"
}

function install_eksctl() {
	INSTALL_PATH="${1}"

	install_tar_binary "${INSTALL_PATH}" "https://github.com/weaveworks/eksctl/releases/download/${EKSCTL_VERSION}/eksctl_${OS^}_${ARCH}.tar.gz" "eksctl"
}

function install_ginkgo() {
	INSTALL_PATH="${1}"

	install_go "${INSTALL_PATH}" "github.com/onsi/ginkgo/v2/ginkgo@${GINKGO_VERSION}" 
}

function install_golangci-lint() {
	INSTALL_PATH="${1}"

	# golangci-lint recommends against installing with `go install`: https://golangci-lint.run/usage/install/#install-from-source
	install_tar_binary "${INSTALL_PATH}" "https://github.com/golangci/golangci-lint/releases/download/${GOLANGCI_LINT_VERSION}/golangci-lint-${GOLANGCI_LINT_VERSION:1}-${OS}-${ARCH}.tar.gz" "golangci-lint-${GOLANGCI_LINT_VERSION:1}-${OS}-${ARCH}/golangci-lint"
}

function install_helm() {
	INSTALL_PATH="${1}"

	install_tar_binary "${INSTALL_PATH}" "https://get.helm.sh/helm-${HELM_VERSION}-${OS}-${ARCH}.tar.gz" "${OS}-${ARCH}/helm"
}

function install_kops() {
	INSTALL_PATH="${1}"

	install_binary "${INSTALL_PATH}" "https://github.com/kubernetes/kops/releases/download/${KOPS_VERSION}/kops-${OS}-${ARCH}" "kops"
}

function install_kubetest2 {
	INSTALL_PATH="${1}"

	install_go "${INSTALL_PATH}" "sigs.k8s.io/kubetest2/...@latest"
}

function install_mockgen() {
	INSTALL_PATH="${1}"

	install_go "${INSTALL_PATH}" "github.com/golang/mock/mockgen@${MOCKGEN_VERSION}"
}

# Utility functions

function create_environment() {
	INSTALL_PATH="${1}"

	if command -v "python3"; then
		PYTHON_CMD="python3"
	else
		PYTHON_CMD="python"
	fi
	VIRTUAL_ENV_DISABLE_PROMPT=1 "${PYTHON_CMD}" -m venv "${INSTALL_PATH}/venv"
}

function install_tool() {
	INSTALL_PATH="${1}"
	TOOL="${2}"

	"install_${TOOL}" "${INSTALL_PATH}"
}

# Script dispatcher

if [ ! -d "${TOOLS_PATH}/venv" ]; then
    create_environment "${TOOLS_PATH}"
fi
install_tool "${TOOLS_PATH}" "${1}"
