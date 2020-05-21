// Copyright (c) 2020 Red Hat, Inc.

package e2e

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Test configuration policy", func() {
	// Describe("Create a policy to configure a pod", func() {
	// 	const podPolicyName string = "pod-policy"
	// 	const podPolicyYaml string = "../resources/configuration_policy/pod-policy.yaml"
	// 	It("should be created on managed cluster", func() {
	// 		By("Creating " + podPolicyYaml)
	// 		utils.Kubectl("apply", "-f", podPolicyYaml, "-n", userNamespace, "--kubeconfig=../../kubeconfig_hub")
	// 		hubPlc := utils.GetWithTimeout(clientHubDynamic, gvrPolicy, podPolicyName, userNamespace, true, defaultTimeoutSeconds)
	// 		Expect(hubPlc).NotTo(BeNil())
	// 		By("Patching " + podPolicyName + "-plr with decision of cluster managed")
	// 		plr := utils.GetWithTimeout(clientHubDynamic, gvrPlacementRule, podPolicyName+"-plr", userNamespace, true, defaultTimeoutSeconds)
	// 		plr.Object["status"] = utils.GeneratePlrStatus("managed")
	// 		plr, err := clientHubDynamic.Resource(gvrPlacementRule).Namespace(userNamespace).UpdateStatus(plr, metav1.UpdateOptions{})
	// 		Expect(err).To(BeNil())
	// 		By("Checking " + podPolicyName + " on managed cluster")
	// 		managedplc := utils.GetWithTimeout(clientManagedDynamic, gvrPolicy, userNamespace+"."+podPolicyName, userNamespace, true, defaultTimeoutSeconds)
	// 		Expect(managedplc).NotTo(BeNil())
	// 	})
	// })
})
