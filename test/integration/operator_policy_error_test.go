// Copyright (c) 2022 Red Hat, Inc.
// Copyright Contributors to the Open Cluster Management project

package integration

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	policiesv1 "open-cluster-management.io/governance-policy-propagator/api/v1"

	"github.com/stolostron/governance-policy-framework/test/common"
)

var _ = Describe("GRC: [P1][Sev1][policy-grc] Test operatorpolicy errors",
	Ordered, Label("BVT"), func() {
		const (
			policyPath            = "../resources/operator_policy/test-op-err-initial.yaml"
			correctPolicyPath     = "../resources/operator_policy/test-op-err-correct.yaml"
			policyName            = "test-op-err-initial"
			operatorPolicyName    = "test-op-err-initial"
			subscriptionNamespace = "grcqeoptest-ns-43568"
		)

		BeforeAll(func() {
			By("Create policy " + policyName + " which contains " + operatorPolicyName)
			common.DoCreatePolicyTest(policyPath, common.GvrOperatorPolicy)
		})

		AfterAll(func() {
			_, err := common.OcHub(
				"delete", "-f", policyPath,
				"-n", userNamespace, "--ignore-not-found=true",
			)
			Expect(err).ToNot(HaveOccurred())

			_, err = common.OcManaged(
				"delete", "ns", subscriptionNamespace, "--ignore-not-found=true",
			)
			Expect(err).ToNot(HaveOccurred())
		})

		It(operatorPolicyName+" should be NonCompliant due to invalid namespace", func() {
			By("Checking if the status of the operator policy is NonCompliant")
			Eventually(
				common.GetComplianceState(policyName),
				defaultTimeoutSeconds,
				1,
			).Should(Equal(policiesv1.NonCompliant))
			Eventually(
				common.GetOpPolicyCompMsg(operatorPolicyName),
				defaultTimeoutSeconds,
				1,
			).Should(ContainSubstring("the operator namespace ('grcqeoptest-notcreated') does not exist"))
		})

		It("Namespace "+subscriptionNamespace+" should be created", func() {
			_, err := common.OcManaged(
				"create", "namespace", subscriptionNamespace,
			)
			Expect(err).ToNot(HaveOccurred())
		})

		It(operatorPolicyName+" should be patched with correct policy", func() {
			_, err := common.OcHub(
				"apply", "-f", correctPolicyPath, "-n", userNamespace,
			)
			Expect(err).ToNot(HaveOccurred())
		})

		It(operatorPolicyName+" should be patched with incorrect source", func() {
			_, err := common.OcHub(
				"patch", "policies.policy.open-cluster-management.io", policyName,
				"-n", userNamespace, "--type=json", "-p", `[{
				"op":"replace", 
				"path":"/spec/policy-templates/0/objectDefinition/spec/subscription/source",
				"value":"redhat-operators-invalid"
			}]`)
			Expect(err).ToNot(HaveOccurred())
		})

		It(operatorPolicyName+" should be NonCompliant due to incorrect source", func() {
			By("Checking if the status of the operator policy is NonCompliant")
			Eventually(
				common.GetComplianceState(policyName),
				defaultTimeoutSeconds,
				1,
			).Should(Equal(policiesv1.NonCompliant))
			Eventually(
				common.GetOpPolicyCompMsg(operatorPolicyName),
				defaultTimeoutSeconds,
				1,
			).Should(ContainSubstring("CatalogSource 'redhat-operators-invalid' was not found"))
		})

		It(operatorPolicyName+" should be patched with correct policy", func() {
			_, err := common.OcHub(
				"apply", "-f", correctPolicyPath, "-n", userNamespace,
			)
			Expect(err).ToNot(HaveOccurred())
		})

		It(operatorPolicyName+" should be patched with incorrect installPlanApproval", func() {
			_, err := common.OcHub(
				"patch", "policies.policy.open-cluster-management.io", policyName,
				"-n", userNamespace, "--type=json", "-p", `[{
				"op":"replace", 
				"path":"/spec/policy-templates/0/objectDefinition/spec/subscription/installPlanApproval",
				"value":"Invalid"
			}]`)
			Expect(err).ToNot(HaveOccurred())
		})

		It(operatorPolicyName+" should be NonCompliant", func() {
			By("Checking if the status of the operator policy is NonCompliant")
			Eventually(
				common.GetComplianceState(policyName),
				defaultTimeoutSeconds,
				1,
			).Should(Equal(policiesv1.NonCompliant))
			Eventually(
				common.GetOpPolicyCompMsg(operatorPolicyName),
				defaultTimeoutSeconds,
				1,
			).Should(ContainSubstring("the policy spec.subscription.installPlanApproval ('Invalid') is invalid"))
		})
	})
