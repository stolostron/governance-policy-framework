#!/bin/bash

set -e

export DOCKER_IMAGE_AND_TAG=${1}

if ! which kubectl > /dev/null; then
    echo "installing kubectl"
    curl -LO https://storage.googleapis.com/kubernetes-release/release/$(curl -s https://storage.googleapis.com/kubernetes-release/release/stable.txt)/bin/linux/amd64/kubectl && chmod +x kubectl && sudo mv kubectl /usr/local/bin/
fi
if ! which kind > /dev/null; then
    echo "installing kind"
    curl -Lo ./kind https://github.com/kubernetes-sigs/kind/releases/download/v0.8.1/kind-$(uname)-amd64
    chmod +x ./kind
    sudo mv ./kind /usr/local/bin/kind
fi
echo "Installing ginkgo ..."
go get github.com/onsi/ginkgo/ginkgo
go get github.com/onsi/gomega/...

make kind-create-cluster

./build/wait_for.sh pod -n kube-system
./build/wait_for.sh pod -l k8s-app=kube-dns -n kube-system
./build/wait_for.sh pod -n local-path-storage

make install-crds 

make install-resources

make kind-deploy-policy-framework

if [ "$deployOnHub" != "true" ]; then\
    ./build/wait_for.sh pod -l name=governance-policy-spec-sync -n multicluster-endpoint
fi
./build/wait_for.sh pod -l name=governance-policy-status-sync -n multicluster-endpoint
./build/wait_for.sh pod -l name=governance-policy-template-sync -n multicluster-endpoint

make kind-deploy-policy-controllers

# wait for controller to start
./build/wait_for.sh pod -l name=config-policy-ctrl -n multicluster-endpoint
./build/wait_for.sh pod -l name=cert-policy-controller -n multicluster-endpoint
./build/wait_for.sh pod -l name=iam-policy-controller -n multicluster-endpoint
./build/wait_for.sh pod -n olm

kubectl get pods -A

echo "all ready! statt to test"

make e2e-test

echo "delete cluster"
make kind-delete-cluster 

