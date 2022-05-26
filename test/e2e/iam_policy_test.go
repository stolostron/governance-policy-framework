// Copyright (c) 2020 Red Hat, Inc.
// Copyright Contributors to the Open Cluster Management project

package e2e

import (
	. "github.com/onsi/ginkgo/v2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	policiesv1 "open-cluster-management.io/governance-policy-propagator/api/v1"
	"open-cluster-management.io/governance-policy-propagator/test/utils"

	"github.com/stolostron/governance-policy-framework/test/common"
)

var _ = Describe("Test iam policy", func() {
	Describe("Test iam policy inform", func() {
		const iamPolicyName string = "iam-policy"
		const iamPolicyYaml string = "../resources/iam_policy/iam-policy.yaml"
		It("should be created on managed cluster", func() {
			common.DoCreatePolicyTest(clientHubDynamic, clientManagedDynamic, iamPolicyYaml)
		})
		It("the policy should be compliant as there is no clusterrolebindings", func() {
			common.DoRootComplianceTest(clientHubDynamic, iamPolicyName, policiesv1.Compliant)
		})
		It("the policy should be noncompliant after creating clusterrolebindings", func() {
			By("Creating ../resources/iam_policy/clusterrolebinding.yaml")
			common.OcManaged("apply", "-f", "../resources/iam_policy/clusterrolebinding.yaml")

			common.DoRootComplianceTest(clientHubDynamic, iamPolicyName, policiesv1.NonCompliant)
		})
		It("the policy should be compliant again after delete clusterrolebindings", func() {
			By("Deleting ../resources/iam_policy/clusterrolebinding.yaml")
			common.OcManaged("delete", "-f", "../resources/iam_policy/clusterrolebinding.yaml")

			common.DoRootComplianceTest(clientHubDynamic, iamPolicyName, policiesv1.Compliant)
		})
		It("should clean up", func() {
			By("Deleting " + iamPolicyYaml)
			common.OcHub("delete", "-f", iamPolicyYaml, "-n", userNamespace)
			By("Checking if there is any policy left")
			utils.ListWithTimeout(clientHubDynamic, common.GvrPolicy, metav1.ListOptions{}, 0, true, defaultTimeoutSeconds)
			utils.ListWithTimeout(clientManagedDynamic, common.GvrPolicy, metav1.ListOptions{}, 0, true, defaultTimeoutSeconds)
			By("Checking if there is any iam policy left")
			utils.ListWithTimeout(clientManagedDynamic, common.GvrIamPolicy, metav1.ListOptions{}, 0, true, defaultTimeoutSeconds)
		})
	})
})
