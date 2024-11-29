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
	"net/http"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"k8s.io/klog/v2"
)

const (
	namespace = "aws_ebs_csi_"
)

var (
	r    *metricRecorder // singleton instance of metricRecorder
	once sync.Once
)

type metricRecorder struct {
	registry *prometheus.Registry
	metrics  map[string]interface{}
}

// Recorder returns the singleton instance of metricRecorder.
// nil is returned if the recorder is not initialized.
func Recorder() *metricRecorder {
	return r
}

// InitializeRecorder initializes a new metricRecorder instance if it hasn't been initialized.
func InitializeRecorder() *metricRecorder {
	once.Do(func() {
		r = &metricRecorder{
			registry: prometheus.NewRegistry(),
			metrics:  make(map[string]interface{}),
		}
	})
	return r
}

// InitializeNVME registers the NVMe collector for gathering metrics from NVMe devices.
func InitializeNVME(r *metricRecorder, csiMountPointPath, instanceID string) {
	registerNVMECollector(r, csiMountPointPath, instanceID)
}

// IncreaseCount increases the counter metric by 1.
func (m *metricRecorder) IncreaseCount(name string, labels map[string]string) {
	if m == nil {
		return // recorder is not initialized
	}

	metric, ok := m.metrics[name]

	if !ok {
		klog.V(4).InfoS("Metric not found, registering", "name", name, "labels", labels)
		m.registerCounterVec(name, "ebs_csi_aws_com metric", getLabelNames(labels))
		m.IncreaseCount(name, labels)
		return
	}

	metricAsCounterVec, ok := metric.(*prometheus.CounterVec)
	if ok {
		metricAsCounterVec.With(labels).Inc()
	} else {
		klog.V(4).InfoS("Could not assert metric as metrics.CounterVec. Metric increase may have been skipped")
	}
}

// ObserveHistogram records the given value in the histogram metric.
func (m *metricRecorder) ObserveHistogram(name string, value float64, labels map[string]string, buckets []float64) {
	if m == nil {
		return // recorder is not initialized
	}
	metric, ok := m.metrics[name]

	if !ok {
		klog.V(4).InfoS("Metric not found, registering", "name", name, "labels", labels, "buckets", buckets)
		m.registerHistogramVec(name, "ebs_csi_aws_com metric", getLabelNames(labels), buckets)
		m.ObserveHistogram(name, value, labels, buckets)
		return
	}

	metricAsHistogramVec, ok := metric.(*prometheus.HistogramVec)
	if ok {
		metricAsHistogramVec.With(labels).Observe(value)
	} else {
		klog.V(4).InfoS("Could not assert metric as metrics.HistogramVec. Metric observation may have been skipped")
	}
}

// InitializeMetricsHandler starts a new HTTP server to expose the metrics.
func (m *metricRecorder) InitializeMetricsHandler(address, path, certFile, keyFile string) {
	if m == nil {
		klog.InfoS("InitializeMetricsHandler: metric recorder is not initialized")
		return
	}

	mux := http.NewServeMux()
	mux.Handle(path, promhttp.HandlerFor(m.registry, promhttp.HandlerOpts{ErrorHandling: promhttp.ContinueOnError}))

	server := &http.Server{
		Addr:        address,
		Handler:     mux,
		ReadTimeout: 3 * time.Second,
	}

	go func() {
		var err error
		klog.InfoS("Metric server listening", "address", address, "path", path)

		if certFile != "" {
			err = server.ListenAndServeTLS(certFile, keyFile)
		} else {
			err = server.ListenAndServe()
		}

		if err != nil && err != http.ErrServerClosed {
			klog.ErrorS(err, "Failed to start metric server", "address", address, "path", path)
			klog.FlushAndExit(klog.ExitFlushTimeout, 1)
		}
	}()
}

func (m *metricRecorder) registerHistogramVec(name, help string, labels []string, buckets []float64) {
	if _, exists := m.metrics[name]; exists {
		return
	}
	histogram := prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    name,
			Help:    help,
			Buckets: buckets,
		},
		labels,
	)
	m.metrics[name] = histogram
	m.registry.MustRegister(histogram)
}

func (m *metricRecorder) registerCounterVec(name, help string, labels []string) {
	if _, exists := m.metrics[name]; exists {
		return
	}
	counter := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: name,
			Help: help,
		},
		labels,
	)
	m.metrics[name] = counter
	m.registry.MustRegister(counter)
}

func getLabelNames(labels map[string]string) []string {
	names := make([]string, 0, len(labels))
	for n := range labels {
		names = append(names, n)
	}
	return names
}
