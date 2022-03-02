#!/bin/bash
# Copyright Contributors to the Open Cluster Management project


set -e

function hub() {
    echo "Hub: clean up"
    for ns in default policy-test e2e-rbac-test-1 e2e-rbac-test-2
        do
            oc delete policies.policy.open-cluster-management.io -n $ns --all --ignore-not-found
            oc delete placementbindings.policy.open-cluster-management.io  -n $ns --all --ignore-not-found
            oc delete placementrules.apps.open-cluster-management.io -n $ns --all --ignore-not-found
        done
    oc delete ns -l e2e=true --ignore-not-found
    oc delete ns policy-test duplicatetest --ignore-not-found
}

function managed() {
    echo "Managed: clean up"
    oc delete pod --all -n default --ignore-not-found
    oc delete issuers.cert-manager.io -l e2e=true -n default --ignore-not-found
    oc delete certificates.cert-manager.io -l e2e=true -n default --ignore-not-found
    oc delete secret -n default rsa-ca-sample-secret --ignore-not-found 
    oc delete clusterrolebinding -l e2e=true --ignore-not-found
    oc delete subscriptions.operators.coreos.com container-security-operator -n openshift-operators --ignore-not-found
    oc delete csv -n openshift-operators `oc get -n openshift-operators csv -o jsonpath='{.items[?(@.spec.displayName=="Quay Container Security")].metadata.name}'` --ignore-not-found || true  # csv might not exist
    oc delete crd imagemanifestvulns.secscan.quay.redhat.com --ignore-not-found
    oc delete operatorgroup awx-resource-operator-operatorgroup -n default --ignore-not-found
    oc delete subscriptions.operators.coreos.com awx-resource-operator -n default --ignore-not-found
    oc delete csv awx-resource-operator.v0.1.1 -n default --ignore-not-found
    oc delete secret grcui-e2e-credential -n default --ignore-not-found
    oc delete LimitRange container-mem-limit-range -n default --ignore-not-found
    oc delete ns prod --ignore-not-found
    oc delete psp restricted-psp --ignore-not-found
    oc delete role deployments-role -n default --ignore-not-found
    oc delete rolebinding operatoruser-rolebinding -n default --ignore-not-found
    oc delete scc restricted-scc --ignore-not-found
    oc delete ns -l e2e=true --ignore-not-found
    oc delete ns e2etestsuccess e2etestfail --ignore-not-found
    oc delete Gatekeeper gatekeeper --ignore-not-found || true # Gatekeeper CRD might not exist
    sleep 5 # Wait for gatekeeper operator to remove the gatekeeper pods
    oc delete ns openshift-gatekeeper-system gatekeeper-system --ignore-not-found
    oc delete subscriptions.operators.coreos.com gatekeeper-operator-product -n openshift-operators --ignore-not-found
    oc delete csv -n openshift-operators `oc get -n openshift-operators csv -o jsonpath='{.items[?(@.spec.displayName=="Gatekeeper Operator")].metadata.name}'` --ignore-not-found || true  # csv might not exist
    oc delete ns openshift-gatekeeper-operator --ignore-not-found
    oc delete crd gatekeepers.operator.gatekeeper.sh --ignore-not-found
    oc delete validatingwebhookconfigurations.admissionregistration.k8s.io gatekeeper-validating-webhook-configuration --ignore-not-found
    oc delete mutatingwebhookconfigurations.admissionregistration.k8s.io gatekeeper-mutating-webhook-configuration --ignore-not-found
}

echo "Login..."
export OC_CLUSTER_URL=$OC_HUB_CLUSTER_URL
export OC_CLUSTER_PASS=$OC_HUB_CLUSTER_PASS
make oc/login

hub
managed

./build/install-cert-manager.sh
