#!/bin/bash

set -e
UI_CURRENT_IMAGE=$1

make docker/login
export DOCKER_URI=quay.io/stolostron/grc-ui-tests:latest-2.2
make docker/pull

docker run --volume $(pwd)/build:/opt/app-root/src/grc-ui/tmp \
    $DOCKER_URI \
    cp ./build/{cluster-clean-up.sh,install-cert-manager.sh} tmp/

echo "Login hub to clean up"
export OC_CLUSTER_URL=$OC_HUB_CLUSTER_URL
export OC_CLUSTER_PASS=$OC_HUB_CLUSTER_PASS
make oc/login
./build/cluster-clean-up.sh hub

echo "Logout"
export OC_COMMAND=logout
make oc/command

echo "Login managed to clean up"
export OC_CLUSTER_URL=${OC_MANAGED_CLUSTER_URL:-${OC_HUB_CLUSTER_URL}}
export OC_CLUSTER_PASS=${OC_MANAGED_CLUSTER_PASS:-${OC_HUB_CLUSTER_PASS}}
make oc/login
./build/cluster-clean-up.sh managed

./build/install-cert-manager.sh

echo "Logout"
export OC_COMMAND=logout
make oc/command

printenv

docker run --volume $(pwd)/results:/opt/app-root/src/grc-ui/test-output/e2e \
    --volume $(pwd)/results-cypress:/opt/app-root/src/grc-ui/test-output/cypress \
    --env OC_CLUSTER_URL=$OC_HUB_CLUSTER_URL \
    --env OC_CLUSTER_PASS=$OC_HUB_CLUSTER_PASS \
    --env OC_CLUSTER_USER=$OC_CLUSTER_USER \
    --env RBAC_PASS=$RBAC_PASS \
    --env PAUSE=0 \
    --env FAIL_FAST=true \
    $DOCKER_URI
