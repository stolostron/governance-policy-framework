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

func isOCP46() bool {
	clusterVersion, err := clientManagedDynamic.Resource(gvrClusterVersion).Get(context.TODO(), "version", metav1.GetOptions{})
	if err != nil && errors.IsNotFound(err) {
		// no version CR, not ocp
		fmt.Println("This is not an OCP cluster")
		return false
	}
	version := clusterVersion.Object["status"].(map[string]interface{})["desired"].(map[string]interface{})["version"].(string)
	fmt.Println("OCP Version %s" + version)
	if strings.HasPrefix(version, "4.3") || strings.HasPrefix(version, "4.4") || strings.HasPrefix(version, "4.5") {
		// ocp 4.3, 4.4 or 4.5
		return false
	}
	return true
}

var _ = Describe("Test stable/policy-comp-operator", func() {
	Describe("Test installing compliance operator", func() {
		const compPolicyURL = "https://raw.githubusercontent.com/open-cluster-management/policy-collection/master/stable/CA-Security-Assessment-and-Authorization/policy-compliance-operator-install.yaml"
		const compPolicyName = "policy-comp-operator"
		It("stable/policy-comp-operator should be created on hub", func() {
			By("Creating policy on hub")
			utils.Kubectl("apply", "-f", compPolicyURL, "-n", userNamespace, "--kubeconfig="+kubeconfigHub)
		})
		It("stable/policy-comp-operator on managed cluster", func() {
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
			if !isOCP46() {
				Skip("Skipping as it is not ocp 4.6 cluster")
			}
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
			if !isOCP46() {
				Skip("Skipping as it is not ocp 4.6 cluster")
			}
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
		It("Informing stable/policy-comp-operator", func() {
			By("Patching remediationAction = inform on root policy")
			rootPlc := utils.GetWithTimeout(clientHubDynamic, gvrPolicy, compPolicyName, userNamespace, true, defaultTimeoutSeconds)
			rootPlc.Object["spec"].(map[string]interface{})["remediationAction"] = "inform"
			rootPlc, err := clientHubDynamic.Resource(gvrPolicy).Namespace(userNamespace).Update(context.TODO(), rootPlc, metav1.UpdateOptions{})
			By("Checking if remediationAction is inform for root policy")
			Expect(err).To(BeNil())
			Eventually(func() interface{} {
				return rootPlc.Object["spec"].(map[string]interface{})["remediationAction"]
			}, defaultTimeoutSeconds, 1).Should(Equal("inform"))
			By("Checking if remediationAction is inform for replicated policy")
			Expect(err).To(BeNil())
			Eventually(func() interface{} {
				managedPlc := utils.GetWithTimeout(clientManagedDynamic, gvrPolicy, userNamespace+"."+compPolicyName, clusterNamespace, true, defaultTimeoutSeconds)
				return managedPlc.Object["spec"].(map[string]interface{})["remediationAction"]
			}, defaultTimeoutSeconds, 1).Should(Equal("inform"))
		})
		It("clean up", func() {
			utils.Kubectl("delete", "-f", compPolicyURL, "-n", userNamespace, "--kubeconfig="+kubeconfigHub)
			Eventually(func() interface{} {
				managedPlc := utils.GetWithTimeout(clientManagedDynamic, gvrPolicy, userNamespace+"."+compPolicyName, clusterNamespace, false, defaultTimeoutSeconds)
				return managedPlc
			}, defaultTimeoutSeconds, 1).Should(BeNil())
			utils.Kubectl("delete", "-n", "openshift-compliance", "ProfileBundle", "--all", "--kubeconfig="+kubeconfigManaged)
			utils.Kubectl("delete", "-n", "openshift-compliance", "subscriptions.operators.coreos.com", "compliance-operator", "--kubeconfig="+kubeconfigManaged)
			utils.Kubectl("delete", "-n", "openshift-compliance", "OperatorGroup", "compliance-operator", "--kubeconfig="+kubeconfigManaged)
			out, _ := exec.Command("kubectl", "delete", "ns", "openshift-compliance", "--kubeconfig="+kubeconfigManaged).CombinedOutput()
			Expect(string(out)).To(Equal("namespace \"openshift-compliance\" deleted\n"))
			utils.Kubectl("delete", "events", "-n", clusterNamespace, "--all", "--kubeconfig="+kubeconfigManaged)
		})
	})
})
