#!/bin/bash

set -euo pipefail

metrics_collector() {
  readonly KUBECONFIG="$1"
  readonly AWS_ACCOUNT_ID="$2"
  readonly AWS_REGION="$3"
  readonly NODE_OS_DISTRO="$4"
  readonly DEPLOYMENT_TIME="$5"
  readonly DRIVER_NAME="$6"
  readonly DRIVER_VERSION="$7"
  readonly METRICS_BASE_DIR=$(dirname "$(realpath "${BASH_SOURCE[0]}")")
  readonly METRICS_DIR_NAME="metrics-$(git rev-parse HEAD)1-${NODE_OS_DISTRO}-${DRIVER_VERSION}"
  readonly METRICS_DIR_PATH="${METRICS_BASE_DIR}/../csi-test-artifacts/${METRICS_DIR_NAME}"
  readonly METRICS_SERVER_URL="https://github.com/kubernetes-sigs/metrics-server/releases/latest/download/components.yaml"
  readonly STORAGE_CLASS="${METRICS_BASE_DIR}/storageclass.yaml"
  readonly CLUSTER_LOADER_CONFIG="${METRICS_BASE_DIR}/cl2-config.yaml"
  readonly CLUSTER_LOADER_OVERRIDE="${METRICS_BASE_DIR}/override.yaml"
  readonly PERF_TESTS_DIR="${METRICS_BASE_DIR}/../csi-test-artifacts/perf-tests"
  readonly PERF_TESTS_URL="https://github.com/kubernetes/perf-tests.git"
  readonly SOURCE_BUCKET="${DRIVER_NAME}-metrics-${AWS_ACCOUNT_ID}"
  readonly LAMBDA_ARN="arn:aws:lambda:us-west-2:204115867930:function:EbsCsiDriverMetricsProcessor"
  readonly LAMBDA_ROLE="arn:aws:iam::204115867930:role/EbsCsiDriverMetricsProces-LambdaExecutionRoleD5C26-1NPQTJ6QEPGNE"

  check_dependencies
  collect_metrics
  upload_metrics
}

log() {
  printf "%s [INFO] - %s\n" "$(date +"%Y-%m-%d %H:%M:%S")" "${*}" >&2
}

check_dependencies() {
  local readonly dependencies=("kubectl" "git" "kubetest2" "aws")

  for cmd in "${dependencies[@]}"; do
    if ! command -v "${cmd}" &>/dev/null; then
      log "${cmd} could not be found, please install it."
      exit 1
    fi
  done
}

collect_metrics() {
  log "Collecting metrics in $METRICS_DIR_PATH"
  mkdir -p "$METRICS_DIR_PATH"
  
  log "Collecting deployment time"
  echo -e "$DEPLOYMENT_TIME" > "$METRICS_DIR_PATH/deployment_time.txt"

  log "Collecting resource metrics"
  install_metrics_server
  check_pod_metrics
  collect_resource_metrics "kube-system" "app=ebs-csi-node" "$METRICS_DIR_PATH/node_resource_metrics.yaml"
  collect_resource_metrics "kube-system" "app=ebs-csi-controller" "$METRICS_DIR_PATH/controller_resource_metrics.yaml"

  log "Collecting clusterloader2 metrics"
  collect_clusterloader2_metrics
}

install_metrics_server() {
  log "Deploying metrics server"
  kubectl apply -f "$METRICS_SERVER_URL" --kubeconfig "$KUBECONFIG"

  kubectl wait \
    --namespace kube-system \
    --for=condition=ready pod \
    --selector=k8s-app=metrics-server \
    --kubeconfig="$KUBECONFIG" \
    --timeout=90s
}

check_pod_metrics() {
  local readonly max_retries=30
  local readonly retry_interval=10

  for ((i = 1; i <= max_retries; i++)); do
    if kubectl get podmetrics.metrics.k8s.io --all-namespaces --kubeconfig "$KUBECONFIG" >/dev/null 2>&1; then
      log "PodMetrics is available"
      return 0
    else
      log "PodMetrics is not available yet, retrying in ${retry_interval} seconds..."
      sleep "${retry_interval}"
    fi
  done

  log "PodMetrics did not become available after ${max_retries} retries"
  return 1
}

