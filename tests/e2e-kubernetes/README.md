# Prerequisites
- kubernetes 1.16+ AWS cluster

# Run
```sh
go test -v -timeout 0 ./... -kubeconfig=$HOME/.kube/config -report-dir=$ARTIFACTS -ginkgo.focus="\[ebs-csi-migration\]" -ginkgo.skip="\[Disruptive\]" -gce-zone=us-west-2a
```

# Update dependencies
```sh
go mod edit -require=k8s.io/kubernetes@v1.15.3
./hack/update-gomod.sh v1.15.3
```