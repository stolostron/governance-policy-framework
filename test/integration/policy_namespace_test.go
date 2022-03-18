// Copyright (c) 2022 Red Hat, Inc.
// Copyright Contributors to the Open Cluster Management project

package integration

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	policiesv1 "github.com/stolostron/governance-policy-propagator/api/v1"
	"github.com/stolostron/governance-policy-propagator/test/utils"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8stypes "k8s.io/apimachinery/pkg/types"

	"github.com/stolostron/governance-policy-framework/test/common"
)

const policyNamespaceURL = "https://raw.githubusercontent.com/stolostron/policy-collection/main/stable/CM-Configuration-Management/policy-namespace.yaml"
const policyNamespaceName = "policy-namespace"

var _ = Describe("GRC: [P1][Sev1][policy-grc] Test the policy-namespace policy", func() {
	It("stable/"+policyNamespaceName+" should be created on the Hub", func() {
		By("Creating policy on hub")
		_, err := utils.KubectlWithOutput(
			"apply", "-f", policyNamespaceURL, "-n", userNamespace, "--kubeconfig="+kubeconfigHub,
		)
		Expect(err).To(BeNil())

		By("Patching placement rule")
		err = common.PatchPlacementRule(
			userNamespace, "placement-"+policyNamespaceName, clusterNamespace, kubeconfigHub,
		)
		Expect(err).To(BeNil())

		By("Checking that " + policyNamespaceName + " exists on the Hub cluster")
		rootPolicy := utils.GetWithTimeout(
			clientHubDynamic, common.GvrPolicy, policyNamespaceName, userNamespace, true, defaultTimeoutSeconds,
		)
		Expect(rootPolicy).NotTo(BeNil())
	})

	It("stable/"+policyNamespaceName+" should be created on managed cluster", func() {
		By("Checking policy-gatekeeper-operator on managed cluster in ns " + clusterNamespace)
		managedPolicy := utils.GetWithTimeout(
			clientManagedDynamic,
			common.GvrPolicy,
			userNamespace+"."+policyNamespaceName,
			clusterNamespace,
			true,
			defaultTimeoutSeconds,
		)
		Expect(managedPolicy).NotTo(BeNil())
	})

	It("stable/"+policyNamespaceName+" should be NonCompliant", func() {
		By("Checking if the status of the root policy is NonCompliant")
		Eventually(
			common.GetComplianceState(clientHubDynamic, userNamespace, policyNamespaceName, clusterNamespace),
			defaultTimeoutSeconds*2,
			1,
		).Should(Equal(policiesv1.NonCompliant))
	})

	It("Enforcing stable/"+policyNamespaceName, func() {
		By("Patching remediationAction = enforce on the root policy")
		_, err := clientHubDynamic.Resource(common.GvrPolicy).Namespace(userNamespace).Patch(
			context.TODO(),
			policyNamespaceName,
			k8stypes.JSONPatchType,
			[]byte(`[{"op": "replace", "path": "/spec/remediationAction", "value": "enforce"}]`),
			metav1.PatchOptions{},
		)
		Expect(err).To(BeNil())
	})

	It("stable/"+policyNamespaceName+" should be Compliant", func() {
		By("Checking if the status of the root policy is Compliant")
		Eventually(
			common.GetComplianceState(clientHubDynamic, userNamespace, policyNamespaceName, clusterNamespace),
			defaultTimeoutSeconds*2,
			1,
		).Should(Equal(policiesv1.Compliant))
	})

	It("The prod Namespace should exist", func() {
		By("Checking the prod namespace")
		Eventually(
			func() error {
				_, err := clientHub.CoreV1().Namespaces().Get(context.TODO(), "prod", metav1.GetOptions{})

				return err
			},
			defaultTimeoutSeconds*2,
			1,
		).Should(BeNil())
	})

	It("Cleans up", func() {
		_, err := utils.KubectlWithOutput(
			"delete", "-f", policyNamespaceURL, "-n", userNamespace, "--kubeconfig="+kubeconfigHub,
		)
		Expect(err).To(BeNil())

		err = clientHub.CoreV1().Namespaces().Delete(context.TODO(), "prod", metav1.DeleteOptions{})
		Expect(err).To(BeNil())
	})
})
