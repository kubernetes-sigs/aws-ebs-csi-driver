package cloud

type FakeCloudProvider struct {
	disks map[string]*DiskOptions
}

func NewFakeCloudProvider() *FakeCloudProvider {
	return &FakeCloudProvider{
		disks: make(map[string]*DiskOptions),
	}
}

func (f *FakeCloudProvider) CreateDisk(diskOptions *DiskOptions) (volumeID VolumeID, err error) {
	//r1 := rand.New(rand.NewSource(time.Now().UnixNano()))
	//volID := fmt.Sprintf("vol-%d", r1.Uint64())
	//f.disks[volID] = diskOptions
	//return VolumeID(volID), nil
	return VolumeID("vol-test"), nil
}

func (f *FakeCloudProvider) DeleteDisk(volumeID VolumeID) (bool, error) {
	//volID := string(volumeID)
	//if _, ok := f.disks[volID]; !ok {
	//return false, status.Error(codes.NotFound, "")
	//}
	//delete(f.disks, volID)
	return true, nil
}
func (f *FakeCloudProvider) GetVolumesByTagName(tagKey, tagVal string) ([]string, error) {
	return nil, nil
}
