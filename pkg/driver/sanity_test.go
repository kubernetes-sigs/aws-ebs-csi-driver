package driver

import (
	"context"
	"fmt"
	"io/ioutil"
	"math/rand"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/kubernetes-csi/csi-test/pkg/sanity"
	"github.com/kubernetes-sigs/aws-ebs-csi-driver/pkg/cloud"
	"github.com/kubernetes-sigs/aws-ebs-csi-driver/pkg/driver/internal"
	"github.com/kubernetes-sigs/aws-ebs-csi-driver/pkg/util"
	"k8s.io/mount-utils"
	"k8s.io/utils/exec"
)

func TestSanity(t *testing.T) {
	// Setup the full driver and its environment
	dir, err := ioutil.TempDir("", "sanity-ebs-csi")
	if err != nil {
		t.Fatalf("error creating directory %v", err)
	}
	defer os.RemoveAll(dir)

	targetPath := filepath.Join(dir, "mount")
	stagingPath := filepath.Join(dir, "staging")
	endpoint := "unix://" + filepath.Join(dir, "csi.sock")

	config := &sanity.Config{
		TargetPath:       targetPath,
		StagingPath:      stagingPath,
		Address:          endpoint,
		CreateTargetDir:  createDir,
		CreateStagingDir: createDir,
	}

	driverOptions := &DriverOptions{
		endpoint: endpoint,
		mode:     AllMode,
	}

	drv := &Driver{
		options: driverOptions,
		controllerService: controllerService{
			cloud:         newFakeCloudProvider(),
			inFlight:      internal.NewInFlight(),
			driverOptions: driverOptions,
		},
		nodeService: nodeService{
			metadata: &cloud.Metadata{
				InstanceID:       "instanceID",
				Region:           "region",
				AvailabilityZone: "az",
			},
			mounter:          newFakeMounter(),
			deviceIdentifier: newNodeDeviceIdentifier(),
			inFlight:         internal.NewInFlight(),
			driverOptions:    &DriverOptions{},
		},
	}
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("recover: %v", r)
		}
	}()
	go func() {
		if err := drv.Run(); err != nil {
			panic(fmt.Sprintf("%v", err))
		}
	}()

	// Now call the test suite
	sanity.Test(t, config)
}

func createDir(targetPath string) (string, error) {
	if err := os.MkdirAll(targetPath, 0300); err != nil {
		if os.IsNotExist(err) {
			return "", err
		}
	}
	return targetPath, nil
}

type fakeCloudProvider struct {
	disks map[string]*fakeDisk
	// snapshots contains mapping from snapshot ID to snapshot
	snapshots map[string]*fakeSnapshot
	pub       map[string]string
	tokens    map[string]int64
}

type fakeDisk struct {
	*cloud.Disk
	tags map[string]string
}

type fakeSnapshot struct {
	*cloud.Snapshot
	tags map[string]string
}

func newFakeCloudProvider() *fakeCloudProvider {
	return &fakeCloudProvider{
		disks:     make(map[string]*fakeDisk),
		snapshots: make(map[string]*fakeSnapshot),
		pub:       make(map[string]string),
		tokens:    make(map[string]int64),
	}
}

func (c *fakeCloudProvider) CreateDisk(ctx context.Context, volumeName string, diskOptions *cloud.DiskOptions) (*cloud.Disk, error) {
	r1 := rand.New(rand.NewSource(time.Now().UnixNano()))
	if len(diskOptions.SnapshotID) > 0 {
		if _, ok := c.snapshots[diskOptions.SnapshotID]; !ok {
			return nil, cloud.ErrNotFound
		}
	}
	if existingDisk, ok := c.disks[volumeName]; ok {
		//Already Created volume
		if existingDisk.Disk.CapacityGiB != util.BytesToGiB(diskOptions.CapacityBytes) {
			return nil, cloud.ErrIdempotentParameterMismatch
		} else {
			return existingDisk.Disk, nil
		}
	}
	d := &fakeDisk{
		Disk: &cloud.Disk{
			VolumeID:         fmt.Sprintf("vol-%d", r1.Uint64()),
			CapacityGiB:      util.BytesToGiB(diskOptions.CapacityBytes),
			AvailabilityZone: diskOptions.AvailabilityZone,
			SnapshotID:       diskOptions.SnapshotID,
		},
		tags: diskOptions.Tags,
	}
	c.disks[volumeName] = d
	return d.Disk, nil
}

