# Kubernetes tests

This directory imports tests from kubernetes/kubernetes and enables:

* External CSI tests, https://github.com/kubernetes/kubernetes/tree/master/test/e2e/storage/external
* CSI migration tests

# Prerequisites
- kubernetes 1.16+ AWS cluster

# Run
* External CSI tests: `FOCUS=External.Storage`.
* CSI migration tests: `FOCUS=ebs-csi-migration`.

```sh
go test -v -timeout 0 ./... -kubeconfig=$HOME/.kube/config -report-dir=$ARTIFACTS -ginkgo.focus="$FOCUS" -ginkgo.skip="\[Disruptive\]" -gce-zone=us-west-2a
```

# Update dependencies
```sh
go mod edit -require=k8s.io/kubernetes@v1.15.3
./hack/update-gomod.sh v1.15.3
```