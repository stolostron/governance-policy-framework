// Copyright (c) 2020 Red Hat, Inc.
// Copyright Contributors to the Open Cluster Management project

package integration

import (
	"context"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog"
	policiesv1 "open-cluster-management.io/governance-policy-propagator/api/v1"
	"open-cluster-management.io/governance-policy-propagator/test/utils"

	"github.com/stolostron/governance-policy-framework/test/common"
)

func isOCP44() bool {
	clusterVersion, err := clientManagedDynamic.Resource(common.GvrClusterVersion).Get(
		context.TODO(),
		"version",
		metav1.GetOptions{},
	)
	if err != nil && k8serrors.IsNotFound(err) {
		// no version CR, not ocp
		klog.V(5).Infof("This is not an OCP cluster")

		return false
	}

	desired := clusterVersion.Object["status"].(map[string]interface{})["desired"]
	version, _ := desired.(map[string]interface{})["version"].(string)

	klog.V(5).Infof("OCP Version %s\n", version)

	return strings.HasPrefix(version, "4.4")
}

var _ = Describe("", Ordered, Label("policy-collection", "community"), func() {
	var getComplianceState func(policyName string) func(Gomega) interface{}
	BeforeAll(func() {
		if isOCP44() {
			Skip("Skipping as this is ocp 4.4")
		}
		if !canCreateOpenshiftNamespaces() {
			Skip("Skipping as upstream gatekeeper operator requires the ability to " +
				"create the openshift-gatekeeper-system namespace")
		}

		// Assign this here to avoid using nil pointers as arguments
		getComplianceState = func(policyName string) func(Gomega) interface{} {
			return common.GetComplianceState(clientHubDynamic, userNamespace, policyName, clusterNamespace)
		}
	})
	const gatekeeperPolicyURL = policyCollectCommunityURL +
		"CM-Configuration-Management/policy-gatekeeper-operator.yaml"
	const gatekeeperPolicyName = "policy-gatekeeper-operator"

	Describe("RHACM4K-1692 GRC: [P1][Sev1][policy-grc] Test installing gatekeeper operator", func() {
		It("community/policy-gatekeeper-operator should be created on hub", func() {
			By("Creating policy on hub")
			_, err := utils.KubectlWithOutput(
				"apply", "-f",
				gatekeeperPolicyURL,
				"-n", userNamespace,
				"--kubeconfig="+kubeconfigHub,
			)
			Expect(err).To(BeNil())

			By("Patching Policy Gatekeeper CR template with namespaceSelector " +
				"to kubernetes.io/metadata.name=" + userNamespace)
			_, err = utils.KubectlWithOutput(
				"patch", "-n", userNamespace,
				"policies.policy.open-cluster-management.io", gatekeeperPolicyName,
				"--type=json", "-p=[{\"op\": \"add\", \"path\": \"/spec/policy-templates/4/objectDefinition"+
					"/spec/object-templates/0/objectDefinition/spec/webhook/namespaceSelector\","+
					" \"value\":{\"matchExpressions\":[{\"key\": \"grc\","+
					"\"operator\":\"In\",\"values\":[\"true\"]}]}}]",
				"--kubeconfig="+kubeconfigHub)
			Expect(err).To(BeNil())
			By("Patching placement rule")
			err = common.PatchPlacementRule(
				userNamespace,
				"placement-"+gatekeeperPolicyName,
				clusterNamespace,
				kubeconfigHub,
			)
			Expect(err).To(BeNil())

			By("Checking policy-gatekeeper-operator on hub cluster in ns " + userNamespace)
			rootPlc := utils.GetWithTimeout(
				clientHubDynamic,
				common.GvrPolicy,
				gatekeeperPolicyName,
				userNamespace,
				true,
				defaultTimeoutSeconds,
			)
			Expect(rootPlc).NotTo(BeNil())
		})
		It("community/policy-gatekeeper-operator should be created on managed cluster", func() {
			By("Checking policy-gatekeeper-operator on managed cluster in ns " + clusterNamespace)
			managedplc := utils.GetWithTimeout(
				clientManagedDynamic,
				common.GvrPolicy,
				userNamespace+"."+gatekeeperPolicyName,
				clusterNamespace,
				true,
				defaultTimeoutSeconds,
			)
			Expect(managedplc).NotTo(BeNil())
		})
		It("community/policy-gatekeeper-operator should be noncompliant", func() {
			By("Checking if the status of root policy is noncompliant")
			Eventually(
				getComplianceState(gatekeeperPolicyName),
				defaultTimeoutSeconds*2,
				1,
			).Should(Equal(policiesv1.NonCompliant))
		})
		It("Enforcing community/policy-gatekeeper-operator", func() {
			Eventually(func() interface{} {
				By("Patching remediationAction = enforce on root policy")
				rootPlc := utils.GetWithTimeout(
					clientHubDynamic,
					common.GvrPolicy,
					gatekeeperPolicyName,
					userNamespace,
					true,
					defaultTimeoutSeconds,
				)
				rootPlc.Object["spec"].(map[string]interface{})["remediationAction"] = "enforce"
				_, err := clientHubDynamic.Resource(common.GvrPolicy).Namespace(userNamespace).Update(
					context.TODO(),
					rootPlc,
					metav1.UpdateOptions{},
				)
				Expect(err).To(BeNil())
				By("Checking if remediationAction is enforce for root policy")
				rootPlc = utils.GetWithTimeout(
					clientHubDynamic,
					common.GvrPolicy,
					gatekeeperPolicyName,
					userNamespace,
					true,
					defaultTimeoutSeconds,
				)

				return rootPlc.Object["spec"].(map[string]interface{})["remediationAction"]
			}, defaultTimeoutSeconds, 1).Should(Equal("enforce"))
			By("Checking if remediationAction is enforce for replicated policy")
			Eventually(func() interface{} {
				managedPlc := utils.GetWithTimeout(
					clientManagedDynamic,
					common.GvrPolicy,
					userNamespace+"."+gatekeeperPolicyName,
					clusterNamespace,
					true,
					defaultTimeoutSeconds,
				)

				return managedPlc.Object["spec"].(map[string]interface{})["remediationAction"]
			}, defaultTimeoutSeconds, 1).Should(Equal("enforce"))
		})
		It("Gatekeeper operator pod should be running", func() {
			By("Checking if pod gatekeeper-operator-controller-manager has been created")
			Eventually(func(g Gomega) interface{} {
				podList, err := clientManaged.CoreV1().Pods("openshift-gatekeeper-operator").List(
					context.TODO(),
					metav1.ListOptions{
						LabelSelector: "control-plane in (controller-manager, gatekeeper-operator-controller-manager)",
					},
				)
				g.Expect(err).To(BeNil())

				return len(podList.Items)
			}, defaultTimeoutSeconds*8, 1).ShouldNot(Equal(0))
			By("Checking if pod gatekeeper-operator-controller-manager is running")
			Eventually(func(g Gomega) interface{} {
				podList, err := clientManaged.CoreV1().Pods("openshift-gatekeeper-operator").List(
					context.TODO(),
					metav1.ListOptions{
						LabelSelector: "control-plane in (controller-manager, gatekeeper-operator-controller-manager)",
					},
				)
				g.Expect(err).To(BeNil())
				for _, item := range podList.Items {
					if strings.HasPrefix(item.ObjectMeta.Name, "gatekeeper-operator-controller") {
						// Log the pod status message if there may be a problem starting the pod
						if len(item.Status.Conditions) > 0 && item.Status.Conditions[0].Status == "False" {
							GinkgoWriter.Println("Pod status error message: " + item.Status.Conditions[0].Message)
						}

						return string(item.Status.Phase)
					}
				}

				return "nil"
			}, defaultTimeoutSeconds*4, 1).Should(Equal("Running"))
		})
		It("Checking if validating webhook gatekeeper-validating-webhook-configuration "+
			"is scoped to grc test namespaces", func() {
			By("Checking if validating webhook gatekeeper-validating-webhook-configuration exists")
			Eventually(func() interface{} {
				out, _ := utils.KubectlWithOutput(
					"get",
					"validatingwebhookconfigurations.admissionregistration.k8s.io",
					"gatekeeper-validating-webhook-configuration",
					"--kubeconfig="+kubeconfigManaged,
				)

				return out
			}, defaultTimeoutSeconds*2, 1).Should(ContainSubstring("AGE\ngatekeeper-validating-webhook-configuration"))
			By("Checking if validating webhook gatekeeper-validating-webhook-configuration contains MatchExpressions")
			Eventually(func() interface{} {
				webhook, _ := clientManaged.AdmissionregistrationV1().ValidatingWebhookConfigurations().Get(
					context.TODO(),
					"gatekeeper-validating-webhook-configuration",
					metav1.GetOptions{},
				)

				return len(webhook.Webhooks)
			}, defaultTimeoutSeconds, 1).Should(Equal(2))
			webhook, err := clientManaged.AdmissionregistrationV1().ValidatingWebhookConfigurations().Get(
				context.TODO(),
				"gatekeeper-validating-webhook-configuration",
				metav1.GetOptions{},
			)
			Expect(err).To(BeNil())
			Expect(len(webhook.Webhooks[0].NamespaceSelector.MatchExpressions)).To(Equal(1))
			Expect(len(webhook.Webhooks[1].NamespaceSelector.MatchExpressions)).To(Equal(1))
			Expect(webhook.Webhooks[0].NamespaceSelector.MatchExpressions[0]).NotTo(BeNil())
			Expect(webhook.Webhooks[1].NamespaceSelector.MatchExpressions[0]).NotTo(BeNil())
		})
		It("Gatekeeper audit pod should be running", func() {
			By("Checking if pod gatekeeper-audit has been created")
			Eventually(func(g Gomega) interface{} {
				podList, err := clientManaged.CoreV1().Pods("openshift-gatekeeper-system").List(
					context.TODO(),
					metav1.ListOptions{
						LabelSelector: "control-plane=audit-controller",
					},
				)
				g.Expect(err).To(BeNil())

				return len(podList.Items)
			}, defaultTimeoutSeconds*2, 1).Should(Equal(1))
			By("Checking if pod gatekeeper-audit is running")
			Eventually(func(g Gomega) interface{} {
				podList, err := clientManaged.CoreV1().Pods("openshift-gatekeeper-system").List(
					context.TODO(),
					metav1.ListOptions{
						LabelSelector: "control-plane=audit-controller",
					},
				)
				g.Expect(err).To(BeNil())

				return string(podList.Items[0].Status.Phase)
			}, defaultTimeoutSeconds*4, 1).Should(Equal("Running"))
		})

		It("Gatekeeper controller manager pods should be running", func() {
			By("Checking if pod gatekeeper-controller-manager has been created")
			Eventually(func(g Gomega) interface{} {
				podList, err := clientManaged.CoreV1().Pods("openshift-gatekeeper-system").List(
					context.TODO(),
					metav1.ListOptions{
						LabelSelector: "control-plane=controller-manager",
					},
				)
				g.Expect(err).To(BeNil())

				return len(podList.Items)
			}, defaultTimeoutSeconds*2, 1).Should(Equal(2))
			By("Checking if pod gatekeeper-controller-manager is running")
			Eventually(func(g Gomega) interface{} {
				podList, err := clientManaged.CoreV1().Pods("openshift-gatekeeper-system").List(
					context.TODO(),
					metav1.ListOptions{
						LabelSelector: "control-plane=controller-manager",
					},
				)
				g.Expect(err).To(BeNil())

				return string(podList.Items[0].Status.Phase) + "/" + string(podList.Items[1].Status.Phase)
			}, defaultTimeoutSeconds*4, 1).Should(Equal("Running/Running"))
		})
		It("community/policy-gatekeeper-operator should be compliant", func() {
			By("Checking if the status of root policy is compliant")
			Eventually(
				getComplianceState(gatekeeperPolicyName),
				defaultTimeoutSeconds*6,
				1,
			).Should(Equal(policiesv1.Compliant))
		})
		It("Informing community/policy-gatekeeper-operator", func() {
			Eventually(func() interface{} {
				By("Patching remediationAction = inform on root policy")
				rootPlc := utils.GetWithTimeout(
					clientHubDynamic,
					common.GvrPolicy,
					gatekeeperPolicyName,
					userNamespace,
					true,
					defaultTimeoutSeconds,
				)
				rootPlc.Object["spec"].(map[string]interface{})["remediationAction"] = "inform"
				_, err := clientHubDynamic.Resource(common.GvrPolicy).Namespace(userNamespace).Update(
					context.TODO(),
					rootPlc,
					metav1.UpdateOptions{},
				)
				Expect(err).To(BeNil())
				By("Checking if remediationAction is inform for root policy")
				rootPlc = utils.GetWithTimeout(
					clientHubDynamic,
					common.GvrPolicy,
					gatekeeperPolicyName,
					userNamespace,
					true,
					defaultTimeoutSeconds,
				)

				return rootPlc.Object["spec"].(map[string]interface{})["remediationAction"]
			}, defaultTimeoutSeconds, 1).Should(Equal("inform"))
			By("Checking if remediationAction is inform for replicated policy")
			Eventually(func() interface{} {
				managedPlc := utils.GetWithTimeout(
					clientManagedDynamic,
					common.GvrPolicy,
					userNamespace+"."+gatekeeperPolicyName,
					clusterNamespace,
					true,
					defaultTimeoutSeconds,
				)

				return managedPlc.Object["spec"].(map[string]interface{})["remediationAction"]
			}, defaultTimeoutSeconds, 1).Should(Equal("inform"))
		})
	})

	AfterAll(func() {
		if CurrentSpecReport().Failed() {
			common.OutputDebugInfo(
				"Gatekeeper policies",
				kubeconfigHub,
				"constrainttemplates.templates.gatekeeper.sh",
			)
		}

		// Clean up community/policy-gatekeeper-operator
		_, err := utils.KubectlWithOutput(
			"delete",
			"-f",
			gatekeeperPolicyURL,
			"-n",
			userNamespace,
			"--kubeconfig="+kubeconfigHub,
			"--ignore-not-found",
		)
		Expect(err).To(BeNil())

		Eventually(func() interface{} {
			managedPlc := utils.GetWithTimeout(
				clientManagedDynamic,
				common.GvrPolicy,
				userNamespace+"."+gatekeeperPolicyName,
				clusterNamespace,
				false,
				defaultTimeoutSeconds,
			)

			return managedPlc
		}, defaultTimeoutSeconds, 1).Should(BeNil())
		utils.Pause(20)
		_, err = utils.KubectlWithOutput(
			"delete",
			"Gatekeeper",
			"gatekeeper",
			"--kubeconfig="+kubeconfigManaged,
			"--ignore-not-found",
		)
		Expect(err).To(BeNil())

		Eventually(func() interface{} {
			out, _ := utils.KubectlWithOutput(
				"get",
				"pods",
				"-n",
				"openshift-gatekeeper-system",
				"--kubeconfig="+kubeconfigManaged,
			)

			return out
			// k8s will respond with this even if the ns was deleted.
		}, defaultTimeoutSeconds*4, 1).Should(ContainSubstring("No resources found"))
		_, err = utils.KubectlWithOutput(
			"delete",
			"-n",
			"openshift-gatekeeper-operator",
			"subscriptions.operators.coreos.com",
			"gatekeeper-operator-sub",
			"--kubeconfig="+kubeconfigManaged,
			"--ignore-not-found",
		)
		Expect(err).To(BeNil())

		_, err = utils.KubectlWithOutput(
			"delete",
			"-n",
			"openshift-gatekeeper-operator",
			"OperatorGroup",
			"gatekeeper-operator",
			"--kubeconfig="+kubeconfigManaged,
			"--ignore-not-found",
		)
		Expect(err).To(BeNil())

		_, err = utils.KubectlWithOutput(
			"delete",
			"crd",
			"gatekeepers.operator.gatekeeper.sh",
			"--kubeconfig="+kubeconfigManaged,
			"--ignore-not-found",
		)
		Expect(err).To(BeNil())

		out, _ := utils.KubectlWithOutput(
			"delete",
			"ns",
			"openshift-gatekeeper-operator",
			"--kubeconfig="+kubeconfigManaged,
		)
		Expect(out).To(ContainSubstring("namespace \"openshift-gatekeeper-operator\" deleted"))

		out, _ = utils.KubectlWithOutput(
			"delete",
			"ns",
			"openshift-gatekeeper-system",
			"--kubeconfig="+kubeconfigManaged,
		)
		Expect(out).To(Or(
			ContainSubstring("namespace \"openshift-gatekeeper-system\" deleted"),
			ContainSubstring("namespaces \"openshift-gatekeeper-system\" not found")))

		_, err = utils.KubectlWithOutput(
			"delete",
			"events",
			"-n",
			clusterNamespace,
			"--field-selector=involvedObject.name="+userNamespace+".policy-gatekeeper-operator",
			"--kubeconfig="+kubeconfigManaged,
			"--ignore-not-found",
		)
		Expect(err).To(BeNil())
	})
})
