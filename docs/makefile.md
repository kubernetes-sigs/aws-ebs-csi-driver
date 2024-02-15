# Use of the EBS CSI Driver `Makefile`

The EBS CSI Driver comes with a Makefile that can be used to develop, build, test, and release the driver. This file documents Makefile targets, the parameters they support, and common usage scenarios.

## Prerequisites

The `Makefile` has the following dependencies:
- `go`: https://go.dev/doc/install
- `python` (may be named `python3`) and `pip`: https://www.python.org/downloads/
- `jq`: https://github.com/jqlang/jq/releases
- `kubectl`: https://kubernetes.io/docs/tasks/tools/#kubectl
- `git`: https://git-scm.com/downloads
- `docker` and `docker buildx`: https://docs.docker.com/get-docker/ and https://github.com/docker/buildx#installing
- `make`
- Standard POSIX tools (`awk`, `grep`, `cat`, etc)

All other tools are downloaded for you at runtime.

## Quickstart Guide

This guide demonstrates the basic workflow for developing for the EBS CSI Driver. More detailed documentation of the available `make` targets is available below.

### 1. Local development for the EBS CSI Driver

If your changes are Helm-only, skip to section 2 (Run E2E tests) below after making your changes.

During development, use `make` at any time to build a driver binary, and thus discover compiler errors. When your change is ready to be tested, run `make test` to execute the unit test suite for the driver. If you are making a significant change to the driver (such as a bugfix or new feature), please add new unit tests or update existing tests where applicable.

### 2. Run E2E tests

To create a `kops` cluster to test the driver against:
```bash
make cluster/create
```

If your change affects the Windows implementation of the driver, instead create an `eksctl` cluster with Windows nodes:
```bash
export WINDOWS="true"
# Note: CLUSTER_TYPE must be set for all e2e tests and cluster deletion
# Re-export it if necessary (for example, if running tests in a separate terminal tab)
export CLUSTER_TYPE="eksctl"
make cluster/create
```

If you are making a change to the driver, the recommended test suite to run is the external tests:
```bash
# Normal external tests (excluding Windows tests)
make e2e/external
# Instead, if testing Windows
make e2e/external-windows
```

If you are making a change to the Helm chart, the recommended test suite to run is the Helm `ct` tests:
```bash
make e2e/helm-ct
```

To cleanup your cluster after finishing testing:
```bash
make cluster/delete
```

### 3. Before submitting a PR

Run `make update` to automatically format go source files and re-generate automatically generated files. If `make update` produces any changes, commit them before submitting a PR.

Run `make verify` to run linters and other similar code checking tools. Fix any issues `make verify` discovers before submitting a PR.

## Building

### `make` or `make bin/aws-ebs-csi-driver` or `make bin/aws-ebs-csi-driver.exe`

Build a binary copy of the EBS CSI Driver for the local platform. This is the default behavior when calling `make` with no target.

The target OS and/or architecture can be overridden via the `OS` and `ARCH` environment variables (for example, `OS=linux ARCH=arm64 make`)

### `make image`

Build and push a single image of the driver based on the local platform (the same overrides as `make` apply, as well as `OSVERSION` to override container OS version). In most cases, `make all-push` is more suitable. Environment variables are accepted to override the `REGISTRY`, `IMAGE` name, and image `TAG`.

### `make all-push`

Build and push a multi-arch image of the driver based on the OSes in `ALL_OS`, architectures in `ALL_ARCH_linux`/`ALL_ARCH_windows`, and OS versions in `ALL_OSVERSION_linux`/`ALL_OSVERSION_windows`. Also supports `REGISTRY`, `IMAGE`, and `TAG`.

## Local Development

### `make test`

Run all unit tests with race condition checking enabled.

### `make test/coverage`

Outputs a filtered version of the each package's unit test coverage profiling via go's coverage tool to a local `coverage.html` file.

### `make test-sanity`

