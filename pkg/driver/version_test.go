/*
Copyright 2018 The Kubernetes Authors.

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
	"reflect"
	"runtime"
	"testing"
)

func TestGetVersion(t *testing.T) {
	version := GetVersion()

	expected := VersionInfo{
		DriverVersion: "",
		GitCommit:     "",
		BuildDate:     "",
		GoVersion:     runtime.Version(),
		Compiler:      runtime.Compiler,
		Platform:      fmt.Sprintf("%s/%s", runtime.GOOS, runtime.GOARCH),
	}

	if !reflect.DeepEqual(version, expected) {
		t.Fatalf("structs not equall\ngot:\n%+v\nexpected:\n%+v", version, expected)
	}
}

func TestGetVersionJSON(t *testing.T) {
	version, err := GetVersionJSON()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := fmt.Sprintf(`{
  "driverVersion": "",
  "gitCommit": "",
  "buildDate": "",
  "goVersion": "%s",
  "compiler": "%s",
  "platform": "%s"
}`, runtime.Version(), runtime.Compiler, fmt.Sprintf("%s/%s", runtime.GOOS, runtime.GOARCH))

	if version != expected {
		t.Fatalf("json not equall\ngot:\n%s\nexpected:\n%s", version, expected)
	}
}
