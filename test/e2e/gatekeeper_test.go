// Copyright (c) 2020 Red Hat, Inc.

package e2e

import (
	"fmt"
	"strings"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/open-cluster-management/governance-policy-propagator/test/utils"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
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
		obj, err = namespace.Get(name, metav1.GetOptions{})
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

func checkForViolationMessage(history []interface{}, message string) (f bool) {
	return strings.HasPrefix(history[0].(map[string]interface{})["message"].(string), message)
}

var _ = Describe("Test gatekeeper", func() {
	Describe("Test gatekeeper policy creation", func() {
		const GKPolicyName string = "policy-gatekeeper"
		const GKPolicyYaml string = "../resources/gatekeeper/policy-gatekeeper.yaml"
		const cfgpolKRLName string = "policy-gatekeeper-k8srequiredlabels"
		const cfgpolauditName string = "policy-gatekeeper-audit"
		const cfgpoladmissionName string = "policy-gatekeeper-admission"
		const NSYamlFail string = "../resources/gatekeeper/ns-create-invalid.yaml"
		It("should deploy gatekeeper release on managed cluster", func() {
			configCRD := GetClusterLevelWithTimeout(clientManagedDynamic, gvrCRD, "configs.config.gatekeeper.sh", true, defaultTimeoutSeconds)
			Expect(configCRD).NotTo(BeNil())
			cpsCRD := GetClusterLevelWithTimeout(clientManagedDynamic, gvrCRD, "constraintpodstatuses.status.gatekeeper.sh", true, defaultTimeoutSeconds)
			Expect(cpsCRD).NotTo(BeNil())
			ctpsCRD := GetClusterLevelWithTimeout(clientManagedDynamic, gvrCRD, "constrainttemplatepodstatuses.status.gatekeeper.sh", true, defaultTimeoutSeconds)
			Expect(ctpsCRD).NotTo(BeNil())
			ctCRD := GetClusterLevelWithTimeout(clientManagedDynamic, gvrCRD, "constrainttemplates.templates.gatekeeper.sh", true, defaultTimeoutSeconds)
			Expect(ctCRD).NotTo(BeNil())
		})
		It("configurationPolicies should be created on managed", func() {
			By("Creating policy on hub")
			utils.Kubectl("apply", "-f", GKPolicyYaml, "-n", "default", "--kubeconfig=../../kubeconfig_hub")
			hubPlc := utils.GetWithTimeout(clientHubDynamic, gvrPolicy, GKPolicyName, "default", true, defaultTimeoutSeconds)
			Expect(hubPlc).NotTo(BeNil())
			By("Patching " + GKPolicyName + " pr with decision of cluster managed")
			plr := utils.GetWithTimeout(clientHubDynamic, gvrPlacementRule, "placement-"+GKPolicyName, "default", true, defaultTimeoutSeconds)
			plr.Object["status"] = utils.GeneratePlrStatus("managed")
			plr, err := clientHubDynamic.Resource(gvrPlacementRule).Namespace("default").UpdateStatus(plr, metav1.UpdateOptions{})
			Expect(err).To(BeNil())
			By("Checking configpolicies on managed")
			krl := utils.GetWithTimeout(clientManagedDynamic, gvrConfigurationPolicy, cfgpolKRLName, clusterNamespace, true, 120)
			Expect(krl).NotTo(BeNil())
			audit := utils.GetWithTimeout(clientManagedDynamic, gvrConfigurationPolicy, cfgpolauditName, clusterNamespace, true, 120)
			Expect(audit).NotTo(BeNil())
			admission := utils.GetWithTimeout(clientManagedDynamic, gvrConfigurationPolicy, cfgpoladmissionName, clusterNamespace, true, 120)
			Expect(admission).NotTo(BeNil())
		})
		It("should generate statuses properly on hub", func() {
			By("Checking statuses on hub policy")
			Eventually(func() interface{} {
				plc := utils.GetWithTimeout(clientHubDynamic, gvrPolicy, "default."+GKPolicyName, clusterNamespace, true, defaultTimeoutSeconds)
				if plc.Object["status"] != nil {
					if plc.Object["status"].(map[string]interface{})["details"] != nil {
						details := plc.Object["status"].(map[string]interface{})["details"].([]interface{})
						if details[1].(map[string]interface{})["history"] != nil {
							return checkForViolationMessage(details[1].(map[string]interface{})["history"].([]interface{}),
								"NonCompliant; violation - k8srequiredlabels `ns-must-have-gk` does not exist as specified")
						}
					}
				}
				return false
			}, defaultTimeoutSeconds, 1).Should(Equal(true))
			By("Checking for violations in events")
			Eventually(func() interface{} {
				plc := utils.GetWithTimeout(clientHubDynamic, gvrPolicy, "default."+GKPolicyName, clusterNamespace, true, defaultTimeoutSeconds)
				if plc.Object["status"] != nil {
					if plc.Object["status"].(map[string]interface{})["details"] != nil {
						details := plc.Object["status"].(map[string]interface{})["details"].([]interface{})
						if details[2].(map[string]interface{})["history"] != nil {
							return checkForViolationMessage(details[2].(map[string]interface{})["history"].([]interface{}),
								"Compliant; notification - no instances of `events` exist as specified, therefore this Object template is compliant")
						}
					}
				}
				return false
			}, defaultTimeoutSeconds, 1).Should(Equal(true))
		})
		It("should properly enforce gatekeeper policy", func() {
			By("Creating invalid namespace on managed")
			utils.Pause(60)
			utils.Kubectl("create", "ns", "e2etestfail", "--kubeconfig=../../kubeconfig_managed")
			Consistently(func() interface{} {
				return GetClusterLevelWithTimeout(clientManagedDynamic, gvrNS, "e2etestfail", false, defaultTimeoutSeconds)
			}, defaultTimeoutSeconds, 1).Should(BeNil())
			By("Checking for violations in events")
			Eventually(func() interface{} {
				plc := utils.GetWithTimeout(clientHubDynamic, gvrPolicy, "default."+GKPolicyName, clusterNamespace, true, defaultTimeoutSeconds)
				if plc.Object["status"] != nil {
					details := plc.Object["status"].(map[string]interface{})["details"].([]interface{})
					return checkForViolationMessage(details[2].(map[string]interface{})["history"].([]interface{}), "NonCompliant; violation - events exist:")
				}
				return false
			}, defaultTimeoutSeconds, 1).Should(Equal(true))
		})
		It("should create relatedObjects properly on managed", func() {
			By("Checking configurationpolicies on managed")
			Eventually(func() interface{} {
				plc := utils.GetWithTimeout(clientManagedDynamic, gvrConfigurationPolicy, cfgpolauditName, clusterNamespace, true, 120)
				if plc.Object["status"] != nil {
					ro := plc.Object["status"].(map[string]interface{})["relatedObjects"].([]interface{})
					md := ro[0].(map[string]interface{})["object"].(map[string]interface{})["metadata"].(map[string]interface{})
					return md["name"]
				}
				return ""
			}, defaultTimeoutSeconds, 1).Should(Equal("ns-must-have-gk"))
		})
		It("should clean up", func() {
			By("Deleting gatekeeper policy on hub")
			utils.Kubectl("delete", "-f", GKPolicyYaml, "-n", "default", "--kubeconfig=../../kubeconfig_hub")
			By("Checking if there is any policy left")
			utils.ListWithTimeout(clientHubDynamic, gvrPolicy, metav1.ListOptions{}, 0, true, defaultTimeoutSeconds)
			utils.ListWithTimeout(clientManagedDynamic, gvrPolicy, metav1.ListOptions{}, 0, true, defaultTimeoutSeconds)
			By("Checking if there is any configuration policy left")
			utils.ListWithTimeout(clientManagedDynamic, gvrConfigurationPolicy, metav1.ListOptions{}, 0, true, defaultTimeoutSeconds)
			By("Deleting gatekeeper ConstraintTemplate and K8sRequiredLabels")
			utils.Kubectl("delete", "ConstraintTemplate", "--all", "--kubeconfig=../../kubeconfig_managed")
			utils.Kubectl("delete", "K8sRequiredLabels", "--all", "--kubeconfig=../../kubeconfig_managed")
		})
	})
})
