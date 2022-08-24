// Copyright (c) 2020 Red Hat, Inc.
// Copyright Contributors to the Open Cluster Management project

package e2e

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/stolostron/governance-policy-framework/test/common"
	policiesv1 "open-cluster-management.io/governance-policy-propagator/api/v1"
)

var _ = Describe("Test iam policy", func() {
	Describe("Test iam policy inform", Ordered, func() {
		const iamPolicyName string = "iam-policy"
		const iamPolicyYaml string = "../resources/iam_policy/iam-policy.yaml"
		It("should be created on managed cluster", func() {
			common.DoCreatePolicyTest(clientHubDynamic, clientManagedDynamic, iamPolicyYaml, common.GvrIamPolicy)
		})
		It("the policy should be compliant as there is no clusterrolebindings", func() {
			common.DoRootComplianceTest(clientHubDynamic, iamPolicyName, policiesv1.Compliant)
		})
		It("the policy should be noncompliant after creating clusterrolebindings", func() {
			By("Creating ../resources/iam_policy/clusterrolebinding.yaml")
			_, err := common.OcManaged("apply", "-f", "../resources/iam_policy/clusterrolebinding.yaml")
			Expect(err).To(BeNil())
			common.DoRootComplianceTest(clientHubDynamic, iamPolicyName, policiesv1.NonCompliant)
		})
		It("the policy should be compliant again after delete clusterrolebindings", func() {
			By("Deleting ../resources/iam_policy/clusterrolebinding.yaml")

			_, err := common.OcManaged(
				"delete", "-f", "../resources/iam_policy/clusterrolebinding.yaml",
				"--ignore-not-found",
			)
			Expect(err).To(BeNil())
			common.DoRootComplianceTest(clientHubDynamic, iamPolicyName, policiesv1.Compliant)
		})
		AfterAll(func() {
			common.DoCleanupPolicy(clientHubDynamic, clientManagedDynamic, iamPolicyYaml, common.GvrIamPolicy)
		})
	})
})
