// Copyright Contributors to the Open Cluster Management project

package integration

import (
	"context"
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/stolostron/governance-policy-framework/test/common"
)

var _ = Describe("GRC: [P1][Sev1][policy-grc] Test the ACM Hardening "+
	"generated PolicySet in an App subscription", Ordered, Label("policy-collection", "stable"), func() {
	policyNames := []string{
		"policy-check-backups",
		"policy-check-policyreports",
		"policy-managedclusteraddon-available",
		"policy-subscriptions",
	}
	const namespace = "policies"

	It("Sets up the application subscription", func() {
		By("Verifying that the default ManagedClusterSet exists")
		mcs := unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": "cluster.open-cluster-management.io/v1beta2",
				"kind":       "ManagedClusterSet",
				"metadata": map[string]interface{}{
					"name": "default",
				},
			},
		}

		_, err := clientHubDynamic.Resource(common.GvrManagedClusterSet).Create(
			context.TODO(), &mcs, metav1.CreateOptions{},
		)
		if !k8serrors.IsAlreadyExists(err) {
			Expect(err).ToNot(HaveOccurred())
		}

		By("Creating the application subscription")
		_, err = common.OcUser(
			gitopsUser,
			"apply",
			"-f",
			"../resources/policy_generator/acm-hardening_subscription.yaml",
			"-n",
			namespace,
		)
		Expect(err).ShouldNot(HaveOccurred())
	})

	It("Validates the propagated policies", func() {
		By("Checking that the policy set was created")
		policySetRsrc := clientHubDynamic.Resource(common.GvrPolicySet)
		var policyset *unstructured.Unstructured
		Eventually(
			func() error {
				var err error
				policyset, err = policySetRsrc.Namespace(namespace).Get(
					context.TODO(), "acm-hardening", metav1.GetOptions{},
				)

				return err
			},
			defaultTimeoutSeconds*2,
			1,
		).ShouldNot(HaveOccurred())

		// Perform some basic validation on the generated policySet.
		policies, found, err := unstructured.NestedSlice(policyset.Object, "spec", "policies")
		Expect(err).ShouldNot(HaveOccurred())
		Expect(found).Should(BeTrue())
		Expect(policies).Should(HaveLen(len(policyNames)))
		for idx, policyName := range policyNames {
			Expect(policies[idx]).Should(Equal(policyName))
		}

		By("Checking that the subscriptions root policy was created and becomes compliant")
		policyRsrc := clientHubDynamic.Resource(common.GvrPolicy)
		Eventually(
			func() error {
				policy, err := policyRsrc.Namespace(namespace).Get(
					context.TODO(), policyNames[3], metav1.GetOptions{},
				)
				if err != nil {
					compliant, found, myerr := unstructured.NestedString(policy.Object, "status", "compliant")
					if myerr != nil {
						return myerr
					}
					if !found {
						return fmt.Errorf("failed to find the compliant field of the policy status")
					} else if compliant != "Compliant" {
						return fmt.Errorf("The policy is not compliant")
					}
				}

				return err
			},
			defaultTimeoutSeconds*2,
			1,
		).ShouldNot(HaveOccurred())

		By("Checking that the policy-managedclusteraddon-available policy " +
			"was propagated to the local-cluster namespace")
		Eventually(
			func() error {
				_, err := policyRsrc.Namespace("local-cluster").Get(
					context.TODO(),
					namespace+"."+policyNames[2],
					metav1.GetOptions{},
				)

				return err
			},
			defaultTimeoutSeconds*2,
			1,
		).ShouldNot(HaveOccurred())

		By("Checking that the policy reports configuration policy was created in the local-cluster namespace")
		configPolicyRsrc := clientHubDynamic.Resource(common.GvrConfigurationPolicy)
		Eventually(
			func() error {
				_, err := configPolicyRsrc.Namespace("local-cluster").Get(
					context.TODO(), policyNames[1], metav1.GetOptions{},
				)

				return err
			},
			defaultTimeoutSeconds*2,
			1,
		).ShouldNot(HaveOccurred())
	})
})
