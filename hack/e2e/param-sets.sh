#!/bin/bash

# Copyright 2025 The Kubernetes Authors.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

# Parameter set definitions for e2e parameter tests.
# Each set defines GINKGO_FOCUS, HELM_EXTRA_FLAGS, and optionally other env vars.
#
# Sets in PARAM_SETS_ALL (no special cluster config needed):
#   standard            - Behavioral params (tagging, metrics, logging, storage classes, etc.)
#   volume-modification - Requires volumeModificationFeature + feature gates
#   volume-attach-limit - Mutually exclusive with reservedVolumeAttachments in standard
#   debug               - debugLogs=true overrides individual logLevel settings
#   infra               - Infrastructure/deployment params (resources, security, strategy, etc.)
#   fips                - Builds FIPS image then validates it is deployed
#
# Special-cluster sets (not in PARAM_SETS_ALL):
#   metadata-labeler    - Needs IMDS/metadata access
#   legacy-compat       - Legacy CSIDriver + XFS behavior
#   selinux             - Needs SELinux-enabled nodes

set -euo pipefail

PARAM_SETS_ALL="standard volume-modification volume-attach-limit debug infra fips"

param_set_standard() {
  GINKGO_FOCUS="\[ebs-csi-e2e\] \[single-az\] \[param:(extraCreateMetadata|k8sTagClusterId|extraVolumeTags|controllerMetrics|nodeMetrics|batching|defaultFsType|controllerLoggingFormat|nodeLoggingFormat|controllerLogLevel|nodeLogLevel|provisionerLogLevel|attacherLogLevel|snapshotterLogLevel|resizerLogLevel|nodeDriverRegistrarLogLevel|storageClasses|volumeSnapshotClasses|defaultStorageClass|snapshotterForceEnable|controllerUserAgentExtra|controllerEnablePrometheusAnnotations|nodeEnablePrometheusAnnotations|nodeKubeletPath|nodeTolerateAllTaints|controllerPodDisruptionBudget|provisionerLeaderElection|attacherLeaderElection|resizerLeaderElection|reservedVolumeAttachments|hostNetwork|nodeDisableMutation|nodeTerminationGracePeriod)\]"
  HELM_EXTRA_FLAGS="--set=controller.extraCreateMetadata=true,controller.k8sTagClusterId=e2e-param-test,controller.extraVolumeTags.TestKey=TestValue,controller.enableMetrics=true,node.enableMetrics=true,controller.batching=true,controller.defaultFsType=xfs,controller.loggingFormat=json,node.loggingFormat=json,controller.logLevel=4,node.logLevel=4,sidecars.provisioner.logLevel=4,sidecars.attacher.logLevel=4,sidecars.snapshotter.logLevel=4,sidecars.resizer.logLevel=4,sidecars.nodeDriverRegistrar.logLevel=4,defaultStorageClass.enabled=true,storageClasses[0].name=test-sc,storageClasses[0].parameters.type=gp3,volumeSnapshotClasses[0].name=test-vsc,volumeSnapshotClasses[0].deletionPolicy=Delete,sidecars.snapshotter.forceEnable=true,controller.userAgentExtra=e2e-test,controller.enablePrometheusAnnotations=true,node.enablePrometheusAnnotations=true,node.kubeletPath=/var/lib/kubelet,node.tolerateAllTaints=true,controller.podDisruptionBudget.enabled=true,sidecars.provisioner.leaderElection.enabled=true,sidecars.attacher.leaderElection.enabled=true,sidecars.resizer.leaderElection.enabled=true,node.reservedVolumeAttachments=2,node.hostNetwork=true,node.serviceAccount.disableMutation=true,node.terminationGracePeriodSeconds=60"
}

param_set_volume-modification() {
  GINKGO_FOCUS="\[ebs-csi-e2e\] \[single-az\] \[param:(volumeModification|volumemodifierLogLevel|volumemodifierLeaderElection)\]"
  HELM_EXTRA_FLAGS="--set=controller.volumeModificationFeature.enabled=true,sidecars.provisioner.additionalArgs[0]='--feature-gates=VolumeAttributesClass=true',sidecars.resizer.additionalArgs[0]='--feature-gates=VolumeAttributesClass=true',sidecars.volumemodifier.logLevel=4,sidecars.volumemodifier.leaderElection.enabled=false"
}

# volume-attach-limit must be separate because volumeAttachLimit and reservedVolumeAttachments are mutually exclusive
param_set_volume-attach-limit() {
  GINKGO_FOCUS="\[ebs-csi-e2e\] \[single-az\] \[param:volumeAttachLimit\]"
  HELM_EXTRA_FLAGS="--set=node.volumeAttachLimit=25"
}

# debugLogs=true overrides individual logLevel settings, so this must be separate from standard
param_set_debug() {
  GINKGO_FOCUS="\[ebs-csi-e2e\] \[single-az\] \[param:(debugLogs|sdkDebugLog)\]"
  HELM_EXTRA_FLAGS="--set=debugLogs=true,controller.sdkDebugLog=true"
}

