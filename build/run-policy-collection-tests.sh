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

ginkgo -v --slowSpecThreshold=10 test/policy-collection -- -kubeconfig_hub=$HUB_KUBECONFIG -kubeconfig_managed=$MANAGED_KUBECONFIG -cluster_namespace=$MANAGED_CLUSTER_NAME