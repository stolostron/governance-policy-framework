#! /bin/bash

set -e

echo "* Check for a running ACM"
acm_installed_namespace=$(oc get subscriptions.operators.coreos.com --all-namespaces | grep advanced-cluster-management | awk '{print $1}')

echo "* Initial Pod state:"
oc -n "${acm_installed_namespace}" get pods

echo "* Waiting for prerequisite pods:"
kubectl wait pod -l 'app in (grc,policyreport,multicluster-operators-standalone-subscription)' \
  -n "${acm_installed_namespace}" --for=condition=Ready --timeout=180s

echo "* Post-wait Pod state:"
oc -n "${acm_installed_namespace}" get pods

./build/patch-dev-images.sh
