#!/bin/bash
# Copyright Contributors to the Open Cluster Management project


set -e

echo "Login hub"
export OC_CLUSTER_URL=$OC_HUB_CLUSTER_URL
export OC_CLUSTER_PASS=$OC_HUB_CLUSTER_PASS
make oc/login

./build/patch-dev-images.sh

oc delete pod -l app=console-header -A
oc delete pod -l app=grc -A
oc delete pod -l component=governance -A
oc delete pod -l app=klusterlet-addon-iampolicyctrl -A
oc delete pod -l app=cert-policy-controller -A

./build/wait_for.sh pod -l app=console-header -A
./build/wait_for.sh pod -l app=grc -A
./build/wait_for.sh pod -l component=governance -A
./build/wait_for.sh pod -l app=klusterlet-addon-iampolicyctrl -A
./build/wait_for.sh pod -l app=cert-policy-controller -A

