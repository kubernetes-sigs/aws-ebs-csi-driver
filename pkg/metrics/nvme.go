//go:build linux
// +build linux

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
	"bytes"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"os/exec"
	"strings"
	"time"
	"unsafe"

	"github.com/prometheus/client_golang/prometheus"
	"golang.org/x/sys/unix"
	"k8s.io/klog/v2"
)

// EBSMetrics represents the parsed metrics from the NVMe log page.
type EBSMetrics struct {
	EBSMagic              uint64
	ReadOps               uint64
	WriteOps              uint64
	ReadBytes             uint64
	WriteBytes            uint64
	TotalReadTime         uint64
	TotalWriteTime        uint64
	EBSIOPSExceeded       uint64
	EBSThroughputExceeded uint64
	EC2IOPSExceeded       uint64
	EC2ThroughputExceeded uint64
	QueueLength           uint64
	ReservedArea          [416]byte
	ReadLatency           Histogram
	WriteLatency          Histogram
}

type Histogram struct {
	BinCount uint64
	Bins     [64]HistogramBin
}

type HistogramBin struct {
	Lower uint64
	Upper uint64
	Count uint64
}

// As defined in <linux/nvme_ioctl.h>.
type nvmePassthruCommand struct {
	opcode      uint8
	flags       uint8
	rsvd1       uint16
	nsid        uint32
	cdw2        uint32
	cdw3        uint32
	metadata    uint64
	addr        uint64
	metadataLen uint32
	dataLen     uint32
	cdw10       uint32
	cdw11       uint32
	cdw12       uint32
	cdw13       uint32
	cdw14       uint32
	cdw15       uint32
	timeoutMs   uint32
	result      uint32
}

type NVMECollector struct {
	metrics            map[string]*prometheus.Desc
	csiMountPointPath  string
	instanceID         string
	collectionDuration prometheus.Histogram
	scrapesTotal       prometheus.Counter
	scrapeErrorsTotal  prometheus.Counter
}

var (
	ErrInvalidEBSMagic = errors.New("invalid EBS magic number")
	ErrParseLogPage    = errors.New("failed to parse log page")
)

// NewNVMECollector creates a new instance of NVMECollector.
func NewNVMECollector(path, instanceID string) *NVMECollector {
	variableLabels := []string{"volume_id"}
	constLabels := prometheus.Labels{"instance_id": instanceID}

	return &NVMECollector{
		metrics: map[string]*prometheus.Desc{
			"total_read_ops":                             prometheus.NewDesc("total_read_ops", "Total number of read operations", variableLabels, constLabels),
			"total_write_ops":                            prometheus.NewDesc("total_write_ops", "Total number of write operations", variableLabels, constLabels),
			"total_read_bytes":                           prometheus.NewDesc("total_read_bytes", "Total number of bytes read", variableLabels, constLabels),
			"total_write_bytes":                          prometheus.NewDesc("total_write_bytes", "Total number of bytes written", variableLabels, constLabels),
			"total_read_time":                            prometheus.NewDesc("total_read_time", "Total time spent on read operations (in microseconds)", variableLabels, constLabels),
			"total_write_time":                           prometheus.NewDesc("total_write_time", "Total time spent on write operations (in microseconds)", variableLabels, constLabels),
			"ebs_volume_performance_exceeded_iops":       prometheus.NewDesc("ebs_volume_performance_exceeded_iops", "Time EBS volume IOPS limit was exceeded (in microseconds)", variableLabels, constLabels),
			"ebs_volume_performance_exceeded_tp":         prometheus.NewDesc("ebs_volume_performance_exceeded_tp", "Time EBS volume throughput limit was exceeded (in microseconds)", variableLabels, constLabels),
			"ec2_instance_ebs_performance_exceeded_iops": prometheus.NewDesc("ec2_instance_ebs_performance_exceeded_iops", "Time EC2 instance EBS IOPS limit was exceeded (in microseconds)", variableLabels, constLabels),
			"ec2_instance_ebs_performance_exceeded_tp":   prometheus.NewDesc("ec2_instance_ebs_performance_exceeded_tp", "Time EC2 instance EBS throughput limit was exceeded (in microseconds)", variableLabels, constLabels),
			"volume_queue_length":                        prometheus.NewDesc("volume_queue_length", "Current volume queue length", variableLabels, constLabels),
			"read_io_latency_histogram":                  prometheus.NewDesc("read_io_latency_histogram", "Histogram of read I/O latencies (in microseconds)", variableLabels, constLabels),
			"write_io_latency_histogram":                 prometheus.NewDesc("write_io_latency_histogram", "Histogram of write I/O latencies (in microseconds)", variableLabels, constLabels),
		},
		csiMountPointPath: path,
		instanceID:        instanceID,
		collectionDuration: prometheus.NewHistogram(prometheus.HistogramOpts{
			Name:        "nvme_collector_duration_seconds",
			Help:        "Histogram of NVMe collector duration in seconds",
			Buckets:     []float64{0.001, 0.0025, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10},
			ConstLabels: constLabels,
		}),
		scrapesTotal: prometheus.NewCounter(prometheus.CounterOpts{
			Name:        "nvme_collector_scrapes_total",
			Help:        "Total number of NVMe collector scrapes",
			ConstLabels: constLabels,
		}),
		scrapeErrorsTotal: prometheus.NewCounter(prometheus.CounterOpts{
			Name:        "nvme_collector_scrape_errors_total",
			Help:        "Total number of NVMe collector scrape errors",
			ConstLabels: constLabels,
		}),
	}
}

