// Copyright (c) 2020 Red Hat, Inc.

package e2e

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/open-cluster-management/governance-policy-propagator/test/utils"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("Test configuration policy", func() {
	Describe("Test object musthave inform", func() {
		const rolePolicyName string = "role-policy-musthave"
		const rolePolicyYaml string = "../resources/configuration_policy/role-policy-musthave.yaml"
		It("should be created on managed cluster", func() {
			By("Creating " + rolePolicyYaml)
			utils.Kubectl("apply", "-f", rolePolicyYaml, "-n", userNamespace, "--kubeconfig=../../kubeconfig_hub")
			hubPlc := utils.GetWithTimeout(clientHubDynamic, gvrPolicy, rolePolicyName, userNamespace, true, defaultTimeoutSeconds)
			Expect(hubPlc).NotTo(BeNil())
			By("Patching " + rolePolicyName + "-plr with decision of cluster managed")
			plr := utils.GetWithTimeout(clientHubDynamic, gvrPlacementRule, rolePolicyName+"-plr", userNamespace, true, defaultTimeoutSeconds)
			plr.Object["status"] = utils.GeneratePlrStatus("managed")
			plr, err := clientHubDynamic.Resource(gvrPlacementRule).Namespace(userNamespace).UpdateStatus(plr, metav1.UpdateOptions{})
			Expect(err).To(BeNil())
			By("Checking " + rolePolicyName + " on managed cluster in ns " + clusterNamespace)
			managedplc := utils.GetWithTimeout(clientManagedDynamic, gvrPolicy, userNamespace+"."+rolePolicyName, clusterNamespace, true, defaultTimeoutSeconds)
			Expect(managedplc).NotTo(BeNil())
		})
		It("the policy should be noncompliant", func() {
			By("Checking if the status of root policy is noncompliant")
			yamlPlc := utils.ParseYaml("../resources/configuration_policy/" + rolePolicyName + "-noncompliant.yaml")
			Eventually(func() interface{} {
				rootPlc := utils.GetWithTimeout(clientHubDynamic, gvrPolicy, rolePolicyName, userNamespace, true, defaultTimeoutSeconds)
				return rootPlc.Object["status"]
			}, defaultTimeoutSeconds, 1).Should(utils.SemanticEqual(yamlPlc.Object["status"]))
		})
		It("the policy should be compliant after manualy creating the role on managed cluster", func() {
			By("Creating the role in default namespace on managed cluster")
			utils.Kubectl("apply", "-f", "../resources/configuration_policy/role-policy-e2e.yaml", "-n", "default", "--kubeconfig=../../kubeconfig_managed")
			By("Checking if the status of root policy is compliant")
			yamlPlc := utils.ParseYaml("../resources/configuration_policy/" + rolePolicyName + "-compliant.yaml")
			Eventually(func() interface{} {
				rootPlc := utils.GetWithTimeout(clientHubDynamic, gvrPolicy, rolePolicyName, userNamespace, true, defaultTimeoutSeconds)
				return rootPlc.Object["status"]
			}, defaultTimeoutSeconds, 1).Should(utils.SemanticEqual(yamlPlc.Object["status"]))
		})
		It("the policy should be noncompliant after removing the role", func() {
			By("Deleting the role in default namespace on managed cluster")
			utils.Kubectl("delete", "-f", "../resources/configuration_policy/role-policy-e2e.yaml", "-n", "default", "--kubeconfig=../../kubeconfig_managed")
			By("Checking if the status of root policy is noncompliant")
			yamlPlc := utils.ParseYaml("../resources/configuration_policy/" + rolePolicyName + "-noncompliant.yaml")
			Eventually(func() interface{} {
				rootPlc := utils.GetWithTimeout(clientHubDynamic, gvrPolicy, rolePolicyName, userNamespace, true, defaultTimeoutSeconds)
				return rootPlc.Object["status"]
			}, defaultTimeoutSeconds, 1).Should(utils.SemanticEqual(yamlPlc.Object["status"]))
		})
		It("should clean up", func() {
			By("Deleting " + rolePolicyYaml)
			utils.Kubectl("delete", "-f", rolePolicyYaml, "-n", userNamespace, "--kubeconfig=../../kubeconfig_hub")
			By("Checking if there is any policy left")
			utils.ListWithTimeout(clientHubDynamic, gvrPolicy, metav1.ListOptions{}, 0, true, defaultTimeoutSeconds)
			utils.ListWithTimeout(clientManagedDynamic, gvrPolicy, metav1.ListOptions{}, 0, true, defaultTimeoutSeconds)
			By("Checking if there is any configuration policy left")
			utils.ListWithTimeout(clientManagedDynamic, gvrConfigurationPolicy, metav1.ListOptions{}, 0, true, defaultTimeoutSeconds)
		})
	})
	Describe("Test object musthave enforce", func() {
		const rolePolicyName string = "role-policy-musthave"
		const rolePolicyYaml string = "../resources/configuration_policy/role-policy-musthave.yaml"
		It("should be created on managed cluster", func() {
			By("Creating " + rolePolicyYaml)
			utils.Kubectl("apply", "-f", rolePolicyYaml, "-n", userNamespace, "--kubeconfig=../../kubeconfig_hub")
			hubPlc := utils.GetWithTimeout(clientHubDynamic, gvrPolicy, rolePolicyName, userNamespace, true, defaultTimeoutSeconds)
			Expect(hubPlc).NotTo(BeNil())
			By("Patching " + rolePolicyName + "-plr with decision of cluster managed")
			plr := utils.GetWithTimeout(clientHubDynamic, gvrPlacementRule, rolePolicyName+"-plr", userNamespace, true, defaultTimeoutSeconds)
			plr.Object["status"] = utils.GeneratePlrStatus("managed")
			plr, err := clientHubDynamic.Resource(gvrPlacementRule).Namespace(userNamespace).UpdateStatus(plr, metav1.UpdateOptions{})
			Expect(err).To(BeNil())
			By("Checking " + rolePolicyName + " on managed cluster in ns " + clusterNamespace)
			managedplc := utils.GetWithTimeout(clientManagedDynamic, gvrPolicy, userNamespace+"."+rolePolicyName, clusterNamespace, true, defaultTimeoutSeconds)
			Expect(managedplc).NotTo(BeNil())
		})
		It("the policy should be noncompliant", func() {
			By("Checking if the status of root policy is noncompliant")
			yamlPlc := utils.ParseYaml("../resources/configuration_policy/" + rolePolicyName + "-noncompliant.yaml")
			Eventually(func() interface{} {
				rootPlc := utils.GetWithTimeout(clientHubDynamic, gvrPolicy, rolePolicyName, userNamespace, true, defaultTimeoutSeconds)
				return rootPlc.Object["status"]
			}, defaultTimeoutSeconds, 1).Should(utils.SemanticEqual(yamlPlc.Object["status"]))
		})
		It("the policy should be compliant after enforcing it", func() {
			By("Patching remediationAction = enforce on root policy")
			rootPlc := utils.GetWithTimeout(clientHubDynamic, gvrPolicy, rolePolicyName, userNamespace, true, defaultTimeoutSeconds)
			rootPlc.Object["spec"].(map[string]interface{})["remediationAction"] = "enforce"
			rootPlc, _ = clientHubDynamic.Resource(gvrPolicy).Namespace(userNamespace).Update(rootPlc, metav1.UpdateOptions{})
			Expect(rootPlc.Object["spec"].(map[string]interface{})["remediationAction"]).To(Equal("enforce"))
			By("Checking if the status of root policy is compliant")
			yamlPlc := utils.ParseYaml("../resources/configuration_policy/" + rolePolicyName + "-compliant.yaml")
			Eventually(func() interface{} {
				rootPlc := utils.GetWithTimeout(clientHubDynamic, gvrPolicy, rolePolicyName, userNamespace, true, defaultTimeoutSeconds)
				return rootPlc.Object["status"]
			}, defaultTimeoutSeconds, 1).Should(utils.SemanticEqual(yamlPlc.Object["status"]))
		})
		It("should recreate the role if manually deleted", func() {
			By("Deleting the role in default namespace on managed cluster")
			utils.Kubectl("delete", "role", "-n", "default", "--all", "--kubeconfig=../../kubeconfig_managed")
			By("Checking if the role has been deleted")
			Eventually(func() interface{} {
				roleList, err := clientManagedDynamic.Resource(gvrRole).Namespace("default").List(metav1.ListOptions{})
				Expect(err).To(BeNil())
				return len(roleList.Items)
			}, defaultTimeoutSeconds, 1).Should(Equal(0))
			By("Checking if the role has been recreated")
			Eventually(func() interface{} {
				roleList, err := clientManagedDynamic.Resource(gvrRole).Namespace("default").List(metav1.ListOptions{})
				Expect(err).To(BeNil())
				return len(roleList.Items)
			}, defaultTimeoutSeconds, 1).Should(Equal(1))
			By("Checking if the status of root policy is still compliant")
			yamlPlc := utils.ParseYaml("../resources/configuration_policy/" + rolePolicyName + "-compliant.yaml")
			Eventually(func() interface{} {
				rootPlc := utils.GetWithTimeout(clientHubDynamic, gvrPolicy, rolePolicyName, userNamespace, true, defaultTimeoutSeconds)
				return rootPlc.Object["status"]
			}, defaultTimeoutSeconds, 1).Should(utils.SemanticEqual(yamlPlc.Object["status"]))
		})
		It("should clean up", func() {
			By("Deleting " + rolePolicyYaml)
			utils.Kubectl("delete", "-f", rolePolicyYaml, "-n", userNamespace, "--kubeconfig=../../kubeconfig_hub")
			By("Checking if there is any policy left")
			utils.ListWithTimeout(clientHubDynamic, gvrPolicy, metav1.ListOptions{}, 0, true, defaultTimeoutSeconds)
			utils.ListWithTimeout(clientManagedDynamic, gvrPolicy, metav1.ListOptions{}, 0, true, defaultTimeoutSeconds)
			By("Checking if there is any configuration policy left")
			utils.ListWithTimeout(clientManagedDynamic, gvrConfigurationPolicy, metav1.ListOptions{}, 0, true, defaultTimeoutSeconds)
			By("Deleting the role in default namespace on managed cluster")
			utils.Kubectl("delete", "-f", "../resources/configuration_policy/role-policy-e2e.yaml", "-n", "default", "--kubeconfig=../../kubeconfig_managed")
		})
	})
})
