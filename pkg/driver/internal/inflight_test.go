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

package internal

import (
	"testing"
)

type testRequest struct {
	volumeID string
	extra    string
	expResp  bool
	delete   bool
}

func TestInFlight(t *testing.T) {
	testCases := []struct {
		name     string
		requests []testRequest
	}{
		{
			name: "success normal",
			requests: []testRequest{
				{

					volumeID: "random-vol-name",
					expResp:  true,
				},
			},
		},
		{
			name: "success adding request with different volumeID",
			requests: []testRequest{
				{
					volumeID: "random-vol-foobar",
					expResp:  true,
				},
				{
					volumeID: "random-vol-name-foobar",
					expResp:  true,
				},
			},
		},
		{
			name: "failed adding request with same volumeID",
			requests: []testRequest{
				{
					volumeID: "random-vol-name-foobar",
					expResp:  true,
				},
				{
					volumeID: "random-vol-name-foobar",
					expResp:  false,
				},
			},
		},
		{
			name: "success add, delete, add copy",
			requests: []testRequest{
				{
					volumeID: "random-vol-name",
					extra:    "random-node-id",
					expResp:  true,
				},
				{
					volumeID: "random-vol-name",
					extra:    "random-node-id",
					expResp:  false,
					delete:   true,
				},
				{
					volumeID: "random-vol-name",
					extra:    "random-node-id",
					expResp:  true,
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			db := NewInFlight()
			for _, r := range tc.requests {
				var resp bool
				if r.delete {
					db.Delete(r.volumeID)
				} else {
					resp = db.Insert(r.volumeID)
				}
				if r.expResp != resp {
					t.Fatalf("expected insert to be %+v, got %+v", r.expResp, resp)
				}
			}
		})
	}
}
