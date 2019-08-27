module github.com/kubernetes-sigs/aws-ebs-csi-driver/tests/e2e-migration

go 1.12

replace k8s.io/api => k8s.io/api v0.0.0-20190822053644-6185379c914a

replace k8s.io/apiextensions-apiserver => k8s.io/apiextensions-apiserver v0.0.0-20190822063004-0670dc4fec4e

replace k8s.io/apimachinery => k8s.io/apimachinery v0.0.0-20190820074809-31b1e1ea64dc

replace k8s.io/apiserver => k8s.io/apiserver v0.0.0-20190822060508-785eacbd19ae

replace k8s.io/cli-runtime => k8s.io/cli-runtime v0.0.0-20190822063658-442a64f3fed7

replace k8s.io/client-go => k8s.io/client-go v0.0.0-20190822054823-0a74433fb222

replace k8s.io/cloud-provider => k8s.io/cloud-provider v0.0.0-20190822065847-2058b41dfbb6

replace k8s.io/cluster-bootstrap => k8s.io/cluster-bootstrap v0.0.0-20190822065536-566e5fc137f7

replace k8s.io/code-generator => k8s.io/code-generator v0.0.0-20190820100630-060a3d12ce80

replace k8s.io/component-base => k8s.io/component-base v0.0.0-20190822055535-1f6a258f5d89

replace k8s.io/cri-api => k8s.io/cri-api v0.0.0-20190820110325-95eec93e2395

replace k8s.io/csi-translation-lib => k8s.io/csi-translation-lib v0.0.0-20190822070154-f51cd605b3ee

replace k8s.io/kube-aggregator => k8s.io/kube-aggregator v0.0.0-20190822061015-a4f93a8219ed

replace k8s.io/kube-controller-manager => k8s.io/kube-controller-manager v0.0.0-20190822065235-826221481525

replace k8s.io/kube-proxy => k8s.io/kube-proxy v0.0.0-20190822064323-7e0495d8a3ff

replace k8s.io/kube-scheduler => k8s.io/kube-scheduler v0.0.0-20190822064931-4470440ed041

replace k8s.io/kubectl => k8s.io/kubectl v0.0.0-20190822071625-14af4a63a1e1

replace k8s.io/kubelet => k8s.io/kubelet v0.0.0-20190822064626-fa8f3d935631

replace k8s.io/legacy-cloud-providers => k8s.io/legacy-cloud-providers v0.0.0-20190822070624-3a30a18bba71

replace k8s.io/metrics => k8s.io/metrics v0.0.0-20190822063337-6c03eb8600ee

replace k8s.io/node-api => k8s.io/node-api v0.0.0-20190822070940-24e163ffb9e7

replace k8s.io/sample-apiserver => k8s.io/sample-apiserver v0.0.0-20190822061642-ab22eab63834

replace k8s.io/sample-cli-plugin => k8s.io/sample-cli-plugin v0.0.0-20190822064016-bcca3cc588da

replace k8s.io/sample-controller => k8s.io/sample-controller v0.0.0-20190822062306-1b561d990eb5

require (
	github.com/onsi/ginkgo v1.9.0
	github.com/onsi/gomega v1.6.0
	k8s.io/kubernetes v1.16.0-beta.1
)
