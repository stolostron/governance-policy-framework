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
			common.DoCreatePolicyTest(certPolicyYaml, common.GvrCertPolicy)
		})
		It("the policy should be compliant as there is no certificate", func() {
			common.DoRootComplianceTest(certPolicyName, policiesv1.Compliant)
		})
		It("the policy should be noncompliant after creating a certficate that expires", func() {
			By("Creating ../resources/cert_policy/issuer.yaml in ns default")
			_, err := common.OcManaged("apply", "-f", "../resources/cert_policy/issuer.yaml", "-n", "default")
			Expect(err).ToNot(HaveOccurred())
			By("Creating ../resources/cert_policy/certificate.yaml in ns default")
			_, err = common.OcManaged("apply", "-f", "../resources/cert_policy/certificate.yaml", "-n", "default")
			Expect(err).ToNot(HaveOccurred())

			common.DoRootComplianceTest(certPolicyName, policiesv1.NonCompliant)
		})
		It("the policy should be compliant after creating a certficate that doesn't expire", func() {
			By("Creating ../resources/cert_policy/certificate_compliant.yaml in ns default")
			_, err := common.OcManaged(
				"apply", "-f",
				"../resources/cert_policy/certificate_compliant.yaml",
				"-n", "default",
			)
			Expect(err).ToNot(HaveOccurred())
			common.DoRootComplianceTest(certPolicyName, policiesv1.Compliant)
		})
		It("the policy should be noncompliant after creating a certficate that expires "+
			"and then is compliant after a fix", func() {
			By("Creating ../resources/cert_policy/issuer.yaml in ns default")
			_, err := common.OcManaged("apply", "-f", "../resources/cert_policy/issuer.yaml", "-n", "default")
			Expect(err).ToNot(HaveOccurred())
			By("Creating ../resources/cert_policy/certificate.yaml in ns default")
			_, err = common.OcManaged("apply", "-f", "../resources/cert_policy/certificate.yaml", "-n", "default")
			Expect(err).ToNot(HaveOccurred())

			common.DoRootComplianceTest(certPolicyName, policiesv1.NonCompliant)
			By("Creating ../resources/cert_policy/certificate_compliant.yaml in ns default")
			_, err = common.OcManaged(
				"apply", "-f",
				"../resources/cert_policy/certificate_compliant.yaml",
				"-n", "default",
			)
			Expect(err).ToNot(HaveOccurred())
			common.DoRootComplianceTest(certPolicyName, policiesv1.Compliant)
		})
		It("the policy should be noncompliant after creating a CA certficate that expires", func() {
			By("Creating ../resources/cert_policy/issuer.yaml in ns default")
			_, err := common.OcManaged("apply", "-f", "../resources/cert_policy/issuer.yaml", "-n", "default")
			Expect(err).ToNot(HaveOccurred())
			By("Creating ../resources/cert_policy/certificate_expired-ca.yaml in ns default")
			_, err = common.OcManaged(
				"apply", "-f",
				"../resources/cert_policy/certificate_expired-ca.yaml",
				"-n", "default",
			)
			Expect(err).ToNot(HaveOccurred())
			common.DoRootComplianceTest(certPolicyName, policiesv1.NonCompliant)
		})
		It("the policy should be compliant after creating a certficate that doesn't expire after CA expired", func() {
			By("Creating ../resources/cert_policy/certificate_compliant.yaml in ns default")
			_, err := common.OcManaged(
				"apply", "-f",
				"../resources/cert_policy/certificate_compliant.yaml",
				"-n", "default",
			)
			Expect(err).ToNot(HaveOccurred())
			common.DoRootComplianceTest(certPolicyName, policiesv1.Compliant)
		})
		It("the policy should be noncompliant after creating a certficate that has too long duration", func() {
			By("Creating ../resources/cert_policy/issuer.yaml in ns default")
			_, err := common.OcManaged("apply", "-f", "../resources/cert_policy/issuer.yaml", "-n", "default")
			Expect(err).ToNot(HaveOccurred())
			By("Creating ../resources/cert_policy/certificate_long.yaml in ns default")
			_, err = common.OcManaged("apply", "-f", "../resources/cert_policy/certificate_long.yaml", "-n", "default")
			Expect(err).ToNot(HaveOccurred())
			common.DoRootComplianceTest(certPolicyName, policiesv1.NonCompliant)
		})
		It("the policy should be compliant after creating a certficate with an expected duration", func() {
			By("Creating ../resources/cert_policy/certificate_compliant.yaml in ns default")
			_, err := common.OcManaged(
				"apply", "-f",
				"../resources/cert_policy/certificate_compliant.yaml",
				"-n", "default",
			)
			Expect(err).ToNot(HaveOccurred())
			common.DoRootComplianceTest(certPolicyName, policiesv1.Compliant)
		})
		It("the policy should be noncompliant after creating a CA certficate that has too long duration", func() {
			By("Creating ../resources/cert_policy/issuer.yaml in ns default")
			_, err := common.OcManaged("apply", "-f", "../resources/cert_policy/issuer.yaml", "-n", "default")
			Expect(err).ToNot(HaveOccurred())
			By("Creating ../resources/cert_policy/certificate_long-ca.yaml in ns default")
			_, err = common.OcManaged(
				"apply", "-f",
				"../resources/cert_policy/certificate_long-ca.yaml",
				"-n", "default",
			)
			Expect(err).ToNot(HaveOccurred())
			common.DoRootComplianceTest(certPolicyName, policiesv1.NonCompliant)
		})
		It("the policy should be compliant after creating a certficate with an expected duration after CA", func() {
			By("Creating ../resources/cert_policy/certificate_compliant.yaml in ns default")
			_, err := common.OcManaged(
				"apply", "-f",
				"../resources/cert_policy/certificate_compliant.yaml",
				"-n", "default",
			)
			Expect(err).ToNot(HaveOccurred())
			common.DoRootComplianceTest(certPolicyName, policiesv1.Compliant)
		})
		It("the policy should be noncompliant after creating a certficate "+
			"that has a DNS entry that is not allowed", func() {
			By("Creating ../resources/cert_policy/issuer.yaml in ns default")
			_, err := common.OcManaged("apply", "-f", "../resources/cert_policy/issuer.yaml", "-n", "default")
			Expect(err).ToNot(HaveOccurred())
			By("Creating ../resources/cert_policy/certificate_allow-noncompliant.yaml in ns default")
			_, err = common.OcManaged(
				"apply", "-f",
				"../resources/cert_policy/certificate_allow-noncompliant.yaml",
				"-n", "default",
			)
			Expect(err).ToNot(HaveOccurred())
			common.DoRootComplianceTest(certPolicyName, policiesv1.NonCompliant)
		})
		It("the policy should be compliant after creating a certficate with allowed dns names", func() {
			By("Creating ../resources/cert_policy/certificate_compliant.yaml in ns default")
			_, err := common.OcManaged(
				"apply", "-f",
				"../resources/cert_policy/certificate_compliant.yaml",
				"-n", "default",
			)
			Expect(err).ToNot(HaveOccurred())
			common.DoRootComplianceTest(certPolicyName, policiesv1.Compliant)
		})
		It("the policy should be noncompliant after creating a certficate with a disallowed wildcard", func() {
			By("Creating ../resources/cert_policy/issuer.yaml in ns default")
			_, err := common.OcManaged("apply", "-f", "../resources/cert_policy/issuer.yaml", "-n", "default")
			Expect(err).ToNot(HaveOccurred())
			By("Creating ../resources/cert_policy/certificate_disallow-noncompliant.yaml in ns default")
			_, err = common.OcManaged(
				"apply", "-f",
				"../resources/cert_policy/certificate_disallow-noncompliant.yaml",
				"-n", "default",
			)
			Expect(err).ToNot(HaveOccurred())
			common.DoRootComplianceTest(certPolicyName, policiesv1.NonCompliant)
		})
		It("the policy should be compliant after creating a certficate with no dns names that are not allowed", func() {
			By("Creating ../resources/cert_policy/certificate_compliant.yaml in ns default")
			_, err := common.OcManaged(
				"apply", "-f",
				"../resources/cert_policy/certificate_compliant.yaml",
				"-n", "default",
			)
			Expect(err).ToNot(HaveOccurred())
			common.DoRootComplianceTest(certPolicyName, policiesv1.Compliant)
		})
		It("the messages from history should match", func() {
			By("the policy should have matched history after all these test")
			common.DoHistoryUpdatedTest(certPolicyName,
				"Compliant",
				"NonCompliant;  1 certificates defined SAN entries do not match pattern Allowed: "+
					".*.test.com Disallowed: [\\*]: default:cert-policy-secret",
				"Compliant",
				"NonCompliant;  1 certificates defined SAN entries do not match pattern Allowed: "+
					".*.test.com Disallowed: [\\*]: default:cert-policy-secret",
				"Compliant",
				"NonCompliant;  1 CA certificates exceed the maximum duration of 26280h0m0s: "+
					"default:cert-policy-secret",
				"Compliant",
				"NonCompliant;  1 certificates exceed the maximum duration of 9528h0m0s: "+
					"default:cert-policy-secret",
				"Compliant",
				"NonCompliant;  1 CA certificates expire in less than 45h0m0s: default:cert-policy-secret",
			)
		})
		It("the messages from history should not repeat", func() {
			By("Creating ../resources/cert_policy/certificate_disallow-noncompliant.yaml in ns default")
			_, err := common.OcManaged(
				"apply", "-f",
				"../resources/cert_policy/certificate_disallow-noncompliant.yaml",
				"-n", "default",
			)
			Expect(err).ToNot(HaveOccurred())
			By("the policy should not duplicate messages")
			Consistently(func() interface{} {
				msg := common.GetDuplicateHistoryMessage(certPolicyName)

				return msg
			}, 60, 10).Should(Equal(""))
		})
		AfterAll(func() {
			By("Deleting the resource, policy and events on managed cluster")

			common.DoCleanupPolicy(certPolicyYaml, common.GvrCertPolicy)

			By("Deleting ../resources/cert_policy/issuer.yaml in ns default")
			_, err := common.OcManaged(
				"delete", "-f", "../resources/cert_policy/issuer.yaml",
				"-n", "default", "--ignore-not-found",
			)
			Expect(err).ToNot(HaveOccurred())

			By("Deleting ../resources/cert_policy/certificate.yaml in ns default")
			_, err = common.OcManaged(
				"delete", "-f", "../resources/cert_policy/certificate.yaml",
				"-n", "default", "--ignore-not-found",
			)
			Expect(err).ToNot(HaveOccurred())

			By("Deleting cert-policy-secret")
			_, err = common.OcManaged(
				"delete", "secret",
				"cert-policy-secret", "-n", "default",
				"--ignore-not-found",
			)
			Expect(err).ToNot(HaveOccurred())

			_, err = common.OcHosting(
				"delete", "events", "-n", common.ClusterNamespace,
				"--field-selector=involvedObject.name="+certPolicyName,
				"--ignore-not-found",
			)
			Expect(err).ToNot(HaveOccurred())
			_, err = common.OcHosting(
				"delete", "events", "-n", common.ClusterNamespace,
				"--field-selector=involvedObject.name="+common.UserNamespace+"."+certPolicyName,
				"--ignore-not-found",
			)
			Expect(err).ToNot(HaveOccurred())
			_, err = common.OcHub(
				"delete", "events", "-n", common.ClusterNamespaceOnHub,
				"--field-selector=involvedObject.name="+common.UserNamespace+"."+certPolicyName,
				"--ignore-not-found",
			)
			ExpectWithOffset(1, err).ToNot(HaveOccurred())
		})
	})
})
