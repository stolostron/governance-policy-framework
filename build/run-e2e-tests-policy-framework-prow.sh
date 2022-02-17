#!/bin/bash
# Copyright Contributors to the Open Cluster Management project


set -e

DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" >/dev/null 2>&1 && pwd )"

echo "===== E2E Cluster Setup ====="
# Specify kubeconfig files (the default values are the ones generated in Prow)
HUB_NAME=${HUB_NAME:-"hub-1"}
MANAGED_NAME=${MANAGED_NAME:-"managed-1"}
HUB_KUBE=${HUB_KUBE:-"${SHARED_DIR}/${HUB_NAME}.kc"}
MANAGED_KUBE=${MANAGED_KUBE:-"${SHARED_DIR}/${MANAGED_NAME}.kc"}

if (ls "${MANAGED_KUBE}" &>/dev/null); then
  export KUBECONFIG=${MANAGED_KUBE}
  export MANAGED_CLUSTER_NAME=${MANAGED_CLUSTER_NAME:-${MANAGED_NAME}}
else
  echo "* Managed cluster not found. Continuing using Hub as managed."
  export KUBECONFIG=${HUB_KUBE}
  export MANAGED_CLUSTER_NAME="local-cluster"
  MANAGED_KUBE=${HUB_KUBE}
fi

echo "* Install cert manager"
$DIR/install-cert-manager.sh

echo "* Set up cluster for test"
$DIR/cluster-patch.sh
cp ${HUB_KUBE} $DIR/../kubeconfig_hub
cp ${MANAGED_KUBE} $DIR/../kubeconfig_managed

echo "===== E2E Test ====="
echo "* Launching grc policy framework test"
for TEST_SUITE in integration policy-collection; do
  CGO_ENABLED=0 ginkgo -v --slow-spec-threshold=10s --junit-report=${TEST_SUITE}.xml --output-dir=test_output test/${TEST_SUITE} -- -cluster_namespace=$MANAGED_CLUSTER_NAME || EXIT_CODE=$?
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

if [[ -n "${ARTIFACT_DIR}" ]]; then
  echo "* Copying 'test-output' directory to '${ARTIFACT_DIR}'..."
  cp -r $DIR/../test-output/* ${ARTIFACT_DIR}
fi

# Since we obfuscated the error code on the test run, we'll manually exit here with the collected code
exit ${ERROR_CODE}