# Amazon Elastic Block Store (EBS) CSI driver Release Process

## Choose the release version and release branch

1. Find the latest release:
   https://github.com/kubernetes-sigs/aws-ebs-csi-driver/releases. For example,
   `v1.3.1`. (Ignore helm releases prefixed by `helm-chart` like
   `helm-chart-aws-ebs-csi-driver-2.5.0`).
2. Increment the version according to semantic versioning https://semver.org/.
   For example, for a release that only contains bug fixes or an updated Amazon
   Linux 2 base image, `v1.3.2`.
3. Find or create the corresponding release branch. Release branches correspond
   to minor version. For example, for `v1.3.2` the release branch would be
   `release-1.3` and it would already exist. For `v1.4.0` it would be
   `release-1.4` and the branch would need to be created. If you do not have
   permission to create the branch, ask an OWNER to do it
   https://github.com/kubernetes-sigs/aws-ebs-csi-driver/blob/master/OWNERS.

## Create the release commit in the release branch

Checkout the release branch you chose above, for example `git checkout release-1.3`.

### Update `CHANGELOG-0.x.md`

1. Generate a Personal Access Token with `repos` permissions.
2. Run hack/release with arguments according to the version and branch you chose above:
  - `--since`: the release version immediately preceding your chosen release version and the chosen release branch to generate the changelog. For example, for v1.3.2 pass `--since v1.3.1`.
  - `--branch`: the release branch you chose. For example, for v1.3.2 pass `--branch release-1.3`.
```
python3 hack/release --github-user=$GITHUB_USER --github-token=$GITHUB_TOKEN note --since $PREVIOUS_VERSION --branch $BRANCH
```
This will print the CHANGELOG to stdout.
3. Create a new section for the new version and copy the output there. Organize and prune the CHANGELOG at your own discretion. For example, release commits like "Release v1.3.3" are not useful and should be removed or put in a "Misc." section.

### Update `docs/README.md`

Search for any references to the previous version on the README, and update them if necessary.

### Update `Makefile`

Update the VERSION variable in the Makefile

### Send a release PR to the release branch

At this point you should have all changes required for the release commit. Verify the changes via `git diff` and send a new PR with the release commit against the release branch. Note that if it doesn't exist, you'll need someone with write privileges to create it for you.

## Tag the release

Once the PR is merged, pull the release branch locally and tag the release commit with the relase tag. You'll need push privileges for this step.

```
git checkout release-0.7
git pull upstream release-0.7
git tag v0.7.0
git push upstream v0.7.0
```

## Verify the release on GitHub

The new tag should trigger a new Github release. It should be a pre-release true because images are not available yet and documentation, like README and CHANGELOG in master branch, does not yet reflect the new release. Verify that it has run by going to [Releases](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/releases). Then, click on the new version and verify all assets have been created:

- Source code (zip)
- Source code (tar.gz)

## Promote the new image on ECR

Follow the AWS-internal process.

## Verify the images are available

In ECR Public:
  - `docker pull public.ecr.aws/ebs-csi-driver/aws-ebs-csi-driver:{release version}`

In ECR:
  - `aws ecr get-login-password --region us-west-2 | docker login --username AWS --password-stdin 602401143452.dkr.ecr.us-west-2.amazonaws.com`
  - `docker pull 602401143452.dkr.ecr.us-west-2.amazonaws.com/eks/aws-ebs-csi-driver:{release version}`

## Create the post-release commit in the release branch

### Update `charts/aws-ebs-csi-driver`

1. Update Helm `appVersion`, `version`, `tag`, and CHANGELOG
  - `charts/aws-ebs-csi-driver/Chart.yaml`
  - `charts/aws-ebs-csi-driver/values.yaml`
  - `charts/aws-ebs-csi-driver/CHANGELOG.md`

### Update `deploy/kubernetes`

1. Update the kustomize overlays
  - `deploy/kubernetes/overlays/stable/kustomization.yaml`
  - `deploy/kubernetes/overlays/stable/ecr/kustomization.yaml`
2. Run make generate-kustomize

### Send a post-release PR to the release branch

The helm and kustomize deployment files must not be updated to refer to the new images until after the images have been verified available, therefore it's necessary to make these changes in a post-release PR rather than the original release PR.

## Merge the release and post-release commits to the main branch

Send a PR to merge both the release and post-release commits to the main branch.

## Verify the helm chart release

Visit the [Releases](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/releases) pages to verify we have a new helm chart release.

## Update the GitHub release to be pre-release false

Now that images are available and documentation is updated, uncheck "This is a pre-release".

## Update AWS EKS documentation

Update the AWS EKS documentation https://docs.aws.amazon.com/eks/latest/userguide/ebs-csi.html by submitting a PR https://github.com/awsdocs/amazon-eks-user-guide/blob/master/doc_source/ebs-csi.md. For example, if the release raises the Kubernetes version requirement then the doc must reflect that.
