#!/bin/bash

# Copyright 2024 The Kubernetes Authors.
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

# This script is used as a stub for helm to substitute --reset-then-reuse-values
# for instances of --reuse-values until https://github.com/helm/chart-testing/pull/531
# or a similar PR is merged and released

exec "$(dirname "${0}")/.helm" "${@//--reuse-values/--reset-then-reuse-values}"
