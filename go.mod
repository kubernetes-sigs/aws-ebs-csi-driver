module github.com/kubernetes-sigs/aws-ebs-csi-driver

require (
	github.com/aws/aws-sdk-go-v2 v1.26.1
	github.com/aws/aws-sdk-go-v2/config v1.27.11
	github.com/aws/aws-sdk-go-v2/feature/ec2/imds v1.16.1
	github.com/aws/aws-sdk-go-v2/service/ec2 v1.157.0
	github.com/aws/smithy-go v1.20.2
	github.com/awslabs/volume-modifier-for-k8s v0.2.1
	github.com/container-storage-interface/spec v1.9.0
	github.com/golang/mock v1.6.0
	github.com/google/go-cmp v0.6.0
	github.com/google/uuid v1.6.0
	github.com/kubernetes-csi/csi-proxy/client v1.1.3
	github.com/kubernetes-csi/csi-proxy/v2 v2.0.0-alpha.1
	github.com/kubernetes-csi/external-snapshotter/client/v4 v4.2.0
	github.com/onsi/ginkgo/v2 v2.17.1
	github.com/onsi/gomega v1.33.0
	github.com/spf13/pflag v1.0.5
	github.com/stretchr/testify v1.9.0
	go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc v0.50.0
	go.opentelemetry.io/otel v1.25.0
	go.opentelemetry.io/otel/exporters/otlp/otlptrace v1.25.0
	go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc v1.25.0
	go.opentelemetry.io/otel/sdk v1.25.0
	golang.org/x/sys v0.19.0
	google.golang.org/grpc v1.63.2
	google.golang.org/protobuf v1.33.0
	k8s.io/api v0.30.0
	k8s.io/apimachinery v0.30.0
	k8s.io/client-go v1.5.2
	k8s.io/component-base v0.30.0
	k8s.io/klog/v2 v2.120.1
	k8s.io/kubernetes v1.30.0
	k8s.io/mount-utils v0.30.0
	k8s.io/pod-security-admission v0.30.0
	k8s.io/utils v0.0.0-20240310230437-4693a0247e57
)

require (
	github.com/antlr/antlr4/runtime/Go/antlr/v4 v4.0.0-20230305170008-8188dc5388df // indirect
	github.com/aws/aws-sdk-go-v2/credentials v1.17.11 // indirect
	github.com/aws/aws-sdk-go-v2/internal/configsources v1.3.5 // indirect
	github.com/aws/aws-sdk-go-v2/internal/endpoints/v2 v2.6.5 // indirect
	github.com/aws/aws-sdk-go-v2/internal/ini v1.8.0 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/accept-encoding v1.11.2 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/presigned-url v1.11.7 // indirect
	github.com/aws/aws-sdk-go-v2/service/sso v1.20.5 // indirect
	github.com/aws/aws-sdk-go-v2/service/ssooidc v1.23.4 // indirect
	github.com/aws/aws-sdk-go-v2/service/sts v1.28.6 // indirect
	github.com/distribution/reference v0.5.0 // indirect
	github.com/gorilla/websocket v1.5.1 // indirect
	github.com/mitchellh/hashstructure/v2 v2.0.2 // indirect
	github.com/mxk/go-flowrate v0.0.0-20140419014527-cca7078d478f // indirect
	github.com/robfig/cron/v3 v3.0.1 // indirect
	github.com/samber/lo v1.39.0 // indirect
	k8s.io/component-helpers v0.30.0 // indirect
	knative.dev/pkg v0.0.0-20230712131115-7051d301e7f4 // indirect
)

