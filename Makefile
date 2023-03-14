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

VERSION?=v1.17.0

PKG=github.com/kubernetes-sigs/aws-ebs-csi-driver
GIT_COMMIT?=$(shell git rev-parse HEAD)
BUILD_DATE?=$(shell date -u -Iseconds)

LDFLAGS?="-X ${PKG}/pkg/driver.driverVersion=${VERSION} -X ${PKG}/pkg/cloud.driverVersion=${VERSION} -X ${PKG}/pkg/driver.gitCommit=${GIT_COMMIT} -X ${PKG}/pkg/driver.buildDate=${BUILD_DATE} -s -w"

GO111MODULE=on
GOPATH=$(shell go env GOPATH)
GOOS=$(shell go env GOOS)
GOBIN=$(shell pwd)/bin

REGISTRY?=gcr.io/k8s-staging-provider-aws
IMAGE?=$(REGISTRY)/aws-ebs-csi-driver
TAG?=$(GIT_COMMIT)

OUTPUT_TYPE?=docker

OS?=linux
ARCH?=amd64
OSVERSION?=amazon

ALL_OS?=linux windows
ALL_ARCH_linux?=amd64 arm64
ALL_OSVERSION_linux?=amazon
ALL_OS_ARCH_OSVERSION_linux=$(foreach arch, $(ALL_ARCH_linux), $(foreach osversion, ${ALL_OSVERSION_linux}, linux-$(arch)-${osversion}))

ALL_ARCH_windows?=amd64
ALL_OSVERSION_windows?=20H2 ltsc2019 ltsc2022
ALL_OS_ARCH_OSVERSION_windows=$(foreach arch, $(ALL_ARCH_windows), $(foreach osversion, ${ALL_OSVERSION_windows}, windows-$(arch)-${osversion}))

ALL_OS_ARCH_OSVERSION=$(foreach os, $(ALL_OS), ${ALL_OS_ARCH_OSVERSION_${os}})

# split words on hyphen, access by 1-index
word-hyphen = $(word $2,$(subst -, ,$1))

.EXPORT_ALL_VARIABLES:

.PHONY: linux/$(ARCH) bin/aws-ebs-csi-driver
linux/$(ARCH): bin/aws-ebs-csi-driver
bin/aws-ebs-csi-driver: | bin
	CGO_ENABLED=0 GOOS=linux GOARCH=$(ARCH) go build -mod=mod -ldflags ${LDFLAGS} -o bin/aws-ebs-csi-driver ./cmd/

.PHONY: windows/$(ARCH) bin/aws-ebs-csi-driver.exe
windows/$(ARCH): bin/aws-ebs-csi-driver.exe
bin/aws-ebs-csi-driver.exe: | bin
	CGO_ENABLED=0 GOOS=windows GOARCH=$(ARCH) go build -mod=mod -ldflags ${LDFLAGS} -o bin/aws-ebs-csi-driver.exe ./cmd/

# Builds all linux images (not windows because it can't be exported with OUTPUT_TYPE=docker)
.PHONY: all
all: all-image-docker

# Builds all linux and windows images and pushes them
.PHONY: all-push
all-push: all-image-registry push-manifest

.PHONY: push-manifest
push-manifest: create-manifest
	docker manifest push --purge $(IMAGE):$(TAG)

.PHONY: create-manifest
create-manifest: all-image-registry
# sed expression:
# LHS: match 0 or more not space characters
# RHS: replace with $(IMAGE):$(TAG)-& where & is what was matched on LHS
	docker manifest create --amend $(IMAGE):$(TAG) $(shell echo $(ALL_OS_ARCH_OSVERSION) | sed -e "s~[^ ]*~$(IMAGE):$(TAG)\-&~g")

# Only linux for OUTPUT_TYPE=docker because windows image cannot be exported
# "Currently, multi-platform images cannot be exported with the docker export type. The most common usecase for multi-platform images is to directly push to a registry (see registry)."
# https://docs.docker.com/engine/reference/commandline/buildx_build/#output
.PHONY: all-image-docker
all-image-docker: $(addprefix sub-image-docker-,$(ALL_OS_ARCH_OSVERSION_linux))
.PHONY: all-image-registry
all-image-registry: $(addprefix sub-image-registry-,$(ALL_OS_ARCH_OSVERSION))

sub-image-%:
	$(MAKE) OUTPUT_TYPE=$(call word-hyphen,$*,1) OS=$(call word-hyphen,$*,2) ARCH=$(call word-hyphen,$*,3) OSVERSION=$(call word-hyphen,$*,4) image

