// Copyright (c) 2020 Red Hat, Inc.
// +build dd

package e2e

import (
	"context"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/open-cluster-management/governance-policy-propagator/test/utils"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("Test iam policy", func() {
	Describe("Test iam policy inform", func() {
		const iamPolicyName string = "iam-policy"
		const iamPolicyYaml string = "../resources/iam_policy/iam-policy.yaml"
		It("should be created on managed cluster", func() {
			By("Creating " + iamPolicyYaml)
			utils.Kubectl("apply", "-f", iamPolicyYaml, "-n", userNamespace, "--kubeconfig=../../kubeconfig_hub")
			hubPlc := utils.GetWithTimeout(clientHubDynamic, gvrPolicy, iamPolicyName, userNamespace, true, defaultTimeoutSeconds)
			Expect(hubPlc).NotTo(BeNil())
			By("Patching " + iamPolicyName + "-plr with decision of cluster managed")
			plr := utils.GetWithTimeout(clientHubDynamic, gvrPlacementRule, iamPolicyName+"-plr", userNamespace, true, defaultTimeoutSeconds)
			plr.Object["status"] = utils.GeneratePlrStatus("managed")
			plr, err := clientHubDynamic.Resource(gvrPlacementRule).Namespace(userNamespace).UpdateStatus(context.TODO(), plr, metav1.UpdateOptions{})
			Expect(err).To(BeNil())
			By("Checking " + iamPolicyName + " on managed cluster in ns " + clusterNamespace)
			managedplc := utils.GetWithTimeout(clientManagedDynamic, gvrPolicy, userNamespace+"."+iamPolicyName, clusterNamespace, true, defaultTimeoutSeconds)
			Expect(managedplc).NotTo(BeNil())
		})
		It("the policy should be compliant as there is no clusterrolebindings", func() {
			By("Checking if the status of root policy is compliant")
			yamlPlc := utils.ParseYaml("../resources/iam_policy/" + iamPolicyName + "-compliant.yaml")
			Eventually(func() interface{} {
				rootPlc := utils.GetWithTimeout(clientHubDynamic, gvrPolicy, iamPolicyName, userNamespace, true, defaultTimeoutSeconds)
				return rootPlc.Object["status"]
			}, defaultTimeoutSeconds, 1).Should(utils.SemanticEqual(yamlPlc.Object["status"]))
		})
		It("the policy should be noncompliant after creating clusterrolebindings", func() {
			By("Creating ../resources/iam_policy/clusterrolebinding.yaml")
			utils.Kubectl("apply", "-f", "../resources/iam_policy/clusterrolebinding.yaml", "--kubeconfig=../../kubeconfig_managed")
			By("Checking if the status of root policy is noncompliant")
			yamlPlc := utils.ParseYaml("../resources/iam_policy/" + iamPolicyName + "-noncompliant.yaml")
			Eventually(func() interface{} {
				rootPlc := utils.GetWithTimeout(clientHubDynamic, gvrPolicy, iamPolicyName, userNamespace, true, defaultTimeoutSeconds)
				return rootPlc.Object["status"]
			}, defaultTimeoutSeconds, 1).Should(utils.SemanticEqual(yamlPlc.Object["status"]))
		})
		It("the policy should be compliant again after delete clusterrolebindings", func() {
			By("Deleting ../resources/iam_policy/clusterrolebinding.yaml")
			utils.Kubectl("delete", "-f", "../resources/iam_policy/clusterrolebinding.yaml", "--kubeconfig=../../kubeconfig_managed")
			By("Checking if the status of root policy is compliant")
			yamlPlc := utils.ParseYaml("../resources/iam_policy/" + iamPolicyName + "-compliant.yaml")
			Eventually(func() interface{} {
				rootPlc := utils.GetWithTimeout(clientHubDynamic, gvrPolicy, iamPolicyName, userNamespace, true, defaultTimeoutSeconds)
				return rootPlc.Object["status"]
			}, defaultTimeoutSeconds, 1).Should(utils.SemanticEqual(yamlPlc.Object["status"]))
		})
		It("should clean up", func() {
			By("Deleting " + iamPolicyYaml)
			utils.Kubectl("delete", "-f", iamPolicyYaml, "-n", userNamespace, "--kubeconfig=../../kubeconfig_hub")
			By("Checking if there is any policy left")
			utils.ListWithTimeout(clientHubDynamic, gvrPolicy, metav1.ListOptions{}, 0, true, defaultTimeoutSeconds)
			utils.ListWithTimeout(clientManagedDynamic, gvrPolicy, metav1.ListOptions{}, 0, true, defaultTimeoutSeconds)
			By("Checking if there is any iam policy left")
			utils.ListWithTimeout(clientManagedDynamic, gvrIamPolicy, metav1.ListOptions{}, 0, true, defaultTimeoutSeconds)
		})
	})
})
