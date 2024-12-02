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

//go:build linux
// +build linux

package sanity

import (
	"fmt"
	"os"
	"path"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws/arn"
	"github.com/golang/mock/gomock"
	csisanity "github.com/kubernetes-csi/csi-test/v5/pkg/sanity"
	"github.com/kubernetes-sigs/aws-ebs-csi-driver/pkg/cloud/metadata"
	"github.com/kubernetes-sigs/aws-ebs-csi-driver/pkg/driver"
	"github.com/kubernetes-sigs/aws-ebs-csi-driver/pkg/util"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func TestSanity(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("Test panicked: %v", r)
		}
	}()

	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	tmpDir, err := os.MkdirTemp("", "csi-sanity-")
	if err != nil {
		t.Fatalf("Failed to create sanity temp working dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	defer func() {
		if err = os.RemoveAll(tmpDir); err != nil {
			t.Fatalf("Failed to clean up sanity temp working dir %s: %v", tmpDir, err.Error())
		}
	}()

	endpoint := fmt.Sprintf("unix:%s/csi.sock", tmpDir)
	mountPath := path.Join(tmpDir, "mount")
	stagePath := path.Join(tmpDir, "stage")
	instanceID := "i-1234567890abcdef0"
	region := "us-west-2"
	availabilityZone := "us-west-2a"

	driverOptions := &driver.Options{
		Mode:                              driver.AllMode,
		ModifyVolumeRequestHandlerTimeout: 60,
		Endpoint:                          endpoint,
	}

	fakeMetadata := &metadata.Metadata{
		InstanceID: instanceID,
		Region:     region,
	}

	outpostArn := &arn.ARN{
		Partition: "aws",
		Service:   "outposts",
		Region:    "us-west-2",
		AccountID: "123456789012",
		Resource:  "op-1234567890abcdef0",
	}

	drv, err := driver.NewDriver(newFakeCloud(*fakeMetadata, mountPath), driverOptions, newFakeMounter(), newFakeMetadataService(instanceID, region, availabilityZone, *outpostArn), nil)
	if err != nil {
		t.Fatalf("Failed to create fake driver: %v", err.Error())
	}
	go func() {
		if err := drv.Run(); err != nil {
			panic(fmt.Sprintf("%v", err))
		}
	}()

	config := csisanity.TestConfig{
		TargetPath:                  mountPath,
		StagingPath:                 stagePath,
		Address:                     endpoint,
		DialOptions:                 []grpc.DialOption{grpc.WithTransportCredentials(insecure.NewCredentials())},
		IDGen:                       csisanity.DefaultIDGenerator{},
		TestVolumeSize:              10 * util.GiB,
		TestVolumeAccessType:        "mount",
		TestVolumeMutableParameters: map[string]string{"iops": "3014", "throughput": "153"},
		TestVolumeParameters:        map[string]string{"type": "gp3", "iops": "3000"},
	}
	csisanity.Test(t, config)
}
