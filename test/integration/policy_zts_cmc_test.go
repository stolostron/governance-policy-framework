// Copyright (c) 2022 Red Hat, Inc.
// Copyright Contributors to the Open Cluster Management project

package integration

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8stypes "k8s.io/apimachinery/pkg/types"
	policiesv1 "open-cluster-management.io/governance-policy-propagator/api/v1"
	"open-cluster-management.io/governance-policy-propagator/test/utils"

	"github.com/stolostron/governance-policy-framework/test/common"
)

var _ = Describe("GRC: [P1][Sev1][policy-grc] Test the zts-cmc policy", Ordered, Label("policy-collection", "stable"), func() {
	const (
		policyPodName   = "policy-zts-cmc"
		policyPodURL    = policyCollectCMURL + policyPodName + ".yaml"
		policyPodNSName = "default"
		deploymentName  = "zts-cmc-app-deploy"
	)

	It("stable/"+policyPodName+" should be created on the Hub", func() {
		By("Creating the policy on the Hub")
		_, err := utils.KubectlWithOutput(
			"apply", "-f", policyPodURL, "-n", userNamespace, "--kubeconfig="+kubeconfigHub,
		)
		Expect(err).To(BeNil())

		By("Patching the namespaceSelector to use the " + policyPodNSName + " namespace")
		_, err = clientHubDynamic.Resource(common.GvrPolicy).Namespace(userNamespace).Patch(
			context.TODO(),
			policyPodName,
			k8stypes.JSONPatchType,
			[]byte(`[{"op": "replace", "path": "/spec/policy-templates/0/objectDefinition/spec/namespaceSelector/include", "value": ["`+policyPodNSName+`"]}]`),
			metav1.PatchOptions{},
		)
		Expect(err).To(BeNil())

		By("Patching placement rule")
		err = common.PatchPlacementRule(
			userNamespace, "placement-"+policyPodName, clusterNamespace, kubeconfigHub,
		)
		Expect(err).To(BeNil())

		By("Checking that " + policyPodName + " exists on the Hub cluster")
		rootPolicy := utils.GetWithTimeout(
			clientHubDynamic, common.GvrPolicy, policyPodName, userNamespace, true, defaultTimeoutSeconds,
		)
		Expect(rootPolicy).NotTo(BeNil())
	})

	It("stable/"+policyPodName+" should be created on managed cluster", func() {
		By("Checking the policy on managed cluster in ns " + clusterNamespace)
		managedPolicy := utils.GetWithTimeout(
			clientManagedDynamic,
			common.GvrPolicy,
			userNamespace+"."+policyPodName,
			clusterNamespace,
			true,
			defaultTimeoutSeconds,
		)
		Expect(managedPolicy).NotTo(BeNil())
	})

	It("stable/"+policyPodName+" should be NonCompliant", func() {
		By("Checking if the status of the root policy is NonCompliant")
		Eventually(
			common.GetComplianceState(clientHubDynamic, userNamespace, policyPodName, clusterNamespace),
			defaultTimeoutSeconds*2,
			1,
		).Should(Equal(policiesv1.NonCompliant))
	})

	It("Enforcing stable/"+policyPodName, func() {
		By("Patching remediationAction = enforce on the root policy")
		_, err := clientHubDynamic.Resource(common.GvrPolicy).Namespace(userNamespace).Patch(
			context.TODO(),
			policyPodName,
			k8stypes.JSONPatchType,
			[]byte(`[{"op": "replace", "path": "/spec/remediationAction", "value": "enforce"}]`),
			metav1.PatchOptions{},
		)
		Expect(err).To(BeNil())
	})

	It("stable/"+policyPodName+" should be Compliant", func() {
		By("Checking if the status of the root policy is Compliant")
		Eventually(
			common.GetComplianceState(clientHubDynamic, userNamespace, policyPodName, clusterNamespace),
			defaultTimeoutSeconds*2,
			1,
		).Should(Equal(policiesv1.Compliant))
	})


	AfterAll(func() {
		out, _ := utils.KubectlWithOutput(
                "get", "deployment", "-n", policyPodNSName, deploymentName, "--kubeconfig="+kubeconfigHub,
		)
		Expect(out).To(ContainSubstring(deploymentName))

		_, err := utils.KubectlWithOutput(
			"delete", "deployment", "-n", policyPodNSName, deploymentName, "--kubeconfig="+kubeconfigHub,
		)
		Expect(err).To(BeNil())

		_, err = utils.KubectlWithOutput(
			"delete", "-f", policyPodURL, "-n", userNamespace, "--kubeconfig="+kubeconfigHub,
		)
		Expect(err).To(BeNil())


	})

})
