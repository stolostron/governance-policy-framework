#!/bin/bash

set -e

if ! which kubectl > /dev/null; then
    echo "Installing oc and kubectl clis..."
    mkdir clis-unpacked
    curl -kLo oc.tar.gz https://mirror.openshift.com/pub/openshift-v4/clients/ocp/4.6.6/openshift-client-linux-4.6.6.tar.gz
    tar -xzf oc.tar.gz -C clis-unpacked
    chmod 755 ./clis-unpacked/oc
    chmod 755 ./clis-unpacked/kubectl
    mv ./clis-unpacked/oc /usr/local/bin/oc
    mv ./clis-unpacked/kubectl /usr/local/bin/kubectl
fi
echo "Installing ginkgo ..."
go get github.com/onsi/ginkgo/ginkgo
go get github.com/onsi/gomega/...

echo "Login hub"
export OC_CLUSTER_URL=${OC_HUB_CLUSTER_URL:-${OC_CLUSTER_URL}}
export OC_CLUSTER_USER=${OC_CLUSTER_USER:-kubeadmin}
export OC_CLUSTER_PASS=${OC_HUB_CLUSTER_PASS:-${OC_CLUSTER_PASS}}
oc login ${OC_CLUSTER_URL} --insecure-skip-tls-verify=true -u ${OC_CLUSTER_USER} -p ${OC_CLUSTER_PASS}

export HUB_KUBECONFIG=${HUB_KUBECONFIG:-`echo ~/.kube/config`}
export MANAGED_KUBECONFIG=${MANAGED_KUBECONFIG:-`echo ~/.kube/config`}
export MANAGED_CLUSTER_NAME=${MANAGED_CLUSTER_NAME:-"local-cluster"}

printenv
ginkgo -v --slowSpecThreshold=10 test/policy-collection -- -kubeconfig_hub=$HUB_KUBECONFIG -kubeconfig_managed=$MANAGED_KUBECONFIG -cluster_namespace=$MANAGED_CLUSTER_NAME