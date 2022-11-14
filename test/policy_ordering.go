// Copyright (c) 2022 Red Hat, Inc.
// Copyright Contributors to the Open Cluster Management project

package test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/stolostron/governance-policy-framework/test/common"
	policiesv1 "open-cluster-management.io/governance-policy-propagator/api/v1"
	"open-cluster-management.io/governance-policy-propagator/test/utils"
)

func PolicyOrdering(labels ...string) bool {
	const (
		initialPolicyYaml      = "../resources/policy_ordering/dep-policy-initial.yaml"
		initialPolicyName      = "dep-policy-initial"
		policyWithDepYaml      = "../resources/policy_ordering/dep-policy-deppol.yaml"
		policyWithDepName      = "dep-policy-deppol"
		policyWithExtraDepYaml = "../resources/policy_ordering/dep-policy-extradepconfig.yaml"
		policyWithExtraDepName = "dep-policy-extradepconfig"
		ignorePendingYaml      = "../resources/policy_ordering/dep-policy-ignorepending.yaml"
		ignorePendingName      = "dep-policy-ignorepending"
	)

	cleanup := func() {
		By("Cleaning up")
		DoCleanupPolicy(initialPolicyYaml)
		DoCleanupPolicy(policyWithDepYaml)
		DoCleanupPolicy(policyWithExtraDepYaml)
		DoCleanupPolicy(ignorePendingYaml)

		configmapNames := []string{
			"dep-initial-cfgmap",
			"deppol-cfgmap",
			"dep-extradepconfig-cfgmap",
			"dep-ignorepending-cfgmap",
			"dep-ignorepending-extra-cfgmap",
		}

		for _, name := range configmapNames {
			_, err := OcManaged("delete", "configmap", name, "-n=default", "--ignore-not-found")
			Expect(err).To(BeNil())
		}
	}

	Describe("GRC: [P1][Sev1][policy-grc] Test policy ordering", Ordered, Label(labels...), func() {
		Describe("Ordering via a dependency on a Policy", func() {
			BeforeAll(func() {
				By("Creating the initial policy to use as a dependency")
				DoCreatePolicyTest(initialPolicyYaml, GvrConfigurationPolicy)
				DoRootComplianceTest(initialPolicyName, policiesv1.NonCompliant)

				By("Creating the policy that depends on the initial policy")
				DoCreatePolicyTest(policyWithDepYaml)
			})
			It("Should be pending while the initial policy is non-compliant", func() {
				DoRootComplianceTest(policyWithDepName, policiesv1.Pending)
			})
			It("Should become active after the initial policy is enforced", func() {
				EnforcePolicy(initialPolicyName, GvrConfigurationPolicy)
				DoRootComplianceTest(initialPolicyName, policiesv1.Compliant)
				DoRootComplianceTest(policyWithDepName, policiesv1.NonCompliant)
			})
			It("Should become pending again when the initial policy is deleted", func() {
				DoCleanupPolicy(initialPolicyYaml)
				DoRootComplianceTest(policyWithDepName, policiesv1.Pending)
			})
			AfterAll(cleanup)
		})
		Describe("Ordering via an extraDependency on a ConfigurationPolicy", func() {
			BeforeAll(func() {
				By("Creating the initial policy to use as a dependency")
				DoCreatePolicyTest(initialPolicyYaml, GvrConfigurationPolicy)
				DoRootComplianceTest(initialPolicyName, policiesv1.NonCompliant)

				By("Creating the policy that depends on the initial policy")
				DoCreatePolicyTest(policyWithExtraDepYaml)
			})
			It("Should be pending while the initial policy is non-compliant", func() {
				DoRootComplianceTest(policyWithExtraDepName, policiesv1.Pending)
			})
			It("Should become active after the initial policy is enforced", func() {
				EnforcePolicy(initialPolicyName, GvrConfigurationPolicy)
				DoRootComplianceTest(initialPolicyName, policiesv1.Compliant)
				DoRootComplianceTest(policyWithExtraDepName, policiesv1.NonCompliant)
			})
			It("Should become pending again when the initial policy is deleted", func() {
				DoCleanupPolicy(initialPolicyYaml)
				DoRootComplianceTest(policyWithExtraDepName, policiesv1.Pending)
			})
			AfterAll(cleanup)
		})
		Describe("IgnorePending should allow policies to be compliant when one template is pending", func() {
			BeforeAll(func() {
				By("Creating the initial policy to use as a dependency")
				DoCreatePolicyTest(initialPolicyYaml, GvrConfigurationPolicy)
				DoRootComplianceTest(initialPolicyName, policiesv1.NonCompliant)

				By("Creating the policy that depends on the initial policy")
				DoCreatePolicyTest(ignorePendingYaml)
			})
			It("Should be compliant overall", func() {
				DoRootComplianceTest(ignorePendingName, policiesv1.Compliant)
			})
			It("The pending template should not be created", func() {
				utils.GetWithTimeout(
					ClientManagedDynamic, GvrConfigurationPolicy, "dep-policy-ignorepending-extra",
					ClusterNamespace, false, DefaultTimeoutSeconds,
				)
			})
			AfterAll(cleanup)
		})
	})

	return true
}
