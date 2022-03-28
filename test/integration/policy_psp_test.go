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

var _ = Describe("GRC: [P1][Sev1][policy-grc] Test the policy-psp policy", Label("policy-collection", "stable"), func() {
	const (
		rootPolicyName   = "policy-podsecuritypolicy"
		rootPolicyURL    = policyCollectSCURL + "policy-psp.yaml"
		rootPolicyNSName = "policy-psp"
		pspName          = "sample-restricted-psp"
	)

	It("stable/"+rootPolicyName+" should be created on the hub cluster", func() {
		By("Creating " + rootPolicyName + " on the hub cluster")
		_, err := utils.KubectlWithOutput(
			"apply",
			"-f",
			rootPolicyURL,
			"-n",
			userNamespace,
			"--kubeconfig="+kubeconfigHub,
		)
		Expect(err).To(BeNil())

		By("Checking " + rootPolicyName + " exists on the hub cluster in ns " + userNamespace)
		rootPolicy := utils.GetWithTimeout(
			clientHubDynamic,
			common.GvrPolicy,
			rootPolicyName,
			userNamespace,
			true,
			defaultTimeoutSeconds,
		)
		Expect(rootPolicy).NotTo(BeNil())
	})

	It("stable/"+rootPolicyName+" should be created on the managed cluster", func() {
		By("Patching placement rule placement-" + rootPolicyName)
		err := common.PatchPlacementRule(
			userNamespace,
			"placement-"+rootPolicyName,
			clusterNamespace,
			kubeconfigHub,
		)
		Expect(err).To(BeNil())

		By("Checking " + rootPolicyName + " on the managed cluster in ns " + clusterNamespace)
		managedPolicy := utils.GetWithTimeout(
			clientManagedDynamic,
			common.GvrPolicy,
			userNamespace+"."+rootPolicyName,
			clusterNamespace,
			true,
			defaultTimeoutSeconds,
		)
		Expect(managedPolicy).NotTo(BeNil())
	})

	It("stable/"+rootPolicyName+" should be NonCompliant", func() {
		By("Checking the status of the root policy " + rootPolicyName + " is NonCompliant")
		Eventually(
			common.GetComplianceState(
				clientHubDynamic,
				userNamespace,
				rootPolicyName,
				clusterNamespace,
			),
			defaultTimeoutSeconds*2,
			1,
		).Should(Equal(policiesv1.NonCompliant))
	})

	It("Enforcing stable/"+rootPolicyName, func() {
		By("Enforcing the root policy " + rootPolicyName + " to make it compliant")
		_, err := clientHubDynamic.Resource(common.GvrPolicy).Namespace(userNamespace).Patch(
			context.TODO(),
			rootPolicyName,
			k8stypes.JSONPatchType,
			[]byte(`[{"op": "replace", "path": "/spec/remediationAction", "value": "enforce"}]`),
			metav1.PatchOptions{},
		)
		Expect(err).To(BeNil())
	})

	It("stable/"+rootPolicyName+" should be Compliant", func() {
		By("Checking if the status of the root policy " + rootPolicyName + " is Compliant")
		Eventually(
			common.GetComplianceState(
				clientHubDynamic,
				userNamespace,
				rootPolicyName,
				clusterNamespace,
			),
			defaultTimeoutSeconds*4,
			1,
		).Should(Equal(policiesv1.Compliant))
	})

	It("The PodSecurityPolicy "+pspName+" should exist on the managed cluster", func() {
		By("Checking the PodSecurityPolicy " + pspName + " on the managed cluster")
		Eventually(
			func() error {
				_, err := clientManaged.PolicyV1beta1().PodSecurityPolicies().Get(
					context.TODO(), pspName, metav1.GetOptions{},
				)

				return err
			},
			defaultTimeoutSeconds*2,
			1,
		).Should(BeNil())
	})

	It("Cleans up", func() {
		By("Deleting the PodSecurityPolicy " + rootPolicyName + " on the hub cluster")
		_, err := utils.KubectlWithOutput(
			"delete",
			"-f",
			rootPolicyURL,
			"-n",
			userNamespace,
			"--kubeconfig="+kubeconfigHub,
		)
		Expect(err).To(BeNil())

		By("Deleting the PodSecurityPolicy " + pspName + " on the managed cluster")
		err = clientManaged.PolicyV1beta1().PodSecurityPolicies().Delete(
			context.TODO(),
			pspName,
			metav1.DeleteOptions{},
		)
		Expect(err).To(BeNil())
	})
})
