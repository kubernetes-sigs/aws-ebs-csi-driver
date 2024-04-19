package metrics

import (
	"net/http"
	"sync"
	"time"

	"k8s.io/component-base/metrics"
	"k8s.io/klog/v2"
)

var (
	r    *metricRecorder // singleton instance of metricRecorder
	once sync.Once
)

type metricRecorder struct {
	registry metrics.KubeRegistry
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
			registry: metrics.NewKubeRegistry(),
			metrics:  make(map[string]interface{}),
		}
	})
	return r
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

	metric.(*metrics.CounterVec).With(metrics.Labels(labels)).Inc()
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

	metric.(*metrics.HistogramVec).With(metrics.Labels(labels)).Observe(value)
}

// InitializeMetricsHandler starts a new HTTP server to expose the metrics.
func (m *metricRecorder) InitializeMetricsHandler(address, path, certFile, keyFile string) {
	if m == nil {
		klog.InfoS("InitializeMetricsHandler: metric recorder is not initialized")
		return
	}

	mux := http.NewServeMux()
	mux.Handle(path, metrics.HandlerFor(
		m.registry,
		metrics.HandlerOpts{
			ErrorHandling: metrics.ContinueOnError,
		}))

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
	histogram := createHistogramVec(name, help, labels, buckets)
	m.metrics[name] = histogram
	m.registry.MustRegister(histogram)
}

func (m *metricRecorder) registerCounterVec(name, help string, labels []string) {
	if _, exists := m.metrics[name]; exists {
		return
	}
	counter := createCounterVec(name, help, labels)
	m.metrics[name] = counter
	m.registry.MustRegister(counter)
}

func createHistogramVec(name, help string, labels []string, buckets []float64) *metrics.HistogramVec {
	opts := &metrics.HistogramOpts{
		Name:           name,
		Help:           help,
		StabilityLevel: metrics.ALPHA,
		Buckets:        buckets,
	}
	return metrics.NewHistogramVec(opts, labels)
}

func createCounterVec(name, help string, labels []string) *metrics.CounterVec {
	return metrics.NewCounterVec(
		&metrics.CounterOpts{
			Name:           name,
			Help:           help,
			StabilityLevel: metrics.ALPHA,
		},
		labels,
	)
}

func getLabelNames(labels map[string]string) []string {
	names := make([]string, 0, len(labels))
	for n := range labels {
		names = append(names, n)
	}
	return names
}
