#!/bin/bash
# Copyright Contributors to the Open Cluster Management project


set -e

ginkgo -v --slowSpecThreshold=10 test/policy-collection -- -cluster_namespace=$MANAGED_CLUSTER_NAME
