// Copyright (c) 2020 Red Hat, Inc.
// Copyright Contributors to the Open Cluster Management project

package e2e

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	policiesv1 "open-cluster-management.io/governance-policy-propagator/api/v1"

	"github.com/stolostron/governance-policy-framework/test/common"
)

/*
 * NOTE: With the current Compliant/NonCompliant validation checks it is important each test alternates the expected
 * result.  In other words, do not run 2 tests in a row that return NonCompliant, the second test will immediately pass
 * using the results of the first test.
 */
var _ = Describe("Test cert policy", func() {
	Describe("Test cert policy inform", Ordered, func() {
		const certPolicyName string = "cert-policy"
		const certPolicyYaml string = "../resources/cert_policy/cert-policy.yaml"
		It("should be created on managed cluster", func() {
			common.DoCreatePolicyTest(clientHubDynamic, clientManagedDynamic, certPolicyYaml, common.GvrCertPolicy)
		})
		It("the policy should be compliant as there is no certificate", func() {
			common.DoRootComplianceTest(clientHubDynamic, certPolicyName, policiesv1.Compliant)
		})
		It("the policy should be noncompliant after creating a certficate that expires", func() {
			By("Creating ../resources/cert_policy/issuer.yaml in ns default")
			_, err := common.OcManaged("apply", "-f", "../resources/cert_policy/issuer.yaml", "-n", "default")
			Expect(err).To(BeNil())
			By("Creating ../resources/cert_policy/certificate.yaml in ns default")
			_, err = common.OcManaged("apply", "-f", "../resources/cert_policy/certificate.yaml", "-n", "default")
			Expect(err).To(BeNil())

			common.DoRootComplianceTest(clientHubDynamic, certPolicyName, policiesv1.NonCompliant)
		})
		It("the policy should be compliant after creating a certficate that doesn't expire", func() {
			By("Creating ../resources/cert_policy/certificate_compliant.yaml in ns default")
			_, err := common.OcManaged(
				"apply", "-f",
				"../resources/cert_policy/certificate_compliant.yaml",
				"-n", "default",
			)
			Expect(err).To(BeNil())
			common.DoRootComplianceTest(clientHubDynamic, certPolicyName, policiesv1.Compliant)
		})
		It("the policy should be noncompliant after creating a certficate that expires "+
			"and then is compliant after a fix", func() {
			By("Creating ../resources/cert_policy/issuer.yaml in ns default")
			_, err := common.OcManaged("apply", "-f", "../resources/cert_policy/issuer.yaml", "-n", "default")
			Expect(err).To(BeNil())
			By("Creating ../resources/cert_policy/certificate.yaml in ns default")
			_, err = common.OcManaged("apply", "-f", "../resources/cert_policy/certificate.yaml", "-n", "default")
			Expect(err).To(BeNil())

			common.DoRootComplianceTest(clientHubDynamic, certPolicyName, policiesv1.NonCompliant)

			By("Creating ../resources/cert_policy/certificate_compliant.yaml in ns default")
			_, err = common.OcManaged(
				"apply", "-f",
				"../resources/cert_policy/certificate_compliant.yaml",
				"-n", "default",
			)
			Expect(err).To(BeNil())
			common.DoRootComplianceTest(clientHubDynamic, certPolicyName, policiesv1.Compliant)
		})
		It("the policy should be noncompliant after creating a CA certficate that expires", func() {
			By("Creating ../resources/cert_policy/issuer.yaml in ns default")
			_, err := common.OcManaged("apply", "-f", "../resources/cert_policy/issuer.yaml", "-n", "default")
			Expect(err).To(BeNil())
			By("Creating ../resources/cert_policy/certificate_expired-ca.yaml in ns default")
			_, err = common.OcManaged(
				"apply", "-f",
				"../resources/cert_policy/certificate_expired-ca.yaml",
				"-n", "default",
			)
			Expect(err).To(BeNil())
			common.DoRootComplianceTest(clientHubDynamic, certPolicyName, policiesv1.NonCompliant)
		})
		It("the policy should be compliant after creating a certficate that doesn't expire after CA expired", func() {
			By("Creating ../resources/cert_policy/certificate_compliant.yaml in ns default")
			_, err := common.OcManaged(
				"apply", "-f",
				"../resources/cert_policy/certificate_compliant.yaml",
				"-n", "default",
			)
			Expect(err).To(BeNil())
			common.DoRootComplianceTest(clientHubDynamic, certPolicyName, policiesv1.Compliant)
		})
		It("the policy should be noncompliant after creating a certficate that has too long duration", func() {
			By("Creating ../resources/cert_policy/issuer.yaml in ns default")
			_, err := common.OcManaged("apply", "-f", "../resources/cert_policy/issuer.yaml", "-n", "default")
			Expect(err).To(BeNil())
			By("Creating ../resources/cert_policy/certificate_long.yaml in ns default")
			_, err = common.OcManaged("apply", "-f", "../resources/cert_policy/certificate_long.yaml", "-n", "default")
			Expect(err).To(BeNil())
			common.DoRootComplianceTest(clientHubDynamic, certPolicyName, policiesv1.NonCompliant)
		})
		It("the policy should be compliant after creating a certficate with an expected duration", func() {
			By("Creating ../resources/cert_policy/certificate_compliant.yaml in ns default")
			_, err := common.OcManaged(
				"apply", "-f",
				"../resources/cert_policy/certificate_compliant.yaml",
				"-n", "default",
			)
			Expect(err).To(BeNil())
			common.DoRootComplianceTest(clientHubDynamic, certPolicyName, policiesv1.Compliant)
		})
		It("the policy should be noncompliant after creating a CA certficate that has too long duration", func() {
			By("Creating ../resources/cert_policy/issuer.yaml in ns default")
			_, err := common.OcManaged("apply", "-f", "../resources/cert_policy/issuer.yaml", "-n", "default")
			Expect(err).To(BeNil())
			By("Creating ../resources/cert_policy/certificate_long-ca.yaml in ns default")
			_, err = common.OcManaged(
				"apply", "-f",
				"../resources/cert_policy/certificate_long-ca.yaml",
				"-n", "default",
			)
			Expect(err).To(BeNil())
			common.DoRootComplianceTest(clientHubDynamic, certPolicyName, policiesv1.NonCompliant)
		})
		It("the policy should be compliant after creating a certficate with an expected duration after CA", func() {
			By("Creating ../resources/cert_policy/certificate_compliant.yaml in ns default")
			_, err := common.OcManaged(
				"apply", "-f",
				"../resources/cert_policy/certificate_compliant.yaml",
				"-n", "default",
			)
			Expect(err).To(BeNil())
			common.DoRootComplianceTest(clientHubDynamic, certPolicyName, policiesv1.Compliant)
		})
		It("the policy should be noncompliant after creating a certficate "+
			"that has a DNS entry that is not allowed", func() {
			By("Creating ../resources/cert_policy/issuer.yaml in ns default")
			_, err := common.OcManaged("apply", "-f", "../resources/cert_policy/issuer.yaml", "-n", "default")
			Expect(err).To(BeNil())
			By("Creating ../resources/cert_policy/certificate_allow-noncompliant.yaml in ns default")
			_, err = common.OcManaged(
				"apply", "-f",
				"../resources/cert_policy/certificate_allow-noncompliant.yaml",
				"-n", "default",
			)
			Expect(err).To(BeNil())
			common.DoRootComplianceTest(clientHubDynamic, certPolicyName, policiesv1.NonCompliant)
		})
		It("the policy should be compliant after creating a certficate with allowed dns names", func() {
			By("Creating ../resources/cert_policy/certificate_compliant.yaml in ns default")
			_, err := common.OcManaged(
				"apply", "-f",
				"../resources/cert_policy/certificate_compliant.yaml",
				"-n", "default",
			)
			Expect(err).To(BeNil())
			common.DoRootComplianceTest(clientHubDynamic, certPolicyName, policiesv1.Compliant)
		})
		It("the policy should be noncompliant after creating a certficate with a disallowed wildcard", func() {
			By("Creating ../resources/cert_policy/issuer.yaml in ns default")
			_, err := common.OcManaged("apply", "-f", "../resources/cert_policy/issuer.yaml", "-n", "default")
			Expect(err).To(BeNil())
			By("Creating ../resources/cert_policy/certificate_disallow-noncompliant.yaml in ns default")
			_, err = common.OcManaged(
				"apply", "-f",
				"../resources/cert_policy/certificate_disallow-noncompliant.yaml",
				"-n", "default",
			)
			Expect(err).To(BeNil())
			common.DoRootComplianceTest(clientHubDynamic, certPolicyName, policiesv1.NonCompliant)
		})
		It("the policy should be compliant after creating a certficate with no dns names that are not allowed", func() {
			By("Creating ../resources/cert_policy/certificate_compliant.yaml in ns default")
			_, err := common.OcManaged(
				"apply", "-f",
				"../resources/cert_policy/certificate_compliant.yaml",
				"-n", "default",
			)
			Expect(err).To(BeNil())
			common.DoRootComplianceTest(clientHubDynamic, certPolicyName, policiesv1.Compliant)
		})
		AfterAll(func() {
			common.DoCleanupPolicy(clientHubDynamic, clientManagedDynamic, certPolicyYaml, common.GvrCertPolicy)

			By("Deleting ../resources/cert_policy/issuer.yaml in ns default")
			_, err := common.OcManaged(
				"delete", "-f", "../resources/cert_policy/issuer.yaml",
				"-n", "default", "--ignore-not-found",
			)
			Expect(err).To(BeNil())

			By("Deleting ../resources/cert_policy/certificate.yaml in ns default")
			_, err = common.OcManaged(
				"delete", "-f", "../resources/cert_policy/certificate.yaml",
				"-n", "default", "--ignore-not-found",
			)
			Expect(err).To(BeNil())

			By("Deleting cert-policy-secret")
			_, err = common.OcManaged(
				"delete", "secret",
				"cert-policy-secret", "-n", "default",
				"--ignore-not-found",
			)
			Expect(err).To(BeNil())
		})
	})
})
