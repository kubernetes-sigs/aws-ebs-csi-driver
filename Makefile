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

###
### This Makefile is documented in docs/makefile.md
###

## Variables/Functions

VERSION?=v1.47.1

PKG=github.com/kubernetes-sigs/aws-ebs-csi-driver
GIT_COMMIT?=$(shell git rev-parse HEAD)
BUILD_DATE?=$(shell date -u -Iseconds)
LDFLAGS?="-X ${PKG}/pkg/driver.driverVersion=${VERSION} -X ${PKG}/pkg/cloud.driverVersion=${VERSION} -X ${PKG}/pkg/driver.gitCommit=${GIT_COMMIT} -X ${PKG}/pkg/driver.buildDate=${BUILD_DATE} -s -w"

OS?=$(shell go env GOHOSTOS)
ARCH?=$(shell go env GOHOSTARCH)
ifeq ($(OS),windows)
	BINARY=aws-ebs-csi-driver.exe
	OSVERSION?=ltsc2022
else
	BINARY=aws-ebs-csi-driver
	OSVERSION?=al2023
endif
FIPS?=false
ifeq ($(FIPS),true)
	FIPS_DOCKER_ARGS=--build-arg=GOEXPERIMENT=boringcrypto
endif

GO_SOURCES=go.mod go.sum $(shell find pkg cmd -type f -name "*.go")

ALL_OS?=linux windows
ALL_ARCH_linux?=amd64 arm64
ALL_OSVERSION_linux?=al2023
ALL_OS_ARCH_OSVERSION_linux=$(foreach arch, $(ALL_ARCH_linux), $(foreach osversion, ${ALL_OSVERSION_linux}, linux-$(arch)-${osversion}))

ALL_ARCH_windows?=amd64
ALL_OSVERSION_windows?=ltsc2019 ltsc2022
ALL_OS_ARCH_OSVERSION_windows=$(foreach arch, $(ALL_ARCH_windows), $(foreach osversion, ${ALL_OSVERSION_windows}, windows-$(arch)-${osversion}))
ALL_OS_ARCH_OSVERSION=$(foreach os, $(ALL_OS), ${ALL_OS_ARCH_OSVERSION_${os}})

CLUSTER_NAME?=ebs-csi-e2e.k8s.local
CLUSTER_TYPE?=kops

GINKGO_WINDOWS_SKIP?="\[Disruptive\]|\[Serial\]|\[Flaky\]|\[LinuxOnly\]|\[Feature:VolumeSnapshotDataSource\]|\(xfs\)|\(ext4\)|\(block volmode\)"
GINKGO_BOTTLEROCKET_SKIP?="\[Disruptive\]|\[Serial\]|\[Flaky\]|should not mount / map unused volumes in a pod \[LinuxOnly\]"

# split words on hyphen, access by 1-index
word-hyphen = $(word $2,$(subst -, ,$1))

.EXPORT_ALL_VARIABLES:

## Default target
# When no target is supplied, make runs the first target that does not begin with a .
# Alias that to building the binary
.PHONY: default
default: bin/$(BINARY)

## Top level targets

.PHONY: clean
clean:
	rm -rf bin/

.PHONY: test
test:
	go test -v -race ./cmd/... ./pkg/... ./tests/sanity/...

.PHONY: test/coverage
test/coverage:
	go test -coverprofile=cover.out ./cmd/... ./pkg/...
	grep -v "mock" cover.out > filtered_cover.out
	go tool cover -html=filtered_cover.out -o coverage.html
	rm cover.out filtered_cover.out

.PHONY: tools
tools: bin/aws bin/ct bin/eksctl bin/ginkgo bin/golangci-lint bin/gomplate bin/helm bin/kops bin/kubetest2 bin/mockgen bin/shfmt

.PHONY: update
update: update/gofix update/gofmt update/kustomize update/mockgen update/gomod update/shfmt update/generate-license-header
	@echo "All updates succeeded!"

.PHONY: verify
verify: verify/govet verify/golangci-lint verify/update
	@echo "All verifications passed!"

.PHONY: cluster/create
cluster/create: bin/kops bin/eksctl bin/aws bin/gomplate
	./hack/e2e/create-cluster.sh

.PHONY: cluster/kubeconfig
cluster/kubeconfig:
	@./hack/e2e/kubeconfig.sh

.PHONY: cluster/image
cluster/image: bin/aws
	./hack/e2e/build-image.sh

