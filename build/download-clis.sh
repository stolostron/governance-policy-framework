#!/bin/bash
# Copyright (c) 2020 Red Hat, Inc.
# Copyright Contributors to the Open Cluster Management project

set -e

CLI_DESTINATION_DIR="${CLI_DESTINATION_DIR:=/usr/local/bin}"

if ! which oc > /dev/null; then
    echo "Installing oc and kubectl clis in $CLI_DESTINATION_DIR..."
    mkdir clis-unpacked
    curl -kLo oc.tar.gz https://mirror.openshift.com/pub/openshift-v4/clients/ocp/4.10.21/openshift-client-linux.tar.gz
    tar -xzf oc.tar.gz -C clis-unpacked
    chmod 755 ./clis-unpacked/oc
    chmod 755 ./clis-unpacked/kubectl
    mv ./clis-unpacked/oc "$CLI_DESTINATION_DIR/oc"
    mv ./clis-unpacked/kubectl "$CLI_DESTINATION_DIR/kubectl"
fi
