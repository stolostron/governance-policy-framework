// Copyright (c) 2022 Red Hat, Inc.
// Copyright Contributors to the Open Cluster Management project

package integration

import (
	"context"
	"errors"
	"os/exec"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8stypes "k8s.io/apimachinery/pkg/types"
	policiesv1 "open-cluster-management.io/governance-policy-propagator/api/v1"
	"open-cluster-management.io/governance-policy-propagator/test/utils"

	"github.com/stolostron/governance-policy-framework/test/common"
)

var _ = Describe("GRC: [P1][Sev1][policy-grc] Test the policy-namespace policy",
	Ordered, Label("policy-collection", "stable"), func() {
		const policyNamespaceName = "policy-namespace"
		policyNamespaceURL := policyCollectCMURL + policyNamespaceName + ".yaml"

		It("stable/"+policyNamespaceName+" should be created on the Hub", func() {
			By("Creating policy on hub")
			_, err := utils.KubectlWithOutput(
				"apply", "-f", policyNamespaceURL, "-n", userNamespace, "--kubeconfig="+kubeconfigHub,
			)
			Expect(err).ToNot(HaveOccurred())

			err = common.PatchPlacementRule(userNamespace, "placement-"+policyNamespaceName)
			Expect(err).ToNot(HaveOccurred())

			By("Checking that " + policyNamespaceName + " exists on the Hub cluster")
			rootPolicy := utils.GetWithTimeout(
				clientHubDynamic,
				common.GvrPolicy,
				policyNamespaceName,
				userNamespace,
				true,
				defaultTimeoutSeconds,
			)
			Expect(rootPolicy).NotTo(BeNil())
		})

		It("stable/"+policyNamespaceName+" should be created on managed cluster", func() {
			By("Checking the policy on managed cluster in ns " + clusterNamespace)
			managedPolicy := utils.GetWithTimeout(
				clientManagedDynamic,
				common.GvrPolicy,
				userNamespace+"."+policyNamespaceName,
				clusterNamespace,
				true,
				defaultTimeoutSeconds,
			)
			Expect(managedPolicy).NotTo(BeNil())
		})

		It("stable/"+policyNamespaceName+" should be NonCompliant", func() {
			By("Checking if the status of the root policy is NonCompliant")
			Eventually(
				common.GetComplianceState(policyNamespaceName),
				defaultTimeoutSeconds*2,
				1,
			).Should(Equal(policiesv1.NonCompliant))
		})

		It("Enforcing stable/"+policyNamespaceName, func() {
			By("Patching remediationAction = enforce on the root policy")
			_, err := clientHubDynamic.Resource(common.GvrPolicy).Namespace(userNamespace).Patch(
				context.TODO(),
				policyNamespaceName,
				k8stypes.JSONPatchType,
				[]byte(`[{"op": "replace", "path": "/spec/remediationAction", "value": "enforce"}]`),
				metav1.PatchOptions{},
			)
			Expect(err).ToNot(HaveOccurred())
		})

		It("stable/"+policyNamespaceName+" should be Compliant", func() {
			By("Checking if the status of the root policy is Compliant")
			Eventually(
				common.GetComplianceState(policyNamespaceName),
				defaultTimeoutSeconds*2,
				1,
			).Should(Equal(policiesv1.Compliant))
		})

		It("The prod Namespace should exist", func() {
			By("Checking the prod namespace")
			Eventually(
				func() error {
					_, err := clientManaged.CoreV1().Namespaces().Get(
						context.TODO(),
						"prod",
						metav1.GetOptions{},
					)

					return err
				},
				defaultTimeoutSeconds*2,
				1,
			).ShouldNot(HaveOccurred())
		})

		AfterAll(func() {
			_, err := utils.KubectlWithOutput(
				"delete", "-f", policyNamespaceURL, "-n",
				userNamespace, "--kubeconfig="+kubeconfigHub,
				"--ignore-not-found",
			)
			Expect(err).ToNot(HaveOccurred())

			err = clientManaged.CoreV1().Namespaces().Delete(context.TODO(), "prod", metav1.DeleteOptions{})
			if !k8serrors.IsNotFound(err) {
				var exitError *exec.ExitError
				ok := errors.As(err, &exitError)
				if ok {
					Expect(exitError.Stderr).To(BeNil())
				} else {
					Expect(err).ToNot(HaveOccurred())
				}
			}
		})
	})
