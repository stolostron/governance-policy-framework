// Copyright (c) 2020 Red Hat, Inc.

package e2e

import (
	"fmt"

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
		It("should properly enforce gatekeeper policy", func() {
			By("Checking for violations in k8srequiredlabels")
			Eventually(func() interface{} {
				plc := utils.GetWithTimeout(clientManagedDynamic, gvrConfigurationPolicy, cfgpolauditName, clusterNamespace, true, defaultTimeoutSeconds)
				if plc.Object["status"] != nil {
					return plc.Object["status"].(map[string]interface{})["compliant"]
				}
				return ""
			}, defaultTimeoutSeconds, 1).Should(Equal("NonCompliant"))
			By("Checking for violations in events")
			Eventually(func() interface{} {
				plc := utils.GetWithTimeout(clientManagedDynamic, gvrConfigurationPolicy, cfgpoladmissionName, clusterNamespace, true, defaultTimeoutSeconds)
				if plc.Object["status"] != nil {
					return plc.Object["status"].(map[string]interface{})["compliant"]
				}
				return ""
			}, defaultTimeoutSeconds, 1).Should(Equal("Compliant"))
			By("Creating invalid namespace on managed")
			utils.Kubectl("create", "ns", "e2etestfail", "--kubeconfig=../../kubeconfig_managed")
			Consistently(func() interface{} {
				return GetClusterLevelWithTimeout(clientManagedDynamic, gvrNS, "e2etestfail", false, defaultTimeoutSeconds)
			}, defaultTimeoutSeconds, 1).Should(BeNil())
			By("Checking for violations in events")
			Eventually(func() interface{} {
				plc := utils.GetWithTimeout(clientManagedDynamic, gvrConfigurationPolicy, cfgpoladmissionName, clusterNamespace, true, defaultTimeoutSeconds)
				if plc.Object["status"] != nil {
					return plc.Object["status"].(map[string]interface{})["compliant"]
				}
				return ""
			}, defaultTimeoutSeconds, 1).Should(Equal("NonCompliant"))
		})
	})
})
