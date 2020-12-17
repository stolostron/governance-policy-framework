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

var _ = PDescribe("Test community/policy-gatekeeper-operator", func() {
	Describe("Test installing gatekeeper operator", func() {
		const gatekeeperPolicyURL = "https://raw.githubusercontent.com/open-cluster-management/policy-collection/master/community/CM-Configuration-Management/policy-gatekeeper-operator.yaml"
		const gatekeeperPolicyName = "policy-gatekeeper-operator"
		It("clean up in case the last build failed", func() {
			By("checking if gatekeeper-system ns exists")
			_, err := clientManaged.CoreV1().Namespaces().Get(context.TODO(), "gatekeeper-system", metav1.GetOptions{})
			if err == nil || !errors.IsNotFound(err) {
				By("namespace gatekeeper-system exists, cleaning up...")
				utils.Kubectl("delete", "-f", gatekeeperPolicyURL, "-n", userNamespace, "--kubeconfig="+kubeconfigHub)
				Eventually(func() interface{} {
					managedPlc := utils.GetWithTimeout(clientManagedDynamic, gvrPolicy, userNamespace+"."+gatekeeperPolicyName, clusterNamespace, false, defaultTimeoutSeconds)
					return managedPlc
				}, defaultTimeoutSeconds, 1).Should(BeNil())
				utils.Kubectl("delete", "-n", "gatekeeper-system", "Gatekeeper", "gatekeeper", "--kubeconfig="+kubeconfigManaged)
				utils.Kubectl("delete", "-n", "gatekeeper-system", "subscriptions.operators.coreos.com", "gatekeeper-operator-sub", "--kubeconfig="+kubeconfigManaged)
				utils.Kubectl("delete", "-n", "gatekeeper-system", "OperatorGroup", "gatekeeper-operator", "--kubeconfig="+kubeconfigManaged)
				utils.Kubectl("delete", "crd", "gatekeepers.operator.gatekeeper.sh", "--kubeconfig="+kubeconfigManaged)
				out, _ := exec.Command("kubectl", "delete", "ns", "gatekeeper-system", "--kubeconfig="+kubeconfigManaged).CombinedOutput()
				Expect(string(out)).To(ContainSubstring("namespace \"gatekeeper-system\" deleted"))
			}
		})
		It("community/policy-gatekeeper-operator should be created on hub", func() {
			By("Creating policy on hub")
			out, _ := exec.Command("kubectl", "apply", "-f", gatekeeperPolicyURL, "-n", userNamespace, "--kubeconfig="+kubeconfigHub).CombinedOutput()
			fmt.Println(string(out))
			By("Patching placement rule")
			out, _ = exec.Command("kubectl", "patch", "-n", userNamespace, "placementrule.apps.open-cluster-management.io/placement-policy-gatekeeper-operator",
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
				podList, err := clientManaged.CoreV1().Pods("gatekeeper-system").List(context.TODO(), metav1.ListOptions{LabelSelector: "control-plane=controller-manager"})
				Expect(err).To(BeNil())
				return len(podList.Items)
			}, defaultTimeoutSeconds*2, 1).ShouldNot(Equal(0))
			By("Checking if pod gatekeeper-operator is running")
			Eventually(func() interface{} {
				podList, err := clientManaged.CoreV1().Pods("gatekeeper-system").List(context.TODO(), metav1.ListOptions{LabelSelector: "control-plane=controller-manager"})
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
				podList, err := clientManaged.CoreV1().Pods("gatekeeper-system").List(context.TODO(), metav1.ListOptions{LabelSelector: "control-plane=audit-controller"})
				Expect(err).To(BeNil())
				return len(podList.Items)
			}, defaultTimeoutSeconds*2, 1).Should(Equal(1))
			By("Checking if pod gatekeeper-audit is running")
			Eventually(func() interface{} {
				podList, err := clientManaged.CoreV1().Pods("gatekeeper-system").List(context.TODO(), metav1.ListOptions{LabelSelector: "control-plane=audit-controller"})
				Expect(err).To(BeNil())
				return string(podList.Items[0].Status.Phase)
			}, defaultTimeoutSeconds*2, 1).Should(Equal("Running"))
		})

		It("Gatekeeper controller manager pods should be running", func() {
			By("Checking if pod gatekeeper-controller-manager has been created")
			Eventually(func() interface{} {
				podList, err := clientManaged.CoreV1().Pods("gatekeeper-system").List(context.TODO(), metav1.ListOptions{LabelSelector: "gatekeeper.sh/operation=webhook"})
				Expect(err).To(BeNil())
				return len(podList.Items)
			}, defaultTimeoutSeconds*2, 1).Should(Equal(2))
			By("Checking if pod gatekeeper-controller-manager is running")
			Eventually(func() interface{} {
				podList, err := clientManaged.CoreV1().Pods("gatekeeper-system").List(context.TODO(), metav1.ListOptions{LabelSelector: "gatekeeper.sh/operation=webhook"})
				Expect(err).To(BeNil())
				return string(podList.Items[0].Status.Phase) + "/" + string(podList.Items[1].Status.Phase)
			}, defaultTimeoutSeconds*2, 1).Should(Equal("Running/Running"))
		})

		It("clean up", func() {
			utils.Kubectl("delete", "-f", gatekeeperPolicyURL, "-n", userNamespace, "--kubeconfig="+kubeconfigHub)
			Eventually(func() interface{} {
				managedPlc := utils.GetWithTimeout(clientManagedDynamic, gvrPolicy, userNamespace+"."+gatekeeperPolicyName, clusterNamespace, false, defaultTimeoutSeconds)
				return managedPlc
			}, defaultTimeoutSeconds, 1).Should(BeNil())
			utils.Kubectl("delete", "-n", "gatekeeper-system", "Gatekeeper", "gatekeeper", "--kubeconfig="+kubeconfigManaged)
			utils.Kubectl("delete", "-n", "gatekeeper-system", "subscriptions.operators.coreos.com", "gatekeeper-operator-sub", "--kubeconfig="+kubeconfigManaged)
			utils.Kubectl("delete", "-n", "gatekeeper-system", "OperatorGroup", "gatekeeper-operator", "--kubeconfig="+kubeconfigManaged)
			utils.Kubectl("delete", "crd", "gatekeepers.operator.gatekeeper.sh", "--kubeconfig="+kubeconfigManaged)
			out, _ := exec.Command("kubectl", "delete", "ns", "gatekeeper-system", "--kubeconfig="+kubeconfigManaged).CombinedOutput()
			Expect(string(out)).To(ContainSubstring("namespace \"gatekeeper-system\" deleted"))
			utils.Kubectl("delete", "events", "-n", clusterNamespace, "--all", "--kubeconfig="+kubeconfigManaged)
		})
	})
})
