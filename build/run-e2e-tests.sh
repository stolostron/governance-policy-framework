#!/bin/bash
# Copyright Contributors to the Open Cluster Management project


set -e

printf "* Running E2E "
if [ "$deployOnHub" == "true" ]; then
    echo "with deployOnHub=true..."
else
    echo "with managed cluster..."
fi

if ! which oc > /dev/null; then
    echo "Installing oc and kubectl clis..."
    mkdir clis-unpacked
    curl -kLo oc.tar.gz https://mirror.openshift.com/pub/openshift-v4/clients/ocp/latest/openshift-client-linux.tar.gz
    tar -xzf oc.tar.gz -C clis-unpacked
    chmod +x ./clis-unpacked/oc
    chmod +x ./clis-unpacked/kubectl
    sudo mv ./clis-unpacked/kubectl /usr/local/bin/
    sudo mv ./clis-unpacked/oc /usr/local/bin/
fi
if ! which kind > /dev/null; then
    KIND_VERSION=${KIND_VERSION:-"$(curl -s https://api.github.com/repos/kubernetes-sigs/kind/releases/latest | jq -r '.tag_name')"}
    echo "* Installing kind ${KIND_VERSION}..."
    curl -Lo ./kind https://github.com/kubernetes-sigs/kind/releases/download/${KIND_VERSION}/kind-$(uname)-amd64
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

if [[ -n "${ERROR_CODE}" ]]; then
    echo "* Detected test failure. Collecting debug logs..."
    make e2e-debug-kind
fi

echo "* Deleting Kind cluster..."
make kind-delete-cluster 

# Since we may have captured an exit code previously, manually exit with it here
exit ${ERROR_CODE}