func registerNVMECollector(r *metricRecorder, csiMountPointPath, instanceID string) {
	collector := NewNVMECollector(csiMountPointPath, instanceID)
	r.registry.MustRegister(collector)
}

// Describe sends the descriptor of each metric in the NVMECollector to Prometheus.
func (c *NVMECollector) Describe(ch chan<- *prometheus.Desc) {
	for _, desc := range c.metrics {
		ch <- desc
	}
	ch <- c.collectionDuration.Desc()
	ch <- c.scrapesTotal.Desc()
	ch <- c.scrapeErrorsTotal.Desc()
}

// Collect is invoked by Prometheus at collection time.
func (c *NVMECollector) Collect(ch chan<- prometheus.Metric) {
	c.scrapesTotal.Inc()
	start := time.Now()
	defer func() {
		duration := time.Since(start).Seconds()
		c.collectionDuration.Observe(duration)

		ch <- c.collectionDuration
		ch <- c.scrapesTotal
		ch <- c.scrapeErrorsTotal
	}()

	devicePaths, err := getCSIManagedDevices(c.csiMountPointPath)
	if err != nil {
		klog.Errorf("Error getting NVMe devices: %v", err)
		c.scrapeErrorsTotal.Inc()
		return
	} else if len(devicePaths) == 0 {
		klog.V(8).InfoS("No NVMe devices found")
		return
	}

	devices, err := fetchDevicePathToVolumeIDMapping(devicePaths)
	if err != nil {
		klog.Errorf("Error getting volume IDs: %v", err)
		c.scrapeErrorsTotal.Inc()
		return
	}

	for devicePath, volumeID := range devices {
		data, err := getNVMEMetrics(devicePath)
		if err != nil {
			klog.Errorf("Error collecting metrics for device %s: %v", devicePath, err)
			c.scrapeErrorsTotal.Inc()
			continue
		}

		metrics, err := parseLogPage(data)
		if err != nil {
			klog.Errorf("Error parsing metrics for device %s: %v", devicePath, err)
			c.scrapeErrorsTotal.Inc()
			continue
		}

		// Send all collected metrics to Prometheus
		ch <- prometheus.MustNewConstMetric(c.metrics["total_read_ops"], prometheus.CounterValue, float64(metrics.ReadOps), volumeID)
		ch <- prometheus.MustNewConstMetric(c.metrics["total_write_ops"], prometheus.CounterValue, float64(metrics.WriteOps), volumeID)
		ch <- prometheus.MustNewConstMetric(c.metrics["total_read_bytes"], prometheus.CounterValue, float64(metrics.ReadBytes), volumeID)
		ch <- prometheus.MustNewConstMetric(c.metrics["total_write_bytes"], prometheus.CounterValue, float64(metrics.WriteBytes), volumeID)
		ch <- prometheus.MustNewConstMetric(c.metrics["total_read_time"], prometheus.CounterValue, float64(metrics.TotalReadTime), volumeID)
		ch <- prometheus.MustNewConstMetric(c.metrics["total_write_time"], prometheus.CounterValue, float64(metrics.TotalWriteTime), volumeID)
		ch <- prometheus.MustNewConstMetric(c.metrics["ebs_volume_performance_exceeded_iops"], prometheus.CounterValue, float64(metrics.EBSIOPSExceeded), volumeID)
		ch <- prometheus.MustNewConstMetric(c.metrics["ebs_volume_performance_exceeded_tp"], prometheus.CounterValue, float64(metrics.EBSThroughputExceeded), volumeID)
		ch <- prometheus.MustNewConstMetric(c.metrics["ec2_instance_ebs_performance_exceeded_iops"], prometheus.CounterValue, float64(metrics.EC2IOPSExceeded), volumeID)
		ch <- prometheus.MustNewConstMetric(c.metrics["ec2_instance_ebs_performance_exceeded_tp"], prometheus.CounterValue, float64(metrics.EC2ThroughputExceeded), volumeID)
		ch <- prometheus.MustNewConstMetric(c.metrics["volume_queue_length"], prometheus.GaugeValue, float64(metrics.QueueLength), volumeID)

		// Read Latency Histogram
		readCount, readBuckets := convertHistogram(metrics.ReadLatency)
		ch <- prometheus.MustNewConstHistogram(
			c.metrics["read_io_latency_histogram"],
			readCount,
			0,
			readBuckets,
			volumeID,
		)

		// Write Latency Histogram
		writeCount, writeBuckets := convertHistogram(metrics.WriteLatency)
		ch <- prometheus.MustNewConstHistogram(
			c.metrics["write_io_latency_histogram"],
			writeCount,
			0,
			writeBuckets,
			volumeID,
		)
	}
}