collect_resource_metrics() {
  local readonly namespace="$1"
  local readonly label="$2"
  local readonly output_file="$3"

  kubectl get PodMetrics --kubeconfig "$KUBECONFIG" -n "${namespace}" -l "${label}" -o yaml > "${output_file}"
}

collect_clusterloader2_metrics() {
  clone_perf_tests_repository

  log "Deploying StorageClass"
  kubectl apply -f "$STORAGE_CLASS" --kubeconfig "$KUBECONFIG"

  log "Running clusterloader2 tests"
  kubetest2 noop \
    --test=clusterloader2 \
    --kubeconfig="$KUBECONFIG" \
    -- \
    --repo-root="$PERF_TESTS_DIR" \
    --test-configs="$CLUSTER_LOADER_CONFIG" \
    --test-overrides="$CLUSTER_LOADER_OVERRIDE" \
    --report-dir="$METRICS_DIR_PATH" \

  local readonly exit_code=$?
  if [[ ${exit_code} -ne 0 ]]; then
    log "Clusterloader2 tests failed with exit code ${exit_code}"
    exit 1
  fi

  log "Clusterloader2 tests completed successfully"
}

clone_perf_tests_repository() {
  if [ ! -d "$PERF_TESTS_DIR" ]; then
    log "Cloning perf-tests repository"
    if ! git clone "$PERF_TESTS_URL" "$PERF_TESTS_DIR"; then
      log "Failed to clone perf-tests repository. Aborting."
      exit 1
    fi
  else
    log "perf-tests repository already exists, skipping cloning"
  fi
}

upload_metrics() {
  log "Checking if S3 bucket $SOURCE_BUCKET exists"
  if ! aws s3api head-bucket --bucket "$SOURCE_BUCKET" >/dev/null 2>&1; then
    log "S3 bucket $SOURCE_BUCKET does not exist. Creating the bucket..."
    aws s3api create-bucket \
      --bucket "$SOURCE_BUCKET" \
      --region "$AWS_REGION" \
      --create-bucket-configuration LocationConstraint="$AWS_REGION"
  fi

  log "Configuring bucket policy to allow Lambda access"
  local readonly bucket_policy='{
    "Statement": [
      {
        "Sid": "AllowLambdaAccess",
        "Effect": "Allow",
        "Principal": {
          "AWS": "'${LAMBDA_ROLE}'"
        },
        "Action": "s3:GetObject",
        "Resource": "arn:aws:s3:::'${SOURCE_BUCKET}'/*"
      }
    ]
  }'

  aws s3api put-bucket-policy \
  --bucket "$SOURCE_BUCKET" \
  --policy "$bucket_policy" \
  --region "$AWS_REGION"

  log "Setting lifecycle policy on S3 bucket $SOURCE_BUCKET"
  local readonly lifecycle_policy='{
    "Rules": [
      {
        "ID": "ExpireMetricsData",
        "Status": "Enabled",
        "Prefix": "",
        "Expiration": {
          "Days": 3
        }
      }
    ]
  }'

  aws s3api put-bucket-lifecycle-configuration \
    --bucket "$SOURCE_BUCKET" \
    --lifecycle-configuration "$lifecycle_policy" \
    --region "$AWS_REGION"

  log "Configuring bucket notification to trigger Lambda function"
  local readonly notification_config='{
    "LambdaFunctionConfigurations": [
      {
        "Id": "MetricsEventNotification",
        "LambdaFunctionArn": "'${LAMBDA_ARN}'",
        "Events": ["s3:ObjectCreated:*"]
      }
    ]
  }'

  aws s3api put-bucket-notification-configuration \
    --bucket "$SOURCE_BUCKET" \
    --notification-configuration "$notification_config" \
    --region "$AWS_REGION"

  log "Uploading metrics to S3 bucket $SOURCE_BUCKET"
  aws s3 sync "$METRICS_DIR_PATH" "s3://$SOURCE_BUCKET/$METRICS_DIR_NAME" --region "$AWS_REGION"
}
