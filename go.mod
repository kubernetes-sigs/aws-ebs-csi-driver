module github.com/kubernetes-sigs/aws-ebs-csi-driver

require (
	github.com/aws/aws-sdk-go v1.44.108
	github.com/container-storage-interface/spec v1.6.0
	github.com/golang/mock v1.6.0
	github.com/golang/protobuf v1.5.2
	github.com/google/go-cmp v0.5.9
	github.com/kubernetes-csi/csi-proxy/client v1.1.1
	github.com/kubernetes-csi/csi-test v2.2.0+incompatible
	github.com/kubernetes-csi/external-snapshotter/client/v4 v4.2.0
	github.com/onsi/ginkgo/v2 v2.2.0
	github.com/onsi/gomega v1.20.2
	github.com/stretchr/testify v1.8.0
	golang.org/x/sys v0.0.0-20220928140112-f11e5e49a4ec
	google.golang.org/grpc v1.49.0
	k8s.io/api v0.25.2
	k8s.io/apimachinery v0.25.2
	k8s.io/client-go v1.22.11
	k8s.io/component-base v0.25.2
	k8s.io/klog/v2 v2.80.1
	k8s.io/kubernetes v1.25.2
	k8s.io/mount-utils v0.25.2
	k8s.io/pod-security-admission v0.25.2
	k8s.io/utils v0.0.0-20220922133306-665eaaec4324
)

require (
	github.com/Microsoft/go-winio v0.5.2 // indirect
	github.com/PuerkitoBio/purell v1.1.1 // indirect
	github.com/PuerkitoBio/urlesc v0.0.0-20170810143723-de5bf2ad4578 // indirect
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/blang/semver/v4 v4.0.0 // indirect
	github.com/cespare/xxhash/v2 v2.1.2 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/docker/distribution v2.8.1+incompatible // indirect
	github.com/elazarl/goproxy v0.0.0-20220901064549-fbd10ff4f5a1 // indirect
	github.com/emicklei/go-restful/v3 v3.8.0 // indirect
	github.com/evanphx/json-patch v4.12.0+incompatible // indirect
	github.com/felixge/httpsnoop v1.0.3 // indirect
	github.com/fsnotify/fsnotify v1.5.4 // indirect
	github.com/go-logr/logr v1.2.3 // indirect
	github.com/go-openapi/jsonpointer v0.19.5 // indirect
	github.com/go-openapi/jsonreference v0.19.5 // indirect
	github.com/go-openapi/swag v0.19.14 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/golang/groupcache v0.0.0-20210331224755-41bb18bfe9da // indirect
	github.com/google/gnostic v0.5.7-v3refs // indirect
	github.com/google/gofuzz v1.2.0 // indirect
	github.com/google/uuid v1.3.0 // indirect
	github.com/grpc-ecosystem/grpc-gateway v1.16.0 // indirect
	github.com/imdario/mergo v0.3.13 // indirect
	github.com/inconshreveable/mousetrap v1.0.1 // indirect
	github.com/jmespath/go-jmespath v0.4.0 // indirect
	github.com/josharian/intern v1.0.0 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/mailru/easyjson v0.7.6 // indirect
	github.com/matttproud/golang_protobuf_extensions v1.0.2-0.20181231171920-c182affec369 // indirect
	github.com/moby/spdystream v0.2.0 // indirect
	github.com/moby/sys/mountinfo v0.6.0 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.2 // indirect
	github.com/munnerz/goautoneg v0.0.0-20191010083416-a7dc8b61c822 // indirect
	github.com/nxadm/tail v1.4.8 // indirect
	github.com/onsi/ginkgo v1.16.5 // indirect
	github.com/opencontainers/go-digest v1.0.0 // indirect
	github.com/opencontainers/selinux v1.10.1 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/prometheus/client_golang v1.13.0 // indirect
	github.com/prometheus/client_model v0.2.0 // indirect
	github.com/prometheus/common v0.37.0 // indirect
	github.com/prometheus/procfs v0.8.0 // indirect
	github.com/spf13/cobra v1.5.0 // indirect
	github.com/spf13/pflag v1.0.5 // indirect
	go.opentelemetry.io/contrib v1.10.0 // indirect
	go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp v0.20.0 // indirect
	go.opentelemetry.io/otel v0.20.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp v0.20.0 // indirect
	go.opentelemetry.io/otel/metric v0.20.0 // indirect
	go.opentelemetry.io/otel/sdk v0.20.0 // indirect
	go.opentelemetry.io/otel/sdk/export/metric v0.20.0 // indirect
	go.opentelemetry.io/otel/sdk/metric v0.20.0 // indirect
	go.opentelemetry.io/otel/trace v0.20.0 // indirect
	go.opentelemetry.io/proto/otlp v0.7.0 // indirect
	golang.org/x/crypto v0.0.0-20220919173607-35f4265a4bc0 // indirect
	golang.org/x/net v0.0.0-20220919171627-f8f703f97925 // indirect
	golang.org/x/oauth2 v0.0.0-20220909003341-f21342109be1 // indirect
	golang.org/x/term v0.0.0-20220919170432-7a66f970e087 // indirect
	golang.org/x/text v0.3.7 // indirect
	golang.org/x/time v0.0.0-20220722155302-e5dcc9cfc0b9 // indirect
	google.golang.org/appengine v1.6.7 // indirect
	google.golang.org/genproto v0.0.0-20220919141832-68c03719ef51 // indirect
	google.golang.org/protobuf v1.28.1 // indirect
	gopkg.in/inf.v0 v0.9.1 // indirect
	gopkg.in/tomb.v1 v1.0.0-20141024135613-dd632973f1e7 // indirect
	gopkg.in/yaml.v2 v2.4.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
	k8s.io/apiextensions-apiserver v0.0.0 // indirect
	k8s.io/apiserver v0.25.2 // indirect
	k8s.io/cloud-provider v0.25.2 // indirect
	k8s.io/component-helpers v0.25.2 // indirect
	k8s.io/kube-openapi v0.0.0-20220803162953-67bda5d908f1 // indirect
	k8s.io/kubectl v0.0.0 // indirect
	k8s.io/kubelet v0.0.0 // indirect
	sigs.k8s.io/apiserver-network-proxy/konnectivity-client v0.0.32 // indirect
	sigs.k8s.io/json v0.0.0-20220713155537-f223a00ba0e2 // indirect
	sigs.k8s.io/structured-merge-diff/v4 v4.2.3 // indirect
	sigs.k8s.io/yaml v1.2.0 // indirect
)