require (
	github.com/Microsoft/go-winio v0.6.1 // indirect
	github.com/NYTimes/gziphandler v1.1.1 // indirect
	github.com/asaskevich/govalidator v0.0.0-20230301143203-a9d515a09cc2 // indirect
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/blang/semver/v4 v4.0.0 // indirect
	github.com/cenkalti/backoff/v4 v4.3.0 // indirect
	github.com/cespare/xxhash/v2 v2.2.0 // indirect
	github.com/coreos/go-semver v0.3.1 // indirect
	github.com/coreos/go-systemd/v22 v22.5.0 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/emicklei/go-restful/v3 v3.12.0 // indirect
	github.com/evanphx/json-patch v4.12.0+incompatible // indirect
	github.com/felixge/httpsnoop v1.0.4 // indirect
	github.com/fsnotify/fsnotify v1.7.0 // indirect
	github.com/go-logr/logr v1.4.1 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/go-logr/zapr v1.3.0 // indirect
	github.com/go-openapi/jsonpointer v0.21.0 // indirect
	github.com/go-openapi/jsonreference v0.21.0 // indirect
	github.com/go-openapi/swag v0.23.0 // indirect
	github.com/go-task/slim-sprig v0.0.0-20230315185526-52ccab3ef572 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/golang/groupcache v0.0.0-20210331224755-41bb18bfe9da // indirect
	github.com/golang/protobuf v1.5.4 // indirect
	github.com/google/cel-go v0.20.1 // indirect
	github.com/google/gnostic-models v0.6.8 // indirect
	github.com/google/gofuzz v1.2.0 // indirect
	github.com/google/pprof v0.0.0-20240319011627-a57c5dfe54fd // indirect
	github.com/grpc-ecosystem/go-grpc-prometheus v1.2.0 // indirect
	github.com/grpc-ecosystem/grpc-gateway/v2 v2.19.1 // indirect
	github.com/imdario/mergo v0.3.16 // indirect
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/jmespath/go-jmespath v0.4.0 // indirect
	github.com/josharian/intern v1.0.0 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/mailru/easyjson v0.7.7 // indirect
	github.com/moby/spdystream v0.2.0 // indirect
	github.com/moby/sys/mountinfo v0.7.1 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.2 // indirect
	github.com/munnerz/goautoneg v0.0.0-20191010083416-a7dc8b61c822 // indirect
	github.com/opencontainers/go-digest v1.0.0 // indirect
	github.com/opencontainers/selinux v1.11.0 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/prometheus/client_golang v1.18.0 // indirect
	github.com/prometheus/client_model v0.6.0 // indirect
	github.com/prometheus/common v0.47.0 // indirect
	github.com/prometheus/procfs v0.12.0 // indirect
	github.com/spf13/cobra v1.8.0 // indirect
	github.com/stoewer/go-strcase v1.3.0 // indirect
	go.etcd.io/etcd/api/v3 v3.5.12 // indirect
	go.etcd.io/etcd/client/pkg/v3 v3.5.12 // indirect
	go.etcd.io/etcd/client/v3 v3.5.12 // indirect
	go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp v0.48.0 // indirect
	go.opentelemetry.io/otel/metric v1.25.0 // indirect
	go.opentelemetry.io/otel/trace v1.25.0 // indirect
	go.opentelemetry.io/proto/otlp v1.1.0 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	go.uber.org/zap v1.27.0 // indirect
	golang.org/x/crypto v0.22.0 // indirect
	golang.org/x/exp v0.0.0-20240318143956-a85f2c67cd81 // indirect
	golang.org/x/mod v0.16.0 // indirect
	golang.org/x/net v0.24.0 // indirect
	golang.org/x/oauth2 v0.18.0 // indirect
	golang.org/x/sync v0.6.0 // indirect
	golang.org/x/term v0.19.0 // indirect
	golang.org/x/text v0.14.0 // indirect
	golang.org/x/time v0.5.0 // indirect
	golang.org/x/tools v0.19.0 // indirect
	google.golang.org/appengine v1.6.8 // indirect
	google.golang.org/genproto v0.0.0-20240318140521-94a12d6c2237 // indirect
	google.golang.org/genproto/googleapis/api v0.0.0-20240318140521-94a12d6c2237 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20240401170217-c3f982113cda // indirect
	gopkg.in/inf.v0 v0.9.1 // indirect
	gopkg.in/natefinch/lumberjack.v2 v2.2.1 // indirect
	gopkg.in/yaml.v2 v2.4.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
	k8s.io/apiextensions-apiserver v0.30.0 // indirect
	k8s.io/apiserver v0.30.0 // indirect
	k8s.io/cloud-provider v0.30.0 // indirect
	k8s.io/controller-manager v0.30.0 // indirect
	k8s.io/kms v0.30.0 // indirect
	k8s.io/kube-openapi v0.0.0-20240228011516-70dd3763d340 // indirect
	k8s.io/kubectl v0.30.0 // indirect
	k8s.io/kubelet v0.30.0 // indirect
	sigs.k8s.io/apiserver-network-proxy/konnectivity-client v0.29.0 // indirect
	sigs.k8s.io/json v0.0.0-20221116044647-bc3834ca7abd // indirect
	sigs.k8s.io/karpenter v0.35.2
	sigs.k8s.io/structured-merge-diff/v4 v4.4.1 // indirect
	sigs.k8s.io/yaml v1.4.0 // indirect
)

