#!/bin/bash
# Copyright Contributors to the Open Cluster Management project


set -e

if ! which kubectl > /dev/null; then
    echo "* Installing kubectl..."
    curl -LO https://storage.googleapis.com/kubernetes-release/release/$(curl -s https://storage.googleapis.com/kubernetes-release/release/stable.txt)/bin/linux/amd64/kubectl && chmod +x kubectl && sudo mv kubectl /usr/local/bin/
fi
if ! which kind > /dev/null; then
    echo "* Installing kind..."
    curl -Lo ./kind https://github.com/kubernetes-sigs/kind/releases/download/v0.8.1/kind-$(uname)-amd64
    chmod +x ./kind
    sudo mv ./kind /usr/local/bin/kind
fi
echo "* Installing ginkgo ..."
make e2e-dependencies

echo "* Creating Kind cluster(s)..."
export MANAGED_CLUSTER_NAME=managed
make kind-create-cluster

./build/wait_for.sh pod -n kube-system
./build/wait_for.sh pod -l k8s-app=kube-dns -n kube-system
./build/wait_for.sh pod -n local-path-storage

echo "* Deploying policy framework..."
make install-crds

make install-resources

make kind-deploy-policy-framework

if [ "$deployOnHub" != "true" ]; then\
    ./build/wait_for.sh pod -l name=governance-policy-spec-sync -n open-cluster-management-agent-addon
fi
./build/wait_for.sh pod -l name=governance-policy-status-sync -n open-cluster-management-agent-addon
./build/wait_for.sh pod -l name=governance-policy-template-sync -n open-cluster-management-agent-addon

make kind-deploy-policy-controllers

# wait for controller to start
./build/wait_for.sh pod -l name=config-policy-controller -n open-cluster-management-agent-addon
./build/wait_for.sh pod -l name=cert-policy-controller -n open-cluster-management-agent-addon
./build/wait_for.sh pod -l name=iam-policy-controller -n open-cluster-management-agent-addon
./build/wait_for.sh pod -n olm
./build/wait_for.sh pod -n cert-manager

echo "* Listing all pods on cluster..."
kubectl get pods -A

echo "* All ready! Start to test..."

make e2e-test || ERROR_CODE=$?

if [[ -n "${ERROR_CODE}" ]] && [[ -n "${ARTIFACT_DIR}" ]]; then
    echo "* Detected test failure. Collecting debug logs..."
    make e2e-debug
fi

echo "* Deleting Kind cluster..."
make kind-delete-cluster 

# Since we may have captured an exit code previously, manually exit with it here
exit ${ERROR_CODE}