Run the official [CSI sanity tests](https://github.com/kubernetes-csi/csi-test). _Warning: Currently, 3 of the tests are known to fail incorrectly._

### `make verify`

Performs local verification that other than unit tests (linters, manifest updates, etc)

### `make update`

Updates Kustomize manifests, formatting, and tidies `go.mod`. `make verify` will ensure that `make update` was run by checking if it creates a diff.

## Cluster Management

### `make cluster/create`

Creates a cluster for running E2E tests against. There are many parameters that can be provided via environment variables, a full list is available in [`config.sh`](../hack/e2e/config.sh), but the primary parameters are:

- `CLUSTER_TYPE`: The tool used to create the cluster, either `kops` or `eksctl` - defaults to `kops`
- `CLUSTER_NAME`: The name of the cluster to create - defaults to `ebs-csi-e2e.k8s.local`
- `INSTANCE_TYPE`: The instance type to use for cluster nodes - defaults to `c5.large`
- `AMI_PARAMETER`: The SSM parameter of where to get the AMI for the cluster nodes (`kops` clusters only) - defaults to `/aws/service/ami-amazon-linux-latest/al2023-ami-kernel-default-x86_64`
- `WINDOWS`: Whether or not to create a Windows node group for the cluster (`eksctl` clusters only) - defaults to `false`
- `AWS_REGION`: Which region to create the cluster in - defaults to `us-west-2`
- `AWS_AVAILABILITY_ZONES`: Which AZs to create nodes for the cluster in - defaults to `us-west-2a,us-west-2b,us-west-2c`

#### Example: Create a default (`kops`) cluster

```bash
make cluster/create
```

#### Example: Create a cluster with only one Availability Zone
```bash
export AWS_AVAILABILITY_ZONES="us-west-2a"
make cluster/create
```

#### Example: Create an `eksctl` cluster

```bash
export CLUSTER_TYPE="eksctl"
make cluster/create
```

#### Example: Create a cluster with Graviton nodes for `arm64` testing

```bash
export INSTANCE_TYPE="m7g.medium"
export AMI_PARAMETER="/aws/service/ami-amazon-linux-latest/al2023-ami-kernel-default-arm64"
make cluster/create
```

#### Example: Create a cluster with Windows nodes

```bash
export WINDOWS="true"
export CLUSTER_TYPE="eksctl"
make cluster/create
```

### `make cluster/delete`

Deletes a cluster created by `make cluster/create`. You must pass the same `CLUSTER_TYPE` and `CLUSTER_NAME` as used when creating the cluster.

## E2E Tests

Run E2E tests against a cluster created by `make cluster/create`. You must pass the same `CLUSTER_TYPE` and `CLUSTER_NAME` as used when creating the cluster.

Alternatively, you may run on an externally created cluster by passing `CLUSTER_TYPE` (required to determine which `values.yaml` to deploy) and `KUBECONFIG`. For `kops` clusters, the node IAM role should include the appropriate IAM policies to use the driver (see [the installation docs](./install.md#set-up-driver-permissions)). For `eksctl` clusters, the `ebs-csi-controller-sa` service account should be pre-created and setup to supply an IRSA role with the appropriate policies.

### `make e2e/external`

Run the Kubernetes upstream [external storage E2E tests](https://github.com/kubernetes/kubernetes/blob/master/test/e2e/README.md). This is the most comprehensive E2E test, recommended for local development.

### `make e2e/single-az`

Run the single-AZ EBS CSI E2E tests. Requires a cluster with only one Availability Zone.

### `make e2e/multi-az`

Run the multi-AZ EBS CSI E2E tests. Requires a cluster with at least two Availability Zones.

### `make e2e/external-arm64`

Run the Kubernetes upstream [external storage E2E tests](https://github.com/kubernetes/kubernetes/blob/master/test/e2e/README.md) using an ARM64 image of the EBS CSI Driver. Requires a cluster with Graviton nodes.

### `make e2e/external-windows`

Run the Kubernetes upstream [external storage E2E tests](https://github.com/kubernetes/kubernetes/blob/master/test/e2e/README.md) with Windows tests enabled. Requires a cluster with Windows nodes.

### `make e2e/external-kustomize`

Run the Kubernetes upstream [external storage E2E tests](https://github.com/kubernetes/kubernetes/blob/master/test/e2e/README.md), but using `kustomize` to deploy the driver instead of `helm`.

### `make e2e/helm-ct`

Test the EBS CSI Driver Helm chart via the [Helm `chart-testing` tool](https://github.com/helm/chart-testing).

## Release Scripts

### `make update-sidecar-dependencies`

Convenience target to perform all sidecar updates and regenerate the manifests. This is the primary target to use unless more granular control is needed.

### `make update-truth-sidecars`

Retrieves the latest sidecar container images and creates or updates `hack/release-scripts/image-digests.yaml`.

### `make generate-sidecar-tags`

Updates the Kustomize and Helm sidecar tags with the values from `hack/release-scripts/image-digests.yaml`.
