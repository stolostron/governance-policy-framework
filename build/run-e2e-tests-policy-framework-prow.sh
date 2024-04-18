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
${DIR}/install-cert-manager.sh

echo "* Set up cluster for test"

# Set tag for images: Use `latest` for `main` and `latest-<version>` for `release-<version>`
# branches. If the PR is in `openshift/release`, parse the job spec for the target branch.
# Otherwise, use `PULL_BASE_REF`.
VERSION_TAG="latest"
if [[ "${REPO_OWNER}" == "openshift" ]] && [[ "${REPO_NAME}" == "release" ]]; then
  TARGET_BRANCH="$(echo "${JOB_SPEC}" | jq -r '.extra_refs[] | select(.workdir == true).base_ref')"
else
  TARGET_BRANCH="${PULL_BASE_REF}"
fi
if [[ "${TARGET_BRANCH}" ]] && [[ "${TARGET_BRANCH}" != "main" ]]; then
  VERSION_TAG="${VERSION_TAG}-${PULL_BASE_REF#*-}"
fi

export KUBECONFIG=${HUB_KUBE}
VERSION_TAG="${VERSION_TAG}" ${DIR}/patch-cluster-prow.sh

if [[ "${HUB_KUBE}" != "${MANAGED_KUBE}" ]]; then
  MANAGED_KUBE=${MANAGED_KUBE} MANAGED_CLUSTER_NAME=${MANAGED_CLUSTER_NAME} ${DIR}/import-managed.sh
fi

cp ${HUB_KUBE} ${DIR}/../kubeconfig_hub
cp ${MANAGED_KUBE} ${DIR}/../kubeconfig_managed

if [[ -z ${GINKGO_LABEL_FILTER} ]]; then 
  echo "* No GINKGO_LABEL_FILTER set"
else
  GINKGO_LABEL_FILTER="--label-filter=${GINKGO_LABEL_FILTER}"
  echo "* Using GINKGO_LABEL_FILTER=${GINKGO_LABEL_FILTER}"
fi

echo "===== E2E Test ====="
echo "* Launching grc policy framework test"
# Run test suite with reporting
CGO_ENABLED=0 ${DIR}/../bin/ginkgo -v --no-color --fail-fast ${GINKGO_LABEL_FILTER} --junit-report=integration.xml --output-dir=test-output test/integration -- -cluster_namespace=$MANAGED_CLUSTER_NAME || EXIT_CODE=$?

# Remove Ginkgo phases from report to prevent corrupting bracketed metadata
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

if [[ -n "${ARTIFACT_DIR}" ]]; then
  echo "* Copying 'test-output' directory to '${ARTIFACT_DIR}'..."
  cp -r $DIR/../test-output/* ${ARTIFACT_DIR}
fi

# Since we captured the error code on the test run, we'll manually exit here with the collected code
exit ${ERROR_CODE}
