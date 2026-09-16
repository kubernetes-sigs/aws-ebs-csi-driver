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

package helmtemplate

import (
	"fmt"
	"strconv"
	"testing"
)

// TestStandard validates the "standard" param set rendering.
func TestStandard(t *testing.T) {
	resources := renderChart(t, "standard")
	want := loadValuesMap(t, "standard")
	controller := mustFind(t, resources, "Deployment", "ebs-csi-controller")
	cPS := podSpec(t, controller)
	ebsPlugin := findContainer(t, cPS, "ebs-plugin")

	nodeDSObj := mustFind(t, resources, "DaemonSet", "ebs-csi-node")
	nPS := podSpec(t, nodeDSObj)
	nodePlugin := findContainer(t, nPS, "ebs-plugin")

	t.Run("controllerMetrics", func(t *testing.T) {
		svc := mustFind(t, resources, "Service", "ebs-csi-controller")
		assertServiceHasPort(t, svc, "metrics", 3301)
	})

	t.Run("nodeMetrics", func(t *testing.T) {
		svc := mustFind(t, resources, "Service", "ebs-csi-node")
		assertServiceHasPort(t, svc, "metrics", 3302)
	})

	t.Run("batching", func(t *testing.T) {
		wantArg := fmt.Sprintf("--batching=%v", nested(t, want, "controller")["batching"])
		if !hasArg(ebsPlugin, wantArg) {
			t.Errorf("controller should have %s", wantArg)
		}
	})

	t.Run("controllerLoggingFormat", func(t *testing.T) {
		wantFmt, _ := nestedString(want, "controller", "loggingFormat")
		wantArg := "--logging-format=" + wantFmt
		if !hasArg(ebsPlugin, wantArg) {
			t.Errorf("controller should have %s", wantArg)
		}
	})

	t.Run("nodeLoggingFormat", func(t *testing.T) {
		wantFmt, _ := nestedString(want, "node", "loggingFormat")
		wantArg := "--logging-format=" + wantFmt
		if !hasArg(nodePlugin, wantArg) {
			t.Errorf("node should have %s", wantArg)
		}
	})

	t.Run("controllerUserAgentExtra", func(t *testing.T) {
		wantUA, _ := nestedString(want, "controller", "userAgentExtra")
		wantArg := "--user-agent-extra=" + wantUA
		if !hasArg(ebsPlugin, wantArg) {
			t.Errorf("controller should have %s", wantArg)
		}
	})

	t.Run("reservedVolumeAttachments", func(t *testing.T) {
		wantN, _ := nestedFloat(want, "node", "reservedVolumeAttachments")
		wantArg := fmt.Sprintf("--reserved-volume-attachments=%d", int(wantN))
		if !hasArg(nodePlugin, wantArg) {
			t.Errorf("node should have %s", wantArg)
		}
	})

	t.Run("hostNetwork", func(t *testing.T) {
		hn, ok := nPS["hostNetwork"].(bool)
		if !ok || !hn {
			t.Error("node pod should use hostNetwork")
		}
	})

	t.Run("nodeTerminationGracePeriod", func(t *testing.T) {
		wantTGP, _ := nestedFloat(want, "node", "terminationGracePeriodSeconds")
		tgp, ok := nestedFloat(nPS, "terminationGracePeriodSeconds")
		if !ok || tgp != wantTGP {
			t.Errorf("node terminationGracePeriodSeconds: got %v, want %v", tgp, wantTGP)
		}
	})

	t.Run("nodeTolerateAllTaints", func(t *testing.T) {
		tolerations := nestedSlice(t, nPS, "tolerations")
		var found bool
		for _, tol := range tolerations {
			tm := mustObj(t, tol)
			if tm["operator"] == "Exists" && (tm["key"] == nil || tm["key"] == "") {
				found = true
			}
		}
		if !found {
			t.Error("node should have tolerate-all toleration")
		}
	})

	t.Run("nodeKubeletPath", func(t *testing.T) {
		wantPath, _ := nestedString(want, "node", "kubeletPath")
		mounts := nestedSlice(t, nodePlugin, "volumeMounts")
		var found bool
		for _, m := range mounts {
			mm := mustObj(t, m)
			if mm["mountPath"] == wantPath {
				found = true
			}
		}
		if !found {
			t.Errorf("node should mount kubelet path at %s", wantPath)
		}
	})

	t.Run("controllerPodDisruptionBudget", func(t *testing.T) {
		mustFind(t, resources, "PodDisruptionBudget", "ebs-csi-controller")
	})

	t.Run("snapshotterForceEnable", func(t *testing.T) {
		if !hasContainer(cPS, "csi-snapshotter") {
			t.Error("controller should have csi-snapshotter when forceEnable=true")
		}
	})

	t.Run("nodeDisableMutation", func(t *testing.T) {
		cr := mustFind(t, resources, "ClusterRole", "ebs-csi-node-role")
		rules := nestedSlice(t, cr, "rules")
		for _, rule := range rules {
			rm := mustObj(t, rule)
			res := nestedSlice(t, rm, "resources")
			for _, r := range res {
				if r == "nodes" {
					verbs := nestedSlice(t, rm, "verbs")
					for _, v := range verbs {
						if v == "patch" || v == "update" {
							t.Errorf("node role should not have %s on nodes when disableMutation=true", v)
						}
					}
				}
			}
		}
	})

	t.Run("storageClasses", func(t *testing.T) {
		wantSCs := nestedSlice(t, want, "storageClasses")
		if len(wantSCs) == 0 {
			t.Fatal("values file has no storageClasses")
		}
		wantSC := mustObj(t, wantSCs[0])
		wantName := mustString(t, wantSC["name"])
		wantType := nested(t, wantSC, "parameters")["type"]
		sc := mustFind(t, resources, "StorageClass", wantName)
		params := nested(t, sc, "parameters")
		if params["type"] != wantType {
			t.Errorf("StorageClass type: got %v, want %v", params["type"], wantType)
		}
	})

	t.Run("volumeSnapshotClasses", func(t *testing.T) {
		wantVSCs := nestedSlice(t, want, "volumeSnapshotClasses")
		if len(wantVSCs) == 0 {
			t.Fatal("values file has no volumeSnapshotClasses")
		}
		wantVSC := mustObj(t, wantVSCs[0])
		wantName := mustString(t, wantVSC["name"])
		wantDP := mustString(t, wantVSC["deletionPolicy"])
		vsc := mustFind(t, resources, "VolumeSnapshotClass", wantName)
		dp, ok := nestedString(vsc, "deletionPolicy")
		if !ok || dp != wantDP {
			t.Errorf("VolumeSnapshotClass deletionPolicy: got %v, want %s", dp, wantDP)
		}
	})

	t.Run("defaultStorageClass", func(t *testing.T) {
		sc := mustFind(t, resources, "StorageClass", "ebs-csi-default-sc")
		meta := nested(t, sc, "metadata")
		ann := nested(t, meta, "annotations")
		if ann["storageclass.kubernetes.io/is-default-class"] != "true" {
			t.Error("default StorageClass should have is-default-class=true annotation")
		}
	})

	t.Run("nodeAllocatableUpdatePeriodSeconds", func(t *testing.T) {
		wantVal, _ := nestedFloat(want, "nodeAllocatableUpdatePeriodSeconds")
		csiDriver := mustFind(t, resources, "CSIDriver", "ebs.csi.aws.com")
		val, ok := nestedFloat(csiDriver, "spec", "nodeAllocatableUpdatePeriodSeconds")
		if !ok || val != wantVal {
			t.Errorf("nodeAllocatableUpdatePeriodSeconds: got %v, want %v", val, wantVal)
		}
	})

	// Log level tests. logLevelPath walks the YAML to the expected log level
	// for that sidecar or plugin; keeping it as []string so the lookup stays
	// in lockstep with the values file.
	logLevelTests := []struct {
		name         string
		container    string
		podType      string // "controller" or "node"
		logLevelPath []string
	}{
		{"controllerLogLevel", "ebs-plugin", "controller", []string{"controller", "logLevel"}},
		{"provisionerLogLevel", "csi-provisioner", "controller", []string{"sidecars", "provisioner", "logLevel"}},
		{"attacherLogLevel", "csi-attacher", "controller", []string{"sidecars", "attacher", "logLevel"}},
		{"snapshotterLogLevel", "csi-snapshotter", "controller", []string{"sidecars", "snapshotter", "logLevel"}},
		{"resizerLogLevel", "csi-resizer", "controller", []string{"sidecars", "resizer", "logLevel"}},
		{"nodeDriverRegistrarLogLevel", "node-driver-registrar", "node", []string{"sidecars", "nodeDriverRegistrar", "logLevel"}},
		{"nodeLogLevel", "ebs-plugin", "node", []string{"node", "logLevel"}},
	}
	for _, tc := range logLevelTests {
		t.Run(tc.name, func(t *testing.T) {
			wantLvl, ok := nestedFloat(want, tc.logLevelPath...)
			if !ok {
				t.Fatalf("values file missing %v", tc.logLevelPath)
			}
			var ps obj
			if tc.podType == "controller" {
				ps = cPS
			} else {
				ps = nPS
			}
			c := findContainer(t, ps, tc.container)
			want := strconv.Itoa(int(wantLvl))
			if !hasArgAny(c, "-v="+want, "--v="+want) {
				t.Errorf("%s should have -v=%s", tc.container, want)
			}
		})
	}

	// Leader election tests. Each row names the sidecar key under
	// standard.yaml's sidecars map and asserts the corresponding flag.
	leaderTests := []struct {
		name      string
		container string
		valuesKey string
	}{
		{"provisionerLeaderElection", "csi-provisioner", "provisioner"},
		{"attacherLeaderElection", "csi-attacher", "attacher"},
		{"resizerLeaderElection", "csi-resizer", "resizer"},
	}
	for _, tc := range leaderTests {
		t.Run(tc.name, func(t *testing.T) {
			enabled := nested(t, want, "sidecars", tc.valuesKey, "leaderElection")["enabled"]
			c := findContainer(t, cPS, tc.container)
			expected := fmt.Sprintf("--leader-election=%v", enabled)
			if !hasArg(c, expected) {
				t.Errorf("%s should have %s", tc.container, expected)
			}
		})
	}

	// Prometheus annotations
	for _, svcName := range []string{"ebs-csi-controller", "ebs-csi-node"} {
		t.Run("prometheusAnnotations/"+svcName, func(t *testing.T) {
			svc := mustFind(t, resources, "Service", svcName)
			meta := nested(t, svc, "metadata")
			ann, ok := meta["annotations"].(obj)
			if !ok {
				t.Fatal("no annotations on service")
			}
			if ann["prometheus.io/scrape"] != "true" {
				t.Error("service should have prometheus.io/scrape=true")
			}
		})
	}
}

// assertServiceHasPort checks that a Service has a port with the given name and number.
func assertServiceHasPort(t *testing.T, svc obj, name string, port int) {
	t.Helper()
	ports := nestedSlice(t, svc, "spec", "ports")
	for _, p := range ports {
		pm := mustObj(t, p)
		pNum, _ := pm["port"].(float64)
		if pm["name"] == name && int(pNum) == port {
			return
		}
	}
	t.Errorf("service should have port %s/%d", name, port)
}
