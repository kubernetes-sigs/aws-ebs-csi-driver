## E2E Testing
E2E test verifies the functionality of EBS CSI driver in the context of Kubernetes. It exercises driver feature e2e including static provisioning, dynamic provisioning, volume scheduling, mount options, etc.

### Requirements
1. AWS credential is [configured](https://docs.aws.amazon.com/cli/latest/userguide/cli-chap-configure.html)
1. AWS CLI v1.16+
1. Kubectl v1.13+
1. Docker CLI v18.09+
1. curl
1. sed
1. Golang 1.11+

### Running a specific test
1. Make sure you have a cluster up with ebs csi driver deployed
2. Current directory is `/aws-ebs-csi-driver/tests/e2e`
3. Run `'ginkgo run --focus='should create multiple PV objects, bind to PVCs and attach all to a single pod'`

### Notes
Some tests marked with `[env]` require specific environmental variables to be set, if not set these tests will be skipped.

```
export AWS_AVAILABILITY_ZONES="us-west-2a,us-west-2b"
```
 
Replacing `us-west-2a,us-west-2b` with the AZ(s) where your Kubernetes worker nodes are located.

By default `make test-e2e-` targets will run 32 tests concurrently, set `GINKGO_NODES` to change the parallelism.


