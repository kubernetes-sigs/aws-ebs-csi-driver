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
