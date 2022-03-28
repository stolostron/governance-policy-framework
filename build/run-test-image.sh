#!/bin/bash
# Copyright Contributors to the Open Cluster Management project

set -e

if [[ ${FAIL_FAST} == "true" ]]; then
  echo "* Running in fail fast mode"
  GINKGO_FAIL_FAST="--fail-fast" 
fi

echo "===== E2E Test ====="
echo "* Launching grc policy framework test"
if [[ -z ${GINKGO_LABEL_FILTER} ]]; then 
  echo "* No GINKGO_LABEL_FILTER set"
  LABEL_FILTERS=("!etcd" "etcd")
else
  echo "* Using GINKGO_LABEL_FILTER=${GINKGO_LABEL_FILTER}"
  LABEL_FILTERS=${GINKGO_LABEL_FILTER}
fi

for LABEL_FILTER in ${LABEL_FILTERS[@]}; do
  CGO_ENABLED=0 ginkgo -v --no-color --fail-fast --label-filter="$LABEL_FILTER" --junit-report=integration-$LABEL_FILTER.xml --output-dir=test-output test/integration -- -cluster_namespace=$MANAGED_CLUSTER_NAME || EXIT_CODE=$?

  # Collect exit code if it's an error
  if [[ "${EXIT_CODE}" != "0" ]]; then
    ERROR_CODE=${EXIT_CODE}
  fi
done

# Remove Ginkgo phases from report to prevent corrupting bracketed metadata
REPORTS=`find test-output/*.xml`
for report in $REPORTS; do  
  echo "* Updating report $report"
  sed -i 's/\[It\] *//g' $report
  sed -i 's/\[BeforeSuite\]/GRC: [P1][Sev1][policy-grc] BeforeSuite/g' $report
  sed -i 's/\[AfterSuite\]/GRC: [P1][Sev1][policy-grc] AfterSuite/g' $report
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
