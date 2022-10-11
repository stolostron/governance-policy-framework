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

var _ = Describe("GRC: [P1][Sev1][policy-grc] Test the Policy Generator "+
	"with a remote Kustomize directory", Ordered, Label("SVT"), func() {
	const policyName = "e2e-grc-remote-policy-app"
	const namespace = "grc-e2e-remote-policy-generator"
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
			Expect(k8serrors.IsAlreadyExists(err)).Should(
				BeTrue(),
				"Expected error to be 'already exists': "+fmt.Sprint(err),
			)
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
			10,
		).Should(BeNil())
		// Delete the kubeconfig file after the test.
		defer func() { os.Remove(kubeconfigSubAdmin) }()

		By("Creating the application subscription")
		_, err = common.OcHub(
			"apply",
			"-f",
			"../resources/policy_generator/subscription-remote.yaml",
			"-n",
			namespace,
			"--kubeconfig="+kubeconfigSubAdmin,
		)
		Expect(err).Should(BeNil())

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
		Expect(err).Should(BeNil())
		Expect(found).Should(BeTrue())
		Expect(len(templates)).Should(Equal(3))

		for _, template := range templates {
			objSpec, found, err := unstructured.NestedMap(template.(map[string]interface{}), "objectDefinition", "spec")
			Expect(err).Should(BeNil())
			Expect(found).Should(BeTrue())
			Expect(objSpec["severity"]).Should(Equal("high"))
			objTemplates, found, err := unstructured.NestedSlice(objSpec, "object-templates")
			Expect(err).Should(BeNil())
			Expect(found).Should(BeTrue())
			Expect(len(objTemplates)).Should(Equal(1))
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

	AfterAll(func() {
		By("Cleaning up the changes made to the cluster in the test")
		cleanup(namespace, secret, ocpUser)
	})
})
