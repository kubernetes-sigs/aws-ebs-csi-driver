#!/bin/bash

# Copyright 2023 The Kubernetes Authors.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

set -euo pipefail

BIN="$(dirname "$(realpath "${BASH_SOURCE[0]}")")/../bin"

# Source-based mocking for internal interfaces
"${BIN}/mockgen" -package cloud -destination=./pkg/cloud/mock_cloud.go -source pkg/cloud/cloud_interface.go
"${BIN}/mockgen" -package cloud -destination=./pkg/cloud/mock_metadata.go -source pkg/cloud/metadata_interface.go
"${BIN}/mockgen" -package driver -destination=./pkg/driver/mock_mount.go -source pkg/driver/mount.go
"${BIN}/mockgen" -package mounter -destination=./pkg/mounter/mock_mount_windows.go -source pkg/mounter/safe_mounter_windows.go
"${BIN}/mockgen" -package cloud -destination=./pkg/cloud/mock_ec2.go -source pkg/cloud/ec2_interface.go EC2API

# Reflection-based mocking for external dependencies
"${BIN}/mockgen" -package driver -destination=./pkg/driver/mock_k8s_client.go -mock_names='Interface=MockKubernetesClient' k8s.io/client-go/kubernetes Interface
"${BIN}/mockgen" -package driver -destination=./pkg/driver/mock_k8s_corev1.go k8s.io/client-go/kubernetes/typed/core/v1 CoreV1Interface,NodeInterface
"${BIN}/mockgen" -package driver -destination=./pkg/driver/mock_k8s_storagev1.go k8s.io/client-go/kubernetes/typed/storage/v1 VolumeAttachmentInterface,StorageV1Interface
"${BIN}/mockgen" -package driver -destination=./pkg/driver/mock_k8s_storagev1_csinode.go k8s.io/client-go/kubernetes/typed/storage/v1 CSINodeInterface

# Fixes "Mounter Type cannot implement 'Mounter' as it has a non-exported method and is defined in a different package"
# See https://github.com/kubernetes/mount-utils/commit/a20fcfb15a701977d086330b47b7efad51eb608e for context.
sed -i '/type MockMounter struct {/a \\tmount_utils.Interface' pkg/driver/mock_mount.go
sed -i '/type MockProxyMounter struct {/a \\tmount.Interface' pkg/mounter/mock_mount_windows.go