// convertHistogram converts the Histogram structure to a format suitable for Prometheus histogram metrics.
func convertHistogram(hist Histogram) (uint64, map[float64]uint64) {
	var count uint64
	buckets := make(map[float64]uint64)

	for i := uint64(0); i < hist.BinCount && i < 64; i++ {
		count += hist.Bins[i].Count
		buckets[float64(hist.Bins[i].Upper)] = count
	}

	return count, buckets
}

// getNVMEMetrics retrieves NVMe metrics by reading the log page from the NVMe device at the given path.
func getNVMEMetrics(devicePath string) ([]byte, error) {
	f, err := os.OpenFile(devicePath, os.O_RDWR, 0)
	if err != nil {
		return nil, fmt.Errorf("getNVMEMetrics: error opening device: %w", err)
	}
	defer f.Close()

	data, err := nvmeReadLogPage(f.Fd(), 0xD0)
	if err != nil {
		return nil, fmt.Errorf("getNVMEMetrics: error reading log page %w", err)
	}

	return data, nil
}

// nvmeReadLogPage reads an NVMe log page via an ioctl system call.
func nvmeReadLogPage(fd uintptr, logID uint8) ([]byte, error) {
	data := make([]byte, 4096) // 4096 bytes is the length of the log page.
	bufferLen := len(data)

	if bufferLen > math.MaxUint32 {
		return nil, errors.New("nvmeReadLogPage: bufferLen exceeds MaxUint32")
	}

	cmd := nvmePassthruCommand{
		opcode:  0x02,
		addr:    uint64(uintptr(unsafe.Pointer(&data[0]))),
		nsid:    1,
		dataLen: uint32(bufferLen),
		cdw10:   uint32(logID) | (1024 << 16),
	}

	status, _, errno := unix.Syscall(unix.SYS_IOCTL, fd, 0xC0484E41, uintptr(unsafe.Pointer(&cmd)))
	if errno != 0 {
		return nil, fmt.Errorf("nvmeReadLogPage: ioctl error %w", errno)
	}
	if status != 0 {
		return nil, fmt.Errorf("nvmeReadLogPage: ioctl command failed with status %d", status)
	}
	return data, nil
}

