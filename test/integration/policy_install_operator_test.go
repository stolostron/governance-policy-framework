// Copyright (c) 2022 Red Hat, Inc.
// Copyright Contributors to the Open Cluster Management project

package integration

import (
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	policiesv1 "open-cluster-management.io/governance-policy-propagator/api/v1"
	"open-cluster-management.io/governance-policy-propagator/test/utils"

	"github.com/stolostron/governance-policy-framework/test/common"
)

var _ = Describe("GRC: [P1][Sev1][policy-grc] Test install Operator",
	Ordered, Label("BVT"), func() {
		const (
			testNS              = "grcqeoptest-ns"
			policyNoGroupYAML   = "../resources/policy_install_operator/operator_policy_no_group.yaml"
			policyWithGroupYAML = "../resources/policy_install_operator/operator_policy_with_group.yaml"
			policyNamePrefix    = "test-op"
			noGroupSuffix       = "-43544"
			withGroupSuffix     = "-43545"
			subName             = "quay-operator"
			opGroupName         = "grcqeopgroup"
		)

		Context("When no OperatorGroup is specified", func() {
			var dynamicOpGroupName, dynamicCSVName string

			BeforeAll(func() {
				_, err := common.OcManaged("create", "ns", testNS+noGroupSuffix)
				Expect(err).ToNot(HaveOccurred())
			})

			AfterAll(func(ctx SpecContext) {
				_, err := common.OcHub(
					"delete",
					"-f",
					policyNoGroupYAML,
					"-n",
					userNamespace,
					"--ignore-not-found=true",
				)
				Expect(err).ToNot(HaveOccurred())

				_, err = common.OcManaged(
					"delete",
					"subscription.operators.coreos.com",
					subName,
					"-n",
					testNS+noGroupSuffix,
					"--ignore-not-found=true",
				)
				Expect(err).ToNot(HaveOccurred())

				if dynamicOpGroupName != "" {
					_, err = common.OcManaged(
						"delete",
						"operatorgroup",
						dynamicOpGroupName,
						"-n", testNS+noGroupSuffix,
						"--ignore-not-found=true",
					)
					Expect(err).ToNot(HaveOccurred())
				}

				csvClient := clientManagedDynamic.Resource(common.GvrClusterServiceVersion)
				csvList, err := csvClient.List(ctx, metav1.ListOptions{})
				Expect(err).ToNot(HaveOccurred())

				for _, csv := range csvList.Items {
					csvName := csv.GetName()
					if strings.HasPrefix(csvName, subName+".") {
						err := csvClient.Namespace(csv.GetNamespace()).Delete(ctx, csvName, metav1.DeleteOptions{})
						if !k8serrors.IsNotFound(err) {
							Expect(err).ToNot(HaveOccurred())
						}
					}
				}

				_, err = common.OcManaged("delete", "ns", testNS+noGroupSuffix)
				Expect(err).ToNot(HaveOccurred())
			})

			It(policyNamePrefix+noGroupSuffix+" should be created on the hub", func() {
				_, err := common.OcHub("apply", "-f", policyNoGroupYAML, "-n", userNamespace)
				Expect(err).ToNot(HaveOccurred())

				By("Patching the placement rule")
				err = common.PatchPlacementRule(userNamespace, policyNamePrefix+noGroupSuffix+"-plr")
				Expect(err).ToNot(HaveOccurred())

				By("Checking that it exists on the hub cluster")
				rootPlc := utils.GetWithTimeout(
					clientHubDynamic, common.GvrPolicy, policyNamePrefix+noGroupSuffix,
					userNamespace, true, defaultTimeoutSeconds,
				)
				Expect(rootPlc).NotTo(BeNil())
			})

			It("operator-policy"+noGroupSuffix+" should be created on the managed cluster", func() {
				By("Checking the policy on managed cluster in the namespace " + clusterNamespace)
				managedPolicy := utils.GetWithTimeout(
					clientManagedDynamic,
					common.GvrPolicy,
					userNamespace+"."+policyNamePrefix+noGroupSuffix,
					clusterNamespace,
					true,
					defaultTimeoutSeconds,
				)
				Expect(managedPolicy).NotTo(BeNil())
			})

			It("operator-policy"+noGroupSuffix+" should be NonCompliant", func() {
				By("Checking if the correct condition is generated")
				Eventually(
					common.GetOpPolicyCompMsg("operator-policy"+noGroupSuffix),
					defaultTimeoutSeconds,
					1,
				).Should(MatchRegexp("NonCompliant.*the OperatorGroup required by the policy was not found.*" +
					"the Subscription required by the policy was not found.*"))

				By("Checking if the status of the root policy is NonCompliant")
				Eventually(
					common.GetComplianceState(policyNamePrefix+noGroupSuffix),
					defaultTimeoutSeconds*2,
					1,
				).Should(Equal(policiesv1.NonCompliant))
			})

			It("Should enforce the policy on the hub", func() {
				common.EnforcePolicy(policyNamePrefix + noGroupSuffix)

				Eventually(
					common.GetOpPolicyCompMsg("operator-policy"+noGroupSuffix),
					defaultTimeoutSeconds,
					1,
				).Should(MatchRegexp("Compliant.*the OperatorGroup matches what is required by the policy.*" +
					"the Subscription matches what is required by the policy.*"))

				msg := common.RegisterDebugMessage()

				By("Checking if the status of the root policy is compliant")
				Eventually(func(g Gomega) interface{} {
					*msg = "Current compliance condition of OperatorPolicy: " +
						common.GetOpPolicyCompMsg("operator-policy"+noGroupSuffix)()

					return common.GetComplianceState(policyNamePrefix + noGroupSuffix)(g)
				}, defaultTimeoutSeconds*4, 1).Should(Equal(policiesv1.Compliant))
			})

			It("Should verify OperatorGroup details", func() {
				By("Getting the OperatorGroup name from relatedObj field")
				opPolicy := utils.GetWithTimeout(
					clientManagedDynamic,
					common.GvrOperatorPolicy,
					"operator-policy"+noGroupSuffix,
					clusterNamespace,
					true,
					defaultTimeoutSeconds,
				)
				Expect(opPolicy).NotTo(BeNil())

				relObjList, found, err := unstructured.NestedSlice(opPolicy.Object, "status", "relatedObjects")
				Expect(err).ToNot(HaveOccurred())
				Expect(found).Should(BeTrue())

				foundOpGroupName := ""
				for _, relObj := range relObjList {
					relObjMap, ok := relObj.(map[string]interface{})
					if !ok {
						continue
					}

					kind, _, _ := unstructured.NestedString(relObjMap, "object", "kind")

					if kind == "OperatorGroup" {
						foundOpGroupName, _, _ = unstructured.NestedString(relObjMap, "object", "metadata", "name")
					}
				}
				Expect(foundOpGroupName).ToNot(BeEmpty())

				dynamicOpGroupName = foundOpGroupName

				opGroup := utils.GetWithTimeout(
					clientManagedDynamic,
					common.GvrOperatorGroup,
					foundOpGroupName,
					testNS+noGroupSuffix,
					true,
					defaultTimeoutSeconds,
				)
				Expect(opGroup).ToNot(BeNil())
			})

			It("Should verify Subscription details", func() {
				sub := utils.GetWithTimeout(
					clientManagedDynamic,
					common.GvrSubscriptionOLM,
					subName,
					testNS+noGroupSuffix,
					true,
					defaultTimeoutSeconds,
				)
				Expect(sub).NotTo(BeNil())

				By("Parsing the Subscription for the CSV name")
				csvName, found, err := unstructured.NestedString(sub.Object, "status", "installedCSV")
				Expect(err).ToNot(HaveOccurred())
				Expect(found).To(BeTrue())
				Expect(csvName).ToNot(BeEmpty())
				dynamicCSVName = csvName
			})

			It("Should verify CSV details", func() {
				Eventually(func() string {
					csv := utils.GetWithTimeout(
						clientManagedDynamic,
						common.GvrClusterServiceVersion,
						dynamicCSVName,
						testNS+noGroupSuffix,
						true,
						defaultTimeoutSeconds*4,
					)
					Expect(csv).NotTo(BeNil())

					phase, _, _ := unstructured.NestedString(csv.Object, "status", "phase")

					return phase
				}, defaultTimeoutSeconds*4, 1).Should(Equal("Succeeded"))
			})

			It("Should verify the intended operator is installed", func() {
				opDeployment := utils.GetWithTimeout(
					clientManagedDynamic,
					common.GvrDeployment,
					dynamicCSVName, // Operator has the same name as its corresponding csv
					testNS+noGroupSuffix,
					true,
					defaultTimeoutSeconds,
				)
				Expect(opDeployment).NotTo(BeNil())
			})
		})

		Context("When an OperatorGroup is specified", func() {
			var dynamicCSVName string

			BeforeAll(func() {
				_, err := common.OcManaged("create", "ns", testNS+withGroupSuffix)
				Expect(err).ToNot(HaveOccurred())
			})

			AfterAll(func() {
				_, err := common.OcHub(
					"delete",
					"-f",
					policyWithGroupYAML,
					"-n",
					userNamespace,
					"--ignore-not-found=true",
				)
				Expect(err).ToNot(HaveOccurred())

				_, err = common.OcManaged(
					"delete",
					"subscription.operators.coreos.com",
					"quay-operator",
					"-n", testNS+withGroupSuffix,
					"--ignore-not-found=true",
				)
				Expect(err).ToNot(HaveOccurred())

				_, err = common.OcManaged(
					"delete",
					"operatorgroup",
					opGroupName+withGroupSuffix,
					"-n", testNS+withGroupSuffix,
					"--ignore-not-found=true",
				)
				Expect(err).ToNot(HaveOccurred())

				if dynamicCSVName != "" {
					_, err = common.OcManaged(
						"delete",
						"clusterserviceversion",
						dynamicCSVName,
						"-n",
						testNS+withGroupSuffix,
						"--ignore-not-found=true",
					)
					Expect(err).ToNot(HaveOccurred())
				}

				_, err = common.OcManaged("delete", "ns", testNS+withGroupSuffix)
				Expect(err).ToNot(HaveOccurred())
			})

			It(policyNamePrefix+withGroupSuffix+" should be created on the hub", func() {
				_, err := common.OcHub("apply", "-f", policyWithGroupYAML, "-n", userNamespace)
				Expect(err).ToNot(HaveOccurred())

				By("Patching the placement rule")
				err = common.PatchPlacementRule(userNamespace, policyNamePrefix+withGroupSuffix+"-plr")
				Expect(err).ToNot(HaveOccurred())

				By("Checking that it exists on the hub cluster")
				rootPlc := utils.GetWithTimeout(
					clientHubDynamic, common.GvrPolicy, policyNamePrefix+withGroupSuffix,
					userNamespace, true, defaultTimeoutSeconds,
				)
				Expect(rootPlc).NotTo(BeNil())
			})

			It("operator-policy"+withGroupSuffix+" should be created on the managed cluster", func() {
				By("Checking the policy on managed cluster in the namespace " + clusterNamespace)
				managedPolicy := utils.GetWithTimeout(
					clientManagedDynamic,
					common.GvrPolicy,
					userNamespace+"."+policyNamePrefix+withGroupSuffix,
					clusterNamespace,
					true,
					defaultTimeoutSeconds,
				)
				Expect(managedPolicy).NotTo(BeNil())
			})

			It("operator-policy"+withGroupSuffix+" should be NonCompliant", func() {
				By("Checking if the correct condition is generated")
				Eventually(
					common.GetOpPolicyCompMsg("operator-policy"+withGroupSuffix),
					defaultTimeoutSeconds,
					1,
				).Should(MatchRegexp("NonCompliant.*the OperatorGroup required by the policy was not found.*" +
					"the Subscription required by the policy was not found.*"))

				debugMsg := common.RegisterDebugMessage()

				By("Checking if the status of the root policy is NonCompliant")
				Eventually(func(g Gomega) interface{} {
					*debugMsg = "Current compliance condition of OperatorPolicy: " +
						common.GetOpPolicyCompMsg("operator-policy"+withGroupSuffix)()

					return common.GetComplianceState(policyNamePrefix + withGroupSuffix)(g)
				}, defaultTimeoutSeconds*2, 1).Should(Equal(policiesv1.NonCompliant))
			})

			It("Should enforce the policy on the hub", func() {
				common.EnforcePolicy(policyNamePrefix + withGroupSuffix)

				Eventually(
					common.GetOpPolicyCompMsg("operator-policy"+withGroupSuffix),
					defaultTimeoutSeconds,
					1,
				).Should(MatchRegexp("Compliant.*the OperatorGroup matches what is required by the policy.*" +
					"the Subscription matches what is required by the policy.*"))

				debugMsg := common.RegisterDebugMessage()

				By("Checking if the status of the root policy is compliant")
				Eventually(func(g Gomega) interface{} {
					*debugMsg = "Current compliance condition of OperatorPolicy: " +
						common.GetOpPolicyCompMsg("operator-policy"+withGroupSuffix)()

					return common.GetComplianceState(policyNamePrefix + withGroupSuffix)(g)
				}, defaultTimeoutSeconds*2, 1).Should(Equal(policiesv1.Compliant))
			})

			It("Should verify OperatorGroup details", func() {
				opGroup := utils.GetWithTimeout(
					clientManagedDynamic,
					common.GvrOperatorGroup,
					opGroupName+withGroupSuffix,
					testNS+withGroupSuffix,
					true,
					defaultTimeoutSeconds,
				)
				Expect(opGroup).ToNot(BeNil())
			})

			It("Should verify Subscription details", func() {
				sub := utils.GetWithTimeout(
					clientManagedDynamic,
					common.GvrSubscriptionOLM,
					subName,
					testNS+withGroupSuffix,
					true,
					defaultTimeoutSeconds,
				)
				Expect(sub).NotTo(BeNil())

				By("Parsing the Subscription for the CSV name")
				csvName, found, err := unstructured.NestedString(sub.Object, "status", "installedCSV")
				Expect(err).ToNot(HaveOccurred())
				Expect(found).To(BeTrue())
				Expect(csvName).ToNot(BeEmpty())
				dynamicCSVName = csvName
			})

			It("Should verify CSV details", func() {
				Eventually(func() string {
					csv := utils.GetWithTimeout(
						clientManagedDynamic,
						common.GvrClusterServiceVersion,
						dynamicCSVName,
						testNS+withGroupSuffix,
						true,
						defaultTimeoutSeconds*4,
					)
					Expect(csv).NotTo(BeNil())

					phase, _, _ := unstructured.NestedString(csv.Object, "status", "phase")

					return phase
				}, defaultTimeoutSeconds*4, 1).Should(Equal("Succeeded"))
			})

			It("Should verify the intended operator is installed", func() {
				opDeployment := utils.GetWithTimeout(
					clientManagedDynamic,
					common.GvrDeployment,
					dynamicCSVName, // Operator has the same name as its corresponding csv
					testNS+withGroupSuffix,
					true,
					defaultTimeoutSeconds,
				)
				Expect(opDeployment).NotTo(BeNil())
			})
		})
	})
