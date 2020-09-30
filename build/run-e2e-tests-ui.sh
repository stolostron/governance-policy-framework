#!/bin/bash

set -e
UI_CURRENT_IMAGE=$1

echo "Login hub to clean up"
export OC_CLUSTER_URL=$OC_HUB_CLUSTER_URL
export OC_CLUSTER_PASS=$OC_HUB_CLUSTER_PASS
make oc/login
for ns in default e2e-rbac-test-1 e2e-rbac-test-2
do
    oc delete policies.policy.open-cluster-management.io -n $ns --all || true
    oc delete placementbindings.policy.open-cluster-management.io  -n $ns --all || true
    oc delete placementrules.apps.open-cluster-management.io -n $ns --all || true
done

echo "Logout"
export OC_COMMAND=logout
make oc/command

echo "Login managed to clean up"
export OC_CLUSTER_URL=${OC_MANAGED_CLUSTER_URL:-${OC_HUB_CLUSTER_URL}}
export OC_CLUSTER_PASS=${OC_MANAGED_CLUSTER_PASS:-${OC_HUB_CLUSTER_PASS}}
make oc/login
oc delete pod --all -n default || true
oc delete issuers.cert-manager.io -l e2e=true -n default || true
oc delete certificates.cert-manager.io -l e2e=true -n default || true
oc delete secret -n default rsa-ca-sample-secret || true # in case secrets are empty
oc delete clusterrolebinding -l e2e=true || true

if [ $(oc get ns cert-manager | grep Active | wc -l | tr -d '[:space:]') -eq 1 ]; then
    echo "Cert manager alreayd installed"
else 
    echo "Install cert manager on managed"
    oc apply --validate=false -f https://github.com/jetstack/cert-manager/releases/download/v1.0.1/cert-manager.yaml
fi

echo "Logout"
export OC_COMMAND=logout
make oc/command

make docker/login
export DOCKER_URI=quay.io/open-cluster-management/grc-ui-tests:latest-dev
make docker/pull

printenv

docker run --volume $(pwd)/results:/opt/app-root/src/grc-ui/test-output/e2e \
    --env OC_CLUSTER_URL=$OC_HUB_CLUSTER_URL \
    --env OC_CLUSTER_PASS=$OC_HUB_CLUSTER_PASS \
    --env OC_CLUSTER_USER=$OC_CLUSTER_USER \
    --env RBAC_PASS=$RBAC_PASS \
    --env PAUSE=0 \
    $DOCKER_URI
