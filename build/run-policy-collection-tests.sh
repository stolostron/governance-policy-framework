#!/bin/bash

set -e

echo "Login hub"
make oc/login

oc create ns policies || true

git clone git@github.com:open-cluster-management/policy-collection.git
cd policy-collection/deploy

./deploy.sh https://github.com/open-cluster-management/policy-collection.git stable policies