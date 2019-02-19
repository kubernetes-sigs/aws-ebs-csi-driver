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

package driver

import (
	"github.com/kubernetes-sigs/aws-ebs-csi-driver/pkg/cloud"
	"github.com/kubernetes-sigs/aws-ebs-csi-driver/pkg/driver/internal"
	"k8s.io/kubernetes/pkg/util/mount"
)

func NewFakeMounter() *mount.FakeMounter {
	return &mount.FakeMounter{
		MountPoints: []mount.MountPoint{},
		Log:         []mount.FakeAction{},
	}
}

func NewFakeSafeFormatAndMounter(fakeMounter *mount.FakeMounter) *mount.SafeFormatAndMount {
	return &mount.SafeFormatAndMount{
		Interface: fakeMounter,
		Exec:      mount.NewFakeExec(nil),
	}

}

// NewFakeDriver creates a new mock driver used for testing
func NewFakeDriver(endpoint string, fakeCloud *cloud.FakeCloudProvider, fakeMounter *mount.FakeMounter) *Driver {
	return &Driver{
		endpoint: endpoint,
		nodeID:   fakeCloud.GetMetadata().GetInstanceID(),
		cloud:    fakeCloud,
		mounter:  NewFakeSafeFormatAndMounter(fakeMounter),
		inFlight: internal.NewInFlight(),
	}
}
