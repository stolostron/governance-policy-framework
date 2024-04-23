#!/bin/bash
# Copyright Contributors to the Open Cluster Management project

set -e

if [[ ${FAIL_FAST} == "true" ]]; then
  echo "* Running in fail fast mode"
  GINKGO_FAIL_FAST="--fail-fast" 
fi

if [[ -z ${GINKGO_LABEL_FILTER} ]]; then 
  echo "* No GINKGO_LABEL_FILTER set"
else
  GINKGO_LABEL_FILTER="--label-filter=${GINKGO_LABEL_FILTER}"
  echo "* Using GINKGO_LABEL_FILTER=${GINKGO_LABEL_FILTER}"
fi

if [[ -z ${POLICY_COLLECTION_BRANCH} ]]; then
  echo "* No POLICY_COLLECTION_BRANCH set, using main for the policy-collection branch"
  POLICY_COLLECTION_BRANCH="main"
else
  echo "* Using POLICY_COLLECTION_BRANCH=${POLICY_COLLECTION_BRANCH}"
fi

if [[ -z ${OCM_NAMESPACE} ]]; then
  echo "* OCM_NAMESPACE not set, using open-cluster-management"
  OCM_NAMESPACE="open-cluster-management"
else
  echo "* Using OCM_NAMESPACE=${OCM_NAMESPACE}"
fi

if [[ -z ${OCM_ADDON_NAMESPACE} ]]; then
  echo "* OCM_ADDON_NAMESPACE not set, using open-cluster-management-agent-addon"
  OCM_ADDON_NAMESPACE="open-cluster-management-agent-addon"
else
  echo "* Using OCM_ADDON_NAMESPACE=${OCM_ADDON_NAMESPACE}"
fi

# Run test suite with reporting
CGO_ENABLED=0 ./bin/ginkgo -v ${GINKGO_FAIL_FAST} ${GINKGO_LABEL_FILTER} --junit-report=integration.xml --output-dir=test-output test/integration -- -cluster_namespace=$MANAGED_CLUSTER_NAME -ocm_namespace=$OCM_NAMESPACE -ocm_addon_namespace=$OCM_ADDON_NAMESPACE -patch_decisions=false -policy_collection_branch=$POLICY_COLLECTION_BRANCH || EXIT_CODE=$?

# Remove Gingko phases from report to prevent corrupting bracketed metadata
if [ -f test-output/integration.xml ]; then
  sed -i 's/\[It\] *//g' test-output/integration.xml
  sed -i 's/\[BeforeSuite\]/GRC: [P1][Sev1][policy-grc] BeforeSuite/g' test-output/integration.xml
  sed -i 's/\[AfterSuite\]/GRC: [P1][Sev1][policy-grc] AfterSuite/g' test-output/integration.xml
fi

# Collect exit code if it's an error
if [[ "${EXIT_CODE}" != "0" ]]; then
  ERROR_CODE=${EXIT_CODE}
fi

if [[ -n "${ERROR_CODE}" ]]; then
    echo "* Detected test failure. Collecting debug logs..."
    # For debugging, the managed cluster might have a different name (i.e. 'local-cluster') but the
    # kubeconfig is still called 'kubeconfig_managed'
    export MANAGED_CLUSTER_NAME="managed"
    make e2e-debug-acm
fi

# Since we may have captured an exit code previously, manually exit with it here
exit ${ERROR_CODE}