replace (
	github.com/google/cel-go => github.com/google/cel-go v0.17.7 // Mitigate https://github.com/kubernetes/apiserver/issues/97
	k8s.io/api => k8s.io/api v0.30.0
	k8s.io/apiextensions-apiserver => k8s.io/apiextensions-apiserver v0.30.0
	k8s.io/apimachinery => k8s.io/apimachinery v0.30.0
	k8s.io/apiserver => k8s.io/apiserver v0.30.0
	k8s.io/cli-runtime => k8s.io/cli-runtime v0.30.0
	k8s.io/client-go => k8s.io/client-go v0.30.0
	k8s.io/cloud-provider => k8s.io/cloud-provider v0.30.0
	k8s.io/cluster-bootstrap => k8s.io/cluster-bootstrap v0.30.0
	k8s.io/code-generator => k8s.io/code-generator v0.30.0
	k8s.io/component-base => k8s.io/component-base v0.30.0
	k8s.io/component-helpers => k8s.io/component-helpers v0.30.0
	k8s.io/controller-manager => k8s.io/controller-manager v0.30.0
	k8s.io/cri-api => k8s.io/cri-api v0.30.0
	k8s.io/csi-translation-lib => k8s.io/csi-translation-lib v0.30.0
	k8s.io/dynamic-resource-allocation => k8s.io/dynamic-resource-allocation v0.30.0
	k8s.io/endpointslice => k8s.io/endpointslice v0.30.0
	k8s.io/kube-aggregator => k8s.io/kube-aggregator v0.30.0
	k8s.io/kube-controller-manager => k8s.io/kube-controller-manager v0.30.0
	k8s.io/kube-proxy => k8s.io/kube-proxy v0.30.0
	k8s.io/kube-scheduler => k8s.io/kube-scheduler v0.30.0
	k8s.io/kubectl => k8s.io/kubectl v0.30.0
	k8s.io/kubelet => k8s.io/kubelet v0.30.0
	k8s.io/legacy-cloud-providers => k8s.io/legacy-cloud-providers v0.30.0
	k8s.io/metrics => k8s.io/metrics v0.30.0
	k8s.io/mount-utils => k8s.io/mount-utils v0.30.0
	k8s.io/node-api => k8s.io/node-api v0.30.0
	k8s.io/pod-security-admission => k8s.io/pod-security-admission v0.30.0
	k8s.io/sample-apiserver => k8s.io/sample-apiserver v0.30.0
	k8s.io/sample-cli-plugin => k8s.io/sample-cli-plugin v0.30.0
	k8s.io/sample-controller => k8s.io/sample-controller v0.30.0
	vbom.ml/util => github.com/fvbommel/util v0.0.2 // Mitigate https://github.com/fvbommel/util/issues/6
)

go 1.22.2
