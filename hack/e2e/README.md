# Usage

run.sh will build and push a driver image, create a kops cluster, helm install the driver pointing to the built image, run ginkgo tests, then clean everything up.

See below for an example.

KOPS_STATE_FILE is an S3 bucket you have write access to.

TEST_ID is a token used for idempotency.

For more details, see the script itself.

For more examples, see the top-level Makefile.

```
TEST_PATH=./tests/e2e-migration/... \
EBS_CHECK_MIGRATION=true \
TEST_ID=18512 \
CLEAN=false \
KOPS_STATE_FILE=s3://mattwon \
AWS_REGION=us-west-2 \
AWS_AVAILABILITY_ZONES=us-west-2a \
GINKGO_FOCUS=Dynamic.\*xfs.\*should.store.data \
GINKGO_NODES=1 \
./hack/e2e/run.sh
```

# git read-tree

Reference: https://stackoverflow.com/questions/23937436/add-subdirectory-of-remote-repo-with-git-subtree

How to consume this directory by read-treeing the ebs repo:

```
git remote add ebs git@github.com:kubernetes-sigs/aws-ebs-csi-driver.git --no-tags
git fetch ebs
git read-tree --prefix=hack/e2e/ -u ebs/master:hack/e2e
```

To commit changes and submit them as a PR back to the ebs repo:

```
git diff ebs/master:hack/e2e HEAD:hack/e2e > /tmp/hack_e2e.diff
pushd $GOPATH/src/github.com/kubernetes-sigs/aws-ebs-csi-driver
git apply --reject --directory hack/e2e /tmp/hack_e2e.diff
git commit
```

To consume newer changes from the ebs repo:

```
git fetch ebs
git diff HEAD:hack/e2e ebs/master:hack/e2e > /tmp/hack_e2e.diff
git apply --reject --directory hack/e2e /tmp/hack_e2e.diff
git add hack/e2e
git commit -m "Update hack/e2e"
```
