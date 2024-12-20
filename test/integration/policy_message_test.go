// Copyright (c) 2022 Red Hat, Inc.
// Copyright Contributors to the Open Cluster Management project

package integration

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	policiesv1 "open-cluster-management.io/governance-policy-propagator/api/v1"

	"github.com/stolostron/governance-policy-framework/test/common"
)

var _ = Describe("GRC: [P1][Sev1][policy-grc] Test Consolidate compliance messages",
	Ordered, Label("SVT"), func() {
		const resourcesPath = "../resources/policy_message/"

		Describe("Consolidate compliance messages using NamespaceSelector", func() {
			const (
				prereqYaml = resourcesPath + "namespace-selector-prereq" + ".yaml"
				policyName = "policy-message-namespace-selector"
				policyYaml = resourcesPath + policyName + ".yaml"
				targetNs   = "policy-message-ns-1"
			)
			BeforeAll(func() {
				By("Create namespace for the test")
				_, err := common.OcManaged("create", "ns", targetNs)
				Expect(err).ToNot(HaveOccurred())

				By("Applying prerequisites")
				_, err = common.OcManaged("apply", "-n", targetNs, "-f", prereqYaml)
				Expect(err).ToNot(HaveOccurred())

				By("Create policy " + policyName)
				_, err = common.OcHub("apply", "-f", policyYaml, "-n", userNamespace)
				Expect(err).ToNot(HaveOccurred())
			})

			AfterAll(func() {
				_, err := common.OcHub("delete", "-n", userNamespace, "-f", policyYaml, "--ignore-not-found")
				Expect(err).ToNot(HaveOccurred())

				_, err = common.OcManaged("delete", "-n", targetNs, "-f", prereqYaml, "--ignore-not-found")
				Expect(err).ToNot(HaveOccurred())

				_, err = common.OcManaged("delete", "ns", targetNs, "--ignore-not-found")
				Expect(err).ToNot(HaveOccurred())
			})

			It("Should verify that the policy message is a consolidated message", func() {
				By("Checking if the status of the root policy is Compliant")
				Eventually(
					common.GetComplianceState(policyName),
					defaultTimeoutSeconds,
					1,
				).Should(Equal(policiesv1.Compliant))

				Eventually(
					common.GetLatestStatusMessage(policyName, 0),
					defaultTimeoutSeconds, 1,
				).Should(Equal("Compliant; notification - pods " +
					"[policy-message-obj-selector-1, policy-message-obj-selector-2, policy-message-obj-selector-3] " +
					"found as specified in namespace " + targetNs))
			})
		})

		Describe("Consolidate compliance messages with ObjectSelector", func() {
			const (
				prereqYaml = resourcesPath + "obj-selector-prereq" + ".yaml"
				policyName = "policy-message-obj-selector"
				policyYaml = resourcesPath + policyName + ".yaml"
				targetNs   = "policy-message-ns"
			)
			BeforeAll(func() {
				By("Create namespace for the test")
				_, err := common.OcManaged("create", "ns", targetNs)
				Expect(err).ToNot(HaveOccurred())

				By("Applying prerequisites")
				_, err = common.OcManaged("apply", "-n", targetNs, "-f", prereqYaml)
				Expect(err).ToNot(HaveOccurred())

				By("Create policy " + policyName)
				_, err = common.OcHub("apply", "-f", policyYaml, "-n", userNamespace)
				Expect(err).ToNot(HaveOccurred())
			})

			AfterAll(func() {
				_, err := common.OcHub("delete", "-n", userNamespace, "-f", policyYaml, "--ignore-not-found")
				Expect(err).ToNot(HaveOccurred())

				_, err = common.OcManaged("delete", "-n", targetNs, "-f", prereqYaml, "--ignore-not-found")
				Expect(err).ToNot(HaveOccurred())

				_, err = common.OcManaged("delete", "ns", targetNs, "--ignore-not-found")
				Expect(err).ToNot(HaveOccurred())
			})

			It("Should verify that the policy message is a consolidated message", func() {
				By("Checking if the status of the root policy is Compliant")
				Eventually(
					common.GetComplianceState(policyName),
					defaultTimeoutSeconds,
					1,
				).Should(Equal(policiesv1.Compliant))

				Eventually(
					common.GetLatestStatusMessage(policyName, 0),
					defaultTimeoutSeconds, 1,
				).Should(Equal("Compliant; notification - pods " +
					"[apple, grape, orange] " +
					"found as specified in namespace " + targetNs))
			})
		})
	})
