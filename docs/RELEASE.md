# Amazon Elastic Block Store (EBS) CSI driver Release Process
NOTE: Your GitHub account must have the required permissions and you must have generated a GitHub token.

## Choosing the release version
Using semantic versioning, pick a release number that makes sense by bumping the major, minor or patch release version.  If its a major or minor release (backwards incompatible changes, and new features, respectively) then you will want to start this process with an alpha release first.  Here are some examples:

Bumping a minor version after releasing a new feature:
```
v1.4.5 -> v1.5.0-alpha.0
```

After testing and allowing some time for feedback on the alpha, releasing v1.5.0:
```
v1.4.5 -> v1.5.0
```

New patch release:
```
v1.5.3 -> v1.5.4
```

New major version release with two alpha releases:
```
v1.6.2 -> v2.0.0-alpha.0
       -> v2.0.0-alpha.1
       -> v2.0.0
```

## Choosing the release branch
You also might need to create a release branch, if it doesn't already exist. For example, in the case that we are backporting a fix to the v0.5 release branch, then we would do the following:

1. Create the release branch (named release-0.5) if it doesn't exist or check it out if it already exists.
2. Cherry-pick the necessary commits onto the release branch.
3. Follow the instructions below to create the release commit.
4. Create a pull request to merge your fork of the release branch into the upstream release branch (i.e. <user>/aws-ebs-csi-driver/release-0.5 -> kubernetes-sigs/aws-ebs-csi-driver/release-0.5).

## Create the release commit in the release branch

### Update `CHANGELOG-0.x.md`
We need to generate the CHANGELOG for the new release by running `./hack/release`. You need to pass previous release tag to generate the changelog.

```
python3 release --github-user=ayberk --github-token=$GITHUB_TOKEN note --since <previous_version_tag>
```

This will print the CHANGELOG to stdout. You should create a new section for the new version and copy the output there.

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
The new tag should trigger a new Github release. Verify that it has run by going to [Releases](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/releases). Then, click on the new version and verify all assets have been created:

- Source code (zip)
- Source code (tar.gz)

## Promote the new image on ECR
Follow the AWS-internal process.

## Verify the images are available
In GCR:
  - `docker pull k8s.gcr.io/provider-aws/aws-ebs-csi-driver:v1.1.1`

In ECR:
  - `aws ecr get-login-password --region us-west-2 | docker login --username AWS --password-stdin 602401143452.dkr.ecr.us-west-2.amazonaws.com`
  - `docker pull 602401143452.dkr.ecr.us-west-2.amazonaws.com/eks/aws-ebs-csi-driver:v1.1.1`

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

### Send a post-release PR to the release branch
The helm and kustomize deployment files must not be updated to refer to the new images until after the images have been verified available, therefore it's necessary to make these changes in a post-release PR rather than the original release PR.

## Merge the release and post-release commits to the main branch

Send a PR to merge both the release and post-release commits to the main branch.

## Verify the helm chart release

Visit the [Releases](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/releases) pages to verify we have a new helm chart release.

## Update AWS EKS documentation

Update the AWS EKS documentation https://docs.aws.amazon.com/eks/latest/userguide/ebs-csi.html by submitting a PR https://github.com/awsdocs/amazon-eks-user-guide/blob/master/doc_source/ebs-csi.md. For example, if the release raises the Kubernetes version requirement then the doc must reflect that.