.PHONY: cluster/delete
cluster/delete: bin/kops bin/eksctl
	./hack/e2e/delete-cluster.sh

.PHONY: cluster/install
cluster/install: bin/helm bin/aws
	./hack/e2e/install.sh

.PHONY: cluster/helm
cluster/helm: bin/helm bin/aws
	HELM_USE_DEFAULT_IMAGE="true" \
	./hack/e2e/install.sh

.PHONY: cluster/uninstall
cluster/uninstall: bin/helm bin/aws
	./hack/e2e/uninstall.sh

## E2E targets
# Targets to run e2e tests

.PHONY: e2e/single-az
e2e/single-az: bin/helm bin/ginkgo
	AWS_AVAILABILITY_ZONES=us-west-2a \
	TEST_PATH=./tests/e2e/... \
	GINKGO_FOCUS="\[ebs-csi-e2e\] \[single-az\]" \
	GINKGO_PARALLEL=5 \
	HELM_EXTRA_FLAGS="--set=controller.volumeModificationFeature.enabled=true,sidecars.provisioner.additionalArgs[0]='--feature-gates=VolumeAttributesClass=true',sidecars.resizer.additionalArgs[0]='--feature-gates=VolumeAttributesClass=true',node.enableMetrics=true" \
	./hack/e2e/run.sh

.PHONY: e2e/multi-az
e2e/multi-az: bin/helm bin/ginkgo
	TEST_PATH=./tests/e2e/... \
	GINKGO_FOCUS="\[ebs-csi-e2e\] \[multi-az\]" \
	GINKGO_PARALLEL=5 \
	./hack/e2e/run.sh

.PHONY: e2e/external
e2e/external: bin/helm bin/kubetest2
	COLLECT_METRICS="true" \
	./hack/e2e/run.sh

.PHONY: e2e/external-a1-eks
e2e/external-a1-eks: bin/helm bin/kubetest2
	HELM_EXTRA_FLAGS="--set=a1CompatibilityDaemonSet=true" \
	./hack/e2e/run.sh

.PHONY: e2e/external-eks-bottlerocket
e2e/external-eks-bottlerocket: bin/helm bin/kubetest2
	GINKGO_SKIP=$(GINKGO_BOTTLEROCKET_SKIP) \
	./hack/e2e/run.sh

.PHONY: e2e/external-fips
e2e/external-fips: bin/helm bin/kubetest2
	HELM_EXTRA_FLAGS="--set=fips=true" \
	./hack/e2e/run.sh

.PHONY: e2e/external-windows
e2e/external-windows: bin/helm bin/kubetest2
	WINDOWS=true \
	GINKGO_SKIP=$(GINKGO_WINDOWS_SKIP) \
	GINKGO_PARALLEL=15 \
	EBS_INSTALL_SNAPSHOT="false" \
	./hack/e2e/run.sh

.PHONY: e2e/external-windows-fips
e2e/external-windows-fips: bin/helm bin/kubetest2
	WINDOWS=true \
	GINKGO_SKIP=$(GINKGO_WINDOWS_SKIP) \
	GINKGO_PARALLEL=15 \
	EBS_INSTALL_SNAPSHOT="false" \
	HELM_EXTRA_FLAGS="--set=fips=true" \
	./hack/e2e/run.sh

.PHONY: e2e/external-windows-hostprocess
e2e/external-windows-hostprocess: bin/helm bin/kubetest2
	WINDOWS_HOSTPROCESS=true \
	WINDOWS=true \
	GINKGO_SKIP=$(GINKGO_WINDOWS_SKIP) \
	GINKGO_PARALLEL=15 \
	EBS_INSTALL_SNAPSHOT="false" \
	./hack/e2e/run.sh

.PHONY: e2e/external-kustomize
e2e/external-kustomize: bin/kubetest2
	DEPLOY_METHOD="kustomize" \
	./hack/e2e/run.sh

.PHONY: e2e/helm-ct
e2e/helm-ct: bin/helm bin/ct
	HELM_CT_TEST="true" \
	./hack/e2e/run.sh

## Release scripts
# Targets run as part of performing a release

.PHONY: update-truth-sidecars
update-truth-sidecars: hack/release-scripts/get-latest-sidecar-images
	./hack/release-scripts/get-latest-sidecar-images

.PHONY: generate-sidecar-tags
generate-sidecar-tags: update-truth-sidecars charts/aws-ebs-csi-driver/values.yaml deploy/kubernetes/overlays/stable/gcr/kustomization.yaml hack/release-scripts/generate-sidecar-tags
	./hack/release-scripts/generate-sidecar-tags

