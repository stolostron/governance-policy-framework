#! /bin/bash

set -e

: "${MANAGED_KUBE?:MANAGED_KUBE must be set.}"
: "${MANAGED_CLUSTER_NAME?:MANAGED_CLUSTER_NAME must be set.}"

echo "* Attaching managed cluster ${MANAGED_CLUSTER_NAME} to hub"

oc create ns "${MANAGED_CLUSTER_NAME}"

cat <<EOF | oc create -f -
apiVersion: cluster.open-cluster-management.io/v1
kind: ManagedCluster
metadata:
  name: ${MANAGED_CLUSTER_NAME}
  labels:
    cloud: auto-detect
    vendor: auto-detect
spec:
  hubAcceptsClient: true
EOF

cat <<EOF | oc create -f -
apiVersion: v1
kind: Secret
metadata:
  name: auto-import-secret
  namespace: ${MANAGED_CLUSTER_NAME}
stringData:
  autoImportRetry: "5"
  kubeconfig: |-
$(sed 's/^/    /g' "${MANAGED_KUBE}")
type: Opaque
EOF

for i in {1..6}; do
  err_code=0
  echo "Waiting for ManagedCluster ${MANAGED_CLUSTER_NAME} to be available (${i}/6) ..."
  oc wait managedcluster "${MANAGED_CLUSTER_NAME}" \
    --for condition=ManagedClusterJoined=True \
    --for condition=ManagedClusterConditionAvailable=True \
    --timeout 30s && break || err_code=$?
done

oc get managedcluster "${MANAGED_CLUSTER_NAME}"

if [[ "${err_code}" != "0" ]]; then
  echo "ManagedCluster ${MANAGED_CLUSTER_NAME} failed to become available."
  exit "${err_code}"
fi

cat <<EOF | oc create -f -
apiVersion: agent.open-cluster-management.io/v1
kind: KlusterletAddonConfig
metadata:
  name: ${MANAGED_CLUSTER_NAME}
  namespace: ${MANAGED_CLUSTER_NAME}
spec:
  applicationManager:
    enabled: true
  certPolicyController:
    enabled: true
  policyController:
    enabled: true
  searchCollector:
    enabled: true
EOF

for idx in {1..6}; do
  err_code=0
  echo "Waiting for ManagedClusterAddons to be available (${idx}/6)"
  kubectl get managedclusteraddons -n "${MANAGED_CLUSTER_NAME}"
  kubectl wait managedclusteraddons -n "${MANAGED_CLUSTER_NAME}" --all \
    --for condition=Available=True &&
    break || err_code=$?
done

kubectl get managedclusteraddons -n "${MANAGED_CLUSTER_NAME}"

if [[ "${err_code}" != "0" ]]; then
  echo "ManagedClusterAddons for ${MANAGED_CLUSTER_NAME} failed to become available."
  exit "${err_code}"
fi
