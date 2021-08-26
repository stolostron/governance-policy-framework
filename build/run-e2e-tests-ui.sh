#!/bin/bash
# Copyright Contributors to the Open Cluster Management project


set -e
UI_CURRENT_IMAGE=$1

make docker/login
export DOCKER_URI=quay.io/open-cluster-management/grc-ui-tests:latest
make docker/pull

printenv

docker run --volume $(pwd)/results:/opt/app-root/src/grc-ui/test-output/e2e \
    --volume $(pwd)/results-cypress:/opt/app-root/src/grc-ui/test-output/cypress \
    --env OC_CLUSTER_URL=$OC_HUB_CLUSTER_URL \
    --env OC_CLUSTER_PASS=$OC_HUB_CLUSTER_PASS \
    --env OC_CLUSTER_USER=$OC_CLUSTER_USER \
    --env RBAC_PASS=$RBAC_PASS \
    --env PAUSE=0 \
    --env FAIL_FAST=true \
    --env CYPRESS_STANDALONE_TESTSUITE_EXECUTION=FALSE \
    --env CYPRESS_TAGS_INCLUDE=$CYPRESS_TAGS_INCLUDE \
    --env CYPRESS_TAGS_EXCLUDE=$CYPRESS_TAGS_EXCLUDE \
    --env MANAGED_CLUSTER_NAME=$MANAGED_CLUSTER_NAME \
    $DOCKER_URI
