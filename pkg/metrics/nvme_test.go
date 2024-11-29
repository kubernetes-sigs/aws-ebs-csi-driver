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

package metrics

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

func TestNewNVMECollector(t *testing.T) {
	testPath := "/test//unclean/../path"
	expectedPath := "/test/path/"
	testInstanceID := "test-instance-1"

	collector := NewNVMECollector(testPath, testInstanceID)

	if collector == nil {
		t.Fatal("NewNVMECollector returned nil")
	}

	if collector.csiMountPointPath != expectedPath {
		t.Errorf("csiMountPointPath = %v, want %v", collector.csiMountPointPath, expectedPath)
	}

	if collector.instanceID != testInstanceID {
		t.Errorf("instanceID = %v, want %v", collector.instanceID, testInstanceID)
	}

	expectedMetrics := []string{
		metricReadOps,
		metricWriteOps,
		metricReadBytes,
		metricWriteBytes,
		metricReadOpsSeconds,
		metricWriteOpsSeconds,
		metricExceededIOPS,
		metricExceededTP,
		metricExceededIOPS,
		metricExceededTP,
		metricVolumeQueueLength,
		metricReadLatency,
		metricWriteLatency,
	}

	for _, metricName := range expectedMetrics {
		if _, exists := collector.metrics[metricName]; !exists {
			t.Errorf("metric %s not found in collector.metrics", metricName)
		}
	}
}

func TestConvertHistogram(t *testing.T) {
	tests := []struct {
		name        string
		histogram   Histogram
		wantCount   uint64
		wantBuckets map[float64]uint64
	}{
		{
			name: "standard histogram with multiple bins",
			histogram: Histogram{
				BinCount: 3,
				Bins: [64]HistogramBin{
					{Lower: 0, Upper: 100, Count: 5},
					{Lower: 100, Upper: 200, Count: 3},
					{Lower: 200, Upper: 300, Count: 2},
				},
			},
			wantCount: 10,
			wantBuckets: map[float64]uint64{
				100 / 1e6: 5,
				200 / 1e6: 8,
				300 / 1e6: 10,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotCount, gotBuckets := convertHistogram(tt.histogram)

			if gotCount != tt.wantCount {
				t.Errorf("convertHistogram() count = %v, want %v", gotCount, tt.wantCount)
			}

			if !reflect.DeepEqual(gotBuckets, tt.wantBuckets) {
				t.Errorf("convertHistogram() buckets = %v, want %v", gotBuckets, tt.wantBuckets)
			}
		})
	}
}

func TestParseLogPage(t *testing.T) {
	tests := []struct {
		name    string
		input   []byte
		want    EBSMetrics
		wantErr string
	}{
		{
			name: "valid log page",
			input: func() []byte {
				metrics := EBSMetrics{
					EBSMagic:              0x3C23B510,
					ReadOps:               100,
					WriteOps:              200,
					ReadBytes:             1024,
					WriteBytes:            2048,
					TotalReadTime:         5000,
					TotalWriteTime:        6000,
					EBSIOPSExceeded:       10,
					EBSThroughputExceeded: 20,
				}
				buf := new(bytes.Buffer)
				if err := binary.Write(buf, binary.LittleEndian, metrics); err != nil {
					t.Fatalf("failed to create test data: %v", err)
				}
				return buf.Bytes()
			}(),
			want: EBSMetrics{
				EBSMagic:              0x3C23B510,
				ReadOps:               100,
				WriteOps:              200,
				ReadBytes:             1024,
				WriteBytes:            2048,
				TotalReadTime:         5000,
				TotalWriteTime:        6000,
				EBSIOPSExceeded:       10,
				EBSThroughputExceeded: 20,
			},
			wantErr: "",
		},
		{
			name: "invalid magic number",
			input: func() []byte {
				metrics := EBSMetrics{
					EBSMagic: 0x12345678,
				}
				buf := new(bytes.Buffer)
				if err := binary.Write(buf, binary.LittleEndian, metrics); err != nil {
					t.Fatalf("failed to create test data: %v", err)
				}
				return buf.Bytes()
			}(),
			want:    EBSMetrics{},
			wantErr: ErrInvalidEBSMagic.Error(),
		},
		{
			name:    "empty data",
			input:   []byte{},
			want:    EBSMetrics{},
			wantErr: ErrParseLogPage.Error(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseLogPage(tt.input)

			if tt.wantErr != "" {
				if err == nil {
					t.Errorf("parseLogPage() error = nil, wantErr %v", tt.wantErr)
					return
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Errorf("parseLogPage() error = %v, wantErr %v", err, tt.wantErr)
					return
				}
				return
			}

			if err != nil {
				t.Errorf("parseLogPage() unexpected error = %v", err)
				return
			}

			if got.EBSMagic != tt.want.EBSMagic {
				t.Errorf("parseLogPage() magic number = %x, want %x", got.EBSMagic, tt.want.EBSMagic)
			}
			if got.ReadOps != tt.want.ReadOps {
				t.Errorf("parseLogPage() ReadOps = %v, want %v", got.ReadOps, tt.want.ReadOps)
			}
			if got.WriteOps != tt.want.WriteOps {
				t.Errorf("parseLogPage() WriteOps = %v, want %v", got.WriteOps, tt.want.WriteOps)
			}
			if got.ReadBytes != tt.want.ReadBytes {
				t.Errorf("parseLogPage() ReadBytes = %v, want %v", got.ReadBytes, tt.want.ReadBytes)
			}
			if got.WriteBytes != tt.want.WriteBytes {
				t.Errorf("parseLogPage() WriteBytes = %v, want %v", got.WriteBytes, tt.want.WriteBytes)
			}
		})
	}
}

