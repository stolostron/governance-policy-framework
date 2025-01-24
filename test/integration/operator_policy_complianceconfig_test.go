// Copyright (c) 2022 Red Hat, Inc.
// Copyright Contributors to the Open Cluster Management project

package integration

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	policiesv1 "open-cluster-management.io/governance-policy-propagator/api/v1"

	"github.com/stolostron/governance-policy-framework/test/common"
)

var _ = Describe("RHACM4K-48381 GRC: [P1][Sev1][policy-grc] Test OperatorPolicy complianceConfig",
	Ordered, func() {
		const (
			policyPath             = "../resources/operator_policy/test-op-complianceconfig.yaml"
			policyPathModified     = "../resources/operator_policy/test-op-complianceconfig-modified.yaml"
			policyName             = "test-op-complianceconfig"
			operatorPolicyName     = "test-op-complianceconfig"
			subscriptionNamespace  = "grcqeoptest-ns-48381"
			operatorVersionInitial = "quay-operator.v3.8.14"
		)

		BeforeAll(func(ctx SpecContext) {
			By("Create policy " + policyName + " which contains " + operatorPolicyName)
			common.DoCreatePolicyTest(ctx, policyPath, common.GvrOperatorPolicy)
		})

		AfterAll(func() {
			_, err := common.OcHub(
				"delete", "-f", policyPathModified,
				"-n", userNamespace, "--ignore-not-found=true",
			)
			Expect(err).ToNot(HaveOccurred())

			_, err = common.OcManaged(
				"delete", "ns", subscriptionNamespace, "--ignore-not-found=true",
			)
			Expect(err).ToNot(HaveOccurred())
		})

		It(operatorPolicyName+" should be Compliant and OperatorGroup and Subscription should be created", func() {
			By("Checking if the status of the operator policy is Compliant")
			Eventually(
				common.GetComplianceState(policyName),
				defaultTimeoutSeconds*2,
				1,
			).Should(Equal(policiesv1.Compliant))

			By("Checking the status messages of the operator policy")
			Eventually(
				common.GetOpPolicyCompMsg(operatorPolicyName),
				defaultTimeoutSeconds,
				1,
			).Should(SatisfyAll(
				ContainSubstring("the OperatorGroup matches what is required by the policy"),
				ContainSubstring("the Subscription matches what is required by the policy"),
				ContainSubstring("all operator Deployments have their minimum availability")))
		})

		It(operatorPolicyName+" should have installed the intended operator "+operatorVersionInitial, func() {
			By("Checking status of the operator policy")
			Eventually(
				common.GetOpPolicyCompMsg(operatorPolicyName),
				defaultTimeoutSeconds,
				1,
			).Should(ContainSubstring("ClusterServiceVersion (" + operatorVersionInitial +
				") - install strategy completed with no errors"))
		})

		It(operatorPolicyName+" should be modified to report NonCompliance when upgrades are available", func() {
			// new policy specifies NonCompliant for upgradesAvailable in complianceConfig
			By("Patching upgradesAvailable complianceConfig on the operator policy to NonCompliant")
			_, err := common.OcHub("apply", "-f", policyPathModified, "-n", userNamespace)
			Expect(err).ToNot(HaveOccurred())

			By("Checking if the status of the operator policy is NonCompliant")
			Eventually(
				common.GetComplianceState(policyName),
				defaultTimeoutSeconds,
				1,
			).Should(Equal(policiesv1.NonCompliant))

			By("Checking the status messages of the operator policy")
			Eventually(
				common.GetOpPolicyCompMsg(operatorPolicyName),
				defaultTimeoutSeconds,
				1,
			).Should(MatchRegexp(`.*InstallPlan to update to \[quay\-operator.v[0-9]+.[0-9]+.[0-9]+]` +
				` is available for approval.*`))
		})

		It(operatorPolicyName+" should be patched with Automatic upgradeApproval", func() {
			_, err := common.OcHub(
				"patch", "policies.policy.open-cluster-management.io", policyName,
				"-n", userNamespace, "--type=json", "-p", `[{
				"op":"replace", 
				"path":"/spec/policy-templates/0/objectDefinition/spec/upgradeApproval",
				"value":"Automatic"
			}]`)
			Expect(err).ToNot(HaveOccurred())
		})

		It(operatorPolicyName+" should become Compliant after modifications to upgradeApproval", func() {
			By("Checking if the status of the operator policy is Compliant")
			Eventually(
				common.GetComplianceState(policyName),
				defaultTimeoutSeconds,
				1,
			).Should(Equal(policiesv1.Compliant))

			By("Checking the status messages of the operator policy")
			Eventually(
				common.GetOpPolicyCompMsg(operatorPolicyName),
				defaultTimeoutSeconds,
				1,
			).Should(SatisfyAll(
				MatchRegexp(`.*ClusterServiceVersion \(quay\-operator.v[0-9]+.[0-9]+.[0-9]+\)`+
					` \- install strategy completed with no errors.*`),
				Not(ContainSubstring(operatorVersionInitial)),
			))
		})
	})

