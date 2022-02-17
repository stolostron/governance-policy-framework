// Copyright (c) 2020 Red Hat, Inc.
// Copyright Contributors to the Open Cluster Management project

package e2e

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/stolostron/governance-policy-propagator/test/utils"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/stolostron/governance-policy-framework/test/common"
)

var _ = Describe("Test configuration policy", func() {
	Describe("Test object musthave inform", func() {
		const rolePolicyName string = "role-policy-musthave"
		const rolePolicyYaml string = "../resources/configuration_policy/role-policy-musthave.yaml"
		It("should be created on managed cluster", func() {
			By("Creating " + rolePolicyYaml)
			utils.Kubectl("apply", "-f", rolePolicyYaml, "-n", userNamespace, "--kubeconfig=../../kubeconfig_hub")
			hubPlc := utils.GetWithTimeout(clientHubDynamic, common.GvrPolicy, rolePolicyName, userNamespace, true, defaultTimeoutSeconds)
			Expect(hubPlc).NotTo(BeNil())
			By("Patching " + rolePolicyName + "-plr with decision of cluster managed")
			plr := utils.GetWithTimeout(clientHubDynamic, common.GvrPlacementRule, rolePolicyName+"-plr", userNamespace, true, defaultTimeoutSeconds)
			plr.Object["status"] = utils.GeneratePlrStatus("managed")
			_, err := clientHubDynamic.Resource(common.GvrPlacementRule).Namespace(userNamespace).UpdateStatus(context.TODO(), plr, metav1.UpdateOptions{})
			Expect(err).To(BeNil())
			By("Checking " + rolePolicyName + " on managed cluster in ns " + clusterNamespace)
			managedplc := utils.GetWithTimeout(clientManagedDynamic, common.GvrPolicy, userNamespace+"."+rolePolicyName, clusterNamespace, true, defaultTimeoutSeconds)
			Expect(managedplc).NotTo(BeNil())
		})
		It("the policy should be noncompliant", func() {
			By("Checking if the status of root policy is noncompliant")
			yamlPlc := utils.ParseYaml("../resources/configuration_policy/" + rolePolicyName + "-noncompliant.yaml")
			Eventually(func() interface{} {
				rootPlc := utils.GetWithTimeout(clientHubDynamic, common.GvrPolicy, rolePolicyName, userNamespace, true, defaultTimeoutSeconds)
				return rootPlc.Object["status"]
			}, defaultTimeoutSeconds, 1).Should(utils.SemanticEqual(yamlPlc.Object["status"]))
		})
		It("the policy should be compliant after manually creating the role that matches", func() {
			By("Creating the role in default namespace on managed cluster")
			utils.Kubectl("apply", "-f", "../resources/configuration_policy/role-policy-e2e.yaml", "-n", "default", "--kubeconfig=../../kubeconfig_managed")
			By("Checking if the status of root policy is compliant")
			yamlPlc := utils.ParseYaml("../resources/configuration_policy/" + rolePolicyName + "-compliant.yaml")
			Eventually(func() interface{} {
				rootPlc := utils.GetWithTimeout(clientHubDynamic, common.GvrPolicy, rolePolicyName, userNamespace, true, defaultTimeoutSeconds)
				return rootPlc.Object["status"]
			}, defaultTimeoutSeconds, 1).Should(utils.SemanticEqual(yamlPlc.Object["status"]))
		})
		It("the policy should be noncompliant after removing the role", func() {
			By("Deleting the role in default namespace on managed cluster")
			utils.Kubectl("delete", "-f", "../resources/configuration_policy/role-policy-e2e.yaml", "-n", "default", "--kubeconfig=../../kubeconfig_managed")
			By("Checking if the status of root policy is noncompliant")
			yamlPlc := utils.ParseYaml("../resources/configuration_policy/" + rolePolicyName + "-noncompliant.yaml")
			Eventually(func() interface{} {
				rootPlc := utils.GetWithTimeout(clientHubDynamic, common.GvrPolicy, rolePolicyName, userNamespace, true, defaultTimeoutSeconds)
				return rootPlc.Object["status"]
			}, defaultTimeoutSeconds, 1).Should(utils.SemanticEqual(yamlPlc.Object["status"]))
		})
		It("the policy should be compliant after manually creating a role that more", func() {
			By("Creating the role in default namespace on managed cluster")
			utils.Kubectl("apply", "-f", "../resources/configuration_policy/role-policy-e2e-more.yaml", "-n", "default", "--kubeconfig=../../kubeconfig_managed")
			By("Checking if the status of root policy is compliant")
			yamlPlc := utils.ParseYaml("../resources/configuration_policy/" + rolePolicyName + "-compliant.yaml")
			Eventually(func() interface{} {
				rootPlc := utils.GetWithTimeout(clientHubDynamic, common.GvrPolicy, rolePolicyName, userNamespace, true, defaultTimeoutSeconds)
				return rootPlc.Object["status"]
			}, defaultTimeoutSeconds, 1).Should(utils.SemanticEqual(yamlPlc.Object["status"]))
		})
		It("the policy should be noncompliant after manually creating a role that has less rule", func() {
			By("Creating the mismatch role in default namespace on managed cluster")
			utils.Kubectl("apply", "-f", "../resources/configuration_policy/role-policy-e2e-less.yaml", "-n", "default", "--kubeconfig=../../kubeconfig_managed")
			By("Checking if the status of root policy is noncompliant")
			yamlPlc := utils.ParseYaml("../resources/configuration_policy/" + rolePolicyName + "-noncompliant.yaml")
			Eventually(func() interface{} {
				rootPlc := utils.GetWithTimeout(clientHubDynamic, common.GvrPolicy, rolePolicyName, userNamespace, true, defaultTimeoutSeconds)
				return rootPlc.Object["status"]
			}, defaultTimeoutSeconds, 1).Should(utils.SemanticEqual(yamlPlc.Object["status"]))
		})
		It("the policy should be compliant after manually creating the role that matches", func() {
			By("Creating the role in default namespace on managed cluster")
			utils.Kubectl("apply", "-f", "../resources/configuration_policy/role-policy-e2e.yaml", "-n", "default", "--kubeconfig=../../kubeconfig_managed")
			By("Checking if the status of root policy is compliant")
			yamlPlc := utils.ParseYaml("../resources/configuration_policy/" + rolePolicyName + "-compliant.yaml")
			Eventually(func() interface{} {
				rootPlc := utils.GetWithTimeout(clientHubDynamic, common.GvrPolicy, rolePolicyName, userNamespace, true, defaultTimeoutSeconds)
				return rootPlc.Object["status"]
			}, defaultTimeoutSeconds, 1).Should(utils.SemanticEqual(yamlPlc.Object["status"]))
		})
		It("the policy should be noncompliant after removing the role", func() {
			By("Deleting the role in default namespace on managed cluster")
			utils.Kubectl("delete", "-f", "../resources/configuration_policy/role-policy-e2e.yaml", "-n", "default", "--kubeconfig=../../kubeconfig_managed")
			By("Checking if the status of root policy is noncompliant")
			yamlPlc := utils.ParseYaml("../resources/configuration_policy/" + rolePolicyName + "-noncompliant.yaml")
			Eventually(func() interface{} {
				rootPlc := utils.GetWithTimeout(clientHubDynamic, common.GvrPolicy, rolePolicyName, userNamespace, true, defaultTimeoutSeconds)
				return rootPlc.Object["status"]
			}, defaultTimeoutSeconds, 1).Should(utils.SemanticEqual(yamlPlc.Object["status"]))
		})
		It("should clean up", func() {
			By("Deleting " + rolePolicyYaml)
			utils.Kubectl("delete", "-f", rolePolicyYaml, "-n", userNamespace, "--kubeconfig=../../kubeconfig_hub")
			By("Checking if there is any policy left")
			utils.ListWithTimeout(clientHubDynamic, common.GvrPolicy, metav1.ListOptions{}, 0, true, defaultTimeoutSeconds)
			utils.ListWithTimeout(clientManagedDynamic, common.GvrPolicy, metav1.ListOptions{}, 0, true, defaultTimeoutSeconds)
			By("Checking if there is any configuration policy left")
			utils.ListWithTimeout(clientManagedDynamic, common.GvrConfigurationPolicy, metav1.ListOptions{}, 0, true, defaultTimeoutSeconds)
		})
	})
	Describe("Test object musthave enforce", func() {
		const rolePolicyName string = "role-policy-musthave"
		const rolePolicyYaml string = "../resources/configuration_policy/role-policy-musthave.yaml"
		It("should be created on managed cluster", func() {
			By("Creating " + rolePolicyYaml)
			utils.Kubectl("apply", "-f", rolePolicyYaml, "-n", userNamespace, "--kubeconfig=../../kubeconfig_hub")
			hubPlc := utils.GetWithTimeout(clientHubDynamic, common.GvrPolicy, rolePolicyName, userNamespace, true, defaultTimeoutSeconds)
			Expect(hubPlc).NotTo(BeNil())
			By("Patching " + rolePolicyName + "-plr with decision of cluster managed")
			plr := utils.GetWithTimeout(clientHubDynamic, common.GvrPlacementRule, rolePolicyName+"-plr", userNamespace, true, defaultTimeoutSeconds)
			plr.Object["status"] = utils.GeneratePlrStatus("managed")
			_, err := clientHubDynamic.Resource(common.GvrPlacementRule).Namespace(userNamespace).UpdateStatus(context.TODO(), plr, metav1.UpdateOptions{})
			Expect(err).To(BeNil())
			By("Checking " + rolePolicyName + " on managed cluster in ns " + clusterNamespace)
			managedplc := utils.GetWithTimeout(clientManagedDynamic, common.GvrPolicy, userNamespace+"."+rolePolicyName, clusterNamespace, true, defaultTimeoutSeconds)
			Expect(managedplc).NotTo(BeNil())
		})
		It("the policy should be noncompliant", func() {
			By("Checking if the status of root policy is noncompliant")
			yamlPlc := utils.ParseYaml("../resources/configuration_policy/" + rolePolicyName + "-noncompliant.yaml")
			Eventually(func() interface{} {
				rootPlc := utils.GetWithTimeout(clientHubDynamic, common.GvrPolicy, rolePolicyName, userNamespace, true, defaultTimeoutSeconds)
				return rootPlc.Object["status"]
			}, defaultTimeoutSeconds, 1).Should(utils.SemanticEqual(yamlPlc.Object["status"]))
		})
		It("the policy should be compliant after enforcing it", func() {
			By("Patching remediationAction = enforce on root policy")
			rootPlc := utils.GetWithTimeout(clientHubDynamic, common.GvrPolicy, rolePolicyName, userNamespace, true, defaultTimeoutSeconds)
			rootPlc.Object["spec"].(map[string]interface{})["remediationAction"] = "enforce"
			rootPlc, _ = clientHubDynamic.Resource(common.GvrPolicy).Namespace(userNamespace).Update(context.TODO(), rootPlc, metav1.UpdateOptions{})
			Expect(rootPlc.Object["spec"].(map[string]interface{})["remediationAction"]).To(Equal("enforce"))
			By("Checking if the status of root policy is compliant")
			yamlPlc := utils.ParseYaml("../resources/configuration_policy/" + rolePolicyName + "-compliant.yaml")
			Eventually(func() interface{} {
				rootPlc := utils.GetWithTimeout(clientHubDynamic, common.GvrPolicy, rolePolicyName, userNamespace, true, defaultTimeoutSeconds)
				return rootPlc.Object["status"]
			}, defaultTimeoutSeconds, 1).Should(utils.SemanticEqual(yamlPlc.Object["status"]))
		})
		It("should recreate the role if manually deleted", func() {
			By("Deleting the role in default namespace on managed cluster")
			utils.Kubectl("delete", "role", "-n", "default", "--all", "--kubeconfig=../../kubeconfig_managed")
			By("Checking if the role has been deleted")
			Eventually(func() interface{} {
				roleList, err := clientManagedDynamic.Resource(common.GvrRole).Namespace("default").List(context.TODO(), metav1.ListOptions{})
				Expect(err).To(BeNil())
				return len(roleList.Items)
			}, defaultTimeoutSeconds, 1).Should(Equal(0))
			By("Checking if the role has been recreated")
			Eventually(func() interface{} {
				roleList, err := clientManagedDynamic.Resource(common.GvrRole).Namespace("default").List(context.TODO(), metav1.ListOptions{})
				Expect(err).To(BeNil())
				return len(roleList.Items)
			}, defaultTimeoutSeconds, 1).Should(Equal(1))
			By("Checking if the status of root policy is still compliant")
			yamlPlc := utils.ParseYaml("../resources/configuration_policy/" + rolePolicyName + "-compliant.yaml")
			Eventually(func() interface{} {
				rootPlc := utils.GetWithTimeout(clientHubDynamic, common.GvrPolicy, rolePolicyName, userNamespace, true, defaultTimeoutSeconds)
				return rootPlc.Object["status"]
			}, defaultTimeoutSeconds, 1).Should(utils.SemanticEqual(yamlPlc.Object["status"]))
		})
		It("the policy should not be patched after manually creating a role that has more rules", func() {
			By("Creating the mismatch role in default namespace on managed cluster")
			utils.Kubectl("apply", "-f", "../resources/configuration_policy/role-policy-e2e-more.yaml", "-n", "default", "--kubeconfig=../../kubeconfig_managed")
			utils.Pause(20)
			By("Checking if the role is not patched to match in 20s")
			yamlRole := utils.ParseYaml("../resources/configuration_policy/role-policy-e2e-more.yaml")
			Eventually(func() interface{} {
				managedRole := utils.GetWithTimeout(clientManagedDynamic, common.GvrRole, "role-policy-e2e", "default", true, defaultTimeoutSeconds)
				return managedRole.Object["rules"]
			}, defaultTimeoutSeconds, 1).Should(utils.SemanticEqual(yamlRole.Object["rules"]))
			Consistently(func() interface{} {
				managedRole := utils.GetWithTimeout(clientManagedDynamic, common.GvrRole, "role-policy-e2e", "default", true, defaultTimeoutSeconds)
				return managedRole.Object["rules"]
			}, 20, 1).Should(utils.SemanticEqual(yamlRole.Object["rules"]))
			By("Checking if the status of root policy is still compliant")
			yamlPlc := utils.ParseYaml("../resources/configuration_policy/" + rolePolicyName + "-compliant.yaml")
			Eventually(func() interface{} {
				rootPlc := utils.GetWithTimeout(clientHubDynamic, common.GvrPolicy, rolePolicyName, userNamespace, true, defaultTimeoutSeconds)
				return rootPlc.Object["status"]
			}, defaultTimeoutSeconds, 1).Should(utils.SemanticEqual(yamlPlc.Object["status"]))
		})
		It("the policy should be patched after manually creating a role that has less rules", func() {
			By("Creating the mismatch role in default namespace on managed cluster")
			utils.Kubectl("apply", "-f", "../resources/configuration_policy/role-policy-e2e-less.yaml", "-n", "default", "--kubeconfig=../../kubeconfig_managed")
			By("Checking if the role has been patched to match")
			yamlRole := utils.ParseYaml("../resources/configuration_policy/role-policy-e2e.yaml")
			Eventually(func() interface{} {
				managedRole := utils.GetWithTimeout(clientManagedDynamic, common.GvrRole, "role-policy-e2e", "default", true, defaultTimeoutSeconds)
				return managedRole.Object["rules"]
			}, defaultTimeoutSeconds, 1).Should(utils.SemanticEqual(yamlRole.Object["rules"]))
			By("Checking if the status of root policy is still compliant")
			yamlPlc := utils.ParseYaml("../resources/configuration_policy/" + rolePolicyName + "-compliant.yaml")
			Eventually(func() interface{} {
				rootPlc := utils.GetWithTimeout(clientHubDynamic, common.GvrPolicy, rolePolicyName, userNamespace, true, defaultTimeoutSeconds)
				return rootPlc.Object["status"]
			}, defaultTimeoutSeconds, 1).Should(utils.SemanticEqual(yamlPlc.Object["status"]))
		})

		It("should clean up", func() {
			By("Deleting " + rolePolicyYaml)
			utils.Kubectl("delete", "-f", rolePolicyYaml, "-n", userNamespace, "--kubeconfig=../../kubeconfig_hub")
			By("Checking if there is any policy left")
			utils.ListWithTimeout(clientHubDynamic, common.GvrPolicy, metav1.ListOptions{}, 0, true, defaultTimeoutSeconds)
			utils.ListWithTimeout(clientManagedDynamic, common.GvrPolicy, metav1.ListOptions{}, 0, true, defaultTimeoutSeconds)
			By("Checking if there is any configuration policy left")
			utils.ListWithTimeout(clientManagedDynamic, common.GvrConfigurationPolicy, metav1.ListOptions{}, 0, true, defaultTimeoutSeconds)
			By("Deleting the role in default namespace on managed cluster")
			utils.Pause(15)
			utils.Kubectl("delete", "role", "-n", "default", "--all", "--kubeconfig=../../kubeconfig_managed")
			By("Checking if there is any role left")
			Eventually(func() interface{} {
				roleList, err := clientManagedDynamic.Resource(common.GvrRole).Namespace("default").List(context.TODO(), metav1.ListOptions{})
				Expect(err).To(BeNil())
				return len(roleList.Items)
			}, defaultTimeoutSeconds, 1).Should(Equal(0))
		})
	})
	Describe("Test object mustnothave inform", func() {
		const rolePolicyName string = "role-policy-mustnothave"
		const rolePolicyYaml string = "../resources/configuration_policy/role-policy-mustnothave.yaml"
		It("should be created on managed cluster", func() {
			By("Creating " + rolePolicyYaml)
			utils.Kubectl("apply", "-f", rolePolicyYaml, "-n", userNamespace, "--kubeconfig=../../kubeconfig_hub")
			hubPlc := utils.GetWithTimeout(clientHubDynamic, common.GvrPolicy, rolePolicyName, userNamespace, true, defaultTimeoutSeconds)
			Expect(hubPlc).NotTo(BeNil())
			By("Patching " + rolePolicyName + "-plr with decision of cluster managed")
			plr := utils.GetWithTimeout(clientHubDynamic, common.GvrPlacementRule, rolePolicyName+"-plr", userNamespace, true, defaultTimeoutSeconds)
			plr.Object["status"] = utils.GeneratePlrStatus("managed")
			_, err := clientHubDynamic.Resource(common.GvrPlacementRule).Namespace(userNamespace).UpdateStatus(context.TODO(), plr, metav1.UpdateOptions{})
			Expect(err).To(BeNil())
			By("Checking " + rolePolicyName + " on managed cluster in ns " + clusterNamespace)
			managedplc := utils.GetWithTimeout(clientManagedDynamic, common.GvrPolicy, userNamespace+"."+rolePolicyName, clusterNamespace, true, defaultTimeoutSeconds)
			Expect(managedplc).NotTo(BeNil())
		})
		It("the policy should be compliant", func() {
			By("Checking if the status of root policy is compliant")
			yamlPlc := utils.ParseYaml("../resources/configuration_policy/" + rolePolicyName + "-compliant.yaml")
			Eventually(func() interface{} {
				rootPlc := utils.GetWithTimeout(clientHubDynamic, common.GvrPolicy, rolePolicyName, userNamespace, true, defaultTimeoutSeconds)
				return rootPlc.Object["status"]
			}, defaultTimeoutSeconds, 1).Should(utils.SemanticEqual(yamlPlc.Object["status"]))
		})
		It("the policy should be noncompliant after manually creating the role on managed cluster", func() {
			By("Creating the role in default namespace on managed cluster")
			utils.Kubectl("apply", "-f", "../resources/configuration_policy/role-policy-e2e.yaml", "-n", "default", "--kubeconfig=../../kubeconfig_managed")
			By("Checking if the status of root policy is compliant")
			yamlPlc := utils.ParseYaml("../resources/configuration_policy/" + rolePolicyName + "-noncompliant.yaml")
			Eventually(func() interface{} {
				rootPlc := utils.GetWithTimeout(clientHubDynamic, common.GvrPolicy, rolePolicyName, userNamespace, true, defaultTimeoutSeconds)
				return rootPlc.Object["status"]
			}, defaultTimeoutSeconds, 1).Should(utils.SemanticEqual(yamlPlc.Object["status"]))
		})
		It("the policy should be compliant after removing the role", func() {
			By("Deleting the role in default namespace on managed cluster")
			utils.Kubectl("delete", "role", "-n", "default", "--all", "--kubeconfig=../../kubeconfig_managed")
			By("Checking if the status of root policy is noncompliant")
			yamlPlc := utils.ParseYaml("../resources/configuration_policy/" + rolePolicyName + "-compliant.yaml")
			Eventually(func() interface{} {
				rootPlc := utils.GetWithTimeout(clientHubDynamic, common.GvrPolicy, rolePolicyName, userNamespace, true, defaultTimeoutSeconds)
				return rootPlc.Object["status"]
			}, defaultTimeoutSeconds, 1).Should(utils.SemanticEqual(yamlPlc.Object["status"]))
		})
		It("should clean up", func() {
			By("Deleting " + rolePolicyYaml)
			utils.Kubectl("delete", "-f", rolePolicyYaml, "-n", userNamespace, "--kubeconfig=../../kubeconfig_hub")
			By("Checking if there is any policy left")
			utils.ListWithTimeout(clientHubDynamic, common.GvrPolicy, metav1.ListOptions{}, 0, true, defaultTimeoutSeconds)
			utils.ListWithTimeout(clientManagedDynamic, common.GvrPolicy, metav1.ListOptions{}, 0, true, defaultTimeoutSeconds)
			By("Checking if there is any configuration policy left")
			utils.ListWithTimeout(clientManagedDynamic, common.GvrConfigurationPolicy, metav1.ListOptions{}, 0, true, defaultTimeoutSeconds)
		})
	})
	Describe("Test object mustnothave enforce", func() {
		const rolePolicyName string = "role-policy-mustnothave"
		const rolePolicyYaml string = "../resources/configuration_policy/role-policy-mustnothave.yaml"
		It("should be created on managed cluster", func() {
			By("Creating " + rolePolicyYaml)
			utils.Kubectl("apply", "-f", rolePolicyYaml, "-n", userNamespace, "--kubeconfig=../../kubeconfig_hub")
			hubPlc := utils.GetWithTimeout(clientHubDynamic, common.GvrPolicy, rolePolicyName, userNamespace, true, defaultTimeoutSeconds)
			Expect(hubPlc).NotTo(BeNil())
			By("Patching " + rolePolicyName + "-plr with decision of cluster managed")
			plr := utils.GetWithTimeout(clientHubDynamic, common.GvrPlacementRule, rolePolicyName+"-plr", userNamespace, true, defaultTimeoutSeconds)
			plr.Object["status"] = utils.GeneratePlrStatus("managed")
			_, err := clientHubDynamic.Resource(common.GvrPlacementRule).Namespace(userNamespace).UpdateStatus(context.TODO(), plr, metav1.UpdateOptions{})
			Expect(err).To(BeNil())
			By("Checking " + rolePolicyName + " on managed cluster in ns " + clusterNamespace)
			managedplc := utils.GetWithTimeout(clientManagedDynamic, common.GvrPolicy, userNamespace+"."+rolePolicyName, clusterNamespace, true, defaultTimeoutSeconds)
			Expect(managedplc).NotTo(BeNil())
		})
		It("the policy should be compliant", func() {
			By("Checking if the status of root policy is compliant")
			yamlPlc := utils.ParseYaml("../resources/configuration_policy/" + rolePolicyName + "-compliant.yaml")
			Eventually(func() interface{} {
				rootPlc := utils.GetWithTimeout(clientHubDynamic, common.GvrPolicy, rolePolicyName, userNamespace, true, defaultTimeoutSeconds)
				return rootPlc.Object["status"]
			}, defaultTimeoutSeconds, 1).Should(utils.SemanticEqual(yamlPlc.Object["status"]))
		})
		It("the policy should be noncompliant after manually creating the role on managed cluster", func() {
			By("Creating the role in default namespace on managed cluster")
			utils.Kubectl("apply", "-f", "../resources/configuration_policy/role-policy-e2e.yaml", "-n", "default", "--kubeconfig=../../kubeconfig_managed")
			By("Checking if the status of root policy is noncompliant")
			yamlPlc := utils.ParseYaml("../resources/configuration_policy/" + rolePolicyName + "-noncompliant.yaml")
			Eventually(func() interface{} {
				rootPlc := utils.GetWithTimeout(clientHubDynamic, common.GvrPolicy, rolePolicyName, userNamespace, true, defaultTimeoutSeconds)
				return rootPlc.Object["status"]
			}, defaultTimeoutSeconds, 1).Should(utils.SemanticEqual(yamlPlc.Object["status"]))
		})
		It("the policy should be compliant after enforcing it", func() {
			By("Patching remediationAction = enforce on root policy")
			rootPlc := utils.GetWithTimeout(clientHubDynamic, common.GvrPolicy, rolePolicyName, userNamespace, true, defaultTimeoutSeconds)
			rootPlc.Object["spec"].(map[string]interface{})["remediationAction"] = "enforce"
			rootPlc, _ = clientHubDynamic.Resource(common.GvrPolicy).Namespace(userNamespace).Update(context.TODO(), rootPlc, metav1.UpdateOptions{})
			Expect(rootPlc.Object["spec"].(map[string]interface{})["remediationAction"]).To(Equal("enforce"))
			By("Checking if the status of root policy is compliant")
			yamlPlc := utils.ParseYaml("../resources/configuration_policy/" + rolePolicyName + "-compliant.yaml")
			Eventually(func() interface{} {
				rootPlc := utils.GetWithTimeout(clientHubDynamic, common.GvrPolicy, rolePolicyName, userNamespace, true, defaultTimeoutSeconds)
				return rootPlc.Object["status"]
			}, defaultTimeoutSeconds, 1).Should(utils.SemanticEqual(yamlPlc.Object["status"]))
		})
		It("the policy should remove the role on managed cluster if manually created", func() {
			By("Creating the role in default namespace on managed cluster")
			utils.Kubectl("apply", "-f", "../resources/configuration_policy/role-policy-e2e.yaml", "-n", "default", "--kubeconfig=../../kubeconfig_managed")
			utils.Pause(20)
			By("Checking if the role has been deleted")
			Eventually(func() interface{} {
				roleList, err := clientManagedDynamic.Resource(common.GvrRole).Namespace("default").List(context.TODO(), metav1.ListOptions{})
				Expect(err).To(BeNil())
				return len(roleList.Items)
			}, defaultTimeoutSeconds, 1).Should(Equal(0))
			By("Checking if the status of root policy is compliant")
			yamlPlc := utils.ParseYaml("../resources/configuration_policy/" + rolePolicyName + "-compliant.yaml")
			Eventually(func() interface{} {
				rootPlc := utils.GetWithTimeout(clientHubDynamic, common.GvrPolicy, rolePolicyName, userNamespace, true, defaultTimeoutSeconds)
				return rootPlc.Object["status"]
			}, defaultTimeoutSeconds, 1).Should(utils.SemanticEqual(yamlPlc.Object["status"]))
		})
		It("should clean up", func() {
			By("Deleting " + rolePolicyYaml)
			utils.Kubectl("delete", "-f", rolePolicyYaml, "-n", userNamespace, "--kubeconfig=../../kubeconfig_hub")
			By("Checking if there is any policy left")
			utils.ListWithTimeout(clientHubDynamic, common.GvrPolicy, metav1.ListOptions{}, 0, true, defaultTimeoutSeconds)
			utils.ListWithTimeout(clientManagedDynamic, common.GvrPolicy, metav1.ListOptions{}, 0, true, defaultTimeoutSeconds)
			By("Checking if there is any configuration policy left")
			utils.ListWithTimeout(clientManagedDynamic, common.GvrConfigurationPolicy, metav1.ListOptions{}, 0, true, defaultTimeoutSeconds)
		})
	})
	Describe("Test object mustonlyhave inform", func() {
		const rolePolicyName string = "role-policy-mustonlyhave"
		const rolePolicyYaml string = "../resources/configuration_policy/role-policy-mustonlyhave.yaml"
		It("should be created on managed cluster", func() {
			By("Creating " + rolePolicyYaml)
			utils.Kubectl("apply", "-f", rolePolicyYaml, "-n", userNamespace, "--kubeconfig=../../kubeconfig_hub")
			hubPlc := utils.GetWithTimeout(clientHubDynamic, common.GvrPolicy, rolePolicyName, userNamespace, true, defaultTimeoutSeconds)
			Expect(hubPlc).NotTo(BeNil())
			By("Patching " + rolePolicyName + "-plr with decision of cluster managed")
			plr := utils.GetWithTimeout(clientHubDynamic, common.GvrPlacementRule, rolePolicyName+"-plr", userNamespace, true, defaultTimeoutSeconds)
			plr.Object["status"] = utils.GeneratePlrStatus("managed")
			_, err := clientHubDynamic.Resource(common.GvrPlacementRule).Namespace(userNamespace).UpdateStatus(context.TODO(), plr, metav1.UpdateOptions{})
			Expect(err).To(BeNil())
			By("Checking " + rolePolicyName + " on managed cluster in ns " + clusterNamespace)
			managedplc := utils.GetWithTimeout(clientManagedDynamic, common.GvrPolicy, userNamespace+"."+rolePolicyName, clusterNamespace, true, defaultTimeoutSeconds)
			Expect(managedplc).NotTo(BeNil())
		})
		It("the policy should be noncompliant", func() {
			By("Checking if the status of root policy is noncompliant")
			yamlPlc := utils.ParseYaml("../resources/configuration_policy/" + rolePolicyName + "-noncompliant.yaml")
			Eventually(func() interface{} {
				rootPlc := utils.GetWithTimeout(clientHubDynamic, common.GvrPolicy, rolePolicyName, userNamespace, true, defaultTimeoutSeconds)
				return rootPlc.Object["status"]
			}, defaultTimeoutSeconds, 1).Should(utils.SemanticEqual(yamlPlc.Object["status"]))
		})
		It("the policy should be compliant if manually created", func() {
			By("Creating the role in default namespace on managed cluster")
			utils.Kubectl("apply", "-f", "../resources/configuration_policy/role-policy-e2e.yaml", "-n", "default", "--kubeconfig=../../kubeconfig_managed")
			By("Checking if the status of root policy is compliant")
			yamlPlc := utils.ParseYaml("../resources/configuration_policy/" + rolePolicyName + "-compliant.yaml")
			Eventually(func() interface{} {
				rootPlc := utils.GetWithTimeout(clientHubDynamic, common.GvrPolicy, rolePolicyName, userNamespace, true, defaultTimeoutSeconds)
				return rootPlc.Object["status"]
			}, defaultTimeoutSeconds, 1).Should(utils.SemanticEqual(yamlPlc.Object["status"]))
		})
		It("the role should be noncompliant if mismatch", func() {
			By("Creating a role with different rules")
			utils.Kubectl("apply", "-f", "../resources/configuration_policy/role-policy-e2e-mismatch.yaml", "-n", "default", "--kubeconfig=../../kubeconfig_managed")
			By("Checking if the status of root policy is noncompliant")
			yamlPlc := utils.ParseYaml("../resources/configuration_policy/" + rolePolicyName + "-noncompliant.yaml")
			Eventually(func() interface{} {
				rootPlc := utils.GetWithTimeout(clientHubDynamic, common.GvrPolicy, rolePolicyName, userNamespace, true, defaultTimeoutSeconds)
				return rootPlc.Object["status"]
			}, defaultTimeoutSeconds, 1).Should(utils.SemanticEqual(yamlPlc.Object["status"]))
		})
		It("the policy should be compliant if matches", func() {
			By("Creating the role in default namespace on managed cluster")
			utils.Kubectl("apply", "-f", "../resources/configuration_policy/role-policy-e2e.yaml", "-n", "default", "--kubeconfig=../../kubeconfig_managed")
			By("Checking if the status of root policy is compliant")
			yamlPlc := utils.ParseYaml("../resources/configuration_policy/" + rolePolicyName + "-compliant.yaml")
			Eventually(func() interface{} {
				rootPlc := utils.GetWithTimeout(clientHubDynamic, common.GvrPolicy, rolePolicyName, userNamespace, true, defaultTimeoutSeconds)
				return rootPlc.Object["status"]
			}, defaultTimeoutSeconds, 1).Should(utils.SemanticEqual(yamlPlc.Object["status"]))
		})
		It("the policy should be noncompliant if has less rules", func() {
			By("Creating the role in default namespace on managed cluster")
			utils.Kubectl("apply", "-f", "../resources/configuration_policy/role-policy-e2e-less.yaml", "-n", "default", "--kubeconfig=../../kubeconfig_managed")
			By("Checking if the status of root policy is noncompliant")
			yamlPlc := utils.ParseYaml("../resources/configuration_policy/" + rolePolicyName + "-noncompliant.yaml")
			Eventually(func() interface{} {
				rootPlc := utils.GetWithTimeout(clientHubDynamic, common.GvrPolicy, rolePolicyName, userNamespace, true, defaultTimeoutSeconds)
				return rootPlc.Object["status"]
			}, defaultTimeoutSeconds, 1).Should(utils.SemanticEqual(yamlPlc.Object["status"]))
		})
		It("the policy should be compliant if matches", func() {
			By("Creating the role in default namespace on managed cluster")
			utils.Kubectl("apply", "-f", "../resources/configuration_policy/role-policy-e2e.yaml", "-n", "default", "--kubeconfig=../../kubeconfig_managed")
			By("Checking if the status of root policy is compliant")
			yamlPlc := utils.ParseYaml("../resources/configuration_policy/" + rolePolicyName + "-compliant.yaml")
			Eventually(func() interface{} {
				rootPlc := utils.GetWithTimeout(clientHubDynamic, common.GvrPolicy, rolePolicyName, userNamespace, true, defaultTimeoutSeconds)
				return rootPlc.Object["status"]
			}, defaultTimeoutSeconds, 1).Should(utils.SemanticEqual(yamlPlc.Object["status"]))
		})
		It("the policy should be noncompliant if has more rules", func() {
			By("Creating the role in default namespace on managed cluster")
			utils.Kubectl("apply", "-f", "../resources/configuration_policy/role-policy-e2e-more.yaml", "-n", "default", "--kubeconfig=../../kubeconfig_managed")
			By("Checking if the status of root policy is noncompliant")
			yamlPlc := utils.ParseYaml("../resources/configuration_policy/" + rolePolicyName + "-noncompliant.yaml")
			Eventually(func() interface{} {
				rootPlc := utils.GetWithTimeout(clientHubDynamic, common.GvrPolicy, rolePolicyName, userNamespace, true, defaultTimeoutSeconds)
				return rootPlc.Object["status"]
			}, defaultTimeoutSeconds, 1).Should(utils.SemanticEqual(yamlPlc.Object["status"]))
		})
		It("should clean up", func() {
			By("Deleting " + rolePolicyYaml)
			utils.Kubectl("delete", "-f", rolePolicyYaml, "-n", userNamespace, "--kubeconfig=../../kubeconfig_hub")
			By("Checking if there is any policy left")
			utils.ListWithTimeout(clientHubDynamic, common.GvrPolicy, metav1.ListOptions{}, 0, true, defaultTimeoutSeconds)
			utils.ListWithTimeout(clientManagedDynamic, common.GvrPolicy, metav1.ListOptions{}, 0, true, defaultTimeoutSeconds)
			By("Checking if there is any configuration policy left")
			utils.ListWithTimeout(clientManagedDynamic, common.GvrConfigurationPolicy, metav1.ListOptions{}, 0, true, defaultTimeoutSeconds)
		})
	})
	Describe("Test object mustonlyhave enforce", func() {
		const rolePolicyName string = "role-policy-mustonlyhave"
		const rolePolicyYaml string = "../resources/configuration_policy/role-policy-mustonlyhave.yaml"
		It("should be created on managed cluster", func() {
			By("Creating " + rolePolicyYaml)
			utils.Kubectl("apply", "-f", rolePolicyYaml, "-n", userNamespace, "--kubeconfig=../../kubeconfig_hub")
			hubPlc := utils.GetWithTimeout(clientHubDynamic, common.GvrPolicy, rolePolicyName, userNamespace, true, defaultTimeoutSeconds)
			Expect(hubPlc).NotTo(BeNil())
			By("Patching " + rolePolicyName + "-plr with decision of cluster managed")
			plr := utils.GetWithTimeout(clientHubDynamic, common.GvrPlacementRule, rolePolicyName+"-plr", userNamespace, true, defaultTimeoutSeconds)
			plr.Object["status"] = utils.GeneratePlrStatus("managed")
			_, err := clientHubDynamic.Resource(common.GvrPlacementRule).Namespace(userNamespace).UpdateStatus(context.TODO(), plr, metav1.UpdateOptions{})
			Expect(err).To(BeNil())
			By("Checking " + rolePolicyName + " on managed cluster in ns " + clusterNamespace)
			managedplc := utils.GetWithTimeout(clientManagedDynamic, common.GvrPolicy, userNamespace+"."+rolePolicyName, clusterNamespace, true, defaultTimeoutSeconds)
			Expect(managedplc).NotTo(BeNil())
		})
		It("the policy should be compliant after enforcing it", func() {
			By("Patching remediationAction = enforce on root policy")
			rootPlc := utils.GetWithTimeout(clientHubDynamic, common.GvrPolicy, rolePolicyName, userNamespace, true, defaultTimeoutSeconds)
			rootPlc.Object["spec"].(map[string]interface{})["remediationAction"] = "enforce"
			clientHubDynamic.Resource(common.GvrPolicy).Namespace(userNamespace).Update(context.TODO(), rootPlc, metav1.UpdateOptions{})
			Eventually(func() interface{} {
				rootPlc := utils.GetWithTimeout(clientHubDynamic, common.GvrPolicy, rolePolicyName, userNamespace, true, defaultTimeoutSeconds)
				remediation, ok := rootPlc.Object["spec"].(map[string]interface{})["remediationAction"]
				if !ok {
					return nil
				}
				return remediation
			}, defaultTimeoutSeconds, 1).Should(utils.SemanticEqual("enforce"))
			By("Checking if the status of root policy is compliant")
			yamlPlc := utils.ParseYaml("../resources/configuration_policy/" + rolePolicyName + "-compliant.yaml")
			Eventually(func() interface{} {
				rootPlc := utils.GetWithTimeout(clientHubDynamic, common.GvrPolicy, rolePolicyName, userNamespace, true, defaultTimeoutSeconds)
				return rootPlc.Object["status"]
			}, defaultTimeoutSeconds, 1).Should(utils.SemanticEqual(yamlPlc.Object["status"]))
		})
		It("the role should be created by policy", func() {
			By("Checking if the role has been created")
			Eventually(func() interface{} {
				roleList, err := clientManagedDynamic.Resource(common.GvrRole).Namespace("default").List(context.TODO(), metav1.ListOptions{})
				Expect(err).To(BeNil())
				return len(roleList.Items)
			}, defaultTimeoutSeconds, 1).Should(Equal(1))
		})
		It("the role should be recreated if manually deleted", func() {
			By("Deleting the role in default namespace on managed cluster")
			utils.Kubectl("delete", "role", "-n", "default", "--all", "--kubeconfig=../../kubeconfig_managed")
			By("Checking if the role has been deleted")
			Eventually(func() interface{} {
				roleList, err := clientManagedDynamic.Resource(common.GvrRole).Namespace("default").List(context.TODO(), metav1.ListOptions{})
				Expect(err).To(BeNil())
				return len(roleList.Items)
			}, defaultTimeoutSeconds, 1).Should(Equal(0))
			By("Checking if the role has been recreated")
			Eventually(func() interface{} {
				roleList, err := clientManagedDynamic.Resource(common.GvrRole).Namespace("default").List(context.TODO(), metav1.ListOptions{})
				Expect(err).To(BeNil())
				return len(roleList.Items)
			}, defaultTimeoutSeconds, 1).Should(Equal(1))
			By("Checking if the status of root policy is still compliant")
			yamlPlc := utils.ParseYaml("../resources/configuration_policy/" + rolePolicyName + "-compliant.yaml")
			Eventually(func() interface{} {
				rootPlc := utils.GetWithTimeout(clientHubDynamic, common.GvrPolicy, rolePolicyName, userNamespace, true, defaultTimeoutSeconds)
				return rootPlc.Object["status"]
			}, defaultTimeoutSeconds, 1).Should(utils.SemanticEqual(yamlPlc.Object["status"]))
		})
		It("the role should be patched if has less rules", func() {
			By("Creating a role with less rules")
			utils.Kubectl("apply", "-f", "../resources/configuration_policy/role-policy-e2e-less.yaml", "-n", "default", "--kubeconfig=../../kubeconfig_managed")
			By("Checking if the role has been patched to have less rules")
			yamlRole := utils.ParseYaml("../resources/configuration_policy/role-policy-e2e-less.yaml")
			Eventually(func() interface{} {
				managedRole := utils.GetWithTimeout(clientManagedDynamic, common.GvrRole, "role-policy-e2e", "default", true, defaultTimeoutSeconds)
				return managedRole.Object["rules"]
			}, defaultTimeoutSeconds, 1).Should(utils.SemanticEqual(yamlRole.Object["rules"]))
			By("Checking if the role has been patched to match by config policy")
			yamlRole = utils.ParseYaml("../resources/configuration_policy/role-policy-e2e.yaml")
			Eventually(func() interface{} {
				managedRole := utils.GetWithTimeout(clientManagedDynamic, common.GvrRole, "role-policy-e2e", "default", true, defaultTimeoutSeconds)
				return managedRole.Object["rules"]
			}, defaultTimeoutSeconds, 1).Should(utils.SemanticEqual(yamlRole.Object["rules"]))
			By("Checking if the status of root policy is still compliant")
			yamlPlc := utils.ParseYaml("../resources/configuration_policy/" + rolePolicyName + "-compliant.yaml")
			Eventually(func() interface{} {
				rootPlc := utils.GetWithTimeout(clientHubDynamic, common.GvrPolicy, rolePolicyName, userNamespace, true, defaultTimeoutSeconds)
				return rootPlc.Object["status"]
			}, defaultTimeoutSeconds, 1).Should(utils.SemanticEqual(yamlPlc.Object["status"]))
		})
		It("the role should be patched if has more rules", func() {
			By("Creating a role with more rules")
			utils.Kubectl("apply", "-f", "../resources/configuration_policy/role-policy-e2e-more.yaml", "-n", "default", "--kubeconfig=../../kubeconfig_managed")
			By("Checking if the role has been patched to have more rules")
			yamlRole := utils.ParseYaml("../resources/configuration_policy/role-policy-e2e-more.yaml")
			Eventually(func() interface{} {
				managedRole := utils.GetWithTimeout(clientManagedDynamic, common.GvrRole, "role-policy-e2e", "default", true, defaultTimeoutSeconds)
				return managedRole.Object["rules"]
			}, defaultTimeoutSeconds, 1).Should(utils.SemanticEqual(yamlRole.Object["rules"]))
			By("Checking if the role has been patched to match by config policy")
			yamlRole = utils.ParseYaml("../resources/configuration_policy/role-policy-e2e.yaml")
			Eventually(func() interface{} {
				managedRole := utils.GetWithTimeout(clientManagedDynamic, common.GvrRole, "role-policy-e2e", "default", true, defaultTimeoutSeconds)
				return managedRole.Object["rules"]
			}, defaultTimeoutSeconds, 1).Should(utils.SemanticEqual(yamlRole.Object["rules"]))
			By("Checking if the status of root policy is still compliant")
			yamlPlc := utils.ParseYaml("../resources/configuration_policy/" + rolePolicyName + "-compliant.yaml")
			Eventually(func() interface{} {
				rootPlc := utils.GetWithTimeout(clientHubDynamic, common.GvrPolicy, rolePolicyName, userNamespace, true, defaultTimeoutSeconds)
				return rootPlc.Object["status"]
			}, defaultTimeoutSeconds, 1).Should(utils.SemanticEqual(yamlPlc.Object["status"]))
		})
		It("the role should be patched if mismatch", func() {
			By("Creating a role with different rules")
			utils.Kubectl("apply", "-f", "../resources/configuration_policy/role-policy-e2e-mismatch.yaml", "-n", "default", "--kubeconfig=../../kubeconfig_managed")
			By("Checking if the role has been patched to mismatch")
			yamlRole := utils.ParseYaml("../resources/configuration_policy/role-policy-e2e-mismatch.yaml")
			Eventually(func() interface{} {
				managedRole := utils.GetWithTimeout(clientManagedDynamic, common.GvrRole, "role-policy-e2e", "default", true, defaultTimeoutSeconds)
				return managedRole.Object["rules"]
			}, defaultTimeoutSeconds, 1).Should(utils.SemanticEqual(yamlRole.Object["rules"]))
			By("Checking if the role has been patched to match by config policy")
			yamlRole = utils.ParseYaml("../resources/configuration_policy/role-policy-e2e.yaml")
			Eventually(func() interface{} {
				managedRole := utils.GetWithTimeout(clientManagedDynamic, common.GvrRole, "role-policy-e2e", "default", true, defaultTimeoutSeconds)
				return managedRole.Object["rules"]
			}, defaultTimeoutSeconds, 1).Should(utils.SemanticEqual(yamlRole.Object["rules"]))
			By("Checking if the status of root policy is still compliant")
			yamlPlc := utils.ParseYaml("../resources/configuration_policy/" + rolePolicyName + "-compliant.yaml")
			Eventually(func() interface{} {
				rootPlc := utils.GetWithTimeout(clientHubDynamic, common.GvrPolicy, rolePolicyName, userNamespace, true, defaultTimeoutSeconds)
				return rootPlc.Object["status"]
			}, defaultTimeoutSeconds, 1).Should(utils.SemanticEqual(yamlPlc.Object["status"]))
		})
		It("should clean up", func() {
			By("Deleting " + rolePolicyYaml)
			utils.Kubectl("delete", "-f", rolePolicyYaml, "-n", userNamespace, "--kubeconfig=../../kubeconfig_hub")
			By("Checking if there is any policy left")
			utils.ListWithTimeout(clientHubDynamic, common.GvrPolicy, metav1.ListOptions{}, 0, true, defaultTimeoutSeconds)
			utils.ListWithTimeout(clientManagedDynamic, common.GvrPolicy, metav1.ListOptions{}, 0, true, defaultTimeoutSeconds)
			By("Checking if there is any configuration policy left")
			utils.ListWithTimeout(clientManagedDynamic, common.GvrConfigurationPolicy, metav1.ListOptions{}, 0, true, defaultTimeoutSeconds)
			By("Deleting the role in default namespace on managed cluster")
			utils.Kubectl("delete", "role", "-n", "default", "--all", "--kubeconfig=../../kubeconfig_managed")
		})
	})
})
