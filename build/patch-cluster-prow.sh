#! /bin/bash

set -e

DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" >/dev/null 2>&1 && pwd )"

echo "* Check for a running ACM"
acm_installed_namespace=`oc get subscriptions.operators.coreos.com --all-namespaces | grep advanced-cluster-management | awk '{print $1}'`
while UNFINISHED="$(oc -n ${acm_installed_namespace} get pods | grep -v -e "Completed" -e "1/1     Running" -e "2/2     Running" -e "3/3     Running" -e "4/4     Running" -e "READY   STATUS" | wc -l)" && [[ "${UNFINISHED}" != "0" ]]; do
  echo "* Waiting on ${UNFINISHED} pods in namespace ${acm_installed_namespace}..."
  sleep 5
done

./build/patch-dev-images.sh

