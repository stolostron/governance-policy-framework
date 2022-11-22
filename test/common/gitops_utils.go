// Copyright Contributors to the Open Cluster Management project

package common

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
	"k8s.io/apimachinery/pkg/types"
)

const gitOpsUserPrefix = "grc-e2e-subadmin-user-"

// GitOpsUserSetup configures a new user to use for the GitOps deployments.
// The provided namespace is deleted and recreated as part of the setup.
// It returns the OCPUser instance, which contains a path to the created kubeconfig file.
func GitOpsUserSetup(
	namespace string, usernameSuffix string, additionalRoles ...types.NamespacedName,
) OCPUser {
	const subAdminBinding = "open-cluster-management:subscription-admin"

	ocpUser := OCPUser{
		ClusterRoles: []types.NamespacedName{
			{Name: "open-cluster-management:admin:local-cluster"},
			{
				Name:      "admin",
				Namespace: namespace,
			},
		},
		ClusterRoleBindings: []string{subAdminBinding},
		Password:            "",
		Username:            gitOpsUserPrefix + usernameSuffix,
	}

	// Append any additional provided ClusterRoles
	ocpUser.ClusterRoles = append(ocpUser.ClusterRoles, additionalRoles...)

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

	By("Verifying that the subscription-admin ClusterRoleBinding exists")

	_, err := ClientHub.RbacV1().ClusterRoleBindings().Create(
		context.TODO(), &subAdminBindingObj, metav1.CreateOptions{},
	)
	if err != nil {
		ExpectWithOffset(1, k8serrors.IsAlreadyExists(err)).Should(
			BeTrue(),
			"Expected error to be 'already exists': "+fmt.Sprint(err),
		)
	}

	By("Cleaning up any existing subscription-admin user config")
	GitOpsCleanup(namespace, ocpUser)

	By("Creating a subscription-admin user and configuring IDP")
	// Create a namespace to house the subscription configuration.
	nsObj := corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: namespace}}
	_, err = ClientHub.CoreV1().Namespaces().Create(
		context.TODO(), &nsObj, metav1.CreateOptions{},
	)
	ExpectWithOffset(1, err).Should(BeNil())

	// Create the OpenShift user that can be used for logging in.
	ocpUser.Password, err = GenerateInsecurePassword()
	ExpectWithOffset(1, err).Should(BeNil())

	err = CreateOCPUser(ClientHub, ClientHubDynamic, ocpUser)
	ExpectWithOffset(1, err).Should(BeNil())

	// Get a kubeconfig logged in as the subscription and local-cluster administrator OpenShift
	// user.
	hubServerURL, err := OcHub("whoami", "--show-server=true")
	ExpectWithOffset(1, err).Should(BeNil())

	hubServerURL = strings.TrimSuffix(hubServerURL, "\n")
	// Use eventually since it can take a while for OpenShift to configure itself with the new
	// identity provider (IDP).
	const fiveMinutes = 5 * 60

	EventuallyWithOffset(1,
		func() error {
			var err error
			ocpUser.Kubeconfig, err = GetKubeConfig(
				hubServerURL, ocpUser.Username, ocpUser.Password,
			)

			return err
		},
		fiveMinutes,
		10,
	).Should(BeNil())

	return ocpUser
}

// GitOpsCleanup will remove any test data/configuration on the OpenShift cluster that was added/updated
// as part of the GitOps test. The kubeconfig file is also deleted from the filesystem. Any errors will
// be propagated as gomega failed assertions.
func GitOpsCleanup(namespace string, user OCPUser) {
	By("Cleaning up artifacts from user " + user.Username)
	// Delete kubeconfig file if it is specified
	if user.Kubeconfig != "" {
		err := os.Remove(user.Kubeconfig)
		ExpectWithOffset(1, err).Should(BeNil())
	}

	err := CleanupOCPUser(ClientHub, ClientHubDynamic, user)
	ExpectWithOffset(1, err).Should(BeNil())

	err = ClientHub.CoreV1().Secrets("openshift-config").Delete(context.TODO(), user.Username, metav1.DeleteOptions{})
	if !k8serrors.IsNotFound(err) {
		ExpectWithOffset(1, err).Should(BeNil())
	}

	By("Deleting namespace " + namespace)

	err = ClientHub.CoreV1().Namespaces().Delete(context.TODO(), namespace, metav1.DeleteOptions{})
	if !k8serrors.IsNotFound(err) {
		ExpectWithOffset(1, err).Should(BeNil())
	}

	// Wait for the namespace to be fully deleted before proceeding.
	EventuallyWithOffset(1,
		func() bool {
			_, err := ClientHub.CoreV1().Namespaces().Get(
				context.TODO(), namespace, metav1.GetOptions{},
			)
			isNotFound := k8serrors.IsNotFound(err)
			if !isNotFound && err != nil {
				GinkgoWriter.Printf("'%s' namespace 'get' error: %w", err)
			}

			return isNotFound
		},
		DefaultTimeoutSeconds*2,
		1,
	).Should(BeTrue())
}
