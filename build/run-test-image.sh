#!/bin/bash

set -e

./build/download-clis.sh

echo "Installing ginkgo ..."
go get github.com/onsi/ginkgo/ginkgo
go get github.com/onsi/gomega/...

ginkgo -v --slowSpecThreshold=10 test/policy-collection -- -cluster_namespace=$MANAGED_CLUSTER_NAME