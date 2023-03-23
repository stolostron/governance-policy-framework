// Copyright (c) 2022 Red Hat, Inc.
// Copyright Contributors to the Open Cluster Management project

package integration

import (
	"context"
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
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

		It("The ConfigMaps should be created on the Hub and Managed clusters", func() {
			for cluster, client := range map[string]kubernetes.Interface{
				"hub":     clientHub,
				"managed": clientManaged,
			} {
				for name, data := range map[string]string{
					configMapName1: "testvalue1",
					configMapName2: "testvalue2",
				} {
					By(fmt.Sprintf("Creating ConfigMap %s/%s on the %s cluster", userNamespace, name, cluster))
					configMap := &corev1.ConfigMap{
						ObjectMeta: metav1.ObjectMeta{Name: name},
						Data:       map[string]string{"name": data},
					}

					_, err := client.CoreV1().ConfigMaps(userNamespace).Create(ctx, configMap, metav1.CreateOptions{})
					if !k8serrors.IsAlreadyExists(err) {
						Expect(err).To(BeNil())
					}
				}
			}
		})

		It(policyHubName+" should be created on the Hub", func() {
			common.DoCreatePolicyTest(policyHubYAML, common.GvrConfigurationPolicy)
		})

		It(policyHubName+" should be Compliant", func() {
			common.DoRootComplianceTest(policyHubName, policiesv1.Compliant)
		})

		It("The ConfigMaps on should be patched with the correct data", func() {
			for cluster, client := range map[string]kubernetes.Interface{
				"hub":     clientHub,
				"managed": clientManaged,
			} {
				for _, name := range []string{
					configMapName1, configMapName2,
				} {
					By(fmt.Sprintf("Checking edited ConfigMap %s/%s on the %s cluster", userNamespace, name, cluster))
					Eventually(
						func() string {
							configMap, err := client.CoreV1().ConfigMaps(userNamespace).Get(
								ctx, name, metav1.GetOptions{},
							)
							if err != nil {
								return ""
							}

							return configMap.Data["extraData"]
						},
						defaultTimeoutSeconds*2,
						1,
					).Should(Equal("exists!"))
				}
			}
		})

		It("Creates a config namespace to copy configMaps into", func() {
			Expect(clientManaged.CoreV1().Namespaces().Create(context.TODO(), &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{Name: configNamespace},
			}, metav1.CreateOptions{})).NotTo(BeNil())
		})

		It(policyNoHubName+" should be created on the Hub", func() {
			common.DoCreatePolicyTest(policyNoHubYAML, common.GvrConfigurationPolicy)
		})

		It(policyNoHubName+" should be Compliant", func() {
			common.DoRootComplianceTest(policyNoHubName, policiesv1.Compliant)
		})

		It("The ConfigMaps should be copied to the new namespace with the correct data", func() {
			for cluster, client := range map[string]kubernetes.Interface{
				"hub":     clientHub,
				"managed": clientManaged,
			} {
				for _, name := range []string{
					configMapName1, configMapName2,
				} {
					By(fmt.Sprintf("Checking copied ConfigMap %s/%s on the %s cluster", configNamespace, name, cluster))
					Eventually(
						func() string {
							configMap, err := client.CoreV1().ConfigMaps(configNamespace).Get(
								ctx, name+"-copy", metav1.GetOptions{},
							)
							if err != nil {
								return ""
							}

							return configMap.Data["extraData"]
						}, defaultTimeoutSeconds*2, 1,
					).Should(Equal("exists!"))
				}
			}
		})

		AfterAll(func() {
			By("Delete policies and config maps")
			common.DoCleanupPolicy(policyHubYAML, common.GvrConfigurationPolicy)
			common.DoCleanupPolicy(policyNoHubYAML, common.GvrConfigurationPolicy)

			for _, client := range []kubernetes.Interface{clientHub, clientManaged} {
				for _, name := range []string{configMapName1, configMapName2} {
					err := client.CoreV1().ConfigMaps(userNamespace).Delete(ctx, name, metav1.DeleteOptions{})
					if !k8serrors.IsNotFound(err) {
						Expect(err).To(BeNil())
					}

					err = client.CoreV1().ConfigMaps(configNamespace).Delete(ctx, name+"-copy", metav1.DeleteOptions{})
					if !k8serrors.IsNotFound(err) {
						Expect(err).To(BeNil())
					}
				}
			}

			By("Delete Namespace " + configNamespace)
			_, err := common.OcHub("delete", "namespace", configNamespace, "--ignore-not-found")
			Expect(err).To(BeNil())
		})
	})
