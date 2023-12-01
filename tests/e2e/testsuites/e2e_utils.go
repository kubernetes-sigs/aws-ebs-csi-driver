/*
Copyright 2018 The Kubernetes Authors.

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

package testsuites

import "fmt"

const (
	DefaultVolumeName = "test-volume-1"
	DefaultMountPath  = "/mnt/default-mount"

	DefaultIopsIoVolumes = "100"
)

func PodCmdWriteToVolume(volumeMountPath string) string {
	return fmt.Sprintf("echo 'hello world' >> %s/data && grep 'hello world' %s/data && sync", volumeMountPath, volumeMountPath)
}

func CreateVolumeDetails(createVolumeParameters map[string]string, volumeSize string) *VolumeDetails {
	allowVolumeExpansion := true

	volume := VolumeDetails{
		MountOptions: []string{"rw"},
		ClaimSize:    volumeSize,
		VolumeMount: VolumeMountDetails{
			NameGenerate:      DefaultVolumeName,
			MountPathGenerate: DefaultMountPath,
		},
		AllowVolumeExpansion:   &allowVolumeExpansion,
		CreateVolumeParameters: createVolumeParameters,
	}

	return &volume
}
