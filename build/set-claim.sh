#! /bin/bash

# Make sure we have `oc`
sudo ./build/download-clis.sh

# Log into Collective cluster
KUBECONFIG_FILE="${PWD}/kubeconfig-collective"
touch ${KUBECONFIG_FILE} || { echo "Failed to create kubeconfig file"; exit 1; }
export KUBECONFIG=${KUBECONFIG_FILE}

OC_OUTPUT=$(oc login --token="${COLLECTIVE_TOKEN}" https://api.collective.aws.red-chesterfield.com:6443 --insecure-skip-tls-verify)
if [ $? = "0" ]; then
  echo "Logged in to Collective cluster"
else
  echo "${OC_OUTPUT}"
  echo "Failed to log in to Collective cluster"
  rm ${KUBECONFIG_FILE}
  exit 1
fi

# Set E2E ClusterClaim PowerState (Running/Hibernating)
CLAIM="grce2e-policy-grc-cp-prow"

if [ "$1" == "--hibernate" ]; then
  POWER_STATE="Hibernating"
else
  POWER_STATE="Running"
fi

echo "Setting ClusterClaim ${CLAIM} to ${POWER_STATE}..."

DEPLOYMENT=$(oc get clusterclaims.hive $CLAIM -o jsonpath={.spec.namespace})

oc patch clusterdeployment.hive $DEPLOYMENT -n $DEPLOYMENT --type='merge' -p $'spec:\n powerState: '${POWER_STATE}''

if [ "${POWER_STATE}" = "Running" ]; then
  # Wait for ClusterClaim to be Running
  for i in {1..20}; do
    echo "Checking whether ClusterClaim ${CLAIM} is Running (${i}/10):"
    oc wait --for=condition=ClusterRunning=True clusterclaims.hive/${CLAIM} --timeout 30s
    EXIT_CODE=$?
    if [[ "${EXIT_CODE}" == "0" ]]; then
      break
    fi
  done
fi

rm ${KUBECONFIG_FILE}

exit ${EXIT_CODE}
