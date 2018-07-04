package aws

import (
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/types"
)

func GetFakeCloudProvider() *FakeCloudProvider {
	return &FakeCloudProvider{}
}

type FakeCloudProvider struct {
}

func (f *FakeCloudProvider) AttachDisk(diskName KubernetesVolumeID, nodeName types.NodeName) (string, error) {
	return "", nil
}

func (f *FakeCloudProvider) DetachDisk(diskName KubernetesVolumeID, nodeName types.NodeName) (string, error) {
	return "", nil
}

func (f *FakeCloudProvider) CreateDisk(volumeOptions *VolumeOptions) (volumeName KubernetesVolumeID, err error) {
	return "vol-test", nil
}

func (f *FakeCloudProvider) DeleteDisk(volumeName KubernetesVolumeID) (bool, error) {
	return false, nil
}

func (f *FakeCloudProvider) GetVolumeLabels(volumeName KubernetesVolumeID) (map[string]string, error) {
	return nil, nil
}

func (f *FakeCloudProvider) GetDiskPath(volumeName KubernetesVolumeID) (string, error) {
	return "", nil
}

func (f *FakeCloudProvider) DiskIsAttached(diskName KubernetesVolumeID, nodeName types.NodeName) (bool, error) {
	return false, nil
}

func (f *FakeCloudProvider) DisksAreAttached(map[types.NodeName][]KubernetesVolumeID) (map[types.NodeName]map[KubernetesVolumeID]bool, error) {
	return nil, nil
}

func (f *FakeCloudProvider) ResizeDisk(diskName KubernetesVolumeID, oldSize resource.Quantity, newSize resource.Quantity) (resource.Quantity, error) {
	return resource.Quantity{}, nil
}

func (f *FakeCloudProvider) GetVolumesByTagName(tagKey, tagVal string) ([]string, error) {
	return []string{}, nil
}