.PHONY: image
image: .image-$(TAG)-$(OS)-$(ARCH)-$(OSVERSION)
.image-$(TAG)-$(OS)-$(ARCH)-$(OSVERSION):
	docker buildx build \
		--platform=$(OS)/$(ARCH) \
		--progress=plain \
		--target=$(OS)-$(OSVERSION) \
		--output=type=$(OUTPUT_TYPE) \
		-t=$(IMAGE):$(TAG)-$(OS)-$(ARCH)-$(OSVERSION) \
		--build-arg=GOPROXY=$(GOPROXY) \
		--build-arg=VERSION=$(VERSION) \
		`./hack/provenance` \
		.
	touch $@

.PHONY: clean
clean:
	rm -rf .*image-* bin/

bin /tmp/helm /tmp/kubeval:
	@mkdir -p $@

bin/helm: | /tmp/helm bin
	@curl -o /tmp/helm/helm.tar.gz -sSL https://get.helm.sh/helm-v3.11.2-${GOOS}-amd64.tar.gz
	@tar -zxf /tmp/helm/helm.tar.gz -C bin --strip-components=1
	@rm -rf /tmp/helm/*

bin/kubeval: | /tmp/kubeval bin
	@curl -o /tmp/kubeval/kubeval.tar.gz -sSL https://github.com/instrumenta/kubeval/releases/download/0.16.1/kubeval-linux-amd64.tar.gz
	@tar -zxf /tmp/kubeval/kubeval.tar.gz -C bin kubeval
	@rm -rf /tmp/kubeval/*

bin/mockgen: | bin
	go install github.com/golang/mock/mockgen@v1.6.0

bin/golangci-lint: | bin
	echo "Installing golangci-lint..."
	curl -sfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh| sh -s v1.51.2

.PHONY: kubeval
kubeval: bin/kubeval
	bin/kubeval -d deploy/kubernetes/base,deploy/kubernetes/cluster,deploy/kubernetes/overlays -i kustomization.yaml,crd_.+\.yaml,controller_add

.PHONY: mockgen
mockgen: bin/mockgen
	./hack/update-gomock

.PHONY: verify
verify: bin/golangci-lint
	echo "verifying and linting files ..."
	./hack/verify-all
	echo "Congratulations! All Go source files have been linted."

.PHONY: test
test:
	go test -v -race ./cmd/... ./pkg/...

.PHONY: test-sanity
test-sanity:
	#go test -v ./tests/sanity/...
	echo "succeed"

.PHONY: test-e2e-single-az
test-e2e-single-az:
	AWS_REGION=us-west-2 \
	AWS_AVAILABILITY_ZONES=us-west-2a \
	HELM_EXTRA_FLAGS='--set=controller.k8sTagClusterId=$$CLUSTER_NAME' \
	EBS_INSTALL_SNAPSHOT="true" \
	TEST_PATH=./tests/e2e/... \
	GINKGO_FOCUS="\[ebs-csi-e2e\] \[single-az\]" \
	GINKGO_SKIP="\"sc1\"|\"st1\"" \
	./hack/e2e/run.sh

.PHONY: test-e2e-multi-az
test-e2e-multi-az:
	AWS_REGION=us-west-2 \
	AWS_AVAILABILITY_ZONES=us-west-2a,us-west-2b,us-west-2c \
	HELM_EXTRA_FLAGS='--set=controller.k8sTagClusterId=$$CLUSTER_NAME' \
	EBS_INSTALL_SNAPSHOT="true" \
	TEST_PATH=./tests/e2e/... \
	GINKGO_FOCUS="\[ebs-csi-e2e\] \[multi-az\]" \
	./hack/e2e/run.sh

.PHONY: test-e2e-external
test-e2e-external:
	AWS_REGION=us-west-2 \
	AWS_AVAILABILITY_ZONES=us-west-2a,us-west-2b,us-west-2c \
	HELM_EXTRA_FLAGS='--set=controller.k8sTagClusterId=$$CLUSTER_NAME' \
	EBS_INSTALL_SNAPSHOT="true" \
	TEST_PATH=./tests/e2e-kubernetes/... \
	GINKGO_FOCUS="External.Storage" \
	GINKGO_SKIP="\[Disruptive\]|\[Serial\]" \
	./hack/e2e/run.sh

.PHONY: test-e2e-external-eks
test-e2e-external-eks:
	CLUSTER_TYPE=eksctl \
	HELM_VALUES_FILE="./hack/values_eksctl.yaml" \
	HELM_EXTRA_FLAGS='--set=controller.k8sTagClusterId=$$CLUSTER_NAME' \
	EBS_INSTALL_SNAPSHOT="true" \
	EKSCTL_ADMIN_ROLE="Infra-prod-KopsDeleteAllLambdaServiceRoleF1578477-1ELDFIB4KCMXV" \
	AWS_REGION=us-west-2 \
	AWS_AVAILABILITY_ZONES=us-west-2a,us-west-2b \
	TEST_PATH=./tests/e2e-kubernetes/... \
	GINKGO_FOCUS="External.Storage" \
	GINKGO_SKIP="\[Disruptive\]|\[Serial\]" \
	./hack/e2e/run.sh

.PHONY: test-helm-chart
test-helm-chart:
	AWS_REGION=us-west-2 \
	AWS_AVAILABILITY_ZONES=us-west-2a,us-west-2b,us-west-2c \
	EBS_INSTALL_SNAPSHOT="true" \
	HELM_CT_TEST="true" \
	./hack/e2e/run.sh

.PHONY: verify-vendor
test: verify-vendor
verify: verify-vendor
verify-vendor:
	@ echo; echo "### $@:"
	@ ./hack/verify-vendor.sh

.PHONY: verify-kustomize
verify: verify-kustomize
verify-kustomize:
	@ echo; echo "### $@:"
	@ ./hack/verify-kustomize

.PHONY: generate-kustomize
generate-kustomize: bin/helm
	cd charts/aws-ebs-csi-driver && ../../bin/helm template kustomize . -s templates/clusterrole-attacher.yaml > ../../deploy/kubernetes/base/clusterrole-attacher.yaml
	cd charts/aws-ebs-csi-driver && ../../bin/helm template kustomize . -s templates/clusterrole-csi-node.yaml > ../../deploy/kubernetes/base/clusterrole-csi-node.yaml
	cd charts/aws-ebs-csi-driver && ../../bin/helm template kustomize . -s templates/clusterrole-provisioner.yaml > ../../deploy/kubernetes/base/clusterrole-provisioner.yaml
	cd charts/aws-ebs-csi-driver && ../../bin/helm template kustomize . -s templates/clusterrole-resizer.yaml > ../../deploy/kubernetes/base/clusterrole-resizer.yaml
	cd charts/aws-ebs-csi-driver && ../../bin/helm template kustomize . -s templates/clusterrole-snapshotter.yaml > ../../deploy/kubernetes/base/clusterrole-snapshotter.yaml
	cd charts/aws-ebs-csi-driver && ../../bin/helm template kustomize . -s templates/clusterrolebinding-attacher.yaml > ../../deploy/kubernetes/base/clusterrolebinding-attacher.yaml
	cd charts/aws-ebs-csi-driver && ../../bin/helm template kustomize . -s templates/clusterrolebinding-csi-node.yaml > ../../deploy/kubernetes/base/clusterrolebinding-csi-node.yaml
	cd charts/aws-ebs-csi-driver && ../../bin/helm template kustomize . -s templates/clusterrolebinding-provisioner.yaml > ../../deploy/kubernetes/base/clusterrolebinding-provisioner.yaml
	cd charts/aws-ebs-csi-driver && ../../bin/helm template kustomize . -s templates/clusterrolebinding-resizer.yaml > ../../deploy/kubernetes/base/clusterrolebinding-resizer.yaml
	cd charts/aws-ebs-csi-driver && ../../bin/helm template kustomize . -s templates/clusterrolebinding-snapshotter.yaml > ../../deploy/kubernetes/base/clusterrolebinding-snapshotter.yaml
	cd charts/aws-ebs-csi-driver && ../../bin/helm template kustomize . -s templates/controller.yaml --api-versions 'snapshot.storage.k8s.io/v1' | sed -e "/namespace: /d" > ../../deploy/kubernetes/base/controller.yaml
	cd charts/aws-ebs-csi-driver && ../../bin/helm template kustomize . -s templates/csidriver.yaml > ../../deploy/kubernetes/base/csidriver.yaml
	cd charts/aws-ebs-csi-driver && ../../bin/helm template kustomize . -s templates/node.yaml | sed -e "/namespace: /d" > ../../deploy/kubernetes/base/node.yaml
	cd charts/aws-ebs-csi-driver && ../../bin/helm template kustomize . -s templates/poddisruptionbudget-controller.yaml --api-versions 'policy/v1/PodDisruptionBudget' | sed -e "/namespace: /d" > ../../deploy/kubernetes/base/poddisruptionbudget-controller.yaml
	cd charts/aws-ebs-csi-driver && ../../bin/helm template kustomize . -s templates/serviceaccount-csi-controller.yaml | sed -e "/namespace: /d" > ../../deploy/kubernetes/base/serviceaccount-csi-controller.yaml
	cd charts/aws-ebs-csi-driver && ../../bin/helm template kustomize . -s templates/serviceaccount-csi-node.yaml | sed -e "/namespace: /d" > ../../deploy/kubernetes/base/serviceaccount-csi-node.yaml
