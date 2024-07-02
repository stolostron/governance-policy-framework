// Copyright (c) 2022 Red Hat, Inc.
// Copyright Contributors to the Open Cluster Management project

package integration

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	policiesv1 "open-cluster-management.io/governance-policy-propagator/api/v1"

	"github.com/stolostron/governance-policy-framework/test/common"
)

var _ = Describe("GRC: [P1][Sev1][policy-grc] Test OperatorPolicy complianceConfig",
	Ordered, Label("BVT"), func() {
		const (
			policyPath             = "../resources/operator_policy/test-op-complianceconfig.yaml"
			policyPathModified     = "../resources/operator_policy/test-op-complianceconfig-modified.yaml"
			policyName             = "test-op-complianceconfig"
			operatorPolicyName     = "test-op-complianceconfig"
			subscriptionNamespace  = "grcqeoptest-ns-48381"
			policyPath1            = "../resources/operator_policy/test-op-complianceconfig-1.yaml"
			policyPath1Modified    = "../resources/operator_policy/test-op-complianceconfig-1-modified.yaml"
			policyName1            = "test-op-complianceconfig-1"
			operatorPolicyName1    = "test-op-complianceconfig-1"
			subscriptionNamespace1 = "grcqeoptest-ns-48382"
			operatorVersionInitial = "quay-operator.v3.8.14"
			operatorVersionRecent  = "quay-operator.v3.8.15" // This may need to be modified for newer versions
		)

		BeforeAll(func() {
			By("Create namespace " + subscriptionNamespace)
			_, err := common.OcManaged(
				"create", "namespace", subscriptionNamespace,
			)
			Expect(err).ToNot(HaveOccurred())

			By("Create namespace " + subscriptionNamespace1)
			_, err = common.OcManaged(
				"create", "namespace", subscriptionNamespace1,
			)
			Expect(err).ToNot(HaveOccurred())

			By("Create policy " + policyName + " which contains " + operatorPolicyName)
			common.DoCreatePolicyTest(policyPath, common.GvrOperatorPolicy)

			By("Create policy " + policyName1 + " which contains " + operatorPolicyName1)
			common.DoCreatePolicyTest(policyPath1, common.GvrOperatorPolicy)
		})

		AfterAll(func() {
			_, err := common.OcHub(
				"delete", "-f", policyPath,
				"-n", userNamespace, "--ignore-not-found=true",
			)
			Expect(err).ToNot(HaveOccurred())

			_, err = common.OcHub(
				"delete", "-f", policyPath1,
				"-n", userNamespace, "--ignore-not-found=true",
			)
			Expect(err).ToNot(HaveOccurred())

			_, err = common.OcManaged(
				"delete", "ns", subscriptionNamespace, "--ignore-not-found=true",
			)
			Expect(err).ToNot(HaveOccurred())

			_, err = common.OcManaged(
				"delete", "ns", subscriptionNamespace1, "--ignore-not-found=true",
			)
			Expect(err).ToNot(HaveOccurred())
		})

		// RHACM4K-48381 - Test the policy status is Noncompliant when the operator has an upgrade available after
		// set upgradesAvailable as Noncompliant in the policy

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
			By("Patching upgradesAvailable complianceConfig on the operator policy")
			_, err := common.OcHub("apply", "-f", policyPathModified, "-n", userNamespace)
			Expect(err).ToNot(HaveOccurred())
		})

		It(operatorPolicyName+" should become NonCompliant after modifications to complianceConfig", func() {
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
			).Should(ContainSubstring("InstallPlan to update to [" + operatorVersionRecent +
				"] is available for approval"))
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
			Eventually(
				common.GetOpPolicyCompMsg(operatorPolicyName),
				defaultTimeoutSeconds,
				1,
			).Should(ContainSubstring("ClusterServiceVersion (" + operatorVersionRecent +
				") - install strategy completed with no errors"))
		})

		// RHACM4K-48382 - Install operator via set all options for spec.complianceConfig as NonCompliant in the policy

		It(operatorPolicyName1+" should be Compliant and OperatorGroup and Subscription should be created", func() {
			By("Checking if the status of the operator policy is Compliant")
			Eventually(
				common.GetComplianceState(policyName1),
				defaultTimeoutSeconds,
				1,
			).Should(Equal(policiesv1.Compliant))
			By("Checking the status messages of the operator policy")
			Eventually(
				common.GetOpPolicyCompMsg(operatorPolicyName1),
				defaultTimeoutSeconds,
				1,
			).Should(SatisfyAll(
				ContainSubstring("the OperatorGroup matches what is required by the policy"),
				ContainSubstring("the Subscription matches what is required by the policy"),
				ContainSubstring("all operator Deployments have their minimum availability")))
		})

		It(operatorPolicyName1+"should have installed the intended operator "+operatorVersionInitial, func() {
			By("Checking status of the operator policy")
			Eventually(
				common.GetOpPolicyCompMsg(operatorPolicyName1),
				defaultTimeoutSeconds,
				1,
			).Should(ContainSubstring("ClusterServiceVersion (" + operatorVersionRecent +
				") - install strategy completed with no errors"))
		})

		It(operatorPolicyName+" should be patched with NonCompliant ComplianceConfig for each option", func() {
			// new policy specifies NonCompliant for each field of complianceConfig
			_, err := common.OcHub("apply", "-f", policyPath1Modified, "-n", userNamespace)
			Expect(err).ToNot(HaveOccurred())
		})

		It(operatorPolicyName1+" should remain Compliant", func() {
			By("Checking if the status of the operator policy is Compliant")
			Eventually(
				common.GetComplianceState(policyName1),
				defaultTimeoutSeconds,
				1,
			).Should(Equal(policiesv1.Compliant))
			By("Checking the status messages of the operator policy")
			Eventually(
				common.GetOpPolicyCompMsg(operatorPolicyName1),
				defaultTimeoutSeconds,
				1,
			).Should(SatisfyAll(
				ContainSubstring("the OperatorGroup matches what is required by the policy"),
				ContainSubstring("the Subscription matches what is required by the policy"),
				ContainSubstring("all operator Deployments have their minimum availability")))
		})
	})
