// Copyright Contributors to the Open Cluster Management project

package integration

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/stolostron/governance-policy-framework/test/common"
)

var _ = Describe("GRC: [P1][Sev1][policy-grc] Test the Policy Generator "+
	"in an App subscription", Ordered, Label("BVT"), func() {
	const policyName = "e2e-grc-policy-app"
	const namespace = "grc-e2e-policy-generator"

	It("Sets up the application subscription", func() {
		By("Creating the application subscription")
		_, err := common.OcUser(
			gitopsUser,
			"apply",
			"-f",
			"../resources/policy_generator/subscription.yaml",
			"-n",
			namespace,
		)
		Expect(err).ShouldNot(HaveOccurred())

		Eventually(func(g Gomega) {
			appSubRsrc, err := clientHubDynamic.Resource(common.GvrSubscription).Namespace(namespace).Get(
				context.TODO(), "grc-e2e-policy-generator-subscription", metav1.GetOptions{},
			)
			g.Expect(err).ShouldNot(HaveOccurred(), "The subscription should exist.")

			appSubPhase, found, err := unstructured.NestedString(appSubRsrc.Object, "status", "phase")
			g.Expect(err).ShouldNot(HaveOccurred(), "The subscription status should be parseable.")
			g.Expect(found).Should(BeTrue(), "The subscription status should have a phase.")
			g.Expect(appSubPhase).Should(Equal("Propagated"), "The subscription should propagate successfully.")
		},
			defaultTimeoutSeconds,
			1,
		).Should(Succeed())
	})

	It("Validates the propagated policies", func() {
		By("Checking that the policy set was created")
		policySetRsrc := clientHubDynamic.Resource(common.GvrPolicySet)
		var policyset *unstructured.Unstructured
		Eventually(
			func() error {
				var err error
				policyset, err = policySetRsrc.Namespace(namespace).Get(
					context.TODO(), "e2e-policyset", metav1.GetOptions{},
				)

				return err
			},
			defaultTimeoutSeconds*4,
			1,
		).ShouldNot(HaveOccurred())

		// Perform some basic validation on the generated policySet. There isn't a need to do any more
		// than this since the policy generator unit tests cover this scenario well. This test is
		// meant to verify that the integration is successful.
		policies, found, err := unstructured.NestedSlice(policyset.Object, "spec", "policies")
		Expect(err).ShouldNot(HaveOccurred())
		Expect(found).Should(BeTrue())
		Expect(policies).Should(HaveLen(1))
		Expect(policies[0]).Should(Equal(policyName))

		By("Checking that the root policy was created")
		policyRsrc := clientHubDynamic.Resource(common.GvrPolicy)
		var policy *unstructured.Unstructured
		Eventually(
			func() error {
				var err error
				policy, err = policyRsrc.Namespace(namespace).Get(
					context.TODO(), policyName, metav1.GetOptions{},
				)

				return err
			},
			defaultTimeoutSeconds*2,
			1,
		).ShouldNot(HaveOccurred())

		// Perform some basic validation on the generated policy. There isn't a need to do any more
		// than this since the policy generator unit tests cover this scenario well. This test is
		// meant to verify that the integration is successful.
		templates, found, err := unstructured.NestedSlice(policy.Object, "spec", "policy-templates")
		Expect(err).ShouldNot(HaveOccurred())
		Expect(found).Should(BeTrue())
		Expect(templates).Should(HaveLen(1))

		objTemplates, found, err := unstructured.NestedSlice(
			templates[0].(map[string]interface{}), "objectDefinition", "spec", "object-templates",
		)
		Expect(err).ShouldNot(HaveOccurred())
		Expect(found).Should(BeTrue())
		Expect(objTemplates).Should(HaveLen(3))

		By("Checking that the policy was propagated to the local-cluster namespace")
		Eventually(
			func() error {
				_, err := policyRsrc.Namespace("local-cluster").Get(
					context.TODO(),
					namespace+"."+policyName,
					metav1.GetOptions{},
				)

				return err
			},
			defaultTimeoutSeconds*2,
			1,
		).ShouldNot(HaveOccurred())

		By("Checking that the configuration policy was created in the local-cluster namespace")
		configPolicyRsrc := clientHubDynamic.Resource(common.GvrConfigurationPolicy)
		Eventually(
			func() error {
				_, err := configPolicyRsrc.Namespace("local-cluster").Get(
					context.TODO(), policyName, metav1.GetOptions{},
				)

				return err
			},
			defaultTimeoutSeconds,
			1,
		).ShouldNot(HaveOccurred())
	})
})
