// Copyright Contributors to the Open Cluster Management project

package integration

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/stolostron/governance-policy-framework/test/common"
)

var _ = Describe("GRC: [P1][Sev1][policy-grc] Test the Policy Generator "+
	"with a Helm chart", Ordered, Label("SVT"), func() {
	const policyName = "e2e-grc-helm-policy-app"
	const namespace = "grc-e2e-helm-policy-generator"

	It("Sets up the application subscription", func(ctx SpecContext) {
		By("Creating the application subscription")
		_, err := common.OcUser(
			gitopsUser,
			"apply",
			"-f",
			"../resources/policy_generator/subscription-helm.yaml",
			"-n",
			namespace,
		)
		Expect(err).ShouldNot(HaveOccurred())

		Eventually(func(g Gomega) {
			appSubRsrc, err := clientHubDynamic.Resource(common.GvrSubscription).Namespace(namespace).Get(
				ctx, "grc-e2e-helm-policy-generator-subscription", metav1.GetOptions{},
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

	It("Validates the propagated policies", func(ctx SpecContext) {
		// Perform some basic validation on the generated policy.
		By("Checking that the root policy was created")
		policyRsrc := clientHubDynamic.Resource(common.GvrPolicy)
		var policy *unstructured.Unstructured
		Eventually(
			func() error {
				var err error
				policy, err = policyRsrc.Namespace(namespace).Get(
					ctx, policyName, metav1.GetOptions{},
				)

				return err
			},
			defaultTimeoutSeconds*2,
			1,
		).ShouldNot(HaveOccurred())

		templates, found, err := unstructured.NestedSlice(policy.Object, "spec", "policy-templates")
		Expect(err).ShouldNot(HaveOccurred())
		Expect(found).Should(BeTrue())
		Expect(templates).Should(HaveLen(3))

		for _, template := range templates {
			objSpec, found, err := unstructured.NestedMap(template.(map[string]interface{}), "objectDefinition", "spec")
			Expect(err).ShouldNot(HaveOccurred())
			Expect(found).Should(BeTrue())
			Expect(objSpec["severity"]).Should(Equal("critical"))
			objTemplates, found, err := unstructured.NestedSlice(objSpec, "object-templates")
			Expect(err).ShouldNot(HaveOccurred())
			Expect(found).Should(BeTrue())
			Expect(objTemplates).Should(HaveLen(1))
			templateObj := objTemplates[0].(map[string]interface{})
			Expect(templateObj["complianceType"]).Should(Equal("musthave"))
		}

		By("Checking that the policy was propagated to the local-cluster namespace")
		Eventually(
			func() error {
				_, err := policyRsrc.Namespace("local-cluster").Get(
					ctx,
					namespace+"."+policyName,
					metav1.GetOptions{},
				)

				return err
			},
			defaultTimeoutSeconds*2,
			1,
		).ShouldNot(HaveOccurred())

		By("Checking that the configuration policies were created in the local-cluster namespace")
		configPolicyRsrc := clientHubDynamic.Resource(common.GvrConfigurationPolicy)
		for _, suffix := range []string{"", "2", "3"} {
			Eventually(
				func() error {
					_, err := configPolicyRsrc.Namespace("local-cluster").Get(
						ctx, policyName+suffix, metav1.GetOptions{},
					)

					return err
				},
				defaultTimeoutSeconds,
				1,
			).ShouldNot(HaveOccurred())
		}

		By("Confirming that the Helm lookup returned nothing")
		helmDeploymentPolicy, err := configPolicyRsrc.Namespace("local-cluster").Get(
			ctx, policyName+"3", metav1.GetOptions{},
		)
		Expect(err).ShouldNot(HaveOccurred())
		objTemplates, found, err := unstructured.NestedSlice(helmDeploymentPolicy.Object, "spec", "object-templates")
		Expect(err).ShouldNot(HaveOccurred())
		Expect(found).Should(BeTrue(), "object-templates should be present in the ConfigurationPolicy")
		Expect(objTemplates).Should(HaveLen(1), "There should only be one object-template in the ConfigurationPolicy")
		labelSpyValue, found, err := unstructured.NestedString(
			objTemplates[0].(map[string]interface{}),
			"objectDefinition", "spec", "template", "metadata", "labels", "label-spy",
		)
		Expect(err).ShouldNot(HaveOccurred())
		Expect(found).Should(BeTrue(), "label 'label-spy' should be present in the ConfigurationPolicy")
		Expect(labelSpyValue).Should(BeEmpty(), "label-spy should not have content from Helm lookup")
	})
})
