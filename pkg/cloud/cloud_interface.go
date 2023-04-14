package cloud

import (
	"context"

	"github.com/aws/aws-sdk-go/service/ec2"
)

type Cloud interface {
	CreateDisk(ctx context.Context, volumeName string, diskOptions *DiskOptions) (disk *Disk, err error)
	DeleteDisk(ctx context.Context, volumeID string) (success bool, err error)
	AttachDisk(ctx context.Context, volumeID string, nodeID string) (devicePath string, err error)
	DetachDisk(ctx context.Context, volumeID string, nodeID string) (err error)
	ResizeDisk(ctx context.Context, volumeID string, reqSize int64) (newSize int64, err error)
	WaitForAttachmentState(ctx context.Context, volumeID, expectedState string, expectedInstance string, expectedDevice string, alreadyAssigned bool) (*ec2.VolumeAttachment, error)
	GetDiskByName(ctx context.Context, name string, capacityBytes int64) (disk *Disk, err error)
	GetDiskByID(ctx context.Context, volumeID string) (disk *Disk, err error)
	IsExistInstance(ctx context.Context, nodeID string) (success bool)
	CreateSnapshot(ctx context.Context, volumeID string, snapshotOptions *SnapshotOptions) (snapshot *Snapshot, err error)
	DeleteSnapshot(ctx context.Context, snapshotID string) (success bool, err error)
	GetSnapshotByName(ctx context.Context, name string) (snapshot *Snapshot, err error)
	GetSnapshotByID(ctx context.Context, snapshotID string) (snapshot *Snapshot, err error)
	ListSnapshots(ctx context.Context, volumeID string, maxResults int64, nextToken string) (listSnapshotsResponse *ListSnapshotsResponse, err error)
	EnableFastSnapshotRestores(ctx context.Context, availabilityZones []string, snapshotID string) (*ec2.EnableFastSnapshotRestoresOutput, error)
	AvailabilityZones(ctx context.Context) (map[string]struct{}, error)
}
