# EBS CSI Driver Plugins

The EBS CSI Driver supports build-time plugins to introduce custom functionality. Currently, these plugins can customize AWS API calls (such as by modifying parameters or using custom IAM configurations), introduce new Prometheus metrics, and modify the driver name from the default of `ebs.csi.aws.com`.

## Plugin Development

Developing a plugin is similar to developing features for the EBS CSI Driver. Place your plugin in the `pkg/plugin/` package. See `pkg/plugin/plugin_common.go` for the plugin interface all plugins are expected to follow. See `pkg/plugin/plugin.go.sample` for a sample plugin.

While developing, you can use `Makefile` targets to test your plugin, such as `make test` and `make cluster/image`. See [CONTRIBUTING.md](../CONTRIBUTING.md) for more details.

## Plugin Support

The EBS CSI Driver plugin system is supported, but bugs introduced by third-party plugins are not. For example, the driver not appropriately redirecting API calls to a plugin should be reported as, and will be triaged as a bug in the EBS CSI Driver itself. However, a bug in a plugin preventing volumes from being created should be reported to the plugin author.

**When reporting a bug, ensure the bug is reproducible without a plugin installed (unless the bug is with the plugin system itself).**
