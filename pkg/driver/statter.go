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

package driver

import (
	"fmt"

	"golang.org/x/sys/unix"
)

type Statter interface {
	StatFS(path string) (int64, int64, int64, int64, int64, int64, error)
	IsBlockDevice(string) (bool, error)
}

var _ Statter = realStatter{}

type realStatter struct {
}

func NewStatter() realStatter {
	return realStatter{}
}

// IsBlock checks if the given path is a block device
func (realStatter) IsBlockDevice(fullPath string) (bool, error) {
	var st unix.Stat_t
	err := unix.Stat(fullPath, &st)
	if err != nil {
		return false, err
	}

	return (st.Mode & unix.S_IFMT) == unix.S_IFBLK, nil
}

func (realStatter) StatFS(path string) (available, capacity, used, inodesFree, inodes, inodesUsed int64, err error) {
	statfs := &unix.Statfs_t{}
	err = unix.Statfs(path, statfs)
	if err != nil {
		err = fmt.Errorf("failed to get fs info on path %s: %v", path, err)
		return
	}

	// Available is blocks available * fragment size
	available = int64(statfs.Bavail) * int64(statfs.Bsize)
	// Capacity is total block count * fragment size
	capacity = int64(statfs.Blocks) * int64(statfs.Bsize)
	// Usage is block being used * fragment size (aka block size).
	used = (int64(statfs.Blocks) - int64(statfs.Bfree)) * int64(statfs.Bsize)
	inodes = int64(statfs.Files)
	inodesFree = int64(statfs.Ffree)
	inodesUsed = inodes - inodesFree
	return
}

type fakeStatter struct{}

func NewFakeStatter() fakeStatter {
	return fakeStatter{}
}

func (fakeStatter) StatFS(path string) (available, capacity, used, inodesFree, inodes, inodesUsed int64, err error) {
	// Assume the file exists and give some dummy values back
	return 1, 1, 1, 1, 1, 1, nil
}

func (fakeStatter) IsBlockDevice(fullPath string) (bool, error) {
	return false, nil
}
