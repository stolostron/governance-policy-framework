// Copyright Contributors to the Open Cluster Management project

package integration

import (
	"context"
	"fmt"
	"os"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"

	"github.com/stolostron/governance-policy-framework/test/common"
)

// cleanup will remove any test data/configuration on the OpenShift cluster that was added/updated
// as part of the policy generator test. Any errors will be propagated as gomega failed assertions.
func cleanup(namespace string, secret string, user common.OCPUser) {
	err := clientHub.CoreV1().Namespaces().Delete(context.TODO(), namespace, metav1.DeleteOptions{})
	if !k8serrors.IsNotFound(err) {
		Expect(err).Should(BeNil())
	}

	// Wait for the namespace to be fully deleted before proceeding.
	Eventually(
		func() bool {
			_, err := clientHub.CoreV1().Namespaces().Get(
				context.TODO(), namespace, metav1.GetOptions{},
			)
			return k8serrors.IsNotFound(err)
		},
		defaultTimeoutSeconds,
		1,
	).Should(BeTrue())

	err = common.CleanupOCPUser(clientHub, clientHubDynamic, secret, user)
	Expect(err).Should(BeNil())

	err = clientHub.CoreV1().Secrets("openshift-config").Delete(context.TODO(), secret, metav1.DeleteOptions{})
	if !k8serrors.IsNotFound(err) {
		Expect(err).Should(BeNil())
	}
}

