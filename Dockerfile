# Copyright 2025 The Kubernetes Authors.
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

FROM --platform=$BUILDPLATFORM public.ecr.aws/docker/library/golang:1.27.0@sha256:4013ae0f9e7994f8535c58c811f8f863fbed38b72e0d51e6592156f758d66146 AS builder
WORKDIR /go/src/github.com/kubernetes-sigs/aws-ebs-csi-driver
RUN go env -w GOCACHE=/gocache GOMODCACHE=/gomodcache
COPY go.* .
ARG GOPROXY
RUN --mount=type=cache,target=/gomodcache go mod download
COPY . .
ARG TARGETOS
ARG TARGETARCH
ARG VERSION
ARG GOFIPS140=certified
RUN --mount=type=cache,target=/gomodcache --mount=type=cache,target=/gocache OS=$TARGETOS ARCH=$TARGETARCH GOFIPS140=$GOFIPS140 make

FROM public.ecr.aws/eks-distro-build-tooling/eks-distro-minimal-base-csi-ebs:latest-al23@sha256:d4d7b498116f79b9de7bd31f9d4b0bf35948937604d8a2895c43752f3635f304 AS linux-al2023
COPY --from=builder /go/src/github.com/kubernetes-sigs/aws-ebs-csi-driver/bin/aws-ebs-csi-driver /bin/aws-ebs-csi-driver
ENV GODEBUG=fips140=off
ENTRYPOINT ["/bin/aws-ebs-csi-driver"]

FROM public.ecr.aws/eks-distro-build-tooling/eks-distro-windows-base:1809@sha256:b4e46d9241a95ccc50ef73123714acb295472737a01cba52577dab153fdadf49 AS windows-ltsc2019
COPY --from=builder /go/src/github.com/kubernetes-sigs/aws-ebs-csi-driver/bin/aws-ebs-csi-driver.exe /aws-ebs-csi-driver.exe
ENV PATH="C:\\Windows\\System32\\WindowsPowerShell\\v1.0;${PATH}"
ENV GODEBUG=fips140=off
ENTRYPOINT ["/aws-ebs-csi-driver.exe"]

FROM public.ecr.aws/eks-distro-build-tooling/eks-distro-windows-base:ltsc2022@sha256:75f0e07262f526660444f06f5b27c834580802808489a8e6f2b7f6db11310571 AS windows-ltsc2022
COPY --from=builder /go/src/github.com/kubernetes-sigs/aws-ebs-csi-driver/bin/aws-ebs-csi-driver.exe /aws-ebs-csi-driver.exe
ENV PATH="C:\\Windows\\System32\\WindowsPowerShell\\v1.0;${PATH}"
ENV GODEBUG=fips140=off
ENTRYPOINT ["/aws-ebs-csi-driver.exe"]

FROM public.ecr.aws/eks-distro-build-tooling/eks-distro-windows-base:ltsc2025@sha256:d9c384028f72213ede7e69bd473e1dc24b35e4f9538eb4d829b02492393a3fb9 AS windows-ltsc2025
COPY --from=builder /go/src/github.com/kubernetes-sigs/aws-ebs-csi-driver/bin/aws-ebs-csi-driver.exe /aws-ebs-csi-driver.exe
ENV PATH="C:\\Windows\\System32\\WindowsPowerShell\\v1.0;${PATH}"
ENV GODEBUG=fips140=off
ENTRYPOINT ["/aws-ebs-csi-driver.exe"]
