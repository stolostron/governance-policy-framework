// Copyright (c) 2022 Red Hat, Inc.
// Copyright Contributors to the Open Cluster Management project

package test

import (
	"context"
	"errors"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/dynamic"
	policiesv1 "open-cluster-management.io/governance-policy-propagator/api/v1"
	"open-cluster-management.io/governance-policy-propagator/test/utils"

	"github.com/stolostron/governance-policy-framework/test/common"
	. "github.com/stolostron/governance-policy-framework/test/common"
)

func ConfigPruneBehavior(labels ...string) bool {
	const (
		pruneConfigMapName string = "test-prune-configmap"
		pruneConfigMapYaml string = "../resources/configuration_policy_prune/configmap-only.yaml"
	)

	cleanPolicy := func(policyName, policyYaml string) func() {
		return func() {
			By("Cleaning up policy " + policyName + ", ignoring if not found")

			outHub, err := OcHub("delete", "-f", policyYaml, "-n", UserNamespace, "--ignore-not-found")
			GinkgoWriter.Printf("cleanPolicy OcHub output: %v\n", outHub)
			Expect(err).ToNot(HaveOccurred())

			Eventually(func(g Gomega) {
				outManaged, err := OcHosting(
					"get", "configurationpolicies", "-A",
				)
				GinkgoWriter.Printf("cleanPolicy OcManaged configurationpolicy output: %v\n", outManaged)
				g.Expect(outManaged).To(BeEmpty())
				g.Expect(err).ToNot(HaveOccurred())
			}, DefaultTimeoutSeconds, 1).Should(Succeed())

			outManaged, err := OcHosting(
				"delete", "events", "-n", ClusterNamespace,
				"--field-selector=involvedObject.name="+UserNamespace+"."+policyName,
				"--ignore-not-found",
			)
			GinkgoWriter.Printf("cleanPolicy OcManaged policy event output: %v\n", outManaged)
			Expect(err).ToNot(HaveOccurred())
			outManaged, err = OcHosting(
				"delete", "events", "-n", ClusterNamespace,
				"--field-selector=involvedObject.name="+policyName,
				"--ignore-not-found",
			)
			GinkgoWriter.Printf("cleanPolicy OcManaged configurationpolicy event output: %v\n", outManaged)
			Expect(err).ToNot(HaveOccurred())
		}
	}

	pruneTestCreatedByPolicy := func(ctx context.Context, policyName, policyYaml string, cmShouldBeDeleted bool) {
		clientManagedDynamic := NewKubeClientDynamic("", KubeconfigManaged, "")

		var clientHostingDynamic dynamic.Interface

		if common.IsHosted {
			clientHostingDynamic = NewKubeClientDynamic("", KubeconfigHub, "")
		} else {
			clientHostingDynamic = NewKubeClientDynamic("", KubeconfigManaged, "")
		}

		DoCreatePolicyTest(ctx, policyYaml, GvrConfigurationPolicy)

		DoRootComplianceTest(policyName, policiesv1.Compliant)

		By("Checking if the status of ConfigurationPolicy " + policyName + " is Compliant")
		Eventually(func() string {
			cfgPol := utils.GetWithTimeout(clientHostingDynamic, GvrConfigurationPolicy,
				policyName, ClusterNamespace, true, DefaultTimeoutSeconds)

			compliant, _, _ := unstructured.NestedString(cfgPol.Object, "status", "compliant")

			return compliant
		}, DefaultTimeoutSeconds, 1).Should(Equal(string(policiesv1.Compliant)))

		if cmShouldBeDeleted {
			By("Checking that the ConfigurationPolicy has a finalizer")
			Eventually(func() []string {
				cfgPol := utils.GetWithTimeout(clientHostingDynamic, GvrConfigurationPolicy,
					policyName, ClusterNamespace, true, DefaultTimeoutSeconds)

				return cfgPol.GetFinalizers()
			}, 30, 5).ShouldNot(BeEmpty())
		}

		By("Checking that the configmap was created")
		utils.GetWithTimeout(
			clientManagedDynamic,
			GvrConfigMap,
			pruneConfigMapName,
			"default",
			true,
			DefaultTimeoutSeconds,
		)

		By("Checking that the ConfigurationPolicy identified that it created the object")
		Eventually(func(g Gomega) interface{} {
			cfgPol := utils.GetWithTimeout(clientHostingDynamic, GvrConfigurationPolicy,
				policyName, ClusterNamespace, true, DefaultTimeoutSeconds)

			relObj, _, err := unstructured.NestedSlice(cfgPol.Object, "status", "relatedObjects")
			g.Expect(err).ToNot(HaveOccurred())
			g.Expect(relObj).ToNot(BeEmpty())

			createdByPolicy, _, err := unstructured.NestedBool(
				relObj[0].(map[string]interface{}), "properties", "createdByPolicy")
			g.Expect(err).ToNot(HaveOccurred())

			return createdByPolicy
		}, DefaultTimeoutSeconds, 5).Should(BeTrue(), "createdByPolicy should be true")

		DoCleanupPolicy(policyYaml, GvrConfigurationPolicy)

		By("Checking if the configmap was deleted")
		utils.GetWithTimeout(
			clientManagedDynamic,
			GvrConfigMap,
			pruneConfigMapName,
			"default",
			!cmShouldBeDeleted,
			DefaultTimeoutSeconds,
		)
	}

	pruneTestForegroundDeletion := func(ctx context.Context, policyName, policyYaml string) {
		clientManagedDynamic := NewKubeClientDynamic("", KubeconfigManaged, "")
		clientHubDynamic := NewKubeClientDynamic("", KubeconfigHub, "")

		var clientHostingDynamic dynamic.Interface

		if common.IsHosted {
			clientHostingDynamic = NewKubeClientDynamic("", KubeconfigHub, "")
		} else {
			clientHostingDynamic = NewKubeClientDynamic("", KubeconfigManaged, "")
		}

		DoCreatePolicyTest(ctx, policyYaml, GvrConfigurationPolicy)

		DoRootComplianceTest(policyName, policiesv1.Compliant)

		By("Checking if the status of ConfigurationPolicy " + policyName + " is Compliant")
		Eventually(func() string {
			cfgPol := utils.GetWithTimeout(clientHostingDynamic, GvrConfigurationPolicy,
				policyName, ClusterNamespace, true, DefaultTimeoutSeconds)

			compliant, _, _ := unstructured.NestedString(cfgPol.Object, "status", "compliant")

			return compliant
		}, DefaultTimeoutSeconds, 1).Should(Equal(string(policiesv1.Compliant)))

		By("Checking that the ConfigurationPolicy has a finalizer")
		Eventually(func() []string {
			cfgPol := utils.GetWithTimeout(clientHostingDynamic, GvrConfigurationPolicy,
				policyName, ClusterNamespace, true, DefaultTimeoutSeconds)

			return cfgPol.GetFinalizers()
		}, 30, 5).ShouldNot(BeEmpty())

		By("Checking that the configmap was created")
		utils.GetWithTimeout(
			clientManagedDynamic,
			GvrConfigMap,
			pruneConfigMapName,
			"default",
			true,
			DefaultTimeoutSeconds,
		)

		By("Applying a finalizer to the configmap")

		_, err := OcManaged(
			"patch",
			"configmap",
			pruneConfigMapName,
			"-n", "default",
			"--type=json", "-p",
			`[{"op":"add", "path":"/metadata/finalizers", `+
				`"value":["test.open-cluster-management.io/prunetest"]}]`,
		)
		Expect(err).ToNot(HaveOccurred())

		By("Deleting the root policy")

		_, err = OcHub(
			"delete", "-f", policyYaml,
			"-n", UserNamespace,
			"--ignore-not-found",
		)
		Expect(err).ToNot(HaveOccurred())

		utils.GetWithTimeout(
			clientHubDynamic,
			GvrPolicy,
			policyName,
			UserNamespace,
			false,
			DefaultTimeoutSeconds,
		)

		// In the future, we might check the replicated Policy on the hub or managed cluster,
		// but for now we only ensure the ConfigurationPolicy remains while things are deleting.

		By("Checking that the ConfigurationPolicy is still on the cluster")
		Consistently(func() interface{} {
			return utils.GetWithTimeout(clientHostingDynamic, GvrConfigurationPolicy, policyName,
				ClusterNamespace, true, DefaultTimeoutSeconds)
		}, 30, 5).ShouldNot(BeNil())

		By("Removing any finalizers from the configmap")

		_, err = OcManaged("patch", "configmap", pruneConfigMapName, "-n", "default",
			"--type=json", "-p", `[{"op":"remove", "path":"/metadata/finalizers"}]`)
		Expect(err).ToNot(HaveOccurred())

		By("Checking that the configmap is deleted")
		utils.GetWithTimeout(
			clientManagedDynamic,
			GvrConfigMap,
			pruneConfigMapName,
			"default",
			false,
			DefaultTimeoutSeconds,
		)

		By("Checking that the ConfigurationPolicy is now cleaned up")
		utils.GetWithTimeout(
			clientHostingDynamic,
			GvrConfigurationPolicy,
			policyName,
			ClusterNamespace,
			false,
			DefaultTimeoutSeconds,
		)
	}

	pruneTestInformPolicy := func(ctx context.Context, policyName, policyYaml string, cmShouldBeDeleted bool) {
		clientManagedDynamic := NewKubeClientDynamic("", KubeconfigManaged, "")

		var clientHostingDynamic dynamic.Interface

		if common.IsHosted {
			clientHostingDynamic = NewKubeClientDynamic("", KubeconfigHub, "")
		} else {
			clientHostingDynamic = NewKubeClientDynamic("", KubeconfigManaged, "")
		}

		DoCreatePolicyTest(ctx, policyYaml, GvrConfigurationPolicy)

		DoRootComplianceTest(policyName, policiesv1.Compliant)

		By("Checking if the status of ConfigurationPolicy " + policyName + " is Compliant")
		Eventually(func() string {
			cfgPol := utils.GetWithTimeout(clientHostingDynamic, GvrConfigurationPolicy,
				policyName, ClusterNamespace, true, DefaultTimeoutSeconds)

			compliant, _, _ := unstructured.NestedString(cfgPol.Object, "status", "compliant")

			return compliant
		}, DefaultTimeoutSeconds, 1).Should(Equal(string(policiesv1.Compliant)))

		if cmShouldBeDeleted {
			By("Checking that the ConfigurationPolicy has a finalizer")
			Eventually(func() []string {
				cfgPol := utils.GetWithTimeout(clientHostingDynamic, GvrConfigurationPolicy,
					policyName, ClusterNamespace, true, DefaultTimeoutSeconds)

				return cfgPol.GetFinalizers()
			}, 30, 5).ShouldNot(BeEmpty())
		}

		By("Checking that the configmap was created")
		utils.GetWithTimeout(
			clientManagedDynamic,
			GvrConfigMap,
			pruneConfigMapName,
			"default", true,
			DefaultTimeoutSeconds,
		)

		By("Changing the policy to inform")

		_, err := OcHub(
			"patch",
			"policies.policy.open-cluster-management.io",
			policyName, "-n", UserNamespace,
			"--type=json", "-p",
			`[{"op":"replace", "path":"/spec/remediationAction", "value":"inform"}]`,
		)
		Expect(err).ToNot(HaveOccurred())
		By("Wait for configpolicy to update to inform")
		Eventually(func() interface{} {
			configpol := utils.GetWithTimeout(
				clientHostingDynamic,
				GvrConfigurationPolicy,
				policyName,
				ClusterNamespace,
				true,
				DefaultTimeoutSeconds,
			)
			if configpol == nil {
				return errors.New("could not get configuration policy")
			}

			remAction, _, _ := unstructured.NestedString(configpol.Object, "spec", "remediationAction")

			return remAction
		}, DefaultTimeoutSeconds, 1).Should(MatchRegexp(".nform"))

		DoCleanupPolicy(policyYaml, GvrConfigurationPolicy)

		By("Checking if the configmap was deleted")
		utils.GetWithTimeout(
			clientManagedDynamic,
			GvrConfigMap,
			pruneConfigMapName,
			"default",
			!cmShouldBeDeleted,
			DefaultTimeoutSeconds,
		)
	}

	pruneTestEditedByPolicy := func(ctx context.Context, policyName, policyYaml string, cmShouldBeDeleted bool) {
		clientManagedDynamic := NewKubeClientDynamic("", KubeconfigManaged, "")

		var clientHostingDynamic dynamic.Interface

		if common.IsHosted {
			clientHostingDynamic = NewKubeClientDynamic("", KubeconfigHub, "")
		} else {
			clientHostingDynamic = NewKubeClientDynamic("", KubeconfigManaged, "")
		}

		By("Creating the configmap before the policy")

		_, err := OcManaged("apply", "-f", pruneConfigMapYaml, "-n", "default")
		Expect(err).ToNot(HaveOccurred())
		By("Checking the configmap's initial data")

		var initialValue string

		Eventually(func(g Gomega) {
			cm := utils.GetWithTimeout(
				clientManagedDynamic,
				GvrConfigMap,
				pruneConfigMapName,
				"default",
				true,
				DefaultTimeoutSeconds,
			)
			data, ok, err := unstructured.NestedMap(cm.Object, "data")
			g.Expect(ok).To(BeTrue())
			g.Expect(err).ToNot(HaveOccurred())

			initialValue, ok = data["testvalue"].(string)
			g.Expect(ok).To(BeTrue())
			g.Expect(initialValue).ToNot(BeEmpty())
		}, DefaultTimeoutSeconds, 1).Should(Succeed())

		DoCreatePolicyTest(ctx, policyYaml, GvrConfigurationPolicy)

		DoRootComplianceTest(policyName, policiesv1.Compliant)

		By("Checking if the status of ConfigurationPolicy " + policyName + " is Compliant")
		Eventually(func() string {
			cfgPol := utils.GetWithTimeout(clientHostingDynamic, GvrConfigurationPolicy,
				policyName, ClusterNamespace, true, DefaultTimeoutSeconds)

			compliant, _, _ := unstructured.NestedString(cfgPol.Object, "status", "compliant")

			return compliant
		}, DefaultTimeoutSeconds, 1).Should(Equal(string(policiesv1.Compliant)))

		if cmShouldBeDeleted {
			By("Checking that the ConfigurationPolicy has a finalizer")
			Eventually(func() []string {
				cfgPol := utils.GetWithTimeout(clientHostingDynamic, GvrConfigurationPolicy,
					policyName, ClusterNamespace, true, DefaultTimeoutSeconds)

				return cfgPol.GetFinalizers()
			}, 30, 5).ShouldNot(BeEmpty())
		}

		By("Checking the configmap's data was updated")
		Eventually(func(g Gomega) {
			cm := utils.GetWithTimeout(
				clientManagedDynamic,
				GvrConfigMap,
				pruneConfigMapName,
				"default",
				true,
				DefaultTimeoutSeconds,
			)
			data, ok, err := unstructured.NestedMap(cm.Object, "data")
			g.Expect(ok).To(BeTrue())
			g.Expect(err).ToNot(HaveOccurred())

			newValue, ok := data["testvalue"].(string)
			g.Expect(ok).To(BeTrue())
			g.Expect(newValue).To(Not(Equal(initialValue)))
		}, DefaultTimeoutSeconds, 1).Should(Succeed())

		DoCleanupPolicy(policyYaml, GvrConfigurationPolicy)

		By("Checking if the configmap was deleted")
		utils.GetWithTimeout(
			clientManagedDynamic,
			GvrConfigMap,
			pruneConfigMapName,
			"default",
			!cmShouldBeDeleted,
			DefaultTimeoutSeconds,
		)
	}

	Describe("GRC: [P1][Sev1][policy-grc] "+
		"Test configuration policy pruning", Ordered, Label(labels...), func() {
		cleanConfigMap := func() {
			By("Removing any finalizers from the configmap")

			_, _ = OcManaged("patch", "configmap", pruneConfigMapName, "-n", "default",
				"--type=json", "-p", `[{"op":"remove", "path":"/metadata/finalizers"}]`)

			By("Deleting the configmap")

			_, err := OcManaged(
				"delete", "-f", pruneConfigMapYaml,
				"--ignore-not-found", "--timeout=30s",
			)
			Expect(err).ToNot(HaveOccurred())
		}
		BeforeEach(cleanConfigMap)
		AfterAll(cleanConfigMap)

		Describe("Test DeleteAll pruning", func() {
			policyName := "cm-policy-prune-all"
			policyYaml := "../resources/configuration_policy_prune/cm-policy-prune-all.yaml"

			BeforeEach(cleanPolicy(policyName, policyYaml))
			AfterAll(cleanPolicy(policyName, policyYaml))

			It("Should delete the configmap created by a DeleteAll policy when the policy is deleted",
				func(ctx SpecContext) {
					pruneTestCreatedByPolicy(ctx, policyName, policyYaml, true)
				})
			It("Should not remove the ConfigurationPolicy while a relatedObject is terminating",
				func(ctx SpecContext) {
					pruneTestForegroundDeletion(ctx, policyName, policyYaml)
				})
			It("Should not delete the configmap when the policy is in inform mode",
				func(ctx SpecContext) {
					pruneTestInformPolicy(ctx, policyName, policyYaml, false)
				})
			It("Should delete the configmap edited by a DeleteAll policy when the policy is deleted",
				func(ctx SpecContext) {
					pruneTestEditedByPolicy(ctx, policyName, policyYaml, true)
				})
		})

		Describe("Test DeleteIfCreated pruning", func() {
			policyName := "cm-policy-prune-if-created"
			policyYaml := "../resources/configuration_policy_prune/cm-policy-prune-if-created.yaml"

			BeforeEach(cleanPolicy(policyName, policyYaml))
			AfterAll(cleanPolicy(policyName, policyYaml))

			It("Should delete the configmap created by "+
				"a DeleteIfCreated policy when the policy is deleted", func(ctx SpecContext) {
				pruneTestCreatedByPolicy(ctx, policyName, policyYaml, true)
			})
			It("Should not delete the configmap edited by "+
				"a DeleteIfCreated policy when the policy is deleted", func(ctx SpecContext) {
				pruneTestEditedByPolicy(ctx, policyName, policyYaml, false)
			})
		})

		Describe("Test None pruning", func() {
			policyName := "cm-policy-prune-none"
			policyYaml := "../resources/configuration_policy_prune/cm-policy-prune-none.yaml"

			BeforeEach(cleanPolicy(policyName, policyYaml))
			AfterAll(cleanPolicy(policyName, policyYaml))

			It("Should not delete the configmap created by a None policy when the policy is deleted",
				func(ctx SpecContext) {
					pruneTestCreatedByPolicy(ctx, policyName, policyYaml, false)
				})
			It("Should not delete the configmap edited by a None policy when the policy is deleted",
				func(ctx SpecContext) {
					pruneTestEditedByPolicy(ctx, policyName, policyYaml, false)
				})
		})

		Describe("Test default pruning", func() {
			policyName := "cm-policy-prune-default"
			policyYaml := "../resources/configuration_policy_prune/cm-policy-prune-default.yaml"

			BeforeEach(cleanPolicy(policyName, policyYaml))
			AfterAll(cleanPolicy(policyName, policyYaml))

			It("Should not delete the configmap created by a policy that "+
				"doesn't specify a Prune behavior when the policy is deleted", func(ctx SpecContext) {
				pruneTestCreatedByPolicy(ctx, policyName, policyYaml, false)
			})
			It("Should not delete the configmap edited by a policy that "+
				"doesn't specify a Prune behavior when the policy is deleted", func(ctx SpecContext) {
				pruneTestEditedByPolicy(ctx, policyName, policyYaml, false)
			})
		})
	})

	return true
}
