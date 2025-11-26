#!/bin/bash
# Copyright Contributors to the Open Cluster Management project

set -e

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" >/dev/null 2>&1 && pwd)"

acm_installed_namespace=$(oc get subscriptions.operators.coreos.com --all-namespaces | grep advanced-cluster-management | awk '{print $1}')
acm_version=$(cat "${script_dir}/../CURRENT_VERSION")
image_tag=${image_tag:-"latest"}
image_repo="quay.io/redhat-user-workloads/crt-redhat-acm-tenant"
image_suffix="-acm-${acm_version//./}"

echo "* Patching hub cluster to ${image_tag}"
oc annotate MultiClusterHub multiclusterhub -n "${acm_installed_namespace}" installer.open-cluster-management.io/pause=true --overwrite

# Patch the propagator on the hub
COMPONENT="governance-policy-propagator"
LABEL="component=ocm-policy-propagator"
DEPLOYMENT=$(oc get deployment -l ${LABEL} -n "${acm_installed_namespace}" -o=jsonpath='{.items[*].metadata.name}')
oc patch deployment "${DEPLOYMENT}" -n "${acm_installed_namespace}" -p '{
  "spec": {
    "template": {
      "spec": {
        "containers": [{
          "name":"'"${COMPONENT}"'",
          "imagePullPolicy":"Always",
          "image":"'"${image_repo}/${COMPONENT}${image_suffix}:${image_tag}"'"
  }]}}}}'

# Patch the addon-controller on the hub
COMPONENT="governance-policy-addon-controller"
LABEL="component=ocm-policy-addon-ctrl"
DEPLOYMENT=$(oc get deployment -l ${LABEL} -n "${acm_installed_namespace}" -o=jsonpath='{.items[*].metadata.name}')
oc patch deployment "${DEPLOYMENT}" -n "${acm_installed_namespace}" -p '{
"spec": {
  "template": {
    "spec": {
      "containers":[{
        "name":"manager",
        "imagePullPolicy":"Always",
        "image":"'"${image_repo}/${COMPONENT}${image_suffix}:${image_tag}"'"
      }]}}}}'

# Patch the addon-controller envs
CONTAINERS=(cert-policy-controller config-policy-controller governance-policy-framework-addon)
for CONTAINER in "${CONTAINERS[@]}"; do
  IMAGE_NAME=$(echo "${CONTAINER}" | tr '[:lower:]' '[:upper:]' | tr '-' '_')_IMAGE
  oc set env "deployment/${DEPLOYMENT}" -n "${acm_installed_namespace}" "${IMAGE_NAME}"="${image_repo}/${CONTAINER}${image_suffix}:${image_tag}"
done

# Patch managed cluster components
echo "* Patching managed clusters to ${image_tag}"
MANAGED_CLUSTERS=$(oc get managedcluster -o=jsonpath='{.items[*].metadata.name}')

ADDON_COMPONENTS=(cert-policy-controller config-policy-controller governance-policy-framework)
for MANAGED_CLUSTER in ${MANAGED_CLUSTERS}; do
  oc annotate klusterletaddonconfig -n "${MANAGED_CLUSTER}" "${MANAGED_CLUSTER}" klusterletaddonconfig-pause=true --overwrite=true
  FOUND="false"
  while [[ "${FOUND}" == "false" ]]; do
    echo "* Wait for manifestwork on ${MANAGED_CLUSTER}:"
    FOUND="true"
    for COMPONENT in "${ADDON_COMPONENTS[@]}"; do
      if (! oc get manifestwork -n "${MANAGED_CLUSTER}" "addon-${COMPONENT}-deploy-0"); then
        FOUND="false"
      fi
    done
    sleep 5
  done
  # Patch imagePullPolicy
  for COMPONENT in "${ADDON_COMPONENTS[@]}"; do
    echo "* Patch imagePullPolicy for ${COMPONENT}"
    oc annotate ManagedClusterAddOn "${COMPONENT}" -n "${MANAGED_CLUSTER}" --overwrite 'addon.open-cluster-management.io/values={"global":{"imagePullPolicy":"Always"}}'
  done
done

echo "* Deleting pods and waiting for restart"
oc delete pod -l app=grc -A
oc delete pod -l app=governance-policy-framework -A
oc delete pod -l app=config-policy-controller -A
oc delete pod -l app=cert-policy-controller -A

./build/wait_for.sh pod -l app=grc -A
./build/wait_for.sh pod -l app=governance-policy-framework -A
./build/wait_for.sh pod -l app=config-policy-controller -A
./build/wait_for.sh pod -l app=cert-policy-controller -A
