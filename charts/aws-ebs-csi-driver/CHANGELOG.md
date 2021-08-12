# Helm chart

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
