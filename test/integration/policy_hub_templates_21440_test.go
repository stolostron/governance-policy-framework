// Copyright (c) 2022 Red Hat, Inc.
// Copyright Contributors to the Open Cluster Management project

package integration

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	policiesv1 "open-cluster-management.io/governance-policy-propagator/api/v1"
	"open-cluster-management.io/governance-policy-propagator/test/utils"

	"github.com/stolostron/governance-policy-framework/test/common"
)

// See https://github.com/stolostron/backlog/issues/21440
var _ = Describe(
	"GRC: [P1][Sev2][policy-grc] Test that the text/template backport is included (21440)",
	Label("policy-collection", "stable"),
	func() {
		const (
			policyName        = "policy-hub-templates-21440"
			policyYAML        = "../resources/policy_hub_templates_21440/policy.yaml"
			configMapName     = policyName
			configMapCopyName = policyName + "-copy"
		)

		ctx := context.TODO()

		It("The ConfigMap "+configMapName+" should be created on the Hub", func() {
			configMap := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name: configMapName,
				},
				Data: map[string]string{
					"host": "redhat.com",
				},
			}

			_, err := clientHub.CoreV1().ConfigMaps("default").Create(ctx, configMap, metav1.CreateOptions{})
			Expect(err).To(BeNil())
		})

		It(policyName+" should be created on the Hub", func() {
			By("Creating the policy on the Hub")
			_, err := utils.KubectlWithOutput(
				"apply", "-f", policyYAML, "-n", "default", "--kubeconfig="+kubeconfigHub,
			)
			Expect(err).To(BeNil())

			By("Patching the placement rule")
			err = common.PatchPlacementRule(
				"default", "placement-"+policyName, clusterNamespace, kubeconfigHub,
			)
			Expect(err).To(BeNil())

			By("Checking that " + policyName + " exists on the Hub cluster")
			rootPolicy := utils.GetWithTimeout(
				clientHubDynamic, common.GvrPolicy, policyName, "default", true, defaultTimeoutSeconds,
			)
			Expect(rootPolicy).NotTo(BeNil())
		})

		It(policyName+" should be created on the managed cluster", func() {
			By("Checking the policy on the managed cluster in the namespace " + clusterNamespace)
			managedPolicy := utils.GetWithTimeout(
				clientManagedDynamic,
				common.GvrPolicy,
				"default."+policyName,
				clusterNamespace,
				true,
				defaultTimeoutSeconds,
			)
			Expect(managedPolicy).NotTo(BeNil())
		})

		It(policyName+" should be Compliant", func() {
			By("Checking if the status of the root policy is Compliant")
			Eventually(
				common.GetComplianceState(clientHubDynamic, "default", policyName, clusterNamespace),
				defaultTimeoutSeconds*2,
				1,
			).Should(Equal(policiesv1.Compliant))
		})

		It("The ConfigMap "+configMapCopyName+" should have been created on the managed cluster", func() {
			By("Checking the copied ConfigMap")
			Eventually(
				func() string {
					configMap, err := clientManaged.CoreV1().ConfigMaps("default").Get(
						ctx, configMapCopyName, metav1.GetOptions{},
					)
					if err != nil {
						return ""
					}

					return configMap.Data["host"]
				},
				defaultTimeoutSeconds*2,
				1,
			).Should(Equal("redhat.com"))
		})

		It("Cleans up", func() {
			_, err := utils.KubectlWithOutput(
				"delete", "-f", policyYAML, "-n", "default", "--kubeconfig="+kubeconfigHub,
			)
			if !k8serrors.IsNotFound(err) {
				Expect(err).To(BeNil())
			}

			err = clientHub.CoreV1().ConfigMaps("default").Delete(ctx, configMapName, metav1.DeleteOptions{})
			if !k8serrors.IsNotFound(err) {
				Expect(err).To(BeNil())
			}

			err = clientManaged.CoreV1().ConfigMaps("default").Delete(ctx, configMapCopyName, metav1.DeleteOptions{})
			if !k8serrors.IsNotFound(err) {
				Expect(err).To(BeNil())
			}
		})
	})
