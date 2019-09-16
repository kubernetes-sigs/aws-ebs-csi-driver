# Copyright 2019 The Kubernetes Authors.
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
IMAGE?=amazon/aws-ebs-csi-driver
VERSION=0.5.0-dirty
GIT_COMMIT?=$(shell git rev-parse HEAD)
BUILD_DATE?=$(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
LDFLAGS?="-X ${PKG}/pkg/driver.driverVersion=${VERSION} -X ${PKG}/pkg/driver.gitCommit=${GIT_COMMIT} -X ${PKG}/pkg/driver.buildDate=${BUILD_DATE} -s -w"
GO111MODULE=on
GOPROXY=direct

.EXPORT_ALL_VARIABLES:

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
	./hack/run-integration-test

.PHONY: test-e2e-single-az
test-e2e-single-az:
	TESTCONFIG=./tester/single-az-config.yaml go run tester/cmd/main.go

.PHONY: test-e2e-multi-az
test-e2e-multi-az:
	TESTCONFIG=./tester/multi-az-config.yaml go run tester/cmd/main.go

.PHONY: test-e2e-migration
test-e2e-migration:
	AWS_REGION=us-west-2 AWS_AVAILABILITY_ZONES=us-west-2a GINKGO_FOCUS="\[ebs-csi-migration\]" ./hack/run-e2e-test
	# TODO: enable migration test to use new framework
	#TESTCONFIG=./tester/migration-test-config.yaml go run tester/cmd/main.go

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
