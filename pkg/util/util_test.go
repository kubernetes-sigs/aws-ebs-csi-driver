//go:build linux
// +build linux

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

package util

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"

	csi "github.com/container-storage-interface/spec/lib/go/csi"
)

func TestRoundUpBytes(t *testing.T) {
	var sizeInBytes int64 = 1024
	actual := RoundUpBytes(sizeInBytes)
	if actual != 1*GiB {
		t.Fatalf("Wrong result for RoundUpBytes. Got: %d, want: %d", actual, 1*GiB)
	}
}

func TestRoundUpGiB(t *testing.T) {
	var size int64 = 1
	actual, err := RoundUpGiB(size)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if actual != 1 {
		t.Fatalf("Wrong result for RoundUpGiB. Got: %d, want: %d", actual, 1)
	}
}

func TestBytesToGiB(t *testing.T) {
	var sizeInBytes int64 = 2147483643
	actual := BytesToGiB(sizeInBytes)
	expected := int32(sizeInBytes / GiB)
	if actual != expected {
		t.Fatalf("Wrong result for BytesToGiB. Got: %d, want: %d", actual, expected)
	}
}

func TestGiBToBytes(t *testing.T) {
	var sizeInGiB int32 = 3

	actual := GiBToBytes(sizeInGiB)
	if actual != 3*GiB {
		t.Fatalf("Wrong result for GiBToBytes. Got: %d", actual)
	}
}

func TestParseEndpoint(t *testing.T) {
	testCases := []struct {
		name      string
		endpoint  string
		expScheme string
		expAddr   string
		expErr    error
	}{
		{
			name:      "valid unix endpoint 1",
			endpoint:  "unix:///csi/csi.sock",
			expScheme: "unix",
			expAddr:   "/csi/csi.sock",
		},
		{
			name:      "valid unix endpoint 2",
			endpoint:  "unix://csi/csi.sock",
			expScheme: "unix",
			expAddr:   "/csi/csi.sock",
		},
		{
			name:      "valid unix endpoint 3",
			endpoint:  "unix:/csi/csi.sock",
			expScheme: "unix",
			expAddr:   "/csi/csi.sock",
		},
		{
			name:      "valid tcp endpoint",
			endpoint:  "tcp:///127.0.0.1/",
			expScheme: "tcp",
			expAddr:   "/127.0.0.1",
		},
		{
			name:      "valid tcp endpoint",
			endpoint:  "tcp:///127.0.0.1",
			expScheme: "tcp",
			expAddr:   "/127.0.0.1",
		},
		{
			name:     "invalid endpoint",
			endpoint: "http://127.0.0.1",
			expErr:   fmt.Errorf("unsupported protocol: http"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			scheme, addr, err := ParseEndpoint(tc.endpoint, false)

			if tc.expErr != nil {
				if err.Error() != tc.expErr.Error() {
					t.Fatalf("Expecting err: expected %v, got %v", tc.expErr, err)
				}

			} else {
				if err != nil {
					t.Fatalf("err is not nil. got: %v", err)
				}
				if scheme != tc.expScheme {
					t.Fatalf("scheme mismatches: expected %v, got %v", tc.expScheme, scheme)
				}

				if addr != tc.expAddr {
					t.Fatalf("addr mismatches: expected %v, got %v", tc.expAddr, addr)
				}
			}
		})
	}

}

func TestGetAccessModes(t *testing.T) {
	testVolCap := []*csi.VolumeCapability{
		{
			AccessMode: &csi.VolumeCapability_AccessMode{
				Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER,
			},
		},
		{
			AccessMode: &csi.VolumeCapability_AccessMode{
				Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_READER_ONLY,
			},
		},
	}
	expectedModes := []string{
		"SINGLE_NODE_WRITER",
		"SINGLE_NODE_READER_ONLY",
	}
	actualModes := GetAccessModes(testVolCap)
	if !reflect.DeepEqual(expectedModes, *actualModes) {
		t.Fatalf("Wrong values returned for volume capabilities. Expected %v, got %v", expectedModes, actualModes)
	}
}

func TestIsAlphanumeric(t *testing.T) {
	testCases := []struct {
		name       string
		testString string
		expResult  bool
	}{
		{
			name:       "success with alphanumeric",
			testString: "4Kib",
			expResult:  true,
		},
		{
			name:       "failure with non-alphanumeric",
			testString: "space 4Kib",
			expResult:  false,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			res := StringIsAlphanumeric(tc.testString)
			assert.Equalf(t, tc.expResult, res, "Wrong value returned for StringIsAlphanumeric. Expected %s for string %s, got %s", tc.expResult, tc.testString, res)
		})
	}
}

type TestRequest struct {
	Name    string
	Secrets map[string]string
}

func TestSanitizeRequest(t *testing.T) {
	tests := []struct {
		name     string
		req      interface{}
		expected interface{}
	}{
		{
			name: "Request with Secrets",
			req: &TestRequest{
				Name: "Test",
				Secrets: map[string]string{
					"key1": "value1",
					"key2": "value2",
				},
			},
			expected: &TestRequest{
				Name:    "Test",
				Secrets: map[string]string{},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := SanitizeRequest(tt.req)
			if !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("SanitizeRequest() = %v, expected %v", result, tt.expected)
			}
		})
	}
}
