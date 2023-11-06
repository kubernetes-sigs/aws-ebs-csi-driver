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
	volumeId string
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

					volumeId: "random-vol-name",
					expResp:  true,
				},
			},
		},
		{
			name: "success adding request with different volumeId",
			requests: []testRequest{
				{
					volumeId: "random-vol-foobar",
					expResp:  true,
				},
				{
					volumeId: "random-vol-name-foobar",
					expResp:  true,
				},
			},
		},
		{
			name: "failed adding request with same volumeId",
			requests: []testRequest{
				{
					volumeId: "random-vol-name-foobar",
					expResp:  true,
				},
				{
					volumeId: "random-vol-name-foobar",
					expResp:  false,
				},
			},
		},

		{
			name: "success add, delete, add copy",
			requests: []testRequest{
				{
					volumeId: "random-vol-name",
					extra:    "random-node-id",
					expResp:  true,
				},
				{
					volumeId: "random-vol-name",
					extra:    "random-node-id",
					expResp:  false,
					delete:   true,
				},
				{
					volumeId: "random-vol-name",
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
					db.Delete(r.volumeId)
				} else {
					resp = db.Insert(r.volumeId)
				}
				if r.expResp != resp {
					t.Fatalf("expected insert to be %+v, got %+v", r.expResp, resp)
				}
			}
		})

	}
}
