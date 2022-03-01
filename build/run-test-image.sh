#!/bin/bash
# Copyright Contributors to the Open Cluster Management project

set -e

if [[ ${FAIL_FAST} == "true" ]]; then
  echo "Running in fail fast mode"
  GINKGO_FAIL_FAST="--fail-fast" 
fi

for TEST_SUITE in integration policy-collection; do
  # Run test suite with reporting
  CGO_ENABLED=0 ginkgo -v ${GINKGO_FAIL_FAST} --junit-report=${TEST_SUITE}.xml --output-dir=test-output test/${TEST_SUITE} -- -cluster_namespace=$MANAGED_CLUSTER_NAME || EXIT_CODE=$?

  # Remove "[It] " from report to prevent corrupting bracketed metadata
  if [ -f test-output/${TEST_SUITE}.xml ]; then
    sed -i 's/\[It\] *//g' test-output/${TEST_SUITE}.xml
  fi

  # Collect exit code if it's an error
  if [[ "${EXIT_CODE}" != "0" ]]; then
    ERROR_CODE=${EXIT_CODE}
  fi
done

if [[ -n "${ERROR_CODE}" ]]; then
    echo "* Detected test failure. Collecting debug logs..."
    # For debugging, the managed cluster might have a different name (i.e. 'local-cluster') but the
    # kubeconfig is still called 'kubeconfig_managed'
    export MANAGED_CLUSTER_NAME="managed"
    make e2e-debug-acm
fi

# Since we may have captured an exit code previously, manually exit with it here
exit ${ERROR_CODE}
