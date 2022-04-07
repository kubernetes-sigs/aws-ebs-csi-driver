# Tagging

The AWS EBS CSI Driver supports tagging through `StorageClass.parameters` (in v1.6.0 and later). 

If a key has the prefix `tagSpecification`, the CSI driver will treat the value as a key-value pair to be applied to the dynamically provisioned volume as tags.


**Example 1**
```
kind: StorageClass
apiVersion: storage.k8s.io/v1
metadata:
  name: ebs-sc
provisioner: ebs.csi.aws.com
parameters:
  tagSpecification_1: "key1=value1"
  tagSpecification_2: "key2=hello world"
  tagSpecification_3: "key3="
```

Provisioning a volume using this StorageClass will apply two tags:

```
key1=value1
key2=hello world
key3=<empty string>
```

________

To allow for PV-level granularity, the CSI driver support runtime string interpolation on the tag values. You can specify placeholders for PVC namespace, PVC name and PV name, which will then be dynamically computed at runtime.

**NOTE: This requires the `--extra-create-metadata` flag to be enabled on the `external-provisioner` sidecar.**

**Example 2**
```
kind: StorageClass
apiVersion: storage.k8s.io/v1
metadata:
  name: ebs-sc
provisioner: ebs.csi.aws.com
parameters:
  tagSpecification_1: "pvcnamespace={{ .PVCNamespace }}"
  tagSpecification_2: "pvcname={{ .PVCName }}"
  tagSpecification_3: "pvname={{ .PVName }}"
```
Provisioning a volume using this StorageClass, with a PVC named 'ebs-claim' in namespace 'default', will apply the following tags:

```
pvcnamespace=default
pvcname=ebs-claim
pvname=<the computed pv name>
```


_________

The driver uses Go's `text/template` package for string interpolation. As such, cluster admins are free to use the constructs provided by the package (except for certain function, see `Failure Modes` below). To aid cluster admins to be more expressive, certain functions have been provided.

They include:

-   **field** delim index str: Split `str` by `delim` and extract the  word at position `index`.
-   **substring** start end str: Get a substring of `str` given the `start` and `end` indices
-   **toUpper** str: Convert `str` to uppercase
-   **toLower** str: Convert `str` to lowercase
-   **contains** str1 str2: Returns a boolean if `str2` contains `str1`


**Example 3**
```
kind: StorageClass
apiVersion: storage.k8s.io/v1
metadata:
  name: ebs-sc
provisioner: ebs.csi.aws.com
parameters:
  tagSpecification_1: 'backup={{ .PVCNamespace | contains "prod" }}'
  tagSpecification_2: 'billingID={{ .PVCNamespace | field "-" 2 | toUpper }}'
```

Assuming the PVC namespace is `ns-prod-abcdef`, the attached tags will be

```
backup=true
billingID=ABCDEF
```

____

## Failure Modes

There can be multipe failure modes:

* The template cannot be parsed.
* The key/interpolated value do not meet the [AWS Tag Requirements](https://docs.aws.amazon.com/general/latest/gr/aws_tagging.html)
* The key is not allowed (such as keys used internally by the CSI driver e.g., 'CSIVolumeName').
* The template uses one of the disabled function calls. The driver disables the following `text/template` functions: `js`, `call`, `html`, `urlquery`. 

In this case, the CSI driver will not provision a volume, but instead return an error.

The driver also defines another flag, `--warn-on-invalid-tag` that will (if set), instead of returning an error, log a warning and skip the offending tag.


