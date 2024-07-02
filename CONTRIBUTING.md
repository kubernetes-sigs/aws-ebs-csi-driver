# Contributing Guidelines

## Prerequisites

Kubernetes projects require that you sign a Contributor License Agreement (CLA) before we can accept your pull requests.  Please see https://git.k8s.io/community/CLA.md for more info

If you are new to CSI, please go through the [CSI Spec](https://github.com/container-storage-interface/spec/blob/master/spec.md) and [General CSI driver development guideline](https://kubernetes-csi.github.io/docs/developing.html) to get some basic understanding of CSI driver before you start.

## Contributing a Patch

1. (Optional, but recommended for large changes) Submit an issue describing your proposed change to the repo in question.
1. If your proposed change is accepted, and you haven't already done so, sign a Contributor License Agreement (see details above).
1. Fork the desired repo, develop and test your code changes (see "Development Quickstart Guide" below).
1. When your changes are complete - including tests and documentation if necessary, [submit a pull request](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/compare).
1. Reviewer(s) from the [repo owners](OWNERS) will review your PR. Communicate with your reviewers and address their feedback as required.
1. After merge, your change will typically be included in the next minor release of the EBS CSI Driver and associated Helm chart, which is performed monthly.

## Development Quickstart Guide

This guide demonstrates the basic workflow for developing for the EBS CSI Driver. More detailed documentation of the available `make` targets is available in the [Makefile documentation](docs/makefile.md).

### 1. Local development

If your changes are Helm-only, skip to section 2 (manual testing) below after making your changes.

Run `make` to build a driver binary, and thus discover compiler/syntax errors. 

When your change is ready for testing, run `make test` to execute the unit test suite for the driver. If you are making a significant change to the driver (such as a bugfix or new feature), add new unit tests or update existing tests where applicable.

### 2. Cluster setup

When your change is ready to test in a real Kubernetes cluster, create a `kops` cluster to test the driver against:
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

Export the `KUBECONFIG` environment variable to access the cluster for manual testing:
```bash
eval "$(make cluster/kubeconfig)"
```

To test a change, build the driver image and then install it:
```bash
make cluster/image
# If testing a Helm-based change, you can use HELM_EXTRA_FLAGS
# to set your new paremeters for testing, for example:
# export HELM_EXTRA_FLAGS="--set=controller.newParameter=true"
make cluster/install
```

When you are finished manually testing, uninstall the driver:
```bash
make cluster/uninstall
```

### 3. Automated E2E tests

Before running any E2E tests, you must create a cluster (see step 2) and build an image if you have not already done so:
```bash
make cluster/image
```

The driver should not be preinstalled when running E2E tests (the tests will automatically install the driver). If it is, uninstall it first:
```bash
make cluster/uninstall
```

If you are making a change to the driver, the recommended test suite to run is the external tests:
```bash
# Normal external tests (excluding Windows tests)
make e2e/external
# Additionally, run the windows tests if changing Windows behavior
make e2e/external-windows
```

If you are making a change to the Helm chart, the recommended test suite to run is the Helm `ct` tests:
```bash
make e2e/helm-ct
```

After finishing testing, cleanup your cluster :
```bash
make cluster/delete
```

### 4. Before submitting a PR

Run `make update` to automatically format go source files and re-generate automatically generated files. If `make update` produces any changes, commit them before submitting a PR.

Run `make verify` to run linters and other similar code checking tools. Fix any issues `make verify` discovers before submitting a PR.

Your PR should contain a full set of unit/E2E tests when applicable. If introducing a change that is not exercised fully by automated testing (such as a new Helm parameter), please provide evidence of the feature working as intended in the PR description.
