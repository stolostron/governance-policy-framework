// Copyright (c) 2020 Red Hat, Inc.
// Copyright Contributors to the Open Cluster Management project

package e2e

import (
	"context"
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"open-cluster-management.io/governance-policy-propagator/test/utils"

	"github.com/stolostron/governance-policy-framework/test/common"
)

// GetClusterLevelWithTimeout keeps polling to get the object for timeout seconds until wantFound is met (true for found, false for not found)
func GetClusterLevelWithTimeout(
	clientHubDynamic dynamic.Interface,
	gvr schema.GroupVersionResource,
	name string,
	wantFound bool,
	timeout int,
) *unstructured.Unstructured {
	if timeout < 1 {
		timeout = 1
	}
	var obj *unstructured.Unstructured

	Eventually(func() error {
		var err error
		namespace := clientHubDynamic.Resource(gvr)
		obj, err = namespace.Get(context.TODO(), name, metav1.GetOptions{})
		if wantFound && err != nil {
			return err
		}
		if !wantFound && err == nil {
			return fmt.Errorf("expected to return IsNotFound error")
		}
		if !wantFound && err != nil && !errors.IsNotFound(err) {
			return err
		}
		return nil
	}, timeout, 1).Should(BeNil())
	if wantFound {
		return obj
	}
	return nil
}

const GKOPolicyYaml string = "../resources/gatekeeper/policy-gatekeeper-operator.yaml"

