//go:build windows
// +build windows

package resizefs

import (
	"github.com/kubernetes-sigs/aws-ebs-csi-driver/pkg/mounter"
	"k8s.io/klog"
)

// resizeFs provides support for resizing file systems
type resizeFs struct {
	proxy mounter.ProxyMounter
}

// NewResizeFs returns an instance of resizeFs
func NewResizeFs(p mounter.ProxyMounter) *resizeFs {
	return &resizeFs{proxy: p}
}

// Resize performs resize of file system
func (r *resizeFs) Resize(_, deviceMountPath string) (bool, error) {
	klog.V(3).Infof("Resize - Expanding mounted volume %s", deviceMountPath)
	return r.proxy.ResizeVolume(deviceMountPath)
}
