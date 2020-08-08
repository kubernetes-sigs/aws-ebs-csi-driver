# v0.6.0
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
