// Copyright (c) 2020 Red Hat, Inc.

package e2e

import (
	"context"
	"fmt"
	"os/exec"
	"strings"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	policiesv1 "github.com/open-cluster-management/governance-policy-propagator/pkg/apis/policies/v1"
	"github.com/open-cluster-management/governance-policy-propagator/test/utils"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

func isOCP46andAbove() bool {
	clusterVersion, err := clientManagedDynamic.Resource(gvrClusterVersion).Get(context.TODO(), "version", metav1.GetOptions{})
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

func cleanupRequired() bool {
	_, err := clientManaged.CoreV1().Namespaces().Get(context.TODO(), "openshift-compliance", metav1.GetOptions{})
	if err == nil || !errors.IsNotFound(err) {
		return true
	}
	return false
}

var _ = Describe("Test compliance operator and scan", func() {
	const compPolicyURL = "https://raw.githubusercontent.com/open-cluster-management/policy-collection/master/stable/CA-Security-Assessment-and-Authorization/policy-compliance-operator-install.yaml"
	const compPolicyName = "policy-comp-operator"
	const compE8ScanPolicyURL = "https://raw.githubusercontent.com/open-cluster-management/policy-collection/master/community/CM-Configuration-Management/policy-compliance-operator-e8-scan.yaml"
	const compE8ScanPolicyName = "policy-e8-scan"
	BeforeEach(func() {
		if !isOCP46andAbove() {
			Skip("Skipping as compliance operator is only supported on OCP 4.6 and above")
		}
	})
	Describe("Clean up before all", func() {
		It("clean up compliance scan e8", func() {
			if !cleanupRequired() {
				Skip("Skipping as clean up not needed")
			}
			utils.Kubectl("delete", "-f", compE8ScanPolicyURL, "-n", userNamespace, "--kubeconfig="+kubeconfigHub)
			Eventually(func() interface{} {
				managedPlc := utils.GetWithTimeout(clientManagedDynamic, gvrPolicy, userNamespace+"."+compE8ScanPolicyName, clusterNamespace, false, defaultTimeoutSeconds)
				return managedPlc
			}, defaultTimeoutSeconds, 1).Should(BeNil())
			utils.Kubectl("delete", "-n", "openshift-compliance", "ScanSettingBinding", "e8", "--kubeconfig="+kubeconfigManaged)
			utils.Kubectl("delete", "-n", "openshift-compliance", "ComplianceSuite", "e8", "--kubeconfig="+kubeconfigManaged)
			utils.ListWithTimeoutByNamespace(clientManagedDynamic, gvrComplianceCheckResult, metav1.ListOptions{}, "openshift-compliance", 0, false, defaultTimeoutSeconds)
		})
		It("clean up compliance operator", func() {
			if !cleanupRequired() {
				Skip("Skipping as clean up not needed")
			}
			utils.Kubectl("delete", "-f", compPolicyURL, "-n", userNamespace, "--kubeconfig="+kubeconfigHub)
			Eventually(func() interface{} {
				managedPlc := utils.GetWithTimeout(clientManagedDynamic, gvrPolicy, userNamespace+"."+compPolicyName, clusterNamespace, false, defaultTimeoutSeconds)
				return managedPlc
			}, defaultTimeoutSeconds, 1).Should(BeNil())
			utils.Kubectl("delete", "-n", "openshift-compliance", "ProfileBundle", "--all", "--kubeconfig="+kubeconfigManaged)
			utils.Kubectl("delete", "-n", "openshift-compliance", "subscriptions.operators.coreos.com", "compliance-operator", "--kubeconfig="+kubeconfigManaged)
			utils.Kubectl("delete", "-n", "openshift-compliance", "OperatorGroup", "compliance-operator", "--kubeconfig="+kubeconfigManaged)
			out, _ := exec.Command("kubectl", "delete", "ns", "openshift-compliance", "--kubeconfig="+kubeconfigManaged).CombinedOutput()
			Expect(string(out)).To(ContainSubstring("namespace \"openshift-compliance\" deleted"))
			utils.Kubectl("delete", "events", "-n", clusterNamespace, "--all", "--kubeconfig="+kubeconfigManaged)
		})
	})
	Describe("Test stable/policy-comp-operator", func() {
		It("clean up in case the last build failed", func() {
			By("checking if openshift-compliance ns exists")
			_, err := clientManaged.CoreV1().Namespaces().Get(context.TODO(), "openshift-compliance", metav1.GetOptions{})
			if err == nil || !errors.IsNotFound(err) {
				By("namespace openshift-compliance exists, cleaning up...")
				utils.Kubectl("delete", "-f", compPolicyURL, "-n", userNamespace, "--kubeconfig="+kubeconfigHub)
				Eventually(func() interface{} {
					managedPlc := utils.GetWithTimeout(clientManagedDynamic, gvrPolicy, userNamespace+"."+compPolicyName, clusterNamespace, false, defaultTimeoutSeconds)
					return managedPlc
				}, defaultTimeoutSeconds, 1).Should(BeNil())
				utils.Kubectl("delete", "-n", "openshift-compliance", "ProfileBundle", "--all", "--kubeconfig="+kubeconfigManaged)
				utils.Kubectl("delete", "-n", "openshift-compliance", "subscriptions.operators.coreos.com", "compliance-operator", "--kubeconfig="+kubeconfigManaged)
				utils.Kubectl("delete", "-n", "openshift-compliance", "OperatorGroup", "compliance-operator", "--kubeconfig="+kubeconfigManaged)
				out, _ := exec.Command("kubectl", "delete", "ns", "openshift-compliance", "--kubeconfig="+kubeconfigManaged).CombinedOutput()
				Expect(string(out)).To(ContainSubstring("namespace \"openshift-compliance\" deleted"))
			}
		})
		It("stable/policy-comp-operator should be created on hub", func() {
			By("Creating policy on hub")
			out, _ := exec.Command("kubectl", "apply", "-f", compPolicyURL, "-n", userNamespace, "--kubeconfig="+kubeconfigHub).CombinedOutput()
			fmt.Println(string(out))
			By("Patching placement rule")
			out, _ = exec.Command("kubectl", "patch", "-n", userNamespace, "placementrule.apps.open-cluster-management.io/placement-"+compPolicyName,
				"--type=json", "-p=[{\"op\": \"replace\", \"path\": \"/spec/clusterSelector/matchExpressions\", \"value\":[{\"key\": \"name\", \"operator\": \"In\", \"values\": ["+clusterNamespace+"]}]}]",
				"--kubeconfig="+kubeconfigHub).CombinedOutput()
			fmt.Println(string(out))
			By("Checking policy-comp-operator on hub cluster in ns " + userNamespace)
			rootPlc := utils.GetWithTimeout(clientHubDynamic, gvrPolicy, compPolicyName, userNamespace, true, defaultTimeoutSeconds)
			Expect(rootPlc).NotTo(BeNil())
		})
		It("stable/policy-comp-operator should be created on managed cluster", func() {
			By("Checking policy-comp-operator on managed cluster in ns " + clusterNamespace)
			managedplc := utils.GetWithTimeout(clientManagedDynamic, gvrPolicy, userNamespace+"."+compPolicyName, clusterNamespace, true, defaultTimeoutSeconds)
			Expect(managedplc).NotTo(BeNil())
		})
		It("stable/policy-comp-operator should be noncompliant", func() {
			By("Checking if the status of root policy is noncompliant")
			Eventually(func() interface{} {
				rootPlc := utils.GetWithTimeout(clientHubDynamic, gvrPolicy, compPolicyName, userNamespace, true, defaultTimeoutSeconds)
				var policy policiesv1.Policy
				err := runtime.DefaultUnstructuredConverter.
					FromUnstructured(rootPlc.UnstructuredContent(), &policy)
				Expect(err).To(BeNil())
				for _, statusPerCluster := range policy.Status.Status {
					if statusPerCluster.ClusterNamespace == clusterNamespace {
						return statusPerCluster.ComplianceState
					}
				}
				return nil
			}, defaultTimeoutSeconds*2, 1).Should(Equal(policiesv1.NonCompliant))
		})
		It("Enforcing stable/policy-comp-operator", func() {
			By("Patching remediationAction = enforce on root policy")
			rootPlc := utils.GetWithTimeout(clientHubDynamic, gvrPolicy, compPolicyName, userNamespace, true, defaultTimeoutSeconds)
			rootPlc.Object["spec"].(map[string]interface{})["remediationAction"] = "enforce"
			rootPlc, err := clientHubDynamic.Resource(gvrPolicy).Namespace(userNamespace).Update(context.TODO(), rootPlc, metav1.UpdateOptions{})
			By("Checking if remediationAction is enforce for root policy")
			Expect(err).To(BeNil())
			Eventually(func() interface{} {
				return rootPlc.Object["spec"].(map[string]interface{})["remediationAction"]
			}, defaultTimeoutSeconds, 1).Should(Equal("enforce"))
			By("Checking if remediationAction is enforce for replicated policy")
			Expect(err).To(BeNil())
			Eventually(func() interface{} {
				managedPlc := utils.GetWithTimeout(clientManagedDynamic, gvrPolicy, userNamespace+"."+compPolicyName, clusterNamespace, true, defaultTimeoutSeconds)
				return managedPlc.Object["spec"].(map[string]interface{})["remediationAction"]
			}, defaultTimeoutSeconds, 1).Should(Equal("enforce"))
		})
		It("stable/policy-comp-operator should be compliant", func() {
			By("Checking if the status of root policy is compliant")
			Eventually(func() interface{} {
				rootPlc := utils.GetWithTimeout(clientHubDynamic, gvrPolicy, compPolicyName, userNamespace, true, defaultTimeoutSeconds)
				var policy policiesv1.Policy
				err := runtime.DefaultUnstructuredConverter.
					FromUnstructured(rootPlc.UnstructuredContent(), &policy)
				Expect(err).To(BeNil())
				for _, statusPerCluster := range policy.Status.Status {
					if statusPerCluster.ClusterNamespace == clusterNamespace {
						return statusPerCluster.ComplianceState
					}
				}
				return nil
			}, defaultTimeoutSeconds*4, 1).Should(Equal(policiesv1.Compliant))
		})
		It("Compliance operator pod should be running", func() {
			By("Checking if pod compliance-operator has been created")
			Eventually(func() interface{} {
				podList, err := clientManaged.CoreV1().Pods("openshift-compliance").List(context.TODO(), metav1.ListOptions{LabelSelector: "name=compliance-operator"})
				Expect(err).To(BeNil())
				return len(podList.Items)
			}, defaultTimeoutSeconds*2, 1).Should(Equal(1))
			By("Checking if pod compliance-operator is running")
			Eventually(func() interface{} {
				podList, err := clientManaged.CoreV1().Pods("openshift-compliance").List(context.TODO(), metav1.ListOptions{LabelSelector: "name=compliance-operator"})
				Expect(err).To(BeNil())
				return string(podList.Items[0].Status.Phase)
			}, defaultTimeoutSeconds*2, 1).Should(Equal("Running"))
		})
		It("Profile bundle pods should be running", func() {
			By("Checking if pod ocp4-pp has been created")
			Eventually(func() interface{} {
				podList, err := clientManaged.CoreV1().Pods("openshift-compliance").List(context.TODO(), metav1.ListOptions{LabelSelector: "profile-bundle=ocp4"})
				Expect(err).To(BeNil())
				return len(podList.Items)
			}, defaultTimeoutSeconds*4, 1).Should(Equal(1))
			By("Checking if pod ocp4-pp is running")
			Eventually(func() interface{} {
				podList, err := clientManaged.CoreV1().Pods("openshift-compliance").List(context.TODO(), metav1.ListOptions{LabelSelector: "profile-bundle=ocp4"})
				Expect(err).To(BeNil())
				return string(podList.Items[0].Status.Phase)
			}, defaultTimeoutSeconds*4, 1).Should(Equal("Running"))
			By("Checking if pod rhcos4-pp has been created")
			Eventually(func() interface{} {
				podList, err := clientManaged.CoreV1().Pods("openshift-compliance").List(context.TODO(), metav1.ListOptions{LabelSelector: "profile-bundle=rhcos4"})
				Expect(err).To(BeNil())
				return len(podList.Items)
			}, defaultTimeoutSeconds*4, 1).Should(Equal(1))
			By("Checking if pod rhcos4-pp is running")
			Eventually(func() interface{} {
				podList, err := clientManaged.CoreV1().Pods("openshift-compliance").List(context.TODO(), metav1.ListOptions{LabelSelector: "profile-bundle=rhcos4"})
				Expect(err).To(BeNil())
				return string(podList.Items[0].Status.Phase)
			}, defaultTimeoutSeconds*4, 1).Should(Equal("Running"))
		})
	})
	Describe("Test community/policy-e8-scan", func() {
		It("community/policy-e8-scan should be created on hub", func() {
			By("Creating policy on hub")
			out, _ := exec.Command("kubectl", "apply", "-f", compE8ScanPolicyURL, "-n", userNamespace, "--kubeconfig="+kubeconfigHub).CombinedOutput()
			fmt.Println(string(out))
			By("Checking policy-e8-scan on hub cluster in ns " + userNamespace)
			rootPlc := utils.GetWithTimeout(clientHubDynamic, gvrPolicy, compE8ScanPolicyName, userNamespace, true, defaultTimeoutSeconds)
			Expect(rootPlc).NotTo(BeNil())
		})
		It("community/policy-e8-scan should be created on managed cluster", func() {
			By("Checking policy-e8-scan on managed cluster in ns " + clusterNamespace)
			managedplc := utils.GetWithTimeout(clientManagedDynamic, gvrPolicy, userNamespace+"."+compE8ScanPolicyName, clusterNamespace, true, defaultTimeoutSeconds)
			Expect(managedplc).NotTo(BeNil())
		})
		It("Enforcing community/policy-e8-scan", func() {
			By("Patching remediationAction = enforce on root policy")
			rootPlc := utils.GetWithTimeout(clientHubDynamic, gvrPolicy, compE8ScanPolicyName, userNamespace, true, defaultTimeoutSeconds)
			rootPlc.Object["spec"].(map[string]interface{})["remediationAction"] = "enforce"
			rootPlc, err := clientHubDynamic.Resource(gvrPolicy).Namespace(userNamespace).Update(context.TODO(), rootPlc, metav1.UpdateOptions{})
			By("Checking if remediationAction is enforce for root policy")
			Expect(err).To(BeNil())
			Eventually(func() interface{} {
				return rootPlc.Object["spec"].(map[string]interface{})["remediationAction"]
			}, defaultTimeoutSeconds, 1).Should(Equal("enforce"))
			By("Checking if remediationAction is enforce for replicated policy")
			Expect(err).To(BeNil())
			Eventually(func() interface{} {
				managedPlc := utils.GetWithTimeout(clientManagedDynamic, gvrPolicy, userNamespace+"."+compE8ScanPolicyName, clusterNamespace, true, defaultTimeoutSeconds)
				return managedPlc.Object["spec"].(map[string]interface{})["remediationAction"]
			}, defaultTimeoutSeconds, 1).Should(Equal("enforce"))
		})
		It("ComplianceSuite e8 should be created", func() {
			By("Checking if ComplianceSuite e8 exists on managed cluster")
			e8 := utils.GetWithTimeout(clientManagedDynamic, gvrComplianceSuite, "e8", "openshift-compliance", true, defaultTimeoutSeconds)
			Expect(e8).NotTo(BeNil())
			By("Checking if ComplianceSuite e8 scan status field has been created")
			Eventually(func() interface{} {
				e8 := utils.GetWithTimeout(clientManagedDynamic, gvrComplianceSuite, "e8", "openshift-compliance", true, defaultTimeoutSeconds)
				return e8.Object["status"]
			}, defaultTimeoutSeconds*4, 1).ShouldNot(BeNil())
			By("Checking if ComplianceSuite e8 scan status.phase is RUNNING")
			Eventually(func() interface{} {
				e8 := utils.GetWithTimeout(clientManagedDynamic, gvrComplianceSuite, "e8", "openshift-compliance", true, defaultTimeoutSeconds)
				return e8.Object["status"].(map[string]interface{})["phase"]
			}, defaultTimeoutSeconds*4, 1).Should(Equal("RUNNING"))
		})
		It("ComplianceCheckResult should be created", func() {
			By("Checking if any ComplianceCheckResult CR exists on managed cluster")
			Eventually(func() interface{} {
				list, err := clientManagedDynamic.Resource(gvrComplianceCheckResult).Namespace("openshift-compliance").List(context.TODO(), metav1.ListOptions{})
				Expect(err).To(BeNil())
				return len(list.Items)
			}, defaultTimeoutSeconds*4, 1).ShouldNot(Equal(0))
		})
		It("ComplianceSuite e8 scan results should be AGGREGATING", func() {
			By("Checking if ComplianceSuite e8 scan status.phase is AGGREGATING")
			Eventually(func() interface{} {
				e8 := utils.GetWithTimeout(clientManagedDynamic, gvrComplianceSuite, "e8", "openshift-compliance", true, defaultTimeoutSeconds)
				return e8.Object["status"].(map[string]interface{})["phase"]
			}, defaultTimeoutSeconds*4, 1).Should(Equal("AGGREGATING"))
		})
		It("ComplianceSuite e8 scan results should be DONE", func() {
			By("Checking if ComplianceSuite e8 scan status.phase is DONE")
			Eventually(func() interface{} {
				e8 := utils.GetWithTimeout(clientManagedDynamic, gvrComplianceSuite, "e8", "openshift-compliance", true, defaultTimeoutSeconds)
				return e8.Object["status"].(map[string]interface{})["phase"]
			}, defaultTimeoutSeconds*4, 1).Should(Equal("DONE"))
		})
	})
	Describe("Clean up after all", func() {
		It("clean up compliance scan e8", func() {
			utils.Kubectl("delete", "-f", compE8ScanPolicyURL, "-n", userNamespace, "--kubeconfig="+kubeconfigHub)
			Eventually(func() interface{} {
				managedPlc := utils.GetWithTimeout(clientManagedDynamic, gvrPolicy, userNamespace+"."+compE8ScanPolicyName, clusterNamespace, false, defaultTimeoutSeconds)
				return managedPlc
			}, defaultTimeoutSeconds, 1).Should(BeNil())
			utils.Kubectl("delete", "-n", "openshift-compliance", "ScanSettingBinding", "e8", "--kubeconfig="+kubeconfigManaged)
			utils.Kubectl("delete", "-n", "openshift-compliance", "ComplianceSuite", "e8", "--kubeconfig="+kubeconfigManaged)
			utils.ListWithTimeoutByNamespace(clientManagedDynamic, gvrComplianceCheckResult, metav1.ListOptions{}, "openshift-compliance", 0, false, defaultTimeoutSeconds)
		})
		It("clean up compliance operator", func() {
			utils.Kubectl("delete", "-f", compPolicyURL, "-n", userNamespace, "--kubeconfig="+kubeconfigHub)
			Eventually(func() interface{} {
				managedPlc := utils.GetWithTimeout(clientManagedDynamic, gvrPolicy, userNamespace+"."+compPolicyName, clusterNamespace, false, defaultTimeoutSeconds)
				return managedPlc
			}, defaultTimeoutSeconds, 1).Should(BeNil())
			utils.Kubectl("delete", "-n", "openshift-compliance", "ProfileBundle", "--all", "--kubeconfig="+kubeconfigManaged)
			utils.Kubectl("delete", "-n", "openshift-compliance", "subscriptions.operators.coreos.com", "compliance-operator", "--kubeconfig="+kubeconfigManaged)
			utils.Kubectl("delete", "-n", "openshift-compliance", "OperatorGroup", "compliance-operator", "--kubeconfig="+kubeconfigManaged)
			out, _ := exec.Command("kubectl", "delete", "ns", "openshift-compliance", "--kubeconfig="+kubeconfigManaged).CombinedOutput()
			Expect(string(out)).To(ContainSubstring("namespace \"openshift-compliance\" deleted"))
			utils.Kubectl("delete", "events", "-n", clusterNamespace, "--all", "--kubeconfig="+kubeconfigManaged)
		})
	})
})
