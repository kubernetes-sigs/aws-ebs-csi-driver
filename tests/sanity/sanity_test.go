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

package sanity

import (
	"os"
	"testing"

	"github.com/bertinatto/ebs-csi-driver/pkg/cloud"
	"github.com/bertinatto/ebs-csi-driver/pkg/driver"
	sanity "github.com/kubernetes-csi/csi-test/pkg/sanity"
)

func TestSanity(t *testing.T) {
	const (
		mountPath = "/tmp/csi/mount"
		stagePath = "/tmp/csi/stage"
		socket    = "/tmp/csi.sock"
		endpoint  = "unix://" + socket
	)

	if err := os.Remove(socket); err != nil && !os.IsNotExist(err) {
		t.Fatalf("could not remove socket file %s: %v", socket, err)
	}

	ebsDriver := driver.NewDriver(cloud.NewFakeCloudProvider(), driver.NewFakeMounter(), endpoint)
	defer ebsDriver.Stop()

	go func() {
		if err := ebsDriver.Run(); err != nil {
			t.Fatalf("could not run CSI driver: %v", err)
		}
	}()

	config := &sanity.Config{
		Address:     endpoint,
		TargetPath:  mountPath,
		StagingPath: stagePath,
	}

	sanity.Test(t, config)
}
