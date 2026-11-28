//go:build windows

/*
Copyright 2026 The Kubernetes Authors.

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

package mounter

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/sys/windows"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestHandleGetDeviceNameFromMountError(t *testing.T) {
	testCases := []struct {
		name        string
		err         error
		expectError bool
	}{
		{
			name:        "csi-proxy v1.2.1 unmounted folder",
			err:         status.Error(codes.Unknown, `error getting volume from mount. cmd: (Get-Item -Path $Env:mountpath).Target, output: , error: <nil>`),
			expectError: false,
		},
		{
			name:        "csi-proxy v1.2.1 missing path",
			err:         status.Error(codes.Unknown, `error getting volume from mount. cmd: (Get-Item -Path $Env:mountpath).Target, output: Get-Item : Cannot find path 'C:\var\lib\kubelet\plugins\kubernetes.io\csi\ebs.csi.aws.com\abc\globalmount' because it does not exist., error: exit status 1`),
			expectError: false,
		},
		{
			name:        "csi-proxy v1.3.0+ unmounted folder over named pipe",
			err:         status.Error(codes.Unknown, "not a mounted folder"),
			expectError: false,
		},
		{
			name:        "csi-proxy v2 library unmounted folder",
			err:         errors.New("not a mounted folder"),
			expectError: false,
		},
		{
			name:        "csi-proxy v2 library missing path",
			err:         fmt.Errorf("wrapped: %w", windows.ERROR_FILE_NOT_FOUND),
			expectError: false,
		},
		{
			name:        "csi-proxy unreachable",
			err:         status.Error(codes.Unavailable, `connection error: desc = "transport: Error while dialing: open \\.\pipe\csi-proxy-volume-v1: The system cannot find the file specified."`),
			expectError: true,
		},
		{
			name:        "other csi-proxy error",
			err:         status.Error(codes.Unknown, "Access is denied."),
			expectError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			device, refCount, err := handleGetDeviceNameFromMountError(tc.err)
			if tc.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			assert.Empty(t, device)
			assert.Equal(t, 0, refCount)
		})
	}
}
