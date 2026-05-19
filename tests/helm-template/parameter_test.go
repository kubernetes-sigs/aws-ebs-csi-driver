/*
Copyright 2025 The Kubernetes Authors.

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

// Package helmtemplate tests Helm chart parameter rendering without a live cluster.
package helmtemplate

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"

	"sigs.k8s.io/yaml"
)

const releaseName = "ebs-csi"

// chartPath returns the absolute path to the helm chart.
func chartPath() string {
	_, f, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(f), "..", "..", "charts", "aws-ebs-csi-driver")
}

// helmBin returns the path to the helm binary.
func helmBin() string {
	_, f, _, _ := runtime.Caller(0)
	bin := filepath.Join(filepath.Dir(f), "..", "..", "bin", ".helm")
	if _, err := os.Stat(bin); err == nil {
		return bin
	}
	return "helm"
}

// testdataPath returns the absolute path to a testdata values file.
func testdataPath(name string) string {
	_, f, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(f), "testdata", name+".yaml")
}

// obj is a generic JSON-like object parsed from YAML.
type obj = map[string]interface{}

// loadValuesMap reads a testdata values YAML file as a generic map so tests can
// read expected values from the same file they pass to `helm template`.
// This avoids hardcoding values in assertions that would silently drift when
// the YAML is updated.
func loadValuesMap(t *testing.T, name string) obj {
	t.Helper()
	data, err := os.ReadFile(testdataPath(name))
	if err != nil {
		t.Fatalf("read values %s: %v", name, err)
	}
	j, err := yaml.YAMLToJSON(data)
	if err != nil {
		t.Fatalf("yaml to json: %v", err)
	}
	var m obj
	if err := json.Unmarshal(j, &m); err != nil {
		t.Fatalf("json unmarshal: %v", err)
	}
	return m
}

// renderChart runs helm template and returns parsed resources as generic maps.
func renderChart(t *testing.T, valuesFile string) []obj {
	t.Helper()
	cmd := exec.Command(helmBin(), "template", releaseName, chartPath(), "-f", testdataPath(valuesFile))
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("helm template failed: %v\nstderr: %s", err, stderr.String())
	}
	return parseYAMLDocs(t, stdout.Bytes())
}

// renderChartWithSet runs helm template with inline --set flags instead of a
// values file. Useful for one- or two-flag render-time tests where a dedicated
// testdata YAML would be overkill.
func renderChartWithSet(t *testing.T, sets ...string) []obj {
	t.Helper()
	args := []string{"template", releaseName, chartPath()}
	for _, s := range sets {
		args = append(args, "--set", s)
	}
	cmd := exec.Command(helmBin(), args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("helm template failed: %v\nstderr: %s", err, stderr.String())
	}
	return parseYAMLDocs(t, stdout.Bytes())
}

// parseYAMLDocs splits multi-doc YAML and converts each to a JSON-like map via sigs.k8s.io/yaml.
func parseYAMLDocs(t *testing.T, data []byte) []obj {
	t.Helper()
	var out []obj
	for _, doc := range bytes.Split(data, []byte("\n---")) {
		doc = bytes.TrimSpace(doc)
		if len(doc) == 0 {
			continue
		}
		j, err := yaml.YAMLToJSON(doc)
		if err != nil {
			t.Fatalf("yaml to json: %v", err)
		}
		var m obj
		if err := json.Unmarshal(j, &m); err != nil {
			t.Fatalf("json unmarshal: %v", err)
		}
		if m["kind"] != nil {
			out = append(out, m)
		}
	}
	return out
}

// find returns the first resource matching kind and name.
func find(resources []obj, kind, name string) (obj, bool) {
	for _, r := range resources {
		if r["kind"] == kind {
			if meta, ok := r["metadata"].(obj); ok && meta["name"] == name {
				return r, true
			}
		}
	}
	return nil, false
}

// mustFind returns a resource or fails the test.
func mustFind(t *testing.T, resources []obj, kind, name string) obj {
	t.Helper()
	r, ok := find(resources, kind, name)
	if !ok {
		t.Fatalf("resource %s/%s not found", kind, name)
	}
	return r
}

// podSpec extracts .spec.template.spec from a Deployment or DaemonSet.
func podSpec(t *testing.T, r obj) obj {
	t.Helper()
	spec := nested(t, r, "spec", "template", "spec")
	return spec
}

// findContainer finds a container by name in a pod spec.
func findContainer(t *testing.T, ps obj, name string) obj {
	t.Helper()
	for _, c := range nestedSlice(t, ps, "containers") {
		cm := c.(obj)
		if cm["name"] == name {
			return cm
		}
	}
	t.Fatalf("container %q not found", name)
	return nil
}

// hasContainer returns true if the pod spec has a container with the given name.
func hasContainer(ps obj, name string) bool {
	containers, ok := ps["containers"].([]interface{})
	if !ok {
		return false
	}
	for _, c := range containers {
		if cm, ok := c.(obj); ok && cm["name"] == name {
			return true
		}
	}
	return false
}

// containerArgs returns the args slice for a container.
func containerArgs(c obj) []string {
	args, ok := c["args"].([]interface{})
	if !ok {
		return nil
	}
	var out []string
	for _, a := range args {
		out = append(out, fmt.Sprint(a))
	}
	return out
}

// hasArg checks if a container has a specific arg.
func hasArg(c obj, arg string) bool {
	for _, a := range containerArgs(c) {
		if a == arg {
			return true
		}
	}
	return false
}

// hasArgAny checks if a container has any of the given args.
func hasArgAny(c obj, args ...string) bool {
	for _, arg := range args {
		if hasArg(c, arg) {
			return true
		}
	}
	return false
}

// nested traverses a map by keys and returns the nested map.
func nested(t *testing.T, m obj, keys ...string) obj {
	t.Helper()
	cur := m
	for _, k := range keys {
		val, ok := cur[k]
		if !ok {
			t.Fatalf("key %q not found in path %v", k, keys)
		}
		cur, ok = val.(obj)
		if !ok {
			t.Fatalf("key %q is not a map in path %v", k, keys)
		}
	}
	return cur
}

// nestedSlice traverses a map by keys and returns the nested slice.
func nestedSlice(t *testing.T, m obj, keys ...string) []interface{} {
	t.Helper()
	cur := m
	for i, k := range keys {
		val, ok := cur[k]
		if !ok {
			t.Fatalf("key %q not found", k)
		}
		if i == len(keys)-1 {
			s, ok := val.([]interface{})
			if !ok {
				t.Fatalf("key %q is not a slice", k)
			}
			return s
		}
		cur, ok = val.(obj)
		if !ok {
			t.Fatalf("key %q is not a map", k)
		}
	}
	return nil
}

// nestedString traverses a map by keys and returns the string value.
func nestedString(m obj, keys ...string) (string, bool) {
	cur := m
	for i, k := range keys {
		val, ok := cur[k]
		if !ok {
			return "", false
		}
		if i == len(keys)-1 {
			s, ok := val.(string)
			return s, ok
		}
		cur, ok = val.(obj)
		if !ok {
			return "", false
		}
	}
	return "", false
}

// nestedFloat traverses a map by keys and returns the float64 value.
func nestedFloat(m obj, keys ...string) (float64, bool) {
	cur := m
	for i, k := range keys {
		val, ok := cur[k]
		if !ok {
			return 0, false
		}
		if i == len(keys)-1 {
			f, ok := val.(float64)
			return f, ok
		}
		cur, ok = val.(obj)
		if !ok {
			return 0, false
		}
	}
	return 0, false
}
