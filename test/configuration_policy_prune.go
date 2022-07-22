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

const (
	pruneConfigMapName string = "test-prune-configmap"
	pruneConfigMapYaml string = "../resources/configuration_policy_prune/configmap-only.yaml"
)

func ConfigPruneBehavior(labels ...string) bool {
	pruneTestCreatedByPolicy := func(policyName, policyYaml string, cmShouldBeDeleted bool) {
		clientManagedDynamic := NewKubeClientDynamic("", KubeconfigManaged, "")
		clientHubDynamic := NewKubeClientDynamic("", KubeconfigHub, "")

		DoCreatePolicyTest(clientHubDynamic, clientManagedDynamic, policyYaml, &GvrConfigurationPolicy)

		DoRootComplianceTest(clientHubDynamic, policyName, policiesv1.Compliant)

		By("Checking that the configmap was created")
		utils.GetWithTimeout(clientManagedDynamic, GvrConfigMap, pruneConfigMapName, "default", true, DefaultTimeoutSeconds)

		DoCleanupPolicy(clientHubDynamic, clientManagedDynamic, policyYaml, &GvrConfigurationPolicy)

		By("Checking if the configmap was deleted")
		utils.GetWithTimeout(clientManagedDynamic, GvrConfigMap, pruneConfigMapName, "default", !cmShouldBeDeleted, DefaultTimeoutSeconds)
	}

	pruneTestInformPolicy := func(policyName, policyYaml string, cmShouldBeDeleted bool) {
		clientManagedDynamic := NewKubeClientDynamic("", KubeconfigManaged, "")
		clientHubDynamic := NewKubeClientDynamic("", KubeconfigHub, "")

		DoCreatePolicyTest(clientHubDynamic, clientManagedDynamic, policyYaml, &GvrConfigurationPolicy)

		DoRootComplianceTest(clientHubDynamic, policyName, policiesv1.Compliant)

		By("Checking that the configmap was created")
		utils.GetWithTimeout(clientManagedDynamic, GvrConfigMap, pruneConfigMapName, "default", true, DefaultTimeoutSeconds)

		By("Changing the policy to inform")
		OcHub("patch", "policies.policy.open-cluster-management.io", policyName, "-n", UserNamespace,
			"--type=json", "-p", `[{"op":"replace", "path":"/spec/remediationAction", "value":"inform"}]`)

		By("Wait for configpolicy to update to inform")
		Eventually(func() interface{} {
			configpol := utils.GetWithTimeout(clientManagedDynamic, GvrConfigurationPolicy, policyName, ClusterNamespace, true, DefaultTimeoutSeconds)
			if configpol == nil {
				return errors.New("could not get configuration policy")
			}

			remAction, _, _ := unstructured.NestedString(configpol.Object, "spec", "remediationAction")
			return remAction
		}, DefaultTimeoutSeconds, 1).Should(MatchRegexp(".nform"))

		DoCleanupPolicy(clientHubDynamic, clientManagedDynamic, policyYaml, &GvrConfigurationPolicy)

		By("Checking if the configmap was deleted")
		utils.GetWithTimeout(clientManagedDynamic, GvrConfigMap, pruneConfigMapName, "default", !cmShouldBeDeleted, DefaultTimeoutSeconds)
	}

	pruneTestEditedByPolicy := func(policyName, policyYaml string, cmShouldBeDeleted bool) {
		clientManagedDynamic := NewKubeClientDynamic("", KubeconfigManaged, "")
		clientHubDynamic := NewKubeClientDynamic("", KubeconfigHub, "")

		By("Creating the configmap before the policy")
		OcManaged("apply", "-f", pruneConfigMapYaml)

		By("Checking the configmap's initial data")
		var initialValue string
		Eventually(func(g Gomega) {
			cm := utils.GetWithTimeout(clientManagedDynamic, GvrConfigMap, pruneConfigMapName, "default", true, DefaultTimeoutSeconds)
			data, ok, err := unstructured.NestedMap(cm.Object, "data")
			g.Expect(ok).To(BeTrue())
			g.Expect(err).To(BeNil())

			initialValue, ok = data["testvalue"].(string)
			g.Expect(ok).To(BeTrue())
			g.Expect(len(initialValue)).To(BeNumerically(">", 0))
		}, DefaultTimeoutSeconds, 1)

		DoCreatePolicyTest(clientHubDynamic, clientManagedDynamic, policyYaml, &GvrConfigurationPolicy)

		DoRootComplianceTest(clientHubDynamic, policyName, policiesv1.Compliant)

		By("Checking the configmap's data was updated")
		Eventually(func(g Gomega) {
			cm := utils.GetWithTimeout(clientManagedDynamic, GvrConfigMap, pruneConfigMapName, "default", true, DefaultTimeoutSeconds)
			data, ok, err := unstructured.NestedMap(cm.Object, "data")
			g.Expect(ok).To(BeTrue())
			g.Expect(err).To(BeNil())

			newValue, ok := data["testvalue"].(string)
			g.Expect(ok).To(BeTrue())
			g.Expect(newValue).To(Not(Equal(initialValue)))
		}, DefaultTimeoutSeconds, 1)

		DoCleanupPolicy(clientHubDynamic, clientManagedDynamic, policyYaml, &GvrConfigurationPolicy)

		By("Checking if the configmap was deleted")
		utils.GetWithTimeout(clientManagedDynamic, GvrConfigMap, pruneConfigMapName, "default", !cmShouldBeDeleted, DefaultTimeoutSeconds)
	}

	Describe("GRC: [P1][Sev1][policy-grc] Test configuration policy pruning", Ordered, Label(labels...), func() {
		cleanConfigMap := func() {
			OcManaged("delete", "-f", pruneConfigMapYaml)
		}
		BeforeEach(cleanConfigMap)
		AfterAll(cleanConfigMap)

		cleanPolicy := func(policyName, policyYaml string) func() {
			return func() {
				OcHub("delete", "-f", policyYaml)
				OcManaged("delete", "events", "-n", ClusterNamespace, "--field-selector=involvedObject.name="+UserNamespace+"."+policyName)
			}
		}

		Describe("Test DeleteAll pruning", func() {
			policyName := "cm-policy-prune-all"
			policyYaml := "../resources/configuration_policy_prune/cm-policy-prune-all.yaml"

			BeforeEach(cleanPolicy(policyName, policyYaml))
			AfterAll(cleanPolicy(policyName, policyYaml))

			It("Should delete the configmap created by a DeleteAll policy when the policy is deleted", func() {
				pruneTestCreatedByPolicy(policyName, policyYaml, true)
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

			It("Should delete the configmap created by a DeleteIfCreated policy when the policy is deleted", func() {
				pruneTestCreatedByPolicy(policyName, policyYaml, true)
			})
			It("Should not delete the configmap edited by a DeleteIfCreated policy when the policy is deleted", func() {
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

			It("Should not delete the configmap created by a policy that doesn't specify a Prune behavior when the policy is deleted", func() {
				pruneTestCreatedByPolicy(policyName, policyYaml, false)
			})
			It("Should not delete the configmap edited by a policy that doesn't specify a Prune behavior when the policy is deleted", func() {
				pruneTestEditedByPolicy(policyName, policyYaml, false)
			})
		})
	})

	return true
}
