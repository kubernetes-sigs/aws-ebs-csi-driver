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
	"testing"
)

func TestInfra(t *testing.T) {
	resources := renderChart(t, "infra")
	want := loadValuesMap(t, "infra")
	deploy := mustFind(t, resources, "Deployment", "ebs-csi-controller")
	dSpec := nested(t, deploy, "spec")
	cPS := podSpec(t, deploy)
	ebsPlugin := findContainer(t, cPS, "ebs-plugin")

	ds := mustFind(t, resources, "DaemonSet", "ebs-csi-node")
	dsSpec := nested(t, ds, "spec")
	nPS := podSpec(t, ds)
	nodePlugin := findContainer(t, nPS, "ebs-plugin")

	t.Run("controllerReplicaCount", func(t *testing.T) {
		wantReplicas, _ := nestedFloat(want, "controller", "replicaCount")
		replicas, ok := nestedFloat(dSpec, "replicas")
		if !ok || replicas != wantReplicas {
			t.Errorf("replicas: got %v, want %v", replicas, wantReplicas)
		}
	})

	t.Run("controllerPriorityClassName", func(t *testing.T) {
		wantPC, _ := nestedString(want, "controller", "priorityClassName")
		if cPS["priorityClassName"] != wantPC {
			t.Errorf("controller priorityClassName: got %v, want %s", cPS["priorityClassName"], wantPC)
		}
	})

	t.Run("controllerResources", func(t *testing.T) {
		wantCPU, _ := nestedString(want, "controller", "resources", "requests", "cpu")
		wantMem, _ := nestedString(want, "controller", "resources", "limits", "memory")
		res := nested(t, ebsPlugin, "resources")
		cpu, _ := nestedString(res, "requests", "cpu")
		mem, _ := nestedString(res, "limits", "memory")
		if cpu != wantCPU {
			t.Errorf("controller cpu request: got %s, want %s", cpu, wantCPU)
		}
		if mem != wantMem {
			t.Errorf("controller memory limit: got %s, want %s", mem, wantMem)
		}
	})

	t.Run("controllerPodAnnotations", func(t *testing.T) {
		wantAnns, _ := want["controller"].(obj)["podAnnotations"].(obj)
		tmpl := nested(t, deploy, "spec", "template", "metadata")
		ann := tmpl["annotations"].(obj)
		for k, v := range wantAnns {
			if ann[k] != v {
				t.Errorf("controller pod annotation %q: got %v, want %v", k, ann[k], v)
			}
		}
	})

	t.Run("controllerPodLabels", func(t *testing.T) {
		wantLabels, _ := want["controller"].(obj)["podLabels"].(obj)
		tmpl := nested(t, deploy, "spec", "template", "metadata")
		labels := tmpl["labels"].(obj)
		for k, v := range wantLabels {
			if labels[k] != v {
				t.Errorf("controller pod label %q: got %v, want %v", k, labels[k], v)
			}
		}
	})

	t.Run("controllerDeploymentAnnotations", func(t *testing.T) {
		wantAnns, _ := want["controller"].(obj)["deploymentAnnotations"].(obj)
		meta := nested(t, deploy, "metadata")
		ann := meta["annotations"].(obj)
		for k, v := range wantAnns {
			if ann[k] != v {
				t.Errorf("deployment annotation %q: got %v, want %v", k, ann[k], v)
			}
		}
	})

	t.Run("controllerRevisionHistoryLimit", func(t *testing.T) {
		wantRHL, _ := nestedFloat(want, "controller", "revisionHistoryLimit")
		rhl, ok := nestedFloat(dSpec, "revisionHistoryLimit")
		if !ok || rhl != wantRHL {
			t.Errorf("revisionHistoryLimit: got %v, want %v", rhl, wantRHL)
		}
	})

	t.Run("controllerTopologySpreadConstraints", func(t *testing.T) {
		wantTSCs, _ := want["controller"].(obj)["topologySpreadConstraints"].([]interface{})
		if len(wantTSCs) == 0 {
			t.Fatal("values file has no topologySpreadConstraints")
		}
		wantKey := wantTSCs[0].(obj)["topologyKey"]
		tscs := nestedSlice(t, cPS, "topologySpreadConstraints")
		if len(tscs) == 0 {
			t.Fatal("no topologySpreadConstraints in render")
		}
		if got := tscs[0].(obj)["topologyKey"]; got != wantKey {
			t.Errorf("topologyKey: got %v, want %v", got, wantKey)
		}
	})

	t.Run("controllerSecurityContext", func(t *testing.T) {
		wantSC, _ := want["controller"].(obj)["securityContext"].(obj)
		sc := nested(t, cPS, "securityContext")
		for k, v := range wantSC {
			if sc[k] != v {
				t.Errorf("controller securityContext.%s: got %v, want %v", k, sc[k], v)
			}
		}
	})

	t.Run("controllerContainerSecurityContext", func(t *testing.T) {
		wantSC, _ := want["controller"].(obj)["containerSecurityContext"].(obj)
		sc := nested(t, ebsPlugin, "securityContext")
		for k, v := range wantSC {
			if sc[k] != v {
				t.Errorf("ebs-plugin securityContext.%s: got %v, want %v", k, sc[k], v)
			}
		}
	})

	t.Run("controllerUpdateStrategy", func(t *testing.T) {
		wantType, _ := nestedString(want, "controller", "updateStrategy", "type")
		strategy := nested(t, dSpec, "strategy")
		if strategy["type"] != wantType {
			t.Errorf("strategy type: got %v, want %s", strategy["type"], wantType)
		}
	})

	t.Run("controllerEnv", func(t *testing.T) {
		wantEnvs, _ := want["controller"].(obj)["env"].([]interface{})
		envs := nestedSlice(t, ebsPlugin, "env")
		for _, we := range wantEnvs {
			wm := we.(obj)
			var found bool
			for _, e := range envs {
				em := e.(obj)
				if em["name"] == wm["name"] && em["value"] == wm["value"] {
					found = true
				}
			}
			if !found {
				t.Errorf("controller should have env %v=%v", wm["name"], wm["value"])
			}
		}
	})

	t.Run("controllerVolumes", func(t *testing.T) {
		wantVols, _ := want["controller"].(obj)["volumes"].([]interface{})
		vols := nestedSlice(t, cPS, "volumes")
		for _, wv := range wantVols {
			wantName := wv.(obj)["name"]
			var found bool
			for _, v := range vols {
				if v.(obj)["name"] == wantName {
					found = true
				}
			}
			if !found {
				t.Errorf("controller should have volume %v", wantName)
			}
		}
	})

	t.Run("controllerVolumeMounts", func(t *testing.T) {
		wantMounts, _ := want["controller"].(obj)["volumeMounts"].([]interface{})
		mounts := nestedSlice(t, ebsPlugin, "volumeMounts")
		for _, wm := range wantMounts {
			wmObj := wm.(obj)
			var found bool
			for _, m := range mounts {
				mm := m.(obj)
				if mm["name"] == wmObj["name"] && mm["mountPath"] == wmObj["mountPath"] {
					found = true
				}
			}
			if !found {
				t.Errorf("controller should have mount %v at %v", wmObj["name"], wmObj["mountPath"])
			}
		}
	})

	t.Run("controllerDnsConfig", func(t *testing.T) {
		wantNS, _ := want["controller"].(obj)["dnsConfig"].(obj)["nameservers"].([]interface{})
		dns := nested(t, cPS, "dnsConfig")
		got := dns["nameservers"].([]interface{})
		for _, wn := range wantNS {
			var found bool
			for _, n := range got {
				if n == wn {
					found = true
				}
			}
			if !found {
				t.Errorf("controller dnsConfig should have nameserver %v", wn)
			}
		}
	})

	t.Run("controllerInitContainers", func(t *testing.T) {
		wantInits, _ := want["controller"].(obj)["initContainers"].([]interface{})
		inits := nestedSlice(t, cPS, "initContainers")
		for _, wi := range wantInits {
			wantName := wi.(obj)["name"]
			var found bool
			for _, c := range inits {
				if c.(obj)["name"] == wantName {
					found = true
				}
			}
			if !found {
				t.Errorf("controller should have init container %v", wantName)
			}
		}
	})

	t.Run("controllerTolerations", func(t *testing.T) {
		wantTols, _ := want["controller"].(obj)["tolerations"].([]interface{})
		tols := nestedSlice(t, cPS, "tolerations")
		for _, wt := range wantTols {
			wtObj := wt.(obj)
			var found bool
			for _, tol := range tols {
				tm := tol.(obj)
				if tm["key"] == wtObj["key"] && tm["value"] == wtObj["value"] && tm["effect"] == wtObj["effect"] {
					found = true
				}
			}
			if !found {
				t.Errorf("controller should have toleration key=%v value=%v effect=%v",
					wtObj["key"], wtObj["value"], wtObj["effect"])
			}
		}
	})

	t.Run("controllerAdditionalArgs", func(t *testing.T) {
		wantArgs, _ := want["controller"].(obj)["additionalArgs"].([]interface{})
		for _, a := range wantArgs {
			if !hasArg(ebsPlugin, a.(string)) {
				t.Errorf("controller should have arg %q", a)
			}
		}
	})

	t.Run("nameOverride", func(t *testing.T) {
		wantName, _ := nestedString(want, "nameOverride")
		meta := nested(t, deploy, "metadata")
		labels := meta["labels"].(obj)
		if labels["app.kubernetes.io/name"] != wantName {
			t.Errorf("app.kubernetes.io/name: got %v, want %s", labels["app.kubernetes.io/name"], wantName)
		}
	})

	t.Run("imagePullPolicy", func(t *testing.T) {
		wantPolicy, _ := nestedString(want, "image", "pullPolicy")
		if ebsPlugin["imagePullPolicy"] != wantPolicy {
			t.Errorf("ebs-plugin imagePullPolicy: got %v, want %s", ebsPlugin["imagePullPolicy"], wantPolicy)
		}
	})

	t.Run("customLabels/controller", func(t *testing.T) {
		wantLabels, _ := want["customLabels"].(obj)
		meta := nested(t, deploy, "metadata")
		labels := meta["labels"].(obj)
		for k, v := range wantLabels {
			if labels[k] != v {
				t.Errorf("controller label %q: got %v, want %v", k, labels[k], v)
			}
		}
	})

	// --- Node DaemonSet assertions ---

	t.Run("nodePriorityClassName", func(t *testing.T) {
		wantPC, _ := nestedString(want, "node", "priorityClassName")
		if nPS["priorityClassName"] != wantPC {
			t.Errorf("node priorityClassName: got %v, want %s", nPS["priorityClassName"], wantPC)
		}
	})

	t.Run("nodeResources", func(t *testing.T) {
		wantCPU, _ := nestedString(want, "node", "resources", "requests", "cpu")
		wantMem, _ := nestedString(want, "node", "resources", "limits", "memory")
		res := nested(t, nodePlugin, "resources")
		cpu, _ := nestedString(res, "requests", "cpu")
		mem, _ := nestedString(res, "limits", "memory")
		if cpu != wantCPU {
			t.Errorf("node cpu request: got %s, want %s", cpu, wantCPU)
		}
		if mem != wantMem {
			t.Errorf("node memory limit: got %s, want %s", mem, wantMem)
		}
	})

	t.Run("nodePodAnnotations", func(t *testing.T) {
		wantAnns, _ := want["node"].(obj)["podAnnotations"].(obj)
		tmpl := nested(t, ds, "spec", "template", "metadata")
		ann := tmpl["annotations"].(obj)
		for k, v := range wantAnns {
			if ann[k] != v {
				t.Errorf("node pod annotation %q: got %v, want %v", k, ann[k], v)
			}
		}
	})

	t.Run("nodeDaemonSetAnnotations", func(t *testing.T) {
		wantAnns, _ := want["node"].(obj)["daemonSetAnnotations"].(obj)
		meta := nested(t, ds, "metadata")
		ann := meta["annotations"].(obj)
		for k, v := range wantAnns {
			if ann[k] != v {
				t.Errorf("daemonset annotation %q: got %v, want %v", k, ann[k], v)
			}
		}
	})

	t.Run("nodeRevisionHistoryLimit", func(t *testing.T) {
		wantRHL, _ := nestedFloat(want, "node", "revisionHistoryLimit")
		rhl, ok := nestedFloat(dsSpec, "revisionHistoryLimit")
		if !ok || rhl != wantRHL {
			t.Errorf("node revisionHistoryLimit: got %v, want %v", rhl, wantRHL)
		}
	})

	t.Run("nodeSecurityContext", func(t *testing.T) {
		// infra.yaml doesn't set node.securityContext, just assert it renders.
		if nPS["securityContext"] == nil {
			t.Error("node should have securityContext")
		}
	})

	t.Run("nodeUpdateStrategy", func(t *testing.T) {
		wantType, _ := nestedString(want, "node", "updateStrategy", "type")
		strategy := nested(t, dsSpec, "updateStrategy")
		if strategy["type"] != wantType {
			t.Errorf("node updateStrategy: got %v, want %s", strategy["type"], wantType)
		}
	})

	t.Run("nodeEnv", func(t *testing.T) {
		wantEnvs, _ := want["node"].(obj)["env"].([]interface{})
		envs := nestedSlice(t, nodePlugin, "env")
		for _, we := range wantEnvs {
			wm := we.(obj)
			var found bool
			for _, e := range envs {
				em := e.(obj)
				if em["name"] == wm["name"] && em["value"] == wm["value"] {
					found = true
				}
			}
			if !found {
				t.Errorf("node should have env %v=%v", wm["name"], wm["value"])
			}
		}
	})

	t.Run("nodeVolumes", func(t *testing.T) {
		wantVols, _ := want["node"].(obj)["volumes"].([]interface{})
		vols := nestedSlice(t, nPS, "volumes")
		for _, wv := range wantVols {
			wantName := wv.(obj)["name"]
			var found bool
			for _, v := range vols {
				if v.(obj)["name"] == wantName {
					found = true
				}
			}
			if !found {
				t.Errorf("node should have volume %v", wantName)
			}
		}
	})

	t.Run("nodeVolumeMounts", func(t *testing.T) {
		wantMounts, _ := want["node"].(obj)["volumeMounts"].([]interface{})
		mounts := nestedSlice(t, nodePlugin, "volumeMounts")
		for _, wm := range wantMounts {
			wmObj := wm.(obj)
			var found bool
			for _, m := range mounts {
				mm := m.(obj)
				if mm["name"] == wmObj["name"] && mm["mountPath"] == wmObj["mountPath"] {
					found = true
				}
			}
			if !found {
				t.Errorf("node should have mount %v at %v", wmObj["name"], wmObj["mountPath"])
			}
		}
	})

	t.Run("nodeAdditionalArgs", func(t *testing.T) {
		wantArgs, _ := want["node"].(obj)["additionalArgs"].([]interface{})
		for _, a := range wantArgs {
			if !hasArg(nodePlugin, a.(string)) {
				t.Errorf("node should have arg %q", a)
			}
		}
	})

	t.Run("nodeDnsConfig", func(t *testing.T) {
		wantNS, _ := want["node"].(obj)["dnsConfig"].(obj)["nameservers"].([]interface{})
		dns := nested(t, nPS, "dnsConfig")
		got := dns["nameservers"].([]interface{})
		for _, wn := range wantNS {
			var found bool
			for _, n := range got {
				if n == wn {
					found = true
				}
			}
			if !found {
				t.Errorf("node dnsConfig should have nameserver %v", wn)
			}
		}
	})

	t.Run("nodeInitContainers", func(t *testing.T) {
		wantInits, _ := want["node"].(obj)["initContainers"].([]interface{})
		inits := nestedSlice(t, nPS, "initContainers")
		for _, wi := range wantInits {
			wantName := wi.(obj)["name"]
			var found bool
			for _, c := range inits {
				if c.(obj)["name"] == wantName {
					found = true
				}
			}
			if !found {
				t.Errorf("node should have init container %v", wantName)
			}
		}
	})

	t.Run("nodeTolerations", func(t *testing.T) {
		wantTols, _ := want["node"].(obj)["tolerations"].([]interface{})
		tols := nestedSlice(t, nPS, "tolerations")
		for _, wt := range wantTols {
			wtObj := wt.(obj)
			var found bool
			for _, tol := range tols {
				tm := tol.(obj)
				if tm["key"] == wtObj["key"] && tm["value"] == wtObj["value"] {
					found = true
				}
			}
			if !found {
				t.Errorf("node should have toleration key=%v value=%v", wtObj["key"], wtObj["value"])
			}
		}
	})

	t.Run("customLabels/node", func(t *testing.T) {
		wantLabels, _ := want["customLabels"].(obj)
		meta := nested(t, ds, "metadata")
		labels := meta["labels"].(obj)
		for k, v := range wantLabels {
			if labels[k] != v {
				t.Errorf("node label %q: got %v, want %v", k, labels[k], v)
			}
		}
	})

	// Sidecar resource tests. expectedCPU is looked up from infra.yaml so
	// values stay in one place.
	sidecarTests := []struct {
		name      string
		resKind   string
		resName   string
		container string
		valuesKey string // key under sidecars in infra.yaml
	}{
		{"provisionerResources", "Deployment", "ebs-csi-controller", "csi-provisioner", "provisioner"},
		{"attacherResources", "Deployment", "ebs-csi-controller", "csi-attacher", "attacher"},
		{"snapshotterResources", "Deployment", "ebs-csi-controller", "csi-snapshotter", "snapshotter"},
		{"resizerResources", "Deployment", "ebs-csi-controller", "csi-resizer", "resizer"},
		{"nodeDriverRegistrarResources", "DaemonSet", "ebs-csi-node", "node-driver-registrar", "nodeDriverRegistrar"},
		{"livenessProbeResources", "DaemonSet", "ebs-csi-node", "liveness-probe", "livenessProbe"},
	}
	for _, tc := range sidecarTests {
		t.Run(tc.name, func(t *testing.T) {
			wantCPU, _ := nestedString(want, "sidecars", tc.valuesKey, "resources", "requests", "cpu")
			r := mustFind(t, resources, tc.resKind, tc.resName)
			ps := podSpec(t, r)
			c := findContainer(t, ps, tc.container)
			res := nested(t, c, "resources")
			cpu, _ := nestedString(res, "requests", "cpu")
			if cpu != wantCPU {
				t.Errorf("%s cpu request: got %s, want %s", tc.container, cpu, wantCPU)
			}
		})
	}

	t.Run("provisionerAdditionalArgs", func(t *testing.T) {
		wantArgs, _ := want["sidecars"].(obj)["provisioner"].(obj)["additionalArgs"].([]interface{})
		provisioner := findContainer(t, cPS, "csi-provisioner")
		for _, a := range wantArgs {
			if !hasArg(provisioner, a.(string)) {
				t.Errorf("provisioner should have arg %q", a)
			}
		}
	})
}

