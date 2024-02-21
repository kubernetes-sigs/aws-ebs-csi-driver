/*
Copyright 2024 The Kubernetes Authors.

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
	"context"
	"fmt"

	"sync"
	"testing"
	"time"

	"github.com/awslabs/volume-modifier-for-k8s/pkg/rpc"
	"github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/kubernetes-sigs/aws-ebs-csi-driver/pkg/driver/internal"
	"github.com/kubernetes-sigs/aws-ebs-csi-driver/pkg/util"
	"k8s.io/klog/v2"

	"github.com/golang/mock/gomock"
	"github.com/kubernetes-sigs/aws-ebs-csi-driver/pkg/cloud"
)

type modifyVolumeExecutor func(ctx context.Context, driver controllerService, name string, params map[string]string) error

func externalResizerModifyVolume(ctx context.Context, driver controllerService, name string, params map[string]string) error {
	_, err := driver.ControllerModifyVolume(ctx, &csi.ControllerModifyVolumeRequest{
		VolumeId:          name,
		MutableParameters: params,
	})
	return err
}

func modifierForK8sModifyVolume(ctx context.Context, driver controllerService, name string, params map[string]string) error {
	_, err := driver.ModifyVolumeProperties(ctx, &rpc.ModifyVolumePropertiesRequest{
		Name:       name,
		Parameters: params,
	})
	return err
}

func TestVolumeModificationWithCoalescing(t *testing.T) {
	testCases := []struct {
		name         string
		testFunction func(t *testing.T, executor modifyVolumeExecutor)
	}{
		{
			name:         "basic request coalescing success",
			testFunction: testBasicRequestCoalescingSuccess,
		},
		{
			name:         "request fail",
			testFunction: testRequestFail,
		},
		{
			name:         "partial fail",
			testFunction: testPartialFail,
		},
		{
			name:         "sequential requests",
			testFunction: testSequentialRequests,
		},
		{
			name:         "duplicate requests",
			testFunction: testDuplicateRequest,
		},
		{
			name:         "context timeout",
			testFunction: testContextTimeout,
		},
		{
			name:         "timing",
			testFunction: testResponseReturnTiming,
		},
	}

	for _, tc := range testCases {
		tc := tc // Not strictly necessary but required by `go vet`
		t.Run(tc.name+": volume-modifier-for-k8s", func(t *testing.T) {
			t.Parallel()
			tc.testFunction(t, modifierForK8sModifyVolume)
		})
		t.Run(tc.name+": external-resizer", func(t *testing.T) {
			t.Parallel()
			tc.testFunction(t, externalResizerModifyVolume)
		})
	}
}

// TestBasicRequestCoalescingSuccess tests the success case of coalescing 2 requests from ControllerExpandVolume and a modify function respectively.
func testBasicRequestCoalescingSuccess(t *testing.T, executor modifyVolumeExecutor) {
	const NewVolumeType = "gp3"
	const NewSize = 5 * util.GiB
	volumeID := t.Name()

	mockCtl := gomock.NewController(t)
	defer mockCtl.Finish()

	mockCloud := cloud.NewMockCloud(mockCtl)
	mockCloud.EXPECT().ResizeOrModifyDisk(gomock.Any(), gomock.Eq(volumeID), gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, volumeID string, newSize int64, options *cloud.ModifyDiskOptions) (int64, error) {
		klog.InfoS("ResizeOrModifyDisk called", "volumeID", volumeID, "newSize", newSize, "options", options)
		if newSize != NewSize {
			t.Errorf("newSize incorrect")
		} else if options.VolumeType != NewVolumeType {
			t.Errorf("VolumeType incorrect")
		}

		return newSize, nil
	})

	awsDriver := controllerService{
		cloud:    mockCloud,
		inFlight: internal.NewInFlight(),
		driverOptions: &DriverOptions{
			modifyVolumeRequestHandlerTimeout: 2 * time.Second,
		},
		modifyVolumeManager: newModifyVolumeManager(),
	}

	var wg sync.WaitGroup
	wg.Add(2)

	go wrapTimeout(t, "ControllerExpandVolume timed out", func() {
		_, err := awsDriver.ControllerExpandVolume(context.Background(), &csi.ControllerExpandVolumeRequest{
			VolumeId: volumeID,
			CapacityRange: &csi.CapacityRange{
				RequiredBytes: NewSize,
			},
		})

		if err != nil {
			t.Error("ControllerExpandVolume returned error")
		}
		wg.Done()
	})
	go wrapTimeout(t, "Modify timed out", func() {
		err := executor(context.Background(), awsDriver, volumeID, map[string]string{
			ModificationKeyVolumeType: NewVolumeType,
		})

		if err != nil {
			t.Error("Modify returned error")
		}
		wg.Done()
	})

	wg.Wait()
}

// TestRequestFail tests failing requests from ResizeOrModifyDisk failure.
func testRequestFail(t *testing.T, executor modifyVolumeExecutor) {
	const NewVolumeType = "gp3"
	const NewSize = 5 * util.GiB
	volumeID := t.Name()

	mockCtl := gomock.NewController(t)
	defer mockCtl.Finish()

	mockCloud := cloud.NewMockCloud(mockCtl)
	mockCloud.EXPECT().ResizeOrModifyDisk(gomock.Any(), gomock.Eq(volumeID), gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, volumeID string, newSize int64, options *cloud.ModifyDiskOptions) (int64, error) {
		klog.InfoS("ResizeOrModifyDisk called", "volumeID", volumeID, "newSize", newSize, "options", options)
		return 0, fmt.Errorf("ResizeOrModifyDisk failed")
	})

	awsDriver := controllerService{
		cloud:    mockCloud,
		inFlight: internal.NewInFlight(),
		driverOptions: &DriverOptions{
			modifyVolumeRequestHandlerTimeout: 2 * time.Second,
		},
		modifyVolumeManager: newModifyVolumeManager(),
	}

	var wg sync.WaitGroup
	wg.Add(2)

	go wrapTimeout(t, "ControllerExpandVolume timed out", func() {
		_, err := awsDriver.ControllerExpandVolume(context.Background(), &csi.ControllerExpandVolumeRequest{
			VolumeId: volumeID,
			CapacityRange: &csi.CapacityRange{
				RequiredBytes: NewSize,
			},
		})

		if err == nil {
			t.Error("ControllerExpandVolume should fail")
		}
		wg.Done()
	})
	go wrapTimeout(t, "Modify timed out", func() {
		err := executor(context.Background(), awsDriver, volumeID, map[string]string{
			ModificationKeyVolumeType: NewVolumeType,
		})

		if err == nil {
			t.Error("Modify should fail")
		}
		wg.Done()
	})

	wg.Wait()
}

// TestPartialFail tests making these 3 requests roughly in parallel:
// 1) Change size
// 2) Change volume type to NewVolumeType1
// 3) Change volume type to NewVolumeType2
// The expected result is the resizing request succeeds and one of the volume-type requests fails.
func testPartialFail(t *testing.T, executor modifyVolumeExecutor) {
	const NewVolumeType1 = "gp3"
	const NewVolumeType2 = "io2"
	const NewSize = 5 * util.GiB
	volumeID := t.Name()

	mockCtl := gomock.NewController(t)
	defer mockCtl.Finish()

	volumeTypeChosen := ""

	mockCloud := cloud.NewMockCloud(mockCtl)
	mockCloud.EXPECT().ResizeOrModifyDisk(gomock.Any(), gomock.Eq(volumeID), gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, volumeID string, newSize int64, options *cloud.ModifyDiskOptions) (int64, error) {
		klog.InfoS("ResizeOrModifyDisk called", "volumeID", volumeID, "newSize", newSize, "options", options)
		if newSize != NewSize {
			t.Errorf("newSize incorrect")
		} else if options.VolumeType == "" {
			t.Errorf("no volume type")
		}

		volumeTypeChosen = options.VolumeType
		return newSize, nil
	})

	awsDriver := controllerService{
		cloud:    mockCloud,
		inFlight: internal.NewInFlight(),
		driverOptions: &DriverOptions{
			modifyVolumeRequestHandlerTimeout: 2 * time.Second,
		},
		modifyVolumeManager: newModifyVolumeManager(),
	}

	var wg sync.WaitGroup
	wg.Add(3)

	volumeType1Err, volumeType2Error := false, false

	go wrapTimeout(t, "ControllerExpandVolume timed out", func() {
		_, err := awsDriver.ControllerExpandVolume(context.Background(), &csi.ControllerExpandVolumeRequest{
			VolumeId: volumeID,
			CapacityRange: &csi.CapacityRange{
				RequiredBytes: NewSize,
			},
		})

		if err != nil {
			t.Error("ControllerExpandVolume returned error")
		}
		wg.Done()
	})
	go wrapTimeout(t, "Modify timed out", func() {
		err := executor(context.Background(), awsDriver, volumeID, map[string]string{
			ModificationKeyVolumeType: NewVolumeType1, // gp3
		})
		volumeType1Err = err != nil
		wg.Done()
	})
	go wrapTimeout(t, "Modify timed out", func() {
		err := executor(context.Background(), awsDriver, volumeID, map[string]string{
			ModificationKeyVolumeType: NewVolumeType2, // io2
		})
		if err != nil {
			klog.InfoS("Got err io2")
		}
		volumeType2Error = err != nil
		wg.Done()
	})

	wg.Wait()

	if volumeTypeChosen == NewVolumeType1 {
		if volumeType1Err {
			t.Error("Controller chose", NewVolumeType1, "but errored request")
		}
		if !volumeType2Error {
			t.Error("Controller chose", NewVolumeType1, "but returned success to", NewVolumeType2, "request")
		}
	} else if volumeTypeChosen == NewVolumeType2 {
		if volumeType2Error {
			t.Error("Controller chose", NewVolumeType2, "but errored request")
		}
		if !volumeType1Err {
			t.Error("Controller chose", NewVolumeType2, "but returned success to", NewVolumeType1, "request")
		}
	} else {
		t.Error("No volume type chosen")
	}
}

// TestSequential tests sending 2 requests sequentially.
func testSequentialRequests(t *testing.T, executor modifyVolumeExecutor) {
	const NewVolumeType = "gp3"
	const NewSize = 5 * util.GiB
	volumeID := t.Name()

	mockCtl := gomock.NewController(t)
	defer mockCtl.Finish()

	mockCloud := cloud.NewMockCloud(mockCtl)
	mockCloud.EXPECT().ResizeOrModifyDisk(gomock.Any(), gomock.Eq(volumeID), gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, volumeID string, newSize int64, options *cloud.ModifyDiskOptions) (int64, error) {
		klog.InfoS("ResizeOrModifyDisk", "volumeID", volumeID, "newSize", newSize, "options", options)
		return newSize, nil
	}).Times(2)

	awsDriver := controllerService{
		cloud:    mockCloud,
		inFlight: internal.NewInFlight(),
		driverOptions: &DriverOptions{
			modifyVolumeRequestHandlerTimeout: 2 * time.Second,
		},
		modifyVolumeManager: newModifyVolumeManager(),
	}

	var wg sync.WaitGroup
	wg.Add(2)

	go wrapTimeout(t, "ControllerExpandVolume timed out", func() {
		_, err := awsDriver.ControllerExpandVolume(context.Background(), &csi.ControllerExpandVolumeRequest{
			VolumeId: volumeID,
			CapacityRange: &csi.CapacityRange{
				RequiredBytes: NewSize,
			},
		})

		if err != nil {
			t.Error("ControllerExpandVolume returned error")
		}
		wg.Done()
	})

	// We expect ModifyVolume to be called by the end of this sleep
	time.Sleep(5 * time.Second)

	go wrapTimeout(t, "Modify timed out", func() {
		err := executor(context.Background(), awsDriver, volumeID, map[string]string{
			ModificationKeyVolumeType: NewVolumeType,
		})

		if err != nil {
			t.Error("Modify returned error")
		}
		wg.Done()
	})

	wg.Wait()
}

// TestDuplicateRequest tests sending multiple same requests roughly in parallel.
func testDuplicateRequest(t *testing.T, executor modifyVolumeExecutor) {
	const NewSize = 5 * util.GiB
	volumeID := t.Name()

	mockCtl := gomock.NewController(t)
	defer mockCtl.Finish()

	mockCloud := cloud.NewMockCloud(mockCtl)
	mockCloud.EXPECT().ResizeOrModifyDisk(gomock.Any(), gomock.Eq(volumeID), gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, volumeID string, newSize int64, options *cloud.ModifyDiskOptions) (int64, error) {
		klog.InfoS("ResizeOrModifyDisk called", "volumeID", volumeID, "newSize", newSize, "options", options)
		return newSize, nil
	})

	awsDriver := controllerService{
		cloud:    mockCloud,
		inFlight: internal.NewInFlight(),
		driverOptions: &DriverOptions{
			modifyVolumeRequestHandlerTimeout: 2 * time.Second,
		},
		modifyVolumeManager: newModifyVolumeManager(),
	}

	var wg sync.WaitGroup
	num := 5
	wg.Add(num * 2)

	for j := 0; j < num; j++ {
		go wrapTimeout(t, "ControllerExpandVolume timed out", func() {
			_, err := awsDriver.ControllerExpandVolume(context.Background(), &csi.ControllerExpandVolumeRequest{
				VolumeId: volumeID,
				CapacityRange: &csi.CapacityRange{
					RequiredBytes: NewSize,
				},
			})
			if err != nil {
				t.Error("Duplicate ControllerExpandVolume request should succeed")
			}
			wg.Done()
		})
		go wrapTimeout(t, "Modify timed out", func() {
			err := executor(context.Background(), awsDriver, volumeID, map[string]string{
				ModificationKeyVolumeType: "io2",
			})
			if err != nil {
				t.Error("Duplicate Modify request should succeed")
			}
			wg.Done()
		})
	}

	wg.Wait()
}

// TestContextTimeout tests request failing due to context cancellation and the behavior of the following request.
func testContextTimeout(t *testing.T, executor modifyVolumeExecutor) {
	const NewVolumeType = "gp3"
	const NewSize = 5 * util.GiB
	volumeID := t.Name()

	mockCtl := gomock.NewController(t)
	defer mockCtl.Finish()

	mockCloud := cloud.NewMockCloud(mockCtl)
	mockCloud.EXPECT().ResizeOrModifyDisk(gomock.Any(), gomock.Eq(volumeID), gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, volumeID string, newSize int64, options *cloud.ModifyDiskOptions) (int64, error) {
		klog.InfoS("ResizeOrModifyDisk called", "volumeID", volumeID, "newSize", newSize, "options", options)
		time.Sleep(3 * time.Second)

		// Controller could decide to coalesce the timed out request, or to drop it
		if newSize != 0 && newSize != NewSize {
			t.Errorf("newSize incorrect")
		} else if options.VolumeType != NewVolumeType {
			t.Errorf("volumeType incorrect")
		}

		return newSize, nil
	})

	awsDriver := controllerService{
		cloud:    mockCloud,
		inFlight: internal.NewInFlight(),
		driverOptions: &DriverOptions{
			modifyVolumeRequestHandlerTimeout: 2 * time.Second,
		},
		modifyVolumeManager: newModifyVolumeManager(),
	}

	var wg sync.WaitGroup
	wg.Add(2)

	ctx, cancel := context.WithCancel(context.Background())
	go wrapTimeout(t, "ControllerExpandVolume timed out", func() {
		_, err := awsDriver.ControllerExpandVolume(ctx, &csi.ControllerExpandVolumeRequest{
			VolumeId: volumeID,
			CapacityRange: &csi.CapacityRange{
				RequiredBytes: NewSize,
			},
		})
		if err == nil {
			t.Error("ControllerExpandVolume should return err because context is cancelled")
		}
		wg.Done()
	})

	// Cancel the context (simulate a "sidecar timeout")
	time.Sleep(500 * time.Millisecond)
	cancel()

	go wrapTimeout(t, "Modify timed out", func() {
		err := executor(context.Background(), awsDriver, volumeID, map[string]string{
			ModificationKeyVolumeType: NewVolumeType,
		})

		if err != nil {
			t.Error("Modify returned error")
		}
		wg.Done()
	})

	wg.Wait()
}

// TestResponseReturnTiming tests the caller of request coalescing blocking until receiving response from cloud.ResizeOrModifyDisk
func testResponseReturnTiming(t *testing.T, executor modifyVolumeExecutor) {
	const NewVolumeType = "gp3"
	const NewSize = 5 * util.GiB
	var ec2ModifyVolumeFinished = false
	volumeID := t.Name()

	mockCtl := gomock.NewController(t)
	defer mockCtl.Finish()

	mockCloud := cloud.NewMockCloud(mockCtl)
	mockCloud.EXPECT().ResizeOrModifyDisk(gomock.Any(), gomock.Eq(volumeID), gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, volumeID string, newSize int64, options *cloud.ModifyDiskOptions) (int64, error) {
		klog.InfoS("ResizeOrModifyDisk called", "volumeID", volumeID, "newSize", newSize, "options", options)

		// Sleep to simulate ec2.ModifyVolume taking a long time
		time.Sleep(5 * time.Second)
		ec2ModifyVolumeFinished = true

		return newSize, nil
	})

	awsDriver := controllerService{
		cloud:    mockCloud,
		inFlight: internal.NewInFlight(),
		driverOptions: &DriverOptions{
			modifyVolumeRequestHandlerTimeout: 2 * time.Second,
		},
		modifyVolumeManager: newModifyVolumeManager(),
	}

	var wg sync.WaitGroup
	wg.Add(2)

	go wrapTimeout(t, "ControllerExpandVolume timed out", func() {
		_, err := awsDriver.ControllerExpandVolume(context.Background(), &csi.ControllerExpandVolumeRequest{
			VolumeId: volumeID,
			CapacityRange: &csi.CapacityRange{
				RequiredBytes: NewSize,
			},
		})

		if !ec2ModifyVolumeFinished {
			t.Error("ControllerExpandVolume returned success BEFORE ResizeOrModifyDisk returns")
		}
		if err != nil {
			t.Error("ControllerExpandVolume returned error")
		}
		wg.Done()
	})
	go wrapTimeout(t, "Modify timed out", func() {
		err := executor(context.Background(), awsDriver, volumeID, map[string]string{
			ModificationKeyVolumeType: NewVolumeType,
		})

		if !ec2ModifyVolumeFinished {
			t.Error("Modify returned success BEFORE ResizeOrModifyDisk returns")
		}
		if err != nil {
			t.Error("Modify returned error")
		}

		wg.Done()
	})

	wg.Wait()
}

func wrapTimeout(t *testing.T, failMessage string, execFunc func()) {
	timeout := time.After(15 * time.Second)
	done := make(chan bool)
	go func() {
		execFunc()
		done <- true
	}()

	select {
	case <-timeout:
		t.Error(failMessage)
	case <-done:
	}
}
