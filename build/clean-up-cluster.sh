#!/bin/bash
# Copyright Contributors to the Open Cluster Management project


set -e

function delete_all_and_wait() {
    RESOURCE=$1
    NAMESPACE=$2
    COUNT=$3
    SKIP_DELETE=$4

    if [ "$SKIP_DELETE" != "true" ] ; then
        oc delete $RESOURCE -n $NAMESPACE --all --ignore-not-found || true # RESOURCE type might not exist
    fi

    START_TIME=$(date -u +%s)
    WAIT_TIME_MAX=5*60  # in seconds
    while [ $(oc get $RESOURCE -n $NAMESPACE --no-headers | wc -l | tr -d '[:space:]') -gt $COUNT ]
    do
        if [ $(($START_TIME+$WAIT_TIME_MAX)) -le $(date -u +%s) ]; then
            echo Timed out...
            break
        fi
        oc get $RESOURCE -n $NAMESPACE
        sleep 2
    done
}

function hub() {
    echo "Hub: clean up"
    oc delete policies.policy.open-cluster-management.io --all-namespaces --all --ignore-not-found
    oc delete placementbindings.policy.open-cluster-management.io  --all-namespaces --all --ignore-not-found
    # Don't clean up in all namespaces because the global-set placement shouldn't be deleted
    for ns in default policy-test e2e-rbac-test-1 e2e-rbac-test-2
        do
            oc delete placements.cluster.open-cluster-management.io -n $ns --all --ignore-not-found
        done
    oc delete ns -l e2e=true --ignore-not-found
    oc delete ns policy-test --ignore-not-found
    oc delete ns duplicatetest --ignore-not-found
}

function managed() {
    echo "Managed: clean up"
    oc delete pod --all -n default --ignore-not-found
    oc delete issuers.cert-manager.io -l e2e=true -n default --ignore-not-found || true  # issuer CRD might not exist
    oc delete certificates.cert-manager.io -l e2e=true -n default --ignore-not-found || true  # certificate CRD might not exist
    oc delete secret -n default rsa-ca-sample-secret --ignore-not-found 
    oc delete clusterrolebinding -l e2e=true --ignore-not-found
    oc delete subscriptions.operators.coreos.com container-security-operator -n openshift-operators --ignore-not-found
    oc delete csv -n openshift-operators "$(oc get -n openshift-operators csv -o jsonpath='{.items[?(@.spec.displayName=="Quay Container Security")].metadata.name}')" --ignore-not-found || true  # csv might not exist
    oc delete csv -n openshift-operators "$(oc get -n openshift-operators csv -o jsonpath='{.items[?(@.spec.displayName=="Red Hat Quay Container Security Operator")].metadata.name}')" --ignore-not-found || true  # csv might not exist
    oc delete crd imagemanifestvulns.secscan.quay.redhat.com --ignore-not-found
    oc delete operatorgroup awx-resource-operator-operatorgroup -n default --ignore-not-found
    oc delete subscriptions.operators.coreos.com awx-resource-operator -n default --ignore-not-found
    oc delete csv awx-resource-operator.v0.1.1 -n default --ignore-not-found
    oc delete secret grcui-e2e-credential -n default --ignore-not-found
    oc delete LimitRange container-mem-limit-range -n default --ignore-not-found
    oc delete ns prod --ignore-not-found
    oc delete psp restricted-psp --ignore-not-found || true # the podsecuritypolicy API might not exist
    oc delete role deployments-role -n default --ignore-not-found
    oc delete rolebinding operatoruser-rolebinding -n default --ignore-not-found
    oc delete scc restricted-scc --ignore-not-found
    oc delete ns -l e2e=true --ignore-not-found
    # Gatekeeper clean up
    oc delete ns e2etestsuccess e2etestfail --ignore-not-found
    oc delete Gatekeeper gatekeeper --ignore-not-found || true # Gatekeeper CRD might not exist
    # Wait for gatekeeper operator to remove the gatekeeper pods
    delete_all_and_wait pods openshift-gatekeeper-system 0
    oc delete ns openshift-gatekeeper-system gatekeeper-system --ignore-not-found
    oc delete subscriptions.operators.coreos.com gatekeeper-operator-product -n openshift-operators --ignore-not-found
    oc delete csv -n openshift-operators "$(oc get -n openshift-operators csv -o jsonpath='{.items[?(@.spec.displayName=="Gatekeeper Operator")].metadata.name}')" --ignore-not-found || true  # csv might not exist
    oc delete ns openshift-gatekeeper-operator --ignore-not-found
    oc delete crd gatekeepers.operator.gatekeeper.sh --ignore-not-found
    oc delete validatingwebhookconfigurations.admissionregistration.k8s.io gatekeeper-validating-webhook-configuration --ignore-not-found
    oc delete mutatingwebhookconfigurations.admissionregistration.k8s.io gatekeeper-mutating-webhook-configuration --ignore-not-found
    # Compliance Operator clean up
    oc delete ScanSettingBinding -n openshift-compliance --all --ignore-not-found || true # ScanSettingBinding CRD might not exist
    RESOURCES=(ComplianceSuite ComplianceCheckResult ComplianceScan)
    for RESOURCE in "${RESOURCES[@]}"; do
        delete_all_and_wait $RESOURCE openshift-compliance 0
    done
    # only three pods should be left
    delete_all_and_wait pods openshift-compliance 3 "true"
    delete_all_and_wait ProfileBundle openshift-compliance 0
    oc delete subscriptions.operators.coreos.com compliance-operator -n openshift-compliance --ignore-not-found
    oc delete operatorgroup compliance-operator -n openshift-compliance --ignore-not-found
    oc delete csv -n openshift-compliance "$(oc get -n openshift-compliance csv -o jsonpath='{.items[?(@.spec.displayName=="Compliance Operator")].metadata.name}')" --ignore-not-found || true  # csv might not exist
    oc delete ns openshift-compliance --ignore-not-found
    oc delete crd -l operators.coreos.com/compliance-operator.openshift-compliance --ignore-not-found
    # Clean up events in cluster ns
    oc delete events -n local-cluster --all --ignore-not-found
}

hub
managed

./build/install-cert-manager.sh
