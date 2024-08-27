// Copyright (c) 2020 Red Hat, Inc.
// Copyright Contributors to the Open Cluster Management project

package integration

import (
	"context"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	admissionv1 "k8s.io/api/admissionregistration/v1"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	policiesv1 "open-cluster-management.io/governance-policy-propagator/api/v1"
	"open-cluster-management.io/governance-policy-propagator/test/utils"

	"github.com/stolostron/governance-policy-framework/test/common"
)

var _ = Describe("RHACM4K-3055", Ordered, Label("policy-collection", "stable", "BVT"), func() {
	gatekeeperPolicyURL := policyCollectStableURL +
		"CM-Configuration-Management/policy-gatekeeper-operator-downstream.yaml"

	const (
		gatekeeperNamespace  = "openshift-gatekeeper-system"
		gatekeeperPolicyName = "policy-gatekeeper-operator"
	)

	Describe("GRC: [P1][Sev1][policy-grc] Test installing gatekeeper operator", func() {
		It("stable/policy-gatekeeper-operator should be created on hub", func(ctx SpecContext) {
			By("Creating policy on hub")
			_, err := utils.KubectlWithOutput(
				"apply", "-f",
				gatekeeperPolicyURL,
				"-n", userNamespace,
				"--kubeconfig="+kubeconfigHub,
			)
			Expect(err).ToNot(HaveOccurred())
			By("Patching Policy Gatekeeper CR template with namespaceSelector " +
				"to kubernetes.io/metadata.name=" + userNamespace)
			_, err = utils.KubectlWithOutput(
				"patch", "-n", userNamespace,
				"policies.policy.open-cluster-management.io",
				gatekeeperPolicyName,
				"--type=json", "-p=[{\"op\": \"add\", \"path\": \"/spec/policy-templates/1/objectDefinition/"+
					"spec/object-templates/0/objectDefinition/spec/webhook/namespaceSelector\","+
					" \"value\":{\"matchExpressions\":[{\"key\": \"grc\",\"operator\":\"In\","+
					"\"values\":[\"true\"]}]}}]",
				"--kubeconfig="+kubeconfigHub,
			)
			Expect(err).ToNot(HaveOccurred())

			err = common.ApplyPlacement(ctx, userNamespace, gatekeeperPolicyName)
			Expect(err).ToNot(HaveOccurred())

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
		It("stable/policy-gatekeeper-operator should be created on managed cluster", func() {
			By("Checking policy-gatekeeper-operator on managed cluster in ns " + clusterNamespace)
			managedplc := utils.GetWithTimeout(
				clientManagedDynamic,
				common.GvrPolicy,
				userNamespace+"."+gatekeeperPolicyName,
				clusterNamespace,
				true,
				defaultTimeoutSeconds*2,
			)
			Expect(managedplc).NotTo(BeNil())
		})
		It("stable/policy-gatekeeper-operator should be noncompliant", func() {
			By("Checking if the status of root policy is noncompliant")
			Eventually(
				common.GetComplianceState(gatekeeperPolicyName),
				defaultTimeoutSeconds*2,
				1,
			).Should(Equal(policiesv1.NonCompliant))
		})
		It("Enforcing stable/policy-gatekeeper-operator", func() {
			common.EnforcePolicy(gatekeeperPolicyName)
		})
		It("Gatekeeper operator pod should be running", func(ctx SpecContext) {
			By("Checking if pod gatekeeper-operator has been created")
			Eventually(func() ([]corev1.Pod, error) {
				podList, err := clientManaged.CoreV1().Pods("openshift-operators").List(ctx,
					metav1.ListOptions{
						LabelSelector: "control-plane in (controller-manager, gatekeeper-operator-controller-manager)",
					},
				)

				return podList.Items, err
			}, defaultTimeoutSeconds*12, 1,
			).Should(HaveLen(1))
			By("Checking if pod gatekeeper-operator is running")
			Eventually(func(g Gomega) interface{} {
				podList, err := clientManaged.CoreV1().Pods("openshift-operators").List(
					ctx,
					metav1.ListOptions{
						LabelSelector: "control-plane in " +
							"(controller-manager, gatekeeper-operator-controller-manager)",
					},
				)
				g.Expect(err).ToNot(HaveOccurred())
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
			Eventually(func() []admissionv1.ValidatingWebhook {
				webhook, _ := clientManaged.AdmissionregistrationV1().ValidatingWebhookConfigurations().Get(
					context.TODO(),
					"gatekeeper-validating-webhook-configuration",
					metav1.GetOptions{},
				)

				return webhook.Webhooks
			}, defaultTimeoutSeconds, 1).Should(HaveLen(2))
			webhook, err := clientManaged.AdmissionregistrationV1().ValidatingWebhookConfigurations().Get(
				context.TODO(),
				"gatekeeper-validating-webhook-configuration",
				metav1.GetOptions{},
			)
			Expect(err).ToNot(HaveOccurred())
			Expect(webhook.Webhooks[0].NamespaceSelector.MatchExpressions).To(HaveLen(1))
			Expect(webhook.Webhooks[1].NamespaceSelector.MatchExpressions).To(HaveLen(1))
			Expect(webhook.Webhooks[0].NamespaceSelector.MatchExpressions[0]).NotTo(BeNil())
			Expect(webhook.Webhooks[1].NamespaceSelector.MatchExpressions[0]).NotTo(BeNil())
		})
		It("Gatekeeper audit pod should be running", func() {
			By("Checking if pod gatekeeper-audit has been created")
			Eventually(func(g Gomega) []corev1.Pod {
				podList, err := clientManaged.CoreV1().Pods(gatekeeperNamespace).List(
					context.TODO(),
					metav1.ListOptions{
						LabelSelector: "control-plane=audit-controller",
					},
				)
				g.Expect(err).ToNot(HaveOccurred())

				return podList.Items
			}, defaultTimeoutSeconds*2, 1).Should(HaveLen(1))
			By("Checking if pod gatekeeper-audit is running")
			Eventually(func(g Gomega) interface{} {
				podList, err := clientManaged.CoreV1().Pods(gatekeeperNamespace).List(
					context.TODO(),
					metav1.ListOptions{
						LabelSelector: "control-plane=audit-controller",
					},
				)
				g.Expect(err).ToNot(HaveOccurred())

				return string(podList.Items[0].Status.Phase)
			}, defaultTimeoutSeconds*4, 1).Should(Equal("Running"))
		})
		It("Gatekeeper controller manager pods should be running", func() {
			By("Checking if pod gatekeeper-controller-manager has been created")
			Eventually(func(g Gomega) []corev1.Pod {
				podList, err := clientManaged.CoreV1().Pods(gatekeeperNamespace).List(
					context.TODO(),
					metav1.ListOptions{
						LabelSelector: "control-plane=controller-manager",
					},
				)
				g.Expect(err).ToNot(HaveOccurred())

				return podList.Items
			}, defaultTimeoutSeconds*2, 1).Should(HaveLen(2))
			By("Checking if pod gatekeeper-controller-manager is running")
			Eventually(func(g Gomega) interface{} {
				podList, err := clientManaged.CoreV1().Pods(gatekeeperNamespace).List(
					context.TODO(),
					metav1.ListOptions{
						LabelSelector: "control-plane=controller-manager",
					},
				)
				g.Expect(err).ToNot(HaveOccurred())

				return string(podList.Items[0].Status.Phase) + "/" + string(podList.Items[1].Status.Phase)
			}, defaultTimeoutSeconds*4, 1).Should(Equal("Running/Running"))
		})
		It("stable/policy-gatekeeper-operator should be compliant", func() {
			By("Checking if the status of root policy is compliant")
			Eventually(
				common.GetComplianceState(gatekeeperPolicyName),
				defaultTimeoutSeconds*6,
				10,
			).Should(Equal(policiesv1.Compliant))
		})
		It("Informing stable/policy-gatekeeper-operator", func() {
			common.InformPolicy(gatekeeperPolicyName)
		})
	})

	AfterAll(func(ctx SpecContext) {
		if CurrentSpecReport().Failed() {
			common.OutputDebugInfo(
				"Gatekeeper policies",
				kubeconfigHub,
			)
		}

		// Clean up stable/policy-gatekeeper-operator
		_, err := utils.KubectlWithOutput(
			"delete",
			"-f",
			gatekeeperPolicyURL,
			"-n",
			userNamespace,
			"--kubeconfig="+kubeconfigHub,
			"--ignore-not-found",
		)
		Expect(err).ToNot(HaveOccurred())

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
		out, err := utils.KubectlWithOutput(
			"delete",
			"Gatekeeper",
			"gatekeeper",
			"--kubeconfig="+kubeconfigManaged,
			"--ignore-not-found",
		)
		if err != nil {
			Expect(strings.TrimSpace(out)).To(Equal("error: the server doesn't have a resource type \"Gatekeeper\""))
		}

		Eventually(func() interface{} {
			out, _ := utils.KubectlWithOutput(
				"get", "pods",
				"-n", gatekeeperNamespace,
				"--kubeconfig="+kubeconfigManaged,
			)

			return out
			// k8s will respond with this even if the ns was deleted.
		}, defaultTimeoutSeconds*4, 1).Should(ContainSubstring("No resources found"))

		_, err = utils.KubectlWithOutput(
			"delete",
			"-n", "openshift-operators",
			"subscriptions.operators.coreos.com",
			"gatekeeper-operator-product",
			"--kubeconfig="+kubeconfigManaged,
			"--ignore-not-found",
		)
		Expect(err).ToNot(HaveOccurred())

		csvClient := clientManagedDynamic.Resource(common.GvrClusterServiceVersion)
		csvList, err := csvClient.List(ctx, metav1.ListOptions{})
		Expect(err).ToNot(HaveOccurred())

		for _, csv := range csvList.Items {
			csvName := csv.GetName()
			if strings.HasPrefix(csvName, "gatekeeper-operator-product.") {
				err := csvClient.Namespace(csv.GetNamespace()).Delete(ctx, csvName, metav1.DeleteOptions{})
				if !k8serrors.IsNotFound(err) {
					Expect(err).ToNot(HaveOccurred())
				}
			}
		}

		_, err = utils.KubectlWithOutput(
			"delete",
			"customresourcedefinitions",
			"gatekeepers.operator.gatekeeper.sh",
			"--kubeconfig="+kubeconfigManaged,
			"--ignore-not-found",
		)
		Expect(err).ToNot(HaveOccurred())

		out, _ = utils.KubectlWithOutput(
			"delete",
			"namespace",
			gatekeeperNamespace,
			"--kubeconfig="+kubeconfigManaged,
		)
		Expect(out).To(Or(
			ContainSubstring("namespace \""+gatekeeperNamespace+"\" deleted"),
			ContainSubstring("namespaces \""+gatekeeperNamespace+"\" not found"),
		))

		_, err = utils.KubectlWithOutput(
			"delete",
			"events",
			"-n", clusterNamespace,
			"--field-selector=involvedObject.name="+userNamespace+".policy-gatekeeper-operator",
			"--kubeconfig="+kubeconfigManaged,
			"--ignore-not-found",
		)
		Expect(err).ToNot(HaveOccurred())

		// Restart the governance-policy-framework addon after Gatekeeper is uninstalled to disable the Gatekeeper
		// status controller. This would happen automatically, but it can take a while since it relies on the health
		// endpoint.
		_, err = utils.KubectlWithOutput(
			"-n",
			ocmAddonNS,
			"rollout",
			"restart",
			"deployment/governance-policy-framework",
			"--kubeconfig="+kubeconfigManaged,
		)
		Expect(err).ToNot(HaveOccurred())

		// Wait for the restart to complete.
		_, err = utils.KubectlWithOutput(
			"-n",
			ocmAddonNS,
			"rollout",
			"status",
			"deployment/governance-policy-framework",
			"--timeout=180s",
			"--kubeconfig="+kubeconfigManaged,
		)
		Expect(err).ToNot(HaveOccurred())

		err = common.DeletePlacement(userNamespace, gatekeeperPolicyName)
		Expect(err).ToNot(HaveOccurred())
	})
})
