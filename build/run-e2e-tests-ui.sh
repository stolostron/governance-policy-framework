#!/bin/bash

set -e
UI_CURRENT_IMAGE=$1

echo "Login hub to clean up"
export OC_CLUSTER_URL=$OC_HUB_CLUSTER_URL
export OC_CLUSTER_PASS=$OC_HUB_CLUSTER_PASS
make oc/login
oc delete policies.policy.open-cluster-management.io -n default --all || true
# placementbindings.mcm.ibm.com throws error when doesn't exist
oc delete placementbindings.policy.open-cluster-management.io  -n default --all || true
oc delete placementrule  -n default --all || true

echo "Logout"
export OC_COMMAND=logout
make oc/command

echo "Login managed to clean up"
export OC_CLUSTER_URL=$OC_MANAGED_CLUSTER_URL
export OC_CLUSTER_PASS=$OC_MANAGED_CLUSTER_PASS
make oc/login
oc delete pod --all -n default || true
# secrets=`oc get certificate -l e2e=true -o=jsonpath='{.items[*].spec.secretName}'`
oc delete issuer -l e2e=true -n default || true
oc delete certificate -l e2e=true -n default || true
oc delete secret -n default rsa-ca-sample-secret || true # in case secrets are empty
oc delete clusterrolebinding -l e2e=true || true

echo "Install cert manager on managed"
oc apply --validate=false -f https://github.com/jetstack/cert-manager/releases/download/v0.15.1/cert-manager.yaml

echo "Logout"
export OC_COMMAND=logout
make oc/command

echo "Login hub again"
export OC_CLUSTER_URL=$OC_HUB_CLUSTER_URL
export OC_CLUSTER_PASS=$OC_HUB_CLUSTER_PASS
make oc/login
# export SERVICEACCT_TOKEN=`${BUILD_HARNESS_PATH}/vendor/oc whoami --show-token`
# echo "SERVICEACCT_TOKEN=$SERVICEACCT_TOKEN"

# echo "Create RBAC users"
# source ${TRAVIS_BUILD_DIR}/build/rbac-setup.sh

make docker/login
export DOCKER_URI=quay.io/open-cluster-management/grc-ui-tests:latest-dev
make docker/pull

export SELENIUM_CLUSTER=https://`oc get route multicloud-console -o=jsonpath='{.spec.host}'   `
export SELENIUM_USER=${SELENIUM_USER:-${OC_CLUSTER_USER}}
export SELENIUM_PASSWORD=${SELENIUM_PASSWORD:-${OC_HUB_CLUSTER_PASS}}


docker run --volume $(pwd)/results:/opt/app-root/src/grc-ui/test-output/e2e --env SELENIUM_CLUSTER=$SELENIUM_CLUSTER --env SELENIUM_PASSWORD=$SELENIUM_PASSWORD --env SELENIUM_USER=$SELENIUM_USER --env DISABLE_CANARY_TEST=true --env SKIP_NIGHTWATCH_COVERAGE=true --env SKIP_LOG_DELETE=true $DOCKER_URI
