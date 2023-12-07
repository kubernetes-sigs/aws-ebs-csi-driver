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

# This script verifies that `make update` does not need to run
# It does so by creating a temporary copy of the repo and running `make update`
# in the copy, and then checking if it produces a diff to the local copy

set -euo pipefail

ROOT="$(dirname "${0}")/../"
TEST_DIR=$(mktemp -d)
trap "rm -rf \"${TEST_DIR}\"" EXIT
cp -rf "${ROOT}/." "${TEST_DIR}"

if ! make -C "${TEST_DIR}" update > /dev/null; then
    echo "\`make update\` failed!"
    exit 1
fi

if ! diff -r "${TEST_DIR}" "${ROOT}"; then
    echo "Auto-generation/formatting needs to run!"
    echo "Run \`make update\` to fix!"
    exit 1
fi
