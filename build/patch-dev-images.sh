#!/bin/bash
# Copyright Contributors to the Open Cluster Management project


set -e

acm_installed_namespace=`oc get subscriptions.operators.coreos.com --all-namespaces | grep advanced-cluster-management | awk '{print $1}'`
VERSION_TAG=${VERSION_TAG:-"latest"}
DOCKER_URI="quay.io/stolostron"

echo "* Patching hub cluster to ${VERSION_TAG}"
oc annotate MultiClusterHub multiclusterhub -n ${acm_installed_namespace} mch-pause=true --overwrite

# Patch the propagator on the hub
COMPONENT="governance-policy-propagator"
LABEL="component=ocm-policy-propagator"
DEPLOYMENT=$(oc get deployment -l ${LABEL} -n ${acm_installed_namespace} -o=jsonpath='{.items[*].metadata.name}')
oc patch deployment ${DEPLOYMENT} -n ${acm_installed_namespace} -p "{\"spec\":{\"template\":{\"spec\":{\"containers\":[{\"name\":\"${COMPONENT}\",\"imagePullPolicy\":\"Always\",\"image\":\"${DOCKER_URI}/${COMPONENT}:${VERSION_TAG}\"}]}}}}"

# Patch the addon-controller on the hub
COMPONENT="governance-policy-addon-controller"
LABEL="component=ocm-policy-addon-ctrl"
DEPLOYMENT=$(oc get deployment -l ${LABEL} -n ${acm_installed_namespace} -o=jsonpath='{.items[*].metadata.name}')
oc patch deployment ${DEPLOYMENT} -n ${acm_installed_namespace} -p "{\"spec\":{\"template\":{\"spec\":{\"containers\":[{\"name\":\"manager\",\"imagePullPolicy\":\"Always\",\"image\":\"${DOCKER_URI}/${COMPONENT}:${VERSION_TAG}\"}]}}}}"

# Patch the addon-controller envs
CONTAINERS=(cert-policy-controller config-policy-controller iam-policy-controller governance-policy-framework-addon)
for CONTAINER in ${CONTAINERS[@]}; do
  IMAGE_NAME=$(echo $CONTAINER | tr 'a-z' 'A-Z' | tr '-' '_')_IMAGE
  oc set env deployment/${DEPLOYMENT} -n ${acm_installed_namespace} ${IMAGE_NAME}=${DOCKER_URI}/${CONTAINER}:${VERSION_TAG}
done

# Patch managed cluster components
echo "* Patching managed clusters to ${VERSION_TAG}"
MANAGED_CLUSTERS=$(oc get managedcluster -o=jsonpath='{.items[*].metadata.name}')

ADDON_COMPONENTS=(cert-policy-controller config-policy-controller iam-policy-controller governance-policy-framework)
for MANAGED_CLUSTER in ${MANAGED_CLUSTERS}; do      
    oc annotate klusterletaddonconfig -n ${MANAGED_CLUSTER} ${MANAGED_CLUSTER} klusterletaddonconfig-pause=true --overwrite=true
    FOUND="false"
    while [[ "${FOUND}" == "false" ]]; do
      echo "* Wait for manifestwork on ${MANAGED_CLUSTER}:"
      FOUND="true"
      for COMPONENT in ${ADDON_COMPONENTS[@]}; do
        if (! oc get manifestwork -n ${MANAGED_CLUSTER} addon-${COMPONENT}-deploy-0); then
          FOUND="false"
        fi
      done
      sleep 5
    done
    # Patch imagePullPolicy
    for COMPONENT in ${ADDON_COMPONENTS[@]}; do
      echo "* Patch imagePullPolicy for ${COMPONENT}"
      oc annotate ManagedClusterAddOn ${COMPONENT} -n ${MANAGED_CLUSTER} --overwrite addon.open-cluster-management.io/values={\"global\":{\"imagePullPolicy\":\"Always\"}}  
    done
done



echo "* Deleting pods and waiting for restart"
oc delete pod -l app=grc -A
oc delete pod -l app=governance-policy-framework -A
oc delete pod -l app=config-policy-controller -A
oc delete pod -l app=iam-policy-controller -A
oc delete pod -l app=cert-policy-controller -A

./build/wait_for.sh pod -l app=grc -A
./build/wait_for.sh pod -l app=governance-policy-framework -A
./build/wait_for.sh pod -l app=config-policy-controller -A
./build/wait_for.sh pod -l app=iam-policy-controller -A
./build/wait_for.sh pod -l app=cert-policy-controller -A
