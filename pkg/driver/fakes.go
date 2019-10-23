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

package driver

import (
	"github.com/kubernetes-sigs/aws-ebs-csi-driver/pkg/cloud"
	"github.com/kubernetes-sigs/aws-ebs-csi-driver/pkg/driver/internal"
	"k8s.io/kubernetes/pkg/util/mount"
)

// NewFakeDriver creates a new mock driver used for testing
func NewFakeDriver(endpoint string, fakeCloud cloud.Cloud, fakeMounter *mount.FakeMounter) *Driver {
	driverOptions := &DriverOptions{
		endpoint: endpoint,
	}
	return &Driver{
		options: driverOptions,
		controllerService: controllerService{
			cloud:         fakeCloud,
			driverOptions: driverOptions,
		},
		nodeService: nodeService{
			metadata: &cloud.Metadata{
				InstanceID:       "instanceID",
				Region:           "region",
				AvailabilityZone: "az",
			},
			mounter:  &NodeMounter{mount.SafeFormatAndMount{Interface: fakeMounter, Exec: mount.NewFakeExec(nil)}},
			inFlight: internal.NewInFlight(),
		},
	}
}
