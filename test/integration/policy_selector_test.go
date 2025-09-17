// Copyright (c) 2022 Red Hat, Inc.
// Copyright Contributors to the Open Cluster Management project

package integration

import (
	"encoding/json"
	"fmt"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	policiesv1 "open-cluster-management.io/governance-policy-propagator/api/v1"

	"github.com/stolostron/governance-policy-framework/test/common"
)

var _ = Describe(
	"GRC: [P1][Sev1][policy-grc] Test objectSelector and context variables",
	Ordered,
	func() {
		const (
			policyName         = "policy-selector"
			policyYAML         = "../resources/policy_selector/policy-selector.yaml"
			configMapName      = "selector-config"
			unrelatedConfigMap = "other-config"
			configNamespace    = "policy-selector-test"
			selectorKey        = "policy-selector-e2e"
		)
		configMapNames := []string{configMapName + "1", configMapName + "2"}

		generateStatus := func(names []string) string {
			return fmt.Sprintf("NonCompliant; violation - configmaps [%s] "+
				"found but not as specified in namespace policy-selector-test",
				strings.Join(names, ", "))
		}

		BeforeAll(func(ctx SpecContext) {
			By("The ConfigMaps should be created on the Managed cluster")
			By(fmt.Sprintf("Create namespace %s on the Managed cluster", configNamespace))
			namespace := &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{Name: configNamespace},
			}

			_, err := clientManaged.CoreV1().Namespaces().Create(ctx, namespace, metav1.CreateOptions{})
			Expect(err).ToNot(HaveOccurred())

			for i, name := range configMapNames {
				By(fmt.Sprintf("Creating ConfigMap %s/%s on the Managed cluster", configNamespace, name))
				configMap := &corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						Name:   name,
						Labels: map[string]string{selectorKey: name},
					},
					Data: map[string]string{"name": fmt.Sprintf("%s%d", "testvalue", i)},
				}

				_, err := clientManaged.CoreV1().ConfigMaps(configNamespace).Create(
					ctx, configMap, metav1.CreateOptions{})
				if !k8serrors.IsAlreadyExists(err) {
					Expect(err).ToNot(HaveOccurred())
				}

				By(fmt.Sprintf("Creating ConfigMap %s/%s on the Managed cluster", configNamespace, unrelatedConfigMap))
				_, err = clientManaged.CoreV1().ConfigMaps(configNamespace).Create(ctx, &corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						Name: unrelatedConfigMap,
					},
				}, metav1.CreateOptions{})
				if !k8serrors.IsAlreadyExists(err) {
					Expect(err).ToNot(HaveOccurred())
				}
			}
		})

		BeforeEach(func(ctx SpecContext) {
			common.DoCreatePolicyTest(ctx, policyYAML, common.GvrConfigurationPolicy)
		})

		// Test parameter struct
		type selectorTest struct {
			selector []metav1.LabelSelectorRequirement
			status   string
		}

		selectorTestRun := func(test selectorTest) {
			By("Patching the objectSelector")
			selector, err := json.Marshal(test.selector)
			Expect(err).NotTo(HaveOccurred())
			jsonPath := "/spec/policy-templates/0/objectDefinition/" +
				"spec/object-templates/0/objectSelector/matchExpressions"
			_, err = common.OcHub(
				"patch", "policies.policy.open-cluster-management.io", policyName,
				"-n", userNamespace, "--type=json", "-p", `[{
					"op":"replace", 
					"path":"`+jsonPath+`",
					"value": `+string(selector)+`}]`)
			Expect(err).ToNot(HaveOccurred())

			By("Checking the status")
			Eventually(common.GetLatestStatusMessage(policyName, 0),
				defaultTimeoutSeconds, 1).Should(Equal(test.status))
		}

		DescribeTable("Reporting the correct status",
			selectorTestRun,
			Entry("With objectSelector with an empty selector", selectorTest{
				selector: []metav1.LabelSelectorRequirement{},
				status:   generateStatus(configMapNames),
			}),
			Entry("With objectSelector with label exists", selectorTest{
				selector: []metav1.LabelSelectorRequirement{
					{Key: selectorKey, Operator: "Exists", Values: []string{}},
				},
				status: generateStatus(configMapNames),
			}),
			Entry("With objectSelector with a label value", selectorTest{
				selector: []metav1.LabelSelectorRequirement{
					{Key: selectorKey, Operator: "In", Values: []string{"selector-config1"}},
				},
				status: generateStatus([]string{"selector-config1"}),
			}),
			Entry("With objectSelector with a non-matching label", selectorTest{
				selector: []metav1.LabelSelectorRequirement{
					{Key: "doesnt-match-anything", Operator: "Exists", Values: []string{}},
				},
				status: "Compliant; notification - No objects of kind ConfigMap " +
					"were matched from the policy objectSelector",
			}),
		)

		Describe("Using skipObject with an argument", func() {
			It("should report the correct status", func() {
				By("Patching skipObject to use an argument")
				_, err := common.OcHub(
					"patch", "policies.policy.open-cluster-management.io", policyName,
					"-n", userNamespace, "--type=json", "-p", `[{
						"op":"replace", 
						"path":"/spec/policy-templates/0/objectDefinition/`+
						`spec/object-templates/0/objectDefinition/metadata/name",
						"value": '{{ not (contains "2" .ObjectName) | skipObject }}{{ .ObjectName }}'}]`)
				Expect(err).ToNot(HaveOccurred())
				selectorTestRun(selectorTest{
					selector: []metav1.LabelSelectorRequirement{},
					status:   generateStatus([]string{"selector-config2"}),
				})
			})
		})

		Describe("Reporting correct status with object updates", func() {
			// Test definition for adding/removing matching ConfigMaps
			It("when a matching ConfigMap is added/removed", func() {
				extraConfigMap := fmt.Sprintf("%s%d-extra", configMapName, len(configMapNames))

				By("Creating a matching ConfigMap")
				newConfigMaps := append(configMapNames, extraConfigMap)
				_, err := common.OcManaged("create", "--namespace", configNamespace, "configmap", extraConfigMap)
				Expect(err).ToNot(HaveOccurred())

				Eventually(common.GetLatestStatusMessage(policyName, 0),
					defaultTimeoutSeconds, 1).Should(Equal(generateStatus(newConfigMaps)))

				By("Deleting a matching ConfigMap")
				_, err = common.OcManaged("delete", "--namespace", configNamespace, "configmap", extraConfigMap)
				Expect(err).ToNot(HaveOccurred())

				Eventually(common.GetLatestStatusMessage(policyName, 0),
					defaultTimeoutSeconds, 1).Should(Equal(generateStatus(configMapNames)))
			})

			It(policyName+" should be Compliant when enforced", func(ctx SpecContext) {
				// Enforce the policy
				common.EnforcePolicy(policyName, common.GvrConfigurationPolicy)
				// Check compliance
				common.DoRootComplianceTest(policyName, policiesv1.Compliant)

				// Check ConfigMaps on the managed cluster
				for _, name := range configMapNames {
					By(fmt.Sprintf("Checking edited ConfigMap %s/%s on the Managed cluster", configNamespace, name))
					Eventually(
						func() (map[string]string, error) {
							configMap, err := clientManaged.CoreV1().ConfigMaps(configNamespace).Get(
								ctx, name, metav1.GetOptions{},
							)

							return configMap.Data, err
						}, defaultTimeoutSeconds*2, 1,
					).Should(
						HaveKeyWithValue("this-is-me", name),
						fmt.Sprintf("ConfigMap %s/%s on the Managed cluster should have expected data",
							configNamespace, name,
						),
					)
				}
			})

			It(policyName+" should have a skipObject status message", func() {
				By("Deleting the test ConfigMaps")
				_, err := common.OcManaged(
					"delete", "--namespace", configNamespace, "configmap", "--selector", selectorKey)
				Expect(err).ToNot(HaveOccurred())

				By("Checking the status")
				Eventually(common.GetLatestStatusMessage(policyName, 0),
					defaultTimeoutSeconds, 1).Should(Equal(
					"Compliant; notification - All objects of kind ConfigMap " +
						"were skipped by the `skipObject` template function"))
			})
		})

		AfterAll(func(ctx SpecContext) {
			By("Deleting policies")
			common.DoCleanupPolicy(policyYAML, common.GvrConfigurationPolicy)
			By(fmt.Sprintf("Deleting Namespace %s from the Managed cluster", configNamespace))
			err := clientManaged.CoreV1().Namespaces().Delete(ctx, configNamespace, metav1.DeleteOptions{})
			if !k8serrors.IsNotFound(err) {
				Expect(err).ToNot(HaveOccurred())
			}
		})
	})
