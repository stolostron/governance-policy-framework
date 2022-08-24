// Copyright (c) 2020 Red Hat, Inc.
// Copyright Contributors to the Open Cluster Management project

package integration

import (
	"context"
	"fmt"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	policiesv1 "open-cluster-management.io/governance-policy-propagator/api/v1"
	"open-cluster-management.io/governance-policy-propagator/test/utils"

	"github.com/stolostron/governance-policy-framework/test/common"
)

func isOCP46andAbove() bool {
	clusterVersion, err := clientManagedDynamic.Resource(common.GvrClusterVersion).Get(context.TODO(), "version", metav1.GetOptions{})
	if err != nil && errors.IsNotFound(err) {
		// no version CR, not ocp
		fmt.Println("This is not an OCP cluster")
		return false
	}
	version := clusterVersion.Object["status"].(map[string]interface{})["desired"].(map[string]interface{})["version"].(string)
	fmt.Println("OCP Version " + version)
	if strings.HasPrefix(version, "4.3") || strings.HasPrefix(version, "4.4") || strings.HasPrefix(version, "4.5") {
		// not ocp 4.3, 4.4 or 4.5
		return false
	}
	// should be ocp 4.6 and above
	return true
}

func complianceScanTest(scanPolicyName string, scanPolicyUrl string, scanName string) {
	Describe("create and enforce the stable/"+scanPolicyName+" policy", Ordered, Label("BVT"), func() {
		It("stable/"+scanPolicyName+" should be created on hub", func() {
			By("Creating policy on hub")
			utils.KubectlWithOutput("apply", "-f", scanPolicyUrl, "-n", userNamespace, "--kubeconfig="+kubeconfigHub)
			By("Patching placement rule")
			common.PatchPlacementRule(userNamespace, "placement-"+scanPolicyName, clusterNamespace, kubeconfigHub)
			By("Checking policy on hub cluster in ns " + userNamespace)
			rootPlc := utils.GetWithTimeout(clientHubDynamic, common.GvrPolicy, scanPolicyName, userNamespace, true, defaultTimeoutSeconds)
			Expect(rootPlc).NotTo(BeNil())
		})
		It("stable/"+scanPolicyName+" should be created on managed cluster", func() {
			By("Checking policy on managed cluster in ns " + clusterNamespace)
			managedplc := utils.GetWithTimeout(clientManagedDynamic, common.GvrPolicy, userNamespace+"."+scanPolicyName, clusterNamespace, true, defaultTimeoutSeconds*2)
			Expect(managedplc).NotTo(BeNil())
		})
		It("Enforcing stable/"+scanPolicyName+"", func() {
			Eventually(func() interface{} {
				By("Patching remediationAction = enforce on root policy")
				rootPlc := utils.GetWithTimeout(clientHubDynamic, common.GvrPolicy, scanPolicyName, userNamespace, true, defaultTimeoutSeconds)
				rootPlc.Object["spec"].(map[string]interface{})["remediationAction"] = "enforce"
				clientHubDynamic.Resource(common.GvrPolicy).Namespace(userNamespace).Update(context.TODO(), rootPlc, metav1.UpdateOptions{})
				By("Checking if remediationAction is enforce for root policy")
				rootPlc = utils.GetWithTimeout(clientHubDynamic, common.GvrPolicy, scanPolicyName, userNamespace, true, defaultTimeoutSeconds)
				return rootPlc.Object["spec"].(map[string]interface{})["remediationAction"]
			}, defaultTimeoutSeconds, 1).Should(Equal("enforce"))
			By("Checking if remediationAction is enforce for replicated policy")
			Eventually(func() interface{} {
				managedPlc := utils.GetWithTimeout(clientManagedDynamic, common.GvrPolicy, userNamespace+"."+scanPolicyName, clusterNamespace, true, defaultTimeoutSeconds)
				return managedPlc.Object["spec"].(map[string]interface{})["remediationAction"]
			}, defaultTimeoutSeconds, 1).Should(Equal("enforce"))
		})
		It("ComplianceSuite "+scanName+" should be created", func() {
			By("Checking if ComplianceSuite " + scanName + " exists on managed cluster")
			compliancesuite := utils.GetWithTimeout(clientManagedDynamic, common.GvrComplianceSuite, scanName, "openshift-compliance", true, defaultTimeoutSeconds*4)
			Expect(compliancesuite).NotTo(BeNil())
			By("Checking if ComplianceSuite " + scanName + " scan status field has been created")
			Eventually(func() interface{} {
				compliancesuite := utils.GetWithTimeout(clientManagedDynamic, common.GvrComplianceSuite, scanName, "openshift-compliance", true, defaultTimeoutSeconds)
				return compliancesuite.Object["status"]
			}, defaultTimeoutSeconds*4, 1).ShouldNot(BeNil())
			By("Checking if ComplianceSuite " + scanName + " scan status.phase is RUNNING")
			Eventually(func() interface{} {
				compliancesuite := utils.GetWithTimeout(clientManagedDynamic, common.GvrComplianceSuite, scanName, "openshift-compliance", true, defaultTimeoutSeconds)
				return compliancesuite.Object["status"].(map[string]interface{})["phase"]
			}, common.MaxTravisTimeoutSeconds, 1).Should(Equal("RUNNING"))
		})
		It("Informing stable/"+scanPolicyName+"", func() {
			Eventually(func() interface{} {
				By("Patching remediationAction = inform on root policy")
				rootPlc := utils.GetWithTimeout(clientHubDynamic, common.GvrPolicy, scanPolicyName, userNamespace, true, defaultTimeoutSeconds)
				rootPlc.Object["spec"].(map[string]interface{})["remediationAction"] = "inform"
				clientHubDynamic.Resource(common.GvrPolicy).Namespace(userNamespace).Update(context.TODO(), rootPlc, metav1.UpdateOptions{})
				By("Checking if remediationAction is inform for root policy")
				rootPlc = utils.GetWithTimeout(clientHubDynamic, common.GvrPolicy, scanPolicyName, userNamespace, true, defaultTimeoutSeconds)
				return rootPlc.Object["spec"].(map[string]interface{})["remediationAction"]
			}, defaultTimeoutSeconds, 1).Should(Equal("inform"))
			By("Checking if remediationAction is inform for replicated policy")
			Eventually(func() interface{} {
				managedPlc := utils.GetWithTimeout(clientManagedDynamic, common.GvrPolicy, userNamespace+"."+scanPolicyName, clusterNamespace, true, defaultTimeoutSeconds)
				return managedPlc.Object["spec"].(map[string]interface{})["remediationAction"]
			}, defaultTimeoutSeconds, 1).Should(Equal("inform"))
		})
	})
	Describe("verify the stable/"+scanPolicyName+" has completed its scan", Ordered, func() {
		It("ComplianceCheckResult should be created", func() {
			By("Checking if any ComplianceCheckResult CR exists on managed cluster")
			Eventually(func(g Gomega) interface{} {
				list, err := clientManagedDynamic.Resource(common.GvrComplianceCheckResult).Namespace("openshift-compliance").List(context.TODO(), metav1.ListOptions{})
				g.Expect(err).To(BeNil())
				return len(list.Items)
			}, common.MaxTravisTimeoutSeconds, 1).ShouldNot(Equal(0))
		})
		It("ComplianceSuite "+scanName+" scan results should be AGGREGATING", func() {
			By("Checking if ComplianceSuite " + scanName + " scan status.phase is AGGREGATING")
			Eventually(func() interface{} {
				compliancesuite := utils.GetWithTimeout(clientManagedDynamic, common.GvrComplianceSuite, scanName, "openshift-compliance", true, defaultTimeoutSeconds)
				return compliancesuite.Object["status"].(map[string]interface{})["phase"]
			}, common.MaxTravisTimeoutSeconds, 1).Should(Equal("AGGREGATING"))
		})
		It("ComplianceSuite "+scanName+" scan results should be DONE", func() {
			By("Checking if ComplianceSuite " + scanName + " scan status.phase is DONE")
			Eventually(func() interface{} {
				compliancesuite := utils.GetWithTimeout(clientManagedDynamic, common.GvrComplianceSuite, scanName, "openshift-compliance", true, defaultTimeoutSeconds)
				return compliancesuite.Object["status"].(map[string]interface{})["phase"]
			}, common.MaxTravisTimeoutSeconds, 1).Should(Equal("DONE"))
		})
	})
	AfterAll(func() {
		By("Removing policy")
		utils.KubectlWithOutput("delete", "-f", scanPolicyUrl, "-n", userNamespace, "--kubeconfig="+kubeconfigHub)
		utils.GetWithTimeout(clientManagedDynamic, common.GvrPolicy, userNamespace+"."+scanPolicyName, clusterNamespace, false, defaultTimeoutSeconds)
		By("Removing ScanSettingBinding")
		out, _ := utils.KubectlWithOutput("delete", "-n", "openshift-compliance", "ScanSettingBinding", scanName, "--kubeconfig="+kubeconfigManaged)
		Expect(out).To(ContainSubstring("scansettingbinding.compliance.openshift.io \"" + scanName + "\" deleted"))
		By("Wait for ComplianceSuite to be deleted")
		utils.KubectlWithOutput("delete", "-n", "openshift-compliance", "ComplianceSuite", scanName, "--kubeconfig="+kubeconfigManaged)
		utils.ListWithTimeoutByNamespace(clientManagedDynamic, common.GvrComplianceSuite, metav1.ListOptions{}, "openshift-compliance", 0, false, defaultTimeoutSeconds)
		By("Wait for compliancecheckresult to be deleted")
		utils.ListWithTimeoutByNamespace(clientManagedDynamic, common.GvrComplianceCheckResult, metav1.ListOptions{}, "openshift-compliance", 0, false, defaultTimeoutSeconds)
		By("Wait for compliancescan to be deleted")
		utils.ListWithTimeoutByNamespace(clientManagedDynamic, common.GvrComplianceScan, metav1.ListOptions{}, "openshift-compliance", 0, false, defaultTimeoutSeconds)
		By("Wait for other pods to be deleted in openshift-compliance ns")
		Eventually(func(g Gomega) interface{} {
			podList, err := clientManaged.CoreV1().Pods("openshift-compliance").List(context.TODO(), metav1.ListOptions{})
			g.Expect(err).To(BeNil())
			return len(podList.Items)
		}, defaultTimeoutSeconds*4, 1).Should(Equal(3))
	})
}

