// Copyright (c) 2023 Red Hat, Inc.
// Copyright Contributors to the Open Cluster Management project

package integration

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	policiesv1 "open-cluster-management.io/governance-policy-propagator/api/v1"

	"github.com/stolostron/governance-policy-framework/test/common"
)

var _ = Describe("GRC: [P1][Sev1][policy-grc] Test hub template variables", Ordered, func() {
	const resourcesPath = "../resources/hub_templates_variables/"

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
			Expect(err).ToNot(HaveOccurred())
		})

		It(policyName+" should be created on the Hub", func(ctx SpecContext) {
			common.DoCreatePolicyTest(ctx, policyYAML, common.GvrConfigurationPolicy)
		})

		It(policyName+" should be Compliant", func() {
			common.DoRootComplianceTest(policyName, policiesv1.Compliant)
		})

		It("Checks that the configmap was created correctly on the managed cluster", func(ctx SpecContext) {
			cm, err := clientManaged.CoreV1().ConfigMaps("default").Get(ctx, createdConfigmapName, metav1.GetOptions{})
			Expect(err).ToNot(HaveOccurred())

			Expect(cm.Data["testvalue"]).To(Equal("Test Success"))
			Expect(cm.Data["label"]).To(Equal("raleigh"))
			Expect(cm.Data["annotation"]).To(Equal("NC"))
			Expect(cm.Data["name"]).To(Equal(policyName))
			Expect(cm.Data["namespace"]).To(Equal(common.UserNamespace))
		})

		AfterAll(func() {
			By("Removing the configmap from the hub")
			_, err := common.OcHub("delete", "-f="+hubConfigmapYAML, "-n="+userNamespace, "--ignore-not-found")
			Expect(err).ToNot(HaveOccurred())

			By("Removing the policy")
			common.DoCleanupPolicy(policyYAML, common.GvrConfigurationPolicy)

			By("Removing the configmap from the managed cluster")
			_, err = common.OcManaged("delete", "configmap", createdConfigmapName, "-n=default", "--ignore-not-found")
			Expect(err).ToNot(HaveOccurred())

			By("Removing policy events from the managed cluster")
			_, err = common.OcManaged(
				"delete", "events", "-n", clusterNamespace,
				"--field-selector=involvedObject.name="+common.UserNamespace+"."+policyName,
				"--ignore-not-found",
			)
			Expect(err).ToNot(HaveOccurred())
		})
	})

	Describe("Ranging over all labels on a ManagedCluster should work", Ordered, func() {
		const (
			policyName           = "mclabels-range-pol"
			policyYAML           = resourcesPath + policyName + ".yaml"
			createdConfigmapName = "mclabels-range-created"
		)

		It(policyName+" should be created on the Hub", func(ctx SpecContext) {
			common.DoCreatePolicyTest(ctx, policyYAML, common.GvrConfigurationPolicy)
		})

		It(policyName+" should be Compliant", func() {
			common.DoRootComplianceTest(policyName, policiesv1.Compliant)
		})

		It("Checks that the configmap was created correctly on the managed cluster", func() {
			value, err := common.OcManaged("get", "configmap", createdConfigmapName, "-n=default",
				"-o=jsonpath={.data}")
			Expect(err).ToNot(HaveOccurred())
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
			Expect(err).ToNot(HaveOccurred())

			By("Removing policy events from the managed cluster")
			_, err = common.OcManaged(
				"delete", "events", "-n", clusterNamespace,
				"--field-selector=involvedObject.name="+common.UserNamespace+"."+policyName,
				"--ignore-not-found",
			)
			Expect(err).ToNot(HaveOccurred())
		})
	})
})
