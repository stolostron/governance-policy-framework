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

while [[ $(kubectl get pods -A | grep -v -e "Completed" | tail -n +2 | wc -l | tr -d '[:space:]') -ne 9 ]]; do 
    echo "waiting for kind cluster pods running"
    kubectl get pods -A
    sleep 1
done

make install-crds 

make install-resources

make kind-deploy-controller

make kind-deploy-policy-controllers

# wait for controller to start
while [[ $(kubectl get pods -l name=config-policy-ctrl -n multicluster-endpoint -o 'jsonpath={..status.conditions[?(@.type=="Ready")].status}') != "True" ]]; do 
    echo "waiting for pod: config-policy-ctrl"
    kubectl get pods -A
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

if [ "$deployOnHub" != "true" ]; then\
    while [[ $(kubectl get pods -l name=governance-policy-spec-sync -n multicluster-endpoint -o 'jsonpath={..status.conditions[?(@.type=="Ready")].status}') != "True" ]]; do 
        echo "waiting for pod: governance-policy-spec-sync"
        kubectl get pods -l name=governance-policy-spec-sync -n multicluster-endpoint
        sleep 1
    done
fi

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

while [[ $(kubectl get pods -l control-plane=controller-manager -n gatekeeper-system -o 'jsonpath={..status.conditions[?(@.type=="Ready")].status}') != "True" ]]; do 
    echo "waiting for pod: gatekeeper-controller-manager"
    kubectl get pods -l control-plane=controller-manager -n gatekeeper-system
    sleep 1
done

kubectl get pods -n multicluster-endpoint

make e2e-test

echo "delete cluster"
make kind-delete-cluster 

