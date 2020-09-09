#!/usr/bin/env bash

# Copyright 2019 The Kubernetes Authors.
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


echo "Repo uses 'go mod'."
if ! (set -x; env GO111MODULE=on go mod tidy); then
    echo "ERROR: vendor check failed."
    exit 1
elif [ "$(git status --porcelain -- go.mod go.sum | wc -l)" -gt 0 ]; then
    echo "ERROR: go module files *not* up-to-date, they did get modified by 'GO111MODULE=on go mod tidy':";
    git diff -- go.mod go.sum
    exit 1
elif [ -d vendor ]; then
    if ! (set -x; env GO111MODULE=on go mod vendor); then
	echo "ERROR: vendor check failed."
	exit 1
    elif [ "$(git status --porcelain -- vendor | wc -l)" -gt 0 ]; then
	echo "ERROR: vendor directory *not* up-to-date, it did get modified by 'GO111MODULE=on go mod vendor':"
	git status -- vendor
	git diff -- vendor
	exit 1
    else
	echo "Go dependencies and vendor directory up-to-date."
    fi
else
    echo "Go dependencies up-to-date."
fi
