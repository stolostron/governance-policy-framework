// Copyright (c) 2022 Red Hat, Inc.
// Copyright Contributors to the Open Cluster Management project

package integration

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"open-cluster-management.io/governance-policy-propagator/test/utils"

	testcommon "github.com/stolostron/governance-policy-framework/test/common"
)

var _ = Describe("GRC: [P1][Sev1][policy-grc] Test policy set", Ordered, Label("BVT"), func() {
	const (
		testPolicyName             string = "test-policy"
		testPolicySetName          string = "test-policyset"
		testPolicySetYaml          string = "../resources/policy_set/test-policyset.yaml"
		testPolicySetPatchYaml     string = "../resources/policy_set/patch-policy-set.yaml"
		testUndoPolicySetPatchYaml string = "../resources/policy_set/undo-patch-policy-set.yaml"
		testedDisablePolicyYaml    string = "../resources/policy_set/disable-policy.yaml"
	)

	Describe("Create policy, policyset, and placement in ns:"+userNamespace, func() {
		It("Should create and process policy and policyset", func() {
			By("Creating " + testPolicySetYaml)
			output, err := utils.KubectlWithOutput("apply",
				"-f", testPolicySetYaml,
				"-n", userNamespace,
				"--kubeconfig="+kubeconfigHub)
			By("Creating " + testPolicySetYaml + " result is " + output)
			Expect(err).ToNot(HaveOccurred())

			By("Checking that the root policy was created")
			rootPolicyRsrc := clientHubDynamic.Resource(testcommon.GvrPolicy)
			var rootPolicy *unstructured.Unstructured
			Eventually(
				func() error {
					var err error
					rootPolicy, err = rootPolicyRsrc.Namespace(userNamespace).Get(
						context.TODO(), testPolicyName, metav1.GetOptions{},
					)

					return err
				},
				defaultTimeoutSeconds*2,
				1,
			).Should(BeNil())

			templates, found, err := unstructured.NestedSlice(rootPolicy.Object, "spec", "policy-templates")
			Expect(err).ShouldNot(HaveOccurred())
			Expect(found).Should(BeTrue())
			Expect(templates).Should(HaveLen(1))

			By("Patching placement rule " + testPolicySetName + "-plr")
			err = testcommon.PatchPlacementRule(userNamespace, testPolicySetName+"-plr")
			Expect(err).ToNot(HaveOccurred())

			By("Checking " + testPolicyName + " on managed cluster in ns " + clusterNamespace)
			policyRsrc := clientHubDynamic.Resource(testcommon.GvrPolicy)
			Eventually(
				func() error {
					var err error
					_, err = policyRsrc.Namespace(clusterNamespace).Get(
						context.TODO(), userNamespace+"."+testPolicyName, metav1.GetOptions{},
					)

					return err
				},
				defaultTimeoutSeconds*2,
				1,
			).Should(BeNil())

			By("Checking the status of policy set")
			yamlPlc := utils.ParseYaml("../resources/policy_set/statuscheck-1.yaml")

			policySetRsrc := clientHubDynamic.Resource(testcommon.GvrPolicySet)
			Eventually(func(g Gomega) interface{} {
				rootPlcSet, err := policySetRsrc.Namespace(userNamespace).Get(
					context.TODO(), testPolicySetName, metav1.GetOptions{},
				)
				g.Expect(err).ToNot(HaveOccurred())

				return rootPlcSet.Object["status"]
			},
				defaultTimeoutSeconds*2,
				1,
			).Should(utils.SemanticEqual(yamlPlc.Object["status"]))
		})

		It("Should add a status entry in policyset for a policy that does not exist", func() {
			By("Creating " + testPolicySetPatchYaml)
			output, err := utils.KubectlWithOutput("apply",
				"-f", testPolicySetPatchYaml,
				"-n", userNamespace,
				"--kubeconfig="+kubeconfigHub)
			By("Creating " + testPolicySetPatchYaml + " result is " + output)
			Expect(err).ToNot(HaveOccurred())

			By("Checking the status of policy set")
			yamlPlc := utils.ParseYaml("../resources/policy_set/statuscheck-2.yaml")

			policySetRsrc := clientHubDynamic.Resource(testcommon.GvrPolicySet)
			Eventually(func(g Gomega) interface{} {
				rootPlcSet, err := policySetRsrc.Namespace(userNamespace).Get(
					context.TODO(), testPolicySetName, metav1.GetOptions{},
				)
				g.Expect(err).ToNot(HaveOccurred())

				return rootPlcSet.Object["status"]
			},
				defaultTimeoutSeconds*2,
				1,
			).Should(utils.SemanticEqual(yamlPlc.Object["status"]))

			By("Undoing patch with " + testPolicySetPatchYaml)
			output, err = utils.KubectlWithOutput("apply",
				"-f", testUndoPolicySetPatchYaml,
				"-n", userNamespace,
				"--kubeconfig="+kubeconfigHub)
			By("Creating " + testUndoPolicySetPatchYaml + " result is " + output)
			Expect(err).ToNot(HaveOccurred())
		})

		It("Should update to compliant if all its child policy violations have been remediated", func() {
			testcommon.EnforcePolicy(testPolicyName)

			By("Checking the status of policy set")
			yamlPlc := utils.ParseYaml("../resources/policy_set/statuscheck-3.yaml")

			policySetRsrc := clientHubDynamic.Resource(testcommon.GvrPolicySet)
			Eventually(func(g Gomega) interface{} {
				rootPlcSet, err := policySetRsrc.Namespace(userNamespace).Get(
					context.TODO(), testPolicySetName, metav1.GetOptions{},
				)
				g.Expect(err).ToNot(HaveOccurred())

				return rootPlcSet.Object["status"]
			},
				defaultTimeoutSeconds*2,
				1,
			).Should(utils.SemanticEqual(yamlPlc.Object["status"]))
		})

		It("Should update status properly if a policy is disabled", func() {
			By("Creating " + testedDisablePolicyYaml)
			output, err := utils.KubectlWithOutput("apply",
				"-f", testedDisablePolicyYaml,
				"-n", userNamespace,
				"--kubeconfig="+kubeconfigHub)
			By("Creating " + testedDisablePolicyYaml + " result is " + output)
			Expect(err).ToNot(HaveOccurred())

			plcRsrc := clientHubDynamic.Resource(testcommon.GvrPolicy)
			Eventually(
				func() error {
					var err error
					_, err = plcRsrc.Namespace(userNamespace).Get(
						context.TODO(), testPolicyName, metav1.GetOptions{},
					)

					return err
				},
				defaultTimeoutSeconds*2,
				1,
			).Should(BeNil())

			By("Checking the status of policy set")
			yamlPlc := utils.ParseYaml("../resources/policy_set/statuscheck-4.yaml")

			policySetRsrc := clientHubDynamic.Resource(testcommon.GvrPolicySet)
			Eventually(func(g Gomega) interface{} {
				rootPlcSet, err := policySetRsrc.Namespace(userNamespace).Get(
					context.TODO(), testPolicySetName, metav1.GetOptions{},
				)
				g.Expect(err).ToNot(HaveOccurred())

				return rootPlcSet.Object["status"]
			},
				defaultTimeoutSeconds*2,
				1,
			).Should(utils.SemanticEqual(yamlPlc.Object["status"]))
		})

		AfterAll(func() {
			output, err := utils.KubectlWithOutput("delete",
				"-f", testPolicySetYaml,
				"-n", userNamespace,
				"--kubeconfig="+kubeconfigHub,
				"--ignore-not-found",
			)
			By("Deleting " + testPolicySetYaml + " result is " + output)
			Expect(err).ToNot(HaveOccurred())
		})
	})
})
