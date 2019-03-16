# Copyright 2018 The Kubernetes Authors.
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

PKG=github.com/kubernetes-sigs/aws-ebs-csi-driver
IMAGE=amazon/aws-ebs-csi-driver
VERSION=0.3.0
GIT_COMMIT?=$(shell git rev-parse HEAD)
BUILD_DATE?=$(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
LDFLAGS?="-X ${PKG}/pkg/driver.driverVersion=${VERSION} -X ${PKG}/pkg/driver.gitCommit=${GIT_COMMIT} -X ${PKG}/pkg/driver.buildDate=${BUILD_DATE} -s -w"
GO111MODULE=on

# Hard-coded version is needed in case GitHub API rate limit is exceeded.
# TODO: When aws-k8s-tester becomes a full release (https://developer.github.com/v3/repos/releases/#get-the-latest-release), use:
# $(shell curl -s --request GET --url https://api.github.com/repos/aws/aws-k8s-tester/releases/latest | jq -r '.tag_name? // "<current version number>"')
AWS_K8S_TESTER_VERSION?=$(shell curl -s --request GET --url https://api.github.com/repos/aws/aws-k8s-tester/tags | jq -r '.[0]?.name // "0.2.5"')
AWS_K8S_TESTER_OS_ARCH?=$(shell go env GOOS)-amd64
AWS_K8S_TESTER_DOWNLOAD_URL?=https://github.com/aws/aws-k8s-tester/releases/download/${AWS_K8S_TESTER_VERSION}/aws-k8s-tester-${AWS_K8S_TESTER_VERSION}-${AWS_K8S_TESTER_OS_ARCH}
AWS_K8S_TESTER_PATH?=/tmp/aws-k8s-tester

.EXPORT_ALL_VARIABLES:

VPC_ID_FLAG=
ifdef AWS_K8S_TESTER_VPC_ID
	VPC_ID_FLAG=--vpc-id=${AWS_K8S_TESTER_VPC_ID}
endif

PR_NUM_FLAG=
ifdef PULL_NUMBER
	PR_NUM_FLAG=--pr-num=${PULL_NUMBER}
endif

.PHONY: aws-ebs-csi-driver
aws-ebs-csi-driver:
	mkdir -p bin
	CGO_ENABLED=0 GOOS=linux go build -ldflags ${LDFLAGS} -o bin/aws-ebs-csi-driver ./cmd/

.PHONY: verify
verify:
	./hack/verify-all

.PHONY: test
test:
	go test -v -race ./pkg/...

.PHONY: test-sanity
test-sanity:
	go test -v ./tests/sanity/...

.PHONY: test-integration
test-integration:
	curl -L ${AWS_K8S_TESTER_DOWNLOAD_URL} -o ${AWS_K8S_TESTER_PATH}
	chmod +x ${AWS_K8S_TESTER_PATH}
	${AWS_K8S_TESTER_PATH} csi test integration --terminate-on-exit=true --timeout=20m ${PR_NUM_FLAG} ${VPC_ID_FLAG}

.PHONY: test-e2e-single-az
test-e2e-single-az:
	AWS_REGION=us-west-2 AWS_AVAILABILITY_ZONES=us-west-2a GINKGO_FOCUS="\[ebs-csi-e2e\] \[single-az\]" ./hack/run-e2e-test

.PHONY: test-e2e-multi-az
test-e2e-multi-az:
	AWS_REGION=us-west-2 AWS_AVAILABILITY_ZONES=us-west-2a,us-west-2b,us-west-2c GINKGO_FOCUS="\[ebs-csi-e2e\] \[multi-az\]" ./hack/run-e2e-test

.PHONY: image-release
image-release:
	docker build -t $(IMAGE):$(VERSION) .

.PHONY: image
image:
	docker build -t $(IMAGE):latest .

.PHONY: push-release
push-release:
	docker push $(IMAGE):$(VERSION)

.PHONY: push
push:
	docker push $(IMAGE):latest
