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

/*
 * NOTE: With the current Compliant/NonCompliant validation checks it is important each test alternates the expected
 * result.  In other words, do not run 2 tests in a row that return NonCompliant, the second test will immediately pass
 * using the results of the first test.
 */
var _ = Describe("Test cert policy", func() {
	Describe("Test cert policy inform", func() {
		const certPolicyName string = "cert-policy"
		const certPolicyYaml string = "../resources/cert_policy/cert-policy.yaml"
		It("should be created on managed cluster", func() {
			common.DoCreatePolicyTest(clientHubDynamic, clientManagedDynamic, certPolicyYaml)
		})
		It("the policy should be compliant as there is no certificate", func() {
			common.DoRootComplianceTest(clientHubDynamic, certPolicyName, policiesv1.Compliant)
		})
		It("the policy should be noncompliant after creating a certficate that expires", func() {
			By("Creating ../resources/cert_policy/issuer.yaml in ns default")
			common.OcManaged("apply", "-f", "../resources/cert_policy/issuer.yaml", "-n", "default")
			By("Creating ../resources/cert_policy/certificate.yaml in ns default")
			common.OcManaged("apply", "-f", "../resources/cert_policy/certificate.yaml", "-n", "default")

			common.DoRootComplianceTest(clientHubDynamic, certPolicyName, policiesv1.NonCompliant)
		})
		It("the policy should be compliant after creating a certficate that doesn't expire", func() {
			By("Creating ../resources/cert_policy/certificate_compliant.yaml in ns default")
			common.OcManaged("apply", "-f", "../resources/cert_policy/certificate_compliant.yaml", "-n", "default")

			common.DoRootComplianceTest(clientHubDynamic, certPolicyName, policiesv1.Compliant)
		})
		It("the policy should be noncompliant after creating a certficate that expires and then is compliant after a fix", func() {
			By("Creating ../resources/cert_policy/issuer.yaml in ns default")
			common.OcManaged("apply", "-f", "../resources/cert_policy/issuer.yaml", "-n", "default")
			By("Creating ../resources/cert_policy/certificate.yaml in ns default")
			common.OcManaged("apply", "-f", "../resources/cert_policy/certificate.yaml", "-n", "default")

			common.DoRootComplianceTest(clientHubDynamic, certPolicyName, policiesv1.NonCompliant)

			By("Creating ../resources/cert_policy/certificate_compliant.yaml in ns default")
			common.OcManaged("apply", "-f", "../resources/cert_policy/certificate_compliant.yaml", "-n", "default")

			common.DoRootComplianceTest(clientHubDynamic, certPolicyName, policiesv1.Compliant)
		})
		It("the policy should be noncompliant after creating a CA certficate that expires", func() {
			By("Creating ../resources/cert_policy/issuer.yaml in ns default")
			common.OcManaged("apply", "-f", "../resources/cert_policy/issuer.yaml", "-n", "default")
			By("Creating ../resources/cert_policy/certificate_expired-ca.yaml in ns default")
			common.OcManaged("apply", "-f", "../resources/cert_policy/certificate_expired-ca.yaml", "-n", "default")

			common.DoRootComplianceTest(clientHubDynamic, certPolicyName, policiesv1.NonCompliant)
		})
		It("the policy should be compliant after creating a certficate that doesn't expire after CA expired", func() {
			By("Creating ../resources/cert_policy/certificate_compliant.yaml in ns default")
			common.OcManaged("apply", "-f", "../resources/cert_policy/certificate_compliant.yaml", "-n", "default")

			common.DoRootComplianceTest(clientHubDynamic, certPolicyName, policiesv1.Compliant)
		})
		It("the policy should be noncompliant after creating a certficate that has too long duration", func() {
			By("Creating ../resources/cert_policy/issuer.yaml in ns default")
			common.OcManaged("apply", "-f", "../resources/cert_policy/issuer.yaml", "-n", "default")
			By("Creating ../resources/cert_policy/certificate_long.yaml in ns default")
			common.OcManaged("apply", "-f", "../resources/cert_policy/certificate_long.yaml", "-n", "default")

			common.DoRootComplianceTest(clientHubDynamic, certPolicyName, policiesv1.NonCompliant)
		})
		It("the policy should be compliant after creating a certficate with an expected duration", func() {
			By("Creating ../resources/cert_policy/certificate_compliant.yaml in ns default")
			common.OcManaged("apply", "-f", "../resources/cert_policy/certificate_compliant.yaml", "-n", "default")

			common.DoRootComplianceTest(clientHubDynamic, certPolicyName, policiesv1.Compliant)
		})
		It("the policy should be noncompliant after creating a CA certficate that has too long duration", func() {
			By("Creating ../resources/cert_policy/issuer.yaml in ns default")
			common.OcManaged("apply", "-f", "../resources/cert_policy/issuer.yaml", "-n", "default")
			By("Creating ../resources/cert_policy/certificate_long-ca.yaml in ns default")
			common.OcManaged("apply", "-f", "../resources/cert_policy/certificate_long-ca.yaml", "-n", "default")

			common.DoRootComplianceTest(clientHubDynamic, certPolicyName, policiesv1.NonCompliant)
		})
		It("the policy should be compliant after creating a certficate with an expected duration after CA", func() {
			By("Creating ../resources/cert_policy/certificate_compliant.yaml in ns default")
			common.OcManaged("apply", "-f", "../resources/cert_policy/certificate_compliant.yaml", "-n", "default")

			common.DoRootComplianceTest(clientHubDynamic, certPolicyName, policiesv1.Compliant)
		})
		It("the policy should be noncompliant after creating a certficate that has a DNS entry that is not allowed", func() {
			By("Creating ../resources/cert_policy/issuer.yaml in ns default")
			common.OcManaged("apply", "-f", "../resources/cert_policy/issuer.yaml", "-n", "default")
			By("Creating ../resources/cert_policy/certificate_allow-noncompliant.yaml in ns default")
			common.OcManaged("apply", "-f", "../resources/cert_policy/certificate_allow-noncompliant.yaml", "-n", "default")

			common.DoRootComplianceTest(clientHubDynamic, certPolicyName, policiesv1.NonCompliant)
		})
		It("the policy should be compliant after creating a certficate with allowed dns names", func() {
			By("Creating ../resources/cert_policy/certificate_compliant.yaml in ns default")
			common.OcManaged("apply", "-f", "../resources/cert_policy/certificate_compliant.yaml", "-n", "default")

			common.DoRootComplianceTest(clientHubDynamic, certPolicyName, policiesv1.Compliant)
		})
		It("the policy should be noncompliant after creating a certficate with a disallowed wildcard", func() {
			By("Creating ../resources/cert_policy/issuer.yaml in ns default")
			common.OcManaged("apply", "-f", "../resources/cert_policy/issuer.yaml", "-n", "default")
			By("Creating ../resources/cert_policy/certificate_disallow-noncompliant.yaml in ns default")
			common.OcManaged("apply", "-f", "../resources/cert_policy/certificate_disallow-noncompliant.yaml", "-n", "default")

			common.DoRootComplianceTest(clientHubDynamic, certPolicyName, policiesv1.NonCompliant)
		})
		It("the policy should be compliant after creating a certficate with no dns names that are not allowed", func() {
			By("Creating ../resources/cert_policy/certificate_compliant.yaml in ns default")
			common.OcManaged("apply", "-f", "../resources/cert_policy/certificate_compliant.yaml", "-n", "default")

			common.DoRootComplianceTest(clientHubDynamic, certPolicyName, policiesv1.Compliant)
		})
		It("should clean up", func() {
			By("Deleting " + certPolicyYaml)
			common.OcHub("delete", "-f", certPolicyYaml, "-n", userNamespace)
			By("Checking if there is any policy left")
			utils.ListWithTimeout(clientHubDynamic, common.GvrPolicy, metav1.ListOptions{}, 0, true, defaultTimeoutSeconds)
			utils.ListWithTimeout(clientManagedDynamic, common.GvrPolicy, metav1.ListOptions{}, 0, true, defaultTimeoutSeconds)
			By("Checking if there is any cert policy left")
			utils.ListWithTimeout(clientManagedDynamic, common.GvrCertPolicy, metav1.ListOptions{}, 0, true, defaultTimeoutSeconds)
			By("Deleting ../resources/cert_policy/issuer.yaml in ns default")
			common.OcManaged("delete", "-f", "../resources/cert_policy/issuer.yaml", "-n", "default")
			By("Deleting ../resources/cert_policy/certificate.yaml in ns default")
			common.OcManaged("delete", "-f", "../resources/cert_policy/certificate.yaml", "-n", "default")
			By("Deleting cert-policy-secret")
			common.OcManaged("delete", "secret", "cert-policy-secret", "-n", "default")
		})
	})
})
