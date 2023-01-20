#!/bin/bash
# Copyright Contributors to the Open Cluster Management project


set -e

echo "Login hub"
# These are provided in the Travis environment
oc login ${OC_HUB_CLUSTER_URL} --insecure-skip-tls-verify=true -u ${OC_CLUSTER_USER} -p ${OC_HUB_CLUSTER_PASS}

./build/patch-dev-images.sh
