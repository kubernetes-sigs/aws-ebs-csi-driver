# Helm chart

## 2.53.0

- Add dnsConfig Helm parameter for node pods. ([#2778](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/2778), [@torredil](https://github.com/torredil))
- Check for specific ServiceMonitor CRD availability instead of generic `monitoring.coreos.com/v1` API group when creating service monitor object for metrics. ([#2779](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/2779), [@torredil](https://github.com/torredil))
- Bump driver version to `v1.53.0`.

## 2.52.1

- Bump driver version to `v1.52.1`.
- Bump sidecars to latest.

## 2.52.0

### Feature

- Bump driver version to `v1.52.0`.
- Add Helm parameter `node.serviceAccount.disableMutation` to disable mutating RBAC permissions to the `ebs-csi-node` service account. When enabled, driver features such as taint removal may not function. ([#2723](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/2723), [@ConnorJC3](https://github.com/ConnorJC3))
- Add ALPHA metadata-labeler sidecar and metadata source ([#2591](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/2591), [@sylviah23](https://github.com/sylviah23))

## 2.51.3

- Bump driver version to `v1.51.2`.
- Bump sidecars to latest.

## 2.51.1

- Bump driver version to `v1.51.1`.

## 2.51.0

- Bump driver version to `v1.51.0`.

### Feature

- Add Helm parameters to customize PDB `maxUnavailable` and `minAvailable` ([#2703](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/2703), [@ConnorJC3](https://github.com/ConnorJC3))

## 2.50.4

- Bump driver version to `v1.50.3`.
- Bump sidecars to latest.

## 2.50.2

- Bump driver version to `v1.50.2`.

## 2.50.1

- Bump driver version to `v1.50.1`.

## 2.50.0

### Feature

- Bump driver version to `v1.50.0`.

## 2.49.3

- Bump driver version to `v1.49.2`.
- Bump sidecars to latest.

## 2.49.2

- Bump driver version to `v1.49.1`

## 2.49.1

### Feature

- Add `terminationMessagePolicy: FallbackToLogsOnError` to all containers to use log messages as termination message ([#2672](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/2672), [@ConnorJC3](https://github.com/ConnorJC3))

## 2.49.0

### Feature

- Add `debugLogs` Helm parameter to turn on maximum verbosity logging ([#2624](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/2624), [@AndrewSirenko](https://github.com/AndrewSirenko))
- Add `containerPort` declarations for containers in Helm chart to support metrics discovery by monitoring systems ([#2654](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/2654), [@torredil](https://github.com/torredil))

## 2.48.0

### Feature

- Bump driver version to `v1.48.0`
- Add support for custom relabelings in ServiceMonitor via `controller.serviceMonitor.extraRelabelings` configuration option ([#2594](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/2594), [@bartier](https://github.com/bartier))
- Align node and controller metrics to consistent experience supporting `prometheus.io` annotations and `ServiceMonitor` objects for both; Enable sidecar metrics when controller metrics are enabled ([#2558](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/2558), [@ConnorJC3](https://github.com/ConnorJC3))

## v2.47.1

- Bump Driver version to `v1.47.1`
- Bump csi-sidecars to new eksbuild version

## v2.47.0

### Feature

- Bump driver version to `v1.47.0`
- Add `ebs-csi-node` readiness probe so that pod is not marked ready until metadata source acquired and starts serving CSI RPCs ([#2579](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/2579), [@AndrewSirenko](https://github.com/AndrewSirenko))

### Bug or Regression

- Allow `null` to be set for `nodeAllocatableUpdatePeriodSeconds.type` in Helm schema ([#2578](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/2578), [@torredil](https://github.com/torredil))

## v2.46.0
- Bump driver version to `v1.46.0`
- Added new Helm parameter: nodeAllocatableUpdatePeriodSeconds. This parameter updates the node's max attachable volume count by directing Kubelet to periodically call NodeGetInfo at the configured interval. Kubernetes enforces a minimum update interval of 10 seconds. This parameter is supported in Kubernetes 1.33+ and requires the MutableCSINodeAllocatableCount feature gate to be enabled in kubelet and kube-apiserver. ([#2538](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/2538), [@torredil](https://github.com/torredil))

## v2.45.1
- Bump csi-sidecars to new eksbuild versions to fix livenessprobe

## v2.45.0

### Feature

- Bump driver version to `v1.45.0`.
- Switch sidecar image repositories from deprecated `public.ecr.aws/eks-distro/kubernetes-csi` to `public.ecr.aws/csi-components/` ([#2518](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/2518), [@AndrewSirenko](https://github.com/AndrewSirenko))

## v2.44.0

### Feature

- Bump driver version to `v1.44.0`.

## v2.43.0

### Feature

- Bump driver version to `v1.43.0`.

## v2.42.0

### Feature

- Set internal traffic policy to local for node metric service ([#2432](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/2432), [@ElijahQuinones](https://github.com/ElijahQuinones))

## v2.41.0

### Feature

- Add `enabled` flag to schema for use in sub-charting ([#2361](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/2361), [@ConnorJC3](https://github.com/ConnorJC3))
- Add Prometheus Annotations to the Node Service ([#2363](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/2363), [@mdzraf](https://github.com/mdzraf))

### Bug or regression

- Prevent nil pointer deref in Helm chart when `node.enableWindows` and `node.otelTracing` are both set ([#2357](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/2357), [@ConnorJC3](https://github.com/ConnorJC3))

## v2.40.3

### Feature

- Upgrade csi-attacher to v4.8.1, csi-snapshotter to v8.2.1, csi-resizer to v1.13.2

### Bug or regression

- Fix incorrect schema entry for controller.podDisruptionBudget.unhealthyPodEvictionPolicy ([#2389](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/2389),[@jamesalford](https://github.com/jamesalford))

## v2.40.2

### Bug or Regression

- Add enabled flag to schema for sub-charting ([#2359](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/2359), [@ConnorJC3](https://github.com/ConnorJC3))

## v2.40.1

### Bug or Regression

- Prevent null deref when enableWindows and otelTracing enabled on node ([#2357](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/2357), [@ConnorJC3](https://github.com/ConnorJC3)) 
- Fix incorrect properties validation in Helm schema ([#2356](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/2356), [@ConnorJC3](https://github.com/ConnorJC3))

## v2.40.0

#### Default for enable windows changed

The default value for enableWindows has been changed from false to true. This change makes it so the node damemonset will be scheduled on windows nodes by default. If you wish to not have the node daemonset scheduled on your windows nodes you will need to change enableWindows to false.

### Feature

- Add values.schema.json to validate changes in values.yaml. ([#2286](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/2286), [@ElijahQuinones](https://github.com/ElijahQuinones))

### Bug or Regression

- Fix helm regression with values.schema.yaml. ([#2322](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/2322), [@ElijahQuinones](https://github.com/ElijahQuinones))
- `global` has been added to the values schema, allowing aws-ebs-csi-driver to be used in a Helm sub chart ([#2321](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/2321), [@kejne](https://github.com/kejne))
- Reconcile some differences between helm chart and values.schema.json ([#2335](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/2335), [@ElijahQuinones](https://github.com/ElijahQuinones))
- Fix helm regression with a1CompatibilityDaemonSet=true ([#2316](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/2316), [@AndrewSirenko](https://github.com/AndrewSirenko))

## v2.39.3

### Urgent Upgrade Notes

Please upgrade from v2.39.2 directly to v2.39.3 to avoid upgrade failures if you are using this chart as a subchart.

### Bug or Regression
- Fix sub-charting by removing values schema ([#2322](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/2322), [@ElijahQuinones]((https://github.com/ElijahQuinones)

## v2.39.2

### Urgent Upgrade Notes

Please upgrade from v2.38.1 directly to v2.39.2 to avoid upgrade failures if you are relying on `a1CompatibilityDaemonSet`. 

### Bug or Regression
- Fix helm regression when `a1CompatibilityDaemonSet=true` ([#2316](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/2316), [@AndrewSirenko](https://github.com/AndrewSirenko))

## v2.39.1

### Bug or Regression
- Fix `node.selinux` to properly set SELinux-specific mounts as ReadOnly ([#2311](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/2311), [@AndrewSirenko](https://github.com/AndrewSirenko))

## v2.39.0

### Feature

- Add Helm parameter `node.selinux` to enable SELinux-specific mounts on the node DaemonSet ([#2253](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/2253), [@ConnorJC3](https://github.com/ConnorJC3))
- Add Helm FIPS parameter ([#2244](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/2244), [@ConnorJC3](https://github.com/ConnorJC3))

## v2.38.1

### Feature

- Render templated controller service account parameters ([#2243](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/2243), [@ElijahQuinones](https://github.com/ElijahQuinones))

### Bug or Regression

- Fix rendering failrue when `node.enableMetrics` is set to `true` ([#2250](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/2250), [@mindw](https://github.com/mindw))
- Remove duplicate 'enableMetrics' key ([#2256](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/2256), [@sule26](https://github.com/sule26))

## v2.37.0
* Bump driver version to `v1.37.0`
* Add init containers to node daemonset ([#2215](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/2215), [@clbx](https://github.com/clbx))
* Fix fetching test package version for kubetest in helm-tester ([#2203](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/2203), [@torredil](https://github.com/torredil))

## v2.36.0
* Bump driver version to `v1.36.0`
* Add recommended autoscalar Tolerations to driver DaemonSet ([#2165](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/2165), [@AndrewSirenko](https://github.com/AndrewSirenko))
* Add support for unhealthyPodEvictionPolicy on PodDisruptionBudget ([#2159](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/2159), [@peterabarr](https://github.com/peterabarr))

## v2.35.1
* Fix an issue causing the `csi-attacher` container to get stuck in `CrashLoopBackoff` on clusters with VAC enabled. Users with a VAC-enabled cluster are strongly encouraged to skip `v2.35.0` and/or upgrade directly to `v2.35.1` or later.

## v2.35.0
* Bump driver version to `v1.35.0`
* Add reservedVolumeAttachments to windows nodes ([#2134](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/2134),[@AndrewSirenko](https://github.com/AndrewSirenko))
* Add legacy-xfs driver option for clusters that mount XFS volumes to nodes with Linux kernel <= 5.4. Warning: This is a temporary workaround for customers unable to immediately upgrade their nodes. It will be removed in a future release. See [the options documentation](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/blob/release-1.35/docs/options.md) for more details.([#2121](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/2121),[@AndrewSirenko](https://github.com/AndrewSirenko))
* Add back "Auto-enable VAC on clusters with beta API version" ([#2141](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/2141), [@ConnorJC3](https://github.com/ConnorJC3))

## v2.34.0
* Bump driver version to `v1.34.0`
* Add toggle for PodDisruptionBudget in chart ([#2109](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/2109), [@AndrewSirenko](https://github.com/AndrewSirenko))
* Add nodeComponentOnly parameter to helm chart ([#2106](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/2106), [@AndrewSirenko](https://github.com/AndrewSirenko))
* fix: sidecars.snapshotter.logLevel not being respect ([#2102](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/2102), [@zyue110026](https://github.com/zyue110026))

## v2.33.0
* Bump driver version to `v1.33.0`
* Bump CSI sidecar container versions
* Add fix for enableLinux node parameter ([#2078](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/2078), [@ElijahQuinones](https://github.com/ElijahQuinones))
* Fix dnsConfig indentation in controller template file ([#2084](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/2084), [@cHiv0rz](https://github.com/cHiv0rz))

## v2.32.0
* Bump driver version to `v1.32.0`
* Bump CSI sidecar container versions
* Add `patch` permission to `PV` to `external-provisioner` role (required by v5 and later)
* Add terminationGracePeriodSeconds as a helm parameter ([#2060](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/2060), [@ElijahQuinones](https://github.com/ElijahQuinones))
* Use release namespace in ClusterRoleBinding subject namespace ([#2059](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/2059), [@etutuit](https://github.com/etutuit))
* Add parameter to override node DaemonSet namespace ([#2052](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/2052), [@RuStyC0der](https://github.com/RuStyC0der))
* Set RuntimeDefault as default seccompProfile in securityContext ([#2061](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/2061), [@torredil](https://github.com/torredil))
* Increase default provisioner, resizer, snapshotter `retry-interval-max` ([#2057](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/2057), [@AndrewSirenko](https://github.com/AndrewSirenko))

## v2.31.0
* Bump driver version to `v1.31.0`
* Expose dnsConfig in Helm Chart for Custom DNS Configuration ([#2034](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/2045), [@omerap12](https://github.com/omerap12))
* Make scrape interval configurable ([#2035](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/2035), [@omerap12](https://github.com/omerap12))
* Add defaultStorageClass parameter ([#2039](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/2039), [@torredil](https://github.com/torredil))
* Upgrade sidecar containers ([#2041](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/2041), [@torredil](https://github.com/torredil))

## v2.30.0
* Bump driver version to `v1.30.0`
* Update voluemessnapshotcontents/status RBAC ([#1991](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/1991), [@AndrewSirenko](https://github.com/AndrewSirenko))
* Upgrade dependencies ([#2016](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/2016), [@torredil](https://github.com/torredil))

## v2.29.1
* Bump driver version to `v1.29.1`
* Remove `--reuse-values` deprecation warning

## v2.29.0
### Urgent Upgrade Notes
*(No, really, you MUST read this before you upgrade)*

The EBS CSI Driver Helm chart no longer supports upgrading with `--reuse-values`. This chart will not test for `--reuse-values` compatibility and upgrading with `--reuse-values` will likely fail. Users of `--reuse-values` are strongly encouraged to migrate to `--reset-then-reuse-values`.

For more information see [the deprecation announcement](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/issues/1864).

### Other Changes
* Bump driver version to `v1.29.0` and sidecars to latest versions
* Add helm-tester enabled flag ([#1954](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/1954), [@nunodomingues-td](https://github.com/nunodomingues-td))

## v2.28.1
* Add `reservedVolumeAttachments` that overrides heuristic-determined reserved attachments via  `--reserved-volume-attachments` CLI option from [PR #1919](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/1919) through Helm ([#1939](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/1939), [@AndrewSirenko](https://github.com/AndrewSirenko)) 
* Add `additionalArgs` parameter to node daemonSet ([#1939](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/1939), [@AndrewSirenko](https://github.com/AndrewSirenko))

## v2.28.0
### Urgent Upgrade Notes
*(No, really, you MUST read this before you upgrade)*

This is the last minor version of the EBS CSI Driver Helm chart to support upgrading with `--reuse-values`. Future versions of the chart (starting with `v2.29.0`) will not test for `--reuse-values` compatibility and upgrading with `--reuse-values` will likely fail. Users of `--reuse-values` are strongly encouraged to migrate to `--reset-then-reuse-values`.

For more information see [the deprecation announcement](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/issues/1864).

### Other Changes
* Bump driver version to `v1.28.0` and sidecars to latest versions
* Add labels to leases role used by EBS CSI controller ([#1914](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/1914), [@cHiv0rz](https://github.com/cHiv0rz))
* Enforce `linux` and `amd64` node affinity for helm tester pod ([#1922](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/1922), [@AndrewSirenko](https://github.com/AndrewSirenko))
* Add configuration for `DaemonSet` annotations ([#1923](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/1923), [@AndrewSirenko](https://github.com/AndrewSirenko))
* Incorporate KubeLinter recommended best practices for chart tester pod ([#1924](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/1924), [@torredil](https://github.com/torredil))
* Add configuration for chart tester pod image ([#1928](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/1928), [@AndrewSirenko](https://github.com/AndrewSirenko))

## v2.27.0
* Bump driver version to `v1.27.0`
* Add parameters for tuning revisionHistoryLimit and emptyDir volumes ([#1840](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/1840), [@bodgit](https://github.com/bodgit))

## v2.26.1
* Bump driver version to `v1.26.1`
* Bump sidecar container versions to fix [restart bug in external attacher, provisioner, resizer, snapshotter, and node-driver-registrar](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/issues/1875) ([#1886](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/1886), [@AndrewSirenko](https://github.com/AndrewSirenko))

## v2.26.0
* Bump driver version to `v1.26.0`
* Bump sidecar container versions ([#1867](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/1867), [@AndrewSirenko](https://github.com/AndrewSirenko)) 
* Add warning about --reuse-values deprecation to NOTES.txt ([#1865](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/1865), [@ConnorJC3](https://github.com/ConnorJC3))

## v2.25.0
* Bump driver version to `v1.25.0`
* Update default sidecar timeout values ([#1824](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/1824), [@torredil](https://github.com/torredil))
* Increase default QPS and worker threads of sidecars ([#1834](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/1834), [@ConnorJC3](https://github.com/ConnorJC3))
* Node-driver-registrar sidecar fixes ([#1815](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/1815), [@jukie](https://github.com/jukie))
* Suggest eks.amazonaws.com/role-arn in values.yaml if EKS IAM for SA is used ([#1804](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/1804), [@tporeba](https://github.com/tporeba))

## v2.24.1
* Bump driver version to `v1.24.1`
* Upgrade sidecar images

## v2.24.0
* Bump driver version to `v1.24.0`
* Add additionalClusterRoleRules to sidecar chart templates. ([#1757](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/1757), [@AndrewSirenko](https://github.com/AndrewSirenko))
* Allow passing template value for clusterName ([#1753](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/1753), [@monicastanciu](https://github.com/monicastanciu))
* Make hostNetwork configurable for daemonset ([#1716](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/1716), [@bseenu](https://github.com/bseenu))
* Add labels to volumesnapshotclass ([#1754](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/1754), [@fad3t](https://github.com/fad3t))
* Update default API version for PodDisruptionBudget ([#1751](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/1751), [@AndrewSirenko](https://github.com/AndrewSirenko))

## v2.23.2
* Bump driver version to `v1.23.2`
* Upgrade sidecar images

## v2.23.1
* Bump driver version to `v1.23.1`

## v2.23.0
* Add `node.enableLinux` parameter ([#1732](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/1732), [@monicastanciu](https://github.com/monicastanciu))
* Additional Node DaemonSets bug fixes ([#1739](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/1739), [@monicastanciu](https://github.com/monicastanciu))
* Additional DaemonSets feature ([#1722](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/1722), [@ConnorJC3](https://github.com/ConnorJC3))
* Add doc of chart value additionalArgs ([#1697](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/1697), [@zitudu](https://github.com/zitudu))

## v2.22.1
* Bump driver version to `v1.22.1`

## v2.22.0
* Default PodDisruptionBudget to policy/v1 ([#1707](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/1707), [@iNoahNothing](https://github.com/iNoahNothing))

## v2.21.0
* Bump driver version to `v1.21.0`
* Enable additional volume mounts on node pods ([#1670](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/1670), [@AndrewSirenko](https://github.com/AndrewSirenko))
* Enable customization of aws-secret name and keys in Helm Chart ([#1668](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/1668), [@AndrewSirenko](https://github.com/AndrewSirenko))
* The sidecars have been updated. The new versions are:
    - csi-snapshotter: `v6.2.2`

## v2.20.0
* Bump driver version to `v1.20.0`
* Enable leader election in csi-resizer sidecar ([#1606](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/1606), [@rdpsin](https://github.com/rdpsin))
* Namespace-scoped leases permissions ([#1614](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/1614), [@torredil](https://github.com/torredil))
* Add additionalArgs parameter for sidecars ([#1627](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/1627), [@ConnorJC3](https://github.com/ConnorJC3))
* Avoid generating manifests with empty envFrom fields ([#1630](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/1630), [@mvgmb](https://github.com/mvgmb))
* Allow to set automountServiceAccountToken in ServiceAccount ([#1619](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/1619), [@kahirokunn](https://github.com/kahirokunn))

## v2.19.0
* Bump driver version to `v1.19.0`
* The sidecars have been updated. The new versions are:
    - csi-provisioner: `v3.5.0`
    - csi-attacher: `v4.3.0`
    - livenessprobe: `v2.10.0`
    - csi-resizer: `v1.8.0`
    - node-driver-registrar: `v2.8.0`
* Remove CPU limits ([#1596](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/1596), [@torredil](https://github.com/torredil))

## v2.18.0
### Urgent Upgrade Notes
*(No, really, you MUST read this before you upgrade)*

The Helm chart now defaults to using specific releases of the EKS-D sidecars, rather than the `-latest` versions. This is done so the chart will specify an exact container image, as well as for consistency with the EKS Addons version of the driver.

The new sidecar tags are:
* csi-provisioner: `v3.4.1-eks-1-26-7`
* csi-attacher: `v4.2.0-eks-1-26-7`
* csi-snapshotter: `v6.2.1-eks-1-26-7`
* livenessprobe: `v2.9.0-eks-1-26-7`
* csi-resizer: `v1.7.0-eks-1-26-7`
* node-driver-registrar: `v2.7.0-eks-1-26-7`

### Improvements
* Bump driver version to `v1.18.0`
* Increase speed and reliability of `helm test` ([#1533](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/1533), [@torredil](https://github.com/torredil))
* Support `VolumeSnapshotClass` in helm chart ([#1540](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/1540), [@hanyuel](https://github.com/hanyuel))

## v2.17.2
* Bump driver version to `v1.17.0`
* Bump `external-resizer` version to `v4.2.0`
* All other sidecars have been updated to the latest rebuild (without an associated version change)

## v2.17.1
* Bump driver version to `v1.16.1`

## v2.17.0
* Bump driver version to `v1.16.0`
* Add support for JSON logging ([#1467](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/1467), [@torredil](https://github.com/torredil))
    * `--logging-format` flag has been added to set the log format. Valid values are `text` and `json`. The default value is `text`.
    * `--logtostderr` is deprecated.
    * Long arguments prefixed with `-` are no longer supported, and must be prefixed with `--`. For example, `--volume-attach-limit` instead of `-volume-attach-limit`.
* The sidecars have been updated. The new versions are:
    - csi-provisioner: `v3.4.0`
    - csi-attacher: `v4.1.0`
    - csi-snapshotter: `v6.2.1`
    - livenessprobe: `v2.9.0`
    - csi-resizer: `v1.7.0`
    - node-driver-registrar: `v2.7.0`


## v2.16.0
* Bump driver version to `v1.15.0`
* Change default sidecars to EKS-D ([#1475](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/1475), [@ConnorJC3](https://github.com/ConnorJC3), [@torredil](https://github.com/torredil))
* The sidecars have been updated. The new versions are:
    - csi-provisioner: `v3.3.0`
    - csi-attacher: `v4.0.0`
    - csi-snapshotter: `v6.1.0`
    - livenessprobe: `v2.8.0`
    - csi-resizer: `v1.6.0`
    - node-driver-registrar: `v2.6.2`

## v2.15.1
* Bugfix: Prevent deployment of testing resources during normal installation by adding `helm.sh/hook: test` annotation.

## v2.15.0
* Set sensible default resource requests/limits
* Add sensible default update strategy
* Add podAntiAffinity so controller pods prefer scheduling on separate nodes if possible
* Add container registry parameter

## v2.14.2
* Bump driver version to `v1.14.1`

## v2.14.1
* Add `controller.sdkDebugLog` parameter

## v2.14.0
* Bump driver version to `v1.14.0`

## v2.13.0
* Bump app/driver to version `v1.13.0`
* Expose volumes and volumeMounts for the ebs-csi-controller deployment ([#1400](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/1436), [@cnmcavoy](https://github.com/cnmcavoy))
* refactor: Move the default controller tolerations in the helm chart values ([#1427](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/1427), [@cnmcavoy](https://github.com/Linutux42))
* Add serviceMonitor.labels parameter ([#1419](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/1419), [@torredil](https://github.com/torredil))
* Add parameter to force enable snapshotter sidecar ([#1418](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/1418), [@ConnorJC3](https://github.com/ConnorJC3))

## v2.12.1
* Bump app/driver to version `v1.12.1`

## v2.12.0
* Bump app/driver to version `v1.12.0`
* Move default toleration to values.yaml so it can be overriden if desired by users ([#1400](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/1400), [@cnmcavoy](https://github.com/cnmcavoy))
* Add enableMetrics configuration ([#1380](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/1380), [@torredil](https://github.com/torredil))
* add initContainer to the controller's template ([#1379](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/1379), [@InsomniaCoder](https://github.com/InsomniaCoder))
* Add controller nodeAffinity to prefer EC2 over Fargate ([#1360](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/1360), [@torredil](https://github.com/torredil))

## v2.11.1
* Add `useOldCSIDriver` parameter to use old `CSIDriver` object.

## v2.11.0

**Important Notice:** This version updates the `CSIDriver` object in order to fix [a bug with static volumes and the `fsGroup` parameter](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/issues/1365). This upgrade will fail on existing clusters because the associated field in `CSIDriver` is immutable.

Users upgrading to this version should pre-delete the existing `CSIDriver` object (example: `kubectl delete csidriver ebs.csi.aws.com`). This will not affect any existing volumes, but will cause the EBS CSI Driver to be unavailable to handle future requests, and should be immediately followed by an upgrade. For users that cannot delete the `CSIDriver` object, v2.11.1 implements a new parameter `useOldCSIDriver` that will use the previous `CSIDriver`.

* Bump app/driver to version `v1.11.3`
* Add support for leader election tuning for `csi-provisioner` and `csi-attacher` ([#1371](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/1371), [@moogzy](https://github.com/moogzy))
* Change `fsGroupPolicy` to `File` ([#1377](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/1377), [@ConnorJC3](https://github.com/ConnorJC3))
* Allow all taint for `csi-node` by default ([#1381](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/1381), [@gtxu](https://github.com/gtxu))

## v2.10.1
* Bump app/driver to version `v1.11.2`

## v2.10.0
* Implement securityContext for containers
* Add securityContext for node pod
* Utilize more secure defaults for securityContext

## v2.9.0
* Bump app/driver to version `v1.10.0`
* Feature: Reference `configMaps` across multiple resources using `envFrom` ([#1312](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/1312), [@jebbens](https://github.com/jebbens))

## v2.8.1
* Bump app/driver to version `v1.9.0`
* Update livenessprobe to version `v2.6.0`

## v2.8.0
* Feature: Support custom affinity definition on node daemon set ([#1277](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/1277), [@vauchok](https://github.com/vauchok))

## v2.7.1
* Bump app/driver to version `v1.8.0`

## v2.7.0
* Support optional ec2 endpoint configuration.
* Fix node driver registrar socket path.
* Fix hardcoded kubelet path.

## v2.6.11
* Bump app/driver to version `v1.7.0`
* Set handle-volume-inuse-error to `false`

## v2.6.10

* Add quotes around the `extra-tags` argument in order to prevent special characters such as `":"` from breaking the manifest YAML after template rendering.

## v2.6.9

* Update csi-snapshotter to version `v6.0.1`
* Update external-attacher to version `v3.4.0`
* Update external-resizer to version `v1.4.0`
* Update external-provisioner to version `v3.1.0`
* Update node-driver-registrar to version `v2.5.1`
* Update livenessprobe to version `v2.5.0`

## v2.6.8

* Bump app/driver to version `v1.6.2`
* Bump sidecar version for nodeDriverRegistrar, provisioner to be consistent with EKS CSI Driver Add-on

## v2.6.7

* Bump app/driver to version `v1.6.1`

## v2.6.6

* Bump app/driver to version `v1.6.0`

## v2.6.5

* Bump app/driver to version `v1.5.3`

## v2.6.4

* Remove exposure all secrets to external-snapshotter-role

## v2.6.3

* Bump app/driver to version `v1.5.1`

## v2.6.2

* Update csi-resizer version to v1.1.0

## v2.6.1

* Add securityContext support for controller Deployment

## v2.5.0

* Bump app/driver version to `v1.5.0`

## v2.4.1

* Replace deprecated arg `--extra-volume-tags` by `--extra-tags`

## v2.4.0

* Bump app/driver version to `v1.4.0`

## v2.3.1

* Bump app/driver version to `v1.3.1`

## v2.3.0

* Support overriding controller `--default-fstype` flag via values

## v2.2.1

* Bump app/driver version to `v1.3.0`

## v2.2.0

* Support setting imagePullPolicy for all containers

## v2.1.1

* Bump app/driver version to `v1.2.1`

## v2.1.0

* Custom `controller.updateStrategy` to set controller deployment strategy.

## v2.0.4

* Use chart app version as default image tag
* Add updateStrategy to daemonsets

## v2.0.3

* Bump app/driver version to `v1.2.0`

## v2.0.2

* Bump app/driver version to `v1.1.3`

## v2.0.1

* Only create Windows daemonset if enableWindows is true
* Update Windows daemonset to align better to the Linux one

## v2.0.0

* Remove support for Helm 2
* Remove deprecated values
* No longer install snapshot controller or its CRDs
* Reorganize additional values

[Upgrade instructions](/docs/README.md#upgrading-from-version-1x-to-2x-of-the-helm-chart)

## v1.2.4

* Bump app/driver version to `v1.1.1`
* Install VolumeSnapshotClass, VolumeSnapshotContent, VolumeSnapshot CRDs if enableVolumeSnapshot is true
* Only run csi-snapshotter sidecar if enableVolumeSnapshot is true or if CRDs are already installed
