# v0.1.0
[Documentation](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/blob/v0.1.0/docs/README.md) 

## Downloads for v0.1.0

filename  | sha512 hash
--------- | ------------
[v0.1.0.zip](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/archive/v0.1.0.zip) | `2e74b202a96a1dc4a604d1e7cc86356706b21bbbe8090a48b476eedbd1c8c80a06bd068d40af24f9e85313eb226544b58c35a9da38a923c9a68ee3032ad3afc0`
[v0.1.0.tar.gz](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/archive/v0.1.0.tar.gz) | `5d8175b912403b069f5863cd5de29668d1c9f43e8e77234f2e306f7e65e30d79a93c7c8eb47838755586347df89e3f9490b951b962ec2342a544f850db1a9825`

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
