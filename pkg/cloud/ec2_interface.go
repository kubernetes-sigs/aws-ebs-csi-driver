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
)

type EC2API interface {
	DescribeVolumes(ctx context.Context, params *ec2.DescribeVolumesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeVolumesOutput, error)
	CreateVolume(ctx context.Context, params *ec2.CreateVolumeInput, optFns ...func(*ec2.Options)) (*ec2.CreateVolumeOutput, error)
	DeleteVolume(ctx context.Context, params *ec2.DeleteVolumeInput, optFns ...func(*ec2.Options)) (*ec2.DeleteVolumeOutput, error)
	AttachVolume(ctx context.Context, params *ec2.AttachVolumeInput, optFns ...func(*ec2.Options)) (*ec2.AttachVolumeOutput, error)
	DetachVolume(ctx context.Context, params *ec2.DetachVolumeInput, optFns ...func(*ec2.Options)) (*ec2.DetachVolumeOutput, error)
	DescribeInstances(ctx context.Context, params *ec2.DescribeInstancesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeInstancesOutput, error)
	DescribeAvailabilityZones(ctx context.Context, params *ec2.DescribeAvailabilityZonesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeAvailabilityZonesOutput, error)
	CreateSnapshot(ctx context.Context, params *ec2.CreateSnapshotInput, optFns ...func(*ec2.Options)) (*ec2.CreateSnapshotOutput, error)
	DeleteSnapshot(ctx context.Context, params *ec2.DeleteSnapshotInput, optFns ...func(*ec2.Options)) (*ec2.DeleteSnapshotOutput, error)
	DescribeSnapshots(ctx context.Context, params *ec2.DescribeSnapshotsInput, optFns ...func(*ec2.Options)) (*ec2.DescribeSnapshotsOutput, error)
	ModifyVolume(ctx context.Context, params *ec2.ModifyVolumeInput, optFns ...func(*ec2.Options)) (*ec2.ModifyVolumeOutput, error)
	DescribeVolumesModifications(ctx context.Context, params *ec2.DescribeVolumesModificationsInput, optFns ...func(*ec2.Options)) (*ec2.DescribeVolumesModificationsOutput, error)
	DescribeTags(ctx context.Context, params *ec2.DescribeTagsInput, optFns ...func(*ec2.Options)) (*ec2.DescribeTagsOutput, error)
	CreateTags(ctx context.Context, params *ec2.CreateTagsInput, optFns ...func(*ec2.Options)) (*ec2.CreateTagsOutput, error)
	EnableFastSnapshotRestores(ctx context.Context, params *ec2.EnableFastSnapshotRestoresInput, optFns ...func(*ec2.Options)) (*ec2.EnableFastSnapshotRestoresOutput, error)
}