replace (
	k8s.io/api => k8s.io/api v0.25.2
	k8s.io/apiextensions-apiserver => k8s.io/apiextensions-apiserver v0.25.2
	k8s.io/apimachinery => k8s.io/apimachinery v0.25.2
	k8s.io/apiserver => k8s.io/apiserver v0.25.2
	k8s.io/cli-runtime => k8s.io/cli-runtime v0.25.2
	k8s.io/client-go => k8s.io/client-go v0.25.2
	k8s.io/cloud-provider => k8s.io/cloud-provider v0.25.2
	k8s.io/cluster-bootstrap => k8s.io/cluster-bootstrap v0.25.2
	k8s.io/code-generator => k8s.io/code-generator v0.25.2
	k8s.io/component-base => k8s.io/component-base v0.25.2
	k8s.io/component-helpers => k8s.io/component-helpers v0.25.2
	k8s.io/controller-manager => k8s.io/controller-manager v0.25.2
	k8s.io/cri-api => k8s.io/cri-api v0.25.2
	k8s.io/csi-translation-lib => k8s.io/csi-translation-lib v0.25.2
	k8s.io/kube-aggregator => k8s.io/kube-aggregator v0.25.2
	k8s.io/kube-controller-manager => k8s.io/kube-controller-manager v0.25.2
	k8s.io/kube-proxy => k8s.io/kube-proxy v0.25.2
	k8s.io/kube-scheduler => k8s.io/kube-scheduler v0.25.2
	k8s.io/kubectl => k8s.io/kubectl v0.25.2
	k8s.io/kubelet => k8s.io/kubelet v0.25.2
	k8s.io/legacy-cloud-providers => k8s.io/legacy-cloud-providers v0.25.2
	k8s.io/metrics => k8s.io/metrics v0.25.2
	k8s.io/mount-utils => k8s.io/mount-utils v0.25.2
	k8s.io/node-api => k8s.io/node-api v0.25.2
	k8s.io/pod-security-admission => k8s.io/pod-security-admission v0.25.2
	k8s.io/sample-apiserver => k8s.io/sample-apiserver v0.25.2
	k8s.io/sample-cli-plugin => k8s.io/sample-cli-plugin v0.25.2
	k8s.io/sample-controller => k8s.io/sample-controller v0.25.2
	vbom.ml/util => github.com/fvbommel/util v0.0.2 // Mitigate https://github.com/fvbommel/util/issues/6
)

go 1.19
