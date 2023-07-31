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
	"with a remote Kustomize directory", Ordered, Label("SVT"), func() {
	const policyName = "e2e-grc-remote-policy-app"
	const namespace = "grc-e2e-remote-policy-generator"

	It("Sets up the application subscription", func() {
		By("Creating the application subscription")
		_, err := common.OcUser(
			gitopsUser,
			"apply",
			"-f",
			"../resources/policy_generator/subscription-remote.yaml",
			"-n",
			namespace,
		)
		Expect(err).ShouldNot(HaveOccurred())
	})

	It("Validates the propagated policies", func() {
		// Perform some basic validation on the generated policy.
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
		).Should(BeNil())

		templates, found, err := unstructured.NestedSlice(policy.Object, "spec", "policy-templates")
		Expect(err).ShouldNot(HaveOccurred())
		Expect(found).Should(BeTrue())
		Expect(templates).Should(HaveLen(3))

		for _, template := range templates {
			objSpec, found, err := unstructured.NestedMap(template.(map[string]interface{}), "objectDefinition", "spec")
			Expect(err).ShouldNot(HaveOccurred())
			Expect(found).Should(BeTrue())
			Expect(objSpec["severity"]).Should(Equal("high"))
			objTemplates, found, err := unstructured.NestedSlice(objSpec, "object-templates")
			Expect(err).ShouldNot(HaveOccurred())
			Expect(found).Should(BeTrue())
			Expect(objTemplates).Should(HaveLen(1))
			templateObj := objTemplates[0].(map[string]interface{})
			Expect(templateObj["complianceType"]).Should(Equal("mustnothave"))
		}

		By("Checking that the policy was propagated to the local-cluster namespace")
		Eventually(
			func() error {
				var err error
				policy, err = policyRsrc.Namespace("local-cluster").Get(
					context.TODO(),
					namespace+"."+policyName,
					metav1.GetOptions{},
				)

				return err
			},
			defaultTimeoutSeconds*2,
			1,
		).Should(BeNil())

		By("Checking that the configuration policies were created in the local-cluster namespace")
		configPolicyRsrc := clientHubDynamic.Resource(common.GvrConfigurationPolicy)
		for _, suffix := range []string{"", "2", "3"} {
			Eventually(
				func() error {
					var err error
					policy, err = configPolicyRsrc.Namespace("local-cluster").Get(
						context.TODO(), policyName+suffix, metav1.GetOptions{},
					)

					return err
				},
				defaultTimeoutSeconds,
				1,
			).Should(BeNil())
		}
	})
})
