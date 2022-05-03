package template

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestEvaluate(t *testing.T) {
	testCases := []struct {
		name         string
		input        []string
		pvcName      string
		pvName       string
		pvcNamespace string
		warnOnly     bool
		expectErr    bool
		expectedTags map[string]string
	}{
		{
			name:         "empty input",
			expectedTags: make(map[string]string),
		},
		{
			name: "no interpolation",
			input: []string{
				"key1=value1",
				"key2=hello world",
			},
			expectedTags: map[string]string{
				"key1": "value1",
				"key2": "hello world",
			},
		},
		{
			name: "no tag values gives empty string",
			input: []string{
				"key1=",
			},
			expectedTags: map[string]string{
				"key1": "",
			},
		},
		{
			name: "no = returns an error",
			input: []string{
				"key1",
			},
			expectErr: true,
		},
		{
			name: "simple substitution",
			input: []string{
				"key1={{ .PVCName }}",
				"key2={{ .PVCNamespace }}",
				"key3={{ .PVName }}",
			},
			pvcName:      "ebs-claim",
			pvcNamespace: "default",
			pvName:       "ebs-claim-012345",
			expectedTags: map[string]string{
				"key1": "ebs-claim",
				"key2": "default",
				"key3": "ebs-claim-012345",
			},
		},
		{
			name: "template parsing error",
			input: []string{
				"key1={{ .PVCName }",
			},
			expectErr: true,
		},
		{
			name: "template parsing error warn only",
			input: []string{
				"key1={{ .PVCName }",
				"key2={{ .PVCNamespace }}",
			},
			pvcName:      "ebs-claim",
			pvcNamespace: "default",
			warnOnly:     true,
			expectedTags: map[string]string{
				"key2": "default",
			},
		},
		{
			name: "test function - html - returns error",
			input: []string{
				`backup={{ .PVCNamespace | html }}`,
			},
			pvcNamespace: "ns-prod",
			expectErr:    true,
		},
		{
			name: "test func - js - returns error",
			input: []string{
				`backup={{ .PVCNamespace | js }}`,
			},
			pvcNamespace: "ns-prod",
			expectErr:    true,
		},
		{
			name: "test func - call - returns error",
			input: []string{
				`backup={{ .PVCNamespace | call }}`,
			},
			pvcNamespace: "ns-prod",
			expectErr:    true,
		},
		{
			name: "test func - urlquery - returns error",
			input: []string{
				`backup={{ .PVCNamespace | urlquery }}`,
			},
			pvcNamespace: "ns-prod",
			expectErr:    true,
		},
		{
			name: "test function - contains",
			input: []string{
				`backup={{ .PVCNamespace | contains "prod" }}`,
			},
			pvcNamespace: "ns-prod",
			expectedTags: map[string]string{
				"backup": "true",
			},
		},
		{
			name: "test function - toUpper",
			input: []string{
				`backup={{ .PVCNamespace | toUpper }}`,
			},
			pvcNamespace: "ns-prod",
			expectedTags: map[string]string{
				"backup": "NS-PROD",
			},
		},
		{
			name: "test function - toLower",
			input: []string{
				`backup={{ .PVCNamespace | toLower }}`,
			},
			pvcNamespace: "ns-PROD",
			expectedTags: map[string]string{
				"backup": "ns-prod",
			},
		},
		{
			name: "test function - field",
			input: []string{
				`backup={{ .PVCNamespace | field "-" 1 }}`,
			},
			pvcNamespace: "ns-prod-default",
			expectedTags: map[string]string{
				"backup": "prod",
			},
		},
		{
			name: "test function - substring",
			input: []string{
				`key1={{ .PVCNamespace | substring 0 4 }}`,
			},
			pvcNamespace: "prod-12345",
			expectedTags: map[string]string{
				"key1": "prod",
			},
		},
		{
			name: "no extra-create-metadata flag",
			input: []string{
				`key1={{ .PVCNamespace }}`,
				`key2={{ .PVCName }}`,
			},
			expectedTags: map[string]string{
				"key1": "",
				"key2": "",
			},
		},
		{
			name: "field returns error",
			input: []string{
				`key1={{ .PVCNamespace | field "-" 1 }}`,
			},
			expectErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {

			props := &Props{
				PVCName:      tc.pvcName,
				PVCNamespace: tc.pvcNamespace,
				PVName:       tc.pvName,
			}

			tags, err := Evaluate(tc.input, props, tc.warnOnly)

			if tc.expectErr {
				if err == nil {
					t.Fatalf("expected an error; got nil")
				}
			} else {
				if err != nil {
					t.Fatalf("err is not nil; err = %v", err)
				}
				if diff := cmp.Diff(tc.expectedTags, tags); diff != "" {
					t.Fatalf("tags are different; diff = %v", diff)
				}
			}
		})
	}
}
