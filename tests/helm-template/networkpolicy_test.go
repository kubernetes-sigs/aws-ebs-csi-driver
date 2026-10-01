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

// assertNetworkPolicyIngressHasPort checks that a NetworkPolicy allows ingress on the given port.
func assertNetworkPolicyIngressHasPort(t *testing.T, np obj, port int) {
	t.Helper()
	rules := nestedSlice(t, np, "spec", "ingress")
	for _, r := range rules {
		rm, ok := r.(obj)
		if !ok {
			continue
		}
		ports, ok := rm["ports"].([]interface{})
		if !ok {
			continue
		}
		for _, p := range ports {
			pm, ok := p.(obj)
			if !ok {
				continue
			}
			if pNum, ok := pm["port"].(float64); ok && int(pNum) == port {
				return
			}
		}
	}
	name, _ := nestedString(np, "metadata", "name")
	t.Errorf("NetworkPolicy %s should allow ingress on port %d", name, port)
}

// assertNetworkPolicyEgressHasIMDS checks that a NetworkPolicy allows egress to the
// EC2 instance metadata service, required when not using IRSA/Pod Identity.
func assertNetworkPolicyEgressHasIMDS(t *testing.T, np obj) {
	t.Helper()
	rules := nestedSlice(t, np, "spec", "egress")
	for _, r := range rules {
		rm, ok := r.(obj)
		if !ok {
			continue
		}
		to, ok := rm["to"].([]interface{})
		if !ok {
			continue
		}
		for _, dest := range to {
			dm, ok := dest.(obj)
			if !ok {
				continue
			}
			ipBlock, ok := dm["ipBlock"].(obj)
			if !ok {
				continue
			}
			if cidr, ok := ipBlock["cidr"].(string); ok && cidr == "169.254.169.254/32" {
				return
			}
		}
	}
	name, _ := nestedString(np, "metadata", "name")
	t.Errorf("NetworkPolicy %s should allow egress to IMDS (169.254.169.254/32)", name)
}

// assertNetworkPolicyIngressLacksPort checks that a NetworkPolicy does NOT allow
// ingress on the given port. Inverse of assertNetworkPolicyIngressHasPort.
func assertNetworkPolicyIngressLacksPort(t *testing.T, np obj, port int) {
	t.Helper()
	rules := nestedSlice(t, np, "spec", "ingress")
	for _, r := range rules {
		rm, ok := r.(obj)
		if !ok {
			continue
		}
		ports, ok := rm["ports"].([]interface{})
		if !ok {
			continue
		}
		for _, p := range ports {
			pm, ok := p.(obj)
			if !ok {
				continue
			}
			if pNum, ok := pm["port"].(float64); ok && int(pNum) == port {
				name, _ := nestedString(np, "metadata", "name")
				t.Errorf("NetworkPolicy %s should NOT allow ingress on port %d when metrics are disabled", name, port)
				return
			}
		}
	}
}

// Regression check: metrics ports must not appear in the rendered NetworkPolicy
// when enableMetrics is false (the chart default), even with networkPolicy.enabled=true.
func TestNetworkPolicyMetricsPortsAbsentWhenDisabled(t *testing.T) {
	resources := renderChartWithSet(t,
		"controller.networkPolicy.enabled=true",
		"node.networkPolicy.enabled=true",
		"controller.enableMetrics=false",
		"node.enableMetrics=false",
		"node.enableWindows=false",
	)

	controllerNP := mustFind(t, resources, "NetworkPolicy", "ebs-csi-controller")
	for _, port := range []int{3301, 3302, 3303, 3304, 3305, 3306, 3307} {
		assertNetworkPolicyIngressLacksPort(t, controllerNP, port)
	}
	// healthz must still be present even with metrics off.
	assertNetworkPolicyIngressHasPort(t, controllerNP, 9808)

	nodeNP := mustFind(t, resources, "NetworkPolicy", "ebs-csi-node")
	assertNetworkPolicyIngressLacksPort(t, nodeNP, 3302)
	assertNetworkPolicyIngressHasPort(t, nodeNP, 9808)
}

