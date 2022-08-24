// Copyright (c) 2022 Red Hat, Inc.
// Copyright Contributors to the Open Cluster Management project

package e2e

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"open-cluster-management.io/governance-policy-propagator/test/utils"

	"github.com/stolostron/governance-policy-framework/test/common"
)

const (
	testPolicyName             string = "test-policy"
	testPolicySetName          string = "test-policyset"
	testPolicySetYaml          string = "../resources/policy_set/test-policyset.yaml"
	testPolicySetPatchYaml     string = "../resources/policy_set/patch-policy-set.yaml"
	testUndoPolicySetPatchYaml string = "../resources/policy_set/undo-patch-policy-set.yaml"
	testedDisablePolicyYaml    string = "../resources/policy_set/disable-policy.yaml"
)

var _ = Describe("Test policy set", func() {
	Describe("Create policy, policyset, and placement in ns: "+userNamespace, Ordered, func() {
		It("Should create and process policy and policyset", func() {
			By("Creating " + testPolicySetYaml)
			_, err := common.OcHub("apply", "-f", testPolicySetYaml, "-n", userNamespace)
			Expect(err).To(BeNil())

			rootPolicy := utils.GetWithTimeout(
				clientHubDynamic, common.GvrPolicy, testPolicyName, userNamespace, true, defaultTimeoutSeconds,
			)
			Expect(rootPolicy).NotTo(BeNil())

			By("Patching " + testPolicySetName + "-plr with decision of cluster managed")
			plr := utils.GetWithTimeout(
				clientHubDynamic, common.GvrPlacementRule, testPolicySetName+"-plr", userNamespace,
				true, defaultTimeoutSeconds,
			)
			plr.Object["status"] = utils.GeneratePlrStatus(clusterNamespace)
			_, err = clientHubDynamic.Resource(common.GvrPlacementRule).Namespace(userNamespace).UpdateStatus(
				context.TODO(), plr, metav1.UpdateOptions{},
			)
			Expect(err).To(BeNil())

			By("Checking " + testPolicyName + " on managed cluster in ns " + clusterNamespace)
			managedplc := utils.GetWithTimeout(
				clientHubDynamic, common.GvrPolicy, userNamespace+"."+testPolicyName, clusterNamespace, true,
				defaultTimeoutSeconds,
			)
			Expect(managedplc).NotTo(BeNil())

			plcSet := utils.GetWithTimeout(
				clientHubDynamic, common.GvrPolicySet, testPolicySetName, userNamespace, true, defaultTimeoutSeconds,
			)
			Expect(plcSet).NotTo(BeNil())

			By("Checking the status of policy set - NonCompliant")
			yamlPlc := utils.ParseYaml("../resources/policy_set/statuscheck-1.yaml")

			Eventually(func() interface{} {
				rootPlcSet := utils.GetWithTimeout(
					clientHubDynamic,
					common.GvrPolicySet,
					testPolicySetName,
					userNamespace,
					true,
					defaultTimeoutSeconds,
				)

				return rootPlcSet.Object["status"]
			}, defaultTimeoutSeconds, 1).Should(utils.SemanticEqual(yamlPlc.Object["status"]))
		})

		It("Should add a status entry in policyset for a policy that does not exist", func() {
			By("Creating " + testPolicySetPatchYaml)
			_, err := common.OcHub("apply", "-f", testPolicySetPatchYaml, "-n", userNamespace)
			Expect(err).To(BeNil())

			plcSet := utils.GetWithTimeout(
				clientHubDynamic, common.GvrPolicySet, testPolicySetName, userNamespace, true, defaultTimeoutSeconds,
			)
			Expect(plcSet).NotTo(BeNil())

			By("Checking the status of policy set")
			yamlPlc := utils.ParseYaml("../resources/policy_set/statuscheck-2.yaml")

			Eventually(func() interface{} {
				rootPlcSet := utils.GetWithTimeout(
					clientHubDynamic,
					common.GvrPolicySet,
					testPolicySetName,
					userNamespace,
					true,
					defaultTimeoutSeconds,
				)

				return rootPlcSet.Object["status"]
			}, defaultTimeoutSeconds, 1).Should(utils.SemanticEqual(yamlPlc.Object["status"]))

			By("Undoing patch with " + testPolicySetPatchYaml)
			_, err = common.OcHub("apply", "-f", testUndoPolicySetPatchYaml, "-n", userNamespace)
			Expect(err).To(BeNil())
		})

		It("Should update to compliant if all its child policy violations have been remediated", func() {
			By("Enforcing the policy to make it compliant")
			rootPlc := utils.GetWithTimeout(
				clientHubDynamic, common.GvrPolicy, testPolicyName, userNamespace, true, defaultTimeoutSeconds,
			)
			rootPlc.Object["spec"].(map[string]interface{})["remediationAction"] = "enforce"
			_, err := clientHubDynamic.Resource(common.GvrPolicy).Namespace(userNamespace).Update(
				context.TODO(),
				rootPlc,
				metav1.UpdateOptions{},
			)
			Expect(err).To(BeNil())

			By("Checking the status of policy set")
			yamlPlc := utils.ParseYaml("../resources/policy_set/statuscheck-5.yaml")

			Eventually(func() interface{} {
				rootPlcSet := utils.GetWithTimeout(
					clientHubDynamic,
					common.GvrPolicySet,
					testPolicySetName,
					userNamespace,
					true,
					defaultTimeoutSeconds*3,
				)

				return rootPlcSet.Object["status"]
			}, defaultTimeoutSeconds, 1).Should(utils.SemanticEqual(yamlPlc.Object["status"]))
		})

		It("Should update status properly if a policy is disabled", func() {
			By("Creating " + testedDisablePolicyYaml)
			_, err := common.OcHub("apply", "-f", testedDisablePolicyYaml, "-n", userNamespace)
			Expect(err).To(BeNil())

			plc := utils.GetWithTimeout(
				clientHubDynamic, common.GvrPolicy, testPolicyName, userNamespace, true, defaultTimeoutSeconds,
			)
			Expect(plc).NotTo(BeNil())

			By("Checking the status of policy set")
			yamlPlc := utils.ParseYaml("../resources/policy_set/statuscheck-6.yaml")

			Eventually(func() interface{} {
				rootPlcSet := utils.GetWithTimeout(
					clientHubDynamic,
					common.GvrPolicySet,
					testPolicySetName,
					userNamespace,
					true,
					defaultTimeoutSeconds,
				)

				return rootPlcSet.Object["status"]
			}, defaultTimeoutSeconds, 1).Should(utils.SemanticEqual(yamlPlc.Object["status"]))
		})

		AfterAll(func() {
			_, err := common.OcHub(
				"delete", "-f", testPolicySetYaml,
				"-n", userNamespace, "--ignore-not-found",
			)
			Expect(err).To(BeNil())

			_, err = common.OcManaged(
				"delete", "pod",
				"-n", "default",
				"pod-that-does-not-exist", "--ignore-not-found",
			)
			Expect(err).To(BeNil())
		})
	})
})
