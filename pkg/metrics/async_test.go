// Copyright 2025 The Kubernetes Authors.
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
	"time"

	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/prometheus/common/expfmt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TODO: Replace sleep with time-based tests once Go testing/synctest stable in Go 1.25.
const windowsRequiredSleepDuration = time.Millisecond * 75

func TestAsyncCollector(t *testing.T) {
	t.Parallel()

	// Setup env
	recorder := InitializeRecorder(false)
	recorder.InitializeAsyncEC2Metrics(0)
	reg := recorder.registry
	a := assert.New(t)
	req := require.New(t)

	// Ensure no metrics exist on startup
	a.Equal(0, testutil.CollectAndCount(reg, metricAsyncDetachSeconds))

	// Track 3 async detachments
	AsyncEC2Metrics().TrackDetachment("vol-a", "i-a", types.VolumeAttachmentStateDetaching)
	AsyncEC2Metrics().TrackDetachment("vol-b", "i-b", types.VolumeAttachmentStateDetaching)
	AsyncEC2Metrics().TrackDetachment("vol-c", "i-a", types.VolumeAttachmentStateDetaching)
	time.Sleep(windowsRequiredSleepDuration)

	// Validate metrics
	metrics, err := testutil.CollectAndFormat(reg, expfmt.TypeTextPlain, metricAsyncDetachSeconds)
	req.NoError(err)
	splitmetrics := strings.Split(strings.ReplaceAll(string(metrics), "\r\n", "\n"), "\n") // Windows...

	a.Equal(3, testutil.CollectAndCount(reg, metricAsyncDetachSeconds))
	assertSomeMetricHasLabels(a, splitmetrics, []string{"vol-a", "i-a", string(types.VolumeAttachmentStateDetaching)})
	assertSomeMetricHasLabels(a, splitmetrics, []string{"vol-b", "i-b", string(types.VolumeAttachmentStateDetaching)})
	assertSomeMetricHasLabels(a, splitmetrics, []string{"vol-c", "i-a", string(types.VolumeAttachmentStateDetaching)})

	// Lint all metrics
	lint, err := testutil.GatherAndLint(reg)
	req.NoError(err)
	a.Empty(lint)

	// Update 2 detachments
	AsyncEC2Metrics().TrackDetachment("vol-a", "i-a", types.VolumeAttachmentStateBusy)
	AsyncEC2Metrics().TrackDetachment("vol-c", "i-a", types.VolumeAttachmentStateDetaching)

	// Clear 1 detachment
	AsyncEC2Metrics().ClearDetachMetric("vol-b", "i-b")
	time.Sleep(windowsRequiredSleepDuration)

	// Validate metrics
	metrics, err = testutil.CollectAndFormat(reg, expfmt.TypeTextPlain, metricAsyncDetachSeconds)
	req.NoError(err)
	splitmetrics = strings.Split(strings.ReplaceAll(string(metrics), "\r\n", "\n"), "\n") // Windows...

	a.Equal(2, testutil.CollectAndCount(reg, metricAsyncDetachSeconds))
	assertSomeMetricHasLabels(a, splitmetrics, []string{"vol-a", "i-a", string(types.VolumeAttachmentStateBusy)})
	assertSomeMetricHasLabels(a, splitmetrics, []string{"vol-c", "i-a", string(types.VolumeAttachmentStateDetaching)})

	assertNoMetricHasLabels(a, splitmetrics, []string{"vol-b", "i-b"})

	// Test cleanup helper
	AsyncEC2Metrics().cleanupCache(0)
	time.Sleep(windowsRequiredSleepDuration)

	a.Empty(AsyncEC2Metrics().detachingVolumes)
	a.Equal(0, testutil.CollectAndCount(reg, metricAsyncDetachSeconds))
}

func assertSomeMetricHasLabels(assert *assert.Assertions, metrics, labels []string) {
	assert.False(noMetricWithLabels(metrics, labels), "AsyncEC2Metrics are missing a metric with expected labels")
}

func assertNoMetricHasLabels(assert *assert.Assertions, metrics, labels []string) {
	assert.True(noMetricWithLabels(metrics, labels), "AsyncEC2Metrics has a metric that shouldn't exist")
}

func noMetricWithLabels(metrics, labels []string) bool {
	for _, m := range metrics {
		b := true
		for _, l := range labels {
			if !strings.Contains(m, l) {
				b = false
			}
		}
		if b {
			return false
		}
	}
	return true
}