// testdata/networkpolicy.yaml enables metrics and supplies a custom ingress rule on
// both controller and node, so this single render proves the required baseline rules
// (health checks, metrics, IMDS egress) are always applied additively alongside any
// user-supplied rules, rather than being replaced by them.
func TestNetworkPolicy(t *testing.T) {
	resources := renderChart(t, "networkpolicy")

	controllerNP := mustFind(t, resources, "NetworkPolicy", "ebs-csi-controller")
	sel := nested(t, controllerNP, "spec", "podSelector", "matchLabels")
	if sel["app"] != "ebs-csi-controller" {
		t.Errorf("controller NetworkPolicy podSelector app: got %v, want ebs-csi-controller", sel["app"])
	}

	hasIngress, hasEgress := false, false
	for _, pt := range nestedSlice(t, controllerNP, "spec", "policyTypes") {
		switch pt {
		case "Ingress":
			hasIngress = true
		case "Egress":
			hasEgress = true
		}
	}
	if !hasIngress || !hasEgress {
		t.Errorf("controller NetworkPolicy policyTypes: got Ingress=%v Egress=%v, want both true", hasIngress, hasEgress)
	}
	// Regression check: kubelet must be able to reach /healthz regardless of enableMetrics
	// or any user-supplied ingress rules.
	assertNetworkPolicyIngressHasPort(t, controllerNP, 9808)
	// Metrics ports must still be present even though a custom ingress rule was also supplied.
	assertNetworkPolicyIngressHasPort(t, controllerNP, 3301)
	// The custom ingress rule must be appended, not dropped.
	assertNetworkPolicyIngressHasPort(t, controllerNP, 9999)
	// Regression check: controller must be able to reach IMDS for node-role credentials
	// when IRSA/Pod Identity is not configured (the default).
	assertNetworkPolicyEgressHasIMDS(t, controllerNP)
	assertNetworkPolicyEgressHasPort(t, controllerNP, 443)

	nodeNP := mustFind(t, resources, "NetworkPolicy", "ebs-csi-node")
	nodeSel := nested(t, nodeNP, "spec", "podSelector", "matchLabels")
	if nodeSel["app"] != "ebs-csi-node" {
		t.Errorf("node NetworkPolicy podSelector app: got %v, want ebs-csi-node", nodeSel["app"])
	}
	assertNetworkPolicyIngressHasPort(t, nodeNP, 9808)
	assertNetworkPolicyIngressHasPort(t, nodeNP, 9809)
	assertNetworkPolicyIngressHasPort(t, nodeNP, 3302)
	assertNetworkPolicyIngressHasPort(t, nodeNP, 9999)
	assertNetworkPolicyEgressHasIMDS(t, nodeNP)
	assertNetworkPolicyEgressHasPort(t, nodeNP, 443)
}

func TestNetworkPolicyDisabledByDefault(t *testing.T) {
	// No testdata/default.yaml exists in this harness; renderChartWithSet with
	// zero --set flags renders the chart's pure built-in defaults.
	resources := renderChartWithSet(t)

	if _, ok := find(resources, "NetworkPolicy", "ebs-csi-controller"); ok {
		t.Error("NetworkPolicy must not render when controller.networkPolicy.enabled is unset (default false)")
	}
	if _, ok := find(resources, "NetworkPolicy", "ebs-csi-node"); ok {
		t.Error("NetworkPolicy must not render when node.networkPolicy.enabled is unset (default false)")
	}
}

func TestNetworkPolicyAdditionalDaemonSet(t *testing.T) {
	resources := renderChart(t, "networkpolicy-additional-daemonset")

	extraNP := mustFind(t, resources, "NetworkPolicy", "ebs-csi-node-extra")
	sel := nested(t, extraNP, "spec", "podSelector", "matchLabels")
	if sel["app"] != "ebs-csi-node-extra" {
		t.Errorf("additional daemonset NetworkPolicy podSelector app: got %v, want ebs-csi-node-extra", sel["app"])
	}
}

