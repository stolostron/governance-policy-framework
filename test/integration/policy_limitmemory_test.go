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

var _ = Describe("GRC: [P1][Sev1][policy-grc] Test the policy-limitmemory policy",
	Ordered, Label("policy-collection", "stable"), func() {
		const (
			policyLimitMemoryName   = "policy-limitmemory"
			policyLimitMemoryURL    = policyCollectSCURL + policyLimitMemoryName + ".yaml"
			policyLimitMemoryNSName = "policy-limitmemory"
			limitRangeName          = "mem-limit-range"
		)

		It("stable/"+policyLimitMemoryName+" should be created on the Hub", func() {
			By("Creating the policy on the Hub")
			_, err := utils.KubectlWithOutput(
				"apply", "-f", policyLimitMemoryURL, "-n", userNamespace, "--kubeconfig="+kubeconfigHub,
			)
			Expect(err).To(BeNil())

			By("Creating the " + policyLimitMemoryNSName + " namespace on the managed cluster")
			namespace := &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name:   policyLimitMemoryNSName,
					Labels: map[string]string{"e2e": "true"},
				},
			}
			_, err = clientManaged.CoreV1().Namespaces().Create(
				context.TODO(),
				namespace,
				metav1.CreateOptions{},
			)
			Expect(err).To(BeNil())

			By("Patching the namespaceSelector to use the " + policyLimitMemoryNSName + " namespace")
			_, err = clientHubDynamic.Resource(common.GvrPolicy).Namespace(userNamespace).Patch(
				context.TODO(),
				policyLimitMemoryName,
				k8stypes.JSONPatchType,
				[]byte(`[{"op": "replace", "path": "/spec/policy-templates/`+
					`0/objectDefinition/spec/namespaceSelector/include", "value": ["`+
					policyLimitMemoryNSName+`"]}]`),
				metav1.PatchOptions{},
			)
			Expect(err).To(BeNil())

			By("Patching placement rule")
			err = common.PatchPlacementRule(
				userNamespace, "placement-"+policyLimitMemoryName, clusterNamespace, kubeconfigHub,
			)
			Expect(err).To(BeNil())

			By("Checking that " + policyLimitMemoryName + " exists on the Hub cluster")
			rootPolicy := utils.GetWithTimeout(
				clientHubDynamic,
				common.GvrPolicy,
				policyLimitMemoryName,
				userNamespace,
				true,
				defaultTimeoutSeconds,
			)
			Expect(rootPolicy).NotTo(BeNil())
		})

		It("stable/"+policyLimitMemoryName+" should be created on managed cluster", func() {
			By("Checking the policy on managed cluster in ns " + clusterNamespace)
			managedPolicy := utils.GetWithTimeout(
				clientManagedDynamic,
				common.GvrPolicy,
				userNamespace+"."+policyLimitMemoryName,
				clusterNamespace,
				true,
				defaultTimeoutSeconds,
			)
			Expect(managedPolicy).NotTo(BeNil())
		})

		It("stable/"+policyLimitMemoryName+" should be NonCompliant", func() {
			By("Checking if the status of the root policy is NonCompliant")
			Eventually(
				common.GetComplianceState(clientHubDynamic, userNamespace, policyLimitMemoryName, clusterNamespace),
				defaultTimeoutSeconds*2,
				1,
			).Should(Equal(policiesv1.NonCompliant))
		})

		It("Enforcing stable/"+policyLimitMemoryName, func() {
			By("Patching remediationAction = enforce on the root policy")
			_, err := clientHubDynamic.Resource(common.GvrPolicy).Namespace(userNamespace).Patch(
				context.TODO(),
				policyLimitMemoryName,
				k8stypes.JSONPatchType,
				[]byte(`[{"op": "replace", "path": "/spec/remediationAction", "value": "enforce"}]`),
				metav1.PatchOptions{},
			)
			Expect(err).To(BeNil())
		})

		It("stable/"+policyLimitMemoryName+" should be Compliant", func() {
			By("Checking if the status of the root policy is Compliant")
			Eventually(
				common.GetComplianceState(clientHubDynamic, userNamespace, policyLimitMemoryName, clusterNamespace),
				defaultTimeoutSeconds*2,
				1,
			).Should(Equal(policiesv1.Compliant))
		})

		It("The LimitRange should exist", func() {
			By("Checking the LimitRange in the " + policyLimitMemoryNSName + " namespace")
			Eventually(
				func() error {
					_, err := clientManaged.CoreV1().LimitRanges(policyLimitMemoryNSName).Get(
						context.TODO(), limitRangeName, metav1.GetOptions{},
					)

					return err
				},
				defaultTimeoutSeconds*2,
				1,
			).Should(BeNil())
		})

		AfterAll(func() {
			_, err := utils.KubectlWithOutput(
				"delete", "-f", policyLimitMemoryURL, "-n",
				userNamespace, "--kubeconfig="+kubeconfigHub,
				"--ignore-not-found",
			)
			Expect(err).To(BeNil())

			err = clientManaged.CoreV1().Namespaces().Delete(
				context.TODO(), policyLimitMemoryNSName, metav1.DeleteOptions{},
			)
			if !k8serrors.IsNotFound(err) {
				var exitError *exec.ExitError
				ok := errors.As(err, &exitError)
				if ok {
					Expect(exitError.Stderr).To(BeNil())
				} else {
					Expect(err).To(BeNil())
				}
			}
		})
	})
