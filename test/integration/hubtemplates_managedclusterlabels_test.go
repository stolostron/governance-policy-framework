// Copyright (c) 2023 Red Hat, Inc.
// Copyright Contributors to the Open Cluster Management project

package integration

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	policiesv1 "open-cluster-management.io/governance-policy-propagator/api/v1"

	"github.com/stolostron/governance-policy-framework/test/common"
)

var _ = Describe("GRC: [P1][Sev1][policy-grc] Test .ManagedClusterLabels in hub templates", Ordered, func() {
	const resourcesPath = "../resources/hub_templates_managedclusterlabels/"

	Describe("A single-value lookup should work", Ordered, Label("BVT"), func() {
		const (
			hubConfigmapYAML     = resourcesPath + "hub-configmap.yaml"
			policyName           = "mclabels-fromcm-pol"
			policyYAML           = resourcesPath + policyName + ".yaml"
			createdConfigmapName = "mclabels-fromcm-created"
		)

		BeforeAll(func() {
			By("Creating a configmap on the hub to use in the test")
			_, err := common.OcHub("apply", "-f="+hubConfigmapYAML, "-n="+userNamespace)
			Expect(err).To(BeNil())
		})

		It(policyName+" should be created on the Hub", func() {
			common.DoCreatePolicyTest(policyYAML, common.GvrConfigurationPolicy)
		})

		It(policyName+" should be Compliant", func() {
			common.DoRootComplianceTest(policyName, policiesv1.Compliant)
		})

		It("Checks that the configmap was created correctly on the managed cluster", func() {
			value, err := common.OcManaged("get", "configmap", createdConfigmapName, "-n=default",
				"-o=jsonpath={.data.testvalue}")
			Expect(err).To(BeNil())
			Expect(value).To(MatchRegexp("Test Success"))
		})

		AfterAll(func() {
			By("Removing the configmap from the hub")
			_, err := common.OcHub("delete", "-f="+hubConfigmapYAML, "-n="+userNamespace, "--ignore-not-found")
			Expect(err).To(BeNil())

			By("Removing the policy")
			common.DoCleanupPolicy(policyYAML, common.GvrConfigurationPolicy)

			By("Removing the configmap from the managed cluster")
			_, err = common.OcManaged("delete", "configmap", createdConfigmapName, "-n=default", "--ignore-not-found")
			Expect(err).To(BeNil())

			By("Removing policy events from the managed cluster")
			_, err = common.OcManaged(
				"delete", "events", "-n", clusterNamespace,
				"--field-selector=involvedObject.name="+common.UserNamespace+"."+policyName,
				"--ignore-not-found",
			)
			Expect(err).To(BeNil())
		})
	})

	Describe("Ranging over all labels on a ManagedCluster should work", Ordered, func() {
		const (
			policyName           = "mclabels-range-pol"
			policyYAML           = resourcesPath + policyName + ".yaml"
			createdConfigmapName = "mclabels-range-created"
		)

		It(policyName+" should be created on the Hub", func() {
			common.DoCreatePolicyTest(policyYAML, common.GvrConfigurationPolicy)
		})

		It(policyName+" should be Compliant", func() {
			common.DoRootComplianceTest(policyName, policiesv1.Compliant)
		})

		It("Checks that the configmap was created correctly on the managed cluster", func() {
			value, err := common.OcManaged("get", "configmap", createdConfigmapName, "-n=default",
				"-o=jsonpath={.data}")
			Expect(err).To(BeNil())
			Expect(value).To(And( // An arbitrary selection of keys that it should have
				MatchRegexp("vendor"),
				MatchRegexp("openshiftVersion"),
				MatchRegexp("cloud"),
				MatchRegexp("feature.open-cluster-management.io"),
			))
		})

		AfterAll(func() {
			By("Removing the policy")
			common.DoCleanupPolicy(policyYAML, common.GvrConfigurationPolicy)

			By("Removing the configmap from the managed cluster")
			_, err := common.OcManaged("delete", "configmap", createdConfigmapName, "-n=default", "--ignore-not-found")
			Expect(err).To(BeNil())

			By("Removing policy events from the managed cluster")
			_, err = common.OcManaged(
				"delete", "events", "-n", clusterNamespace,
				"--field-selector=involvedObject.name="+common.UserNamespace+"."+policyName,
				"--ignore-not-found",
			)
			Expect(err).To(BeNil())
		})
	})
})
