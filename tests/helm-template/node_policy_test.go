/*
Copyright 2026 The Kubernetes Authors.

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
	"bytes"
	"os/exec"
	"testing"
)

// TestNodeAdmissionPolicy validates when the chart renders the ValidatingAdmissionPolicy
// that limits the EBS CSI node pods' service account to removing the agent-not-ready taint from the Node each pod runs on.
func TestNodeAdmissionPolicy(t *testing.T) {
	render := func(t *testing.T, args ...string) []obj {
		t.Helper()
		args = append([]string{"template", releaseName, chartPath(), "--namespace", "ebs"}, args...)
		cmd := exec.CommandContext(t.Context(), helmBin(), args...)
		var stdout, stderr bytes.Buffer
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr
		if err := cmd.Run(); err != nil {
			t.Fatalf("helm template failed: %v\nstderr: %s", err, stderr.String())
		}
		return parseYAMLDocs(t, stdout.Bytes())
	}

	tests := []struct {
		name         string
		args         []string
		want         bool
		wantUsername string
	}{
		{
			name:         "rendered on 1.30",
			args:         []string{"--kube-version", "1.30.0"},
			want:         true,
			wantUsername: "system:serviceaccount:ebs:ebs-csi-node-sa",
		},
		{
			name: "skipped on 1.29",
			args: []string{"--kube-version", "1.29.0"},
		},
		{
			name: "skipped when disabled",
			args: []string{"--kube-version", "1.30.0", "--set", "node.validatingAdmissionPolicy.enabled=false"},
		},
		{
			name: "skipped with disableMutation",
			args: []string{"--kube-version", "1.30.0", "--set", "node.serviceAccount.disableMutation=true"},
		},
		{
			name:         "namespaceOverride",
			args:         []string{"--kube-version", "1.30.0", "--set", "node.namespaceOverride=ebs-node"},
			want:         true,
			wantUsername: "system:serviceaccount:ebs-node:ebs-csi-node-sa",
		},
		{
			name:         "existing service account",
			args:         []string{"--kube-version", "1.30.0", "--set", "node.serviceAccount.create=false", "--set", "node.serviceAccount.name=my-node-sa"},
			want:         true,
			wantUsername: "system:serviceaccount:ebs:my-node-sa",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			resources := render(t, tc.args...)
			policy, gotPolicy := find(resources, "ValidatingAdmissionPolicy", "ebs-csi-node")
			binding, gotBinding := find(resources, "ValidatingAdmissionPolicyBinding", "ebs-csi-node")
			if gotPolicy != tc.want || gotBinding != tc.want {
				t.Fatalf("policy rendered: %v, binding rendered: %v, want %v", gotPolicy, gotBinding, tc.want)
			}
			if !tc.want {
				return
			}
			if got := nested(t, binding, "spec")["policyName"]; got != "ebs-csi-node" {
				t.Errorf("binding policyName: got %v, want ebs-csi-node", got)
			}
			conds := nestedSlice(t, nested(t, policy, "spec"), "matchConditions")
			wantExpr := `request.userInfo.username == "` + tc.wantUsername + `"`
			if len(conds) != 1 || mustObj(t, conds[0])["expression"] != wantExpr {
				t.Errorf("matchConditions: got %v, want one with expression %s", conds, wantExpr)
			}
		})
	}
}