func TestDebug(t *testing.T) {
	resources := renderChart(t, "debug")
	controller := mustFind(t, resources, "Deployment", "ebs-csi-controller")
	cPS := podSpec(t, controller)
	ebsPlugin := findContainer(t, cPS, "ebs-plugin")

	t.Run("debugLogs", func(t *testing.T) {
		if !hasArgAny(ebsPlugin, "-v=7", "--v=7") {
			t.Error("controller ebs-plugin should have -v=7 when debugLogs=true")
		}
	})

	t.Run("sdkDebugLog", func(t *testing.T) {
		if !hasArg(ebsPlugin, "--aws-sdk-debug-log=true") {
			t.Error("controller should have --aws-sdk-debug-log=true")
		}
	})
}

func TestMiscellaneous(t *testing.T) {
	resources := renderChart(t, "miscellaneous")
	controller := mustFind(t, resources, "Deployment", "ebs-csi-controller")
	cPS := podSpec(t, controller)

	ds := mustFind(t, resources, "DaemonSet", "ebs-csi-node")
	nPS := podSpec(t, ds)
	nodePlugin := findContainer(t, nPS, "ebs-plugin")

	t.Run("volumeModification", func(t *testing.T) {
		if !hasContainer(cPS, "volumemodifier") {
			t.Error("controller should have volumemodifier sidecar")
		}
	})

	t.Run("volumemodifierLogLevel", func(t *testing.T) {
		c := findContainer(t, cPS, "volumemodifier")
		if !hasArgAny(c, "-v=5", "--v=5") {
			t.Error("volumemodifier should have -v=5")
		}
	})

	t.Run("volumemodifierLeaderElection", func(t *testing.T) {
		c := findContainer(t, cPS, "volumemodifier")
		if !hasArg(c, "--leader-election=false") {
			t.Error("volumemodifier should have --leader-election=false")
		}
	})

	t.Run("volumeAttachLimit", func(t *testing.T) {
		if !hasArg(nodePlugin, "--volume-attach-limit=25") {
			t.Error("node should have --volume-attach-limit=25")
		}
	})

	t.Run("metadataLabeler", func(t *testing.T) {
		if !hasContainer(cPS, "metadata-labeler") {
			t.Error("controller should have metadata-labeler sidecar")
		}
	})

	t.Run("metadataLabelerLogLevel", func(t *testing.T) {
		c := findContainer(t, cPS, "metadata-labeler")
		if !hasArgAny(c, "-v=5", "--v=5") {
			t.Error("metadata-labeler should have -v=5")
		}
	})

	t.Run("additionalDaemonSets", func(t *testing.T) {
		extraDS := mustFind(t, resources, "DaemonSet", "ebs-csi-node-extra")
		extraPS := podSpec(t, extraDS)
		c := findContainer(t, extraPS, "ebs-plugin")
		if !hasArg(c, "--volume-attach-limit=15") {
			t.Error("additional DaemonSet should have --volume-attach-limit=15")
		}
	})
}

