// Copyright (c) 2022 Red Hat, Inc.
// Copyright Contributors to the Open Cluster Management project

package test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	policiesv1 "open-cluster-management.io/governance-policy-propagator/api/v1"
	"open-cluster-management.io/governance-policy-propagator/test/utils"

	. "github.com/stolostron/governance-policy-framework/test/common"
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
		testPolicySetYaml      = "../resources/policy_ordering/dep-plcset.yaml"
		testPolicySetName      = "test-policyset"
		testPolicyName         = "test-policy"
		plcWithDepOnSetYaml    = "../resources/policy_ordering/dep-policy-dep-on-plcset.yaml"
		plcWithDepOnSetName    = "dep-policy-dep-on-plcset"
	)

	cleanup := func() {
		By("Cleaning up")
		DoCleanupPolicy(initialPolicyYaml)
		DoCleanupPolicy(policyWithDepYaml)
		DoCleanupPolicy(policyWithExtraDepYaml)
		DoCleanupPolicy(ignorePendingYaml)
		DoCleanupPolicy(plcWithDepOnSetYaml)

		configmapNames := []string{
			"dep-initial-cfgmap",
			"deppol-cfgmap",
			"dep-extradepconfig-cfgmap",
			"dep-ignorepending-cfgmap",
			"dep-ignorepending-extra-cfgmap",
		}

		for _, name := range configmapNames {
			_, err := OcManaged("delete", "configmap", name, "-n=default", "--ignore-not-found")
			Expect(err).ToNot(HaveOccurred())
		}

		_, err := OcHub(
			"delete", "-f", testPolicySetYaml,
			"-n", UserNamespace, "--ignore-not-found",
		)
		Expect(err).ToNot(HaveOccurred())
	}

	Describe("GRC: [P1][Sev1][policy-grc] Test policy ordering", Ordered, Label(labels...), func() {
		Describe("Ordering via a dependency on a Policy", Ordered, func() {
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
		Describe("Ordering via an extraDependency on a ConfigurationPolicy", Ordered, func() {
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
		Describe("IgnorePending should allow policies to be compliant when one template is pending", Ordered, func() {
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
		Describe("Ordering via a dependency on a PolicySet", Ordered, func() {
			It("Should create policyset with noncompliant status", func() {
				By("Creating the initial policy set to use as a dependency")
				_, err := OcHub("apply", "-f", testPolicySetYaml, "-n", UserNamespace)
				Expect(err).ToNot(HaveOccurred())

				rootPolicy := utils.GetWithTimeout(
					ClientHubDynamic, GvrPolicy, testPolicyName, UserNamespace, true, DefaultTimeoutSeconds,
				)
				Expect(rootPolicy).NotTo(BeNil())

				By("Patching " + testPolicySetName + "-plr with decision of cluster managed")
				plr := utils.GetWithTimeout(
					ClientHubDynamic, GvrPlacementRule, testPolicySetName+"-plr", UserNamespace,
					true, DefaultTimeoutSeconds,
				)
				plr.Object["status"] = utils.GeneratePlrStatus(ClusterNamespaceOnHub)
				_, err = ClientHubDynamic.Resource(GvrPlacementRule).Namespace(UserNamespace).UpdateStatus(
					context.TODO(), plr, metav1.UpdateOptions{},
				)
				Expect(err).ToNot(HaveOccurred())

				By("Checking " + testPolicyName + " on managed cluster in ns " + ClusterNamespaceOnHub)
				managedplc := utils.GetWithTimeout(
					ClientHubDynamic, GvrPolicy, UserNamespace+"."+testPolicyName, ClusterNamespaceOnHub, true,
					DefaultTimeoutSeconds,
				)
				Expect(managedplc).NotTo(BeNil())

				plcSet := utils.GetWithTimeout(
					ClientHubDynamic, GvrPolicySet, testPolicySetName, UserNamespace, true, DefaultTimeoutSeconds,
				)
				Expect(plcSet).NotTo(BeNil())

				By("Checking the status of policy set - NonCompliant")
				yamlPlc := utils.ParseYaml("../resources/policy_ordering/dep-plcset-statuscheck.yaml")

				Eventually(func() interface{} {
					rootPlcSet := utils.GetWithTimeout(
						ClientHubDynamic,
						GvrPolicySet,
						testPolicySetName,
						UserNamespace,
						true,
						DefaultTimeoutSeconds,
					)

					return rootPlcSet.Object["status"]
				}, DefaultTimeoutSeconds, 1).Should(utils.SemanticEqual(yamlPlc.Object["status"]))
			})
			It("Should be pending while the initial policy is non-compliant", func() {
				By("Creating the policy that depends on the initial policy")
				DoCreatePolicyTest(plcWithDepOnSetYaml)
				DoRootComplianceTest(plcWithDepOnSetName, policiesv1.Pending)
			})
			It("Should become active after the initial policy is enforced", func() {
				EnforcePolicy(testPolicyName, GvrConfigurationPolicy)
				DoRootComplianceTest(testPolicyName, policiesv1.Compliant)
				DoRootComplianceTest(plcWithDepOnSetName, policiesv1.NonCompliant)
			})
			It("Should become pending again when the initial policy is deleted", func() {
				_, err := OcHub(
					"delete", "-f", testPolicySetYaml,
					"-n", UserNamespace, "--ignore-not-found",
				)
				Expect(err).ToNot(HaveOccurred())

				_, err = OcManaged(
					"delete", "pod",
					"-n", "default",
					"pod-dne", "--ignore-not-found",
				)
				Expect(err).ToNot(HaveOccurred())
				DoRootComplianceTest(plcWithDepOnSetName, policiesv1.Pending)
			})
			AfterAll(cleanup)
		})
	})

	return true
}
