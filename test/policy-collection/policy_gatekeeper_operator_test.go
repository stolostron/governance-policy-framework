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

func isOCP44() bool {
	clusterVersion, err := clientManagedDynamic.Resource(gvrClusterVersion).Get(context.TODO(), "version", metav1.GetOptions{})
	if err != nil && errors.IsNotFound(err) {
		// no version CR, not ocp
		fmt.Println("This is not an OCP cluster")
		return false
	}
	version := clusterVersion.Object["status"].(map[string]interface{})["desired"].(map[string]interface{})["version"].(string)
	fmt.Println("OCP Version " + version)
	if strings.HasPrefix(version, "4.4") {
		// not ocp 4.3, 4.4 or 4.5
		return true
	}
	// should be ocp 4.6 and above
	return false
}

var _ = Describe("", func() {
	BeforeEach(func() {
		if isOCP44() {
			Skip("Skipping as this is ocp 4.4")
		}
	})
	const gatekeeperPolicyURL = "https://raw.githubusercontent.com/open-cluster-management/policy-collection/master/community/CM-Configuration-Management/policy-gatekeeper-operator.yaml"
	const gatekeeperPolicyName = "policy-gatekeeper-operator"
	const GKPolicyYaml = "https://raw.githubusercontent.com/open-cluster-management/policy-collection/master/community/CM-Configuration-Management/policy-gatekeeper-sample.yaml"
	const GKPolicyName = "policy-gatekeeper"
	Describe("RHACM4K-1692 GRC: [P1][Sev1][policy-grc] Test installing gatekeeper operator", func() {
		It("Clean up before all", func() {
			By("checking if openshift-gatekeeper-operator ns exists")
			_, err := clientManaged.CoreV1().Namespaces().Get(context.TODO(), "openshift-gatekeeper-operator", metav1.GetOptions{})
			if err == nil || !errors.IsNotFound(err) {
				utils.Kubectl("delete", "-f", GKPolicyYaml, "-n", userNamespace, "--kubeconfig="+kubeconfigHub)
				Eventually(func() interface{} {
					managedPlc := utils.GetWithTimeout(clientManagedDynamic, gvrPolicy, userNamespace+"."+GKPolicyName, clusterNamespace, false, defaultTimeoutSeconds)
					return managedPlc
				}, defaultTimeoutSeconds, 1).Should(BeNil())
				utils.Kubectl("delete", "ns", "e2etestsuccess", "--kubeconfig="+kubeconfigManaged)
				utils.Kubectl("delete", "ns", "e2etestfail", "--kubeconfig="+kubeconfigManaged)
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
			Eventually(func() interface{} {
				By("Patching remediationAction = enforce on root policy")
				rootPlc := utils.GetWithTimeout(clientHubDynamic, gvrPolicy, gatekeeperPolicyName, userNamespace, true, defaultTimeoutSeconds)
				rootPlc.Object["spec"].(map[string]interface{})["remediationAction"] = "enforce"
				rootPlc, _ = clientHubDynamic.Resource(gvrPolicy).Namespace(userNamespace).Update(context.TODO(), rootPlc, metav1.UpdateOptions{})
				By("Checking if remediationAction is enforce for root policy")
				rootPlc = utils.GetWithTimeout(clientHubDynamic, gvrPolicy, gatekeeperPolicyName, userNamespace, true, defaultTimeoutSeconds)
				return rootPlc.Object["spec"].(map[string]interface{})["remediationAction"]
			}, defaultTimeoutSeconds, 1).Should(Equal("enforce"))
			By("Checking if remediationAction is enforce for replicated policy")
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
			}, defaultTimeoutSeconds*6, 1).Should(Equal(policiesv1.Compliant))
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
			Eventually(func() interface{} {
				By("Patching remediationAction = inform on root policy")
				rootPlc := utils.GetWithTimeout(clientHubDynamic, gvrPolicy, gatekeeperPolicyName, userNamespace, true, defaultTimeoutSeconds)
				rootPlc.Object["spec"].(map[string]interface{})["remediationAction"] = "inform"
				rootPlc, _ = clientHubDynamic.Resource(gvrPolicy).Namespace(userNamespace).Update(context.TODO(), rootPlc, metav1.UpdateOptions{})
				By("Checking if remediationAction is inform for root policy")
				rootPlc = utils.GetWithTimeout(clientHubDynamic, gvrPolicy, gatekeeperPolicyName, userNamespace, true, defaultTimeoutSeconds)
				return rootPlc.Object["spec"].(map[string]interface{})["remediationAction"]
			}, defaultTimeoutSeconds, 1).Should(Equal("inform"))
			By("Checking if remediationAction is inform for replicated policy")
			Eventually(func() interface{} {
				managedPlc := utils.GetWithTimeout(clientManagedDynamic, gvrPolicy, userNamespace+"."+gatekeeperPolicyName, clusterNamespace, true, defaultTimeoutSeconds)
				return managedPlc.Object["spec"].(map[string]interface{})["remediationAction"]
			}, defaultTimeoutSeconds, 1).Should(Equal("inform"))
		})

	})

	Describe("RHACM4K-1274/RHACM4K-1282 GRC: [P1][Sev1][policy-grc] Test community/policy-gatekeeper-sample", func() {
		It("community/policy-gatekeeper-sample should be created on hub", func() {
			By("Creating policy on hub")
			out, _ := exec.Command("kubectl", "apply", "-f", GKPolicyYaml, "-n", userNamespace, "--kubeconfig="+kubeconfigHub).CombinedOutput()
			fmt.Println(string(out))
			By("Patching placement rule")
			out, _ = exec.Command("kubectl", "patch", "-n", userNamespace, "placementrule.apps.open-cluster-management.io/placement-"+GKPolicyName,
				"--type=json", "-p=[{\"op\": \"replace\", \"path\": \"/spec/clusterSelector/matchExpressions\", \"value\":[{\"key\": \"name\", \"operator\": \"In\", \"values\": ["+clusterNamespace+"]}]}]",
				"--kubeconfig="+kubeconfigHub).CombinedOutput()
			fmt.Println(string(out))
			By("Checking policy-gatekeeper namespace on hub cluster in ns " + userNamespace)
			rootPlc := utils.GetWithTimeout(clientHubDynamic, gvrPolicy, GKPolicyName, userNamespace, true, defaultTimeoutSeconds)
			Expect(rootPlc).NotTo(BeNil())
		})
		It("community/policy-gatekeeper-sample should be compliant", func() {
			By("Checking if the status of root policy is compliant")
			Eventually(func() interface{} {
				rootPlc := utils.GetWithTimeout(clientHubDynamic, gvrPolicy, GKPolicyName, userNamespace, true, defaultTimeoutSeconds)
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
			}, defaultTimeoutSeconds*6, 1).Should(Equal(policiesv1.Compliant))
			By("Checking if status for policy template policy-gatekeeper-audit is compliant")
			Eventually(func() interface{} {
				plc := utils.GetWithTimeout(clientHubDynamic, gvrPolicy, userNamespace+"."+GKPolicyName, clusterNamespace, true, defaultTimeoutSeconds)
				details := plc.Object["status"].(map[string]interface{})["details"].([]interface{})
				return details[1].(map[string]interface{})["compliant"]
			}, defaultTimeoutSeconds, 1).Should(Equal("Compliant"))
		})
		It("Creating a valid ns should not be blocked by gatekeeper", func() {
			By("Creating a namespace called e2etestsuccess on managed")
			Eventually(func() interface{} {
				out, _ := exec.Command("kubectl", "apply", "-f", "../resources/gatekeeper/ns-create-valid.yaml", "--kubeconfig="+kubeconfigManaged).CombinedOutput()
				fmt.Println(string(out))
				return string(out)
			}, defaultTimeoutSeconds*6, 1).Should(ContainSubstring("namespace/e2etestsuccess created"))
		})
		It("Creating an invalid ns should generate a violation message", func() {
			By("Creating invalid namespace on managed")
			Eventually(func() interface{} {
				out, _ := exec.Command("kubectl", "create", "ns", "e2etestfail", "--kubeconfig="+kubeconfigManaged).CombinedOutput()
				fmt.Println(string(out))
				return string(out)
			}, defaultTimeoutSeconds*2, 1).Should(ContainSubstring("denied by ns-must-have-gk"))
			By("Checking if status for policy template policy-gatekeeper-admission is noncompliant")
			Eventually(func() interface{} {
				plc := utils.GetWithTimeout(clientHubDynamic, gvrPolicy, userNamespace+"."+GKPolicyName, clusterNamespace, true, defaultTimeoutSeconds)
				details := plc.Object["status"].(map[string]interface{})["details"].([]interface{})
				return details[2].(map[string]interface{})["compliant"]
			}, defaultTimeoutSeconds, 1).Should(Equal("NonCompliant"))
			By("Checking if violation message for policy template policy-gatekeeper-admission is noncompliant")
			Eventually(func() interface{} {
				plc := utils.GetWithTimeout(clientHubDynamic, gvrPolicy, userNamespace+"."+GKPolicyName, clusterNamespace, true, defaultTimeoutSeconds)
				details := plc.Object["status"].(map[string]interface{})["details"].([]interface{})
				fmt.Printf("%v\n", details[2].(map[string]interface{})["history"].([]interface{})[0].(map[string]interface{})["message"])
				return details[2].(map[string]interface{})["history"].([]interface{})[0].(map[string]interface{})["message"]
			}, defaultTimeoutSeconds, 1).Should(ContainSubstring("NonCompliant; violation - events found: [e2etestfail."))
			By("Checking if status for policy template policy-gatekeeper-audit is still compliant")
			Eventually(func() interface{} {
				plc := utils.GetWithTimeout(clientHubDynamic, gvrPolicy, userNamespace+"."+GKPolicyName, clusterNamespace, true, defaultTimeoutSeconds)
				details := plc.Object["status"].(map[string]interface{})["details"].([]interface{})
				return details[1].(map[string]interface{})["compliant"]
			}, defaultTimeoutSeconds, 1).Should(Equal("Compliant"))
		})
		It("community/policy-gatekeeper-sample should be noncompliant", func() {
			By("Checking if the status of root policy is noncompliant")
			Eventually(func() interface{} {
				rootPlc := utils.GetWithTimeout(clientHubDynamic, gvrPolicy, GKPolicyName, userNamespace, true, defaultTimeoutSeconds)
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
			}, defaultTimeoutSeconds*4, 1).Should(Equal(policiesv1.NonCompliant))
		})
	})

	Describe("GRC: [P1][Sev1][policy-grc] Clean up after all", func() {
		It("Clean up community/policy-gatekeeper-sample", func() {
			utils.Kubectl("delete", "-f", GKPolicyYaml, "-n", userNamespace, "--kubeconfig="+kubeconfigHub)
			Eventually(func() interface{} {
				managedPlc := utils.GetWithTimeout(clientManagedDynamic, gvrPolicy, userNamespace+"."+GKPolicyName, clusterNamespace, false, defaultTimeoutSeconds)
				return managedPlc
			}, defaultTimeoutSeconds, 1).Should(BeNil())
			utils.Kubectl("delete", "ns", "e2etestsuccess", "--kubeconfig="+kubeconfigManaged)
			utils.Kubectl("delete", "ns", "e2etestfail", "--kubeconfig="+kubeconfigManaged)
		})
		It("Clean up community/policy-gatekeeper-operator", func() {
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