var _ = Describe("RHACM4K-48382 GRC: [P1][Sev1][policy-grc] Test OperatorPolicy complianceConfig",
	Ordered, func() {
		const (
			policyPath            = "../resources/operator_policy/test-op-complianceconfig-1.yaml"
			policyPathModified    = "../resources/operator_policy/test-op-complianceconfig-1-modified.yaml"
			policyName            = "test-op-complianceconfig-1"
			operatorPolicyName    = "test-op-complianceconfig-1"
			subscriptionNamespace = "grcqeoptest-ns-48382"
		)

		BeforeAll(func(ctx SpecContext) {
			By("Create policy " + policyName + " which contains " + operatorPolicyName)
			common.DoCreatePolicyTest(ctx, policyPath, common.GvrOperatorPolicy)
		})

		AfterAll(func() {
			_, err := common.OcHub(
				"delete", "-f", policyPathModified,
				"-n", userNamespace, "--ignore-not-found=true",
			)
			Expect(err).ToNot(HaveOccurred())

			_, err = common.OcManaged(
				"delete", "ns", subscriptionNamespace, "--ignore-not-found=true",
			)
			Expect(err).ToNot(HaveOccurred())
		})

		It(operatorPolicyName+" should be Compliant and OperatorGroup and Subscription should be created", func() {
			By("Checking if the status of the operator policy is Compliant")
			Eventually(
				common.GetComplianceState(policyName),
				defaultTimeoutSeconds*2,
				1,
			).Should(Equal(policiesv1.Compliant))

			By("Checking the status messages of the operator policy")
			Eventually(
				common.GetOpPolicyCompMsg(operatorPolicyName),
				defaultTimeoutSeconds,
				1,
			).Should(SatisfyAll(
				ContainSubstring("the OperatorGroup matches what is required by the policy"),
				ContainSubstring("the Subscription matches what is required by the policy"),
				ContainSubstring("all operator Deployments have their minimum availability")))
		})

		It(operatorPolicyName+"should have installed the intended operator", func() {
			By("Checking status of the operator policy")
			Eventually(
				common.GetOpPolicyCompMsg(operatorPolicyName),
				defaultTimeoutSeconds,
				1,
			).Should(MatchRegexp(`.*ClusterServiceVersion \(quay\-operator.v[0-9]+.[0-9]+.[0-9]+\)` +
				` \- install strategy completed with no errors.*`))
		})

		It(operatorPolicyName+" should be patched with NonCompliant ComplianceConfig for each option", func() {
			// new policy specifies NonCompliant for each field of complianceConfig
			By("Modifying complianceConfig to NonCompliant for each field")
			_, err := common.OcHub("apply", "-f", policyPathModified, "-n", userNamespace)
			Expect(err).ToNot(HaveOccurred())

			By("Checking if the status of the operator policy remains Compliant")
			Eventually(
				common.GetComplianceState(policyName),
				defaultTimeoutSeconds,
				1,
			).Should(Equal(policiesv1.Compliant))

			By("Checking the status messages of the operator policy")
			Eventually(
				common.GetOpPolicyCompMsg(operatorPolicyName),
				defaultTimeoutSeconds,
				1,
			).Should(SatisfyAll(
				ContainSubstring("the OperatorGroup matches what is required by the policy"),
				ContainSubstring("the Subscription matches what is required by the policy"),
				ContainSubstring("all operator Deployments have their minimum availability")))
		})
	})
