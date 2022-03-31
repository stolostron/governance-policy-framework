// Copyright (c) 2022 Red Hat, Inc.
// Copyright Contributors to the Open Cluster Management project

package integration

import (
	"context"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	policiesv1 "github.com/stolostron/governance-policy-propagator/api/v1"
	"github.com/stolostron/governance-policy-propagator/test/utils"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8stypes "k8s.io/apimachinery/pkg/types"

	"github.com/stolostron/governance-policy-framework/test/common"
)

var _ = Describe("GRC: [P1][Sev1][policy-grc] Test the policy-imagemanifestvuln policy", Label("policy-collection", "stable", "BVT"), func() {
	const policyIMVURL = policyCollectSIURL + "policy-imagemanifestvuln.yaml"
	const policyIMVName = "policy-imagemanifestvuln"
	const subName = "container-security-operator"
	const operatorNS = "openshift-operators"

	It("stable/"+policyIMVName+" should be created on the Hub", func() {
		By("Creating the policy on the Hub")
		_, err := utils.KubectlWithOutput(
			"apply", "-f", policyIMVURL, "-n", userNamespace, "--kubeconfig="+kubeconfigHub,
		)
		Expect(err).To(BeNil())

		By("Patching placement rule")
		err = common.PatchPlacementRule(
			userNamespace, "placement-"+policyIMVName, clusterNamespace, kubeconfigHub,
		)
		Expect(err).To(BeNil())

		By("Checking that " + policyIMVName + " exists on the Hub cluster")
		rootPolicy := utils.GetWithTimeout(
			clientHubDynamic, common.GvrPolicy, policyIMVName, userNamespace, true, defaultTimeoutSeconds,
		)
		Expect(rootPolicy).NotTo(BeNil())
	})

	It("stable/"+policyIMVName+" should be created on managed cluster", func() {
		By("Checking the policy on managed cluster in ns " + clusterNamespace)
		managedPolicy := utils.GetWithTimeout(
			clientManagedDynamic,
			common.GvrPolicy,
			userNamespace+"."+policyIMVName,
			clusterNamespace,
			true,
			defaultTimeoutSeconds,
		)
		Expect(managedPolicy).NotTo(BeNil())
	})

	It("stable/"+policyIMVName+" should be NonCompliant", func() {
		By("Checking if the status of the root policy is NonCompliant")
		Eventually(
			common.GetComplianceState(clientHubDynamic, userNamespace, policyIMVName, clusterNamespace),
			defaultTimeoutSeconds*2,
			1,
		).Should(Equal(policiesv1.NonCompliant))
	})

	It("Enforcing stable/"+policyIMVName, func() {
		By("Patching remediationAction = enforce on the root policy")
		_, err := clientHubDynamic.Resource(common.GvrPolicy).Namespace(userNamespace).Patch(
			context.TODO(),
			policyIMVName,
			k8stypes.JSONPatchType,
			[]byte(`[{"op": "replace", "path": "/spec/remediationAction", "value": "enforce"}]`),
			metav1.PatchOptions{},
		)
		Expect(err).To(BeNil())
	})

	It("The subscription should exist", func() {
		By("Checking the subscription in the " + operatorNS + " namespace")
		Eventually(
			func() string {
				output, _ := common.OcManaged("get", "subscriptions.operators.coreos.com", "-n",
					operatorNS, subName, "-o", "name")

				return strings.TrimSpace(output)
			},
			defaultTimeoutSeconds*2,
			1,
		).Should(Equal("subscription.operators.coreos.com/container-security-operator"))
	})

	It("The operator should exist", func() {
		By("Checking the operator in the " + operatorNS + " namespace")
		Eventually(
			func() string {
				output, _ := common.OcManaged("get", "operators", subName+"."+operatorNS, "-o", "name")

				return strings.TrimSpace(output)
			},
			defaultTimeoutSeconds*2,
			1,
		).Should(Equal("operator.operators.coreos.com/container-security-operator.openshift-operators"))
	})

	It("stable/"+policyIMVName+" should detect imageManifestVulns", func() {
		By("Checking if the status of the subscription config policy is Compliant")
		Eventually(
			func() string {
				cfgPol := utils.GetWithTimeout(
					clientManagedDynamic,
					common.GvrConfigurationPolicy,
					"policy-imagemanifestvuln-example-sub",
					clusterNamespace,
					true,
					defaultTimeoutSeconds,
				).Object
				if status, ok := cfgPol["status"]; ok {
					if compliance, ok := status.(map[string]interface{})["compliant"]; ok {
						return compliance.(string)
					}
				}
				return ""
			},
			defaultTimeoutSeconds*2,
			1,
		).Should(Equal(string(policiesv1.Compliant)))

		By("Checking if the status of the CSV config policy is Compliant")
		Eventually(
			func() string {
				cfgPol := utils.GetWithTimeout(
					clientManagedDynamic,
					common.GvrConfigurationPolicy,
					"policy-imagemanifestvuln-status",
					clusterNamespace,
					true,
					defaultTimeoutSeconds,
				).Object
				if status, ok := cfgPol["status"]; ok {
					if compliance, ok := status.(map[string]interface{})["compliant"]; ok {
						return compliance.(string)
					}
				}
				return ""
			},
			defaultTimeoutSeconds*2,
			1,
		).Should(Equal(string(policiesv1.Compliant)))

		By("Checking if the status of the IMV config policy is NonCompliant")
		Eventually(
			func() string {
				cfgPol := utils.GetWithTimeout(
					clientManagedDynamic,
					common.GvrConfigurationPolicy,
					"policy-imagemanifestvuln-example-imv",
					clusterNamespace,
					true,
					defaultTimeoutSeconds,
				).Object
				if status, ok := cfgPol["status"]; ok {
					if compliance, ok := status.(map[string]interface{})["compliant"]; ok {
						return compliance.(string)
					}
				}
				return ""
			},
			defaultTimeoutSeconds*2,
			1,
		).Should(Equal(string(policiesv1.NonCompliant)))
	})

	It("stable/"+policyIMVName+" should be NonCompliant", func() {
		By("Checking if the status of the root policy is NonCompliant")
		Eventually(
			common.GetComplianceState(clientHubDynamic, userNamespace, policyIMVName, clusterNamespace),
			defaultTimeoutSeconds*2,
			1,
		).Should(Equal(policiesv1.NonCompliant))
	})

	It("Cleans up", func() {
		_, err := utils.KubectlWithOutput(
			"delete", "-f", policyIMVURL, "-n", userNamespace, "--kubeconfig="+kubeconfigHub,
		)
		Expect(err).To(BeNil())

		_, err = utils.KubectlWithOutput(
			"delete", "subscriptions.operators.coreos.com", subName, "-n", "openshift-operators",
			"--kubeconfig="+kubeconfigManaged,
		)
		Expect(err).To(BeNil())

		csvName, err := utils.KubectlWithOutput(
			"get", "clusterserviceversions",
			"-n", operatorNS,
			"-o",
			"jsonpath={.items[?(@.spec.displayName==\"Quay Container Security\")].metadata.name}",
			"--kubeconfig="+kubeconfigManaged,
		)
		Expect(err).To(BeNil())
		_, err = utils.KubectlWithOutput(
			"delete", "csv", csvName, "-n", operatorNS, "--kubeconfig="+kubeconfigManaged,
		)
		Expect(err).To(BeNil())

		_, err = utils.KubectlWithOutput(
			"delete", "crd", "imagemanifestvulns.secscan.quay.redhat.com", "--kubeconfig="+kubeconfigManaged,
		)
		Expect(err).To(BeNil())
	})
})
