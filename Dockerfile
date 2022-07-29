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
FROM --platform=$BUILDPLATFORM golang:1.17 AS builder
WORKDIR /go/src/github.com/kubernetes-sigs/aws-ebs-csi-driver
COPY go.* .
ARG GOPROXY
RUN go mod download
COPY . .
ARG TARGETOS
ARG TARGETARCH
RUN OS=$TARGETOS ARCH=$TARGETARCH make $TARGETOS/$TARGETARCH

FROM public.ecr.aws/eks-distro-build-tooling/eks-distro-minimal-base-csi-ebs:latest.2 AS linux-amazon
COPY --from=builder /go/src/github.com/kubernetes-sigs/aws-ebs-csi-driver/bin/aws-ebs-csi-driver /bin/aws-ebs-csi-driver
ENTRYPOINT ["/bin/aws-ebs-csi-driver"]

FROM mcr.microsoft.com/windows/servercore:1809 AS windows-1809
COPY --from=builder /go/src/github.com/kubernetes-sigs/aws-ebs-csi-driver/bin/aws-ebs-csi-driver.exe /aws-ebs-csi-driver.exe
ENTRYPOINT ["/aws-ebs-csi-driver.exe"]

FROM mcr.microsoft.com/windows/servercore:20H2 AS windows-20H2
COPY --from=builder /go/src/github.com/kubernetes-sigs/aws-ebs-csi-driver/bin/aws-ebs-csi-driver.exe /aws-ebs-csi-driver.exe
ENTRYPOINT ["/aws-ebs-csi-driver.exe"]

FROM mcr.microsoft.com/windows/servercore:ltsc2019 AS windows-ltsc2019
COPY --from=builder /go/src/github.com/kubernetes-sigs/aws-ebs-csi-driver/bin/aws-ebs-csi-driver.exe /aws-ebs-csi-driver.exe
ENTRYPOINT ["/aws-ebs-csi-driver.exe"]
