#!/bin/bash
# Copyright Contributors to the Open Cluster Management project


set -e
UI_CURRENT_IMAGE=$1

make docker/login
export DOCKER_URI=quay.io/open-cluster-management/grc-ui-tests:latest-dev
make docker/pull

docker run --volume $(pwd)/build:/opt/app-root/src/grc-ui/tmp \
    $DOCKER_URI \
    cp ./build/{cluster-clean-up.sh,install-cert-manager.sh} tmp/

echo "Login hub to clean up"
export OC_CLUSTER_URL=$OC_HUB_CLUSTER_URL
export OC_CLUSTER_PASS=$OC_HUB_CLUSTER_PASS
make oc/login
./build/cluster-clean-up.sh 
./build/cluster-clean-up.sh 

./build/install-cert-manager.sh
