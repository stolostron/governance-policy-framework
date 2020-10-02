#!/bin/bash

set -e

if ! which kubectl > /dev/null; then
    echo "installing kubectl"
    curl -LO https://storage.googleapis.com/kubernetes-release/release/$(curl -s https://storage.googleapis.com/kubernetes-release/release/stable.txt)/bin/linux/amd64/kubectl && chmod +x kubectl && sudo mv kubectl /usr/local/bin/
fi

echo "Login hub"
export OC_CLUSTER_URL=$OC_HUB_CLUSTER_URL
export OC_CLUSTER_PASS=$OC_HUB_CLUSTER_PASS
make oc/login

kubectl create ns policy-collection-e2e || true

git clone https://github.com/open-cluster-management/policy-collection.git
cd policy-collection/deploy

./deploy.sh https://github.com/open-cluster-management/policy-collection.git stable policy-collection-e2e

function cleanup {
    kubectl delete subscriptions -n policy-collection-e2e --all || true
    sleep 10
    kubectl delete channels -n policy-collection-e2e --all || true
    kubectl delete policies -n policy-collection-e2e --all || true
}

COMPLETE=1
for i in {1..20}; do
    ROOT_POLICIES=$(kubectl get policies -n policy-collection-e2e | tail -n +2 | wc -l | tr -d '[:space:]')
    TOTAL_POLICIES=$(kubectl get policies -A | grep e2e | wc -l | tr -d '[:space:]')
    echo "Number of expected Policies : 11/22"
    echo "Number of actual Policies : $ROOT_POLICIES/$TOTAL_POLICIES"
    if [ $ROOT_POLICIES -eq 11 ]; then
        COMPLETE=0
        break
    fi
    sleep 10
done
if [ $COMPLETE -eq 1 ]; then
    echo "Failed to deploy policies from policy repo"
    kubectl get policies -A
   cleanup
    exit 1
fi
echo "Test was successful! cleaning up..."
cleanup
exit 0
