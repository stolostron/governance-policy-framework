// Copyright (c) 2020 Red Hat, Inc.

package e2e

import (
	"github.com/open-cluster-management/governance-policy-propagator/test/utils"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("Test cert policy", func() {
	Describe("Test cert policy inform", func() {
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
			plr, err := clientHubDynamic.Resource(gvrPlacementRule).Namespace(userNamespace).UpdateStatus(plr, metav1.UpdateOptions{})
			Expect(err).To(BeNil())
			By("Checking " + iamPolicyName + " on managed cluster in ns " + clusterNamespace)
			managedplc := utils.GetWithTimeout(clientManagedDynamic, gvrPolicy, userNamespace+"."+iamPolicyName, clusterNamespace, true, defaultTimeoutSeconds)
			Expect(managedplc).NotTo(BeNil())
		})
	})
})