func (c *fakeCloudProvider) DeleteDisk(ctx context.Context, volumeID string) (bool, error) {
	for volName, f := range c.disks {
		if f.Disk.VolumeID == volumeID {
			delete(c.disks, volName)
		}
	}
	return true, nil
}

func (c *fakeCloudProvider) AttachDisk(ctx context.Context, volumeID, nodeID string) (string, error) {
	if _, ok := c.pub[volumeID]; ok {
		return "", cloud.ErrAlreadyExists
	}
	c.pub[volumeID] = nodeID
	return "/tmp", nil
}

func (c *fakeCloudProvider) DetachDisk(ctx context.Context, volumeID, nodeID string) error {
	return nil
}

func (c *fakeCloudProvider) WaitForAttachmentState(ctx context.Context, volumeID, expectedState string, expectedInstance string, expectedDevice string, alreadyAssigned bool) (*ec2.VolumeAttachment, error) {
	return nil, nil
}

func (c *fakeCloudProvider) GetDiskByName(ctx context.Context, name string, capacityBytes int64) (*cloud.Disk, error) {
	var disks []*fakeDisk
	for _, d := range c.disks {
		for key, value := range d.tags {
			if key == cloud.VolumeNameTagKey && value == name {
				disks = append(disks, d)
			}
		}
	}
	if len(disks) > 1 {
		return nil, cloud.ErrMultiDisks
	} else if len(disks) == 1 {
		if capacityBytes != disks[0].Disk.CapacityGiB*util.GiB {
			return nil, cloud.ErrDiskExistsDiffSize
		}
		return disks[0].Disk, nil
	}
	return nil, nil
}

func (c *fakeCloudProvider) GetDiskByID(ctx context.Context, volumeID string) (*cloud.Disk, error) {
	for _, f := range c.disks {
		if f.Disk.VolumeID == volumeID {
			return f.Disk, nil
		}
	}
	return nil, cloud.ErrNotFound
}

func (c *fakeCloudProvider) IsExistInstance(ctx context.Context, nodeID string) bool {
	return nodeID == "instanceID"
}

func (c *fakeCloudProvider) CreateSnapshot(ctx context.Context, volumeID string, snapshotOptions *cloud.SnapshotOptions) (snapshot *cloud.Snapshot, err error) {
	r1 := rand.New(rand.NewSource(time.Now().UnixNano()))
	snapshotID := fmt.Sprintf("snapshot-%d", r1.Uint64())

	for _, existingSnapshot := range c.snapshots {
		if existingSnapshot.Snapshot.SnapshotID == snapshotID && existingSnapshot.Snapshot.SourceVolumeID == volumeID {
			return nil, cloud.ErrAlreadyExists
		}
	}

	s := &fakeSnapshot{
		Snapshot: &cloud.Snapshot{
			SnapshotID:     snapshotID,
			SourceVolumeID: volumeID,
			Size:           1,
			CreationTime:   time.Now(),
		},
		tags: snapshotOptions.Tags,
	}
	c.snapshots[snapshotID] = s
	return s.Snapshot, nil

}

func (c *fakeCloudProvider) DeleteSnapshot(ctx context.Context, snapshotID string) (success bool, err error) {
	delete(c.snapshots, snapshotID)
	return true, nil

}

func (c *fakeCloudProvider) GetSnapshotByName(ctx context.Context, name string) (snapshot *cloud.Snapshot, err error) {
	var snapshots []*fakeSnapshot
	for _, s := range c.snapshots {
		snapshotName, exists := s.tags[cloud.SnapshotNameTagKey]
		if !exists {
			continue
		}
		if snapshotName == name {
			snapshots = append(snapshots, s)
		}
	}
	if len(snapshots) == 0 {
		return nil, cloud.ErrNotFound
	}

	return snapshots[0].Snapshot, nil
}

