# EBS CSI Driver Scalability Tests

EBS uses EBS CSI Driver scalability tests to validate that each release of our driver can manage EBS volume lifecycle for large-scale clusters. 

Setup and run an EBS CSI Driver scalability test with our `scale-test` tool:  

```shell
# Set scalability parameters
export CLUSTER_TYPE="karpenter"
export TEST_TYPE="scale-sts"
export REPLICAS="1000"

# Setup an EKS scalability cluster and install EBS CSI Driver. 
./scale-test setup

# Run a scalability test and export results.
./scale-test run

# Cleanup all AWS resources related to scalability cluster. 
./scale-test cleanup
```

Results will be exported to a local directory (`$EXPORT_DIR`) and an S3 Bucket in your AWS account (`$S3_BUCKET`).

Note: Any `ebs-csi-controller` pod(s) will be restarted at the beginning of every scale run to clear metrics/logs.  

## Pre-requisites

You will need access to an AWS account role where you have [eksctl's minimum IAM policies](https://eksctl.io/usage/minimum-iam-policies/) and have permission to sync your `$S3_BUCKET`. 

Additionally, please install the following commandline tools:
- [gomplate](https://github.com/hairyhenderson/gomplate) - used to render configuration files based on environment variables.
- [aws cli v2](https://docs.aws.amazon.com/cli/latest/userguide/getting-started-install.html)
- [eksctl](https://eksctl.io/installation/)
- [kubectl](https://kubernetes.io/docs/tasks/tools/#kubectl)

## Overridable parameters

You can modify the kind of scalability cluster test run, or the names of script artifacts, through environment variables.

Note: The environment variables set when you run `scale-test setup` must remain the same for future `scale-test run`/`scale-test clean` commands on that scalability cluster.  

```sh
# Affect test
CLUSTER_TYPE              # Type of scalability cluster to create.
TEST_TYPE                 # Type of scale test to run.
REPLICAS                  # Number of StatefulSet replicas or snapshots to create.
DRIVER_VALUES_FILEPATH    # Custom values file passed to EBS CSI Driver Helm chart.

# Names
CLUSTER_NAME              # Base name used by `eksctl` to create AWS resources.
EXPORT_DIR                # Where to export scale test metrics/logs locally.
S3_BUCKET                 # Name of S3 bucket used for holding scalability run results.
SCALABILITY_TEST_RUN_NAME # Name of test run. Used as name of directory for adding run results in $S3_BUCKET.

# Snapshot
SNAPSHOTS_PER_VOLUME # How many snapshots per volume

# Find default values at top of `scale-test` script. 
```

## Types of scalability tests

Set the `CLUSTER_TYPE` and `TEST_TYPE` environment variables to set up and run different scalability tests. 

- `CLUSTER_TYPE` dictates what type of scalability cluster `scale-test` creates and which nodes are used during a scalability test run. Options include: 
  - 'pre-allocated': Additional worker nodes are created during cluster setup. By default, we pre-allocate 1 `m7a.48xlarge` EC2 instance for every 100 StatefulSet replicas.
  - 'karpenter': Installs [Karpenter](https://github.com/aws/karpenter-provider-aws) during cluster setup. Karpenter will provision and delete worker nodes during scalability test run.

- `TEST_TYPE` dictates what type of scalability test we want to run. Options include: 
  - 'scale-sts': Scales a StatefulSet to `$REPLICAS`. Waits for all pods to be ready. Delete Sts. Waits for all PVs to be deleted. Exercises the complete dynamic provisioning lifecycle for block volumes.
  - 'expand-and-modify': Creates `$REPLICAS` block volumes. Patches PVC capacity and VACName at rate of 5 PVCs per second. Ensures PVCs are expanded and modified before deleting them. Exercises ControllerExpandVolume & ControllerModifyVolume. Set `MODIFY_ONLY` or `EXPAND_ONLY` to 'true' to test solely volume modification/expansion.
  - 'snapshot-volume-scale': Creates `$REPLICAS` number of volumes. Takes `$SNAPSHOTS_PER_VOLUME` snapshots of each volume.

You can mix and match `CLUSTER_TYPE` and `TEST_TYPE`.

## Contributing scalability tests

`scale-test` parses arguments and wraps scripts and configuration files in the `helpers` directory. These helper scripts manage the scalability cluster and test runs. 

The `helpers` directory includes:
- `/helpers/cluster-setup`: Holds scripts and configuration for cluster setup/cleanup.
- `/helpers/scale-test`: Holds directory for each scale test. Also holds utility scripts used by every test (like exporting logs/metrics to S3).
