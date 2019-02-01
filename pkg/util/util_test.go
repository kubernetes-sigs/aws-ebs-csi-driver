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

package util

import (
	"fmt"
	"testing"
)

func TestRoundUpBytes(t *testing.T) {
	var sizeInBytes int64 = 1024
	actual := RoundUpBytes(sizeInBytes)
	if actual != 1*GiB {
		t.Fatalf("Wrong result for RoundUpBytes. Got: %d", actual)
	}
}

func TestRoundUpGiB(t *testing.T) {
	var sizeInBytes int64 = 1
	actual := RoundUpGiB(sizeInBytes)
	if actual != 1 {
		t.Fatalf("Wrong result for RoundUpGiB. Got: %d", actual)
	}
}

func TestBytesToGiB(t *testing.T) {
	var sizeInBytes int64 = 5 * GiB

	actual := BytesToGiB(sizeInBytes)
	if actual != 5 {
		t.Fatalf("Wrong result for BytesToGiB. Got: %d", actual)
	}
}

func TestGiBToBytes(t *testing.T) {
	var sizeInGiB int64 = 3

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
			scheme, addr, err := ParseEndpoint(tc.endpoint)

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
