# Copyright 2024 The Kubernetes Authors.
#
# Licensed under the Apache License, Version 2.0 (the 'License');
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#    http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an 'AS IS' BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

#!/bin/bash

set -uo pipefail

function loudecho() {
  echo "###"
  echo "## ${1}"
  echo "#"
}

function generate_ssh_key() {
  SSH_KEY_PATH=${1}
  if [[ ! -e ${SSH_KEY_PATH} ]]; then
    loudecho "Generating SSH key $SSH_KEY_PATH"
    ssh-keygen -P csi-e2e -f "${SSH_KEY_PATH}"
  else
    loudecho "Reusing SSH key $SSH_KEY_PATH"
  fi
}
