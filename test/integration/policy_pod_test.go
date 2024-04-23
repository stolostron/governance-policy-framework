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

var _ = Describe("GRC: [P1][Sev1][policy-grc] Test the policy-pod policy",
	Ordered, Label("policy-collection", "stable"), func() {
		const (
			policyPodName   = "policy-pod"
			policyPodNSName = "policy-pod"
			podName         = "sample-nginx-pod"
		)
		policyPodURL := policyCollectCMURL + policyPodName + ".yaml"

		It("stable/"+policyPodName+" should be created on the Hub", func() {
			By("Creating the policy on the Hub")
			_, err := utils.KubectlWithOutput(
				"apply", "-f", policyPodURL, "-n", userNamespace, "--kubeconfig="+kubeconfigHub,
			)
			Expect(err).ToNot(HaveOccurred())

			By("Creating the " + policyPodNSName + " namespace on the managed cluster")
			namespace := &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name:   policyPodNSName,
					Labels: map[string]string{"e2e": "true"},
				},
			}
			_, err = clientManaged.CoreV1().Namespaces().Create(
				context.TODO(),
				namespace,
				metav1.CreateOptions{},
			)
			Expect(err).ToNot(HaveOccurred())

			By("Patching the namespaceSelector to use the " + policyPodNSName + " namespace")
			_, err = clientHubDynamic.Resource(common.GvrPolicy).Namespace(userNamespace).Patch(
				context.TODO(),
				policyPodName,
				k8stypes.JSONPatchType,
				[]byte(`[{"op": "replace", "path": "/spec/policy-templates`+
					`/0/objectDefinition/spec/namespaceSelector/include", "value": ["`+
					policyPodNSName+`"]}]`),
				metav1.PatchOptions{},
			)
			Expect(err).ToNot(HaveOccurred())

			By("Patching placement rule")
			err = common.PatchPlacementRule(userNamespace, "placement-"+policyPodName)
			Expect(err).ToNot(HaveOccurred())

			By("Checking that " + policyPodName + " exists on the Hub cluster")
			rootPolicy := utils.GetWithTimeout(
				clientHubDynamic, common.GvrPolicy, policyPodName, userNamespace, true, defaultTimeoutSeconds,
			)
			Expect(rootPolicy).NotTo(BeNil())
		})

		It("stable/"+policyPodName+" should be created on managed cluster", func() {
			By("Checking the policy on managed cluster in ns " + clusterNamespace)
			managedPolicy := utils.GetWithTimeout(
				clientManagedDynamic,
				common.GvrPolicy,
				userNamespace+"."+policyPodName,
				clusterNamespace,
				true,
				defaultTimeoutSeconds,
			)
			Expect(managedPolicy).NotTo(BeNil())
		})

		It("stable/"+policyPodName+" should be NonCompliant", func() {
			By("Checking if the status of the root policy is NonCompliant")
			Eventually(
				common.GetComplianceState(policyPodName),
				defaultTimeoutSeconds*2,
				1,
			).Should(Equal(policiesv1.NonCompliant))
		})

		It("Enforcing stable/"+policyPodName, func() {
			By("Patching remediationAction = enforce on the root policy")
			_, err := clientHubDynamic.Resource(common.GvrPolicy).Namespace(userNamespace).Patch(
				context.TODO(),
				policyPodName,
				k8stypes.JSONPatchType,
				[]byte(`[{"op": "replace", "path": "/spec/remediationAction", "value": "enforce"}]`),
				metav1.PatchOptions{},
			)
			Expect(err).ToNot(HaveOccurred())
		})

		It("stable/"+policyPodName+" should be Compliant", func() {
			By("Checking if the status of the root policy is Compliant")
			Eventually(
				common.GetComplianceState(policyPodName),
				defaultTimeoutSeconds*2,
				1,
			).Should(Equal(policiesv1.Compliant))
		})

		It("The Pod should exist", func() {
			By("Checking the Pod in the " + policyPodNSName + " namespace")
			Eventually(
				func() error {
					_, err := clientManaged.CoreV1().Pods(policyPodNSName).Get(
						context.TODO(), podName, metav1.GetOptions{},
					)

					return err
				},
				defaultTimeoutSeconds*2,
				1,
			).ShouldNot(HaveOccurred())
		})

		AfterAll(func() {
			_, err := utils.KubectlWithOutput(
				"delete", "-f", policyPodURL, "-n", userNamespace,
				"--kubeconfig="+kubeconfigHub, "--ignore-not-found",
			)
			Expect(err).ToNot(HaveOccurred())

			err = clientManaged.CoreV1().Namespaces().Delete(
				context.TODO(), policyPodNSName, metav1.DeleteOptions{},
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