// parseLogPage parses the binary data from an EBS log page into EBSMetrics.
func parseLogPage(data []byte) (EBSMetrics, error) {
	var metrics EBSMetrics
	reader := bytes.NewReader(data)

	if err := binary.Read(reader, binary.LittleEndian, &metrics); err != nil {
		return EBSMetrics{}, fmt.Errorf("%w: %w", ErrParseLogPage, err)
	}

	if metrics.EBSMagic != 0x3C23B510 {
		return EBSMetrics{}, fmt.Errorf("%w: %x", ErrInvalidEBSMagic, metrics.EBSMagic)
	}

	return metrics, nil
}

// getCSIManagedDevices returns a slice of unique device paths for NVMe devices mounted under the given path.
func getCSIManagedDevices(path string) ([]string, error) {
	if len(path) == 0 {
		klog.V(4).InfoS("getCSIManagedDevices: empty path provided, no devices will be matched")
		return []string{}, nil
	}

	deviceMap := make(map[string]bool)

	// Read /proc/self/mountinfo to identify NVMe devices
	mountinfo, err := os.ReadFile("/proc/self/mountinfo")
	if err != nil {
		return nil, fmt.Errorf("getCSIManagedDevices: error reading mountinfo: %w", err)
	}

	lines := strings.Split(string(mountinfo), "\n")
	for _, line := range lines {
		fields := strings.Fields(line)

		// https://man7.org/linux/man-pages/man5/proc.5.html
		if len(fields) < 10 {
			continue // Skip lines with insufficient fields
		}

		mountPoint := fields[4]
		if !strings.HasPrefix(mountPoint, path) {
			continue
		}

		// Check mount source (field 3) for directly mounted NVMe devices
		m := fields[3]
		if strings.HasPrefix(m, "/nvme") {
			device := "/dev" + m
			deviceMap[device] = true
		}

		// Check root (field 9) for block devices
		r := fields[9]
		if strings.HasPrefix(r, "/dev/nvme") {
			deviceMap[r] = true
		}
	}

	devices := make([]string, 0, len(deviceMap))
	for device := range deviceMap {
		devices = append(devices, device)
	}

	return devices, nil
}

type BlockDevice struct {
	Name   string `json:"name"`
	Serial string `json:"serial"`
}

type LsblkOutput struct {
	BlockDevices []BlockDevice `json:"blockdevices"`
}

// mapDevicePathsToVolumeIDs takes a list of device paths and lsblk output, and returns a map of device paths to volume IDs.
func mapDevicePathsToVolumeIDs(devicePaths []string, lsblkOutput []byte) (map[string]string, error) {
	m := make(map[string]string)

	var lsblkData LsblkOutput
	if err := json.Unmarshal(lsblkOutput, &lsblkData); err != nil {
		return nil, fmt.Errorf("mapDevicePathsToVolumeIDs: error unmarshaling JSON: %w", err)
	}

	for _, device := range lsblkData.BlockDevices {
		devicePath := "/dev/" + device.Name

		for _, path := range devicePaths {
			if strings.HasPrefix(path, devicePath) {
				volumeID := device.Serial

				if strings.HasPrefix(volumeID, "vol") && !strings.HasPrefix(volumeID, "vol-") {
					volumeID = "vol-" + volumeID[3:]
				}

				m[path] = volumeID
				break
			}
		}
	}

	return m, nil
}

func executeLsblk() ([]byte, error) {
	cmd := exec.Command("lsblk", "-nd", "--json", "-o", "NAME,SERIAL")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("executeLsblk: error running lsblk: %w", err)
	}
	return output, nil
}

func fetchDevicePathToVolumeIDMapping(devicePaths []string) (map[string]string, error) {
	output, err := executeLsblk()
	if err != nil {
		return nil, fmt.Errorf("fetchDevicePathToVolumeIDMapping: %w", err)
	}

	return mapDevicePathsToVolumeIDs(devicePaths, output)
}