func (c *fakeCloudProvider) GetSnapshotByID(ctx context.Context, snapshotID string) (snapshot *cloud.Snapshot, err error) {
	ret, exists := c.snapshots[snapshotID]
	if !exists {
		return nil, cloud.ErrNotFound
	}

	return ret.Snapshot, nil
}

func (c *fakeCloudProvider) ListSnapshots(ctx context.Context, volumeID string, maxResults int64, nextToken string) (listSnapshotsResponse *cloud.ListSnapshotsResponse, err error) {
	var snapshots []*cloud.Snapshot
	var retToken string
	for _, fakeSnapshot := range c.snapshots {
		if fakeSnapshot.Snapshot.SourceVolumeID == volumeID || len(volumeID) == 0 {
			snapshots = append(snapshots, fakeSnapshot.Snapshot)
		}
	}
	if maxResults > 0 {
		r1 := rand.New(rand.NewSource(time.Now().UnixNano()))
		retToken = fmt.Sprintf("token-%d", r1.Uint64())
		c.tokens[retToken] = maxResults
		snapshots = snapshots[0:maxResults]
		fmt.Printf("%v\n", snapshots)
	}
	if len(nextToken) != 0 {
		snapshots = snapshots[c.tokens[nextToken]:]
	}
	return &cloud.ListSnapshotsResponse{
		Snapshots: snapshots,
		NextToken: retToken,
	}, nil

}

func (c *fakeCloudProvider) ResizeDisk(ctx context.Context, volumeID string, newSize int64) (int64, error) {
	for volName, f := range c.disks {
		if f.Disk.VolumeID == volumeID {
			c.disks[volName].CapacityGiB = newSize
			return newSize, nil
		}
	}
	return 0, cloud.ErrNotFound
}

type fakeMounter struct {
	exec.Interface
}

func newFakeMounter() *fakeMounter {
	return &fakeMounter{
		exec.New(),
	}
}

func (f *fakeMounter) IsCorruptedMnt(err error) bool {
	return false
}

func (f *fakeMounter) Mount(source string, target string, fstype string, options []string) error {
	return nil
}

func (f *fakeMounter) MountSensitive(source string, target string, fstype string, options []string, sensitiveOptions []string) error {
	return nil
}

func (f *fakeMounter) MountSensitiveWithoutSystemd(source string, target string, fstype string, options []string, sensitiveOptions []string) error {
	return nil
}

func (f *fakeMounter) Unmount(target string) error {
	return nil
}

func (f *fakeMounter) List() ([]mount.MountPoint, error) {
	return []mount.MountPoint{}, nil
}

func (f *fakeMounter) IsLikelyNotMountPoint(file string) (bool, error) {
	return false, nil
}

func (f *fakeMounter) GetMountRefs(pathname string) ([]string, error) {
	return []string{}, nil
}

func (f *fakeMounter) FormatAndMount(source string, target string, fstype string, options []string) error {
	return nil
}

func (f *fakeMounter) GetDeviceNameFromMount(mountPath string) (string, int, error) {
	return "", 0, nil
}

func (f *fakeMounter) MakeFile(pathname string) error {
	file, err := os.OpenFile(pathname, os.O_CREATE, os.FileMode(0644))
	if err != nil {
		if !os.IsExist(err) {
			return err
		}
	}
	if err = file.Close(); err != nil {
		return err
	}
	return nil
}

func (f *fakeMounter) MakeDir(pathname string) error {
	err := os.MkdirAll(pathname, os.FileMode(0755))
	if err != nil {
		if !os.IsExist(err) {
			return err
		}
	}
	return nil
}

func (f *fakeMounter) PathExists(filename string) (bool, error) {
	if _, err := os.Stat(filename); os.IsNotExist(err) {
		return false, nil
	} else if err != nil {
		return false, err
	}
	return true, nil
}

func (f *fakeMounter) NeedResize(source string, path string) (bool, error) {
	return false, nil
}
