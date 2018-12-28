## Integration Testing

### Requirements 

1. macOS or Linux
1. `GOPATH` environment variable [set](https://github.com/golang/go/wiki/SettingGOPATH)
1. [AWS account](https://aws.amazon.com/account/) that has been [configured locally](https://docs.aws.amazon.com/cli/latest/userguide/cli-chap-configure.html)

Must satisfy also the requirements for `aws-ebs-csi-driver`

### Run Integration Tests Locally

```bash
make test-integration
```

#### Overriding Defaults
- The master branch of `aws-ebs-csi-driver` is used by default. To run using a pull request for `aws-ebs-csi-driver`, set `PULL_NUMBER` as an environment variable with a value equal to the pull request number.

- When the tests are run, a new VPC is created by default. To run using an existing VPC, set `AWS_K8S_TESTER_VPC_ID` as an environment variable with a value equal to an existing VPC ID.

### Additional Information

- GitHub [repo](https://github.com/aws/aws-k8s-tester) for `aws-k8s-tester`, which includes information about releases and running locally
- Kubernetes Enhancement Proposal ([KEP](https://github.com/kubernetes/enhancements/blob/master/keps/sig-aws/20181126-aws-k8s-tester.md)) for `aws-k8s-tester`