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

package coalescer

import (
	"errors"
	"fmt"
	"testing"
	"time"
)

var (
	errFailedToMerge   = fmt.Errorf("Failed to merge")
	errFailedToExecute = fmt.Errorf("Failed to execute")
)

// Merge function used to test the coalescer
// For testing purposes, positive numbers are added to the existing input,
// and negative numbers return an error ("fail to merge")
func mockMerge(input int, existing int) (int, error) {
	if input < 0 {
		return 0, errFailedToMerge
	} else {
		return input + existing, nil
	}
}

// Execute function used to test the coalescer
// For testing purposes, small numbers (numbers less than 100) successfully execute,
// and large numbers (numbers 100 or greater) fail to execute
func mockExecute(_ string, input int) (string, error) {
	if input < 100 {
		return "success", nil
	} else {
		return "failure", errFailedToExecute
	}
}

func TestCoalescer(t *testing.T) {
	testCases := []struct {
		name                 string
		inputs               []int
		expectMergeFailure   bool
		expectExecuteFailure bool
	}{
		{
			name:   "one input",
			inputs: []int{42},
		},
		{
			name:   "many inputs",
			inputs: []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10},
		},
		{
			name:               "failed merge",
			inputs:             []int{1, -2, 3, -4, 5, -6, 7, -8, 9, -10},
			expectMergeFailure: true,
		},
		{
			name:                 "failed execute",
			inputs:               []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 100},
			expectExecuteFailure: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			c := New[int, string](50*time.Millisecond, mockMerge, mockExecute)
			testChannel := make(chan error)

			for _, i := range tc.inputs {
				go func() {
					_, err := c.Coalesce("testKey", i)
					testChannel <- err
				}()
			}

			mergeFailure := false
			executeFailure := false
			for range tc.inputs {
				err := <-testChannel
				if err != nil {
					if errors.Is(err, errFailedToMerge) {
						mergeFailure = true
					} else if errors.Is(err, errFailedToExecute) {
						executeFailure = true
					} else {
						t.Fatalf("Unexpected error %v", err)
					}
				}
			}

			if mergeFailure != tc.expectMergeFailure {
				if tc.expectMergeFailure {
					t.Fatalf("Expected to observe merge failure, did not")
				} else {
					t.Fatalf("Observed unexpected merge failure")
				}
			}
			if executeFailure != tc.expectExecuteFailure {
				if tc.expectExecuteFailure {
					t.Fatalf("Expected to observe execute failure, did not")
				} else {
					t.Fatalf("Observed unexpected execute failure")
				}
			}
		})
	}
}
