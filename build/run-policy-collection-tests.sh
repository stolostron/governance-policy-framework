#!/bin/bash

set -e

if ! which kubectl > /dev/null; then
    echo "installing kubectl"
    curl -LO https://storage.googleapis.com/kubernetes-release/release/$(curl -s https://storage.googleapis.com/kubernetes-release/release/stable.txt)/bin/linux/amd64/kubectl && chmod +x kubectl && sudo mv kubectl /usr/local/bin/
fi
echo "Installing ginkgo ..."
go get github.com/onsi/ginkgo/ginkgo
go get github.com/onsi/gomega/...

echo "Login hub"
export OC_CLUSTER_URL=${OC_HUB_CLUSTER_URL:-${OC_CLUSTER_URL}}
export OC_CLUSTER_URL=${OC_CLUSTER_URL:-kubeadmin}
export OC_CLUSTER_PASS=${OC_HUB_CLUSTER_PASS:-${OC_CLUSTER_PASS}}
make oc/login
export HUB_KUBECONFIG=`echo ~/.kube/config`
export MANAGED_KUBECONFIG=`echo ~/.kube/config`
export MANAGED_CLUSTER_NAME=${MANAGED_CLUSTER_NAME:-"local-cluster"}
ginkgo -v --slowSpecThreshold=10 test/policy-collection -- -kubeconfig_hub=$HUB_KUBECONFIG -kubeconfig_managed=$MANAGED_KUBECONFIG -cluster_namespace=$MANAGED_CLUSTER_NAME