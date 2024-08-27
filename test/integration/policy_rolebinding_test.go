// Copyright (c) 2022 Red Hat, Inc.
// Copyright Contributors to the Open Cluster Management project

package integration

import (
	"context"
	"errors"
	"os/exec"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8stypes "k8s.io/apimachinery/pkg/types"
	policiesv1 "open-cluster-management.io/governance-policy-propagator/api/v1"
	"open-cluster-management.io/governance-policy-propagator/test/utils"

	"github.com/stolostron/governance-policy-framework/test/common"
)

var _ = Describe("GRC: [P1][Sev1][policy-grc] Test the policy-rolebinding policy",
	Ordered, Label("policy-collection", "stable"), func() {
		const (
			policyRoleBindingName   = "policy-rolebinding"
			policyRoleBindingNSName = policyRoleBindingName + "ns"
			roleBindingName         = "sample-rolebinding"
		)
		policyRoleBindingURL := policyCollectACURL + policyRoleBindingName + ".yaml"

		It("stable/"+policyRoleBindingName+" should be created on the Hub", func(ctx SpecContext) {
			By("Creating policy on hub")
			_, err := utils.KubectlWithOutput(
				"apply", "-f", policyRoleBindingURL, "-n", userNamespace, "--kubeconfig="+kubeconfigHub,
			)
			Expect(err).ToNot(HaveOccurred())

			By("Creating the " + policyRoleBindingNSName + " namespace on the managed cluster")
			namespace := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{
				Name:   policyRoleBindingNSName,
				Labels: map[string]string{"e2e": "true"},
			}}
			_, err = clientManaged.CoreV1().Namespaces().Create(context.TODO(), namespace, metav1.CreateOptions{})
			Expect(err).ToNot(HaveOccurred())

			By("Patching the namespaceSelector to use the " + policyRoleBindingNSName + " namespace")
			_, err = clientHubDynamic.Resource(common.GvrPolicy).Namespace(userNamespace).Patch(
				context.TODO(),
				policyRoleBindingName,
				k8stypes.JSONPatchType,
				[]byte(`[{"op": "replace", "path": "/spec/policy-templates/0/`+
					`objectDefinition/spec/namespaceSelector/include", "value": ["`+
					policyRoleBindingNSName+`"]}]`),
				metav1.PatchOptions{},
			)
			Expect(err).ToNot(HaveOccurred())

			err = common.ApplyPlacement(ctx, userNamespace, policyRoleBindingName)
			Expect(err).ToNot(HaveOccurred())

			By("Checking that " + policyRoleBindingName + " exists on the Hub cluster")
			rootPolicy := utils.GetWithTimeout(
				clientHubDynamic, common.GvrPolicy, policyRoleBindingName, userNamespace, true, defaultTimeoutSeconds,
			)
			Expect(rootPolicy).NotTo(BeNil())
		})

		It("stable/"+policyRoleBindingName+" should be created on managed cluster", func() {
			By("Checking the policy on managed cluster in ns " + clusterNamespace)
			managedPolicy := utils.GetWithTimeout(
				clientManagedDynamic,
				common.GvrPolicy,
				userNamespace+"."+policyRoleBindingName,
				clusterNamespace,
				true,
				defaultTimeoutSeconds,
			)
			Expect(managedPolicy).NotTo(BeNil())
		})

		It("stable/"+policyRoleBindingName+" should be NonCompliant", func() {
			By("Checking if the status of the root policy is NonCompliant")
			Eventually(
				common.GetComplianceState(policyRoleBindingName),
				defaultTimeoutSeconds*2,
				1,
			).Should(Equal(policiesv1.NonCompliant))
		})

		It("Enforcing stable/"+policyRoleBindingName, func() {
			By("Patching remediationAction = enforce on the root policy")
			_, err := clientHubDynamic.Resource(common.GvrPolicy).Namespace(userNamespace).Patch(
				context.TODO(),
				policyRoleBindingName,
				k8stypes.JSONPatchType,
				[]byte(`[{"op": "replace", "path": "/spec/remediationAction", "value": "enforce"}]`),
				metav1.PatchOptions{},
			)
			Expect(err).ToNot(HaveOccurred())
		})

		It("stable/"+policyRoleBindingName+" should be Compliant", func() {
			By("Checking if the status of the root policy is Compliant")
			Eventually(
				common.GetComplianceState(policyRoleBindingName),
				defaultTimeoutSeconds*2,
				1,
			).Should(Equal(policiesv1.Compliant))
		})

		It("The RoleBinding should exist", func() {
			By("Checking the RoleBinding in the " + policyRoleBindingNSName + " namespace")
			Eventually(
				func() error {
					_, err := clientManaged.RbacV1().RoleBindings(policyRoleBindingNSName).Get(
						context.TODO(), roleBindingName, metav1.GetOptions{},
					)

					return err
				},
				defaultTimeoutSeconds*2,
				1,
			).ShouldNot(HaveOccurred())
		})

		AfterAll(func() {
			_, err := utils.KubectlWithOutput(
				"delete",
				"-f",
				policyRoleBindingURL,
				"-n",
				userNamespace,
				"--kubeconfig="+kubeconfigHub,
				"--ignore-not-found",
			)
			Expect(err).ToNot(HaveOccurred())

			err = clientManaged.CoreV1().Namespaces().Delete(
				context.TODO(),
				policyRoleBindingNSName,
				metav1.DeleteOptions{},
			)
			if !k8serrors.IsNotFound(err) {
				var exitError *exec.ExitError
				ok := errors.As(err, &exitError)
				if ok {
					Expect(exitError.Stderr).To(BeNil())
				} else {
					Expect(err).ToNot(HaveOccurred())
				}
			}

			err = common.DeletePlacement(userNamespace, policyRoleBindingName)
			Expect(err).ToNot(HaveOccurred())
		})
	})
