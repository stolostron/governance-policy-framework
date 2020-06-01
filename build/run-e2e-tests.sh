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

make install-crds 

make kind-deploy-controller

make kind-deploy-policy-controllers

make install-resources

# wait for controller to start
while [[ $(kubectl get pods -l name=config-policy-ctrl -n multicluster-endpoint -o 'jsonpath={..status.conditions[?(@.type=="Ready")].status}') != "True" ]]; do 
    echo "waiting for pod: config-policy-ctrl"
    kubectl get pods -l name=config-policy-ctrl -n multicluster-endpoint
    sleep 1
done

while [[ $(kubectl get pods -l name=cert-policy-controller -n multicluster-endpoint -o 'jsonpath={..status.conditions[?(@.type=="Ready")].status}') != "True" ]]; do 
    echo "waiting for pod: cert-policy-controller"
    kubectl get pods -l name=cert-policy-controller -n multicluster-endpoint
    sleep 1
done

while [[ $(kubectl get pods -l name=iam-policy-controller -n multicluster-endpoint -o 'jsonpath={..status.conditions[?(@.type=="Ready")].status}') != "True" ]]; do 
    echo "waiting for pod: iam-policy-controller"
    kubectl get pods -l name=iam-policy-controller -n multicluster-endpoint
    sleep 1
done

while [[ $(kubectl get pods -l name=governance-policy-spec-sync -n multicluster-endpoint -o 'jsonpath={..status.conditions[?(@.type=="Ready")].status}') != "True" ]]; do 
    echo "waiting for pod: governance-policy-spec-sync"
    kubectl get pods -l name=governance-policy-spec-sync -n multicluster-endpoint
    sleep 1
done

while [[ $(kubectl get pods -l name=governance-policy-status-sync -n multicluster-endpoint -o 'jsonpath={..status.conditions[?(@.type=="Ready")].status}') != "True" ]]; do 
    echo "waiting for pod: governance-policy-status-sync"
    kubectl get pods -l name=governance-policy-status-sync -n multicluster-endpoint
    sleep 1
done

while [[ $(kubectl get pods -l name=governance-policy-template-sync -n multicluster-endpoint -o 'jsonpath={..status.conditions[?(@.type=="Ready")].status}') != "True" ]]; do 
    echo "waiting for pod: governance-policy-template-sync"
    kubectl get pods -l name=governance-policy-template-sync -n multicluster-endpoint
    sleep 1
done

kubectl get pods -n multicluster-endpoint

make e2e-test

echo "delete cluster"
make kind-delete-cluster 

