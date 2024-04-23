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

var _ = Describe("GRC: [P1][Sev1][policy-grc] Test the policy-psp policy",
	Ordered, Label("policy-collection", "stable"), func() {
		rootPolicyURL := policyCollectSCURL + "policy-psp.yaml"
		const (
			rootPolicyName = "policy-podsecuritypolicy"
			pspName        = "sample-restricted-psp"
		)

		BeforeAll(func() {
			if common.IsAtLeastVersion("4.12") {
				Skip("Skipping as the PodSecurityPolicy is removed in OCP v4.12 and above")
			}
		})

		It("stable/"+rootPolicyName+" should be created on the hub cluster", func() {
			By("Creating " + rootPolicyName + " on the hub cluster")
			_, err := utils.KubectlWithOutput(
				"apply",
				"-f",
				rootPolicyURL,
				"-n",
				userNamespace,
				"--kubeconfig="+kubeconfigHub,
			)
			Expect(err).ToNot(HaveOccurred())

			By("Checking " + rootPolicyName + " exists on the hub cluster in ns " + userNamespace)
			rootPolicy := utils.GetWithTimeout(
				clientHubDynamic,
				common.GvrPolicy,
				rootPolicyName,
				userNamespace,
				true,
				defaultTimeoutSeconds,
			)
			Expect(rootPolicy).NotTo(BeNil())
		})

		It("stable/"+rootPolicyName+" should be created on the managed cluster", func() {
			By("Patching placement rule placement-" + rootPolicyName)
			err := common.PatchPlacementRule(userNamespace, "placement-"+rootPolicyName)
			Expect(err).ToNot(HaveOccurred())

			By("Checking " + rootPolicyName + " on the managed cluster in ns " + clusterNamespace)
			managedPolicy := utils.GetWithTimeout(
				clientManagedDynamic,
				common.GvrPolicy,
				userNamespace+"."+rootPolicyName,
				clusterNamespace,
				true,
				defaultTimeoutSeconds,
			)
			Expect(managedPolicy).NotTo(BeNil())
		})

		It("stable/"+rootPolicyName+" should be NonCompliant", func() {
			By("Checking the status of the root policy " + rootPolicyName + " is NonCompliant")
			Eventually(
				common.GetComplianceState(rootPolicyName),
				defaultTimeoutSeconds*2,
				1,
			).Should(Equal(policiesv1.NonCompliant))
		})

		It("Enforcing stable/"+rootPolicyName, func() {
			By("Enforcing the root policy " + rootPolicyName + " to make it compliant")
			_, err := clientHubDynamic.Resource(common.GvrPolicy).Namespace(userNamespace).Patch(
				context.TODO(),
				rootPolicyName,
				k8stypes.JSONPatchType,
				[]byte(`[{"op": "replace", "path": "/spec/remediationAction", "value": "enforce"}]`),
				metav1.PatchOptions{},
			)
			Expect(err).ToNot(HaveOccurred())
		})

		It("stable/"+rootPolicyName+" should be Compliant", func() {
			By("Checking if the status of the root policy " + rootPolicyName + " is Compliant")
			Eventually(
				common.GetComplianceState(rootPolicyName),
				defaultTimeoutSeconds*4,
				1,
			).Should(Equal(policiesv1.Compliant))
		})

		It("The PodSecurityPolicy "+pspName+" should exist on the managed cluster", func() {
			By("Checking the PodSecurityPolicy " + pspName + " on the managed cluster")
			Eventually(
				func() error {
					_, err := clientManaged.PolicyV1beta1().PodSecurityPolicies().Get(
						context.TODO(), pspName, metav1.GetOptions{},
					)

					return err
				},
				defaultTimeoutSeconds*2,
				1,
			).ShouldNot(HaveOccurred())
		})

		AfterAll(func() {
			By("Deleting the PodSecurityPolicy " + rootPolicyName + " on the hub cluster")
			_, err := utils.KubectlWithOutput(
				"delete",
				"-f",
				rootPolicyURL,
				"-n",
				userNamespace,
				"--kubeconfig="+kubeconfigHub,
				"--ignore-not-found",
			)
			Expect(err).ToNot(HaveOccurred())

			By("Deleting the PodSecurityPolicy " + pspName + " on the managed cluster")
			err = clientManaged.PolicyV1beta1().PodSecurityPolicies().Delete(
				context.TODO(),
				pspName,
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
		})
	})