var _ = Describe("GRC: [P1][Sev1][policy-grc] Test the ACM Hardening generated PolicySet in an App subscription", func() {
	const namespace = "policies"
	const secret = "grc-e2e-subscription-admin-user"
	const clustersetRoleName = "grc-e2e-clusterset-role"
	const subAdminBinding = "open-cluster-management:subscription-admin"
	ocpUser := common.OCPUser{
		ClusterRoles: []types.NamespacedName{
			{Name: "open-cluster-management:admin:local-cluster"},
			{
				Name:      "admin",
				Namespace: namespace,
			},
			{Name: clustersetRoleName},
		},
		// To be considered a subscription-admin you must be part of this cluster role binding.
		// Having the proper role in another cluster role binding does not work.
		ClusterRoleBindings: []string{subAdminBinding},
		Password:            "",
		Username:            "grc-e2e-subscription-admin",
	}

	It("Sets up the application subscription", func() {
		By("Verifying that the subscription-admin ClusterRoleBinding exists")
		// Occasionally, the subscription-admin ClusterRoleBinding may not exist due to some unknown
		// error. This ClusterRoleBinding is supposed to have been created by the App Lifecycle
		// controllers. In this unusual case, create the ClusterRoleBinding based on the advice from
		// the Application Lifecycle squad.
		subAdminBindingObj := rbacv1.ClusterRoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name: subAdminBinding,
			},
			RoleRef: rbacv1.RoleRef{
				APIGroup: "rbac.authorization.k8s.io",
				Kind:     "ClusterRole",
				Name:     subAdminBinding,
			},
		}
		_, err := clientHub.RbacV1().ClusterRoleBindings().Create(
			context.TODO(), &subAdminBindingObj, metav1.CreateOptions{},
		)
		if err != nil {
			Expect(k8serrors.IsAlreadyExists(err)).Should(BeTrue())
		}

		By("Verifying that the managed cluster set binding ClusterRole exists")
		clusterSetRule := rbacv1.ClusterRole{
			ObjectMeta: metav1.ObjectMeta{
				Name: clustersetRoleName,
			},
			Rules: []rbacv1.PolicyRule{
				{
					APIGroups:     []string{"cluster.open-cluster-management.io"},
					Verbs:         []string{"create"},
					Resources:     []string{"managedclustersets/bind"},
					ResourceNames: []string{"default"},
				},
			},
		}

		_, err = clientHub.RbacV1().ClusterRoles().Create(
			context.TODO(), &clusterSetRule, metav1.CreateOptions{},
		)
		if err != nil {
			Expect(k8serrors.IsAlreadyExists(err)).Should(BeTrue())
		}

		By("Cleaning up any existing subscription-admin user config")
		cleanup(namespace, secret, ocpUser)

		By("Creating a subscription-admin user and configuring IDP")
		// Create a namespace to house the subscription configuration.
		nsObj := corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: namespace}}
		_, err = clientHub.CoreV1().Namespaces().Create(
			context.TODO(), &nsObj, metav1.CreateOptions{},
		)
		Expect(err).Should(BeNil())

		// Create a subscription and local-cluster administrator OpenShift user that can be used
		// for logging in.
		userPassword, err := common.GenerateInsecurePassword()
		Expect(err).Should(BeNil())
		ocpUser.Password = userPassword
		err = common.CreateOCPUser(clientHub, clientHubDynamic, secret, ocpUser)
		Expect(err).Should(BeNil())

		// Get a kubeconfig logged in as the subscription and local-cluster administrator OpenShift
		// user.
		hubServerURL, err := common.OcHub("whoami", "--show-server=true")
		Expect(err).Should(BeNil())
		hubServerURL = strings.TrimSuffix(hubServerURL, "\n")
		// Use eventually since it can take a while for OpenShift to configure itself with the new
		// identity provider (IDP).
		var kubeconfigSubAdmin string
		const fiveMinutes = 5 * 60
		Eventually(
			func() error {
				var err error
				kubeconfigSubAdmin, err = common.GetKubeConfig(
					hubServerURL, ocpUser.Username, ocpUser.Password,
				)
				return err
			},
			fiveMinutes,
			1,
		).Should(BeNil())
		// Delete the kubeconfig file after the test.
		defer func() { os.Remove(kubeconfigSubAdmin) }()

		By("Creating the application subscription")
		_, err = common.OcHub(
			"apply",
			"-f",
			"../resources/policy_generator/acm-hardening_subscription.yaml",
			"-n",
			namespace,
			"--kubeconfig="+kubeconfigSubAdmin,
		)
		Expect(err).Should(BeNil())

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
		).Should(BeNil())

		// Perform some basic validation on the generated policySet.
		policies, found, err := unstructured.NestedSlice(policyset.Object, "spec", "policies")
		Expect(err).Should(BeNil())
		Expect(found).Should(BeTrue())
		Expect(len(policies)).Should(Equal(4))
		Expect(policies[0]).Should(Equal("policy-check-backups"))
		Expect(policies[1]).Should(Equal("policy-check-policyreports"))
		Expect(policies[2]).Should(Equal("policy-managedclusteraddon-available"))
		Expect(policies[3]).Should(Equal("policy-subscriptions"))

		By("Checking that the subscriptions root policy was created and becomes compliant")
		policyRsrc := clientHubDynamic.Resource(common.GvrPolicy)
		var policy *unstructured.Unstructured
		Eventually(
			func() error {
				var err error
				policy, err = policyRsrc.Namespace(namespace).Get(
					context.TODO(), "policy-subscriptions", metav1.GetOptions{},
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
		).Should(BeNil())

		By("Checking that the policy-managedclusteraddon-available policy was propagated to the local-cluster namespace")
		Eventually(
			func() error {
				var err error
				policy, err = policyRsrc.Namespace("local-cluster").Get(
					context.TODO(),
					"policies.policy-managedclusteraddon-available",
					metav1.GetOptions{},
				)
				return err
			},
			defaultTimeoutSeconds,
			1,
		).Should(BeNil())

		By("Checking that the policy reports configuration policy was created in the local-cluster namespace")
		configPolicyRsrc := clientHubDynamic.Resource(common.GvrConfigurationPolicy)
		Eventually(
			func() error {
				var err error
				policy, err = configPolicyRsrc.Namespace("local-cluster").Get(
					context.TODO(), "policy-check-policyreports", metav1.GetOptions{},
				)
				return err
			},
			defaultTimeoutSeconds,
			1,
		).Should(BeNil())
	})

	It("Cleans up", func() {
		By("Cleaning up the changes made to the cluster in the test")
		cleanup(namespace, secret, ocpUser)
	})
})
