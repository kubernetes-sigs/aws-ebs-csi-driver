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

### Helper script to create/delete eks ebs-scale-test clusters and install add-ons.

set -euo pipefail

# We expect this helper script is sourced from hack/ebs-scale-test
path_to_cluster_setup_dir="${BASE_DIR}/helpers/cluster-setup/"

## Cluster

create_cluster() {
  if eksctl get cluster --name "$CLUSTER_NAME" --region "$AWS_REGION" >/dev/null 2>&1; then
    echo "EKS cluster '$CLUSTER_NAME' already up in $AWS_REGION."
    aws eks update-kubeconfig --name "$CLUSTER_NAME" --region "$AWS_REGION"
  else
    echo "Deploying EKS cluster. See configuration in $EXPORT_DIR/cluster-config.yaml"
    gomplate -f "$path_to_cluster_setup_dir/scale-cluster-config.yaml" -o "$EXPORT_DIR/cluster-config.yaml"
    eksctl create cluster -f "$EXPORT_DIR/cluster-config.yaml"
    aws eks update-kubeconfig --name "$CLUSTER_NAME" --region "$AWS_REGION"
  fi
}

cleanup_cluster() {
  eksctl delete cluster "$CLUSTER_NAME"
}

## Misc

check_lingering_volumes() {
  lingering_vol_count=$(aws ec2 describe-volumes \
    --filters "Name=tag:ebs-scale-test,Values=${SCALABILITY_TEST_RUN_NAME}" \
    --query 'length(Volumes[*])' \
    --output text)

  [[ lingering_vol_count -ne 0 ]] && echo "WARNING: detected $lingering_vol_count lingering ebs-scale-test EBS volumes in $AWS_ACCOUNT_ID. Please run \`aws ec2 describe-volumes --filters 'Name=tag-key,Values=ebs-scale-test'\` and audit their AWS resource tags. Note these volumes may belong to a different scalability run than $SCALABILITY_TEST_RUN_NAME"
}

check_lingering_snapshots() {
  lingering_snap_count=$(aws ec2 describe-snapshots \
    --filters "Name=tag:ebs-scale-test,Values=${SCALABILITY_TEST_RUN_NAME}" \
    --query 'length(Snapshots[*])' \
    --output text)

  [[ lingering_snap_count -ne 0 ]] && echo "WARNING: detected $lingering_snap_count lingering ebs-scale-test EBS snapshots from run ${SCALABILITY_TEST_RUN_NAME} in $AWS_ACCOUNT_ID. Please run \`aws ec2 describe-snapshots --filters 'Name=tag:ebs-scale-test,Values=${SCALABILITY_TEST_RUN_NAME}'\` and audit their AWS resource tags."
}

## EBS CSI Driver

deploy_ebs_csi_driver() {
  path_to_chart="${BASE_DIR}/../../charts/aws-ebs-csi-driver"
  echo "Deploying EBS CSI driver from chart $path_to_chart"

  # We use helm install instead of upgrade to ensure the release does not already exist
  helm install aws-ebs-csi-driver \
    --namespace kube-system \
    --values "$DRIVER_VALUES_FILEPATH" \
    --wait \
    --timeout 15m \
    "$path_to_chart"
}

## Karpenter

