/*
Copyright 2019 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package cloud

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/service/ec2"
)

// EC2 abstracts aws.EC2 to facilitate its mocking.
// See https://docs.aws.amazon.com/sdk-for-go/api/service/ec2/ for details
type EC2 interface {
	DescribeVolumesWithContext(ctx aws.Context, input *ec2.DescribeVolumesInput, opts ...request.Option) (*ec2.DescribeVolumesOutput, error)
	DescribeVolumeStatusWithContext(ctx aws.Context, input *ec2.DescribeVolumeStatusInput, opts ...request.Option) (*ec2.DescribeVolumeStatusOutput, error)
	CreateVolumeWithContext(ctx aws.Context, input *ec2.CreateVolumeInput, opts ...request.Option) (*ec2.Volume, error)
	DeleteVolumeWithContext(ctx aws.Context, input *ec2.DeleteVolumeInput, opts ...request.Option) (*ec2.DeleteVolumeOutput, error)
	DetachVolumeWithContext(ctx aws.Context, input *ec2.DetachVolumeInput, opts ...request.Option) (*ec2.VolumeAttachment, error)
	AttachVolumeWithContext(ctx aws.Context, input *ec2.AttachVolumeInput, opts ...request.Option) (*ec2.VolumeAttachment, error)
	DescribeInstancesWithContext(ctx aws.Context, input *ec2.DescribeInstancesInput, opts ...request.Option) (*ec2.DescribeInstancesOutput, error)
	CreateSnapshotWithContext(ctx aws.Context, input *ec2.CreateSnapshotInput, opts ...request.Option) (*ec2.Snapshot, error)
	DeleteSnapshotWithContext(ctx aws.Context, input *ec2.DeleteSnapshotInput, opts ...request.Option) (*ec2.DeleteSnapshotOutput, error)
	DescribeSnapshotsWithContext(ctx aws.Context, input *ec2.DescribeSnapshotsInput, opts ...request.Option) (*ec2.DescribeSnapshotsOutput, error)
	ModifyVolumeWithContext(ctx aws.Context, input *ec2.ModifyVolumeInput, opts ...request.Option) (*ec2.ModifyVolumeOutput, error)
	DescribeVolumesModificationsWithContext(ctx aws.Context, input *ec2.DescribeVolumesModificationsInput, opts ...request.Option) (*ec2.DescribeVolumesModificationsOutput, error)
	DescribeAvailabilityZonesWithContext(ctx aws.Context, input *ec2.DescribeAvailabilityZonesInput, opts ...request.Option) (*ec2.DescribeAvailabilityZonesOutput, error)
	CreateTagsWithContext(ctx aws.Context, input *ec2.CreateTagsInput, opts ...request.Option) (*ec2.CreateTagsOutput, error)
}
