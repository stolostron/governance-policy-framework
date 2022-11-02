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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("RHACM4K-3055", func() {
	var getComplianceState func(policyName string) func() interface{}

	BeforeEach(func() {
		if isOCP44() {
			Skip("Skipping as this is ocp 4.4")
		}

		// Assign this here to avoid using nil pointers as arguments
		getComplianceState = func(policyName string) func() interface{} {
			return common.GetComplianceState(clientHubDynamic, userNamespace, policyName, clusterNamespace)
		}
	})
	const gatekeeperPolicyURL = "https://raw.githubusercontent.com/stolostron/policy-collection/main/stable/CM-Configuration-Management/policy-gatekeeper-operator-downstream.yaml"
	const gatekeeperPolicyName = "policy-gatekeeper-operator"

	Describe("GRC: [P1][Sev1][policy-grc] Test installing gatekeeper operator", func() {
		It("stable/policy-gatekeeper-operator should be created on hub", func() {
			By("Creating policy on hub")
			utils.KubectlWithOutput("apply", "-f", gatekeeperPolicyURL, "-n", userNamespace, "--kubeconfig="+kubeconfigHub)
			By("Patching Policy Gatekeeper CR template with namespaceSelector to kubernetes.io/metadata.name=" + userNamespace)
			utils.KubectlWithOutput("patch", "-n", userNamespace, "policy", gatekeeperPolicyName,
				"--type=json", "-p=[{\"op\": \"add\", \"path\": \"/spec/policy-templates/2/objectDefinition/spec/object-templates/0/objectDefinition/spec/webhook/namespaceSelector\","+
					" \"value\":{\"matchExpressions\":[{\"key\": \"kubernetes.io/metadata.name\",\"operator\":\"In\",\"values\":["+userNamespace+", \"e2etestfail\", \"e2etestsuccess\"]}]}}]",
				"--kubeconfig="+kubeconfigHub)
			By("Patching placement rule")
			common.PatchPlacementRule(userNamespace, "placement-"+gatekeeperPolicyName, clusterNamespace, kubeconfigHub)
			By("Checking policy-gatekeeper-operator on hub cluster in ns " + userNamespace)
			rootPlc := utils.GetWithTimeout(clientHubDynamic, common.GvrPolicy, gatekeeperPolicyName, userNamespace, true, defaultTimeoutSeconds)
			Expect(rootPlc).NotTo(BeNil())
		})
		It("stable/policy-gatekeeper-operator should be created on managed cluster", func() {
			By("Checking policy-gatekeeper-operator on managed cluster in ns " + clusterNamespace)
			managedplc := utils.GetWithTimeout(clientManagedDynamic, common.GvrPolicy, userNamespace+"."+gatekeeperPolicyName, clusterNamespace, true, defaultTimeoutSeconds)
			Expect(managedplc).NotTo(BeNil())
		})
		It("stable/policy-gatekeeper-operator should be noncompliant", func() {
			By("Checking if the status of root policy is noncompliant")
			Eventually(getComplianceState(gatekeeperPolicyName), defaultTimeoutSeconds*2, 1).Should(Equal(policiesv1.NonCompliant))
		})
		It("Enforcing stable/policy-gatekeeper-operator", func() {
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
			By("Checking if pod gatekeeper-operator has been created")
			var i int = 0
			Eventually(func(g Gomega) interface{} {
				if i == 60*2 || i == 60*4 {
					fmt.Println("gatekeeper operator pod still not created, deleting subscription and let it recreate", i)
					utils.KubectlWithOutput("delete", "-n", "openshift-operators", "subscriptions.operators.coreos.com", "gatekeeper-operator-product", "--kubeconfig="+kubeconfigManaged)
				}
				i++
				podList, err := clientManaged.CoreV1().Pods("openshift-operators").List(context.TODO(), metav1.ListOptions{LabelSelector: "control-plane in (controller-manager, gatekeeper-operator-controller-manager)"})
				g.Expect(err).To(BeNil())
				return len(podList.Items)
			}, defaultTimeoutSeconds*12, 1).Should(Equal(1))
			By("Checking if pod gatekeeper-operator is running")
			Eventually(func(g Gomega) interface{} {
				podList, err := clientManaged.CoreV1().Pods("openshift-operators").List(context.TODO(), metav1.ListOptions{LabelSelector: "control-plane in (controller-manager, gatekeeper-operator-controller-manager)"})
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
		It("stable/policy-gatekeeper-operator should be compliant", func() {
			By("Checking if the status of root policy is compliant")
			Eventually(getComplianceState(gatekeeperPolicyName), defaultTimeoutSeconds*6, 1).Should(Equal(policiesv1.Compliant))
		})
		It("Informing stable/policy-gatekeeper-operator", func() {
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
		It("Clean up stable/policy-gatekeeper-operator", func() {
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
			utils.KubectlWithOutput("delete", "-n", "openshift-operators", "subscriptions.operators.coreos.com", "gatekeeper-operator-product", "--kubeconfig="+kubeconfigManaged)
			csvName, _ := utils.KubectlWithOutput("get", "-n", "openshift-operators", "csv", "-o", "jsonpath=\"{.items[?(@.spec.displayName==\"Gatekeeper Operator\")].metadata.name}\"", "--kubeconfig="+kubeconfigManaged)
			csvName = strings.Trim(csvName, "\"")
			utils.KubectlWithOutput("delete", "-n", "openshift-operators", "csv", csvName, "--kubeconfig="+kubeconfigManaged)
			utils.KubectlWithOutput("delete", "crd", "gatekeepers.operator.gatekeeper.sh", "--kubeconfig="+kubeconfigManaged)
			out, _ := utils.KubectlWithOutput("delete", "ns", "openshift-gatekeeper-system", "--kubeconfig="+kubeconfigManaged)
			Expect(out).To(Or(
				ContainSubstring("namespace \"openshift-gatekeeper-system\" deleted"),
				ContainSubstring("namespaces \"openshift-gatekeeper-system\" not found")))
			utils.KubectlWithOutput("delete", "events", "-n", clusterNamespace, "--field-selector=involvedObject.name="+userNamespace+".policy-gatekeeper-operator", "--kubeconfig="+kubeconfigManaged)
		})
	})
})