func TestNodeComponentOnly(t *testing.T) {
	resources := renderChart(t, "node-component-only")

	t.Run("noController", func(t *testing.T) {
		_, found := find(resources, "Deployment", "ebs-csi-controller")
		if found {
			t.Error("controller Deployment should not exist when nodeComponentOnly=true")
		}
	})

	t.Run("nodeExists", func(t *testing.T) {
		mustFind(t, resources, "DaemonSet", "ebs-csi-node")
	})
}

// TestUseOldCSIDriver verifies that setting useOldCSIDriver=true omits
// fsGroupPolicy from the rendered CSIDriver object.
func TestUseOldCSIDriver(t *testing.T) {
	resources := renderChartWithSet(t, "useOldCSIDriver=true")
	csiDriver := mustFind(t, resources, "CSIDriver", "ebs.csi.aws.com")

	t.Run("fsGroupPolicyAbsent", func(t *testing.T) {
		spec := nested(t, csiDriver, "spec")
		if _, ok := spec["fsGroupPolicy"]; ok {
			t.Error("CSIDriver spec should not have fsGroupPolicy when useOldCSIDriver=true")
		}
	})
}

// TestSelinux verifies that setting node.selinux=true sets seLinuxMount on the
// CSIDriver and adds the SELinux host mounts to the node ebs-plugin container.
// Skips the live cluster by asserting on rendered templates, so no
// SELinux-enabled nodes are required.
func TestSelinux(t *testing.T) {
	resources := renderChartWithSet(t, "node.selinux=true")
	csiDriver := mustFind(t, resources, "CSIDriver", "ebs.csi.aws.com")
	ds := mustFind(t, resources, "DaemonSet", "ebs-csi-node")
	nodePlugin := findContainer(t, podSpec(t, ds), "ebs-plugin")

	t.Run("seLinuxMountSet", func(t *testing.T) {
		spec := nested(t, csiDriver, "spec")
		if spec["seLinuxMount"] != true {
			t.Errorf("CSIDriver spec.seLinuxMount: got %v, want true", spec["seLinuxMount"])
		}
	})

	for _, wantPath := range []string{"/sys/fs/selinux", "/etc/selinux/config"} {
		t.Run("nodeMount/"+wantPath, func(t *testing.T) {
			mounts, _ := nodePlugin["volumeMounts"].([]interface{})
			var found bool
			for _, m := range mounts {
				if m.(obj)["mountPath"] == wantPath {
					found = true
				}
			}
			if !found {
				t.Errorf("node ebs-plugin should mount %s", wantPath)
			}
		})
	}
}