.PHONY: update-sidecar-dependencies
update-sidecar-dependencies: update-truth-sidecars generate-sidecar-tags update/kustomize

.PHONY: update-image-dependencies
update-image-dependencies: update-sidecar-dependencies
	./hack/release-scripts/update-e2e-images

.PHONY: security
security: bin/govulncheck
	./hack/tools/check-security.sh

## CI aliases
# Targets intended to be executed mostly or only by CI jobs

.PHONY: sub-push
sub-push: all-image-registry push-manifest

.PHONY: sub-push-fips
sub-push-fips:
	$(MAKE) FIPS=true TAG=$(TAG)-fips sub-push

.PHONY: sub-push-a1compat
sub-push-a1compat:
	$(MAKE) DOCKER_EXTRA_ARGS="-t=$(IMAGE):$(TAG)-a1compat" sub-image-linux-arm64-al2

.PHONY: all-push
all-push: sub-push sub-push-fips sub-push-a1compat

test-e2e-%:
	./hack/prow-e2e.sh test-e2e-$*

test-helm-chart:
	./hack/prow-e2e.sh test-helm-chart

.PHONY: test-images 
test-images: bin/aws 
	./hack/e2e/test-images.sh 

## Builds

bin:
	@mkdir -p $@

bin/$(BINARY): $(GO_SOURCES) | bin
	CGO_ENABLED=0 GOOS=$(OS) GOARCH=$(ARCH) go build -mod=readonly -ldflags ${LDFLAGS} -o $@ ./cmd/

.PHONY: all-image-registry
all-image-registry: $(addprefix sub-image-,$(ALL_OS_ARCH_OSVERSION))

sub-image-%:
	$(MAKE) OS=$(call word-hyphen,$*,1) ARCH=$(call word-hyphen,$*,2) OSVERSION=$(call word-hyphen,$*,3) image

.PHONY: image
image:
	BUILDX_NO_DEFAULT_ATTESTATIONS=1 docker buildx build \
		--platform=$(OS)/$(ARCH) \
		--progress=plain \
		--target=$(OS)-$(OSVERSION) \
		--output=type=registry \
		-t=$(IMAGE):$(TAG)-$(OS)-$(ARCH)-$(OSVERSION) \
		--build-arg=GOPROXY=$(GOPROXY) \
		--build-arg=VERSION=$(VERSION) \
		$(FIPS_DOCKER_ARGS) \
		$(DOCKER_EXTRA_ARGS) \
		.

.PHONY: create-manifest
create-manifest: all-image-registry
# sed expression:
# LHS: match 0 or more not space characters
# RHS: replace with $(IMAGE):$(TAG)-& where & is what was matched on LHS
	docker manifest create --amend $(IMAGE):$(TAG) $(shell echo $(ALL_OS_ARCH_OSVERSION) | sed -e "s~[^ ]*~$(IMAGE):$(TAG)\-&~g")

.PHONY: push-manifest
push-manifest: create-manifest
	docker manifest push --purge $(IMAGE):$(TAG)

## Tools
# Tools necessary to perform other targets

bin/%: hack/tools/install.sh hack/tools/python-runner.sh
	@TOOLS_PATH="$(shell pwd)/bin" ./hack/tools/install.sh $*

## Updaters
# Automatic generators/formatters for code

.PHONY: update/gofix
update/gofix:
	go fix ./...

.PHONY: update/gofmt
update/gofmt:
	gofmt -s -w .

.PHONY: update/kustomize
update/kustomize: bin/helm
	./hack/update-kustomize.sh

.PHONY: update/mockgen
update/mockgen: bin/mockgen
	./hack/update-mockgen.sh

.PHONY: update/gomod
update/gomod:
	go mod tidy
	go mod tidy -C tests/e2e/

.PHONY: update/shfmt
update/shfmt: bin/shfmt
	./bin/shfmt -w -i 2 -d ./hack/

.PHONY: update/generate-license-header
update/generate-license-header:
	./hack/generate-license-header.sh

## Verifiers
# Linters and similar

.PHONY: verify/golangci-lint
verify/golangci-lint: bin/golangci-lint
	./bin/golangci-lint run --timeout=10m --verbose

.PHONY: verify/govet
verify/govet:
	go vet $$(go list ./...)

.PHONY: verify/update
verify/update: bin/helm bin/mockgen
	./hack/verify-update.sh
