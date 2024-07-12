// Copyright 2024 The Kubernetes Authors.
//
// Licensed under the Apache License, Version 2.0 (the 'License');
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//    http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an 'AS IS' BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package cloud

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
)

type Cloud interface {
	CreateDisk(ctx context.Context, volumeName string, diskOptions *DiskOptions) (disk *Disk, err error)
	DeleteDisk(ctx context.Context, volumeID string) (success bool, err error)
	AttachDisk(ctx context.Context, volumeID string, nodeID string) (devicePath string, err error)
	DetachDisk(ctx context.Context, volumeID string, nodeID string) (err error)
	ModifyTags(ctx context.Context, volumeID string, tagOptions ModifyTagsOptions) (err error)
	ResizeOrModifyDisk(ctx context.Context, volumeID string, newSizeBytes int64, options *ModifyDiskOptions) (newSize int32, err error)
	WaitForAttachmentState(ctx context.Context, volumeID, expectedState string, expectedInstance string, expectedDevice string, alreadyAssigned bool) (*types.VolumeAttachment, error)
	GetDiskByName(ctx context.Context, name string, capacityBytes int64) (disk *Disk, err error)
	GetDiskByID(ctx context.Context, volumeID string) (disk *Disk, err error)
	CreateSnapshot(ctx context.Context, volumeID string, snapshotOptions *SnapshotOptions) (snapshot *Snapshot, err error)
	DeleteSnapshot(ctx context.Context, snapshotID string) (success bool, err error)
	GetSnapshotByName(ctx context.Context, name string) (snapshot *Snapshot, err error)
	GetSnapshotByID(ctx context.Context, snapshotID string) (snapshot *Snapshot, err error)
	ListSnapshots(ctx context.Context, volumeID string, maxResults int32, nextToken string) (listSnapshotsResponse *ListSnapshotsResponse, err error)
	EnableFastSnapshotRestores(ctx context.Context, availabilityZones []string, snapshotID string) (*ec2.EnableFastSnapshotRestoresOutput, error)
	AvailabilityZones(ctx context.Context) (map[string]struct{}, error)
}
