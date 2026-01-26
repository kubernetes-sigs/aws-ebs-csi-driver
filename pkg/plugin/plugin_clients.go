/*
Copyright 2025 The Kubernetes Authors.

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

// PLUGIN AUTHORS: DO NOT MODIFY THIS FILE
// This file contains the driver-maintained plugin API bases.
//
// In order to create a new plugin, create a new .go file in this package
// that implements your plugin logic, see plugin.go.sample and docs/plugins.md.

package plugin

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/sagemaker"
)

// Ec2ClientBase implements stub functionality for EC2API. It can be used
// by plugins that only want to customize a subset of the EC2 API calls.
// Use of this struct is NOT required, plugins may return any compliant
// EC2API implementation from GetEC2Client.
type Ec2ClientBase struct {
	client *ec2.Client
}

// init sets up the EC2 client in Ec2ClientBase. Must be called first.
func (b *Ec2ClientBase) init(cfg aws.Config, optFns ...func(*ec2.Options)) {
	b.client = ec2.NewFromConfig(cfg, optFns...)
}

// sagemakerClientBase implements stub functionality for SageMakerAPI. It can be used
// by plugins that only want to customize a subset of the SageMaker API calls.
// Use of this struct is NOT required, plugins may return any compliant
// SageMakerAPI implementation from GetSageMakerClient.
type sageMakerClientBase struct {
	client *sagemaker.Client
}

// init sets up the SageMaker client in sageMakerClientBase. Must be called first.
func (b *sageMakerClientBase) init(cfg aws.Config, optFns ...func(*sagemaker.Options)) {
	b.client = sagemaker.NewFromConfig(cfg, optFns...)
}

// EC2API stub functions.

func (b *Ec2ClientBase) DescribeVolumes(ctx context.Context, params *ec2.DescribeVolumesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeVolumesOutput, error) {
	return b.client.DescribeVolumes(ctx, params, optFns...)
}
func (b *Ec2ClientBase) DescribeVolumeStatus(ctx context.Context, params *ec2.DescribeVolumeStatusInput, optFns ...func(options *ec2.Options)) (*ec2.DescribeVolumeStatusOutput, error) {
	return b.client.DescribeVolumeStatus(ctx, params, optFns...)
}
func (b *Ec2ClientBase) CreateVolume(ctx context.Context, params *ec2.CreateVolumeInput, optFns ...func(*ec2.Options)) (*ec2.CreateVolumeOutput, error) {
	return b.client.CreateVolume(ctx, params, optFns...)
}
func (b *Ec2ClientBase) CopyVolumes(ctx context.Context, params *ec2.CopyVolumesInput, optFns ...func(*ec2.Options)) (*ec2.CopyVolumesOutput, error) {
	return b.client.CopyVolumes(ctx, params, optFns...)
}
func (b *Ec2ClientBase) DeleteVolume(ctx context.Context, params *ec2.DeleteVolumeInput, optFns ...func(*ec2.Options)) (*ec2.DeleteVolumeOutput, error) {
	return b.client.DeleteVolume(ctx, params, optFns...)
}
func (b *Ec2ClientBase) AttachVolume(ctx context.Context, params *ec2.AttachVolumeInput, optFns ...func(*ec2.Options)) (*ec2.AttachVolumeOutput, error) {
	return b.client.AttachVolume(ctx, params, optFns...)
}
func (b *Ec2ClientBase) DetachVolume(ctx context.Context, params *ec2.DetachVolumeInput, optFns ...func(*ec2.Options)) (*ec2.DetachVolumeOutput, error) {
	return b.client.DetachVolume(ctx, params, optFns...)
}
func (b *Ec2ClientBase) DescribeInstances(ctx context.Context, params *ec2.DescribeInstancesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeInstancesOutput, error) {
	return b.client.DescribeInstances(ctx, params, optFns...)
}
func (b *Ec2ClientBase) DescribeAvailabilityZones(ctx context.Context, params *ec2.DescribeAvailabilityZonesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeAvailabilityZonesOutput, error) {
	return b.client.DescribeAvailabilityZones(ctx, params, optFns...)
}
func (b *Ec2ClientBase) CreateSnapshot(ctx context.Context, params *ec2.CreateSnapshotInput, optFns ...func(*ec2.Options)) (*ec2.CreateSnapshotOutput, error) {
	return b.client.CreateSnapshot(ctx, params, optFns...)
}
func (b *Ec2ClientBase) DeleteSnapshot(ctx context.Context, params *ec2.DeleteSnapshotInput, optFns ...func(*ec2.Options)) (*ec2.DeleteSnapshotOutput, error) {
	return b.client.DeleteSnapshot(ctx, params, optFns...)
}
func (b *Ec2ClientBase) DescribeSnapshots(ctx context.Context, params *ec2.DescribeSnapshotsInput, optFns ...func(*ec2.Options)) (*ec2.DescribeSnapshotsOutput, error) {
	return b.client.DescribeSnapshots(ctx, params, optFns...)
}
func (b *Ec2ClientBase) ModifyVolume(ctx context.Context, params *ec2.ModifyVolumeInput, optFns ...func(*ec2.Options)) (*ec2.ModifyVolumeOutput, error) {
	return b.client.ModifyVolume(ctx, params, optFns...)
}
func (b *Ec2ClientBase) DescribeVolumesModifications(ctx context.Context, params *ec2.DescribeVolumesModificationsInput, optFns ...func(*ec2.Options)) (*ec2.DescribeVolumesModificationsOutput, error) {
	return b.client.DescribeVolumesModifications(ctx, params, optFns...)
}
func (b *Ec2ClientBase) DescribeTags(ctx context.Context, params *ec2.DescribeTagsInput, optFns ...func(*ec2.Options)) (*ec2.DescribeTagsOutput, error) {
	return b.client.DescribeTags(ctx, params, optFns...)
}
func (b *Ec2ClientBase) CreateTags(ctx context.Context, params *ec2.CreateTagsInput, optFns ...func(*ec2.Options)) (*ec2.CreateTagsOutput, error) {
	return b.client.CreateTags(ctx, params, optFns...)
}
func (b *Ec2ClientBase) DeleteTags(ctx context.Context, params *ec2.DeleteTagsInput, optFns ...func(*ec2.Options)) (*ec2.DeleteTagsOutput, error) {
	return b.client.DeleteTags(ctx, params, optFns...)
}
func (b *Ec2ClientBase) EnableFastSnapshotRestores(ctx context.Context, params *ec2.EnableFastSnapshotRestoresInput, optFns ...func(*ec2.Options)) (*ec2.EnableFastSnapshotRestoresOutput, error) {
	return b.client.EnableFastSnapshotRestores(ctx, params, optFns...)
}

// SagmeMakerAPI stub functions.

func (b *sageMakerClientBase) AttachClusterNodeVolume(ctx context.Context, params *sagemaker.AttachClusterNodeVolumeInput, optFns ...func(*sagemaker.Options)) (*sagemaker.AttachClusterNodeVolumeOutput, error) {
	return b.client.AttachClusterNodeVolume(ctx, params, optFns...)
}
func (b *sageMakerClientBase) DetachClusterNodeVolume(ctx context.Context, params *sagemaker.DetachClusterNodeVolumeInput, optFns ...func(*sagemaker.Options)) (*sagemaker.DetachClusterNodeVolumeOutput, error) {
	return b.client.DetachClusterNodeVolume(ctx, params, optFns...)
}