param_set_infra() {
  GINKGO_FOCUS="\[ebs-csi-e2e\] \[single-az\] \[param:(controllerReplicaCount|controllerPriorityClassName|controllerResources|controllerPodAnnotations|controllerPodLabels|controllerDeploymentAnnotations|controllerRevisionHistoryLimit|nodePriorityClassName|nodeResources|nodePodAnnotations|nodeDaemonSetAnnotations|nodeRevisionHistoryLimit|provisionerResources|attacherResources|snapshotterResources|resizerResources|nodeDriverRegistrarResources|livenessProbeResources|customLabels|controllerEnv|nodeEnv|controllerTopologySpreadConstraints|controllerSecurityContext|nodeSecurityContext|controllerContainerSecurityContext|controllerVolumes|controllerVolumeMounts|nodeVolumes|nodeVolumeMounts|controllerDnsConfig|nodeDnsConfig|controllerInitContainers|nodeInitContainers|imagePullPolicy|controllerUpdateStrategy|nodeUpdateStrategy)\]"
  HELM_EXTRA_FLAGS="--set=controller.replicaCount=3,controller.priorityClassName=system-cluster-critical,controller.resources.requests.cpu=100m,controller.resources.limits.memory=256Mi,controller.podAnnotations.test-annotation=test-value,controller.podLabels.test-label=test-value,controller.deploymentAnnotations.deploy-annotation=deploy-value,controller.revisionHistoryLimit=5,node.priorityClassName=system-node-critical,node.resources.requests.cpu=50m,node.resources.limits.memory=128Mi,node.podAnnotations.node-annotation=node-value,node.daemonSetAnnotations.ds-annotation=ds-value,node.revisionHistoryLimit=3,sidecars.provisioner.resources.requests.cpu=20m,sidecars.attacher.resources.requests.cpu=15m,sidecars.snapshotter.resources.requests.cpu=15m,sidecars.resizer.resources.requests.cpu=15m,sidecars.nodeDriverRegistrar.resources.requests.cpu=10m,sidecars.livenessProbe.resources.requests.cpu=5m,customLabels.custom-label=custom-value,controller.env[0].name=TEST_ENV,controller.env[0].value=test-value,node.env[0].name=NODE_ENV,node.env[0].value=node-value,controller.topologySpreadConstraints[0].maxSkew=1,controller.topologySpreadConstraints[0].topologyKey=topology.kubernetes.io/zone,controller.topologySpreadConstraints[0].whenUnsatisfiable=ScheduleAnyway,controller.securityContext.runAsNonRoot=true,controller.containerSecurityContext.readOnlyRootFilesystem=true,controller.volumes[0].name=extra-volume,controller.volumes[0].configMap.name=kube-root-ca.crt,controller.volumeMounts[0].name=extra-volume,controller.volumeMounts[0].mountPath=/extra,node.volumes[0].name=node-extra-volume,node.volumes[0].configMap.name=kube-root-ca.crt,node.volumeMounts[0].name=node-extra-volume,node.volumeMounts[0].mountPath=/node-extra,controller.dnsConfig.nameservers[0]=8.8.8.8,node.dnsConfig.nameservers[0]=8.8.4.4,controller.initContainers[0].name=init-container,controller.initContainers[0].image=busybox,controller.initContainers[0].command[0]=echo,controller.initContainers[0].command[1]=init,node.initContainers[0].name=node-init-container,node.initContainers[0].image=busybox,node.initContainers[0].command[0]=echo,node.initContainers[0].command[1]=node-init,image.pullPolicy=Always,controller.updateStrategy.type=Recreate,controller.updateStrategy.rollingUpdate=null,node.updateStrategy.type=OnDelete"
}

param_set_metadata-labeler() {
  GINKGO_FOCUS="\[ebs-csi-e2e\] \[single-az\] \[param:(metadataLabeler|metadataLabelerLogLevel)\]"
  GINKGO_PARALLEL=1
  EBS_INSTALL_SNAPSHOT=false
  HELM_EXTRA_FLAGS="--set=sidecars.metadataLabeler.enabled=true,node.metadataSources='metadata-labeler',sidecars.metadataLabeler.logLevel=4"
}

param_set_legacy-compat() {
  GINKGO_FOCUS="\[ebs-csi-e2e\] \[single-az\] \[param:(useOldCSIDriver|legacyXFS)\]"
  HELM_EXTRA_FLAGS="--set=useOldCSIDriver=true,node.legacyXFS=true"
}

param_set_selinux() {
  GINKGO_FOCUS="\[ebs-csi-e2e\] \[single-az\] \[param:selinux\]"
  HELM_EXTRA_FLAGS="--set=node.selinux=true"
}

