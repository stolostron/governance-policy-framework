// Copyright (c) 2020 Red Hat, Inc.
// Copyright Contributors to the Open Cluster Management project

package integration

import (
	"context"
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	policiesv1 "open-cluster-management.io/governance-policy-propagator/api/v1"
	"open-cluster-management.io/governance-policy-propagator/test/utils"

	"github.com/stolostron/governance-policy-framework/test/common"
)

func complianceScanTest(scanPolicyName string, scanPolicyURL string, scanName string) {
	Describe("create and enforce the stable/"+scanPolicyName+" policy", Ordered, Label("BVT"), func() {
		It("stable/"+scanPolicyName+" should be created on hub", func() {
			By("Creating policy on hub")
			_, err := utils.KubectlWithOutput(
				"apply",
				"-f",
				scanPolicyURL,
				"-n",
				userNamespace,
				"--kubeconfig="+kubeconfigHub,
			)
			Expect(err).ToNot(HaveOccurred())

			By("Patching placement rule")
			err = common.PatchPlacementRule(userNamespace, "placement-"+scanPolicyName)
			Expect(err).ToNot(HaveOccurred())

			By("Checking policy on hub cluster in ns " + userNamespace)
			rootPlc := utils.GetWithTimeout(
				clientHubDynamic,
				common.GvrPolicy,
				scanPolicyName,
				userNamespace,
				true,
				defaultTimeoutSeconds,
			)
			Expect(rootPlc).NotTo(BeNil())
		})
		It("stable/"+scanPolicyName+" should be created on managed cluster", func() {
			By("Checking policy on managed cluster in ns " + clusterNamespace)
			managedplc := utils.GetWithTimeout(
				clientManagedDynamic,
				common.GvrPolicy,
				userNamespace+"."+scanPolicyName,
				clusterNamespace,
				true,
				defaultTimeoutSeconds*2,
			)
			Expect(managedplc).NotTo(BeNil())
		})
		It("Enforcing stable/"+scanPolicyName, func() {
			common.EnforcePolicy(scanPolicyName)
		})
		It("ComplianceSuite "+scanName+" should be created", func() {
			By("Checking if ComplianceSuite " + scanName + " exists on managed cluster")
			compliancesuite := utils.GetWithTimeout(
				clientManagedDynamic,
				common.GvrComplianceSuite,
				scanName,
				"openshift-compliance",
				true,
				defaultTimeoutSeconds*4,
			)
			Expect(compliancesuite).NotTo(BeNil())
			By("Checking if ComplianceSuite " + scanName + " scan status field has been created")
			Eventually(func() interface{} {
				compliancesuite := utils.GetWithTimeout(
					clientManagedDynamic,
					common.GvrComplianceSuite,
					scanName,
					"openshift-compliance",
					true,
					defaultTimeoutSeconds,
				)

				return compliancesuite.Object["status"]
			}, defaultTimeoutSeconds*4, 1).ShouldNot(BeNil())
			By("Checking if ComplianceSuite " + scanName + " scan status.phase is RUNNING")
			Eventually(func() interface{} {
				compliancesuite := utils.GetWithTimeout(
					clientManagedDynamic,
					common.GvrComplianceSuite,
					scanName,
					"openshift-compliance",
					true,
					defaultTimeoutSeconds,
				)

				return compliancesuite.Object["status"].(map[string]interface{})["phase"]
			}, common.MaxTimeoutSeconds, 1).Should(Equal("RUNNING"))
		})
		It("Informing stable/"+scanPolicyName+"", func() {
			common.InformPolicy(scanPolicyName)
		})
	})
	Describe("verify the stable/"+scanPolicyName+" has completed its scan", Ordered, func() {
		It("ComplianceCheckResult should be created", func() {
			By("Checking if any ComplianceCheckResult CR exists on managed cluster")
			Eventually(func(g Gomega) []unstructured.Unstructured {
				list, err := clientManagedDynamic.Resource(
					common.GvrComplianceCheckResult).Namespace(
					"openshift-compliance").List(
					context.TODO(),
					metav1.ListOptions{},
				)
				g.Expect(err).ToNot(HaveOccurred())

				return list.Items
			}, common.MaxTimeoutSeconds, 1).ShouldNot(BeEmpty())
		})
		It("ComplianceSuite "+scanName+" scan results should be AGGREGATING", func() {
			By("Checking if ComplianceSuite " + scanName + " scan status.phase is AGGREGATING")
			Eventually(func() interface{} {
				compliancesuite := utils.GetWithTimeout(
					clientManagedDynamic,
					common.GvrComplianceSuite,
					scanName,
					"openshift-compliance",
					true,
					defaultTimeoutSeconds,
				)

				return compliancesuite.Object["status"].(map[string]interface{})["phase"]
			}, common.MaxTimeoutSeconds, 1).Should(Or(Equal("AGGREGATING"), Equal("RUNNING")))
		})
	})
	AfterAll(func() {
		By("Removing policy")
		_, err := utils.KubectlWithOutput(
			"delete",
			"-f",
			scanPolicyURL,
			"-n",
			userNamespace,
			"--kubeconfig="+kubeconfigHub,
			"--ignore-not-found",
		)
		Expect(err).ToNot(HaveOccurred())

		utils.GetWithTimeout(
			clientManagedDynamic,
			common.GvrPolicy,
			userNamespace+"."+scanPolicyName,
			clusterNamespace,
			false,
			defaultTimeoutSeconds,
		)

		By("Removing ScanSettingBinding")
		out, _ := utils.KubectlWithOutput(
			"delete",
			"-n",
			"openshift-compliance",
			"ScanSettingBinding",
			scanName,
			"--kubeconfig="+kubeconfigManaged,
		)
		Expect(out).To(ContainSubstring("scansettingbinding.compliance.openshift.io \"" + scanName + "\" deleted"))
		By("Wait for ComplianceSuite to be deleted")
		_, err = utils.KubectlWithOutput(
			"delete",
			"-n",
			"openshift-compliance",
			"ComplianceSuite",
			scanName,
			"--kubeconfig="+kubeconfigManaged,
			"--ignore-not-found",
		)
		Expect(err).ToNot(HaveOccurred())

		utils.ListWithTimeoutByNamespace(
			clientManagedDynamic,
			common.GvrComplianceSuite,
			metav1.ListOptions{},
			"openshift-compliance",
			0,
			false,
			defaultTimeoutSeconds,
		)
		By("Wait for compliancecheckresult to be deleted")
		utils.ListWithTimeoutByNamespace(
			clientManagedDynamic,
			common.GvrComplianceCheckResult,
			metav1.ListOptions{},
			"openshift-compliance",
			0,
			false,
			defaultTimeoutSeconds,
		)
		By("Wait for compliancescan to be deleted")
		utils.ListWithTimeoutByNamespace(
			clientManagedDynamic,
			common.GvrComplianceScan,
			metav1.ListOptions{},
			"openshift-compliance",
			0,
			false,
			defaultTimeoutSeconds,
		)
	})
}