var _ = Describe("Test gatekeeper", Ordered, func() {
	const gatekeeperNS = "gatekeeper-system"

	Describe("Test gatekeeper operator", func() {
		const GKOPolicyName string = "policy-gatekeeper-operator"
		It("gatekeeper operator policy should be created on managed", func() {
			common.DoCreatePolicyTest(clientHubDynamic, clientManagedDynamic, GKOPolicyYaml)
		})
		It("should create gatekeeper pods on managed cluster", func() {
			By("Checking number of pods in gatekeeper-system ns")
			utils.ListWithTimeoutByNamespace(clientManagedDynamic, common.GvrPod, metav1.ListOptions{}, gatekeeperNS, 6, true, 240)
		})

		AfterAll(func() {
			if CurrentSpecReport().Failed() {
				utils.KubectlWithOutput("-n", gatekeeperNS, "get", "pods", "--kubeconfig="+kubeconfigManaged)
				utils.KubectlWithOutput("-n", gatekeeperNS, "logs", "deployment/gatekeeper-operator-controller", "-c", "manager")
				common.OutputDebugInfo("gatekeeper operator", kubeconfigHub)
			}
		})
	})
	Describe("Test gatekeeper policy creation", Ordered, func() {
		const GKPolicyName string = "policy-gatekeeper"
		const GKPolicyYaml string = "../resources/gatekeeper/policy-gatekeeper.yaml"
		const cfgpolKRLName string = "policy-gatekeeper-k8srequiredlabels"
		const cfgpolauditName string = "policy-gatekeeper-audit"
		const cfgpoladmissionName string = "policy-gatekeeper-admission"
		It("should deploy gatekeeper release on managed cluster", func() {
			configCRD := GetClusterLevelWithTimeout(clientManagedDynamic, common.GvrCRD, "configs.config.gatekeeper.sh", true, defaultTimeoutSeconds)
			Expect(configCRD).NotTo(BeNil())
			cpsCRD := GetClusterLevelWithTimeout(clientManagedDynamic, common.GvrCRD, "constraintpodstatuses.status.gatekeeper.sh", true, defaultTimeoutSeconds)
			Expect(cpsCRD).NotTo(BeNil())
			ctpsCRD := GetClusterLevelWithTimeout(clientManagedDynamic, common.GvrCRD, "constrainttemplatepodstatuses.status.gatekeeper.sh", true, defaultTimeoutSeconds)
			Expect(ctpsCRD).NotTo(BeNil())
			ctCRD := GetClusterLevelWithTimeout(clientManagedDynamic, common.GvrCRD, "constrainttemplates.templates.gatekeeper.sh", true, defaultTimeoutSeconds)
			Expect(ctCRD).NotTo(BeNil())
		})
		It("configurationPolicies should be created on managed", func() {
			common.DoCreatePolicyTest(clientHubDynamic, clientManagedDynamic, GKPolicyYaml)

			By("Checking configpolicies on managed")
			krl := utils.GetWithTimeout(clientManagedDynamic, common.GvrConfigurationPolicy, cfgpolKRLName, clusterNamespace, true, defaultTimeoutSeconds)
			Expect(krl).NotTo(BeNil())
			audit := utils.GetWithTimeout(clientManagedDynamic, common.GvrConfigurationPolicy, cfgpolauditName, clusterNamespace, true, defaultTimeoutSeconds)
			Expect(audit).NotTo(BeNil())
			admission := utils.GetWithTimeout(clientManagedDynamic, common.GvrConfigurationPolicy, cfgpoladmissionName, clusterNamespace, true, defaultTimeoutSeconds)
			Expect(admission).NotTo(BeNil())
		})
		It("K8sRequiredLabels ns-must-have-gk should be created on managed", func() {
			By("Checking if K8sRequiredLabels CRD exists")
			k8srequiredlabelsCRD := GetClusterLevelWithTimeout(clientManagedDynamic, common.GvrCRD, "k8srequiredlabels.constraints.gatekeeper.sh", true, defaultTimeoutSeconds*2)
			Expect(k8srequiredlabelsCRD).NotTo(BeNil())
			By("Checking if ns-must-have-gk CR exists")
			nsMustHaveGkCR := GetClusterLevelWithTimeout(clientManagedDynamic, common.GvrK8sRequiredLabels, "ns-must-have-gk", true, defaultTimeoutSeconds*2)
			Expect(nsMustHaveGkCR).NotTo(BeNil())
		})
		It("K8sRequiredLabels ns-must-have-gk should be properly enforced for audit, no violation expected", func() {
			By("Checking if ns-must-have-gk status field has been updated")
			Eventually(func() interface{} {
				nsMustHaveGkCR := GetClusterLevelWithTimeout(clientManagedDynamic, common.GvrK8sRequiredLabels, "ns-must-have-gk", true, defaultTimeoutSeconds)
				return nsMustHaveGkCR.Object["status"]
			}, defaultTimeoutSeconds*2, 1).ShouldNot(BeNil())
			By("Checking if ns-must-have-gk status.totalViolations is equal to 0")
			Eventually(func() interface{} {
				nsMustHaveGkCR := GetClusterLevelWithTimeout(clientManagedDynamic, common.GvrK8sRequiredLabels, "ns-must-have-gk", true, defaultTimeoutSeconds)
				return nsMustHaveGkCR.Object["status"].(map[string]interface{})["totalViolations"]
			}, defaultTimeoutSeconds*4, 1).Should(Equal(int64(0)))
			By("Checking if ns-must-have-gk status.violations field has been updated")
			Eventually(func() interface{} {
				nsMustHaveGkCR := GetClusterLevelWithTimeout(clientManagedDynamic, common.GvrK8sRequiredLabels, "ns-must-have-gk", true, defaultTimeoutSeconds)
				fmt.Printf("%v\n", nsMustHaveGkCR.Object["status"].(map[string]interface{})["violations"])
				return nsMustHaveGkCR.Object["status"].(map[string]interface{})["violations"]
			}, defaultTimeoutSeconds*2, 1).Should(BeNil())
		})
		It("K8sRequiredLabels ns-must-have-gk should be properly enforced for admission", func() {
			By("Checking if ns-must-have-gk status.byPod field size is 3")
			Eventually(func() interface{} {
				nsMustHaveGkCR := GetClusterLevelWithTimeout(clientManagedDynamic, common.GvrK8sRequiredLabels, "ns-must-have-gk", true, defaultTimeoutSeconds)
				return len(nsMustHaveGkCR.Object["status"].(map[string]interface{})["byPod"].([]interface{}))
			}, defaultTimeoutSeconds*8, 1).Should(Equal(3))
		})
		It("should generate statuses properly on hub, no violation expected", func() {
			By("Checking if status for policy template policy-gatekeeper-k8srequiredlabels is compliant")
			Eventually(func(g Gomega) interface{} {
				plc := utils.GetWithTimeout(clientHubDynamic, common.GvrPolicy, userNamespace+"."+GKPolicyName, clusterNamespace, true, defaultTimeoutSeconds)
				status, ok := plc.Object["status"].(map[string]interface{})
				g.Expect(ok).To(BeTrue())
				details, ok := status["details"].([]interface{})
				g.Expect(ok).To(BeTrue())

				return details[0].(map[string]interface{})["compliant"]
			}, defaultTimeoutSeconds, 1).Should(Equal("Compliant"))
			By("Checking if violation message for policy template policy-gatekeeper-audit is compliant")
			Eventually(func() interface{} {
				plc := utils.GetWithTimeout(clientHubDynamic, common.GvrPolicy, userNamespace+"."+GKPolicyName, clusterNamespace, true, defaultTimeoutSeconds)
				details := plc.Object["status"].(map[string]interface{})["details"].([]interface{})
				return details[1].(map[string]interface{})["compliant"]
			}, defaultTimeoutSeconds, 1).Should(Equal("Compliant"))
			Eventually(func() interface{} {
				plc := utils.GetWithTimeout(clientHubDynamic, common.GvrPolicy, userNamespace+"."+GKPolicyName, clusterNamespace, true, defaultTimeoutSeconds)
				details := plc.Object["status"].(map[string]interface{})["details"].([]interface{})
				return details[1].(map[string]interface{})["history"]
			}, defaultTimeoutSeconds, 1).ShouldNot(BeNil())
			Eventually(func(g Gomega) interface{} {
				plc := utils.GetWithTimeout(clientHubDynamic, common.GvrPolicy, userNamespace+"."+GKPolicyName, clusterNamespace, true, defaultTimeoutSeconds)
				details := plc.Object["status"].(map[string]interface{})["details"].([]interface{})
				g.Expect(details[1].(map[string]interface{})["history"]).NotTo(BeNil())
				return details[1].(map[string]interface{})["history"].([]interface{})[0].(map[string]interface{})["message"]
			}, defaultTimeoutSeconds, 1).Should(Equal("Compliant; notification - k8srequiredlabels [ns-must-have-gk] found as specified, therefore this Object template is compliant"))
			By("Checking if violation message for policy template policy-gatekeeper-admission is compliant")
			Eventually(func() interface{} {
				plc := utils.GetWithTimeout(clientHubDynamic, common.GvrPolicy, userNamespace+"."+GKPolicyName, clusterNamespace, true, defaultTimeoutSeconds)
				details := plc.Object["status"].(map[string]interface{})["details"].([]interface{})
				return details[2].(map[string]interface{})["compliant"]
			}, defaultTimeoutSeconds, 1).Should(Equal("Compliant"))
			Eventually(func() interface{} {
				plc := utils.GetWithTimeout(clientHubDynamic, common.GvrPolicy, userNamespace+"."+GKPolicyName, clusterNamespace, true, defaultTimeoutSeconds)
				details := plc.Object["status"].(map[string]interface{})["details"].([]interface{})
				return details[2].(map[string]interface{})["history"]
			}, defaultTimeoutSeconds, 1).ShouldNot(BeNil())
			Eventually(func() interface{} {
				plc := utils.GetWithTimeout(clientHubDynamic, common.GvrPolicy, userNamespace+"."+GKPolicyName, clusterNamespace, true, defaultTimeoutSeconds)
				details := plc.Object["status"].(map[string]interface{})["details"].([]interface{})
				fmt.Printf("%v\n", details[2].(map[string]interface{})["history"].([]interface{})[0].(map[string]interface{})["message"])
				return details[2].(map[string]interface{})["history"].([]interface{})[0].(map[string]interface{})["message"]
			}, defaultTimeoutSeconds, 1).Should(Equal("Compliant; notification - events in namespace gatekeeper-system missing as expected, therefore this Object template is compliant"))
		})
		It("Creating a valid ns should not be blocked by gatekeeper", func() {
			By("Creating a namespace called e2etestsuccess on managed")
			out, _ := common.OcManaged("apply", "-f", "../resources/gatekeeper/ns-create-valid.yaml")
			Expect(out).Should(ContainSubstring("namespace/e2etestsuccess created"))
		})
		It("Creating an invalid ns should generate a violation message", func() {
			By("Creating invalid namespace on managed")
			Eventually(func() interface{} {
				_, out := common.OcManaged("create", "ns", "e2etestfail")
				return out.Error()
			}, defaultTimeoutSeconds, 1).Should(And(
				ContainSubstring("validation.gatekeeper.sh"),
				ContainSubstring("denied"),
				ContainSubstring("ns-must-have-gk")))
			By("Checking if status for policy template policy-gatekeeper-admission is noncompliant")
			Eventually(func() interface{} {
				plc := utils.GetWithTimeout(clientHubDynamic, common.GvrPolicy, userNamespace+"."+GKPolicyName, clusterNamespace, true, defaultTimeoutSeconds)
				details := plc.Object["status"].(map[string]interface{})["details"].([]interface{})
				return details[2].(map[string]interface{})["compliant"]
			}, defaultTimeoutSeconds, 1).Should(Equal("NonCompliant"))
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
		It("should create relatedObjects properly on managed", func() {
			By("Checking configurationpolicies on managed")
			Eventually(func() interface{} {
				plc := utils.GetWithTimeout(clientManagedDynamic, common.GvrConfigurationPolicy, cfgpolauditName, clusterNamespace, true, defaultTimeoutSeconds)
				ro := plc.Object["status"].(map[string]interface{})["relatedObjects"].([]interface{})
				return ro[0].(map[string]interface{})["object"].(map[string]interface{})["metadata"].(map[string]interface{})["name"]
			}, defaultTimeoutSeconds, 1).Should(Equal("ns-must-have-gk"))
		})
		AfterAll(func() {
			common.DoCleanupPolicy(clientHubDynamic, clientManagedDynamic, GKOPolicyYaml)
			common.DoCleanupPolicy(clientHubDynamic, clientManagedDynamic, GKPolicyYaml)

			By("Deleting gatekeeper ConstraintTemplate and K8sRequiredLabels")
			common.OcManaged("delete", "K8sRequiredLabels", "--all")
			common.OcManaged("delete", "crd", "k8srequiredlabels.constraints.gatekeeper.sh")
			By("Deleting all events in gatekeeper-system")
			common.OcManaged("delete", "events", "--all", "-n", gatekeeperNS)
			By("Deleting ns e2etestsuccess")
			common.OcManaged("delete", "ns", "e2etestsuccess")
		})
	})
})
