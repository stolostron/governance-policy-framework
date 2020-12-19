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

var _ = Describe("Test community/policy-gatekeeper-operator", func() {
	Describe("Test installing gatekeeper operator", func() {
		const gatekeeperPolicyURL = "https://raw.githubusercontent.com/open-cluster-management/policy-collection/master/community/CM-Configuration-Management/policy-gatekeeper-operator.yaml"
		const gatekeeperPolicyName = "policy-gatekeeper-operator"
		It("Clean up before all", func() {
			By("checking if openshift-gatekeeper-operator ns exists")
			_, err := clientManaged.CoreV1().Namespaces().Get(context.TODO(), "openshift-gatekeeper-operator", metav1.GetOptions{})
			if err == nil || !errors.IsNotFound(err) {
				By("namespace openshift-gatekeeper-operator exists, cleaning up...")
				utils.Kubectl("delete", "-f", gatekeeperPolicyURL, "-n", userNamespace, "--kubeconfig="+kubeconfigHub)
				Eventually(func() interface{} {
					managedPlc := utils.GetWithTimeout(clientManagedDynamic, gvrPolicy, userNamespace+"."+gatekeeperPolicyName, clusterNamespace, false, defaultTimeoutSeconds)
					return managedPlc
				}, defaultTimeoutSeconds, 1).Should(BeNil())
				utils.Kubectl("delete", "-n", "openshift-gatekeeper-operator", "Gatekeeper", "gatekeeper", "--kubeconfig="+kubeconfigManaged)
				utils.Kubectl("delete", "-n", "openshift-gatekeeper-operator", "subscriptions.operators.coreos.com", "gatekeeper-operator-sub", "--kubeconfig="+kubeconfigManaged)
				utils.Kubectl("delete", "-n", "openshift-gatekeeper-operator", "OperatorGroup", "gatekeeper-operator", "--kubeconfig="+kubeconfigManaged)
				utils.Kubectl("delete", "crd", "gatekeepers.operator.gatekeeper.sh", "--kubeconfig="+kubeconfigManaged)
				out, _ := exec.Command("kubectl", "delete", "ns", "openshift-gatekeeper-operator", "--kubeconfig="+kubeconfigManaged).CombinedOutput()
				Expect(string(out)).To(ContainSubstring("namespace \"openshift-gatekeeper-operator\" deleted"))
			}
		})
		It("community/policy-gatekeeper-operator should be created on hub", func() {
			By("Creating policy on hub")
			out, _ := exec.Command("kubectl", "apply", "-f", gatekeeperPolicyURL, "-n", userNamespace, "--kubeconfig="+kubeconfigHub).CombinedOutput()
			fmt.Println(string(out))
			By("Patching placement rule")
			out, _ = exec.Command("kubectl", "patch", "-n", userNamespace, "placementrule.apps.open-cluster-management.io/placement-"+gatekeeperPolicyName,
				"--type=json", "-p=[{\"op\": \"replace\", \"path\": \"/spec/clusterSelector/matchExpressions\", \"value\":[{\"key\": \"name\", \"operator\": \"In\", \"values\": ["+clusterNamespace+"]}]}]",
				"--kubeconfig="+kubeconfigHub).CombinedOutput()
			fmt.Println(string(out))
			By("Checking policy-gatekeeper-operator on hub cluster in ns " + userNamespace)
			rootPlc := utils.GetWithTimeout(clientHubDynamic, gvrPolicy, gatekeeperPolicyName, userNamespace, true, defaultTimeoutSeconds)
			Expect(rootPlc).NotTo(BeNil())
		})
		It("community/policy-gatekeeper-operator should be created on managed cluster", func() {
			By("Checking policy-gatekeeper-operator on managed cluster in ns " + clusterNamespace)
			managedplc := utils.GetWithTimeout(clientManagedDynamic, gvrPolicy, userNamespace+"."+gatekeeperPolicyName, clusterNamespace, true, defaultTimeoutSeconds)
			Expect(managedplc).NotTo(BeNil())
		})
		It("community/policy-gatekeeper-operator should be noncompliant", func() {
			By("Checking if the status of root policy is noncompliant")
			Eventually(func() interface{} {
				rootPlc := utils.GetWithTimeout(clientHubDynamic, gvrPolicy, gatekeeperPolicyName, userNamespace, true, defaultTimeoutSeconds)
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
		It("Enforcing community/policy-gatekeeper-operator", func() {
			By("Patching remediationAction = enforce on root policy")
			rootPlc := utils.GetWithTimeout(clientHubDynamic, gvrPolicy, gatekeeperPolicyName, userNamespace, true, defaultTimeoutSeconds)
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
				managedPlc := utils.GetWithTimeout(clientManagedDynamic, gvrPolicy, userNamespace+"."+gatekeeperPolicyName, clusterNamespace, true, defaultTimeoutSeconds)
				return managedPlc.Object["spec"].(map[string]interface{})["remediationAction"]
			}, defaultTimeoutSeconds, 1).Should(Equal("enforce"))
		})
		It("community/policy-gatekeeper-operator should be compliant", func() {
			By("Checking if the status of root policy is compliant")
			Eventually(func() interface{} {
				rootPlc := utils.GetWithTimeout(clientHubDynamic, gvrPolicy, gatekeeperPolicyName, userNamespace, true, defaultTimeoutSeconds)
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
		It("Gatekeeper operator pod should be running", func() {
			By("Checking if pod gatekeeper-operator has been created")
			Eventually(func() interface{} {
				podList, err := clientManaged.CoreV1().Pods("openshift-gatekeeper-operator").List(context.TODO(), metav1.ListOptions{LabelSelector: "control-plane=controller-manager"})
				Expect(err).To(BeNil())
				return len(podList.Items)
			}, defaultTimeoutSeconds*2, 1).ShouldNot(Equal(0))
			By("Checking if pod gatekeeper-operator is running")
			Eventually(func() interface{} {
				podList, err := clientManaged.CoreV1().Pods("openshift-gatekeeper-operator").List(context.TODO(), metav1.ListOptions{LabelSelector: "control-plane=controller-manager"})
				Expect(err).To(BeNil())
				for _, item := range podList.Items {
					if strings.HasPrefix(item.ObjectMeta.Name, "gatekeeper-operator-controller-manager") {
						return string(item.Status.Phase)
					}
				}
				return "nil"
			}, defaultTimeoutSeconds*2, 1).Should(Equal("Running"))
		})
		It("Gatekeeper audit pod should be running", func() {
			By("Checking if pod gatekeeper-audit has been created")
			Eventually(func() interface{} {
				podList, err := clientManaged.CoreV1().Pods("openshift-gatekeeper-system").List(context.TODO(), metav1.ListOptions{LabelSelector: "control-plane=audit-controller"})
				Expect(err).To(BeNil())
				return len(podList.Items)
			}, defaultTimeoutSeconds*2, 1).Should(Equal(1))
			By("Checking if pod gatekeeper-audit is running")
			Eventually(func() interface{} {
				podList, err := clientManaged.CoreV1().Pods("openshift-gatekeeper-system").List(context.TODO(), metav1.ListOptions{LabelSelector: "control-plane=audit-controller"})
				Expect(err).To(BeNil())
				return string(podList.Items[0].Status.Phase)
			}, defaultTimeoutSeconds*2, 1).Should(Equal("Running"))
		})

		It("Gatekeeper controller manager pods should be running", func() {
			By("Checking if pod gatekeeper-controller-manager has been created")
			Eventually(func() interface{} {
				podList, err := clientManaged.CoreV1().Pods("openshift-gatekeeper-system").List(context.TODO(), metav1.ListOptions{LabelSelector: "gatekeeper.sh/operation=webhook"})
				Expect(err).To(BeNil())
				return len(podList.Items)
			}, defaultTimeoutSeconds*2, 1).Should(Equal(2))
			By("Checking if pod gatekeeper-controller-manager is running")
			Eventually(func() interface{} {
				podList, err := clientManaged.CoreV1().Pods("openshift-gatekeeper-system").List(context.TODO(), metav1.ListOptions{LabelSelector: "gatekeeper.sh/operation=webhook"})
				Expect(err).To(BeNil())
				return string(podList.Items[0].Status.Phase) + "/" + string(podList.Items[1].Status.Phase)
			}, defaultTimeoutSeconds*2, 1).Should(Equal("Running/Running"))
		})
		It("Informing community/policy-gatekeeper-operator", func() {
			By("Patching remediationAction = inform on root policy")
			rootPlc := utils.GetWithTimeout(clientHubDynamic, gvrPolicy, gatekeeperPolicyName, userNamespace, true, defaultTimeoutSeconds)
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
				managedPlc := utils.GetWithTimeout(clientManagedDynamic, gvrPolicy, userNamespace+"."+gatekeeperPolicyName, clusterNamespace, true, defaultTimeoutSeconds)
				return managedPlc.Object["spec"].(map[string]interface{})["remediationAction"]
			}, defaultTimeoutSeconds, 1).Should(Equal("inform"))
		})
		It("Clean up after all", func() {
			utils.Kubectl("delete", "-f", gatekeeperPolicyURL, "-n", userNamespace, "--kubeconfig="+kubeconfigHub)
			Eventually(func() interface{} {
				managedPlc := utils.GetWithTimeout(clientManagedDynamic, gvrPolicy, userNamespace+"."+gatekeeperPolicyName, clusterNamespace, false, defaultTimeoutSeconds)
				return managedPlc
			}, defaultTimeoutSeconds, 1).Should(BeNil())
			utils.Kubectl("delete", "-n", "openshift-gatekeeper-operator", "Gatekeeper", "gatekeeper", "--kubeconfig="+kubeconfigManaged)
			utils.Kubectl("delete", "-n", "openshift-gatekeeper-operator", "subscriptions.operators.coreos.com", "gatekeeper-operator-sub", "--kubeconfig="+kubeconfigManaged)
			utils.Kubectl("delete", "-n", "openshift-gatekeeper-operator", "OperatorGroup", "gatekeeper-operator", "--kubeconfig="+kubeconfigManaged)
			utils.Kubectl("delete", "crd", "gatekeepers.operator.gatekeeper.sh", "--kubeconfig="+kubeconfigManaged)
			out, _ := exec.Command("kubectl", "delete", "ns", "openshift-gatekeeper-operator", "--kubeconfig="+kubeconfigManaged).CombinedOutput()
			Expect(string(out)).To(ContainSubstring("namespace \"openshift-gatekeeper-operator\" deleted"))
			utils.Kubectl("delete", "events", "-n", clusterNamespace, "--all", "--kubeconfig="+kubeconfigManaged)
		})
	})
})
