#!/bin/bash

set -e

DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" >/dev/null 2>&1 && pwd )"

grcui=`oc get deploy -l component=ocm-grcui -n open-cluster-management -o=jsonpath='{.items[*].metadata.name}'`
grcuiapi=`oc get deploy -l component=ocm-grcuiapi -n open-cluster-management -o=jsonpath='{.items[*].metadata.name}'`
consoleheader=`oc get deploy -l component=console-header -n open-cluster-management -o=jsonpath='{.items[*].metadata.name}'`
policypropagator=`oc get deploy -l component=ocm-policy-propagator -n open-cluster-management -o=jsonpath='{.items[*].metadata.name}'`
oc patch deployment $grcui -n open-cluster-management -p "{\"spec\":{\"template\":{\"spec\":{\"containers\":[{\"name\":\"grc-ui\",\"image\":\"quay.io/stolostron/grc-ui:latest-2.2\"}]}}}}"
oc patch deployment $grcuiapi -n open-cluster-management -p "{\"spec\":{\"template\":{\"spec\":{\"containers\":[{\"name\":\"grc-ui-api\",\"image\":\"quay.io/stolostron/grc-ui-api:latest-2.2\"}]}}}}"
oc patch deployment $policypropagator -n open-cluster-management -p "{\"spec\":{\"template\":{\"spec\":{\"containers\":[{\"name\":\"governance-policy-propagator\",\"image\":\"quay.io/stolostron/governance-policy-propagator:latest-2.2\"}]}}}}"
oc patch deployment $consoleheader -n open-cluster-management -p "{\"spec\":{\"template\":{\"spec\":{\"containers\":[{\"name\":\"console-header\",\"image\":\"quay.io/open-cluster-management/console-header:latest-dev\"}]}}}}"

managedclusters=`oc get managedcluster -o=jsonpath='{.items[*].metadata.name}'`
for managedcluster in $managedclusters
do
    oc annotate klusterletaddonconfig -n $managedcluster $managedcluster klusterletaddonconfig-pause=true --overwrite=true
    oc patch manifestwork -n $managedcluster $managedcluster-klusterlet-addon-iampolicyctrl --type='json' -p=`cat $DIR/patches/iampolicycontroller.json` || true
    oc patch manifestwork -n $managedcluster $managedcluster-klusterlet-addon-certpolicyctrl --type='json' -p=`cat $DIR/patches/certpolicycontroller.json` || true
    oc patch manifestwork -n $managedcluster $managedcluster-klusterlet-addon-policyctrl --type='json' -p=`cat $DIR/patches/policycontroller.json` || true
done