var _ = Describe("RHACM4K-2222 GRC: [P1][Sev1][policy-grc] "+
	"Test compliance operator and scan", Ordered, Label("policy-collection", "stable"), func() {
	const (
		compPolicyURL         = policyCollectCAURL + "policy-compliance-operator-install.yaml"
		compPolicyName        = "policy-comp-operator"
		compE8scanPolicyURL   = policyCollectCMURL + "policy-compliance-operator-e8-scan.yaml"
		compE8ScanPolicyName  = "policy-e8-scan"
		compCISscanPolicyURL  = policyCollectCMURL + "policy-compliance-operator-cis-scan.yaml"
		compCISScanPolicyName = "policy-cis-scan"
	)

	var getComplianceState func(Gomega) interface{}

	BeforeAll(func() {
		if !common.IsAtLeastVersion("4.6") {
			Skip("Skipping as compliance operator is only supported on OCP 4.6 and above")
		}
		if !canCreateOpenshiftNamespaces() {
			Skip("Skipping as compliance operator requires the ability to create the openshift-compliance namespace")
		}

		// Assign this here to avoid using nil pointers as arguments
		getComplianceState = common.GetComplianceState(compPolicyName)
	})
	Describe("Test stable/"+compPolicyName, Label("BVT"), func() {
		It("stable/"+compPolicyName+" should be created on hub", func() {
			By("Creating policy on hub")
			_, err := utils.KubectlWithOutput(
				"apply", "-f",
				compPolicyURL, "-n",
				userNamespace,
				"--kubeconfig="+kubeconfigHub,
			)
			Expect(err).ToNot(HaveOccurred())
			By("Patching placement rule")
			err = common.PatchPlacementRule(userNamespace, "placement-"+compPolicyName)
			Expect(err).ToNot(HaveOccurred())
			By("Checking " + compPolicyName + " on hub cluster in ns " + userNamespace)
			rootPlc := utils.GetWithTimeout(
				clientHubDynamic,
				common.GvrPolicy,
				compPolicyName,
				userNamespace,
				true,
				defaultTimeoutSeconds,
			)
			Expect(rootPlc).NotTo(BeNil())
		})
		It("stable/"+compPolicyName+" should be created on managed cluster", func() {
			By("Checking " + compPolicyName + " on managed cluster in ns " + clusterNamespace)
			managedplc := utils.GetWithTimeout(
				clientManagedDynamic,
				common.GvrPolicy,
				userNamespace+"."+compPolicyName,
				clusterNamespace,
				true,
				defaultTimeoutSeconds*2,
			)
			Expect(managedplc).NotTo(BeNil())
		})
		It("stable/"+compPolicyName+" should be noncompliant", func() {
			By("Checking if the status of root policy is noncompliant")
			Eventually(getComplianceState, defaultTimeoutSeconds*2, 1).Should(Equal(policiesv1.NonCompliant))
		})
		It("Enforcing stable/"+compPolicyName, func() {
			common.EnforcePolicy(compPolicyName)
		})
		It("Compliance operator pod should be running", func() {
			By("Checking if pod compliance-operator has been created")
			i := 0
			Eventually(func(g Gomega) []corev1.Pod {
				if i == 60*2 || i == 60*4 {
					fmt.Println("compliance operator pod still not created, "+
						"deleting subscription and let it recreate", i)
					_, err := utils.KubectlWithOutput(
						"get", "-n",
						"openshift-compliance",
						"subscriptions.operators.coreos.com",
						"compliance-operator",
						"-oyaml",
						"--kubeconfig="+kubeconfigManaged,
					)
					g.Expect(err).ToNot(HaveOccurred())

					_, err = utils.KubectlWithOutput(
						"delete", "-n",
						"openshift-compliance",
						"subscriptions.operators.coreos.com",
						"compliance-operator",
						"--kubeconfig="+kubeconfigManaged,
						"--ignore-not-found",
					)
					g.Expect(err).ToNot(HaveOccurred())
				}
				i++
				podList, err := clientManaged.CoreV1().Pods("openshift-compliance").List(
					context.TODO(),
					metav1.ListOptions{LabelSelector: "name=compliance-operator"},
				)
				g.Expect(err).ToNot(HaveOccurred())

				return podList.Items
			}, defaultTimeoutSeconds*12, 1).Should(HaveLen(1))
			By("Checking if pod compliance-operator is running")
			Eventually(func(g Gomega) interface{} {
				podList, err := clientManaged.CoreV1().Pods("openshift-compliance").List(
					context.TODO(),
					metav1.ListOptions{LabelSelector: "name=compliance-operator"},
				)
				g.Expect(err).ToNot(HaveOccurred())

				return string(podList.Items[0].Status.Phase)
			}, defaultTimeoutSeconds*6, 1).Should(Equal("Running"))
		})
		It("Profile bundle pods should be running", func() {
			By("Checking if pod ocp4-pp has been created")
			Eventually(func(g Gomega) []corev1.Pod {
				podList, err := clientManaged.CoreV1().Pods("openshift-compliance").List(
					context.TODO(),
					metav1.ListOptions{LabelSelector: "profile-bundle=ocp4"},
				)
				g.Expect(err).ToNot(HaveOccurred())

				return podList.Items
			}, defaultTimeoutSeconds*6, 1).Should(HaveLen(1))
			By("Checking if pod ocp4-pp is running")
			Eventually(func(g Gomega) interface{} {
				podList, err := clientManaged.CoreV1().Pods("openshift-compliance").List(
					context.TODO(),
					metav1.ListOptions{LabelSelector: "profile-bundle=ocp4"},
				)
				g.Expect(err).ToNot(HaveOccurred())

				return string(podList.Items[0].Status.Phase)
			}, defaultTimeoutSeconds*8, 1).Should(Equal("Running"))
			By("Checking if pod rhcos4-pp has been created")
			Eventually(func(g Gomega) []corev1.Pod {
				podList, err := clientManaged.CoreV1().Pods("openshift-compliance").List(
					context.TODO(),
					metav1.ListOptions{LabelSelector: "profile-bundle=rhcos4"},
				)
				g.Expect(err).ToNot(HaveOccurred())

				return podList.Items
			}, defaultTimeoutSeconds*6, 1).Should(HaveLen(1))
			By("Checking if pod rhcos4-pp is running")
			Eventually(func(g Gomega) interface{} {
				podList, err := clientManaged.CoreV1().Pods("openshift-compliance").List(
					context.TODO(),
					metav1.ListOptions{
						LabelSelector: "profile-bundle=rhcos4",
					},
				)
				g.Expect(err).ToNot(HaveOccurred())

				return string(podList.Items[0].Status.Phase)
			}, defaultTimeoutSeconds*8, 1).Should(Equal("Running"))
		})
		It("stable/"+compPolicyName+" should be compliant", func() {
			By("Checking if the status of root policy is compliant")
			Eventually(getComplianceState, defaultTimeoutSeconds*4, 1).Should(Equal(policiesv1.Compliant))
		})
		It("Informing stable/"+compPolicyName, func() {
			common.InformPolicy(compPolicyName)
		})
	})
	Describe("Test stable/"+compE8ScanPolicyName, Ordered, func() {
		complianceScanTest(compE8ScanPolicyName, compE8scanPolicyURL, "e8")
	})
	Describe("Test stable/"+compCISScanPolicyName, Ordered, func() {
		complianceScanTest(compCISScanPolicyName, compCISscanPolicyURL, "cis")
	})
	AfterAll(func() {
		// clean up compliance operator
		_, err := utils.KubectlWithOutput(
			"delete", "-f",
			compPolicyURL,
			"-n",
			userNamespace,
			"--kubeconfig="+kubeconfigHub,
			"--ignore-not-found",
		)
		Expect(err).ToNot(HaveOccurred())

		utils.GetWithTimeout(
			clientManagedDynamic,
			common.GvrPolicy,
			userNamespace+"."+compPolicyName,
			clusterNamespace,
			false,
			defaultTimeoutSeconds,
		)
		_, err = utils.KubectlWithOutput(
			"delete", "-n",
			"openshift-compliance",
			"ProfileBundle", "--all",
			"--kubeconfig="+kubeconfigManaged,
			"--ignore-not-found",
		)
		Expect(err).ToNot(HaveOccurred())

		_, err = utils.KubectlWithOutput(
			"delete", "-n",
			"openshift-compliance",
			"subscriptions.operators.coreos.com",
			"compliance-operator",
			"--kubeconfig="+kubeconfigManaged,
			"--ignore-not-found",
		)
		Expect(err).ToNot(HaveOccurred())

		_, err = utils.KubectlWithOutput(
			"delete", "-n",
			"openshift-compliance",
			"OperatorGroup",
			"compliance-operator",
			"--kubeconfig="+kubeconfigManaged,
			"--ignore-not-found",
		)
		Expect(err).ToNot(HaveOccurred())

		out, _ := utils.KubectlWithOutput(
			"delete", "ns",
			"openshift-compliance",
			"--kubeconfig="+kubeconfigManaged,
		)
		Expect(out).To(ContainSubstring("namespace \"openshift-compliance\" deleted"))

		_, err = utils.KubectlWithOutput(
			"delete", "events", "-n",
			clusterNamespace,
			"--field-selector=involvedObject.name="+userNamespace+"."+compPolicyName,
			"--kubeconfig="+kubeconfigManaged,
			"--ignore-not-found",
		)
		Expect(err).ToNot(HaveOccurred())

		_, err = utils.KubectlWithOutput(
			"delete", "events", "-n",
			clusterNamespace,
			"--field-selector=involvedObject.name="+userNamespace+"."+compCISScanPolicyName,
			"--kubeconfig="+kubeconfigManaged,
			"--ignore-not-found",
		)
		Expect(err).ToNot(HaveOccurred())

		_, err = utils.KubectlWithOutput(
			"delete", "events", "-n",
			clusterNamespace,
			"--field-selector=involvedObject.name="+userNamespace+"."+compE8ScanPolicyName,
			"--kubeconfig="+kubeconfigManaged,
			"--ignore-not-found",
		)
		Expect(err).ToNot(HaveOccurred())
	})
})
