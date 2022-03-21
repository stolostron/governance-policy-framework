// Copyright (c) 2020 Red Hat, Inc.
// Copyright Contributors to the Open Cluster Management project

package integration

import (
	"context"
	"fmt"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	policiesv1 "github.com/stolostron/governance-policy-propagator/api/v1"
	"github.com/stolostron/governance-policy-propagator/test/utils"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

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

var (
	canCreateOpenshiftNamespacesInitialized bool
	canCreateOpenshiftNamespacesResult      bool
)

func canCreateOpenshiftNamespaces() bool {
	// Only check once - this makes it faster and prevents the answer changing mid-suite.
	if canCreateOpenshiftNamespacesInitialized {
		return canCreateOpenshiftNamespacesResult
	}

	canCreateOpenshiftNamespacesInitialized = true
	canCreateOpenshiftNamespacesResult = false

	// A server-side dry run will check the admission webhooks, but it needs to use the right
	// serviceaccount because the kubeconfig might have superuser privileges to get around them.
	out, _ := utils.KubectlWithOutput("create", "ns", "openshift-grc-test", "--kubeconfig="+kubeconfigManaged,
		"--dry-run=server", "--as=system:serviceaccount:open-cluster-management-agent-addon:config-policy-controller-sa")
	if strings.Contains(out, "namespace/openshift-grc-test created") {
		canCreateOpenshiftNamespacesResult = true
	}
	if strings.Contains(out, "namespaces \"openshift-grc-test\" already exists") {
		// Weird situation, but probably means it could make the namespace
		canCreateOpenshiftNamespacesResult = true
	}
	if strings.Contains(out, "admission webhook \"mutation.gatekeeper.sh\" does not support dry run") {
		// Gatekeeper is installed, so assume the namespace could be created
		canCreateOpenshiftNamespacesResult = true
	}
	return canCreateOpenshiftNamespacesResult
}

func complianceScanTest(scanPolicyName string, scanPolicyUrl string, scanName string) {
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
		}, defaultTimeoutSeconds*4, 1).Should(Equal("RUNNING"))
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
	It("ComplianceCheckResult should be created", func() {
		By("Checking if any ComplianceCheckResult CR exists on managed cluster")
		Eventually(func() interface{} {
			list, err := clientManagedDynamic.Resource(common.GvrComplianceCheckResult).Namespace("openshift-compliance").List(context.TODO(), metav1.ListOptions{})
			Expect(err).To(BeNil())
			return len(list.Items)
		}, defaultTimeoutSeconds*12, 1).ShouldNot(Equal(0))
	})
	It("ComplianceSuite "+scanName+" scan results should be AGGREGATING", func() {
		By("Checking if ComplianceSuite " + scanName + " scan status.phase is AGGREGATING")
		Eventually(func() interface{} {
			compliancesuite := utils.GetWithTimeout(clientManagedDynamic, common.GvrComplianceSuite, scanName, "openshift-compliance", true, defaultTimeoutSeconds)
			return compliancesuite.Object["status"].(map[string]interface{})["phase"]
		}, defaultTimeoutSeconds*10, 1).Should(Equal("AGGREGATING"))
	})
	It("ComplianceSuite "+scanName+" scan results should be DONE", func() {
		By("Checking if ComplianceSuite " + scanName + " scan status.phase is DONE")
		Eventually(func() interface{} {
			compliancesuite := utils.GetWithTimeout(clientManagedDynamic, common.GvrComplianceSuite, scanName, "openshift-compliance", true, defaultTimeoutSeconds)
			return compliancesuite.Object["status"].(map[string]interface{})["phase"]
		}, defaultTimeoutSeconds*10, 1).Should(Equal("DONE"))
	})
	It("Clean up compliance scan "+scanName+"", func() {
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
		Eventually(func() interface{} {
			podList, err := clientManaged.CoreV1().Pods("openshift-compliance").List(context.TODO(), metav1.ListOptions{})
			Expect(err).To(BeNil())
			return len(podList.Items)
		}, defaultTimeoutSeconds*4, 1).Should(Equal(3))
	})
}

