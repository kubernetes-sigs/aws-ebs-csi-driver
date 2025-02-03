# EBS CSI Driver Dependencies Upgrade Guide

## Pre-release dependencies checklist

For code security and hygiene, it’s important to keep all the dependencies up to date ahead of each EBS CSI Driver release.

Before each release, we make a best-effort attempt at updating the following dependencies:

1. Go modules (`go get -u ./...`)
2. Go K8s dependencies are at latest patch version of K8s (excluding k8s.io/utils)
3. [GitHub Actions in the workflows](../../.github/workflows)
4. Kubernetes-csi sidecar images the [Helm Chart](../../charts/aws-ebs-csi-driver/values.yaml) + [Kustomize manifests](../../deploy/kubernetes)
5. Helm test [kubekins image](../../charts/aws-ebs-csi-driver/values.yaml)
6. gcb-docker-cloud in [cloudbuild.yaml](../../cloudbuild.yaml) 
7. [hack directory binaries](../../hack/tools/install.sh)
8. [CI E2E test configurations](../../hack/e2e/config.sh)
9. EC2 [attachment limits](../../pkg/cloud/volume_limits.go) for any new instances
10. `make update` succeeds 

Note that:
- Steps 1-3 are updated weekly by Dependabot
- Steps 4-6 are taken care of by running `make upgrade-image-dependencies`
- Steps 7-8 are not yet automated
- Step 9 can be generated with [generate-table scripts](../../hack)

## Dependabot Guide

[Dependabot](https://docs.github.com/en/code-security/getting-started/dependabot-quickstart-guide) creates PRs to keep the Go modules and GitHub Actions up-to-date. It's built-in to GitHub, used by most other [Kubernetes projects](https://github.com/kubernetes-csi/external-provisioner/blob/master/.github/dependabot.yaml), and helps us spot breaking changes in the dependencies before they delay the releases.

[.github/dependabot.yaml](../../.github/dependabot.yaml) contains the dependabot configuration file. We group together Go modules, Go k8s-dependencies, and GitHub Actions into separate weekly PRs.

### Common Workflows

For a full list of commands see [Managing pull requests for dependency updates | GitHub](https://docs.github.com/en/code-security/dependabot/working-with-dependabot/managing-pull-requests-for-dependency-updates)

#### Dependabot PR failing CI due to breaking changes?

1. Take-over the PR with something like `gh pr checkout {pr_number}` or `git pull upstream/dependabot/{rest_of_path}`
2. Make the necessary repository changes. 
3. Commit and add the string `[dependabot-skip]` to the message so Dependabot can keep rebasing the PR. Push!

Alternatively you can create a new PR and just copy the PR description. 

#### Ignore/un-ignore specific dependency upgrades

See [Dependabot documentation](https://docs.github.com/en/code-security/dependabot/working-with-dependabot/managing-pull-requests-for-dependency-updates#managing-dependabot-pull-requests-with-comment-commands)