deploy_karpenter() {
  echo "Deploying Karpenter Pre-requisites"
  echo "Deploying Karpenter-${CLUSTER_NAME} CF Stack"
  curl -fsSL https://raw.githubusercontent.com/aws/karpenter-provider-aws/v"${KARPENTER_VERSION}"/website/content/en/preview/getting-started/getting-started-with-karpenter/cloudformation.yaml >"$TEMPOUT" &&
    aws cloudformation deploy \
      --stack-name "Karpenter-${CLUSTER_NAME}" \
      --template-file "$TEMPOUT" \
      --capabilities CAPABILITY_NAMED_IAM \
      --parameter-overrides "ClusterName=${CLUSTER_NAME}"

  echo "Creating Karpenter iamidentitymapping"
  eksctl create iamidentitymapping \
    --cluster "${CLUSTER_NAME}" \
    --region="${AWS_REGION}" \
    --arn "arn:aws:iam::${AWS_ACCOUNT_ID}:role/KarpenterNodeRole-${CLUSTER_NAME}" \
    --group "system:bootstrappers" \
    --group "system:nodes" \
    --username "system:node:{{EC2PrivateDNSName}}"

  echo "Creating Karpenter podidentityassociation"
  eksctl create podidentityassociation \
    --cluster "${CLUSTER_NAME}" \
    --namespace kube-system \
    --service-account-name karpenter \
    --role-name "${CLUSTER_NAME}-karpenter" \
    --permission-policy-arns "arn:aws:iam::${AWS_ACCOUNT_ID}:policy/KarpenterControllerPolicy-${CLUSTER_NAME}" || true

  aws iam create-service-linked-role --aws-service-name spot.amazonaws.com >/dev/null 2>&1 || true

  KARPENTER_IAM_ROLE_ARN="arn:aws:iam::${AWS_ACCOUNT_ID}:role/${CLUSTER_NAME}-karpenter"
  echo "Karpenter IAM Role: ${KARPENTER_IAM_ROLE_ARN}"

  echo "Installing Karpenter to cluster"
  helm registry logout public.ecr.aws || true

  helm upgrade --install karpenter oci://public.ecr.aws/karpenter/karpenter --version "${KARPENTER_VERSION}" --namespace "kube-system" --create-namespace \
    --set "settings.clusterName=${CLUSTER_NAME}" \
    --set "settings.interruptionQueue=${CLUSTER_NAME}" \
    --set controller.resources.requests.cpu=1 \
    --set controller.resources.requests.memory=2Gi \
    --set controller.resources.limits.cpu=2 \
    --set controller.resources.limits.memory=2Gi \
    --wait \
    --timeout 15m

  echo "Deploying ebs-scale-test NodePool & EC2NodeClass to cluster"
  gomplate -f "$path_to_cluster_setup_dir/nodes-karpenter.yaml" | kubectl apply -f -
}

cleanup_karpenter() {
  # Ignore failures to preserve idempotency. Recommended by official Karpenter uninstallation guide.
  echo "Cleaning up Karpenter resources. Should see 'Karpenter Cleanup Complete'"

  # Karpenter needs to delete all instances it manages before uninstalled. Otherwise instances may be orphaned.
  kubectl delete nodepools --all --wait=true --timeout=1200s || true
  kubectl delete ec2nodeclass --all --wait=true || true
  kubectl delete nodeclaims --all --wait=true || true

  helm uninstall karpenter -n "kube-system" || true
  eksctl delete podidentityassociation --cluster "${CLUSTER_NAME}" --namespace kube-system --service-account-name karpenter || true
  aws ec2 describe-launch-templates --filters "Name=tag:karpenter.k8s.aws/cluster,Values=${CLUSTER_NAME}" |
    jq -r ".LaunchTemplates[].LaunchTemplateName" |
    xargs -I{} aws ec2 delete-launch-template --launch-template-name {}
  aws cloudformation delete-stack --stack-name "Karpenter-${CLUSTER_NAME}" || true

  echo "Karpenter cleanup complete, but check EC2 Console to ensure no lingering instances"
}

(return 0 2>/dev/null) || (
  echo "This script is not meant to be run directly, only sourced as a helper!"
  exit 1
)

deploy_snapshot_controller() {
  echo "Deploying snapshot controller and CRDs"
  kubectl apply -f https://raw.githubusercontent.com/kubernetes-csi/external-snapshotter/"${EBS_INSTALL_SNAPSHOT_VERSION}"/deploy/kubernetes/snapshot-controller/rbac-snapshot-controller.yaml
  kubectl apply -f https://raw.githubusercontent.com/kubernetes-csi/external-snapshotter/"${EBS_INSTALL_SNAPSHOT_VERSION}"/deploy/kubernetes/snapshot-controller/setup-snapshot-controller.yaml
  kubectl apply -f https://raw.githubusercontent.com/kubernetes-csi/external-snapshotter/"${EBS_INSTALL_SNAPSHOT_VERSION}"/client/config/crd/snapshot.storage.k8s.io_volumesnapshotclasses.yaml
  kubectl apply -f https://raw.githubusercontent.com/kubernetes-csi/external-snapshotter/"${EBS_INSTALL_SNAPSHOT_VERSION}"/client/config/crd/snapshot.storage.k8s.io_volumesnapshotcontents.yaml
  kubectl apply -f https://raw.githubusercontent.com/kubernetes-csi/external-snapshotter/"${EBS_INSTALL_SNAPSHOT_VERSION}"/client/config/crd/snapshot.storage.k8s.io_volumesnapshots.yaml
}
