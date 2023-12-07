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

# There is no reliable way to check if a buildx installation supports
# --provenance other than trying to execute it. You cannot even rely
# on the version, because buildx's own installation docs will result
# in installations of buildx that do not correctly report their version
# via `docker buildx version`.
#
# Additionally, if the local buildkit worker is the Docker daemon,
# attestation should not be supported and must be disabled.
#
# Thus, this script echos back the flag `--provenance=false` if and only
# if the local buildx installation supports it. If not, it exits silently.

BUILDX_TEST=`docker buildx build --provenance=false 2>&1`
if [[ "${BUILDX_TEST}" == *"See 'docker buildx build --help'."* ]]; then
  if [[ "${BUILDX_TEST}" == *"requires exactly 1 argument"* ]] && ! docker buildx inspect | grep -qE "^Driver:\s*docker$"; then
    echo "--provenance=false"
  fi
else
  echo "Local buildx installation broken?" >&2
  exit 1
fi
