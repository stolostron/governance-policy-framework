#!/bin/bash
# Copyright Contributors to the Open Cluster Management project


set -e

echo "Login hub"
export OC_CLUSTER_URL=$OC_HUB_CLUSTER_URL
export OC_CLUSTER_PASS=$OC_HUB_CLUSTER_PASS
make oc/login

./build/patch-dev-images.sh