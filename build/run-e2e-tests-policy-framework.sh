#!/bin/bash

set -e
TEST_IMAGE_URI=$1

printenv

docker run --volume $(pwd)/results:/go/src/github.com/open-cluster-management/governance-policy-framework/test-output \
    --env OC_CLUSTER_URL=$OC_HUB_CLUSTER_URL \
    --env OC_CLUSTER_PASS=$OC_HUB_CLUSTER_PASS \
    --env OC_CLUSTER_USER=$OC_CLUSTER_USER \
    $TEST_IMAGE_URI
