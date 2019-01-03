## Integration Testing
Integration test verifies the functionality of EBS CSI driver as a standalone server outside Kubernetes. It exercises the lifecycle of the volume by creating, attaching, staging, mounting volumes and the reverse operations. And it verifies data can be written onto an EBS volume without any issue.

## Run Integration Tests Locally
The integration test is executed using [aws-k8s-tester](https://github.com/aws/aws-k8s-tester) which is CLI tool for k8s testing on AWS. With aws-k8s-tester, it automates the process of provisioning EC2 instance, pulling down and building EBS CSI driver, running the defined integration test and sending test result back. See aws-k8s-tester for more details about how to use it.

### Requirements
1. AWS credential is [configured](https://docs.aws.amazon.com/cli/latest/userguide/cli-chap-configure.html)
1. Other requirements needed by aws-k8s-tester

```
make test-integration
```

#### Overriding Defaults
- The master branch of `aws-ebs-csi-driver` is used by default. To run using a pull request for `aws-ebs-csi-driver`, set `PULL_NUMBER` as an environment variable with a value equal to the pull request number.

- When the tests are run, a new Amazon Virtual Private Cloud (VPC) is created by default. To run using an existing VPC, set `AWS_K8S_TESTER_VPC_ID` as an environment variable with a value equal to an existing VPC ID. This will be useful when VPC limit is reached in the region under test.