// assertNetworkPolicyEgressHasPort checks that a NetworkPolicy allows egress on the given port.
func assertNetworkPolicyEgressHasPort(t *testing.T, np obj, port int) {
	t.Helper()
	rules := nestedSlice(t, np, "spec", "egress")
	for _, r := range rules {
		rm, ok := r.(obj)
		if !ok {
			continue
		}
		ports, ok := rm["ports"].([]interface{})
		if !ok {
			continue
		}
		for _, p := range ports {
			pm, ok := p.(obj)
			if !ok {
				continue
			}
			if pNum, ok := pm["port"].(float64); ok && int(pNum) == port {
				return
			}
		}
	}
	name, _ := nestedString(np, "metadata", "name")
	t.Errorf("NetworkPolicy %s should allow egress on port %d", name, port)
}

// Regression check: enabling node.networkPolicy on a hostNetwork daemonset must
// fail the render loudly rather than silently produce an inert NetworkPolicy,
// since Kubernetes NetworkPolicy is not enforced against hostNetwork pods.
func TestNetworkPolicyFailsWithHostNetwork(t *testing.T) {
	renderChartWithSetExpectError(t,
		"does not apply to pods using the host network namespace",
		"node.networkPolicy.enabled=true",
		"node.hostNetwork=true",
		"node.enableWindows=false",
	)
}

// Regression check: node.networkPolicy.enabled=true must fail the render when
// node.enableWindows=true (the chart default), since Windows node pods always
// run with hostNetwork=true and are therefore unprotected by the policy.
func TestNetworkPolicyFailsWithWindowsEnabled(t *testing.T) {
	renderChartWithSetExpectError(t,
		"Windows node pods always run with hostNetwork=true",
		"node.networkPolicy.enabled=true",
		"node.enableWindows=true",
	)
}

// assertNetworkPolicyEgressHasCIDR checks that a NetworkPolicy allows egress to
// the given ipBlock CIDR. Generic version of assertNetworkPolicyEgressHasIMDS.
func assertNetworkPolicyEgressHasCIDR(t *testing.T, np obj, cidr string) {
	t.Helper()
	rules := nestedSlice(t, np, "spec", "egress")
	for _, r := range rules {
		rm, ok := r.(obj)
		if !ok {
			continue
		}
		to, ok := rm["to"].([]interface{})
		if !ok {
			continue
		}
		for _, dest := range to {
			dm, ok := dest.(obj)
			if !ok {
				continue
			}
			ipBlock, ok := dm["ipBlock"].(obj)
			if !ok {
				continue
			}
			if c, ok := ipBlock["cidr"].(string); ok && c == cidr {
				return
			}
		}
	}
	name, _ := nestedString(np, "metadata", "name")
	t.Errorf("NetworkPolicy %s should allow egress to %s", name, cidr)
}

// Regression check: IMDS IPv6 and EKS Pod Identity egress must be present by
// default, alongside the existing IMDS IPv4 rule.
func TestNetworkPolicyIMDSIPv6AndPodIdentity(t *testing.T) {
	resources := renderChartWithSet(t,
		"controller.networkPolicy.enabled=true",
		"node.networkPolicy.enabled=true",
		"node.enableWindows=false",
	)

	controllerNP := mustFind(t, resources, "NetworkPolicy", "ebs-csi-controller")
	assertNetworkPolicyEgressHasCIDR(t, controllerNP, "169.254.169.254/32")
	assertNetworkPolicyEgressHasCIDR(t, controllerNP, "fd00:ec2::254/128")
	assertNetworkPolicyEgressHasCIDR(t, controllerNP, "169.254.170.23/32")

	nodeNP := mustFind(t, resources, "NetworkPolicy", "ebs-csi-node")
	assertNetworkPolicyEgressHasCIDR(t, nodeNP, "169.254.169.254/32")
	assertNetworkPolicyEgressHasCIDR(t, nodeNP, "fd00:ec2::254/128")
	assertNetworkPolicyEgressHasCIDR(t, nodeNP, "169.254.170.23/32")
}
