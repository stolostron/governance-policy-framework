// Copyright (c) 2020 Red Hat, Inc.
// Copyright Contributors to the Open Cluster Management project

package e2e

import (
	"context"
	"fmt"
	"strings"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/stolostron/governance-policy-framework/test/common"
	policiesv1 "github.com/stolostron/governance-policy-propagator/api/v1"
	"github.com/stolostron/governance-policy-propagator/test/utils"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func isOCP44() bool {
	clusterVersion, err := clientManagedDynamic.Resource(common.GvrClusterVersion).Get(context.TODO(), "version", metav1.GetOptions{})
	if err != nil && errors.IsNotFound(err) {
		// no version CR, not ocp
		fmt.Println("This is not an OCP cluster")
		return false
	}
	version := clusterVersion.Object["status"].(map[string]interface{})["desired"].(map[string]interface{})["version"].(string)
	fmt.Println("OCP Version " + version)
	return strings.HasPrefix(version, "4.4")
}

var _ = Describe("", func() {
	var getComplianceState func(policyName string) func() interface{}
	BeforeEach(func() {
		if isOCP44() {
			Skip("Skipping as this is ocp 4.4")
		}
		if !canCreateOpenshiftNamespaces() {
			Skip("Skipping as upstream gatekeeper operator requires the ability to create the openshift-gatekeeper-system namespace")
		}

		// Assign this here to avoid using nil pointers as arguments
		getComplianceState = func(policyName string) func() interface{} {
			return common.GetComplianceState(clientHubDynamic, userNamespace, policyName, clusterNamespace)
		}
	})
	const gatekeeperPolicyURL = "https://raw.githubusercontent.com/stolostron/policy-collection/main/community/CM-Configuration-Management/policy-gatekeeper-operator.yaml"
	const gatekeeperPolicyName = "policy-gatekeeper-operator"

	Describe("RHACM4K-1692 GRC: [P1][Sev1][policy-grc] Test installing gatekeeper operator", func() {
		It("Clean up before all", func() {
			By("checking if openshift-gatekeeper-operator ns exists")
			_, err := clientManaged.CoreV1().Namespaces().Get(context.TODO(), "openshift-gatekeeper-operator", metav1.GetOptions{})
			if err == nil || !errors.IsNotFound(err) {
				utils.KubectlWithOutput("delete", "ns", "e2etestsuccess", "--kubeconfig="+kubeconfigManaged)
				utils.KubectlWithOutput("delete", "ns", "e2etestfail", "--kubeconfig="+kubeconfigManaged)
				By("namespace openshift-gatekeeper-operator exists, cleaning up...")
				utils.KubectlWithOutput("delete", "-f", gatekeeperPolicyURL, "-n", userNamespace, "--kubeconfig="+kubeconfigHub)
				Eventually(func() interface{} {
					managedPlc := utils.GetWithTimeout(clientManagedDynamic, common.GvrPolicy, userNamespace+"."+gatekeeperPolicyName, clusterNamespace, false, defaultTimeoutSeconds)
					return managedPlc
				}, defaultTimeoutSeconds, 1).Should(BeNil())
				utils.KubectlWithOutput("delete", "-n", "openshift-gatekeeper-operator", "Gatekeeper", "gatekeeper", "--kubeconfig="+kubeconfigManaged)
				utils.KubectlWithOutput("delete", "-n", "openshift-gatekeeper-operator", "subscriptions.operators.coreos.com", "gatekeeper-operator-sub", "--kubeconfig="+kubeconfigManaged)
				utils.KubectlWithOutput("delete", "-n", "openshift-gatekeeper-operator", "OperatorGroup", "gatekeeper-operator", "--kubeconfig="+kubeconfigManaged)
				utils.KubectlWithOutput("delete", "crd", "gatekeepers.operator.gatekeeper.sh", "--kubeconfig="+kubeconfigManaged)
				out, _ := utils.KubectlWithOutput("delete", "ns", "openshift-gatekeeper-operator", "--kubeconfig="+kubeconfigManaged)
				Expect(out).To(ContainSubstring("namespace \"openshift-gatekeeper-operator\" deleted"))
			}
		})
		It("community/policy-gatekeeper-operator should be created on hub", func() {
			By("Creating policy on hub")
			utils.KubectlWithOutput("apply", "-f", gatekeeperPolicyURL, "-n", userNamespace, "--kubeconfig="+kubeconfigHub)

			By("Patching Policy Gatekeeper CR template with namespaceSelector to kubernetes.io/metadata.name=" + userNamespace)
			utils.KubectlWithOutput("patch", "-n", userNamespace, "policy", gatekeeperPolicyName,
				"--type=json", "-p=[{\"op\": \"add\", \"path\": \"/spec/policy-templates/4/objectDefinition/spec/object-templates/0/objectDefinition/spec/webhook/namespaceSelector\","+
					" \"value\":{\"matchExpressions\":[{\"key\": \"kubernetes.io/metadata.name\",\"operator\":\"In\",\"values\":["+userNamespace+", \"e2etestfail\", \"e2etestsuccess\"]}]}}]",
				"--kubeconfig="+kubeconfigHub)

			By("Patching placement rule")
			common.PatchPlacementRule(userNamespace, "placement-"+gatekeeperPolicyName, clusterNamespace, kubeconfigHub)

			By("Checking policy-gatekeeper-operator on hub cluster in ns " + userNamespace)
			rootPlc := utils.GetWithTimeout(clientHubDynamic, common.GvrPolicy, gatekeeperPolicyName, userNamespace, true, defaultTimeoutSeconds)
			Expect(rootPlc).NotTo(BeNil())
		})
		It("community/policy-gatekeeper-operator should be created on managed cluster", func() {
			By("Checking policy-gatekeeper-operator on managed cluster in ns " + clusterNamespace)
			managedplc := utils.GetWithTimeout(clientManagedDynamic, common.GvrPolicy, userNamespace+"."+gatekeeperPolicyName, clusterNamespace, true, defaultTimeoutSeconds)
			Expect(managedplc).NotTo(BeNil())
		})
		It("community/policy-gatekeeper-operator should be noncompliant", func() {
			By("Checking if the status of root policy is noncompliant")
			Eventually(getComplianceState(gatekeeperPolicyName), defaultTimeoutSeconds*2, 1).Should(Equal(policiesv1.NonCompliant))
		})
		It("Enforcing community/policy-gatekeeper-operator", func() {
			Eventually(func() interface{} {
				By("Patching remediationAction = enforce on root policy")
				rootPlc := utils.GetWithTimeout(clientHubDynamic, common.GvrPolicy, gatekeeperPolicyName, userNamespace, true, defaultTimeoutSeconds)
				rootPlc.Object["spec"].(map[string]interface{})["remediationAction"] = "enforce"
				clientHubDynamic.Resource(common.GvrPolicy).Namespace(userNamespace).Update(context.TODO(), rootPlc, metav1.UpdateOptions{})
				By("Checking if remediationAction is enforce for root policy")
				rootPlc = utils.GetWithTimeout(clientHubDynamic, common.GvrPolicy, gatekeeperPolicyName, userNamespace, true, defaultTimeoutSeconds)
				return rootPlc.Object["spec"].(map[string]interface{})["remediationAction"]
			}, defaultTimeoutSeconds, 1).Should(Equal("enforce"))
			By("Checking if remediationAction is enforce for replicated policy")
			Eventually(func() interface{} {
				managedPlc := utils.GetWithTimeout(clientManagedDynamic, common.GvrPolicy, userNamespace+"."+gatekeeperPolicyName, clusterNamespace, true, defaultTimeoutSeconds)
				return managedPlc.Object["spec"].(map[string]interface{})["remediationAction"]
			}, defaultTimeoutSeconds, 1).Should(Equal("enforce"))
		})
		It("Gatekeeper operator pod should be running", func() {
			By("Checking if pod gatekeeper-operator-controller-manager has been created")
			Eventually(func(g Gomega) interface{} {
				podList, err := clientManaged.CoreV1().Pods("openshift-gatekeeper-operator").List(context.TODO(), metav1.ListOptions{LabelSelector: "control-plane in (controller-manager, gatekeeper-operator-controller-manager)"})
				g.Expect(err).To(BeNil())
				return len(podList.Items)
			}, defaultTimeoutSeconds*8, 1).ShouldNot(Equal(0))
			By("Checking if pod gatekeeper-operator-controller-manager is running")
			Eventually(func(g Gomega) interface{} {
				podList, err := clientManaged.CoreV1().Pods("openshift-gatekeeper-operator").List(context.TODO(), metav1.ListOptions{LabelSelector: "control-plane in (controller-manager, gatekeeper-operator-controller-manager)"})
				g.Expect(err).To(BeNil())
				for _, item := range podList.Items {
					if strings.HasPrefix(item.ObjectMeta.Name, "gatekeeper-operator-controller") {
						return string(item.Status.Phase)
					}
				}
				return "nil"
			}, defaultTimeoutSeconds*4, 1).Should(Equal("Running"))
		})
		// set to ignore to ensure it won't fail other tests running in parallel
		It("Patching webhook check-ignore-label.gatekeeper.sh failurePolicy to ignore", func() {
			By("Checking if validating webhook gatekeeper-validating-webhook-configuration exists")
			Eventually(func() interface{} {
				out, _ := utils.KubectlWithOutput("get", "validatingwebhookconfigurations.admissionregistration.k8s.io", "gatekeeper-validating-webhook-configuration", "--kubeconfig="+kubeconfigManaged)
				return out
			}, defaultTimeoutSeconds*2, 1).Should(ContainSubstring("AGE\ngatekeeper-validating-webhook-configuration"))
			By("Patching if validating webhook gatekeeper-validating-webhook-configuration exists")
			out, _ := utils.KubectlWithOutput("patch", "validatingwebhookconfigurations.admissionregistration.k8s.io", "gatekeeper-validating-webhook-configuration",
				"--type=json", "-p=[{\"op\": \"replace\", \"path\": \"/webhooks/1/failurePolicy\", \"value\": \"Ignore\"}]", "--kubeconfig="+kubeconfigManaged)
			Expect(out).To(ContainSubstring("validatingwebhookconfiguration.admissionregistration.k8s.io/gatekeeper-validating-webhook-configuration patched"))
		})
		It("Gatekeeper audit pod should be running", func() {
			By("Checking if pod gatekeeper-audit has been created")
			Eventually(func(g Gomega) interface{} {
				podList, err := clientManaged.CoreV1().Pods("openshift-gatekeeper-system").List(context.TODO(), metav1.ListOptions{LabelSelector: "control-plane=audit-controller"})
				g.Expect(err).To(BeNil())
				return len(podList.Items)
			}, defaultTimeoutSeconds*2, 1).Should(Equal(1))
			By("Checking if pod gatekeeper-audit is running")
			Eventually(func(g Gomega) interface{} {
				podList, err := clientManaged.CoreV1().Pods("openshift-gatekeeper-system").List(context.TODO(), metav1.ListOptions{LabelSelector: "control-plane=audit-controller"})
				g.Expect(err).To(BeNil())
				return string(podList.Items[0].Status.Phase)
			}, defaultTimeoutSeconds*4, 1).Should(Equal("Running"))
		})

		It("Gatekeeper controller manager pods should be running", func() {
			By("Checking if pod gatekeeper-controller-manager has been created")
			Eventually(func(g Gomega) interface{} {
				podList, err := clientManaged.CoreV1().Pods("openshift-gatekeeper-system").List(context.TODO(), metav1.ListOptions{LabelSelector: "control-plane=controller-manager"})
				g.Expect(err).To(BeNil())
				return len(podList.Items)
			}, defaultTimeoutSeconds*2, 1).Should(Equal(2))
			By("Checking if pod gatekeeper-controller-manager is running")
			Eventually(func(g Gomega) interface{} {
				podList, err := clientManaged.CoreV1().Pods("openshift-gatekeeper-system").List(context.TODO(), metav1.ListOptions{LabelSelector: "control-plane=controller-manager"})
				g.Expect(err).To(BeNil())
				return string(podList.Items[0].Status.Phase) + "/" + string(podList.Items[1].Status.Phase)
			}, defaultTimeoutSeconds*4, 1).Should(Equal("Running/Running"))
		})
		It("community/policy-gatekeeper-operator should be compliant", func() {
			By("Checking if the status of root policy is compliant")
			Eventually(getComplianceState(gatekeeperPolicyName), defaultTimeoutSeconds*6, 1).Should(Equal(policiesv1.Compliant))
		})
		It("Informing community/policy-gatekeeper-operator", func() {
			Eventually(func() interface{} {
				By("Patching remediationAction = inform on root policy")
				rootPlc := utils.GetWithTimeout(clientHubDynamic, common.GvrPolicy, gatekeeperPolicyName, userNamespace, true, defaultTimeoutSeconds)
				rootPlc.Object["spec"].(map[string]interface{})["remediationAction"] = "inform"
				clientHubDynamic.Resource(common.GvrPolicy).Namespace(userNamespace).Update(context.TODO(), rootPlc, metav1.UpdateOptions{})
				By("Checking if remediationAction is inform for root policy")
				rootPlc = utils.GetWithTimeout(clientHubDynamic, common.GvrPolicy, gatekeeperPolicyName, userNamespace, true, defaultTimeoutSeconds)
				return rootPlc.Object["spec"].(map[string]interface{})["remediationAction"]
			}, defaultTimeoutSeconds, 1).Should(Equal("inform"))
			By("Checking if remediationAction is inform for replicated policy")
			Eventually(func() interface{} {
				managedPlc := utils.GetWithTimeout(clientManagedDynamic, common.GvrPolicy, userNamespace+"."+gatekeeperPolicyName, clusterNamespace, true, defaultTimeoutSeconds)
				return managedPlc.Object["spec"].(map[string]interface{})["remediationAction"]
			}, defaultTimeoutSeconds, 1).Should(Equal("inform"))
		})
	})

	Describe("GRC: [P1][Sev1][policy-grc] Clean up after all", func() {
		It("Clean up community/policy-gatekeeper-operator", func() {
			utils.KubectlWithOutput("delete", "-f", gatekeeperPolicyURL, "-n", userNamespace, "--kubeconfig="+kubeconfigHub)
			Eventually(func() interface{} {
				managedPlc := utils.GetWithTimeout(clientManagedDynamic, common.GvrPolicy, userNamespace+"."+gatekeeperPolicyName, clusterNamespace, false, defaultTimeoutSeconds)
				return managedPlc
			}, defaultTimeoutSeconds, 1).Should(BeNil())
			utils.Pause(20)
			utils.KubectlWithOutput("delete", "Gatekeeper", "gatekeeper", "--kubeconfig="+kubeconfigManaged)
			Eventually(func() interface{} {
				out, _ := utils.KubectlWithOutput("get", "pods", "-n", "openshift-gatekeeper-system", "--kubeconfig="+kubeconfigManaged)
				return out
			}, defaultTimeoutSeconds*4, 1).Should(ContainSubstring("No resources found")) // k8s will respond with this even if the ns was deleted.
			utils.KubectlWithOutput("delete", "-n", "openshift-gatekeeper-operator", "subscriptions.operators.coreos.com", "gatekeeper-operator-sub", "--kubeconfig="+kubeconfigManaged)
			utils.KubectlWithOutput("delete", "-n", "openshift-gatekeeper-operator", "OperatorGroup", "gatekeeper-operator", "--kubeconfig="+kubeconfigManaged)
			utils.KubectlWithOutput("delete", "crd", "gatekeepers.operator.gatekeeper.sh", "--kubeconfig="+kubeconfigManaged)
			out, _ := utils.KubectlWithOutput("delete", "ns", "openshift-gatekeeper-operator", "--kubeconfig="+kubeconfigManaged)
			Expect(out).To(ContainSubstring("namespace \"openshift-gatekeeper-operator\" deleted"))
			out, _ = utils.KubectlWithOutput("delete", "ns", "openshift-gatekeeper-system", "--kubeconfig="+kubeconfigManaged)
			Expect(out).To(Or(
				ContainSubstring("namespace \"openshift-gatekeeper-system\" deleted"),
				ContainSubstring("namespaces \"openshift-gatekeeper-system\" not found")))
			utils.KubectlWithOutput("delete", "events", "-n", clusterNamespace, "--field-selector=involvedObject.name="+userNamespace+".policy-gatekeeper-operator", "--kubeconfig="+kubeconfigManaged)
		})
	})
})
