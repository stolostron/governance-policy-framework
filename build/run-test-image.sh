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

# Run test suite with reporting
CGO_ENABLED=0 ginkgo -v ${GINKGO_FAIL_FAST} ${GINKGO_LABEL_FILTER} --junit-report=integration.xml --output-dir=test-output test/integration -- -cluster_namespace=$MANAGED_CLUSTER_NAME || EXIT_CODE=$?

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
