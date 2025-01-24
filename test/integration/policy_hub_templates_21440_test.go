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

	"github.com/stolostron/governance-policy-framework/test/common"
)

// cleanup will remove any test data/configuration on the OpenShift cluster that was added/updated
// as part of the test. Any errors will be propagated as gomega failed assertions.
func cleanupConfig(ctx context.Context, configMapName string, configMapCopyName string) {
	err := clientHub.CoreV1().ConfigMaps(userNamespace).Delete(ctx, configMapName, metav1.DeleteOptions{})
	if !k8serrors.IsNotFound(err) {
		Expect(err).ToNot(HaveOccurred())
	}

	err = clientManaged.CoreV1().ConfigMaps("default").Delete(ctx, configMapCopyName, metav1.DeleteOptions{})
	if !k8serrors.IsNotFound(err) {
		Expect(err).ToNot(HaveOccurred())
	}
}

// See https://github.com/stolostron/backlog/issues/21440
var _ = Describe(
	"GRC: [P1][Sev2][policy-grc] Test that the text/template backport is included (21440)",
	Ordered,
	func() {
		const (
			policyName        = "policy-hub-templates-21440"
			policyYAML        = "../resources/policy_hub_templates_21440/policy-hub-templates-21440.yaml"
			configMapName     = policyName
			configMapCopyName = policyName + "-copy"
		)

		It("The ConfigMap "+configMapName+" should be created on the Hub", func(ctx SpecContext) {
			cleanupConfig(ctx, configMapName, configMapCopyName)

			configMap := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name: configMapName,
				},
				Data: map[string]string{
					"host": "redhat.com",
				},
			}

			_, err := clientHub.CoreV1().ConfigMaps(userNamespace).Create(ctx, configMap, metav1.CreateOptions{})
			Expect(err).ToNot(HaveOccurred())
		})

		It(policyName+" should be created on the Hub", func() {
			common.DoCreatePolicyTest(policyYAML, common.GvrConfigurationPolicy)
		})

		It(policyName+" should be Compliant", func() {
			common.DoRootComplianceTest(policyName, policiesv1.Compliant)
		})

		It("The ConfigMap "+configMapCopyName+
			" should have been created on the managed cluster", func(ctx SpecContext) {
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

		AfterAll(func(ctx SpecContext) {
			common.DoCleanupPolicy(policyYAML, common.GvrConfigurationPolicy)
			cleanupConfig(ctx, configMapName, configMapCopyName)
		})
	})
