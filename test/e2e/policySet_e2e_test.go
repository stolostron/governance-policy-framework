// Copyright (c) 2022 Red Hat, Inc.
// Copyright Contributors to the Open Cluster Management project

package e2e

import (
	"context"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/stolostron/governance-policy-propagator/test/utils"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	testcommon "github.com/stolostron/governance-policy-framework/test/common"
)

const (
	testPolicyName          string = "test-policy"
	testPolicySetName       string = "test-policyset"
	testPolicySetYaml       string = "../resources/policy_set/test-policyset.yaml"
	testPolicySetPatchYaml  string = "../resources/policy_set/patch-policy-set.yaml"
	testedDisablePolicyYaml string = "../resources/policy_set/disable-policy.yaml"
)

// update clusterName in checking YAML based clusterNamespace in testing env
func UpdateClusterName(
	yamlPlc *unstructured.Unstructured,
	clusterNamespace string,
) *unstructured.Unstructured {
	status := yamlPlc.Object["status"].(map[string]interface{})
	results := status["results"].([]interface{})
	clusters := results[0].(map[string]interface{})["clusters"].([]interface{})
	cluster := clusters[0].(map[string]interface{})
	cluster["clusterName"] = clusterNamespace
	cluster["clusterNamespace"] = clusterNamespace
	return yamlPlc
}

