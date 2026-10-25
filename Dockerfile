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

FROM --platform=$BUILDPLATFORM public.ecr.aws/docker/library/golang:1.27.3@sha256:957861185dbf29d93668ace41e011d091c6e779953d47f0316ef2cc58e28fe2b AS builder
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

FROM public.ecr.aws/eks-distro-build-tooling/eks-distro-minimal-base-csi-ebs:latest-al23@sha256:b55e9aceadcc0258636207dcf5f184de294cb0d922e448eefc0cdefa21a83adc AS linux-al2023
COPY --from=builder /go/src/github.com/kubernetes-sigs/aws-ebs-csi-driver/bin/aws-ebs-csi-driver /bin/aws-ebs-csi-driver
ENV GODEBUG=fips140=off
ENTRYPOINT ["/bin/aws-ebs-csi-driver"]

FROM public.ecr.aws/eks-distro-build-tooling/eks-distro-windows-base:1809@sha256:4287cb03c77a1ccbb7a4f1cbe280545f5a22522b22389cb2acc9c89503a471bc AS windows-ltsc2019
COPY --from=builder /go/src/github.com/kubernetes-sigs/aws-ebs-csi-driver/bin/aws-ebs-csi-driver.exe /aws-ebs-csi-driver.exe
ENV PATH="C:\\Windows\\System32\\WindowsPowerShell\\v1.0;${PATH}"
ENV GODEBUG=fips140=off
ENTRYPOINT ["/aws-ebs-csi-driver.exe"]

FROM public.ecr.aws/eks-distro-build-tooling/eks-distro-windows-base:ltsc2022@sha256:feb7680e34f3edaa8c34800496b785570feafdc0888465990ba33a4faddd5156 AS windows-ltsc2022
COPY --from=builder /go/src/github.com/kubernetes-sigs/aws-ebs-csi-driver/bin/aws-ebs-csi-driver.exe /aws-ebs-csi-driver.exe
ENV PATH="C:\\Windows\\System32\\WindowsPowerShell\\v1.0;${PATH}"
ENV GODEBUG=fips140=off
ENTRYPOINT ["/aws-ebs-csi-driver.exe"]

FROM public.ecr.aws/eks-distro-build-tooling/eks-distro-windows-base:ltsc2025@sha256:23b6f5c624c0fd7ef4dea247a205641cc0fc3f72acec67fb422e4a523b7ebcc1 AS windows-ltsc2025
COPY --from=builder /go/src/github.com/kubernetes-sigs/aws-ebs-csi-driver/bin/aws-ebs-csi-driver.exe /aws-ebs-csi-driver.exe
ENV PATH="C:\\Windows\\System32\\WindowsPowerShell\\v1.0;${PATH}"
ENV GODEBUG=fips140=off
ENTRYPOINT ["/aws-ebs-csi-driver.exe"]
