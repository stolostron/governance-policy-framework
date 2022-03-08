#!/bin/bash
# Copyright (c) 2020 Red Hat, Inc.
# Copyright Contributors to the Open Cluster Management project


if [ $(oc get ns cert-manager | grep Active | wc -l | tr -d '[:space:]') -eq 1 ]; then
    echo "Cert manager already installed"
else 
    echo "Install cert manager on managed"
    oc apply --validate=false -f https://github.com/jetstack/cert-manager/releases/download/v1.5.5/cert-manager.yaml
    ./build/wait_for.sh pod -n cert-manager
fi