param_set_fips() {
  GINKGO_FOCUS="\[ebs-csi-e2e\] \[single-az\] \[param:fips\]"
  FIPS_TEST=true
  HELM_EXTRA_FLAGS="--set=fips=true"
  # Build and push FIPS image (appends -fips suffix to tag)
  FIPS_TEST=true make cluster/image
}

# Load a parameter set by name, exporting GINKGO_FOCUS and HELM_EXTRA_FLAGS
load_param_set() {
  local name="$1"
  local func="param_set_${name}"
  if ! declare -f "$func" > /dev/null 2>&1; then
    echo "Unknown parameter set: ${name}" >&2
    echo "Available sets: standard, volume-modification, volume-attach-limit, debug, infra, metadata-labeler, legacy-compat, selinux, fips" >&2
    exit 1
  fi
  "$func"
  export GINKGO_FOCUS HELM_EXTRA_FLAGS
  export GINKGO_PARALLEL="${GINKGO_PARALLEL:-5}"
  export AWS_AVAILABILITY_ZONES="${AWS_AVAILABILITY_ZONES:-us-west-2a}"
  export TEST_PATH="${TEST_PATH:-./tests/e2e/...}"
  export JUNIT_REPORT="${REPORT_DIR:-/logs/artifacts}/junit-params-${name}.xml"
  # Export optional vars if set by the param set function
  if [[ -n "${EBS_INSTALL_SNAPSHOT+x}" ]]; then export EBS_INSTALL_SNAPSHOT; fi
  if [[ -n "${FIPS_TEST+x}" ]]; then export FIPS_TEST; fi
}

# Run a single parameter set
run_param_set() {
  load_param_set "$1"
  echo "### Running parameter set: $1"
  ./hack/e2e/run.sh
}

# Merge per-set JUnit XMLs into a single file with duplicate skipped tests removed.
# Each Ginkgo run reports ALL specs (most as skipped), so the same skipped test appears
# in every per-set file. This merges all results into one file, keeping non-skipped results
# (passed/failed) over skipped duplicates, and emitting each skipped test only once.
merge_junit_results() {
  local report_dir="${REPORT_DIR:-/logs/artifacts}"
  local output="${report_dir}/junit-params.xml"

  python3 - "$report_dir" "$output" <<'PYEOF'
import glob, sys, xml.etree.ElementTree as ET

report_dir, output = sys.argv[1], sys.argv[2]
merged = {}
time_total = 0.0

def priority(tc):
    if tc.find("failure") is not None or tc.find("error") is not None:
        return 2  # failed/errored
    if tc.find("skipped") is not None:
        return 0  # skipped
    return 1      # passed

for path in sorted(glob.glob(f"{report_dir}/junit-params-*.xml")):
    tree = ET.parse(path)
    time_total += float(tree.getroot().get("time", "0"))
    for tc in tree.iter("testcase"):
        name = tc.get("name", "")
        if name not in merged or priority(tc) > priority(merged[name]):
            merged[name] = tc

tests = list(merged.values())
skipped = sum(1 for tc in tests if tc.find("skipped") is not None)
failed = sum(1 for tc in tests if tc.find("failure") is not None)
errored = sum(1 for tc in tests if tc.find("error") is not None)

root = ET.Element("testsuites", tests=str(len(tests)), disabled=str(skipped),
                  errors=str(errored), failures=str(failed), time=str(time_total))
suite = ET.SubElement(root, "testsuite", name="AWS EBS CSI Driver Parameter Tests",
                      tests=str(len(tests)), skipped=str(skipped),
                      errors=str(errored), failures=str(failed), time=str(time_total))
for tc in tests:
    suite.append(tc)

ET.ElementTree(root).write(output, xml_declaration=True, encoding="UTF-8")
PYEOF

  # Remove per-set files only if merge succeeded, so CI only sees the merged result
  if [[ $? -eq 0 && -f "$output" ]]; then
    rm -f "${report_dir}"/junit-params-*.xml
    echo "Merged JUnit results into ${output}"
  else
    echo "WARNING: JUnit merge failed, keeping per-set files" >&2
  fi
}

# Run all standard parameter sets sequentially
run_all_param_sets() {
  echo "Running all parameter sets sequentially..."
  for set in $PARAM_SETS_ALL; do
    run_param_set "$set"
  done
  merge_junit_results
  echo "All parameter sets completed successfully!"
}

# Allow direct invocation: ./hack/e2e/param-sets.sh run <name> or ./hack/e2e/param-sets.sh run-all
if [[ "${BASH_SOURCE[0]}" == "${0}" ]]; then
  case "${1:-}" in
    run)
      [[ -z "${2:-}" ]] && { echo "Usage: $0 run <param-set-name>" >&2; exit 1; }
      run_param_set "$2"
      merge_junit_results
      ;;
    run-all)
      run_all_param_sets
      ;;
    *)
      echo "Usage: $0 {run <param-set-name>|run-all}" >&2
      exit 1
      ;;
  esac
fi
