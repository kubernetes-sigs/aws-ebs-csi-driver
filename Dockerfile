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

FROM --platform=$BUILDPLATFORM public.ecr.aws/docker/library/golang:1.27.4@sha256:205c7636d74e0ffbe3d007162df7d74f859f5a8c8322dfb9f907977686c126cf AS builder
WORKDIR /go/src/github.com/kubernetes-sigs/aws-ebs-csi-driver
RUN go env -w GOCACHE=/gocache GOMODCACHE=/gomodcache
COPY go.* .
ARG GOPROXY
RUN --mount=type=cache,target=/gomodcache go mod download
COPY . .
ARG TARGETOS
ARG TARGETARCH
ARG VERSION
RUN --mount=type=cache,target=/gomodcache --mount=type=cache,target=/gocache OS=$TARGETOS ARCH=$TARGETARCH GOFIPS140=certified make

FROM public.ecr.aws/eks-distro-build-tooling/eks-distro-minimal-base-csi-ebs:latest-al23@sha256:61519467fcad0efeb0bb7e3882002b60dae2467933cfb9b0e7295c32d6631dc3 AS linux-al2023
COPY --from=builder /go/src/github.com/kubernetes-sigs/aws-ebs-csi-driver/bin/aws-ebs-csi-driver /bin/aws-ebs-csi-driver
ENV GODEBUG=fips140=off
ENTRYPOINT ["/bin/aws-ebs-csi-driver"]

FROM public.ecr.aws/eks-distro-build-tooling/eks-distro-windows-base:1809@sha256:13003564f90ad02c3e75f463cef9a2634fc52036c9ff41d291a53b5e7dab5eca AS windows-ltsc2019
COPY --from=builder /go/src/github.com/kubernetes-sigs/aws-ebs-csi-driver/bin/aws-ebs-csi-driver.exe /aws-ebs-csi-driver.exe
ENV PATH="C:\\Windows\\System32\\WindowsPowerShell\\v1.0;${PATH}"
ENV GODEBUG=fips140=off
ENTRYPOINT ["/aws-ebs-csi-driver.exe"]

FROM public.ecr.aws/eks-distro-build-tooling/eks-distro-windows-base:ltsc2022@sha256:d0823369e0f5d4f5a943c2ff3d4d1ff12ecef38f1ea8488ef785680ebfca4581 AS windows-ltsc2022
COPY --from=builder /go/src/github.com/kubernetes-sigs/aws-ebs-csi-driver/bin/aws-ebs-csi-driver.exe /aws-ebs-csi-driver.exe
ENV PATH="C:\\Windows\\System32\\WindowsPowerShell\\v1.0;${PATH}"
ENV GODEBUG=fips140=off
ENTRYPOINT ["/aws-ebs-csi-driver.exe"]

FROM public.ecr.aws/eks-distro-build-tooling/eks-distro-windows-base:ltsc2025@sha256:de7d5fb011ca4b9f1e924906839569bc8043d309248897325526302d887dcb4a AS windows-ltsc2025
COPY --from=builder /go/src/github.com/kubernetes-sigs/aws-ebs-csi-driver/bin/aws-ebs-csi-driver.exe /aws-ebs-csi-driver.exe
ENV PATH="C:\\Windows\\System32\\WindowsPowerShell\\v1.0;${PATH}"
ENV GODEBUG=fips140=off
ENTRYPOINT ["/aws-ebs-csi-driver.exe"]
