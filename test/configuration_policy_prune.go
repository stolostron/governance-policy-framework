// Copyright (c) 2022 Red Hat, Inc.
// Copyright Contributors to the Open Cluster Management project

package test

import (
	"errors"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/stolostron/governance-policy-framework/test/common"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	policiesv1 "open-cluster-management.io/governance-policy-propagator/api/v1"
	"open-cluster-management.io/governance-policy-propagator/test/utils"
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
			Expect(err).To(BeNil())

			outManaged, err := OcManaged(
				"delete", "events", "-n", ClusterNamespace,
				"--field-selector=involvedObject.name="+UserNamespace+"."+policyName,
				"--ignore-not-found",
			)
			GinkgoWriter.Printf("cleanPolicy OcManaged output: %v\n", outManaged)
			Expect(err).To(BeNil())
		}
	}

	pruneTestCreatedByPolicy := func(policyName, policyYaml string, cmShouldBeDeleted bool) {
		clientManagedDynamic := NewKubeClientDynamic("", KubeconfigManaged, "")

		DoCreatePolicyTest(policyYaml, GvrConfigurationPolicy)

		DoRootComplianceTest(policyName, policiesv1.Compliant)

		By("Checking that the configmap was created")
		utils.GetWithTimeout(
			clientManagedDynamic,
			GvrConfigMap,
			pruneConfigMapName,
			"default",
			true,
			DefaultTimeoutSeconds,
		)

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

	pruneTestForegroundDeletion := func(policyName, policyYaml string) {
		clientManagedDynamic := NewKubeClientDynamic("", KubeconfigManaged, "")
		clientHubDynamic := NewKubeClientDynamic("", KubeconfigHub, "")

		DoCreatePolicyTest(policyYaml, GvrConfigurationPolicy)

		DoRootComplianceTest(policyName, policiesv1.Compliant)

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
		Expect(err).To(BeNil())

		By("Deleting the root policy")

		_, err = OcHub(
			"delete", "-f", policyYaml,
			"-n", UserNamespace,
			"--ignore-not-found",
		)
		Expect(err).To(BeNil())

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
			return utils.GetWithTimeout(clientManagedDynamic, GvrConfigurationPolicy, policyName,
				ClusterNamespace, true, DefaultTimeoutSeconds)
		}, 30, 5).ShouldNot(BeNil())

		By("Removing any finalizers from the configmap")

		_, err = OcManaged("patch", "configmap", pruneConfigMapName, "-n", "default",
			"--type=json", "-p", `[{"op":"remove", "path":"/metadata/finalizers"}]`)
		Expect(err).To(BeNil())

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
			clientManagedDynamic,
			GvrConfigurationPolicy,
			policyName,
			ClusterNamespace,
			false,
			DefaultTimeoutSeconds,
		)
	}

	pruneTestInformPolicy := func(policyName, policyYaml string, cmShouldBeDeleted bool) {
		clientManagedDynamic := NewKubeClientDynamic("", KubeconfigManaged, "")

		DoCreatePolicyTest(policyYaml, GvrConfigurationPolicy)

		DoRootComplianceTest(policyName, policiesv1.Compliant)

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
		Expect(err).To(BeNil())
		By("Wait for configpolicy to update to inform")
		Eventually(func() interface{} {
			configpol := utils.GetWithTimeout(
				clientManagedDynamic,
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

	pruneTestEditedByPolicy := func(policyName, policyYaml string, cmShouldBeDeleted bool) {
		clientManagedDynamic := NewKubeClientDynamic("", KubeconfigManaged, "")

		By("Creating the configmap before the policy")

		_, err := OcManaged("apply", "-f", pruneConfigMapYaml)
		Expect(err).To(BeNil())
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
			g.Expect(err).To(BeNil())

			initialValue, ok = data["testvalue"].(string)
			g.Expect(ok).To(BeTrue())
			g.Expect(len(initialValue)).To(BeNumerically(">", 0))
		}, DefaultTimeoutSeconds, 1).Should(Succeed())

		DoCreatePolicyTest(policyYaml, GvrConfigurationPolicy)

		DoRootComplianceTest(policyName, policiesv1.Compliant)

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
			g.Expect(err).To(BeNil())

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
			_, err := OcManaged(
				"delete", "-f", pruneConfigMapYaml,
				"--ignore-not-found",
			)
			Expect(err).To(BeNil())
		}
		BeforeEach(cleanConfigMap)
		AfterAll(cleanConfigMap)

		Describe("Test DeleteAll pruning", func() {
			policyName := "cm-policy-prune-all"
			policyYaml := "../resources/configuration_policy_prune/cm-policy-prune-all.yaml"

			BeforeEach(cleanPolicy(policyName, policyYaml))
			AfterAll(cleanPolicy(policyName, policyYaml))

			It("Should delete the configmap created by a DeleteAll policy when the policy is deleted", func() {
				pruneTestCreatedByPolicy(policyName, policyYaml, true)
			})
			It("Should not remove the ConfigurationPolicy while a relatedObject is terminating", func() {
				pruneTestForegroundDeletion(policyName, policyYaml)
			})
			It("Should not delete the configmap when the policy is in inform mode", func() {
				pruneTestInformPolicy(policyName, policyYaml, false)
			})
			It("Should delete the configmap edited by a DeleteAll policy when the policy is deleted", func() {
				pruneTestEditedByPolicy(policyName, policyYaml, true)
			})
		})

		Describe("Test DeleteIfCreated pruning", func() {
			policyName := "cm-policy-prune-if-created"
			policyYaml := "../resources/configuration_policy_prune/cm-policy-prune-if-created.yaml"

			BeforeEach(cleanPolicy(policyName, policyYaml))
			AfterAll(cleanPolicy(policyName, policyYaml))

			It("Should delete the configmap created by "+
				"a DeleteIfCreated policy when the policy is deleted", func() {
				pruneTestCreatedByPolicy(policyName, policyYaml, true)
			})
			It("Should not delete the configmap edited by "+
				"a DeleteIfCreated policy when the policy is deleted", func() {
				pruneTestEditedByPolicy(policyName, policyYaml, false)
			})
		})

		Describe("Test None pruning", func() {
			policyName := "cm-policy-prune-none"
			policyYaml := "../resources/configuration_policy_prune/cm-policy-prune-none.yaml"

			BeforeEach(cleanPolicy(policyName, policyYaml))
			AfterAll(cleanPolicy(policyName, policyYaml))

			It("Should not delete the configmap created by a None policy when the policy is deleted", func() {
				pruneTestCreatedByPolicy(policyName, policyYaml, false)
			})
			It("Should not delete the configmap edited by a None policy when the policy is deleted", func() {
				pruneTestEditedByPolicy(policyName, policyYaml, false)
			})
		})

		Describe("Test default pruning", func() {
			policyName := "cm-policy-prune-default"
			policyYaml := "../resources/configuration_policy_prune/cm-policy-prune-default.yaml"

			BeforeEach(cleanPolicy(policyName, policyYaml))
			AfterAll(cleanPolicy(policyName, policyYaml))

			It("Should not delete the configmap created by a policy that "+
				"doesn't specify a Prune behavior when the policy is deleted", func() {
				pruneTestCreatedByPolicy(policyName, policyYaml, false)
			})
			It("Should not delete the configmap edited by a policy that "+
				"doesn't specify a Prune behavior when the policy is deleted", func() {
				pruneTestEditedByPolicy(policyName, policyYaml, false)
			})
		})
	})

	return true
}
