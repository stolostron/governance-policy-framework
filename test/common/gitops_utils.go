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

var gitopsTestNamespaces = []string{
	"grc-e2e-policy-generator",
	"grc-e2e-remote-policy-generator",
	"policies",
}

// GitOpsUserSetup configures a new user to use for the GitOps tests. It updates the provided
// OCPUser instance, which contains a path to the created kubeconfig file.
func GitOpsUserSetup(ocpUser *OCPUser) {
	const subAdminBinding = "open-cluster-management:subscription-admin"
	const clustersetRoleName = "grc-e2e-clusterset-role"

	ocpUser.ClusterRoles = []types.NamespacedName{
		{Name: "open-cluster-management:admin:local-cluster"},
		{Name: clustersetRoleName},
	}
	ocpUser.ClusterRoleBindings = []string{subAdminBinding}
	ocpUser.Username = "grc-e2e-subadmin-user"

	// Add additional cluster roles for each namespace
	for _, ns := range gitopsTestNamespaces {
		ocpUser.ClusterRoles = append(ocpUser.ClusterRoles, types.NamespacedName{
			Name:      "admin",
			Namespace: ns,
		})
	}

	By("Setting up the managed cluster set binding role for the GitOps user")

	clusterSetRule := rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: clustersetRoleName,
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups:     []string{"cluster.open-cluster-management.io"},
				Verbs:         []string{"create"},
				Resources:     []string{"managedclustersets/bind"},
				ResourceNames: []string{"default", "global"},
			},
			{
				APIGroups: []string{"cluster.open-cluster-management.io"},
				Verbs:     []string{"get", "list", "watch"},
				Resources: []string{"placementdecisions"},
			},
		},
	}

	_, err := ClientHub.RbacV1().ClusterRoles().Create(
		context.TODO(), &clusterSetRule, metav1.CreateOptions{},
	)
	if err != nil {
		Expect(k8serrors.IsAlreadyExists(err)).Should(BeTrue())
	}

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

	_, err = ClientHub.RbacV1().ClusterRoleBindings().Create(
		context.TODO(), &subAdminBindingObj, metav1.CreateOptions{},
	)
	if err != nil {
		ExpectWithOffset(1, k8serrors.IsAlreadyExists(err)).Should(
			BeTrue(),
			"Expected error to be 'already exists': "+fmt.Sprint(err),
		)
	}

	By("Cleaning up any existing subscription-admin user config")
	GitOpsCleanup(*ocpUser)

	for _, ns := range gitopsTestNamespaces {
		CleanupHubNamespace(ns)
	}

	By("Creating a subscription-admin user and configuring IDP")
	// Create a namespace to house the subscription configuration.
	for _, ns := range gitopsTestNamespaces {
		nsObj := corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: ns}}
		_, err = ClientHub.CoreV1().Namespaces().Create(
			context.TODO(), &nsObj, metav1.CreateOptions{},
		)
		ExpectWithOffset(1, err).ShouldNot(HaveOccurred())
	}

	// Create the OpenShift user that can be used for logging in.
	ocpUser.Password, err = GenerateInsecurePassword()
	ExpectWithOffset(1, err).ShouldNot(HaveOccurred())

	err = CreateOCPUser(ClientHub, ClientHubDynamic, *ocpUser)
	ExpectWithOffset(1, err).ShouldNot(HaveOccurred())

	// Get a kubeconfig logged in as the subscription and local-cluster administrator OpenShift
	// user.
	hubServerURL, err := OcHub("whoami", "--show-server=true")
	ExpectWithOffset(1, err).ShouldNot(HaveOccurred())

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
}

// GitOpsCleanup will remove any test data/configuration on the OpenShift cluster that was added/updated
// as part of the GitOps test. The kubeconfig file is also deleted from the filesystem. Any errors will
// be propagated as gomega failed assertions.
func GitOpsCleanup(user OCPUser) {
	By("Cleaning up artifacts from user " + user.Username)
	// Delete kubeconfig file if it is specified
	if user.Kubeconfig != "" {
		err := os.Remove(user.Kubeconfig)
		ExpectWithOffset(1, err).ShouldNot(HaveOccurred())
	}

	err := CleanupOCPUser(ClientHub, ClientHubDynamic, user)
	ExpectWithOffset(1, err).ShouldNot(HaveOccurred())

	err = ClientHub.CoreV1().Secrets("openshift-config").Delete(context.TODO(), user.Username, metav1.DeleteOptions{})
	if !k8serrors.IsNotFound(err) {
		ExpectWithOffset(1, err).ShouldNot(HaveOccurred())
	}

	gitopsTestNamespaces := []string{
		"grc-e2e-policy-generator",
		"grc-e2e-remote-policy-generator",
		"policies",
	}
	for _, ns := range gitopsTestNamespaces {
		CleanupHubNamespace(ns)
	}

	for _, ns := range gitopsTestNamespaces {
		CleanupHubNamespace(ns)
	}
}
