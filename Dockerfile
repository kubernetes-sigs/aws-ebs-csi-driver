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

# See
# https://docs.docker.com/engine/reference/builder/#automatic-platform-args-in-the-global-scope
# for info on BUILDPLATFORM, TARGETOS, TARGETARCH, etc.
FROM --platform=$BUILDPLATFORM public.ecr.aws/docker/library/golang:1.25@sha256:8305f5fa8ea63c7b5bc85bd223ccc62941f852318ebfbd22f53bbd0b358c07e1 AS builder
WORKDIR /go/src/github.com/kubernetes-sigs/aws-ebs-csi-driver
RUN go env -w GOCACHE=/gocache GOMODCACHE=/gomodcache
COPY go.* .
ARG GOPROXY
RUN --mount=type=cache,target=/gomodcache go mod download
COPY . .
ARG TARGETOS
ARG TARGETARCH
ARG VERSION
ARG GOEXPERIMENT
RUN --mount=type=cache,target=/gomodcache --mount=type=cache,target=/gocache OS=$TARGETOS ARCH=$TARGETARCH make

FROM public.ecr.aws/eks-distro-build-tooling/eks-distro-minimal-base-csi-ebs:latest-al23@sha256:cc8243a719217c06acfb957fe8dd05eb26957d48d5b52e9a073170722a10c498 AS linux-al2023
COPY --from=builder /go/src/github.com/kubernetes-sigs/aws-ebs-csi-driver/bin/aws-ebs-csi-driver /bin/aws-ebs-csi-driver
ENTRYPOINT ["/bin/aws-ebs-csi-driver"]

FROM public.ecr.aws/eks-distro-build-tooling/eks-distro-minimal-base-csi-ebs:latest-al2@sha256:78017dc239f8b948676d84ea3a06d9b4fda32de1c518d5ad38fd907841ba99c3 AS linux-al2
COPY --from=builder /go/src/github.com/kubernetes-sigs/aws-ebs-csi-driver/bin/aws-ebs-csi-driver /bin/aws-ebs-csi-driver
ENTRYPOINT ["/bin/aws-ebs-csi-driver"]

FROM mcr.microsoft.com/windows/nanoserver:ltsc2019@sha256:ea1b43fa8972684a5a284a6f441f91991fa7545d6912d2aecbf6c5ba60e73155 AS windows-ltsc2019
COPY --from=builder /go/src/github.com/kubernetes-sigs/aws-ebs-csi-driver/bin/aws-ebs-csi-driver.exe /aws-ebs-csi-driver.exe
ENV PATH="C:\\Windows\\System32\\WindowsPowerShell\\v1.0;${PATH}"
ENTRYPOINT ["/aws-ebs-csi-driver.exe"]

FROM mcr.microsoft.com/windows/nanoserver:ltsc2022@sha256:580b7fa4040be7b47d79c25fb73e3d6da2e68f32b95d9d4dfb70bde33564fc4a AS windows-ltsc2022
COPY --from=builder /go/src/github.com/kubernetes-sigs/aws-ebs-csi-driver/bin/aws-ebs-csi-driver.exe /aws-ebs-csi-driver.exe
ENV PATH="C:\\Windows\\System32\\WindowsPowerShell\\v1.0;${PATH}"
ENTRYPOINT ["/aws-ebs-csi-driver.exe"]

FROM mcr.microsoft.com/windows/nanoserver:ltsc2025@sha256:252ba371c2f50ae797d81ae9d87405b55119714817fead9f406218b556e3ddeb AS windows-ltsc2025
COPY --from=builder /go/src/github.com/kubernetes-sigs/aws-ebs-csi-driver/bin/aws-ebs-csi-driver.exe /aws-ebs-csi-driver.exe
ENV PATH="C:\\Windows\\System32\\WindowsPowerShell\\v1.0;${PATH}"
ENTRYPOINT ["/aws-ebs-csi-driver.exe"]
