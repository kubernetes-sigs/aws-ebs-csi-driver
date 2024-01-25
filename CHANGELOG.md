# v1.24.1
### Bug Fixes
* Add compatibility workaround for A1 instance family ([#1811](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/1811), [@ConnorJC3](https://github.com/ConnorJC3))

### Improvements
* Upgrade dependencies (and resolve CVEs found in [#1800](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/issues/1800)) ([#1809](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/1809), [@ConnorJC3](https://github.com/ConnorJC3))

# v1.24.0
### Notable Changes
* Support clustered allocation with ext4 filesystems. This allows developers to enable [torn write prevention](https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/storage-twp.html) on their dynamically provisioned volumes to improve the performance of I/O-intensive relational database workloads. ([#1706](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/1706), [@AndrewSirenko](https://github.com/AndrewSirenko))
* Add volume limits for m7a, c7a, c7i, r7a, r7iz instance families ([#1742](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/1742) & [#1776](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/1776), [@torredil](https://github.com/torredil))

### Bug Fixes
* Fix DeleteDisk error handling in volume creation failure ([#1782](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/1782), [@maaoBit](https://github.com/maaoBit))

### Improvements
* Document topologies in parameters.md ([#1764](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/1764), [@ConnorJC3](https://github.com/ConnorJC3))
* Upgrade dependencies ([#1781](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/1781), [@AndrewSirenko](https://github.com/AndrewSirenko))
* Metric Instrumentation Framework ([#1767](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/1767), [@torredil](https://github.com/torredil))

# v1.23.2
### Bug Fixes
* Add compatibility workaround for A1 instance family ([#1811](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/1811), [@ConnorJC3](https://github.com/ConnorJC3))

### Improvements
* Upgrade dependencies (and resolve CVEs found in [#1800](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/issues/1800)) ([#1809](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/1809), [@ConnorJC3](https://github.com/ConnorJC3))

# v1.23.1
### Bug Fixes
* Upgrade volume-modifier-for-k8s sidecar to 0.1.3 for Leader election conflict with csi-resizer bug fix ([#14](https://github.com/awslabs/volume-modifier-for-k8s/pull/14), [@torredil](https://github.com/torredil))

# v1.23.0
### Urgent Upgrade Notes
*(No, really, you MUST read this before you upgrade)*

The EBS CSI Driver's Linux base image was upgraded from Amazon Linux 2 (AL2) to Amazon Linux 2023 (AL2023) in this release. This change will continue to improve the performance and security of the EBS CSI Driver via updates available only on AL2023.

As part of this change, e2fsprogs will be upgraded from `1.42.9` to `1.46.5` and xfsprogs will be upgraded from `5.0.0` to `5.18.0`. New volumes created on versions of the EBS CSI Driver with an AL2023 base image may fail to mount or resize on versions of the EBS CSI Driver with an AL2 base image. For this reason, downgrading the EBS CSI Driver across base images will not be supported and is strongly discouraged. Please see [[Announcement] Base image upgrade to AL2023 · Issue #1719 · kubernetes-sigs/aws-ebs-csi-driver](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/issues/1719) to provide any questions or feedback.

### Notable Changes
* PreStop lifecycle hook to alleviate 6+ minute force-detach delay ([#1736](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/1736), [@torredil](https://github.com/torredil))
* Add option for opentelemetry tracing of gRPC calls ([#1714](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/1714), [@Fricounet](https://github.com/Fricounet))
* Upgrade Linux base image to AL2023 ([#1731](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/1731), [@AndrewSirenko](https://github.com/AndrewSirenko))

### Bug Fixes
* Do not call ModifyVolume if the volume is already in the desired state ([#1741](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/1741), [@ConnorJC3](https://github.com/ConnorJC3))

### Improvements
* Dependancy upgrades ([#1743](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/1743), [@AndrewSirenko](https://github.com/AndrewSirenko))

# v1.22.1
### Bug Fixes
* Cherry-pick from v1.23.1: Do not call ModifyVolume if the volume is already in the desired state ([#1741](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/1741), [@ConnorJC3](https://github.com/ConnorJC3))
* Upgrade volume-modifier-for-k8s sidecar to 0.1.3 for Leader election conflict with csi-resizer bug fix ([#14](https://github.com/awslabs/volume-modifier-for-k8s/pull/14), [@torredil](https://github.com/torredil))

# 1.22.0
### Urgent Upgrade Notes
*(No, really, you MUST read this before you upgrade)*

In an upcoming version, the EBS CSI Driver will upgrade the base image from AL2 to AL2023. For more information and to provide feedback about this change, see [issue #1719](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/issues/1719)

### Notable Changes
* Request coalescing for resizing and modifying volume ([#1676](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/1676), [@hanyuel](https://github.com/hanyuel))
* Support specifying inode size for filesystem format ([#1661](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/1661), [@fgksgf](https://github.com/fgksgf))

### Bug Fixes
* Correct volume limits for i4i instance types ([#1699](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/1699), [@talnevo](https://github.com/talnevo))
* Use SSM to get latest stable AMI for EC2 nodes ([#1689](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/1689), [@torredil](https://github.com/torredil))
* Add `i4i.large` to volume limits config ([#1715](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/1715), [@torredil](https://github.com/torredil))

### Improvements
* Add volume limits for m7i family ([#1710](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/1710), [@ConnorJC3](https://github.com/ConnorJC3))

### Misc
* Bump golang.org/x/net/html to fix CVE-2023-3978 ([#1711](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/1711), [@jsafrane](https://github.com/jsafrane))

# v1.21.0
### Bug Fixes
* Enable setting throughput without specifying volume type when modifying volumes ([#1667](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/1667), [@Indresh2410](https://github.com/Indresh2410))
* Reorder device names to prevent bad behavior on non-nitro instance types ([#1675](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/1675), [@ConnorJC3](https://github.com/ConnorJC3))

### Improvements
* Replace deprecated command with environment file in CI ([#1636](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/1636), [@jongwooo](https://github.com/jongwooo))

# v1.20.0
### Notable Changes
* Enable leader election in csi-resizer sidecar ([#1606](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/1606), [@rdpsin](https://github.com/rdpsin))
* Namespace-scoped leases permissions ([#1614](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/1614), [@torredil](https://github.com/torredil))
* Add additionalArgs parameter for sidecars ([#1627](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/1627), [@ConnorJC3](https://github.com/ConnorJC3))
* Fix context handling in WaitForVolumeAttachment & add in-flight checks to attachment/detachment operations ([#1621](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/1621), [@torredil](https://github.com/torredil))
* Extend resource list in Kustomization file ([#1634](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/1634), [@torredil](https://github.com/torredil))

### Bug Fixes
* Idempotent unmount from NodeUnstageVolume / NodeUnpublishVolume ([#1605](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/1605), [@dobsonj](https://github.com/dobsonj))
* Remove condition on iopspergb key being mandatory for io1 volumes ([#1590](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/1590), [@surian](https://github.com/surian))
* Avoid generating manifests with empty envFrom fields ([#1630](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/1630), [@mvgmb](https://github.com/mvgmb))
* Update DM allocator to use all available names ([#1626](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/1626), [@ConnorJC3](https://github.com/ConnorJC3))

### Improvements
* Update logline to remove "formatted" ([#1612](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/1612), [@odinuge](https://github.com/odinuge))
* Bump kOps k8s version to 1.27; Bump eksctl k8s version to 1.26 ([#1567](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/1567), [@ConnorJC3](https://github.com/ConnorJC3))
* Revert Increase external test pod start timeout ([#1615](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/1615), [@torredil](https://github.com/torredil))
* Remove old coverage banner from README ([#1617](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/1617), [@jacobwolfaws](https://github.com/jacobwolfaws))
* Allow to set automountServiceAccountToken in ServiceAccount ([#1619](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/1619), [@kahirokunn](https://github.com/kahirokunn))
* Upgrade dependencies ([#1637](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/1637), [@torredil](https://github.com/torredil))

# v1.19.0
### Urgent Upgrade Notes
*(No, really, you MUST read this before you upgrade)*

Windows 20H2 hosts are no longer supported. Windows 20H2 is [no longer supported by Microsoft](https://learn.microsoft.com/en-us/lifecycle/announcements/windows-10-20h2-end-of-servicing).

### Notable Changes
* Add support for annotation-based volume modification via volume-modifier-for-k8s sidecar ([#1600](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/1600), [@rdpsin](https://github.com/rdpsin))
* Add startup taint removal feature ([#1588](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/1588), [@ConnorJC3](https://github.com/ConnorJC3) and [@gtxu](https://github.com/gtxu))

### Bug Fixes
* Check for 'not mounted' in linux Unstage/Unpublish ([#1597](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/1597), [@ConnorJC3](https://github.com/ConnorJC3))
* Update list of nitro instances ([#1573](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/1573), [@patderek](https://github.com/patderek))
* Allow throughput with defaulted GP3 volume type ([#1584](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/1584), [@ConnorJC3](https://github.com/ConnorJC3))
* Use dl.k8s.io instead of kubernetes-release bucket ([#1593](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/1593), [@ratnopamc](https://github.com/ratnopamc))

### Improvements
* Migrate to EKS-D Windows base images ([#1601](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/1601), [@ConnorJC3](https://github.com/ConnorJC3))
* Drop support for Windows 20H2 ([#1598](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/1598), [@torredil](https://github.com/torredil))
* Add option to append extra string to user agent ([#1599](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/1599), [@torredil](https://github.com/torredil))

# v1.18.0
### Urgent Upgrade Notes
*(No, really, you MUST read this before you upgrade)*

This will be the last minor release of the AWS EBS CSI Driver to support Windows 20H2 hosts. Windows 20H2 is [no longer supported by Microsoft](https://learn.microsoft.com/en-us/lifecycle/announcements/windows-10-20h2-end-of-servicing). Future releases of the AWS EBS CSI Driver will no longer be built for Windows 20H2.

### Notable Changes
* Add support for Fast Snapshot Restore ([#1554](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/1554), [@torredil](https://github.com/torredil))
* Support for interpolated tags in `VolumeSnapshotClass` ([#1558](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/1558), [@hanyuel](https://github.com/hanyuel))
* Add target to run External Storage tests on Windows nodes ([#1521](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/1521), [@torredil](https://github.com/torredil))

### Bug Fixes
* Add non-negative check on volume limit for `CSINode` ([#1542](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/1542), [@gtxu](https://github.com/gtxu))
* Fix volume mounts on AWS Snow devices ([#1546](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/1546), [@ConnorJC3](https://github.com/ConnorJC3))
* Improve consistency/idempotency of Windows mounting ([#1526](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/1526), [@torredil](https://github.com/torredil))
* Add liveness probe for node-driver-registrar container ([#1570](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/1570), [@gtxu](https://github.com/gtxu))
* Fix calculation of attached block devices from IMDS ([#1561](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/1561), [@torredil](https://github.com/torredil))

### Improvements
* Migrate Kustomize configuration from 'bases' to 'resources' ([#1539](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/1539), [@torredil](https://github.com/torredil))
* Reduce CI flakiness by removing unnecessary SSH certificates ([#1566](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/1566), [@ConnorJC3](https://github.com/ConnorJC3))

# v1.17.0
### Urgent Upgrade Notes
*(No, really, you MUST read this before you upgrade)*

[`k8s.gcr.io` will be redirected on Monday March 20th](https://kubernetes.io/blog/2023/03/10/image-registry-redirect/), and may stop working entirely in the near future. If you are using `k8s.gcr.io` you MUST [move to `registry.k8s.io`](https://kubernetes.io/blog/2023/02/06/k8s-gcr-io-freeze-announcement/) to continue receiving support.

Issues related to `k8s.gcr.io` will no longer be accepted. `public.ecr.aws` and `registry.k8s.io` images are unaffected and remain supported as per [the support policy](https://github.com/kubernetes-sigs/aws-ebs-csi-driver#support).

### Notable Changes
* Add support for XFS custom block sizes ([#1523](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/1523), [@ConnorJC3](https://github.com/ConnorJC3))
* Add support for instances with more than 52 volumes attached ([#1518](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/1518), [@ConnorJC3](https://github.com/ConnorJC3))

### Bug Fixes
* Fix improper handling of manually-mounted volumes ([#1518](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/1518), [@ConnorJC3](https://github.com/ConnorJC3))

### Improvements
* Log driver version in lower verbosities ([#1525](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/1525), [@torredil](https://github.com/torredil))
* Upgrade dependencies ([#1529](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/1529), [@torredil](https://github.com/torredil))

# v1.16.1
### Notable Changes
* Security fixes

# v1.16.0
### Notable Changes
* Add support for JSON logging ([#1467](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/1467), [@torredil](https://github.com/torredil))
    * `--logging-format` flag has been added to set the log format. Valid values are `text` and `json`. The default value is `text`.
    * `--logtostderr` is deprecated.
    * Long arguments prefixed with `-` are no longer supported, and must be prefixed with `--`. For example, `--volume-attach-limit` instead of `-volume-attach-limit`.
* k8s.gcr.io -> registry.k8s.io ([#1488](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/1488), [@ConnorJC3](https://github.com/ConnorJC3))
    * The GCR manifests now use `registry.k8s.io` instead of `k8s.gcr.io` for the image repository. For users that rely on it, the images will still be pushed to `k8s.gcr.io` for the forseeable future, but we recommend migration to `registry.k8s.io` as soon as reasonably possible. For more information, see [registry.k8s.io: faster, cheaper and Generally Available (GA)](https://kubernetes.io/blog/2022/11/28/registry-k8s-io-faster-cheaper-ga/).
* The sidecars have been updated. The new versions are:
    - csi-provisioner: `v3.4.0`
    - csi-attacher: `v4.1.0`
    - csi-snapshotter: `v6.2.1`
    - livenessprobe: `v2.9.0`
    - csi-resizer: `v1.7.0`
    - node-driver-registrar: `v2.7.0`

### Improvements
* Bump CI k8s version to 1.26.1 (and other CI tools upgrades) ([#1487](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/1487), [@ConnorJC3](https://github.com/ConnorJC3))
* Bump GitHub Actions workflows ([#1491](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/1491), [@ConnorJC3](https://github.com/ConnorJC3))
* Upgrade golangci-lint ([#1505](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/1505), [@torredil](https://github.com/torredil))

### Bug Fixes
* Use test driver image when testing upgrades with CT ([#1486](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/1486), [@torredil](https://github.com/torredil))
* Disable buildx provenance ([#1491](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/1491), [@ConnorJC3](https://github.com/ConnorJC3))

# v1.15.1
### Notable Changes
* Security fixes

# v1.15.0
### Notable Changes
* Support specifying block size for filesystem format ([#1452](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/1452), [@ConnorJC3](https://github.com/ConnorJC3))
* Change default sidecars to EKS-D ([#1475](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/1475), [@ConnorJC3](https://github.com/ConnorJC3), [@torredil](https://github.com/torredil))
* The sidecars have been updated. The new versions are:
    - csi-provisioner: `v3.3.0`
    - csi-attacher: `v4.0.0`
    - csi-snapshotter: `v6.1.0`
    - livenessprobe: `v2.8.0`
    - csi-resizer: `v1.6.0`
    - node-driver-registrar: `v2.6.2`

### Bug Fixes
* Manually setup remote for CT on Prow ([#1473](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/1473), [@ConnorJC3](https://github.com/ConnorJC3))
* Fix volume limits for `m6id` and `x2idn` instance types ([#1463](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/1463), [@talnevo](https://github.com/talnevo))

### Improvements
* Update compatibility info in README ([#1465](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/1465), [@torredil](https://github.com/torredil))

### Acknowledgments
* We would like to sincerely thank:
[@talnevo](https://github.com/talnevo)

# v1.14.1
### Bug Fixes
* (Cherry-Pick) Fixed handling of volume limits for instance types m6id and x2idn

# v1.14.0
### Improvements
* Bumped golang dependencies
* Rebuilt driver container with newer base image (containing security fixes)
* In the next minor release (v1.15.0, scheduled for January) the default sidecars will be changed, see https://github.com/kubernetes-sigs/aws-ebs-csi-driver/issues/1456

# v1.13.0
### Bug Fixes

* Add version information from tag to GCR build ([#1426](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/1426), [@ConnorJC3](https://github.com/ConnorJC3))
* `pkg/driver/controller.go` uses ToLower ([#1429](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/1429), [@yevhenvolchenko](https://github.com/yevhenvolchenko))
* Increase cloudbuild timeout ([#1430](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/1430), [@torredil](https://github.com/torredil))
* Use `PULL_BASE_REF` for `VERSION` instead of `GIT_TAG` for GCR builds ([#1439](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/1439), [@ConnorJC3](https://github.com/ConnorJC3))
* Grab version via tag directly from git ([#1441](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/1441), [@ConnorJC3](https://github.com/ConnorJC3))

### Improvements
* Upgrade K8s to `v1.25`; Upgrade ginkgo to `v2`; Use upstream binary for `e2e-kubernetes` ([#1341](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/1341), [@torredil](https://github.com/torredil))
* Add release and support policy to README.md ([#1392](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/1392), [@torredil](https://github.com/torredil))
* Update and run update-gomock ([#1422](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/1422), [@torredil](https://github.com/torredil))
* Upgrade Go/CI dependencies ([#1433](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/1433), [@torredil](https://github.com/torredil))
* Upgrade golangci-lint; Fix linter errors ([#1435](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/1435), [@torredil](https://github.com/torredil))

### Acknowledgments
* We would like to sincerely thank:
[@yevhenvolchenko](https://github.com/yevhenvolchenko)

# v1.12.1
### Security
* Addreses [ALAS2-2022-1854](https://alas.aws.amazon.com/AL2/ALAS-2022-1854.html) and [ALAS2-2022-1849](https://alas.aws.amazon.com/AL2/ALAS-2022-1849.html)

# v1.11.5
### Backported Security
* Addreses [ALAS2-2022-1854](https://alas.aws.amazon.com/AL2/ALAS-2022-1854.html) and [ALAS2-2022-1849](https://alas.aws.amazon.com/AL2/ALAS-2022-1849.html)

# v1.12.0
### Notable Changes
* Unify IOPS handling across volume types ([#1366](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/1366), [@torredil](https://github.com/torredil))
* Change fsGroupPolicy to File ([#1377](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/1377), [@ConnorJC3](https://github.com/ConnorJC3))
* Add resolver to handle custom endpoints ([#1398](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/1398), [@bertinatto](https://github.com/bertinatto))
* Add enableMetrics configuration ([#1380](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/1380), [@torredil](https://github.com/torredil))
* Build Windows container for Windows Server 2022 LTSC ([#1408](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/1408), [@ConnorJC3](https://github.com/ConnorJC3))
* Add support for io2 Block Express volumes ([#1409](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/1409), [@ConnorJC3](https://github.com/ConnorJC3))

### Bug Fixes
* c6i.metal and g5g.metal are nitro instances ([#1358](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/1358), [@wmesard](https://github.com/wmesard))
* Update release notes; Implement useOldCSIDriver parameter ([#1391](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/1391), [@ConnorJC3](https://github.com/ConnorJC3))

### Improvements
* Add controller nodeAffinity to prefer EC2 over Fargate ([#1360](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/1360), [@torredil](https://github.com/torredil))
* Add warning message when region is unavailable on the controller ([#1359](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/1359), [@ConnorJC3](https://github.com/ConnorJC3))
* Retrieve region/AZ from topology label ([#1360](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/1360), [@torredil](https://github.com/torredil))
* Update the kustomization deployment to latest image tag ([#1367](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/1367), [@gtxu](https://github.com/gtxu))
* Update module k8s.io/klog to v2 ([#1370](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/1370), [@torredil](https://github.com/torredil))
* Updating static example to include setting fsType ([#1376](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/1376), [@jbehrends](https://github.com/jbehrends))
* Allow all taint for csi-node by default ([#1381](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/1381), [@gtxu](https://github.com/gtxu))
* add link to install guide ([#1383](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/1383), [@geoffcline](https://github.com/geoffcline))
* Add self to OWNERS ([#1399](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/1399), [@ConnorJC3](https://github.com/ConnorJC3))
* Cleanup OWNERS ([#1403](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/1403), [@ConnorJC3](https://github.com/ConnorJC3))
* Add snow device types to parameters ([#1404](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/1404), [@TerryHowe](https://github.com/TerryHowe))
* revise preqs for install docs ([#1389](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/1389), [@geoffcline](https://github.com/geoffcline))
* Update workflows ([#1401](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/1401), [@torredil](https://github.com/torredil))
* Add .image-* files from Makefile to .gitignore ([#1410](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/1410), [@ConnorJC3](https://github.com/ConnorJC3))
* Update trivy.yaml workflow event trigger ([#1411](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/1411), [@torredil](https://github.com/torredil))

### Acknowledgments
* We would like to sincerely thank:
[@TerryHowe](https://github.com/TerryHowe), [@bertinatto](https://github.com/bertinatto), [@geoffcline](https://github.com/geoffcline), & [@jbehrends](https://github.com/jbehrends)

# v1.11.4
### Improvements
* Update go version; Update dependencies ([#1394](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/1394), [@torredil](https://github.com/torredil))
    - go `1.17` -> `1.19`
    - github.com/aws/aws-sdk-go `v1.44.45` -> `v1.44.101`
    - github.com/google/go-cmp `v0.5.8` -> `v0.5.9`
    - github.com/onsi/gomega `v1.19.0` -> `v1.20.2`
    - golang.org/x/sys `v0.0.0-20220728004956-3c1f35247d10` -> `v0.0.0-20220919091848-fb04ddd9f9c8`
    - google.golang.org/grpc `v1.47.0` -> `v1.49.0`

# v1.11.3
### Vulnerability Fixes
* Address CVEs ([#1384](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/1384), [@torredil](https://github.com/torredil))
    - Upgrade github.com/prometheus/client_golang `v1.11.0` -> `v1.11.1` to address [CVE-2022-21698](https://github.com/advisories/GHSA-cg3q-j54f-5p7p).
    - Upgrade golang.org/x/net `v0.0.0-20220225172249-27dd8689420f` -> `v0.0.0-20220906165146-f3363e06e74c` to address [CVE-2022-27664](https://github.com/advisories/GHSA-69cg-p879-7622).

# v1.11.2
### Notable Changes
* Enable EBS CSI driver for AWS Snow devices ([#1314](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/1314), [@jigisha620](https://github.com/jigisha620))
* Implement securityContext for containers ([#1333](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/1333), [@ConnorJC3](https://github.com/ConnorJC3))

### Bug Fixes
* Apply fix from helm chart to kustomize manifests ([#1350](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/1350), [@ConnorJC3](https://github.com/ConnorJC3))
* Explicitly pass VERSION as a build-arg ([#1351](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/1351), [@torredil](https://github.com/torredil))

### Miscellaneous
* Automate ECR release ([#1339](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/1339), [@torredil](https://github.com/torredil))
* Remove /vendor directory ([#1328](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/1338), [@torredil](https://github.com/torredil))
* Set release draft to true ([#1351](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/1351), [@torredil](https://github.com/torredil))
* Set VERSION env variable in publish-ecr workflow ([#1346](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/1346), [@torredil](https://github.com/torredil))
* doc: update pvc binding ([#1337](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/1337), [@vikram-katkar](https://github.com/vikram-katkar))
* Skip Testpattern: Pre-provisioned PV in migration tests ([#1329](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/1329), [@torredil](https://github.com/torredil))
* Only run helm action when Chart.yaml modified ([#1334](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/1334), [@ConnorJC3](https://github.com/ConnorJC3))
* Update parameters.md ([#1329](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/1329), [@ConnorJC3](https://github.com/ConnorJC3), [@torredil](https://github.com/torredil))
* Update to kOps v1.23.0 ([#1329](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/1329), [@wongma7](https://github.com/wongma7), [@ConnorJC3](https://github.com/ConnorJC3), [@torredil](https://github.com/torredil))
* Improve build time ([#1331](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/1331), [@torredil](https://github.com/torredil))
* Pass GOPROXY to image builder ([#1330](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/1330), [@wongma7](https://github.com/wongma7))
* Run hack/update-gofm with go1.19rc2 ([#1325](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/1325), [@torredil](https://github.com/torredil))

### Acknowledgments
* We would like to sincerely thank:
[@jigisha620](https://github.com/jigisha620), [@ConnorJC3](https://github.com/ConnorJC3), [@wongma7](https://github.com/wongma7), [@olemarkus](https://github.com/olemarkus), [@vikram](https://github.com/vikram)

*Versions [v1.11.0, v1.11.1] were skipped due to incorrect version metadata in the container.*

# v1.10.0
## Announcement
* OS/Architecture specific tags are no longer being pushed to public ECR ([#1315](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/issues/1315))

### Miscellaneous
* Validate `csi.storage.k8s.io/fstype` before mounting ([#1319](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/1319), [@torredil](https://github.com/torredil))
* Update install.md ([#1313](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/1313), [@torredil](https://github.com/torredil))

# v1.9.0
### Notable Changes
* Upgrade dependencies ([#1296](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/1296), [@torredil](https://github.com/torredil))
    - k8s.io/kubernetes `v1.21.11` -> `v1.22.11`
    - github.com/aws/aws-sdk-go `v1.43.37` -> `v1.44.45`
    - github.com/container-storage-interface/spec `v1.3.0` -> `v1.6.0`
    - github.com/golang/mock `v1.5.0` -> `v1.6.0`
    - github.com/golang/protobuf `v1.5.0` -> `v1.5.2`
    - github.com/google/go-cmp `v0.5.5` -> `v0.5.8`
    - github.com/kubernetes-csi/csi-proxy/client `v1.0.1` -> `v1.1.1`
    - github.com/kubernetes-csi/csi-test `v2.0.0+incompatible` -> `v2.2.0+incompatible`
    - github.com/kubernetes-csi/external-snapshotter/client/v4 `v4.0.0` -> `v4.2.0`
    - github.com/onsi/ginkgo `v1.11.0` -> `v1.16.5`
    - github.com/onsi/gomega `v1.7.1` -> `v1.19.0`
    - github.com/stretchr/testify `v1.6.1` -> `v1.8.0`
    - golang.org/x/sys `v0.0.0-20211216021012-1d35b9e2eb4e` -> `v0.0.0-20220627191245-f75cf1eec38b`
    - google.golang.org/grpc `v1.34.0` -> `v1.47.0`
* Add GitHub actions ([#1297](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/1297), [@torredil](https://github.com/torredil))
    - Fix broken CHANGELOG link in release.yaml
    - Add codeql-analysis.yaml for additional vulnerability scanning
    - Add unit-tests.yaml for multi-platform unit testing (Linux/Windows)
    - Add verify.yaml which runs `make verify`
* Update livenessprobe to `v2.6.0` ([#1303](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/1303), [@t0rr3sp3dr0](https://github.com/t0rr3sp3dr0))

### Bug Fixes
* Fix version of K8s manifest images ([#1303](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/1303), [@t0rr3sp3dr0](https://github.com/t0rr3sp3dr0))
* Fix image tags in ecr-public kustomization ([#1305](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/1305), [@torredil](https://github.com/torredil))

### Miscellaneous
* Use mount utils to check if volume needs resizing ([#1165](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/1165), [@RomanBednar](https://github.com/RomanBednar))
* Improve metadata_ec2.go error logging ([#1294](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/1294), [@torredil](https://github.com/torredil))

### Acknowledgments
* We would like to sincerely thank:
[@RomanBednar](https://github.com/RomanBednar) and [@t0rr3sp3dr0](https://github.com/t0rr3sp3dr0)

# v1.8.0
## Notable Changes
* Change base image from Amazon Linux 2 to EKS minimal for linux builds

### Acknowledgments
* We would like to sincerely thank:
[@jaxesn](https://github.com/jaxesn)

# v1.7.0
## Announcement
* To improve the security of the container images, the base image will be switched from [Amazon Linux 2](https://hub.docker.com/_/amazonlinux) to [EKS Distro Minimal](https://gallery.ecr.aws/eks-distro-build-tooling/eks-distro-minimal-base-csi-ebs) in an upcoming release. The new minimal base image only contains the necessary driver dependencies which means it will not include a shell. **Please be aware that while this change won't break workloads, it may break processes for debugging if you are using a shell**.

### Notable Changes
* Set handle-volume-inuse-error to false which fixes csi-resizer getting OOMKilled ([#1280](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/1280), [@stijndehaes](https://github.com/stijndehaes))
* Update sidecars ([#1260](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/1260), [@gtxu](https://github.com/gtxu))
* Remove container-image.yaml ([#1239](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/1239), [@torredil](https://github.com/torredil))
* Replace Windows 2004(EOL) with ltsc2019 ([#1231](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/1231), [@torredil](https://github.com/torredil))

### Features
* Enable unit testing on windows ([#1219](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/1219), [@torredil](https://github.com/torredil))

### Bug Fixes
* Fix unable to create CSI snapshot-EBS csi driver ([#1257](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/1257), [@torredil](https://github.com/torredil))
* Temporarily fix CI ([#1240](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/1240), [@torredil](https://github.com/torredil))
* Fix IOPS parameter bug when no volume type is defined ([#1236](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/1236), [@torredil](https://github.com/torredil))
* Add quotes around the extra-tags argument in chart template ([#1198](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/1198), [@Kaezon](https://github.com/Kaezon))

### Vulnerability Fixes
* Address ALAS2-2022-1801, ALAS2-2022-1802, ALAS2-2022-1805
* Update golang.org/x/crypto for CVE-2022-27191 ([#1210](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/1210), [@jsafrane](https://github.com/jsafrane))

### Miscellaneous
* Bump up Helm chart to v2.6.10 ([#1272](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/1272), [@torredil](https://github.com/torredil))
* Upgrade eksctl to v0.101.0 ([#1271](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/1271), [@torredil](https://github.com/torredil))
* Avoid git tag conflicts when vendoring hack/e2e in other repos (efs/fsx) ([#1270](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/1270), [@wongma7](https://github.com/wongma7))
* Update parameters.md ([#1269](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/1269), [@torredil](https://github.com/torredil))
* Update documentation ([#1263](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/1263), [@torredil](https://github.com/torredil))
* Bump up helm chart to v2.6.9 ([#1262](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/1262), [@torredil](https://github.com/torredil))
* Post-release v1.6.2 ([#1244](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/1244), [@gtxu](https://github.com/gtxu))
* Prepare release v1.6.2 ([#1241](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/1241), [@gtxu](https://github.com/gtxu))
* Cleanup OWNERS list ([#1238](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/1238), [@torredil](https://github.com/torredil))
* Update gcb-docker-gcloud to latest ([#1230](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/1230), [@rdpsin](https://github.com/rdpsin))
* Use docker buildx 0.8.x --no-cache-filter to avoid using cached amazon linux image ([#1221](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/1221), [@wongma7](https://github.com/wongma7))
* Add self to OWNERS ([#1229](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/1229), [@torredil](https://github.com/torredil))
* Add self to OWNERS ([#1228](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/1228), [@rdpsin](https://github.com/rdpsin))

### Acknowledgments
* We would like to sincerely thank:
[@jsafrane](https://github.com/jsafrane), [@Kaezon](https://github.com/Kaezon), [@stijndehaes](https://github.com/stijndehaes)

# v1.6.2
## Notable changes
* Address CVE ALAS-2022-1792

# v1.6.1
## Notable changes
* Address CVE ALAS2-2022-1782, ALAS2-2022-1788, ALAS2-2022-1784

# v1.6.0
## Notable changes
* Platform agnostic device removal ([#1193](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/1193), [@torredil](https://github.com/torredil))

### Bug fixes
* Fix windows mounting bug ([#1189](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/1189), [@torredil](https://github.com/torredil))

### New features
* Adding tagging support through StorageClass.parameters ([#1199](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/1199), [@rdpsin](https://github.com/rdpsin))
* Add volume resizing support for windows ([#1207](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/1207), [@torredil](https://github.com/torredil))

### Misc.
* Update deprecated command `go get` ([#1194](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/1194), [@gtxu](https://github.com/gtxu))
* Upgrade PodDisruptionBudget api version for kubernetes 1.21+ ([#1196](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/1196), [@wangshu3000](https://github.com/wangshu3000))
* Bump prometheus/client_golang to v1.11.1 ([#1197](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/1197), [@dobsonj](https://github.com/dobsonj))
* Updated TAGGING.md to mention minimum version for tagging ([#1202](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/1202), [@rdpsin](https://github.com/rdpsin))
* Update README.md to reflect correct tag key for snapshots ([#1203](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/1203), [@rdpsin](https://github.com/rdpsin))

# v1.5.3
## Notable changes
* Ensure image OCI compliance ([#1205](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/1205), [@torredil](https://github.com/torredil))
* Update driver dependencies ([#1208](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/1208), [@rdpsin](https://github.com/rdpsin))

# v1.5.2
## Notable changes
* Address CVE ALAS-2022-1764

# v1.5.1
## Notable changes
* Address CVE ALAS-2021-1552, ALAS2-2022-1736, ALAS2-2022-1738, ALAS2-2022-1743

# v1.5.0
### Misc.
* Update windows example to refer to v1.4.0 ([#1093](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/1093), [@wongma7](https://github.com/wongma7))
* Bump eksctl used in e2e tests to 0.69.0 ([#1094](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/1094), [@wongma7](https://github.com/wongma7))
* Update to go 1.17 ([#1109](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/1109), [@bertinatto](https://github.com/bertinatto))

# v1.4.0
## Notable changes
* Recognize instance-type node label when EC2 metadata isn't available ([#1060](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/1060), [@rifelpet](https://github.com/rifelpet))
* Fix windows NodePublish failing because mount target doesn't exist ([#1081](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/1081), [@wongma7](https://github.com/wongma7))
* Search for nvme device path even if non-nvme exists ([#1082](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/1082), [@wongma7](https://github.com/wongma7))

### Misc.
* Bump csi-proxy from RC v1.0.0 to GA v1.0.1 ([#1018](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/1018), [@wongma7](https://github.com/wongma7))
* Fix spacing in RELEASE.md ([#1035](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/1035), [@wongma7](https://github.com/wongma7))
* [chart] Support image.pullPolicy for csi-resizer image ([#1045](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/1045), [@jyaworski](https://github.com/jyaworski))
* merge 1.3.0 release and post-release commits into master ([#1068](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/1068), [@vdhanan](https://github.com/vdhanan))
* Allow default fstype to be overriden via values.yaml ([#1069](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/1069), [@jcrsilva](https://github.com/jcrsilva))
* Update windows example for image release ([#1070](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/1070), [@wongma7](https://github.com/wongma7))
* Refactor pkg/cloud/metadata.go into pkg/cloud/metadata_*.go files ([#1074](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/1074), [@wongma7](https://github.com/wongma7))
* Move mocks to parent package to avoid import cycle ([#1078](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/1078), [@wongma7](https://github.com/wongma7))
* deploy: Add resizer and snapshotter images to kustomization ([#1080](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/1080), [@maxbrunet](https://github.com/maxbrunet))
* deploy: Fix csi-resizer tag and bump to v1.1.0 ([#1085](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/1085), [@maxbrunet](https://github.com/maxbrunet))
* Reorder isMounted for readability ([#1087](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/1087), [@wongma7](https://github.com/wongma7))

# v1.3.1
* Push multi-arch/os image manifest to ECR.

# v1.3.0
## Notable changes
* Make NodePublish Mount Idempotent ([#1019](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/1019), [@nirmalaagash](https://github.com/nirmalaagash))
* Build and push multi-arch/os (amazon and windows, no debian) image manifest via Make rules ([#957](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/957), [@wongma7](https://github.com/wongma7))

### Bug fixes
* Fix windows build IsCorruptedMnt not implemented ([#1047](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/1047), [@wongma7](https://github.com/wongma7))
* Hash volume name to get client token ([#1041](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/1041), [@vdhanan](https://github.com/vdhanan))
* Include ClusterRole and ClusterRoleBinding for csi-node ([#1021](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/1021), [@groodt](https://github.com/groodt))
* Fix gcr prow builld failing because docker missing --os-version ([#1020](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/1020), [@wongma7](https://github.com/wongma7))
* Fix gcr prow build failing because of IMAGE variable collision ([#1017](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/1017), [@wongma7](https://github.com/wongma7))
* Fix github build failing because of wrong docker hub registry name  ([#1016](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/1016), [@wongma7](https://github.com/wongma7))

### New features
* [chart] Add controller strategy ([#1008](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/1008), [@stevehipwell](https://github.com/stevehipwell))
* [chart] Node update strategy & auto driver image tag ([#988](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/988), [@stevehipwell](https://github.com/stevehipwell))

### Misc.
* Update helm chart alongside kustomize, after images have been pushed, for consistency ([#1015](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/1015), [@wongma7](https://github.com/wongma7))
* Update kustomize templates only after verifying images are available in registries ([#995](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/995), [@wongma7](https://github.com/wongma7))

# v1.2.1
## Notable changes
- Fix mount idempotency ([#1019](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/1019), [@nirmalaagash](https://github.com/nirmalaagash))

# v1.2.0
## Notable changes
* utilize latest go sdk to ensure createVolume idempotency ([#982](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/982), [@AndyXiangLi](https://github.com/AndyXiangLi))
* Implement Windows NodePublish/Unpublish ([#823](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/823), [@wongma7](https://github.com/wongma7))
- In a future release, the debian-based image will be removed and only an al2-based image will be maintained and pushed to GCR and ECR
- In a future release, images will stop getting pushed to Docker Hub

### Bug fixes
* Update driver capabilities ([#922](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/922), [@wongma7](https://github.com/wongma7))
* update inFlight cache to avoid race condition on volume operation ([#924](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/924), [@AndyXiangLi](https://github.com/AndyXiangLi))
* Update example policy, use it in tests, and document it ([#940](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/940), [@wongma7](https://github.com/wongma7))
* Default extra-create-metadata true so that volumes get created with pvc/pv tags ([#937](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/937), [@wongma7](https://github.com/wongma7))
* Default controller.extra-create-metadata true so that volumes get created with pvc/pv tags ([#941](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/941), [@wongma7](https://github.com/wongma7))

### New features
* Implement Windows NodePublish/Unpublish ([#823](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/823), [@wongma7](https://github.com/wongma7))
* Feature/allow add debug args ([#970](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/970), [@mkkatica](https://github.com/mkkatica))
* Updated default setting of windows daemon set ([#978](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/978), [@nirmalaagash](https://github.com/nirmalaagash))
* Update to csi-proxy v1 APIs ([#966](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/966), [@wongma7](https://github.com/wongma7))

### Installation updates
* Add test-e2e-external-eks make rule that tests EKS with pod instance metadata disabled. Remove hostNetwork from DaemonSet ([#907](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/907), [@wongma7](https://github.com/wongma7))
* helm chart configurable log verbosity ([#908](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/908), [@wongma7](https://github.com/wongma7))
* Fix podLabels case in Helm chart ([#925](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/925), [@eytanhanig](https://github.com/eytanhanig))
* Add KubernetesCluster tag to provisioned volumes when cluster-id set ([#932](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/932), [@wongma7](https://github.com/wongma7))
* Stop pushing latest tag and remove all references to it ([#949](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/949), [@wongma7](https://github.com/wongma7))
* Install snapshot controller independently of helm for e2e tests ([#968](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/968), [@wongma7](https://github.com/wongma7))
* Several breaking changes to the helm chart ([#965](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/965), [@krmichel](https://github.com/krmichel))
* Increased the helm chart version ([#980](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/980), [@nirmalaagash](https://github.com/nirmalaagash))
* [helm-chart] csi-snapshotter in ebs-csi-controller now checks for enableVolumeSnapshot before including it in containers ([#960](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/960), [@missingcharacter](https://github.com/missingcharacter))

### Misc.
* Disable uuid checks on XFS ([#913](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/913), [@jsafrane](https://github.com/jsafrane))
* merge v1.1.0 release commits back to master ([#921](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/921), [@vdhanan](https://github.com/vdhanan))
* Add migration upgrade/downgrade test ([#927](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/927), [@wongma7](https://github.com/wongma7))
* Grant EKSCTL_ADMIN_ROLE admin access to eksctl clusters ([#933](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/933), [@wongma7](https://github.com/wongma7))
* Adding CRDs VolumeSnapshotClass, VolumeSnapshotContent, VolumeSnapshot for snapshot.storage.k8s.io/v1 ([#938](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/938), [@missingcharacter](https://github.com/missingcharacter))
* Revert "Fix kustomize RBAC bindings to have namespace kube-system" ([#947](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/947), [@TheRealDwright](https://github.com/TheRealDwright))
* Clarify that using instance profile for permission requires instance metadata access on ([#952](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/952), [@wongma7](https://github.com/wongma7))
* Release v1.1.1 and chart v1.2.4 ([#959](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/959), [@wongma7](https://github.com/wongma7))
* Download fixed version of eksctl to avoid bugs ([#967](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/967), [@wongma7](https://github.com/wongma7))
* Nit: Fix typo in the CHANGELOG ([#971](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/971), [@ialidzhikov](https://github.com/ialidzhikov))
* Add how to consume new hack/e2e scripts in other repos (efs/fsx) ([#972](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/972), [@wongma7](https://github.com/wongma7))
* Updated README.md and changed the version in snapshot example ([#976](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/976), [@nirmalaagash](https://github.com/nirmalaagash))
* Update base images: yum update al2, bump debian tag ([#986](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/986), [@wongma7](https://github.com/wongma7))
* Release 1.1.3 ([#992](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/992), [@wongma7](https://github.com/wongma7))
* add ecr images to readme ([#998](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/998), [@vdhanan](https://github.com/vdhanan))

# v1.1.4

## Notable changes
- Fix mount idempotency ([#1019](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/1019), [@nirmalaagash](https://github.com/nirmalaagash))

# v1.1.3

## Notable changes
- Fix ecr image being debian-based
- In a future release, the debian-based image will be removed and only an al2-based image will be maintained and pushed to GCR and ECR
- In a future release, images will stop getting pushed to Docker Hub

# v1.1.2

## Notable changes
- Update base images: yum update al2, bump debian tag ([#986](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/986), [@wongma7](https://github.com/wongma7))

# v1.1.1

### Bug fixes
- update inFlight cache to avoid race condition on volume operation ([#924](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/924), [@AndyXiangLi](https://github.com/AndyXiangLi))

# v1.1.0

## Notable changes
- Helm chart cleaned up ([#856](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/856), [@krmichel](https://github.com/krmichel))

### New features
* Add podAnnotations to snapshotController StatefulSet ([#884](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/884), [@snstanton](https://github.com/snstanton))
* Support custom pod labels in Helm chart ([#905](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/905), [@eytanhanig](https://github.com/eytanhanig))

### Bug fixes
* fix naming mistake in clusterrolebinding, expose env var to controller via downward api ([#874](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/874), [@vdhanan](https://github.com/vdhanan))
* Fix kustomize RBAC bindings to have namespace kube-system ([#878](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/878), [@wongma7](https://github.com/wongma7))
* rename node clusterrolebinding to make auto upgrade work ([#894](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/894), [@vdhanan](https://github.com/vdhanan))
* remove hardcoded namespace for pod disruption budget ([#895](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/895), [@vdhanan](https://github.com/vdhanan))
* Only initialize the in-cluster kube client when metadata service is actually unavailable ([#897](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/897), [@chrisayoub](https://github.com/chrisayoub))
* Reduce default log level to 2 ([#903](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/903), [@wongma7](https://github.com/wongma7))
* Add pod disruption budgets that got missed in a rebase ([#906](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/906), [@krmichel](https://github.com/krmichel))
* remove WellKnownTopologyKey from PV Topology ([#912](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/912), [@Elbehery](https://github.com/Elbehery))
* Skip volume expansion if block node ([#916](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/916), [@gnufied](https://github.com/gnufied))

### Misc.
* Add eksctl support to e2e scripts ([#852](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/852), [@wongma7](https://github.com/wongma7))
* release v1.0.0 ([#865](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/865), [@vdhanan](https://github.com/vdhanan))
* add self as owner ([#866](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/866), [@vdhanan](https://github.com/vdhanan))
* bump helm chart version ([#881](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/881), [@vdhanan](https://github.com/vdhanan))
* add custom useragent suffix ([#910](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/910), [@vdhanan](https://github.com/vdhanan))
* Bump chart-releaser-action to v1.2.1 ([#914](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/914), [@gliptak](https://github.com/gliptak))

# v1.0.0
## Notable changes
- With this release, the EBS CSI Driver is now Generally Available!

### New features
* add options to enable aws sdk debug log and add more logs when driver… ([#830](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/830), [@AndyXiangLi](https://github.com/AndyXiangLi))
* Emit AWS API operation duration/error/throttle metrics ([#842](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/842), [@wongma7](https://github.com/wongma7))
* add pod disruption budget for csi controller ([#857](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/857), [@vdhanan](https://github.com/vdhanan))

### Bug fixes
* Resize filesystem when restore a snapshot to larger size volume ([#753](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/753), [@AndyXiangLi](https://github.com/AndyXiangLi))
* handling describe instances consistency issue ([#801](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/801), [@vdhanan](https://github.com/vdhanan))
* Cap IOPS when calculating from iopsPerGB ([#809](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/809), [@jsafrane](https://github.com/jsafrane))
* Fix broken gomocks ([#843](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/843), [@wongma7](https://github.com/wongma7))
* Fix missing import ([#849](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/849), [@wongma7](https://github.com/wongma7))
* instance metadata issue fix ([#855](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/855), [@vdhanan](https://github.com/vdhanan))

### Misc.
* release v0.10.0 ([#820](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/820), [@vdhanan](https://github.com/vdhanan))
* release v0.10.1 ([#827](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/827), [@AndyXiangLi](https://github.com/AndyXiangLi))
* Rebase 1.21 ([#828](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/828), [@jsafrane](https://github.com/jsafrane))
* update installation command to use latest stable version ([#832](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/832), [@AndyXiangLi](https://github.com/AndyXiangLi))
* Bump/reconcile sidecar versions in helm/kustomize ([#834](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/834), [@wongma7](https://github.com/wongma7))
* update IAM policy sample and add new driver level tag ([#835](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/835), [@AndyXiangLi](https://github.com/AndyXiangLi))
* Switch to non-deprecated apiVersion ([#836](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/836), [@dntosas](https://github.com/dntosas))
* Update readme file to provide more info on driver options and tagging ([#844](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/844), [@AndyXiangLi](https://github.com/AndyXiangLi))
* Add empty StorageClasses to static example ([#850](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/850), [@wongma7](https://github.com/wongma7))
* Add additional logging for outpost arn handling ([#851](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/851), [@ayberk](https://github.com/ayberk))

# v0.10.1
## Notable changes
* support volume partition, users can specify partition in the pv and driver will mount the device on the specified partition ([#824](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/824), [@AndyXiangLi](https://github.com/AndyXiangLi))

### Misc.
* Warn users of migrating without draining ([#822](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/822), [@partcyborg](https://github.com/partcyborg))

# v0.10.0

## Notable changes
- Prep for Windows support: Copy pkg/mounter and refactor to use k8s.io/mount-utils ([#786](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/786), [@wongma7](https://github.com/wongma7))
- Add well-known topology label ([#773](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/773), [@ayberk](https://github.com/ayberk))
- Update livenessprobe image version from 2.1.0 to 2.2.0 ([#756](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/756), [@mowangdk](https://github.com/mowangdk))
- Remove arm overlay ([#719](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/719), [@ayberk](https://github.com/ayberk))
- Add readiness probe so controller does not report "Ready" prematurely ([#751](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/751), [@vdhanan](https://github.com/vdhanan))
- Add toleration time to NoExecute effect ([#776](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/776), [@AndyXiangLi](https://github.com/AndyXiangLi))

### New features
* Add ability to specify topologySpreadConstraints ([#770](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/770), [@arcivanov](https://github.com/arcivanov))

### Bug fixes
* delete leaked volume if driver don't know the volume status ([#771](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/771), [@AndyXiangLi](https://github.com/AndyXiangLi))
* modify error message when request volume is in use with other node ([#698](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/698), [@AndyXiangLi](https://github.com/AndyXiangLi))
* Make CreateVolume idempotent ([#744](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/744), [@chrishenzie](https://github.com/chrishenzie))

### Misc.
* Add documentation for release process ([#610](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/610), [@ayberk](https://github.com/ayberk))
* feat: Add option to provision StorageClasses ([#697](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/697), [@gazal-k](https://github.com/gazal-k))
* Refactor inFlight key to add lock per volumeId ([#702](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/702), [@AndyXiangLi](https://github.com/AndyXiangLi))
* Add support for node existing service accounts ([#704](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/704), [@mper0003](https://github.com/mper0003))
* More controll over snapshot-controller scheduling ([#708](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/708), [@alex-berger](https://github.com/alex-berger))
* Remove hardcoded snapshot controller image references ([#711](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/711), [@ig0rsky](https://github.com/ig0rsky))
* release 0.9.0 ([#718](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/718), [@AndyXiangLi](https://github.com/AndyXiangLi))
* Move cr.yaml out of github workflows ([#720](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/720), [@ayberk](https://github.com/ayberk))
* Bump chart version ([#724](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/724), [@ayberk](https://github.com/ayberk))
* Integrate external e2e test in the testsuits ([#726](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/726), [@AndyXiangLi](https://github.com/AndyXiangLi))
* Allow all fields to be set on StorageClasses ([#730](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/730), [@haines](https://github.com/haines))
* [chart] Allow resources override for node DaemonSet + priorityClassName ([#732](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/732), [@dntosas](https://github.com/dntosas))
* [chart]  Add storage class annotation and label handling ([#734](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/734), [@nicholasmhughes](https://github.com/nicholasmhughes))
* Updated installation to use latest 0.9 release ([#735](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/735), [@PhilThurston](https://github.com/PhilThurston))
* patch stable release to use gcr image ([#740](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/740), [@AndyXiangLi](https://github.com/AndyXiangLi))
* correct kustomization gcr image repo ([#742](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/742), [@AndyXiangLi](https://github.com/AndyXiangLi))
* Update ECR overlay ([#745](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/745), [@ayberk](https://github.com/ayberk))
* Set enableVolumeScheduling to true by default in the helm chart ([#752](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/752), [@mtougeron](https://github.com/mtougeron))
* Sets the imagePullSecrets if the value is set in the chart ([#755](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/755), [@mtougeron](https://github.com/mtougeron))
* Update test k8s version to 1.18.16 ([#759](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/759), [@ayberk](https://github.com/ayberk))
* add a document separator for storageclass template file ([#762](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/762), [@nvnmandadhi](https://github.com/nvnmandadhi))
* Allow setting http proxy and no proxy environment values ([#765](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/765), [@rubroboletus](https://github.com/rubroboletus))
* Fix error message when IOPSPerGB is missing in io1 volumes ([#767](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/767), [@jsafrane](https://github.com/jsafrane))
* removed harcoded NAMESPACE from helm chart ([#768](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/768), [@alexandrst88](https://github.com/alexandrst88))
* Aws client config: increase MaxRetries ([#769](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/769), [@josselin-c](https://github.com/josselin-c))
* Update chart version ([#772](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/772), [@ayberk](https://github.com/ayberk))
* Add self as reviewer ([#774](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/774), [@AndyXiangLi](https://github.com/AndyXiangLi))
* go mod tidy ([#777](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/777), [@vdhanan](https://github.com/vdhanan))
* Removing prestop hook for node-driver-registrar ([#778](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/778), [@tsunny](https://github.com/tsunny))
* hack/e2e: Support passing helm values as values.yaml and make other similar files optional ([#787](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/787), [@wongma7](https://github.com/wongma7))
* Print csi plugin logs at end of e2e test ([#789](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/789), [@wongma7](https://github.com/wongma7))
* Update snapshot controller resources ([#791](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/791), [@tirumerla](https://github.com/tirumerla))
* Remove storageclass from static example ([#794](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/794), [@wongma7](https://github.com/wongma7))
* Don't exit script prematurely if test fails ([#802](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/802), [@wongma7](https://github.com/wongma7))
* csi.storage.k8s.io/fstype is case sensitive ([#807](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/807), [@jsafrane](https://github.com/jsafrane))
* fix deploy stable ecr error kustomization file ([#808](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/808), [@ABNER-1](https://github.com/ABNER-1))
* release v0.9.1 ([#813](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/813), [@vdhanan](https://github.com/vdhanan))
* Use the old topology key for e2e tests ([#814](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/814), [@ayberk](https://github.com/ayberk))
* Track driver deploy time in e2e test pipeline ([#815](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/815), [@AndyXiangLi](https://github.com/AndyXiangLi))
* AWS EBS CSI Driver Helm chart to inject environment variables ([#817](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/817), [@tomdymond](https://github.com/tomdymond))

# v0.9.1

## Notable changes
- Change helm deploy settings: default tolerationAllTaints to false, NoExecute toleration time is 300s and will tolerate `CriticalAddonsOnly`

### New features
* Integrate external e2e test in the testsuits ([#726](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/726), [@AndyXiangLi](https://github.com/AndyXiangLi))

### Bug fixes
* delete leaked volume if driver don't know the volume status ([#771](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/771), [@AndyXiangLi](https://github.com/AndyXiangLi))
* Update test k8s version to 1.18.16 ([#759](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/759), [@ayberk](https://github.com/ayberk))

# v0.9.0

## Notable changes
- All images (including sidecars) are Multiarch
- Enable volume stats metrics on Node service

### New features
* Feature: Add ability to customize node daemonset nodeselector ([#647](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/647), [@pliu](https://github.com/pliu))
* add volume stats metrics - ([#677](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/677), [@AndyXiangLi](https://github.com/AndyXiangLi))
* Add support for existing service accounts ([#688](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/688), [@ayberk](https://github.com/ayberk))
* NodeExpandVolume no-op for raw block ([#695](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/695), [@AndyXiangLi](https://github.com/AndyXiangLi))
* Allow specifying --volume-attach-limit in the helm chart ([#700](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/700), [@keznikl](https://github.com/keznikl))

### Bug fixes
* Fix outdated ecr login command ([#680](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/680), [@wongma7](https://github.com/wongma7))
* Update sidecars to newer version ([#707](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/707), [@AndyXiangLi](https://github.com/AndyXiangLi))

### Misc.
* Update README.md ([#607](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/607), [@robisoh88](https://github.com/robisoh88))
* Add self to OWNERS ([#638](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/638), [@ayberk](https://github.com/ayberk))
* Bring Go to 1.15.6 in Travis ([#648](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/648), [@gliptak](https://github.com/gliptak))
* Fix overlays not being updated for gcr migration ([#649](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/649), [@wongma7](https://github.com/wongma7))
* Arm overlay ([#653](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/653), [@ayberk](https://github.com/ayberk))
* Use buildx in cloudbuild ([#658](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/658), [@wongma7](https://github.com/wongma7))
* (Try to) fix cloudbuild ([#659](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/659), [@wongma7](https://github.com/wongma7))
* Fix stray argument in cloudbuild.yaml ([#661](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/661), [@wongma7](https://github.com/wongma7))
* Add note for gp3 on outposts ([#665](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/665), [@ayberk](https://github.com/ayberk))
* Call hack/prow.sh from cloudbuild ([#666](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/666), [@wongma7](https://github.com/wongma7))
* cloudbuild: Set _STAGING_PROJECT ([#668](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/668), [@wongma7](https://github.com/wongma7))
* add import snapshot e2e test ([#678](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/678), [@AndyXiangLi](https://github.com/AndyXiangLi))
* Prefix helm chart releases with "helm-chart-" ([#682](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/682), [@wongma7](https://github.com/wongma7))
* Release 0.8.1 ([#683](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/683), [@wongma7](https://github.com/wongma7))
* Push debian target to Docker Hub ([#686](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/686), [@wongma7](https://github.com/wongma7))
* Adds patch for ebs-csi-controller-sa to volumeattachments/status ([#690](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/690), [@cuppett](https://github.com/cuppett))
* Add a prerequisite to dynamic provisioning ([#691](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/691), [@ronenl1](https://github.com/ronenl1))
* Refactor e2e testing scripts to be more reusable and use them instead… ([#694](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/694), [@wongma7](https://github.com/wongma7))
* Update to golang@1.15.6 ([#699](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/699), [@ialidzhikov](https://github.com/ialidzhikov))
* add e2e test for volume resizing ([#705](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/705), [@AndyXiangLi](https://github.com/AndyXiangLi))

# v0.8.1

## Notable changes
- Images in k8s.gcr.io are multiarch.

### Bug fixes
* release-0.8: Use buildx in cloudbuild ([#670](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/670), [@wongma7](https://github.com/wongma7))

# v0.8.0

## Notable changes
- gp3 is now the default volume type. gp3 is **not** supported on outposts. Outpost customers need to use a different type for their volumes.
- Images will be built on a Debian base by default. Images built on Amazon Linux will still be available but with the tag suffix `-amazonlinux`.
- Images will be published to k8s.gcr.io in addition to ECR and Docker Hub.

### New features
* Chart option to disable default toleration of all taints ([#526](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/526), [@risinger](https://github.com/risinger))
* Apply extra volume tags to EBS snapshots ([#568](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/568), [@chrishenzie](https://github.com/chrishenzie))
* [helm] add tag options and update csi-provisioner ([#577](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/577), [@kcking](https://github.com/kcking))
* vendor: bump aws sdk for AssumeRoleWithWebIdentity support ([#614](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/614), [@abhinavdahiya](https://github.com/abhinavdahiya))
* Add EBS gp3 support ([#633](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/633), [@samjo-nyang](https://github.com/samjo-nyang))
* Apply resource constraints to all sidecar containers ([#640](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/640), [@tirumerla](https://github.com/tirumerla))

### Bug fixes
* Fix the name of the snapshot controller leader election RoleBinding ([#601](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/601), [@robbie-demuth](https://github.com/robbie-demuth))

### Misc.
* Post-release v0.7.0 ([#576](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/576), [@ayberk](https://github.com/ayberk))
* Fixing Helm install command ([#578](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/578), [@danil-smirnov](https://github.com/danil-smirnov))
* Fix markdown issue in README.md ([#579](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/579), [@ialidzhikov](https://github.com/ialidzhikov))
* Document behavior wrt minimum and maximum iops ([#582](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/582), [@wongma7](https://github.com/wongma7))
* Set CSIMigrationAWSComplete for migration tests ([#593](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/593), [@wongma7](https://github.com/wongma7))
* Bump migration kops and k8s version ([#602](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/602), [@wongma7](https://github.com/wongma7))
* Update hack/run-e2e-test to be more idempotent and pleasant to use ([#616](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/616), [@wongma7](https://github.com/wongma7))
* Post-release v0.7.1 ([#619](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/619), [@ayberk](https://github.com/ayberk))
* Move chart to charts directory and add workflow to publish new chart versions ([#624](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/624), [@krmichel](https://github.com/krmichel))
* docs(readme): update link to developer documentation ([#629](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/629), [@BondAnthony](https://github.com/BondAnthony))
* Update ecr overlay image tag ([#630](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/630), [@ayberk](https://github.com/ayberk))
* Add cloudbuild.yaml for image pushing to gcr ([#632](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/632), [@wongma7](https://github.com/wongma7))
* Add latest tags to cloudbuild ([#634](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/634), [@wongma7](https://github.com/wongma7))
* Fix target name in cloudbuild.yaml from amazon to amazonlinux ([#636](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/636), [@wongma7](https://github.com/wongma7))
* Suffix amazonlinux image with -amazonlinux and push debian image to GitHub ([#639](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/639), [@wongma7](https://github.com/wongma7))
* Set up QEMU to build for arm64 ([#641](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/641), [@wongma7](https://github.com/wongma7))

# v0.7.1
[Documentation](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/blob/v0.7.1/docs/README.md)

filename  | sha512 hash
--------- | ------------
[v0.7.1.zip](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/archive/v0.7.1.zip) | `0c8b1e539f5852e54b5f4ab48cb3054ac52145db3d692cdc6b3ac683c39ebf11951c5ff3823a83666605a56a30b38953d20f392397c16bf39a5727c66ddf0827`
[v0.7.1.tar.gz](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/archive/v0.7.1.tar.gz) | `157ed2c7aa00635a61438a1574bd7e124676bcabd9e27cfe865c7bbb3194609894536b1eb38a12a8e5bfa71b540e0f1cde12000b02d90b390d17987fc913042e`

## Notable changes
This release includes a fix for the helm chart to point to the correct image.

# v0.7.0
[Documentation](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/blob/v0.7.0/docs/README.md)

filename  | sha512 hash
--------- | ------------
[v0.7.0.zip](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/archive/v0.7.0.zip) | `6e1117ce046d0030c3008b3eec8ba3196c516adf0ecef8909fcfd3d68e63624a73a992033356e208bf0d5563f7dec2e40675f0fee7f322bd4f69d7b03750961a`
[v0.7.0.tar.gz](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/archive/v0.7.0.tar.gz) | `4dc3402ffa3dcc59c9af1f7d776a3f53a288f62a31c05cde00aeceeef6000be16ca6cdae08712b4f7f64c9e89ceeaa13df7f1ca4bf3d62ba62845b52cc13eadf`

## Notable changes
### New features
* Add arm support ([#527](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/527), [@leakingtapan](https://github.com/leakingtapan))
* Add EBS IO2 support ([#558](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/558), [@ayberk](https://github.com/ayberk))
* Create volumes in outpost for outpost instances ([#561](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/561), [@ayberk](https://github.com/ayberk))

### Improvements
* Make EBS controllerexpansion idempotent ([#552](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/552), [@gnufied](https://github.com/gnufied))
* Add overlay for ECR images ([#570](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/570), [@ayberk](https://github.com/ayberk))

# v0.6.0
[Documentation](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/blob/v0.6.0/docs/README.md)

filename  | sha512 hash
--------- | ------------
[v0.6.0.zip](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/archive/v0.6.0.zip) | `67dc79703c2d022cbc53a370e8ac7279bf4345030a3ecc5b2bdff2b722ec807b712f2cd6eae79598edb87e15d92e683e98dde7c25e52f705233bc3ece649c693`
[v0.6.0.tar.gz](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/archive/v0.6.0.tar.gz) | `a3b5e95ec05ce6b4e6eb22ae00c7898cb876f21719354636dae5d323934c7a0bb32a7a8e89abdfcc6b0a0827c7169a349cba9dce32b7bf25e7287a2ec0387f21`

## Notable changes
### New features
* Allow volume attach limit overwrite via command line parameter ([#522](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/522), [@rfranzke](https://github.com/rfranzke))
* Add tags that the in-tree volume plugin uses ([#530](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/530), [@jsafrane](https://github.com/jsafrane))

### Bug fixes
* Adding amd64 as nodeSelector to avoid arm64 archtectures (#471) ([#472](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/472), [@hugoprudente](https://github.com/hugoprudente))
* Update stable overlay to 0.5.0 ([#495](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/495), [@wongma7](https://github.com/wongma7))

### Improvements
* Update aws-sdk to v1.29.11 to get IMDSv2 support ([#463](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/463), [@msau42](https://github.com/msau42))
* Fix e2e test ([#468](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/468), [@leakingtapan](https://github.com/leakingtapan))
* Generate deployment manifests from helm chart ([#475](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/475), [@krmichel](https://github.com/krmichel))
* Correct golint warning ([#478](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/478), [@gliptak](https://github.com/gliptak))
* Bump Go to 1.14.1 ([#479](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/479), [@gliptak](https://github.com/gliptak))
* Add mount unittest ([#481](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/481), [@gliptak](https://github.com/gliptak))
* Remove volume IOPS limit ([#483](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/483), [@jacobmarble](https://github.com/jacobmarble))
* Additional mount unittest ([#484](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/484), [@gliptak](https://github.com/gliptak))
* docs/README: add missing "--namespace" flag to "helm" command ([#486](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/486), [@gyuho](https://github.com/gyuho))
* Add nodeAffinity to avoid Fargate worker nodes ([#488](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/488), [@bgsilvait](https://github.com/bgsilvait))
* remove deprecated "beta.kubernetes.io/os" nodeSelector ([#489](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/489), [@gyuho](https://github.com/gyuho))
* Update kubernetes-csi/external-snapshotter components to v2.1.1 ([#490](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/490), [@ialidzhikov](https://github.com/ialidzhikov))
* Improve csi-snapshotter ClusterRole ([#491](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/491), [@ialidzhikov](https://github.com/ialidzhikov))
* Fix migration test ([#500](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/500), [@leakingtapan](https://github.com/leakingtapan))
* Add missing IAM permissions ([#501](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/501), [@robbiet480](https://github.com/robbiet480))
* Fixed resizing docs to refer the right path to example spec ([#504](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/504), [@amuraru](https://github.com/amuraru))
* optimization: cache go mod during docker build ([#513](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/513), [@leakingtapan](https://github.com/leakingtapan))

# v0.5.0
[Documentation](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/blob/v0.5.0/docs/README.md)

filename  | sha512 hash
--------- | ------------
[v0.5.0.zip](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/archive/v0.5.0.zip) | `c53327e090352a7f79ee642dbf8c211733f4a2cb78968ec688a1eade55151e65f1f97cd228d22168317439f1db9f3d2f07dcaa2873f44732ad23aaf632cbef3a`
[v0.5.0.tar.gz](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/archive/v0.5.0.tar.gz) | `ec4963d34c601cdf718838d90b8aa6f36b16c9ac127743e73fbe76118a606d41aced116aaaab73370c17bcc536945d5ccd735bc5a4a00f523025c8e41ddedcb8`

## Notable changes
### New features
* Add a cmdline option to add extra volume tags ([#353](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/353), [@jieyu](https://github.com/jieyu))
* Switch to use kustomize for manifest ([#360](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/360), [@leakingtapan](https://github.com/leakingtapan))
* enable users to set ec2-endpoint for nonstandard regions ([#369](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/369), [@amdonov](https://github.com/amdonov))
* Add standard volume type ([#379](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/379), [@leakingtapan](https://github.com/leakingtapan))
* Update aws sdk version to enable EKS IAM for SA ([#386](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/386), [@leakingtapan](https://github.com/leakingtapan))
* Implement different driver modes and AWS Region override for controller service ([#438](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/438), [@rfranzke](https://github.com/rfranzke))
* Add manifest files for snapshotter 2.0 ([#452](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/452), [@leakingtapan](https://github.com/leakingtapan))

### Bug fixes
* Return success if instance or volume are not found ([#375](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/375), [@bertinatto](https://github.com/bertinatto))
* Patch k8scsi sidecars CVE-2019-11255 ([#413](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/413), [@jnaulty](https://github.com/jnaulty))
* Handle mount flags in NodeStageVolume ([#430](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/430), [@bertinatto](https://github.com/bertinatto))

### Improvements
* Run upstream e2e test suites with migration  ([#341](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/341), [@wongma7](https://github.com/wongma7))
* Use new test framework for test orchestration ([#359](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/359), [@leakingtapan](https://github.com/leakingtapan))
* Update to use 1.16 cluster with inline test enabled ([#362](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/362), [@leakingtapan](https://github.com/leakingtapan))
* Enable leader election ([#380](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/380), [@leakingtapan](https://github.com/leakingtapan))
* Update go mod and mount library ([#388](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/388), [@leakingtapan](https://github.com/leakingtapan))
* Refactor NewCloud by pass in region ([#394](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/394), [@leakingtapan](https://github.com/leakingtapan))
* helm: provide an option to set extra volume tags ([#396](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/396), [@jieyu](https://github.com/jieyu))
* Allow override for csi-provisioner image ([#401](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/401), [@gliptak](https://github.com/gliptak))
* Enable volume expansion e2e test for CSI migration ([#407](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/407), [@leakingtapan](https://github.com/leakingtapan))
* Swith to use kops 1.16 ([#409](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/409), [@leakingtapan](https://github.com/leakingtapan))
* Added tolerations for node support ([#420](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/420), [@zerkms](https://github.com/zerkms))
* Update helm chart to better match available values and add the ability to add annotations ([#423](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/423), [@krmichel](https://github.com/krmichel))
* [helm] Also add toleration support to controller ([#433](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/433), [@jyaworski](https://github.com/jyaworski))
* Add ec2:ModifyVolume action ([#434](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/434), [@zodiac12k](https://github.com/zodiac12k))
* Schedule the EBS CSI DaemonSet on all nodes by default ([#441](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/441), [@pcfens](https://github.com/pcfens))

# v0.4.0
[Documentation](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/blob/v0.4.0/docs/README.md)

filename  | sha512 hash
--------- | ------------
[v0.4.0.zip](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/archive/v0.4.0.zip) | `2f46b54211178ad1e55926284b9f6218be874038a1a62ef364809a5d2c37b7bbbe58a2cc4991b9cf44cbfe4966c61dd6c16df0790627dffac4f7df9ffc084a0c`
[v0.4.0.tar.gz](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/archive/v0.4.0.tar.gz) | `0199df52ac1e19ee6b04efb80439024dde11de3d8fc292ce10527f2e658b393d8bfd4e37a6ec321cb415c9bdbee83ff5dbdf58e2336d03fe5d1b2717ccb11169`

## Action Required
* Update Kubernetes cluster to 1.14+ before installing the driver, since the released driver manifest assumes 1.14+ cluster.
* storageclass parameter's `fstype` key is deprecated in favor of `csi.storage.k8s.io/fstype` key. Please update the key in you stroage parameters.

## Changes since v0.3.0
See [details](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/compare/v0.3.0...v0.4.0) for all the changes.

### Notable changes
* Make secret optional ([#247](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/247), [@leakingtapan](https://github.com/leakingtapan/))
* Add support for XFS filesystem ([#253](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/253), [@leakingtapan](https://github.com/leakingtapan/))
* Upgrade CSI spec to 1.1.0 ([#263](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/263), [@leakingtapan](https://github.com/leakingtapan/))
* Refactor controller unit test with proper mock ([#269](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/269), [@zacharya](https://github.com/zacharya/))
* Refactor device path allocator ([#274](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/274), [@leakingtapan](https://github.com/leakingtapan/))
* Implementing ListSnapshots ([#286](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/286), [@zacharya](https://github.com/zacharya/))
* Add max number of volumes that can be attached to an instance ([#289](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/289), [@bertinatto](https://github.com/bertinatto/))
* Add helm chart ([#303](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/303), [@leakingtapan](https://github.com/leakingtapan/))
* Add volume expansion ([#271](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/271), [@bertinatto](https://github.com/bertinatto/))
* Remove cluster-driver-registrar ([#322](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/322), [@jsafrane](https://github.com/jsafrane/))
* Upgrade to golang 1.12 ([#329](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/329), [@leakingtapan](https://github.com/leakingtapan/))
* Fix bugs by passing fstype correctly ([#335](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/335), [@leakingtapan](https://github.com/leakingtapan/))
* Output junit to ARTIFACTS for testgrid ([#340](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/340), [@wongma7](https://github.com/wongma7/))

# v0.3.0
[Documentation](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/blob/v0.3.0/docs/README.md)

filename  | sha512 hash
--------- | ------------
[v0.3.0.zip](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/archive/v0.3.0.zip) | `27a7a1cd4fc7a8afa1c0dd8fb3ce4cb1d9fc7439ebdbeba7ac0bfb0df723acb654a92f88270bc68ab4dd6c8943febf779efa8cbebdf3ea2ada145ff7ce426870`
[v0.3.0.tar.gz](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/archive/v0.3.0.tar.gz) | `9126a3493f958aaa4727bc62b1a5c545ac8795f08844a605541aac3d38dea8769cee12c7db94f44179a91af7e8702174bba2533b4e30eb3f32f9b8338101a5db`

## Action Required
* None

## Upgrade Driver
Driver upgrade should be performed one version at a time by using following steps:
1. Delete the old driver controller service and node service along with other resources including cluster roles, cluster role bindings and service accounts.
1. Deploy the new driver controller service and node service along with other resources including cluster roles, cluster role bindings and service accounts.

## Changes since v0.2.0
See [details](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/compare/v0.2.0...v0.3.0) for all the changes.

### Notable changes
* Strip symbol for production build ([#201](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/201), [@leakingtapan](https://github.com/leakingtapan/))
* Remove vendor directory ([#198](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/198), [@leakingtapan](https://github.com/leakingtapan/))
* Use same mount to place in the csi.sock, remove obsolete volumes ([#212](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/212), [@frittentheke](https://github.com/frittentheke/))
* Add snapshot support ([#131](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/131), [@tsmetana](https://github.com/tsmetana/))
* Add snapshot examples ([#210](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/210), [@tsmetana](https://github.com/tsmetana/))
* Implement raw block volume support ([#215](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/215), [@leakingtapan](https://github.com/leakingtapan/))
* Add unit tests for ControllerPublish and ControllerUnpublish requests ([#219](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/219), [@sreis](https://github.com/sreis/))
* New block volume e2e tests ([#226](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/226), [@dkoshkin](https://github.com/dkoshkin/))
* Implement device path discovery for NVMe support ([#231](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/231), [@leakingtapan](https://github.com/leakingtapan/))
* Cleanup README and examples ([@232](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/232), [@dkoshkin](https://github.com/dkoshkin/))
* New volume snapshot e2e tests ([#235](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/235), [@dkoshkin](https://github.com/dkoshkin/))

# v0.2.0
[Documentation](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/blob/v0.2.0/docs/README.md)

filename  | sha512 hash
--------- | ------------
[v0.2.0.zip](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/archive/v0.2.0.zip) | `a9733881c43dfb788f6c657320b6b4acdd8ee9726649c850282f8a7f15f816a6aa5db187a5d415781a76918a30ac227c03a81b662027c5b192ab57a050bf28ee`
[v0.2.0.tar.gz](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/archive/v0.2.0.tar.gz) | `0d7a3efd0c1b0c6bf01b08c3cbd48d867aeab1cf1f7f12274f42d561f64526c0345f23d5947ddada7a333046f101679eea620c9ab8985f9d4d1c8c3f28de49ce`

## Action Required
* Upgrade the Kubernetes cluster to 1.13+ before deploying the driver. Since CSI 1.0 is only supported starting from Kubernetes 1.13.

## Upgrade Driver
Driver upgrade should be performed one version at a time by using following steps:
1. Delete the old driver controller service and node service along with other resources including cluster roles, cluster role bindings and service accounts.
1. Deploy the new driver controller service and node service along with other resources including cluster roles, cluster role bindings and service accounts.

## Changes since v0.1.0
See [details](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/compare/v0.1.0...v0.2.0) for all the changes.

### Notable changes
* Update to CSI 1.0 ([#122](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/122), [@bertinatto](https://github.com/bertinatto/))
* Add mountOptions support ([#130](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/130), [@bertinatto](https://github.com/bertinatto/))
* Resolve memory addresses in log messages ([#132](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/132), [@bertinatto](https://github.com/bertinatto/))
* Add version flag ([#136](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/136), [@dkoshkin](https://github.com/dkoshkin/))
* Wait for volume to become available ([#126](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/126), [@bertinatto](https://github.com/bertinatto/))
* Add first few e2e test cases #151 ([#151](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/151/commits), [@dkoshkin](https://github.com/dkoshkin/))
* Make test-integration uses aws-k8s-tester ([#153](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/153), [@kschumy](https://github.com/kschumy))
* Rename VolumeNameTagKey ([#161](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/issues/161), [@leakingtapan](https://github.com/leakingtapan/))
* CSI image version and deployment manifests updates  ([#171](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/171), [@dkoshkin](https://github.com/dkoshkin/))
* Update driver manifest files ([#181](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/181), [@leakingtapan](https://github.com/leakingtapan/))
* More e2e tests ([#173](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/173), [@dkoshkin](https://github.com/dkoshkin/))
* Update run-e2e-test script to setup cluster ([#186](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/186), [@leakingtapan](https://github.com/leakingtapan/))
* Check if target path is mounted before unmounting ([#183](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/183), [@sreis](https://github.com/sreis/))

# v0.1.0
[Documentation](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/blob/v0.1.0/docs/README.md)

## Downloads for v0.1.0

filename  | sha512 hash
--------- | ------------
[v0.1.0.zip](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/archive/v0.1.0.zip) | `03841418496e292c3f91cee7942b545395bce049e9c4d2305532545fb82ad2e5189866afec2ed937924e144142b0b915a9467bac42e9f2b881181aba6aa80a68`
[v0.1.0.tar.gz](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/archive/v0.1.0.tar.gz) | `106b6c2011acd42b0f10117b7f104ab188dde798711e98119137cf3d8265e381df09595b8e861c0c9fdcf8772f4a711e338e822602e98bfd68f54f9e1c7f8f16`

## Changelog since initial commit

### Notable changes
* Update driver name and topology key ([#105](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/105), [@leakingtapan](https://github.com/leakingtapan/))
* Add support for creating encrypted volume and unit test ([#80](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/80), [@leakingtapan](https://github.com/leakingtapan/))
* Implement support for storage class parameter - volume type ([#73](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/73), [@leakingtapan](https://github.com/leakingtapan/))
* Implement support for storage class parameter - fsType ([#67](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/67), [@leakingtapan](https://github.com/leakingtapan/))
* Add missing capability and clusterrole permission to enable tology awareness scheduling ([#61](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/commit/2873e0b), [@leakingtapan](https://github.com/leakingtapan/))
* Wait for correct attachment state ([#58](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/58), [@bertinatto](https://github.com/bertinatto/))
* Implement topology awareness support for dynamic provisioning ([#42](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/42), [@leakingtapan](https://github.com/leakingtapan/))
* Wait for volume status in e2e test ([#34](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/34), [@bertinatto](https://github.com/bertinatto/))
* Update cloud provider interface to take in context ([#45](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/pull/45), [@leakingtapan](https://github.com/leakingtapan/))
* Initial driver implementation ([9ba4c5d](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/commit/9ba4c5d), [@bertinatto](https://github.com/bertinatto/))
