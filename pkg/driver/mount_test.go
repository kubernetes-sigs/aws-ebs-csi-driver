/*
Copyright 2020 The Kubernetes Authors.

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
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
)

func TestMakeDir(t *testing.T) {
	// Setup the full driver and its environment
	dir, err := ioutil.TempDir("", "mount-ebs-csi")
	if err != nil {
		t.Fatalf("error creating directory %v", err)
	}
	defer os.RemoveAll(dir)

	targetPath := filepath.Join(dir, "targetdir")

	var (
		mountObj = newNodeMounter()
	)

	if mountObj.MakeDir(targetPath) != nil {
		t.Fatalf("Expect no error but got: %v", err)
	}

	if mountObj.MakeDir(targetPath) != nil {
		t.Fatalf("Expect no error but got: %v", err)
	}

	if exists, err := mountObj.ExistsPath(targetPath); !exists {
		t.Fatalf("Expect no error but got: %v", err)
	}
}

func TestMakeFile(t *testing.T) {
	// Setup the full driver and its environment
	dir, err := ioutil.TempDir("", "mount-ebs-csi")
	if err != nil {
		t.Fatalf("error creating directory %v", err)
	}
	defer os.RemoveAll(dir)

	targetPath := filepath.Join(dir, "targetfile")

	var (
		mountObj = newNodeMounter()
	)

	if mountObj.MakeFile(targetPath) != nil {
		t.Fatalf("Expect no error but got: %v", err)
	}

	if mountObj.MakeFile(targetPath) != nil {
		t.Fatalf("Expect no error but got: %v", err)
	}

	if exists, err := mountObj.ExistsPath(targetPath); !exists {
		t.Fatalf("Expect no error but got: %v", err)
	}

}

func TestExistsPath(t *testing.T) {
	// Setup the full driver and its environment
	dir, err := ioutil.TempDir("", "mount-ebs-csi")
	if err != nil {
		t.Fatalf("error creating directory %v", err)
	}
	defer os.RemoveAll(dir)

	targetPath := filepath.Join(dir, "notafile")

	var (
		mountObj = newNodeMounter()
	)

	exists, err := mountObj.ExistsPath(targetPath)

	if err != nil {
		t.Fatalf("Expect no error but got: %v", err)
	}

	if exists {
		t.Fatalf("Expected file %s to not exist", targetPath)
	}

}

func TestGetDeviceName(t *testing.T) {
	// Setup the full driver and its environment
	dir, err := ioutil.TempDir("", "mount-ebs-csi")
	if err != nil {
		t.Fatalf("error creating directory %v", err)
	}
	defer os.RemoveAll(dir)

	targetPath := filepath.Join(dir, "notafile")

	var (
		mountObj = newNodeMounter()
	)

	if _, _, err := mountObj.GetDeviceName(targetPath); err != nil {
		t.Fatalf("Expect no error but got: %v", err)
	}

}
