#!/bin/bash

set -e

DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" >/dev/null 2>&1 && pwd )"

grcui=`oc get deploy -l component=ocm-grcui -n open-cluster-management -o=jsonpath='{.items[*].metadata.name}'`
grcuiapi=`oc get deploy -l component=ocm-grcuiapi -n open-cluster-management -o=jsonpath='{.items[*].metadata.name}'`
consoleheader=`oc get deploy -l component=console-header -n open-cluster-management -o=jsonpath='{.items[*].metadata.name}'`
policypropagator=`oc get deploy -l component=ocm-policy-propagator -n open-cluster-management -o=jsonpath='{.items[*].metadata.name}'`
oc patch deployment $grcui -n open-cluster-management -p "{\"spec\":{\"template\":{\"spec\":{\"containers\":[{\"name\":\"grc-ui\",\"image\":\"quay.io/open-cluster-management/grc-ui:latest-dev\"}]}}}}"
oc patch deployment $grcuiapi -n open-cluster-management -p "{\"spec\":{\"template\":{\"spec\":{\"containers\":[{\"name\":\"grc-ui-api\",\"image\":\"quay.io/open-cluster-management/grc-ui-api:latest-dev\"}]}}}}"
oc patch deployment $policypropagator -n open-cluster-management -p "{\"spec\":{\"template\":{\"spec\":{\"containers\":[{\"name\":\"governance-policy-propagator\",\"image\":\"quay.io/open-cluster-management/governance-policy-propagator:latest-dev\"}]}}}}"
oc patch deployment $consoleheader -n open-cluster-management -p "{\"spec\":{\"template\":{\"spec\":{\"containers\":[{\"name\":\"console-header\",\"image\":\"quay.io/open-cluster-management/console-header:latest-dev\"}]}}}}"

managedclusters=`oc get managedcluster -o=jsonpath='{.items[*].metadata.name}'`
for managedcluster in $managedclusters
do
    oc annotate klusterletaddonconfig -n $managedcluster $managedcluster klusterletaddonconfig-pause=true --overwrite=true
    oc patch manifestwork -n $managedcluster $managedcluster-klusterlet-addon-iampolicyctrl --type='json' -p=`cat $DIR/patch-iampolicycontroller.json`
    oc patch manifestwork -n $managedcluster $managedcluster-klusterlet-addon-certpolicyctrl --type='json' -p=`cat $DIR/patch-certpolicycontroller.json`
    oc patch manifestwork -n $managedcluster $managedcluster-klusterlet-addon-policyctrl --type='json' -p=`cat $DIR/patch-policycontroller.json`
done