var _ = Describe("RHACM4K-2222 GRC: [P1][Sev1][policy-grc] Test compliance operator and scan", Label("policy-collection", "stable", "BVT"), func() {
	const policyCollectStableURL = "https://raw.githubusercontent.com/stolostron/policy-collection/main/stable"
	const compPolicyURL = policyCollectStableURL + "/CA-Security-Assessment-and-Authorization/policy-compliance-operator-install.yaml"
	const compPolicyName = "policy-comp-operator"
	var getComplianceState func() interface{}

	BeforeEach(func() {
		if !isOCP46andAbove() {
			Skip("Skipping as compliance operator is only supported on OCP 4.6 and above")
		}
		if !canCreateOpenshiftNamespaces() {
			Skip("Skipping as compliance operator requires the ability to create the openshift-compliance namespace")
		}

		// Assign this here to avoid using nil pointers as arguments
		getComplianceState = common.GetComplianceState(clientHubDynamic, userNamespace, compPolicyName, clusterNamespace)
	})
	Describe("Test stable/policy-comp-operator", func() {
		It("stable/policy-comp-operator should be created on hub", func() {
			By("Creating policy on hub")
			utils.KubectlWithOutput("apply", "-f", compPolicyURL, "-n", userNamespace, "--kubeconfig="+kubeconfigHub)
			By("Patching placement rule")
			common.PatchPlacementRule(userNamespace, "placement-"+compPolicyName, clusterNamespace, kubeconfigHub)
			By("Checking policy-comp-operator on hub cluster in ns " + userNamespace)
			rootPlc := utils.GetWithTimeout(clientHubDynamic, common.GvrPolicy, compPolicyName, userNamespace, true, defaultTimeoutSeconds)
			Expect(rootPlc).NotTo(BeNil())
		})
		It("stable/policy-comp-operator should be created on managed cluster", func() {
			By("Checking policy-comp-operator on managed cluster in ns " + clusterNamespace)
			managedplc := utils.GetWithTimeout(clientManagedDynamic, common.GvrPolicy, userNamespace+"."+compPolicyName, clusterNamespace, true, defaultTimeoutSeconds*2)
			Expect(managedplc).NotTo(BeNil())
		})
		It("stable/policy-comp-operator should be noncompliant", func() {
			By("Checking if the status of root policy is noncompliant")
			Eventually(getComplianceState, defaultTimeoutSeconds*2, 1).Should(Equal(policiesv1.NonCompliant))
		})
		It("Enforcing stable/policy-comp-operator", func() {
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
			Eventually(func() interface{} {
				if i == 60*2 || i == 60*4 {
					fmt.Println("compliance operator pod still not created, deleting subscription and let it recreate", i)
					utils.KubectlWithOutput("get", "-n", "openshift-compliance", "subscriptions.operators.coreos.com", "compliance-operator", "-oyaml", "--kubeconfig="+kubeconfigManaged)
					utils.KubectlWithOutput("delete", "-n", "openshift-compliance", "subscriptions.operators.coreos.com", "compliance-operator", "--kubeconfig="+kubeconfigManaged)
				}
				i++
				podList, err := clientManaged.CoreV1().Pods("openshift-compliance").List(context.TODO(), metav1.ListOptions{LabelSelector: "name=compliance-operator"})
				Expect(err).To(BeNil())
				return len(podList.Items)
			}, defaultTimeoutSeconds*12, 1).Should(Equal(1))
			By("Checking if pod compliance-operator is running")
			Eventually(func() interface{} {
				podList, err := clientManaged.CoreV1().Pods("openshift-compliance").List(context.TODO(), metav1.ListOptions{LabelSelector: "name=compliance-operator"})
				Expect(err).To(BeNil())
				return string(podList.Items[0].Status.Phase)
			}, defaultTimeoutSeconds*6, 1).Should(Equal("Running"))
		})
		It("Profile bundle pods should be running", func() {
			By("Checking if pod ocp4-pp has been created")
			Eventually(func() interface{} {
				podList, err := clientManaged.CoreV1().Pods("openshift-compliance").List(context.TODO(), metav1.ListOptions{LabelSelector: "profile-bundle=ocp4"})
				Expect(err).To(BeNil())
				return len(podList.Items)
			}, defaultTimeoutSeconds*6, 1).Should(Equal(1))
			By("Checking if pod ocp4-pp is running")
			Eventually(func() interface{} {
				podList, err := clientManaged.CoreV1().Pods("openshift-compliance").List(context.TODO(), metav1.ListOptions{LabelSelector: "profile-bundle=ocp4"})
				Expect(err).To(BeNil())
				return string(podList.Items[0].Status.Phase)
			}, defaultTimeoutSeconds*8, 1).Should(Equal("Running"))
			By("Checking if pod rhcos4-pp has been created")
			Eventually(func() interface{} {
				podList, err := clientManaged.CoreV1().Pods("openshift-compliance").List(context.TODO(), metav1.ListOptions{LabelSelector: "profile-bundle=rhcos4"})
				Expect(err).To(BeNil())
				return len(podList.Items)
			}, defaultTimeoutSeconds*6, 1).Should(Equal(1))
			By("Checking if pod rhcos4-pp is running")
			Eventually(func() interface{} {
				podList, err := clientManaged.CoreV1().Pods("openshift-compliance").List(context.TODO(), metav1.ListOptions{LabelSelector: "profile-bundle=rhcos4"})
				Expect(err).To(BeNil())
				return string(podList.Items[0].Status.Phase)
			}, defaultTimeoutSeconds*8, 1).Should(Equal("Running"))
		})
		It("stable/policy-comp-operator should be compliant", func() {
			By("Checking if the status of root policy is compliant")
			Eventually(getComplianceState, defaultTimeoutSeconds*4, 1).Should(Equal(policiesv1.Compliant))
		})
		It("Informing stable/policy-comp-operator", func() {
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
	Describe("Test stable/policy-e8-scan", func() {
		const compE8ScanPolicyURL = policyCollectStableURL + "/CM-Configuration-Management/policy-compliance-operator-e8-scan.yaml"
		const compE8ScanPolicyName = "policy-e8-scan"
		complianceScanTest(compE8ScanPolicyName, compE8ScanPolicyURL, "e8")
	})
	Describe("Test stable/policy-cis-scan", func() {
		const compCISScanPolicyURL = policyCollectStableURL + "/CM-Configuration-Management/policy-compliance-operator-cis-scan.yaml"
		const compCISScanPolicyName = "policy-cis-scan"
		complianceScanTest(compCISScanPolicyName, compCISScanPolicyURL, "cis")
	})
	Describe("Clean up after all", func() {
		It("clean up compliance operator", func() {
			utils.KubectlWithOutput("delete", "-f", compPolicyURL, "-n", userNamespace, "--kubeconfig="+kubeconfigHub)
			utils.GetWithTimeout(clientManagedDynamic, common.GvrPolicy, userNamespace+"."+compPolicyName, clusterNamespace, false, defaultTimeoutSeconds)
			utils.KubectlWithOutput("delete", "-n", "openshift-compliance", "ProfileBundle", "--all", "--kubeconfig="+kubeconfigManaged)
			utils.KubectlWithOutput("delete", "-n", "openshift-compliance", "subscriptions.operators.coreos.com", "compliance-operator", "--kubeconfig="+kubeconfigManaged)
			utils.KubectlWithOutput("delete", "-n", "openshift-compliance", "OperatorGroup", "compliance-operator", "--kubeconfig="+kubeconfigManaged)
			out, _ := utils.KubectlWithOutput("delete", "ns", "openshift-compliance", "--kubeconfig="+kubeconfigManaged)
			Expect(out).To(ContainSubstring("namespace \"openshift-compliance\" deleted"))
			utils.KubectlWithOutput("delete", "events", "-n", clusterNamespace, "--field-selector=involvedObject.name="+userNamespace+".policy-comp-operator", "--kubeconfig="+kubeconfigManaged)
			utils.KubectlWithOutput("delete", "events", "-n", clusterNamespace, "--field-selector=involvedObject.name="+userNamespace+".policy-e8-scan", "--kubeconfig="+kubeconfigManaged)
		})
	})
})
