// Copyright (c) 2020 Red Hat, Inc.
// Copyright Contributors to the Open Cluster Management project

package e2e

import (
	"context"
	"fmt"
	"strings"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/open-cluster-management/governance-policy-framework/test/common"
	policiesv1 "github.com/open-cluster-management/governance-policy-propagator/pkg/apis/policy/v1"
	"github.com/open-cluster-management/governance-policy-propagator/test/utils"
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
	const gatekeeperPolicyURL = "https://raw.githubusercontent.com/open-cluster-management/policy-collection/main/stable/CM-Configuration-Management/policy-gatekeeper-operator-downstream.yaml"
	const gatekeeperPolicyName = "policy-gatekeeper-operator"
	const GKPolicyYaml = "https://raw.githubusercontent.com/open-cluster-management/policy-collection/main/community/CM-Configuration-Management/policy-gatekeeper-sample.yaml"
	const GKPolicyName = "policy-gatekeeper"
	const GKAssignPolicyYaml = "https://raw.githubusercontent.com/open-cluster-management/policy-collection/main/community/CM-Configuration-Management/policy-gatekeeper-image-pull-policy.yaml"
	const GKAssignPolicyName = "policy-gatekeeper-image-pull-policy"
	const GKAssignMetadataPolicyYaml = "https://raw.githubusercontent.com/open-cluster-management/policy-collection/main/community/CM-Configuration-Management/policy-gatekeeper-annotation-owner.yaml"
	const GKAssignMetadataPolicyName = "policy-gatekeeper-annotation-owner"

	Describe("GRC: [P1][Sev1][policy-grc] Test installing gatekeeper operator", func() {
		It("stable/policy-gatekeeper-operator should be created on hub", func() {
			By("Creating policy on hub")
			utils.KubectlWithOutput("apply", "-f", gatekeeperPolicyURL, "-n", userNamespace, "--kubeconfig="+kubeconfigHub)
			By("Patching placement rule")
			utils.KubectlWithOutput("patch", "-n", userNamespace, "placementrule.apps.open-cluster-management.io/placement-"+gatekeeperPolicyName,
				"--type=json", "-p=[{\"op\": \"replace\", \"path\": \"/spec/clusterSelector/matchExpressions\", \"value\":[{\"key\": \"name\", \"operator\": \"In\", \"values\": ["+clusterNamespace+"]}]}]",
				"--kubeconfig="+kubeconfigHub)
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
			Eventually(func() interface{} {
				if i == 60*2 || i == 60*4 {
					fmt.Println("gatekeeper operator pod still not created, deleting subscription and let it recreate", i)
					utils.KubectlWithOutput("delete", "-n", "openshift-operators", "subscriptions.operators.coreos.com", "gatekeeper-operator-product", "--kubeconfig="+kubeconfigManaged)
				}
				i++
				podList, err := clientManaged.CoreV1().Pods("openshift-operators").List(context.TODO(), metav1.ListOptions{LabelSelector: "control-plane in (controller-manager, gatekeeper-operator-controller-manager)"})
				Expect(err).To(BeNil())
				return len(podList.Items)
			}, defaultTimeoutSeconds*12, 1).Should(Equal(1))
			By("Checking if pod gatekeeper-operator is running")
			Eventually(func() interface{} {
				podList, err := clientManaged.CoreV1().Pods("openshift-operators").List(context.TODO(), metav1.ListOptions{LabelSelector: "control-plane in (controller-manager, gatekeeper-operator-controller-manager)"})
				Expect(err).To(BeNil())
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
			}, defaultTimeoutSeconds*4, 1).Should(Equal("Running"))
		})

		It("Gatekeeper controller manager pods should be running", func() {
			By("Checking if pod gatekeeper-controller-manager has been created")
			Eventually(func() interface{} {
				podList, err := clientManaged.CoreV1().Pods("openshift-gatekeeper-system").List(context.TODO(), metav1.ListOptions{LabelSelector: "control-plane=controller-manager"})
				Expect(err).To(BeNil())
				return len(podList.Items)
			}, defaultTimeoutSeconds*2, 1).Should(Equal(2))
			By("Checking if pod gatekeeper-controller-manager is running")
			Eventually(func() interface{} {
				podList, err := clientManaged.CoreV1().Pods("openshift-gatekeeper-system").List(context.TODO(), metav1.ListOptions{LabelSelector: "control-plane=controller-manager"})
				Expect(err).To(BeNil())
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

	Describe("GRC: [P1][Sev1][policy-grc] Test stable/policy-gatekeeper-sample", func() {
		It("stable/policy-gatekeeper-sample should be created on hub", func() {
			By("Creating policy on hub")
			utils.KubectlWithOutput("apply", "-f", GKPolicyYaml, "-n", userNamespace, "--kubeconfig="+kubeconfigHub)
			By("Patching placement rule")
			utils.KubectlWithOutput("patch", "-n", userNamespace, "placementrule.apps.open-cluster-management.io/placement-"+GKPolicyName,
				"--type=json", "-p=[{\"op\": \"replace\", \"path\": \"/spec/clusterSelector/matchExpressions\", \"value\":[{\"key\": \"name\", \"operator\": \"In\", \"values\": ["+clusterNamespace+"]}]}]",
				"--kubeconfig="+kubeconfigHub)
			By("Patching to remove dryrun")
			utils.KubectlWithOutput(
				"patch", "-n", userNamespace, common.GvrPolicy.Resource+"."+common.GvrPlacementBinding.Group, GKPolicyName,
				"--type=json", "-p=[{\"op\":\"remove\", \"path\": "+
					"\"/spec/policy-templates/0/objectDefinition/spec/object-templates/1/objectDefinition/spec/enforcementAction\"}]",
			)
			By("Checking policy-gatekeeper namespace on hub cluster in ns " + userNamespace)
			rootPlc := utils.GetWithTimeout(clientHubDynamic, common.GvrPolicy, GKPolicyName, userNamespace, true, defaultTimeoutSeconds)
			Expect(rootPlc).NotTo(BeNil())
		})
		It("stable/policy-gatekeeper-sample should be compliant", func() {
			By("Checking if the status of root policy is compliant")
			Eventually(getComplianceState(GKPolicyName), defaultTimeoutSeconds*6, 1).Should(Equal(policiesv1.Compliant))
			By("Checking if status for policy template policy-gatekeeper-audit is compliant")
			Eventually(func() interface{} {
				plc := utils.GetWithTimeout(clientHubDynamic, common.GvrPolicy, userNamespace+"."+GKPolicyName, clusterNamespace, true, defaultTimeoutSeconds)
				details := plc.Object["status"].(map[string]interface{})["details"].([]interface{})
				return details[1].(map[string]interface{})["compliant"]
			}, defaultTimeoutSeconds, 1).Should(Equal("Compliant"))
		})
		It("Creating a valid ns should not be blocked by gatekeeper", func() {
			By("Creating a namespace called e2etestsuccess on managed")
			Eventually(func() interface{} {
				out, _ := utils.KubectlWithOutput("apply", "-f", "../resources/gatekeeper/ns-create-valid.yaml", "--kubeconfig="+kubeconfigManaged)
				return out
			}, defaultTimeoutSeconds*6, 1).Should(ContainSubstring("namespace/e2etestsuccess created"))
		})
		It("Creating an invalid ns should generate a violation message", func() {
			By("Creating invalid namespace on managed")
			Eventually(func() interface{} {
				out, _ := utils.KubectlWithOutput("create", "ns", "e2etestfail", "--kubeconfig="+kubeconfigManaged)
				return out
			}, defaultTimeoutSeconds*6, 1).Should(And(
				ContainSubstring("validation.gatekeeper.sh"),
				ContainSubstring("denied"),
				ContainSubstring("ns-must-have-gk")))
			By("Checking if status for policy template policy-gatekeeper-admission is noncompliant")
			Eventually(func() interface{} {
				plc := utils.GetWithTimeout(clientHubDynamic, common.GvrPolicy, userNamespace+"."+GKPolicyName, clusterNamespace, true, defaultTimeoutSeconds)
				details := plc.Object["status"].(map[string]interface{})["details"].([]interface{})
				return details[2].(map[string]interface{})["compliant"]
			}, defaultTimeoutSeconds*2, 1).Should(Equal("NonCompliant"))
			By("Checking if violation message for policy template policy-gatekeeper-admission is noncompliant")
			Eventually(func() interface{} {
				plc := utils.GetWithTimeout(clientHubDynamic, common.GvrPolicy, userNamespace+"."+GKPolicyName, clusterNamespace, true, defaultTimeoutSeconds)
				details := plc.Object["status"].(map[string]interface{})["details"].([]interface{})
				fmt.Printf("%v\n", details[2].(map[string]interface{})["history"].([]interface{})[0].(map[string]interface{})["message"])
				return details[2].(map[string]interface{})["history"].([]interface{})[0].(map[string]interface{})["message"]
			}, defaultTimeoutSeconds, 1).Should(And(
				ContainSubstring("NonCompliant; violation - events found:"),
				ContainSubstring("e2etestfail.")))
		})
		It("stable/policy-gatekeeper-sample should be noncompliant", func() {
			By("Checking if the status of root policy is noncompliant")
			Eventually(getComplianceState(GKPolicyName), defaultTimeoutSeconds*4, 1).Should(Equal(policiesv1.NonCompliant))
		})
	})

	Describe("GRC: [P1][Sev1][policy-grc] Test enabling gatekeeper mutation feature", func() {
		It("Enabling mutation feature through policy", func() {
			Eventually(func() interface{} {
				By("Patching mutatingWebhook = Enabled on root policy")
				rootPlc := utils.GetWithTimeout(clientHubDynamic, common.GvrPolicy, gatekeeperPolicyName, userNamespace, true, defaultTimeoutSeconds)
				rootPlc.Object["spec"].(map[string]interface{})["policy-templates"].([]interface{})[1].(map[string]interface{})["objectDefinition"].(map[string]interface{})["spec"].(map[string]interface{})["object-templates"].([]interface{})[0].(map[string]interface{})["objectDefinition"].(map[string]interface{})["spec"].(map[string]interface{})["mutatingWebhook"] = "Enabled"
				clientHubDynamic.Resource(common.GvrPolicy).Namespace(userNamespace).Update(context.TODO(), rootPlc, metav1.UpdateOptions{})
				By("Checking if mutatingWebhook is Enabled for root policy")
				rootPlc = utils.GetWithTimeout(clientHubDynamic, common.GvrPolicy, gatekeeperPolicyName, userNamespace, true, defaultTimeoutSeconds)
				return rootPlc.Object["spec"].(map[string]interface{})["policy-templates"].([]interface{})[1].(map[string]interface{})["objectDefinition"].(map[string]interface{})["spec"].(map[string]interface{})["object-templates"].([]interface{})[0].(map[string]interface{})["objectDefinition"].(map[string]interface{})["spec"].(map[string]interface{})["mutatingWebhook"]
			}, defaultTimeoutSeconds, 1).Should(Equal("Enabled"))
		})
		It("Enforcing policy-gatekeeper-operator to enable mutation feature", func() {
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
		It("policy-gatekeeper-operator should be compliant", func() {
			By("Checking if the status of root policy is compliant")
			Eventually(getComplianceState(gatekeeperPolicyName), defaultTimeoutSeconds*2, 1).Should(Equal(policiesv1.Compliant))
		})
		It("Checking if Assign/AssingnMetadata CRDs have been created", func() {
			Eventually(func() interface{} {
				out, _ := utils.KubectlWithOutput("get", "crd", "assign.mutations.gatekeeper.sh", "--kubeconfig="+kubeconfigManaged)
				return out
			}, defaultTimeoutSeconds*8, 1).Should(ContainSubstring("CREATED AT\nassign.mutations.gatekeeper.sh"))
			Eventually(func() interface{} {
				out, _ := utils.KubectlWithOutput("get", "crd", "assignmetadata.mutations.gatekeeper.sh", "--kubeconfig="+kubeconfigManaged)
				return out
			}, defaultTimeoutSeconds*2, 1).Should(ContainSubstring("CREATED AT\nassignmetadata.mutations.gatekeeper.sh"))
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
		It("Checking if gatekeeper controller manager has mutation flag on", func() {
			Eventually(func() interface{} {
				podList, _ := clientManaged.CoreV1().Pods("openshift-gatekeeper-system").List(context.TODO(), metav1.ListOptions{LabelSelector: "control-plane=controller-manager"})
				return fmt.Sprintf("%d;%d", len(podList.Items[0].Spec.Containers[0].Args), len(podList.Items[1].Spec.Containers[0].Args))
			}, defaultTimeoutSeconds*15, 1).Should(Equal("7;7"))
			Eventually(func() interface{} {
				podList, _ := clientManaged.CoreV1().Pods("openshift-gatekeeper-system").List(context.TODO(), metav1.ListOptions{LabelSelector: "control-plane=controller-manager"})
				// create a list to avoid hard-coding checking the order the arguments
				args := make([]string, 7)
				for i := 0; i < 7; i++ {
					args[i] = podList.Items[0].Spec.Containers[0].Args[i] + ";" + podList.Items[1].Spec.Containers[0].Args[i]
				}
				return args
			}, defaultTimeoutSeconds, 1).Should(ContainElement("--enable-mutation=true;--enable-mutation=true"))
		})
	})

	Describe("GRC: [P1][Sev1][policy-grc] Install mutation policy", func() {
		It("Creating mutation policy on hub", func() {
			By("Creating " + GKAssignPolicyName + " on hub")
			utils.KubectlWithOutput("apply", "-f", GKAssignPolicyYaml, "-n", userNamespace, "--kubeconfig="+kubeconfigHub)
			By("Patching placement rule")
			utils.KubectlWithOutput("patch", "-n", userNamespace, "placementrule.apps.open-cluster-management.io/placement-"+GKAssignPolicyName,
				"--type=json", "-p=[{\"op\": \"replace\", \"path\": \"/spec/clusterSelector/matchExpressions\", \"value\":[{\"key\": \"name\", \"operator\": \"In\", \"values\": ["+clusterNamespace+"]}]}]",
				"--kubeconfig="+kubeconfigHub)
			By("Creating " + GKAssignMetadataPolicyName + " on hub")
			utils.KubectlWithOutput("apply", "-f", GKAssignMetadataPolicyYaml, "-n", userNamespace, "--kubeconfig="+kubeconfigHub)
			By("Patching placement rule")
			utils.KubectlWithOutput("patch", "-n", userNamespace, "placementrule.apps.open-cluster-management.io/placement-"+GKAssignMetadataPolicyName,
				"--type=json", "-p=[{\"op\": \"replace\", \"path\": \"/spec/clusterSelector/matchExpressions\", \"value\":[{\"key\": \"name\", \"operator\": \"In\", \"values\": ["+clusterNamespace+"]}]}]",
				"--kubeconfig="+kubeconfigHub)
		})
		It(GKAssignPolicyName+" should be compliant", func() {
			By("Checking if the status of root policy is compliant")
			Eventually(getComplianceState(GKAssignPolicyName), defaultTimeoutSeconds*2, 1).Should(Equal(policiesv1.Compliant))
		})
		It(GKAssignMetadataPolicyName+" should be compliant", func() {
			By("Checking if the status of root policy is compliant")
			Eventually(getComplianceState(GKAssignMetadataPolicyName), defaultTimeoutSeconds*2, 1).Should(Equal(policiesv1.Compliant))
		})

	})

	Describe("GRC: [P1][Sev1][policy-grc] Test gatekeeper mutation feature", func() {
		It("Verify mutation feature", func() {
			Eventually(func() interface{} {
				By("Creating a pod to test AssignMetadata")
				utils.KubectlWithOutput("apply", "-f", "../resources/gatekeeper/pod-mutation.yaml", "-n", "default", "--kubeconfig="+kubeconfigManaged)
				By("Check if pod contains annotation owner=admin")
				pod, _ := clientManaged.CoreV1().Pods("default").Get(context.TODO(), "pod-mutation", metav1.GetOptions{})
				return pod.Annotations["owner"]
			}, defaultTimeoutSeconds*6, 1).Should(Equal("admin"))
			Eventually(func() interface{} {
				By("Creating a pod to test Assign")
				utils.KubectlWithOutput("apply", "-f", "../resources/gatekeeper/pod-mutation.yaml", "-n", "default", "--kubeconfig="+kubeconfigManaged)
				By("Check if imagepullpolicy has been mutated to Always")
				pod, _ := clientManaged.CoreV1().Pods("default").Get(context.TODO(), "pod-mutation", metav1.GetOptions{})
				return string(pod.Spec.Containers[0].ImagePullPolicy)
			}, defaultTimeoutSeconds*6, 1).Should(Equal("Always"))

		})
	})

	Describe("GRC: [P1][Sev1][policy-grc] Test disabling gatekeeper mutation feature", func() {
		It("Disabling mutation feature through policy", func() {
			Eventually(func() interface{} {
				By("Patching mutatingWebhook = Disabled on root policy")
				rootPlc := utils.GetWithTimeout(clientHubDynamic, common.GvrPolicy, gatekeeperPolicyName, userNamespace, true, defaultTimeoutSeconds)
				rootPlc.Object["spec"].(map[string]interface{})["policy-templates"].([]interface{})[1].(map[string]interface{})["objectDefinition"].(map[string]interface{})["spec"].(map[string]interface{})["object-templates"].([]interface{})[0].(map[string]interface{})["objectDefinition"].(map[string]interface{})["spec"].(map[string]interface{})["mutatingWebhook"] = "Disabled"
				clientHubDynamic.Resource(common.GvrPolicy).Namespace(userNamespace).Update(context.TODO(), rootPlc, metav1.UpdateOptions{})
				By("Checking if mutatingWebhook is Disabled for root policy")
				rootPlc = utils.GetWithTimeout(clientHubDynamic, common.GvrPolicy, gatekeeperPolicyName, userNamespace, true, defaultTimeoutSeconds)
				return rootPlc.Object["spec"].(map[string]interface{})["policy-templates"].([]interface{})[1].(map[string]interface{})["objectDefinition"].(map[string]interface{})["spec"].(map[string]interface{})["object-templates"].([]interface{})[0].(map[string]interface{})["objectDefinition"].(map[string]interface{})["spec"].(map[string]interface{})["mutatingWebhook"]
			}, defaultTimeoutSeconds, 1).Should(Equal("Disabled"))
		})
		It("Checking if Assign/AssingnMetadata CRDs have been removed", func() {
			Eventually(func() interface{} {
				out, _ := utils.KubectlWithOutput("get", "crd", "assign.mutations.gatekeeper.sh", "--kubeconfig="+kubeconfigManaged)
				return out
			}, defaultTimeoutSeconds*4, 1).Should(ContainSubstring("not found"))
			Eventually(func() interface{} {
				out, _ := utils.KubectlWithOutput("get", "crd", "assignmetadata.mutations.gatekeeper.sh", "--kubeconfig="+kubeconfigManaged)
				return out
			}, defaultTimeoutSeconds*4, 1).Should(ContainSubstring("not found"))
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
		It("Checking if gatekeeper controller manager has mutation flag off", func() {
			Eventually(func() interface{} {
				podList, _ := clientManaged.CoreV1().Pods("openshift-gatekeeper-system").List(context.TODO(), metav1.ListOptions{LabelSelector: "control-plane=controller-manager"})
				return fmt.Sprintf("%d/%d", len(podList.Items[0].Spec.Containers[0].Args), len(podList.Items[1].Spec.Containers[0].Args))
			}, defaultTimeoutSeconds*15, 1).Should(Equal("6/6"))
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
		It("Clean up mutation policies", func() {
			utils.KubectlWithOutput("delete", "-f", GKAssignMetadataPolicyYaml, "-n", userNamespace, "--kubeconfig="+kubeconfigHub)
			Eventually(func() interface{} {
				managedPlc := utils.GetWithTimeout(clientManagedDynamic, common.GvrPolicy, userNamespace+"."+GKAssignMetadataPolicyName, clusterNamespace, false, defaultTimeoutSeconds)
				return managedPlc
			}, defaultTimeoutSeconds, 1).Should(BeNil())
			utils.KubectlWithOutput("delete", "-f", GKAssignPolicyYaml, "-n", userNamespace, "--kubeconfig="+kubeconfigHub)
			Eventually(func() interface{} {
				managedPlc := utils.GetWithTimeout(clientManagedDynamic, common.GvrPolicy, userNamespace+"."+GKAssignPolicyName, clusterNamespace, false, defaultTimeoutSeconds)
				return managedPlc
			}, defaultTimeoutSeconds, 1).Should(BeNil())
			utils.KubectlWithOutput("delete", "-f", "../resources/gatekeeper/pod-mutation.yaml", "-n", "default", "--kubeconfig="+kubeconfigManaged)

		})
		It("Clean up stable/policy-gatekeeper-sample", func() {
			utils.KubectlWithOutput("delete", "-f", GKPolicyYaml, "-n", userNamespace, "--kubeconfig="+kubeconfigHub)
			Eventually(func() interface{} {
				managedPlc := utils.GetWithTimeout(clientManagedDynamic, common.GvrPolicy, userNamespace+"."+GKPolicyName, clusterNamespace, false, defaultTimeoutSeconds)
				return managedPlc
			}, defaultTimeoutSeconds, 1).Should(BeNil())
			utils.KubectlWithOutput("delete", "ns", "e2etestsuccess", "--kubeconfig="+kubeconfigManaged)
			utils.KubectlWithOutput("delete", "ns", "e2etestfail", "--kubeconfig="+kubeconfigManaged)
		})
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
			utils.KubectlWithOutput("delete", "events", "-n", clusterNamespace, "--field-selector=involvedObject.name="+userNamespace+".policy-gatekeeper", "--kubeconfig="+kubeconfigManaged)
			utils.KubectlWithOutput("delete", "events", "-n", clusterNamespace, "--field-selector=involvedObject.name="+userNamespace+".policy-gatekeeper-image-pull-policy", "--kubeconfig="+kubeconfigManaged)
			utils.KubectlWithOutput("delete", "events", "-n", clusterNamespace, "--field-selector=involvedObject.name="+userNamespace+".policy-gatekeeper-annotation-owner", "--kubeconfig="+kubeconfigManaged)
		})
	})
})
