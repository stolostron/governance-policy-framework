#!/bin/bash
# Copyright Contributors to the Open Cluster Management project

set -e

ginkgo -v --slowSpecThreshold=10 test/policy-collection test/integration -- -cluster_namespace=$MANAGED_CLUSTER_NAME || ERROR_CODE=$?

if [[ -n "${ERROR_CODE}" ]]; then
    echo "* Detected test failure. Collecting debug logs..."
    # For debugging, the managed cluster might have a different name (i.e. 'local-cluster') but the
    # kubeconfig is still called 'kubeconfig_managed'
    export MANAGED_CLUSTER_NAME="managed"
    make e2e-debug-acm
fi

# Since we may have captured an exit code previously, manually exit with it here
exit ${ERROR_CODE}
