#!/bin/bash

set -uo pipefail

OS_ARCH=$(go env GOOS)-amd64

helm::install() {
    declare -r helm_name=helm-v2.16.0-$OS_ARCH.tar.gz
    wget https://get.helm.sh/$helm_name
    tar xvzf $helm_name
    mv $OS_ARCH/helm /usr/local/bin/helm
}

helm::init() {
    declare -r rbac_file_path=$(dirname "${BASH_SOURCE}")/tiller-rbac.yaml
    kubectl apply -f $rbac_file_path
    helm init --service-account tiller --history-max 200 --wait
    kubectl get po -n kube-system
}