var _ = Describe("RHACM4K-2222 GRC: [P1][Sev1][policy-grc] Test compliance operator and scan", Ordered, Label("policy-collection", "stable"), func() {
	const compPolicyURL = policyCollectCAURL + "policy-compliance-operator-install.yaml"
	const compPolicyName = "policy-comp-operator"
	const compE8ScanPolicyURL = policyCollectCMURL + "policy-compliance-operator-e8-scan.yaml"
	const compE8ScanPolicyName = "policy-e8-scan"
	const compCISScanPolicyURL = policyCollectCMURL + "policy-compliance-operator-cis-scan.yaml"
	const compCISScanPolicyName = "policy-cis-scan"

	var getComplianceState func(Gomega) interface{}

	BeforeAll(func() {
		if !isOCP46andAbove() {
			Skip("Skipping as compliance operator is only supported on OCP 4.6 and above")
		}
		if !canCreateOpenshiftNamespaces() {
			Skip("Skipping as compliance operator requires the ability to create the openshift-compliance namespace")
		}

		// Assign this here to avoid using nil pointers as arguments
		getComplianceState = common.GetComplianceState(clientHubDynamic, userNamespace, compPolicyName, clusterNamespace)
	})
	Describe("Test stable/"+compPolicyName, Label("BVT"), func() {
		It("stable/"+compPolicyName+" should be created on hub", func() {
			By("Creating policy on hub")
			utils.KubectlWithOutput("apply", "-f", compPolicyURL, "-n", userNamespace, "--kubeconfig="+kubeconfigHub)
			By("Patching placement rule")
			common.PatchPlacementRule(userNamespace, "placement-"+compPolicyName, clusterNamespace, kubeconfigHub)
			By("Checking " + compPolicyName + " on hub cluster in ns " + userNamespace)
			rootPlc := utils.GetWithTimeout(clientHubDynamic, common.GvrPolicy, compPolicyName, userNamespace, true, defaultTimeoutSeconds)
			Expect(rootPlc).NotTo(BeNil())
		})
		It("stable/"+compPolicyName+" should be created on managed cluster", func() {
			By("Checking " + compPolicyName + " on managed cluster in ns " + clusterNamespace)
			managedplc := utils.GetWithTimeout(clientManagedDynamic, common.GvrPolicy, userNamespace+"."+compPolicyName, clusterNamespace, true, defaultTimeoutSeconds*2)
			Expect(managedplc).NotTo(BeNil())
		})
		It("stable/"+compPolicyName+" should be noncompliant", func() {
			By("Checking if the status of root policy is noncompliant")
			Eventually(getComplianceState, defaultTimeoutSeconds*2, 1).Should(Equal(policiesv1.NonCompliant))
		})
		It("Enforcing stable/"+compPolicyName, func() {
			Eventually(func() interface{} {
				By("Patching remediationAction = enforce on root policy")
				rootPlc := utils.GetWithTimeout(clientHubDynamic, common.GvrPolicy, compPolicyName, userNamespace, true, defaultTimeoutSeconds)
				rootPlc.Object["spec"].(map[string]interface{})["remediationAction"] = "enforce"
				clientHubDynamic.Resource(common.GvrPolicy).Namespace(userNamespace).Update(context.TODO(), rootPlc, metav1.UpdateOptions{})
				By("Checking if remediationAction is enforce for root policy")
				rootPlc = utils.GetWithTimeout(clientHubDynamic, common.GvrPolicy, compPolicyName, userNamespace, true, defaultTimeoutSeconds)
				return rootPlc.Object["spec"].(map[string]interface{})["remediationAction"]
			}, defaultTimeoutSeconds, 1).Should(Equal("enforce"))
			By("Checking if remediationAction is enforce for replicated policy")
			Eventually(func() interface{} {
				managedPlc := utils.GetWithTimeout(clientManagedDynamic, common.GvrPolicy, userNamespace+"."+compPolicyName, clusterNamespace, true, defaultTimeoutSeconds)
				return managedPlc.Object["spec"].(map[string]interface{})["remediationAction"]
			}, defaultTimeoutSeconds, 1).Should(Equal("enforce"))
		})
		It("Compliance operator pod should be running", func() {
			By("Checking if pod compliance-operator has been created")
			var i int = 0
			Eventually(func(g Gomega) interface{} {
				if i == 60*2 || i == 60*4 {
					fmt.Println("compliance operator pod still not created, deleting subscription and let it recreate", i)
					utils.KubectlWithOutput("get", "-n", "openshift-compliance", "subscriptions.operators.coreos.com", "compliance-operator", "-oyaml", "--kubeconfig="+kubeconfigManaged)
					utils.KubectlWithOutput("delete", "-n", "openshift-compliance", "subscriptions.operators.coreos.com", "compliance-operator", "--kubeconfig="+kubeconfigManaged)
				}
				i++
				podList, err := clientManaged.CoreV1().Pods("openshift-compliance").List(context.TODO(), metav1.ListOptions{LabelSelector: "name=compliance-operator"})
				g.Expect(err).To(BeNil())
				return len(podList.Items)
			}, defaultTimeoutSeconds*12, 1).Should(Equal(1))
			By("Checking if pod compliance-operator is running")
			Eventually(func(g Gomega) interface{} {
				podList, err := clientManaged.CoreV1().Pods("openshift-compliance").List(context.TODO(), metav1.ListOptions{LabelSelector: "name=compliance-operator"})
				g.Expect(err).To(BeNil())
				return string(podList.Items[0].Status.Phase)
			}, defaultTimeoutSeconds*6, 1).Should(Equal("Running"))
		})
		It("Profile bundle pods should be running", func() {
			By("Checking if pod ocp4-pp has been created")
			Eventually(func(g Gomega) interface{} {
				podList, err := clientManaged.CoreV1().Pods("openshift-compliance").List(context.TODO(), metav1.ListOptions{LabelSelector: "profile-bundle=ocp4"})
				g.Expect(err).To(BeNil())
				return len(podList.Items)
			}, defaultTimeoutSeconds*6, 1).Should(Equal(1))
			By("Checking if pod ocp4-pp is running")
			Eventually(func(g Gomega) interface{} {
				podList, err := clientManaged.CoreV1().Pods("openshift-compliance").List(context.TODO(), metav1.ListOptions{LabelSelector: "profile-bundle=ocp4"})
				g.Expect(err).To(BeNil())
				return string(podList.Items[0].Status.Phase)
			}, defaultTimeoutSeconds*8, 1).Should(Equal("Running"))
			By("Checking if pod rhcos4-pp has been created")
			Eventually(func(g Gomega) interface{} {
				podList, err := clientManaged.CoreV1().Pods("openshift-compliance").List(context.TODO(), metav1.ListOptions{LabelSelector: "profile-bundle=rhcos4"})
				g.Expect(err).To(BeNil())
				return len(podList.Items)
			}, defaultTimeoutSeconds*6, 1).Should(Equal(1))
			By("Checking if pod rhcos4-pp is running")
			Eventually(func(g Gomega) interface{} {
				podList, err := clientManaged.CoreV1().Pods("openshift-compliance").List(context.TODO(), metav1.ListOptions{LabelSelector: "profile-bundle=rhcos4"})
				g.Expect(err).To(BeNil())
				return string(podList.Items[0].Status.Phase)
			}, defaultTimeoutSeconds*8, 1).Should(Equal("Running"))
		})
		It("stable/"+compPolicyName+" should be compliant", func() {
			By("Checking if the status of root policy is compliant")
			Eventually(getComplianceState, defaultTimeoutSeconds*4, 1).Should(Equal(policiesv1.Compliant))
		})
		It("Informing stable/"+compPolicyName, func() {
			Eventually(func() interface{} {
				By("Patching remediationAction = inform on root policy")
				rootPlc := utils.GetWithTimeout(clientHubDynamic, common.GvrPolicy, compPolicyName, userNamespace, true, defaultTimeoutSeconds)
				rootPlc.Object["spec"].(map[string]interface{})["remediationAction"] = "inform"
				clientHubDynamic.Resource(common.GvrPolicy).Namespace(userNamespace).Update(context.TODO(), rootPlc, metav1.UpdateOptions{})
				By("Checking if remediationAction is inform for root policy")
				rootPlc = utils.GetWithTimeout(clientHubDynamic, common.GvrPolicy, compPolicyName, userNamespace, true, defaultTimeoutSeconds)
				return rootPlc.Object["spec"].(map[string]interface{})["remediationAction"]
			}, defaultTimeoutSeconds, 1).Should(Equal("inform"))
			By("Checking if remediationAction is inform for replicated policy")
			Eventually(func() interface{} {
				managedPlc := utils.GetWithTimeout(clientManagedDynamic, common.GvrPolicy, userNamespace+"."+compPolicyName, clusterNamespace, true, defaultTimeoutSeconds)
				return managedPlc.Object["spec"].(map[string]interface{})["remediationAction"]
			}, defaultTimeoutSeconds, 1).Should(Equal("inform"))
		})
	})
	Describe("Test stable/"+compE8ScanPolicyName, Ordered, func() {
		complianceScanTest(compE8ScanPolicyName, compE8ScanPolicyURL, "e8")
	})
	Describe("Test stable/"+compCISScanPolicyName, Ordered, func() {
		complianceScanTest(compCISScanPolicyName, compCISScanPolicyURL, "cis")
	})
	AfterAll(func() {
		// clean up compliance operator
		utils.KubectlWithOutput("delete", "-f", compPolicyURL, "-n", userNamespace, "--kubeconfig="+kubeconfigHub)
		utils.GetWithTimeout(clientManagedDynamic, common.GvrPolicy, userNamespace+"."+compPolicyName, clusterNamespace, false, defaultTimeoutSeconds)
		utils.KubectlWithOutput("delete", "-n", "openshift-compliance", "ProfileBundle", "--all", "--kubeconfig="+kubeconfigManaged)
		utils.KubectlWithOutput("delete", "-n", "openshift-compliance", "subscriptions.operators.coreos.com", "compliance-operator", "--kubeconfig="+kubeconfigManaged)
		utils.KubectlWithOutput("delete", "-n", "openshift-compliance", "OperatorGroup", "compliance-operator", "--kubeconfig="+kubeconfigManaged)
		out, _ := utils.KubectlWithOutput("delete", "ns", "openshift-compliance", "--kubeconfig="+kubeconfigManaged)
		Expect(out).To(ContainSubstring("namespace \"openshift-compliance\" deleted"))
		utils.KubectlWithOutput("delete", "events", "-n", clusterNamespace, "--field-selector=involvedObject.name="+userNamespace+"."+compPolicyName, "--kubeconfig="+kubeconfigManaged)
		utils.KubectlWithOutput("delete", "events", "-n", clusterNamespace, "--field-selector=involvedObject.name="+userNamespace+"."+compCISScanPolicyName, "--kubeconfig="+kubeconfigManaged)
		utils.KubectlWithOutput("delete", "events", "-n", clusterNamespace, "--field-selector=involvedObject.name="+userNamespace+"."+compE8ScanPolicyName, "--kubeconfig="+kubeconfigManaged)
	})
})
