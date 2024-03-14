#!/bin/bash

# Copyright 2023 The Kubernetes Authors.
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

# Generates instance store table for `pkg/cloud/volume_limits.go` from the AWS API
# Ensure you are opted into all opt-in regions before running
# Ensure your account isn't in any private instance type betas before running

set -euo pipefail

BIN="$(dirname "$(realpath "${BASH_SOURCE[0]}")")/../bin"

function get_instance_stores_for_region() {
  REGION="${1}"
  echo "Getting limits for ${REGION}..." >&2
  "${BIN}/aws" ec2 describe-instance-types --region "${REGION}" --filters "Name=instance-storage-supported,Values=true" --query "InstanceTypes[].[InstanceType, InstanceStorageInfo]" |
    jq -r 'map("\"" + .[0] + "\": " + (.[1].Disks | map(.Count) | add | tostring) + ",") | .[]'
}

function get_all_instance_stores() {
  "${BIN}/aws" account list-regions --max-results 50 | jq -r '.Regions | map(.RegionName) | .[]' | while read REGION; do
    get_instance_stores_for_region $REGION
  done
}

get_all_instance_stores | sort | uniq
