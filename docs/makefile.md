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

## Building

### `make cluster/image`

Build and push a single image of the driver based on the local platform (the same overrides as `make` apply, as well as `OSVERSION` to override container OS version). In most cases, `make all-push` is more suitable. Environment variables are accepted to override the `REGISTRY`, `IMAGE` name, and image `TAG`.

## Local Development

### `make` or `make bin/aws-ebs-csi-driver` or `make bin/aws-ebs-csi-driver.exe`

Build a binary copy of the EBS CSI Driver for the local platform. This is the default behavior when calling `make` with no target.

The target OS and/or architecture can be overridden via the `OS` and `ARCH` environment variables (for example, `OS=linux ARCH=arm64 make`)

### `make test`

Run all unit tests with race condition checking enabled.

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
- `OUTPOST_ARN`: If set, create an additional nodegroup on an [outpost](https://aws.amazon.com/outposts/) (`eksctl clusters only)
- `OUTPOST_INSTANCE_TYPE`: The instance type to use for the outpost nodegroup (only used when `OUTPOST_ARN` is non-empty) - defaults to `INSTANCE_TYPE`

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

#### Example: Create a cluster with an outpost nodegroup

```bash
export CLUSTER_TYPE="eksctl"
export OUTPOST_ARN="arn:aws:outposts:us-east-1:123456789012:outpost/op-0f39f7c0af9b166a3"
export OUTPOST_INSTANCE_TYPE=c5.xlarge
make cluster/create
```

### `make cluster/image`

Builds an image for use in the E2E tests. This will automatically build the most appropriate image (for example, skipping Windows builds unless `WINDOWS` is set to `true`).

#### Example: Build a standard image

```bash
make cluster/image
```

#### Example: Build an arm64 image

```bash
export IMAGE_ARCH="arm64"
make cluster/image
```

#### Example: Build a Windows-compatible image

```bash
export WINDOWS="true"
make cluster/image
```

### `make cluster/kubeconfig`

Prints the `KUBECONFIG` environment variable for a cluster. You must pass the same `CLUSTER_TYPE` and `CLUSTER_NAME` as used when creating the cluster. This command must be `eval`ed to import the environment variables into your shell.

#### Example: Export the `KUBECONFIG` for a default cluster

```bash
eval "$(make cluster/kubeconfig)"
```

#### Example: Export the `KUBECONFIG` for an `eksctl` cluster

```bash
export CLUSTER_TYPE="eksctl"
eval "$(make cluster/kubeconfig)"
```

### `make cluster/delete`

Deletes a cluster created by `make cluster/create`. You must pass the same `CLUSTER_TYPE` and `CLUSTER_NAME` as used when creating the cluster.

### `make cluster/install`

Install the EBS CSI Driver to the cluster via Helm. You must have already run `make cluster/image` to build the image for the cluster, or provide an image of your own.

#### Example: Install the EBS CSI Driver to a cluster for testing

```bash
make cluster/install
```

### `make cluster/uninstall`

Uninstall an installation of the EBS CSI Driver previously installed by `make cluster/install`.

#### Example: Uninstall the EBS CSI Driver

```bash
make cluster/uninstall
```

## E2E Tests

Run E2E tests against a cluster created by `make cluster/create`. You must pass the same `CLUSTER_TYPE` and `CLUSTER_NAME` as used when creating the cluster. You must have already run `make cluster/image` to build the image for the cluster, or provide an image of your own.

Alternatively, you may run on an externally created cluster by passing `CLUSTER_TYPE` (required to determine which `values.yaml` to deploy) and `KUBECONFIG`. For `kops` clusters, the node IAM role should include the appropriate IAM policies to use the driver (see [the installation docs](./install.md#set-up-driver-permissions)). For `eksctl` clusters, the `ebs-csi-controller-sa` service account should be pre-created and setup to supply an IRSA role with the appropriate policies.

### `make e2e/external`

Run the Kubernetes upstream [external storage E2E tests](https://github.com/kubernetes/kubernetes/blob/master/test/e2e/README.md). This is the most comprehensive E2E test, recommended for local development.

### `make e2e/single-az`

Run the single-AZ EBS CSI E2E tests. Requires a cluster with only one Availability Zone.

### `make e2e/multi-az`

Run the multi-AZ EBS CSI E2E tests. Requires a cluster with at least two Availability Zones.

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
