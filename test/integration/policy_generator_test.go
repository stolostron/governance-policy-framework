// Copyright Contributors to the Open Cluster Management project

package integration

import (
	"context"
	"encoding/hex"
	"math/rand"
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

// generateInsecurePassword is a random password generator from 15-30 bytes. It is insecure
// since the characters are limited to just hex values (i.e. 1-9,a-f) from the random bytes. An
// error is returned if the random bytes cannot be read.
func generateInsecurePassword() (string, error) {
	// A password ranging from 15-30 bytes
	pwSize := rand.Intn(15) + 15
	bytes := make([]byte, pwSize)

	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}

	return hex.EncodeToString(bytes), nil
}

// cleanup will remove any test data/configuration on the OpenShift cluster that was added/updated
// as part of the policy generator test. Any errors will be propagated as gomega failed assertions.
func cleanup(namespace string, secret string, user common.OCPUser) {
	err := clientHub.CoreV1().Namespaces().Delete(context.TODO(), namespace, metav1.DeleteOptions{})
	if !k8serrors.IsNotFound(err) {
		Expect(err).Should(BeNil())
	}

	err = common.CleanupOCPUser(clientHub, clientHubDynamic, secret, user)
	Expect(err).Should(BeNil())

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
}

var _ = Describe("GRC: [P1][Sev1][policy-grc] Test the Policy Generator in an App subscription", func() {
	const namespace = "grc-e2e-policy-generator"
	const secret = "grc-e2e-subscription-admin-user"
	const subAdminBinding = "open-cluster-management:subscription-admin"
	ocpUser := common.OCPUser{
		ClusterRoles: []types.NamespacedName{
			{Name: "open-cluster-management:admin:local-cluster"},
			{
				Name:      "admin",
				Namespace: namespace,
			},
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
		userPassword, err := generateInsecurePassword()
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
			"../resources/policy_generator/subscription.yaml",
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
					context.TODO(), "e2e-policyset", metav1.GetOptions{},
				)

				return err
			},
			defaultTimeoutSeconds*2,
			1,
		).Should(BeNil())

		// Perform some basic validation on the generated policySet. There isn't a need to do any more
		// than this since the policy generator unit tests cover this scenario well. This test is
		// meant to verify that the integration is successful.
		policies, found, err := unstructured.NestedSlice(policyset.Object, "spec", "policies")
		Expect(err).Should(BeNil())
		Expect(found).Should(BeTrue())
		Expect(len(policies)).Should(Equal(1))
		Expect(policies[0]).Should(Equal("e2e-grc-policy-app"))

		By("Checking that the root policy was created")
		policyRsrc := clientHubDynamic.Resource(common.GvrPolicy)
		var policy *unstructured.Unstructured
		Eventually(
			func() error {
				var err error
				policy, err = policyRsrc.Namespace(namespace).Get(
					context.TODO(), "e2e-grc-policy-app", metav1.GetOptions{},
				)

				return err
			},
			defaultTimeoutSeconds*2,
			1,
		).Should(BeNil())

		// Perform some basic validation on the generated policy. There isn't a need to do any more
		// than this since the policy generator unit tests cover this scenario well. This test is
		// meant to verify that the integration is successful.
		templates, found, err := unstructured.NestedSlice(policy.Object, "spec", "policy-templates")
		Expect(err).Should(BeNil())
		Expect(found).Should(BeTrue())
		Expect(len(templates)).Should(Equal(1))

		objTemplates, found, err := unstructured.NestedSlice(
			templates[0].(map[string]interface{}), "objectDefinition", "spec", "object-templates",
		)
		Expect(err).Should(BeNil())
		Expect(found).Should(BeTrue())
		Expect(len(objTemplates)).Should(Equal(3))

		By("Checking that the policy was propagated to the local-cluster namespace")
		Eventually(
			func() error {
				var err error
				policy, err = policyRsrc.Namespace("local-cluster").Get(
					context.TODO(),
					"grc-e2e-policy-generator.e2e-grc-policy-app",
					metav1.GetOptions{},
				)

				return err
			},
			defaultTimeoutSeconds,
			1,
		).Should(BeNil())

		By("Checking that the configuration policy was created in the local-cluster namespace")
		configPolicyRsrc := clientHubDynamic.Resource(common.GvrConfigurationPolicy)
		Eventually(
			func() error {
				var err error
				policy, err = configPolicyRsrc.Namespace("local-cluster").Get(
					context.TODO(), "e2e-grc-policy-app", metav1.GetOptions{},
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
