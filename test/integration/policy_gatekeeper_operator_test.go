// Copyright (c) 2020 Red Hat, Inc.
// Copyright Contributors to the Open Cluster Management project

package integration

import (
	"context"
	"fmt"
	"strings"
	"time"

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
	const GKPolicyYaml = policyCollectCommunityURL +
		"CM-Configuration-Management/policy-gatekeeper-sample.yaml"
	const GKPolicyName = "policy-gatekeeper"
	const GKAssignPolicyYaml = policyCollectCommunityURL +
		"CM-Configuration-Management/policy-gatekeeper-image-pull-policy.yaml"
	const GKAssignPolicyName = "policy-gatekeeper-image-pull-policy"
	const GKAssignMetadataPolicyYaml = policyCollectCommunityURL +
		"CM-Configuration-Management/policy-gatekeeper-annotation-owner.yaml"
	const GKAssignMetadataPolicyName = "policy-gatekeeper-annotation-owner"

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

	Describe("RHACM4K-1274/RHACM4K-1282 GRC: [P1][Sev1][policy-grc] Test community/policy-gatekeeper-sample", func() {
		It("community/policy-gatekeeper-sample should be created on hub", func() {
			By("Creating policy on hub")
			_, err := utils.KubectlWithOutput(
				"apply", "-f",
				GKPolicyYaml,
				"-n", userNamespace,
				"--kubeconfig="+kubeconfigHub,
			)
			Expect(err).To(BeNil())
			By("Policy should be created on hub")
			utils.GetWithTimeout(
				clientHubDynamic,
				common.GvrPolicy,
				GKPolicyName,
				userNamespace,
				true,
				defaultTimeoutSeconds,
			)
			By("Patching to remove dryrun")
			_, err = utils.KubectlWithOutput(
				"patch", "-n", userNamespace,
				common.GvrPolicy.Resource+"."+common.GvrPolicy.Group,
				GKPolicyName,
				"--type=json", "-p=[{\"op\":\"replace\", \"path\": "+
					"\"/spec/policy-templates/0/objectDefinition/spec/object-templates/1/"+
					"objectDefinition/spec/enforcementAction\", \"value\":\"deny\"}]",
				"--kubeconfig="+kubeconfigHub,
			)
			Expect(err).To(BeNil())
			By("Patching placement rule")
			err = common.PatchPlacementRule(
				userNamespace,
				"placement-"+GKPolicyName,
				clusterNamespace,
				kubeconfigHub,
			)
			Expect(err).To(BeNil())
			By("Checking policy-gatekeeper namespace on hub cluster in ns " + userNamespace)
			rootPlc := utils.GetWithTimeout(
				clientHubDynamic,
				common.GvrPolicy,
				GKPolicyName,
				userNamespace,
				true,
				defaultTimeoutSeconds,
			)
			Expect(rootPlc).NotTo(BeNil())
		})
		It("community/policy-gatekeeper-sample should be compliant", func() {
			By("Checking if the status of root policy is compliant")
			Eventually(getComplianceState(GKPolicyName), 10*time.Minute, 1).Should(Equal(policiesv1.Compliant))
			By("Checking if status for policy template policy-gatekeeper-audit is compliant")
			Eventually(func() interface{} {
				plc := utils.GetWithTimeout(
					clientHubDynamic,
					common.GvrPolicy,
					userNamespace+"."+GKPolicyName,
					clusterNamespace,
					true,
					defaultTimeoutSeconds,
				)
				details := plc.Object["status"].(map[string]interface{})["details"].([]interface{})

				return details[1].(map[string]interface{})["compliant"]
			}, defaultTimeoutSeconds*4, 1).Should(Equal("Compliant"))
		})
		It("Grabs the gatekeeper audit duration metric", func() {
			auditPodName, _ := common.OcManaged("get", "pod", "-n=openshift-gatekeeper-system",
				"-l=control-plane=audit-controller", `-o=jsonpath={.items[0].metadata.name}`)
			_, _ = common.OcManaged(
				"exec", "-n=openshift-gatekeeper-system", auditPodName, "--", "bash",
				"-c", "curl -s localhost:8888/metrics | grep -A1 audit_duration_seconds_sum")
			/* example output:
			gatekeeper-audit-9c88bf969-mgf5r
			gatekeeper_audit_duration_seconds_sum 1005.4185594219999
			gatekeeper_audit_duration_seconds_count 25
			*/
		})
		It("Creating a valid ns should not be blocked by gatekeeper", func() {
			By("Creating a namespace called e2etestsuccess on managed")
			Eventually(func() interface{} {
				out, _ := utils.KubectlWithOutput(
					"apply", "-f",
					"../resources/gatekeeper/ns-create-valid.yaml",
					"--kubeconfig="+kubeconfigManaged,
				)

				return out
			}, defaultTimeoutSeconds*6, 1).Should(ContainSubstring("namespace/e2etestsuccess created"))
		})
		It("Creating an invalid ns should generate a violation message", func() {
			By("Creating invalid namespace on managed")
			Eventually(func() interface{} {
				out, _ := utils.KubectlWithOutput(
					"apply", "-f",
					"../resources/gatekeeper/ns-create-invalid.yaml",
					"--kubeconfig="+kubeconfigManaged,
				)

				if strings.Contains(out, "namespace/e2etestfail created") {
					GinkgoWriter.Println("Deleting created namespace to retry create:")
					_, err := utils.KubectlWithOutput(
						"delete",
						"-f",
						"../resources/gatekeeper/ns-create-invalid.yaml",
						"--kubeconfig="+kubeconfigManaged,
						"--ignore-not-found",
					)
					Expect(err).To(BeNil())
				}

				return out
			}, defaultTimeoutSeconds*6, 5).Should(And(
				ContainSubstring("validation.gatekeeper.sh"),
				ContainSubstring("denied"),
				ContainSubstring("ns-must-have-gk")))
			By("Checking if status for policy template policy-gatekeeper-admission is noncompliant")
			Eventually(func() interface{} {
				plc := utils.GetWithTimeout(
					clientHubDynamic,
					common.GvrPolicy,
					userNamespace+"."+GKPolicyName,
					clusterNamespace,
					true,
					defaultTimeoutSeconds,
				)
				details := plc.Object["status"].(map[string]interface{})["details"].([]interface{})

				return details[2].(map[string]interface{})["compliant"]
			}, defaultTimeoutSeconds*2, 1).Should(Equal("NonCompliant"))
			By("Checking if violation message for policy template policy-gatekeeper-admission is noncompliant")
			Eventually(func() interface{} {
				plc := utils.GetWithTimeout(
					clientHubDynamic,
					common.GvrPolicy,
					userNamespace+"."+GKPolicyName,
					clusterNamespace,
					true,
					defaultTimeoutSeconds,
				)
				details := plc.Object["status"].(map[string]interface{})["details"].([]interface{})
				history := details[2].(map[string]interface{})["history"]
				fmt.Printf("%v\n", history.([]interface{})[0].(map[string]interface{})["message"])

				return history.([]interface{})[0].(map[string]interface{})["message"]
			}, defaultTimeoutSeconds, 1).Should(And(
				ContainSubstring("NonCompliant; violation - events found:"),
				ContainSubstring("e2etestfail.")))
		})
		It("community/policy-gatekeeper-sample should be noncompliant", func() {
			By("Checking if the status of root policy is noncompliant")
			Eventually(
				getComplianceState(GKPolicyName),
				defaultTimeoutSeconds*4,
				1,
			).Should(Equal(policiesv1.NonCompliant))
		})
	})

	Describe("GRC: [P1][Sev1][policy-grc] Test enabling gatekeeper mutation feature", func() {
		It("Enabling mutation feature through policy", func() {
			Eventually(func() interface{} {
				By("Patching mutatingWebhook = Enabled on root policy")
				rootPlc := utils.GetWithTimeout(
					clientHubDynamic,
					common.GvrPolicy,
					gatekeeperPolicyName,
					userNamespace,
					true,
					defaultTimeoutSeconds,
				)

				policyTemplates := rootPlc.Object["spec"].(map[string]interface{})["policy-templates"]
				objectDefinition := policyTemplates.([]interface{})[4].(map[string]interface{})["objectDefinition"]
				spec := objectDefinition.(map[string]interface{})["spec"]
				objectTemplates := spec.(map[string]interface{})["object-templates"]
				finalObjectDefinition := objectTemplates.([]interface{})[0].(map[string]interface{})["objectDefinition"]
				finalSpec := finalObjectDefinition.(map[string]interface{})["spec"]
				finalSpec.(map[string]interface{})["mutatingWebhook"] = "Enabled"

				_, err := clientHubDynamic.Resource(common.GvrPolicy).Namespace(userNamespace).Update(
					context.TODO(),
					rootPlc,
					metav1.UpdateOptions{},
				)
				if err != nil {
					return fmt.Errorf("failed to enable the gatekeeper policy %s on name space %s: %w",
						gatekeeperPolicyName, userNamespace, err)
				}

				By("Checking if mutatingWebhook is Enabled for root policy")
				rootPlc = utils.GetWithTimeout(
					clientHubDynamic,
					common.GvrPolicy,
					gatekeeperPolicyName,
					userNamespace,
					true,
					defaultTimeoutSeconds,
				)

				policyTemplates = rootPlc.Object["spec"].(map[string]interface{})["policy-templates"]
				objectDefinition = policyTemplates.([]interface{})[4].(map[string]interface{})["objectDefinition"]
				spec = objectDefinition.(map[string]interface{})["spec"]
				objectTemplates = spec.(map[string]interface{})["object-templates"]
				finalObjectDefinition = objectTemplates.([]interface{})[0].(map[string]interface{})["objectDefinition"]
				finalSpec = finalObjectDefinition.(map[string]interface{})["spec"]

				return finalSpec.(map[string]interface{})["mutatingWebhook"]
			}, defaultTimeoutSeconds, 1).Should(Equal("Enabled"))
		})
		It("Enforcing policy-gatekeeper-operator to enable mutation feature", func() {
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
		It("policy-gatekeeper-operator should be compliant", func() {
			By("Checking if the status of root policy is compliant")
			Eventually(
				getComplianceState(gatekeeperPolicyName),
				defaultTimeoutSeconds*2,
				1,
			).Should(Equal(policiesv1.Compliant))
		})
		It("Checking if Assign/AssingnMetadata CRDs have been created", func() {
			Eventually(func() interface{} {
				out, _ := utils.KubectlWithOutput(
					"get", "crd",
					"assign.mutations.gatekeeper.sh",
					"--kubeconfig="+kubeconfigManaged,
				)

				return out
			}, defaultTimeoutSeconds*8, 1).Should(
				ContainSubstring("CREATED AT\nassign.mutations.gatekeeper.sh"))
			Eventually(func() interface{} {
				out, _ := utils.KubectlWithOutput(
					"get", "crd",
					"assignmetadata.mutations.gatekeeper.sh",
					"--kubeconfig="+kubeconfigManaged,
				)

				return out
			}, defaultTimeoutSeconds*4, 1).Should(
				ContainSubstring("CREATED AT\nassignmetadata.mutations.gatekeeper.sh"))
		})
		It("Checking if mutating webhook gatekeeper-mutating-webhook-configuration exists", func() {
			Eventually(func() interface{} {
				out, _ := utils.KubectlWithOutput(
					"get",
					"mutatingwebhookconfigurations.admissionregistration.k8s.io",
					"gatekeeper-mutating-webhook-configuration",
					"--kubeconfig="+kubeconfigManaged,
				)

				return out
			}, defaultTimeoutSeconds*2, 1).Should(
				ContainSubstring("AGE\ngatekeeper-mutating-webhook-configuration"))
		})
		It("Checking if gatekeeper controller manager has mutation flag on", func() {
			Eventually(func() interface{} {
				podList, _ := clientManaged.CoreV1().Pods("openshift-gatekeeper-system").List(
					context.TODO(),
					metav1.ListOptions{
						LabelSelector: "control-plane=controller-manager",
					},
				)
				// ensure there are two pods before checking the args
				if len(podList.Items) != 2 {
					return "0;0"
				}

				return fmt.Sprintf(
					"%d;%d",
					len(podList.Items[0].Spec.Containers[0].Args),
					len(podList.Items[1].Spec.Containers[0].Args),
				)
			}, common.MaxTravisTimeoutSeconds, 1).Should(Equal("7;7"))
			Eventually(func() interface{} {
				podList, _ := clientManaged.CoreV1().Pods("openshift-gatekeeper-system").List(
					context.TODO(),
					metav1.ListOptions{
						LabelSelector: "control-plane=controller-manager",
					},
				)
				// create a list to avoid hard-coding checking the order the arguments
				args := make([]string, 7)
				for i := 0; i < 7; i++ {
					args[i] = podList.Items[0].Spec.Containers[0].Args[i] +
						";" +
						podList.Items[1].Spec.Containers[0].Args[i]
				}

				return args
			}, defaultTimeoutSeconds, 1).Should(ContainElement("--enable-mutation=true;--enable-mutation=true"))
		})
	})

	Describe("GRC: [P1][Sev1][policy-grc] Install mutation policy", func() {
		It("Creating mutation policy on hub", func() {
			By("Creating " + GKAssignPolicyName + " on hub")
			_, err := utils.KubectlWithOutput(
				"apply", "-f",
				GKAssignPolicyYaml,
				"-n",
				userNamespace,
				"--kubeconfig="+kubeconfigHub,
			)
			Expect(err).To(BeNil())
			By("Patching placement rule")
			err = common.PatchPlacementRule(
				userNamespace,
				"placement-"+GKAssignPolicyName,
				clusterNamespace,
				kubeconfigHub,
			)
			Expect(err).To(BeNil())
			By("Creating " + GKAssignMetadataPolicyName + " on hub")
			_, err = utils.KubectlWithOutput(
				"apply", "-f",
				GKAssignMetadataPolicyYaml,
				"-n",
				userNamespace,
				"--kubeconfig="+kubeconfigHub,
			)
			Expect(err).To(BeNil())
			By("Patching placement rule")
			err = common.PatchPlacementRule(
				userNamespace,
				"placement-"+GKAssignMetadataPolicyName,
				clusterNamespace,
				kubeconfigHub,
			)
			Expect(err).To(BeNil())
		})
		It(GKAssignPolicyName+" should be compliant", func() {
			By("Checking if the status of root policy is compliant")
			Eventually(
				getComplianceState(GKAssignPolicyName),
				defaultTimeoutSeconds*2,
				1,
			).Should(Equal(policiesv1.Compliant))
		})
		It(GKAssignMetadataPolicyName+" should be compliant", func() {
			By("Checking if the status of root policy is compliant")
			Eventually(
				getComplianceState(GKAssignMetadataPolicyName),
				defaultTimeoutSeconds*2,
				1,
			).Should(Equal(policiesv1.Compliant))
		})
	})

	Describe("GRC: [P1][Sev1][policy-grc] Test gatekeeper mutation feature", func() {
		It("Verify mutation feature", func() {
			Eventually(func() interface{} {
				By("Creating a pod to test AssignMetadata")
				_, err := utils.KubectlWithOutput(
					"apply",
					"-f",
					"../resources/gatekeeper/pod-mutation.yaml",
					"-n",
					"e2etestsuccess",
					"--kubeconfig="+kubeconfigManaged,
				)
				Expect(err).To(BeNil())

				By("Check if pod contains annotation owner=admin")
				pod, _ := clientManaged.CoreV1().Pods("e2etestsuccess").Get(
					context.TODO(),
					"pod-mutation",
					metav1.GetOptions{},
				)

				return pod.Annotations["owner"]
			}, defaultTimeoutSeconds*6, 1).Should(Equal("admin"))
			Eventually(func() interface{} {
				By("Creating a pod to test Assign")
				_, err := utils.KubectlWithOutput(
					"apply",
					"-f",
					"../resources/gatekeeper/pod-mutation.yaml",
					"-n",
					"e2etestsuccess",
					"--kubeconfig="+kubeconfigManaged,
				)
				Expect(err).To(BeNil())

				By("Check if imagepullpolicy has been mutated to Always")
				pod, _ := clientManaged.CoreV1().Pods("e2etestsuccess").Get(
					context.TODO(),
					"pod-mutation",
					metav1.GetOptions{},
				)

				return string(pod.Spec.Containers[0].ImagePullPolicy)
			}, defaultTimeoutSeconds*6, 1).Should(Equal("Always"))
		})
	})

	Describe("GRC: [P1][Sev1][policy-grc] Test disabling gatekeeper mutation feature", func() {
		It("Disabling mutation feature through policy", func() {
			Eventually(func() interface{} {
				By("Patching mutatingWebhook = Disabled on root policy")
				rootPlc := utils.GetWithTimeout(
					clientHubDynamic,
					common.GvrPolicy,
					gatekeeperPolicyName,
					userNamespace,
					true,
					defaultTimeoutSeconds,
				)

				policyTemplates := rootPlc.Object["spec"].(map[string]interface{})["policy-templates"]
				objectDefinition := policyTemplates.([]interface{})[4].(map[string]interface{})["objectDefinition"]
				spec := objectDefinition.(map[string]interface{})["spec"]
				objectTemplates := spec.(map[string]interface{})["object-templates"]
				finalObjectDefinition := objectTemplates.([]interface{})[0].(map[string]interface{})["objectDefinition"]
				finalSpec := finalObjectDefinition.(map[string]interface{})["spec"]
				finalSpec.(map[string]interface{})["mutatingWebhook"] = "Disabled"

				_, err := clientHubDynamic.Resource(common.GvrPolicy).Namespace(userNamespace).Update(
					context.TODO(),
					rootPlc,
					metav1.UpdateOptions{},
				)
				if err != nil {
					return fmt.Errorf("failed to disable the gatekeeper policy %s on namespace %s: %w",
						gatekeeperPolicyName, userNamespace, err)
				}

				By("Checking if mutatingWebhook is Disabled for root policy")
				rootPlc = utils.GetWithTimeout(
					clientHubDynamic,
					common.GvrPolicy,
					gatekeeperPolicyName,
					userNamespace,
					true,
					defaultTimeoutSeconds,
				)
				policyTemplates = rootPlc.Object["spec"].(map[string]interface{})["policy-templates"]
				objectDefinition = policyTemplates.([]interface{})[4].(map[string]interface{})["objectDefinition"]
				spec = objectDefinition.(map[string]interface{})["spec"]
				objectTemplates = spec.(map[string]interface{})["object-templates"]
				finalObjectDefinition = objectTemplates.([]interface{})[0].(map[string]interface{})["objectDefinition"]
				finalSpec = finalObjectDefinition.(map[string]interface{})["spec"]

				return finalSpec.(map[string]interface{})["mutatingWebhook"]
			}, defaultTimeoutSeconds, 1).Should(Equal("Disabled"))
		})
		It("Checking if Assign/AssingnMetadata CRDs have been removed", func() {
			Eventually(func() interface{} {
				out, _ := utils.KubectlWithOutput(
					"get", "crd",
					"assign.mutations.gatekeeper.sh",
					"--kubeconfig="+kubeconfigManaged,
				)

				return out
			}, defaultTimeoutSeconds*8, 1).Should(ContainSubstring("not found"))
			Eventually(func() interface{} {
				out, _ := utils.KubectlWithOutput(
					"get", "crd",
					"assignmetadata.mutations.gatekeeper.sh",
					"--kubeconfig="+kubeconfigManaged,
				)

				return out
			}, defaultTimeoutSeconds*2, 1).Should(ContainSubstring("not found"))
		})
		It("Checking if gatekeeper controller manager has mutation flag off", func() {
			Eventually(func() interface{} {
				podList, _ := clientManaged.CoreV1().Pods("openshift-gatekeeper-system").List(
					context.TODO(),
					metav1.ListOptions{
						LabelSelector: "control-plane=controller-manager",
					},
				)

				if len(podList.Items) != 2 {
					return "0/0"
				}

				return fmt.Sprintf(
					"%d/%d",
					len(podList.Items[0].Spec.Containers[0].Args),
					len(podList.Items[1].Spec.Containers[0].Args),
				)
			}, common.MaxTravisTimeoutSeconds, 1).Should(Equal("6/6"))
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
		// Clean up mutation policies
		_, err := utils.KubectlWithOutput(
			"delete", "-f",
			GKAssignMetadataPolicyYaml,
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
				userNamespace+"."+GKAssignMetadataPolicyName,
				clusterNamespace,
				false,
				defaultTimeoutSeconds,
			)

			return managedPlc
		}, defaultTimeoutSeconds, 1).Should(BeNil())

		_, err = utils.KubectlWithOutput(
			"delete", "-f",
			GKAssignPolicyYaml,
			"-n", userNamespace,
			"--kubeconfig="+kubeconfigHub,
			"--ignore-not-found",
		)
		Expect(err).To(BeNil())

		Eventually(func() interface{} {
			managedPlc := utils.GetWithTimeout(
				clientManagedDynamic,
				common.GvrPolicy,
				userNamespace+"."+GKAssignPolicyName,
				clusterNamespace,
				false,
				defaultTimeoutSeconds,
			)

			return managedPlc
		}, defaultTimeoutSeconds, 1).Should(BeNil())

		_, err = utils.KubectlWithOutput(
			"delete", "-f",
			"../resources/gatekeeper/pod-mutation.yaml",
			"-n", "e2etestsuccess",
			"--kubeconfig="+kubeconfigManaged,
			"--ignore-not-found",
		)
		Expect(err).To(BeNil())

		// Clean up community/policy-gatekeeper-sample
		_, err = utils.KubectlWithOutput(
			"delete", "-f",
			GKPolicyYaml,
			"-n", userNamespace,
			"--kubeconfig="+kubeconfigHub,
			"--ignore-not-found",
		)
		Expect(err).To(BeNil())

		Eventually(func() interface{} {
			managedPlc := utils.GetWithTimeout(
				clientManagedDynamic,
				common.GvrPolicy,
				userNamespace+"."+GKPolicyName,
				clusterNamespace,
				false,
				defaultTimeoutSeconds,
			)

			return managedPlc
		}, defaultTimeoutSeconds, 1).Should(BeNil())
		_, err = utils.KubectlWithOutput(
			"delete", "ns", "e2etestsuccess",
			"--kubeconfig="+kubeconfigManaged,
			"--ignore-not-found",
		)
		Expect(err).To(BeNil())

		_, err = utils.KubectlWithOutput(
			"delete", "ns", "e2etestfail",
			"--kubeconfig="+kubeconfigManaged,
			"--ignore-not-found",
		)
		Expect(err).To(BeNil())

		// Clean up community/policy-gatekeeper-operator
		_, err = utils.KubectlWithOutput(
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

		_, err = utils.KubectlWithOutput(
			"delete",
			"events",
			"-n",
			clusterNamespace,
			"--field-selector=involvedObject.name="+userNamespace+".policy-gatekeeper",
			"--kubeconfig="+kubeconfigManaged,
			"--ignore-not-found",
		)
		Expect(err).To(BeNil())

		_, err = utils.KubectlWithOutput(
			"delete",
			"events",
			"-n",
			clusterNamespace,
			"--field-selector=involvedObject.name="+userNamespace+".policy-gatekeeper-image-pull-policy",
			"--kubeconfig="+kubeconfigManaged,
			"--ignore-not-found",
		)
		Expect(err).To(BeNil())

		_, err = utils.KubectlWithOutput(
			"delete",
			"events",
			"-n",
			clusterNamespace,
			"--field-selector=involvedObject.name="+userNamespace+".policy-gatekeeper-annotation-owner",
			"--kubeconfig="+kubeconfigManaged,
			"--ignore-not-found",
		)
		Expect(err).To(BeNil())
	})
})
