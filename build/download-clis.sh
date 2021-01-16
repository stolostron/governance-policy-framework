#!/bin/bash

# Copyright (c) 2020 Red Hat, Inc.

set -e

if ! which oc > /dev/null; then
    echo "Installing oc and kubectl clis..."
    mkdir clis-unpacked
    curl -kLo oc.tar.gz https://mirror.openshift.com/pub/openshift-v4/clients/ocp/latest/openshift-client-linux.tar.gz
    tar -xzf oc.tar.gz -C clis-unpacked
    chmod 755 ./clis-unpacked/oc
    chmod 755 ./clis-unpacked/kubectl
    mv ./clis-unpacked/oc /usr/local/bin/oc
    mv ./clis-unpacked/kubectl /usr/local/bin/kubectl
fi