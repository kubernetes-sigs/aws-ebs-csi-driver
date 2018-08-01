package util

import (
	"fmt"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
)

// TODO: check division by zero and int overflow

func roundUpSize(volumeSizeBytes int64, allocationUnitBytes int64) int64 {
	return (volumeSizeBytes + allocationUnitBytes - 1) / allocationUnitBytes
}

func RoundUpBytes(volumeSizeBytes int64) int64 {
	sizeGiB := roundUpSize(volumeSizeBytes, 1024*1024*1024)
	return sizeGiB * 1024 * 1024 * 1024
}

func RoundUpGiB(volumeSizeBytes int64) int64 {
	return roundUpSize(volumeSizeBytes, 1024*1024*1024)
}

func BytesToGiB(volumeSizeBytes int64) int64 {
	return ((volumeSizeBytes / 1024) / 1024) / 1024
}

func GiBToBytes(volumeSizeBytes int64) int64 {
	return volumeSizeBytes * 1024 * 1024 * 1024
}

func ParseEndpoint(endpoint string) (string, string, error) {
	u, err := url.Parse(endpoint)
	if err != nil {
		return "", "", fmt.Errorf("could not parse endpoint: %v", err)
	}

	addr := path.Join(u.Host, filepath.FromSlash(u.Path))

	scheme := strings.ToLower(u.Scheme)
	switch scheme {
	case "tcp":
	case "unix":
		addr = path.Join("/", addr)
		if err := os.Remove(addr); err != nil && !os.IsNotExist(err) {
			return "", "", fmt.Errorf("could not remove unix domain socket %q: %v", addr, err)
		}
	default:
		return "", "", fmt.Errorf("unsupported protocol: %s", scheme)
	}

	return scheme, addr, nil
}
