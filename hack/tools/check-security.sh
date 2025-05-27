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

set -euo pipefail

# Script that runs a few security tools to ensure we have no vulnerabilities.

BASE_DIR="$(dirname "$(realpath "${BASH_SOURCE[0]}")")"
ROOT_DIR="$BASE_DIR/../.."
BIN="${ROOT_DIR}/bin"
echo "Will run $BIN/govulncheck -C $ROOT_DIR ./cmd/... ./pkg/..."
# Set GOMAXPROCS=1 GOMEMLIMIT=3000000000 to prevent CI running forever due to memory limitations
GOMAXPROCS=1 GOMEMLIMIT=3000000000 "$BIN/govulncheck" -C "$ROOT_DIR" "./cmd/..." "./pkg/..."
