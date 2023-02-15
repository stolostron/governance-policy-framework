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

// See https://issues.redhat.com/browse/ACM-1682
var _ = Describe(
	"GRC: [P1][Sev1][policy-grc] Test multiline templatization with the object-templates-raw field",
	Ordered,
	Label("policy-collection", "stable"),
	func() {
		const (
			policyHubName   = "policy-multiline-template-hub"
			policyHubYAML   = "../resources/policy_multiline_templatization/policy-multiline-template-hub.yaml"
			policyNoHubName = "policy-multiline-template-nohub"
			policyNoHubYAML = "../resources/policy_multiline_templatization/policy-multiline-template-nohub.yaml"
			configMapName1  = "templatization-config1"
			configMapName2  = "templatization-config2"
			configNamespace = "config-test"
		)

		ctx := context.TODO()

		It("The ConfigMaps should be created on the Hub", func() {
			configMap := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name: configMapName1,
				},
				Data: map[string]string{
					"name": "testvalue1",
				},
			}

			_, err := clientHub.CoreV1().ConfigMaps(userNamespace).Create(ctx, configMap, metav1.CreateOptions{})
			Expect(err).To(BeNil())

			configMap2 := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name: configMapName2,
				},
				Data: map[string]string{
					"name": "testvalue2",
				},
			}

			_, err = clientHub.CoreV1().ConfigMaps(userNamespace).Create(ctx, configMap2, metav1.CreateOptions{})
			Expect(err).To(BeNil())
		})

		It(policyHubName+" should be created on the Hub", func() {
			common.DoCreatePolicyTest(policyHubYAML, common.GvrConfigurationPolicy)
		})

		It(policyHubName+" should be Compliant", func() {
			common.DoRootComplianceTest(policyHubName, policiesv1.Compliant)
		})

		It("The ConfigMaps on the hub cluster should be patched with the correct data", func() {
			By("Checking the edited configMaps")
			Eventually(
				func() string {
					configMap, err := clientHub.CoreV1().ConfigMaps(userNamespace).Get(
						ctx, configMapName1, metav1.GetOptions{},
					)
					if err != nil {
						return ""
					}

					return configMap.Data["extraData"]
				},
				defaultTimeoutSeconds*2,
				1,
			).Should(Equal("exists!"))
			Eventually(
				func() string {
					configMap, err := clientHub.CoreV1().ConfigMaps(userNamespace).Get(
						ctx, configMapName2, metav1.GetOptions{},
					)
					if err != nil {
						return ""
					}

					return configMap.Data["extraData"]
				},
				defaultTimeoutSeconds*2,
				1,
			).Should(Equal("exists!"))
		})

		It("Creates a config namespace to copy configMaps into", func() {
			Expect(clientHub.CoreV1().Namespaces().Create(context.TODO(), &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: configNamespace,
				},
			}, metav1.CreateOptions{})).NotTo(BeNil())
		})

		It(policyNoHubName+" should be created on the Hub", func() {
			common.DoCreatePolicyTest(policyNoHubYAML, common.GvrConfigurationPolicy)
		})

		It(policyNoHubName+" should be Compliant", func() {
			common.DoRootComplianceTest(policyNoHubName, policiesv1.Compliant)
		})

		It("The ConfigMaps should be copied to the new namespace with the correct data", func() {
			By("Checking the copied configMaps")
			Eventually(
				func() string {
					configMap, err := clientHub.CoreV1().ConfigMaps(configNamespace).Get(
						ctx, configMapName1+"-copy", metav1.GetOptions{},
					)
					if err != nil {
						return ""
					}

					return configMap.Data["extraData"]
				},
				defaultTimeoutSeconds*2,
				1,
			).Should(Equal("exists!"))
			Eventually(
				func() string {
					configMap, err := clientHub.CoreV1().ConfigMaps(configNamespace).Get(
						ctx, configMapName2+"-copy", metav1.GetOptions{},
					)
					if err != nil {
						return ""
					}

					return configMap.Data["extraData"]
				},
				defaultTimeoutSeconds*2,
				1,
			).Should(Equal("exists!"))
		})

		AfterAll(func() {
			By("Delete policies and config maps")
			common.DoCleanupPolicy(policyHubYAML, common.GvrConfigurationPolicy)
			common.DoCleanupPolicy(policyNoHubYAML, common.GvrConfigurationPolicy)

			for _, name := range []string{configMapName1, configMapName2} {
				err := clientHub.CoreV1().ConfigMaps(userNamespace).Delete(ctx, name, metav1.DeleteOptions{})
				if !k8serrors.IsNotFound(err) {
					Expect(err).To(BeNil())
				}
			}

			for _, name := range []string{configMapName1, configMapName2} {
				err := clientHub.CoreV1().ConfigMaps(configNamespace).Delete(ctx, name+"-copy", metav1.DeleteOptions{})
				if !k8serrors.IsNotFound(err) {
					Expect(err).To(BeNil())
				}
			}

			By("Delete Config Namespace if needed")
			_, err := common.OcHub(
				"delete", "namespace", configNamespace,
				"--ignore-not-found",
			)
			Expect(err).To(BeNil())
		})
	})
