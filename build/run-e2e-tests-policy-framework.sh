#!/bin/bash
# Copyright Contributors to the Open Cluster Management project


set -e
TEST_IMAGE_URI=$1

sudo ./build/download-clis.sh

echo "Login hub"
export OC_CLUSTER_URL=${OC_HUB_CLUSTER_URL:-${OC_CLUSTER_URL}}
export OC_CLUSTER_USER=${OC_CLUSTER_USER:-kubeadmin}
export OC_CLUSTER_PASS=${OC_HUB_CLUSTER_PASS:-${OC_CLUSTER_PASS}}
oc login ${OC_CLUSTER_URL} --insecure-skip-tls-verify=true -u ${OC_CLUSTER_USER} -p ${OC_CLUSTER_PASS}

export HUB_KUBECONFIG=${HUB_KUBECONFIG:-`echo ~/.kube/config`}
export MANAGED_KUBECONFIG=${MANAGED_KUBECONFIG:-`echo ~/.kube/config`}
export MANAGED_CLUSTER_NAME=${MANAGED_CLUSTER_NAME:-"local-cluster"}

docker run --volume $(pwd)/results:/go/src/github.com/stolostron/governance-policy-framework/test-output \
    --volume $HUB_KUBECONFIG:/go/src/github.com/stolostron/governance-policy-framework/kubeconfig_hub \
    --volume $MANAGED_KUBECONFIG:/go/src/github.com/stolostron/governance-policy-framework/kubeconfig_managed \
    --env MANAGED_CLUSTER_NAME=$MANAGED_CLUSTER_NAME \
    --env FAIL_FAST=true \ 
    $TEST_IMAGE_URI
