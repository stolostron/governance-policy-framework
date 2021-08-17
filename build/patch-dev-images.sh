#!/bin/bash

set -e

DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" >/dev/null 2>&1 && pwd )"

RHACM_VERSION=2.2
HAS_ADDITIONAL="true"
i=0
while [[ "${HAS_ADDITIONAL}" == "true" ]] && [[ -z "${RHACM_SNAPSHOT}" ]]; do
    ((i++))
    HAS_ADDITIONAL=$(curl -s "https://quay.io/api/v1/repository/open-cluster-management/acm-custom-registry/tag/?onlyActiveTags=true&page=${i}" | jq -r '.has_additional')
    RHACM_SNAPSHOT=$(curl -s "https://quay.io/api/v1/repository/open-cluster-management/acm-custom-registry/tag/?onlyActiveTags=true&page=${i}" | jq -r '.tags[].name' | grep -v "nonesuch\|-$" | grep -F "${RHACM_VERSION}" | head -n 1)
done

grcui=`oc get deploy -l component=ocm-grcui -n open-cluster-management -o=jsonpath='{.items[*].metadata.name}'`
grcuiapi=`oc get deploy -l component=ocm-grcuiapi -n open-cluster-management -o=jsonpath='{.items[*].metadata.name}'`
consoleheader=`oc get deploy -l component=console-header -n open-cluster-management -o=jsonpath='{.items[*].metadata.name}'`
policypropagator=`oc get deploy -l component=ocm-policy-propagator -n open-cluster-management -o=jsonpath='{.items[*].metadata.name}'`
oc patch deployment $grcui -n open-cluster-management -p "{\"spec\":{\"template\":{\"spec\":{\"containers\":[{\"name\":\"grc-ui\",\"image\":\"quay.io/open-cluster-management/grc-ui:${RHACM_SNAPSHOT}\"}]}}}}"
oc patch deployment $grcuiapi -n open-cluster-management -p "{\"spec\":{\"template\":{\"spec\":{\"containers\":[{\"name\":\"grc-ui-api\",\"image\":\"quay.io/open-cluster-management/grc-ui-api:${RHACM_SNAPSHOT}\"}]}}}}"
oc patch deployment $policypropagator -n open-cluster-management -p "{\"spec\":{\"template\":{\"spec\":{\"containers\":[{\"name\":\"governance-policy-propagator\",\"image\":\"quay.io/open-cluster-management/governance-policy-propagator:${RHACM_SNAPSHOT}\"}]}}}}"
oc patch deployment $consoleheader -n open-cluster-management -p "{\"spec\":{\"template\":{\"spec\":{\"containers\":[{\"name\":\"console-header\",\"image\":\"quay.io/open-cluster-management/console-header:${RHACM_SNAPSHOT}\"}]}}}}"

managedclusters=`oc get managedcluster -o=jsonpath='{.items[*].metadata.name}'`
for managedcluster in $managedclusters
do
    oc annotate klusterletaddonconfig -n $managedcluster $managedcluster klusterletaddonconfig-pause=true --overwrite=true
    oc patch manifestwork -n $managedcluster $managedcluster-klusterlet-addon-iampolicyctrl --type='json' -p=`cat $DIR/patches/iampolicycontroller.json | sed 's/latest-dev/'${RHACM_SNAPSHOT}'/g'` || true
    oc patch manifestwork -n $managedcluster $managedcluster-klusterlet-addon-certpolicyctrl --type='json' -p=`cat $DIR/patches/certpolicycontroller.json | sed 's/latest-dev/'${RHACM_SNAPSHOT}'/g'` || true
    oc patch manifestwork -n $managedcluster $managedcluster-klusterlet-addon-policyctrl --type='json' -p=`cat $DIR/patches/policycontroller.json | sed 's/latest-dev/'${RHACM_SNAPSHOT}'/g'` || true
done
