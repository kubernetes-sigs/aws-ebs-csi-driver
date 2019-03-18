# v0.3.0
[Documentation](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/blob/master/docs/README.md)

## Action Required
* None

## Upgrade Driver
Driver upgrade should be performed one version at a time by using following steps:
1. Delete the old driver controller service and node service along with other resources including cluster roles, cluster role bindings and service accounts.
1. Deploy the new driver controller service and node service along with other resources including cluster roles, cluster role bindings and service accounts.

## Changes since v0.2.0
See [details](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/compare/v0.2.0...master) for all the changes.

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