var _ = Describe("Test policy set", func() {
	Describe("Create policy, policyset, and placement in ns:"+userNamespace, func() {
		It("Should create and process policy and policyset", func() {
			By("Creating " + testPolicySetYaml)
			output, err := utils.KubectlWithOutput("apply",
				"-f", testPolicySetYaml,
				"-n", userNamespace,
				"--kubeconfig=../../kubeconfig_hub")
			By("Creating " + testPolicySetYaml + " result is " + output)
			Expect(err).To(BeNil())

			rootPolicy := utils.GetWithTimeout(
				clientHubDynamic, testcommon.GvrPolicy, testPolicyName, userNamespace, true, defaultTimeoutSeconds,
			)
			Expect(rootPolicy).NotTo(BeNil())

			By("Patching " + testPolicySetName + "-plr with decision of cluster managed")
			plr := utils.GetWithTimeout(
				clientHubDynamic, testcommon.GvrPlacementRule, testPolicySetName+"-plr", userNamespace,
				true, defaultTimeoutSeconds,
			)
			plr.Object["status"] = utils.GeneratePlrStatus(clusterNamespace)
			_, err = clientHubDynamic.Resource(testcommon.GvrPlacementRule).Namespace(userNamespace).UpdateStatus(
				context.TODO(), plr, metav1.UpdateOptions{},
			)
			Expect(err).To(BeNil())

			By("Checking " + testPolicyName + " on managed cluster in ns " + clusterNamespace)
			managedplc := utils.GetWithTimeout(
				clientHubDynamic, testcommon.GvrPolicy, userNamespace+"."+testPolicyName, clusterNamespace, true,
				defaultTimeoutSeconds,
			)
			Expect(managedplc).NotTo(BeNil())

			plcSet := utils.GetWithTimeout(
				clientHubDynamic, testcommon.GvrPolicySet, testPolicySetName, userNamespace, true, defaultTimeoutSeconds,
			)
			Expect(plcSet).NotTo(BeNil())

			By("Checking the status of policy set - NonCompliant")
			yamlPlc := utils.ParseYaml("../resources/policy_set/statuscheck-1.yaml")
			// set checking yaml clusterName and clusterNamespace by env variable
			By("Updating the checking YAML clusterName to " + clusterNamespace)
			yamlPlc = UpdateClusterName(yamlPlc, clusterNamespace)

			Eventually(func() interface{} {
				rootPlcSet := utils.GetWithTimeout(
					clientHubDynamic, testcommon.GvrPolicySet, testPolicySetName, userNamespace, true, defaultTimeoutSeconds,
				)

				return rootPlcSet.Object["status"]
			}, defaultTimeoutSeconds, 1).Should(utils.SemanticEqual(yamlPlc.Object["status"]))
		})

		It("Should add a status entry in policyset for a policy that does not exist", func() {
			By("Creating " + testPolicySetPatchYaml)
			output, err := utils.KubectlWithOutput("apply",
				"-f", testPolicySetPatchYaml,
				"-n", userNamespace,
				"--kubeconfig=../../kubeconfig_hub")
			By("Creating " + testPolicySetPatchYaml + " result is " + output)
			Expect(err).To(BeNil())

			plcSet := utils.GetWithTimeout(
				clientHubDynamic, testcommon.GvrPolicySet, testPolicySetName, userNamespace, true, defaultTimeoutSeconds,
			)
			Expect(plcSet).NotTo(BeNil())

			By("Checking the status of policy set")
			yamlPlc := utils.ParseYaml("../resources/policy_set/statuscheck-2.yaml")
			By("Updating the checking YAML clusterName to " + clusterNamespace)
			yamlPlc = UpdateClusterName(yamlPlc, clusterNamespace)

			Eventually(func() interface{} {
				rootPlcSet := utils.GetWithTimeout(
					clientHubDynamic, testcommon.GvrPolicySet, testPolicySetName, userNamespace, true, defaultTimeoutSeconds,
				)

				return rootPlcSet.Object["status"]
			}, defaultTimeoutSeconds, 1).Should(utils.SemanticEqual(yamlPlc.Object["status"]))
		})

		It("Should update to compliant if all its child policy violations have been remediated", func() {
			By("Enforcing the policy to make it compliant")
			rootPlc := utils.GetWithTimeout(
				clientHubDynamic, testcommon.GvrPolicy, testPolicyName, userNamespace, true, defaultTimeoutSeconds,
			)
			rootPlc.Object["spec"].(map[string]interface{})["remediationAction"] = "enforce"
			_, err := clientHubDynamic.Resource(testcommon.GvrPolicy).Namespace(userNamespace).Update(context.TODO(), rootPlc, metav1.UpdateOptions{})
			Expect(err).To(BeNil())

			By("Checking the status of policy set")
			yamlPlc := utils.ParseYaml("../resources/policy_set/statuscheck-3.yaml")
			By("Updating the checking YAML clusterName to " + clusterNamespace)
			yamlPlc = UpdateClusterName(yamlPlc, clusterNamespace)

			Eventually(func() interface{} {
				rootPlcSet := utils.GetWithTimeout(
					clientHubDynamic, testcommon.GvrPolicySet, testPolicySetName, userNamespace, true, defaultTimeoutSeconds*3,
				)

				return rootPlcSet.Object["status"]
			}, defaultTimeoutSeconds, 1).Should(utils.SemanticEqual(yamlPlc.Object["status"]))
		})

		It("Should update status properly if a policy is disabled", func() {
			By("Creating " + testedDisablePolicyYaml)
			output, err := utils.KubectlWithOutput("apply",
				"-f", testedDisablePolicyYaml,
				"-n", userNamespace,
				"--kubeconfig=../../kubeconfig_hub")
			By("Creating " + testedDisablePolicyYaml + " result is " + output)
			Expect(err).To(BeNil())

			plc := utils.GetWithTimeout(
				clientHubDynamic, testcommon.GvrPolicy, testPolicyName, userNamespace, true, defaultTimeoutSeconds,
			)
			Expect(plc).NotTo(BeNil())

			By("Checking the status of policy set")
			yamlPlc := utils.ParseYaml("../resources/policy_set/statuscheck-4.yaml")

			Eventually(func() interface{} {
				rootPlcSet := utils.GetWithTimeout(
					clientHubDynamic, testcommon.GvrPolicySet, testPolicySetName, userNamespace, true, defaultTimeoutSeconds,
				)

				return rootPlcSet.Object["status"]
			}, defaultTimeoutSeconds, 1).Should(utils.SemanticEqual(yamlPlc.Object["status"]))
		})

		It("should clean up", func() {
			output, err := utils.KubectlWithOutput("delete",
				"-f", testPolicySetYaml,
				"-n", userNamespace,
				"--kubeconfig=../../kubeconfig_hub")
			By("Deleting " + testPolicySetYaml + " result is " + output)
			Expect(err).To(BeNil())

			opt := metav1.ListOptions{}
			By("Checking if there is any policy left")
			utils.ListWithTimeout(clientHubDynamic, testcommon.GvrPolicy, opt, 0, false, defaultTimeoutSeconds)
			utils.ListWithTimeout(clientManagedDynamic, testcommon.GvrPolicy, opt, 0, true, defaultTimeoutSeconds)
		})
	})
})