func TestMapDevicePathsToVolumeIDs(t *testing.T) {
	tests := []struct {
		name        string
		devicePaths []string
		lsblkOutput []byte
		want        map[string]string
		wantErr     bool
	}{
		{
			name: "standard device mapping",
			devicePaths: []string{
				"/dev/nvme1n1",
				"/dev/nvme2n1",
			},
			lsblkOutput: func() []byte {
				data := LsblkOutput{
					BlockDevices: []BlockDevice{
						{Name: "nvme1n1", Serial: "vol-123456789"},
						{Name: "nvme2n1", Serial: "vol-987654321"},
					},
				}
				b, err := json.Marshal(data)
				if err != nil {
					t.Fatalf("failed to create test data: %v", err)
				}
				return b
			}(),
			want: map[string]string{
				"/dev/nvme1n1": "vol-123456789",
				"/dev/nvme2n1": "vol-987654321",
			},
			wantErr: false,
		},
		{
			name: "device without hyphen",
			devicePaths: []string{
				"/dev/nvme1n1",
			},
			lsblkOutput: func() []byte {
				data := LsblkOutput{
					BlockDevices: []BlockDevice{
						{Name: "nvme1n1", Serial: "vol123456789"},
					},
				}
				b, err := json.Marshal(data)
				if err != nil {
					t.Fatalf("failed to create test data: %v", err)
				}
				return b
			}(),
			want: map[string]string{
				"/dev/nvme1n1": "vol-123456789",
			},
			wantErr: false,
		},
		{
			name:        "empty device paths",
			devicePaths: []string{},
			lsblkOutput: func() []byte {
				data := LsblkOutput{
					BlockDevices: []BlockDevice{
						{Name: "nvme1n1", Serial: "vol-123456789"},
					},
				}
				b, err := json.Marshal(data)
				if err != nil {
					t.Fatalf("failed to create test data: %v", err)
				}
				return b
			}(),
			want:    map[string]string{},
			wantErr: false,
		},
		{
			name:        "invalid json",
			devicePaths: []string{"/dev/nvme1n1"},
			lsblkOutput: []byte(`invalid json`),
			want:        nil,
			wantErr:     true,
		},
		{
			name: "no matching devices",
			devicePaths: []string{
				"/dev/nvme3n1",
			},
			lsblkOutput: func() []byte {
				data := LsblkOutput{
					BlockDevices: []BlockDevice{
						{Name: "nvme1n1", Serial: "vol-123456789"},
					},
				}
				b, err := json.Marshal(data)
				if err != nil {
					t.Fatalf("failed to create test data: %v", err)
				}
				return b
			}(),
			want:    map[string]string{},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := mapDevicePathsToVolumeIDs(tt.devicePaths, tt.lsblkOutput)

			if (err != nil) != tt.wantErr {
				t.Errorf("mapDevicePathsToVolumeIDs() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("mapDevicePathsToVolumeIDs() = %v, want %v", got, tt.want)
			}
		})
	}
}
