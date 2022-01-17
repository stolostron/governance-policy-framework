#!/bin/bash

set -e

export DOCKER_IMAGE_AND_TAG=${1}

RHACM_VERSION="2.2"
HAS_ADDITIONAL="true"
i=0
while [[ "${HAS_ADDITIONAL}" == "true" ]] && [[ -z "${RHACM_SNAPSHOT}" ]]; do
    (( i += 1 ))
    HAS_ADDITIONAL=$(curl -s "https://quay.io/api/v1/repository/stolostron/acm-custom-registry/tag/?onlyActiveTags=true&page=${i}" | jq -r '.has_additional')
    export RHACM_SNAPSHOT=$(curl -s "https://quay.io/api/v1/repository/stolostron/acm-custom-registry/tag/?onlyActiveTags=true&page=${i}" | jq -r '.tags[].name' | grep -v "nonesuch\|-$" | grep -F "${RHACM_VERSION}" | head -n 1)
done

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
go get github.com/onsi/ginkgo/ginkgo@v1.14.2
go get github.com/onsi/gomega/...@v1.10.4

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
./build/wait_for.sh pod -n cert-manager

kubectl get pods -A

echo "all ready! statt to test"

make e2e-test

echo "delete cluster"
make kind-delete-cluster 

