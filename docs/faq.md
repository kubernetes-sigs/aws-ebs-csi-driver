# Frequently Asked Questions

## CreateVolume (`StorageClass`) Parameters

### `ext4BigAlloc` and `ext4ClusterSize`

Warnings:
- Ext4's `bigalloc` is an experimental feature, under active development. Please pay particular attention to your node's kernel version. See the [ext4(5) man-page](https://man7.org/linux/man-pages/man5/ext4.5.html) for details.
- Linux kernel release 4.15 added support for resizing ext4 filesystems using clustered allocation. **Resizing volumes mounted to nodes running a Linux kernel version prior to 4.15 will fail.** 
