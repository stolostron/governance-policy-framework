#!/bin/bash

# Copyright (c) 2020 Red Hat, Inc.

# Description:
#     Sets up cluster users and resources for RBAC tests
#
# From the grc-ui project directory, invoke as:
#     ./build/rbac-setup.sh
# Be sure to export the desired password beforehand:
#     export RBAC_PASS=<your-password>

set -e
RBAC_DIR=${TRAVIS_BUILD_DIR:-.}

if [ -z ${RBAC_PASS} ]; then
  echo "Error: RBAC password not set in variable RBAC_PASS."
  exit 1
fi

if ! which htpasswd &>/dev/null; then
  if which apt-get &>/dev/null; then
    sudo apt-get update
    sudo apt-get install -y apache2-utils
  else
    echo "Error: Package manager apt-get not found. Failed to find or install htpasswd."
    exit 1
  fi
fi

touch ${RBAC_DIR}/htpasswd
for access in cluster ns; do
  for role in cluster-admin admin edit view group; do
    htpasswd -b ${RBAC_DIR}/htpasswd e2e-${role}-${access} ${RBAC_PASS}
  done
done
oc create secret generic e2e-users --from-file=htpasswd=${RBAC_DIR}/htpasswd -n openshift-config || true
rm ${RBAC_DIR}/htpasswd
oc apply --validate=false -k ${RBAC_DIR}

export SELENIUM_USER=e2e-cluster-admin-cluster
export SELENIUM_PASSWORD=${RBAC_PASS}
export SELENIUM_USER_SELECT=e2e-htpasswd
