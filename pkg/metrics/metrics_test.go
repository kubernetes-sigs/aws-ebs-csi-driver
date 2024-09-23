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

package metrics

import (
	"strings"
	"testing"

	"k8s.io/component-base/metrics/testutil"
)

func TestMetricRecorder(t *testing.T) {
	tests := []struct {
		name     string
		exec     func(m *metricRecorder)
		expected string
		recorder bool
	}{
		{
			name: "TestMetricRecorder: IncreaseCounterMetric",
			exec: func(m *metricRecorder) {
				m.IncreaseCount("test_total", map[string]string{"key": "value"})
			},
			expected: `
# HELP test_total [ALPHA] ebs_csi_aws_com metric
# TYPE test_total counter
test_total{key="value"} 1
			`,
			recorder: true,
		},
		{
			name: "TestMetricRecorder: ObserveHistogramMetric",
			exec: func(m *metricRecorder) {
				m.ObserveHistogram("test", 1.5, map[string]string{"key": "value"}, []float64{1, 2, 3})
			},
			expected: `
# HELP test [ALPHA] ebs_csi_aws_com metric
# TYPE test histogram
test{key="value",le="1"} 0
test{key="value",le="2"} 1
test{key="value",le="3"} 1
test_bucket{key="value",le="+Inf"} 1
test_sum{key="value"} 1.5
test_count{key="value"} 1
			`,
			recorder: true,
		},
		{
			name: "TestMetricRecorder: Re-register metric",
			exec: func(m *metricRecorder) {
				m.IncreaseCount("test_re_register_total", map[string]string{"key": "value1"})
				m.registerCounterVec("test_re_register_total", "ebs_csi_aws_com metric", []string{"key"})
				m.IncreaseCount("test_re_register_total", map[string]string{"key": "value1"})
				m.IncreaseCount("test_re_register_total", map[string]string{"key": "value2"})
			},
			expected: `
# HELP test_re_register_total [ALPHA] ebs_csi_aws_com metric
# TYPE test_re_register_total counter
test_re_register_total{key="value1"} 2
test_re_register_total{key="value2"} 1
			`,
			recorder: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.recorder {
				InitializeRecorder()
			}
			m := Recorder()

			tt.exec(m)

			if err := testutil.GatherAndCompare(m.registry, strings.NewReader(tt.expected), getMetricNameFromExpected(tt.expected)); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func getMetricNameFromExpected(expected string) string {
	lines := strings.Split(expected, "\n")
	for _, line := range lines {
		if strings.Contains(line, "{") {
			return strings.Split(line, "{")[0]
		}
	}
	return ""
}
