// Copyright (c) 2022 Red Hat, Inc.
// Copyright Contributors to the Open Cluster Management project

package integration

import (
	"context"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8stypes "k8s.io/apimachinery/pkg/types"
	policiesv1 "open-cluster-management.io/governance-policy-propagator/api/v1"
	"open-cluster-management.io/governance-policy-propagator/test/utils"

	"github.com/stolostron/governance-policy-framework/test/common"
)

var _ = Describe("GRC: [P1][Sev1][policy-grc] Test the zts-cmc policy", Ordered, Label("policy-collection", "stable"), func() {
	const (
		policyName     = "policy-zts-cmc"
		policyURL      = policyCollectCMURL + policyName + ".yaml"
		deploymentNS   = "default"
		deploymentName = "zts-cmc-app-deploy"
	)

	It("stable/"+policyName+" should be created on the Hub", func() {
		By("Creating the policy on the Hub")
		_, err := utils.KubectlWithOutput(
			"apply", "-f", policyURL, "-n", userNamespace, "--kubeconfig="+kubeconfigHub,
		)
		Expect(err).To(BeNil())

		By("Patching the namespaceSelector to use the " + deploymentNS + " namespace")
		_, err = clientHubDynamic.Resource(common.GvrPolicy).Namespace(userNamespace).Patch(
			context.TODO(),
			policyName,
			k8stypes.JSONPatchType,
			[]byte(`[{"op": "replace", "path": "/spec/policy-templates/0/objectDefinition/spec/namespaceSelector/include", "value": ["`+deploymentNS+`"]}]`),
			metav1.PatchOptions{},
		)
		Expect(err).To(BeNil())

		By("Patching placement rule")
		err = common.PatchPlacementRule(
			userNamespace, "placement-"+policyName, clusterNamespace, kubeconfigHub,
		)
		Expect(err).To(BeNil())

		By("Checking that " + policyName + " exists on the Hub cluster")
		rootPolicy := utils.GetWithTimeout(
			clientHubDynamic, common.GvrPolicy, policyName, userNamespace, true, defaultTimeoutSeconds,
		)
		Expect(rootPolicy).NotTo(BeNil())
	})

	It("stable/"+policyName+" should be created on managed cluster", func() {
		By("Checking the policy on managed cluster in ns " + clusterNamespace)
		managedPolicy := utils.GetWithTimeout(
			clientManagedDynamic,
			common.GvrPolicy,
			userNamespace+"."+policyName,
			clusterNamespace,
			true,
			defaultTimeoutSeconds,
		)
		Expect(managedPolicy).NotTo(BeNil())
	})

	It("stable/"+policyName+" should be NonCompliant", func() {
		By("Checking if the status of the root policy is NonCompliant")
		Eventually(
			common.GetComplianceState(clientHubDynamic, userNamespace, policyName, clusterNamespace),
			defaultTimeoutSeconds*2,
			1,
		).Should(Equal(policiesv1.NonCompliant))
	})

	It("Enforcing stable/"+policyName, func() {
		By("Patching remediationAction = enforce on the root policy")
		_, err := clientHubDynamic.Resource(common.GvrPolicy).Namespace(userNamespace).Patch(
			context.TODO(),
			policyName,
			k8stypes.JSONPatchType,
			[]byte(`[{"op": "replace", "path": "/spec/remediationAction", "value": "enforce"}]`),
			metav1.PatchOptions{},
		)
		Expect(err).To(BeNil())
	})

	It("the "+deploymentName+" deployment should exist in namespace "+deploymentNS, func() {
		Eventually(func() string {
			output, _ := common.OcManaged("get", "deployment.apps", "-n",
				deploymentNS, deploymentName, "-o", "name")

			return strings.TrimSpace(output)
		},
			defaultTimeoutSeconds*2,
			1,
		).Should(Equal("deployment.apps/" + deploymentName))
	})

	It("stable/"+policyName+" should be Compliant", func() {
		By("Checking if the status of the root policy is Compliant")
		Eventually(
			common.GetComplianceState(clientHubDynamic, userNamespace, policyName, clusterNamespace),
			defaultTimeoutSeconds*2,
			1,
		).Should(Equal(policiesv1.Compliant))
	})

	AfterAll(func() {
		out, _ := utils.KubectlWithOutput(
			"get", "deployment", "-n", deploymentNS, deploymentName, "--kubeconfig="+kubeconfigHub,
		)
		Expect(out).To(ContainSubstring(deploymentName))

		_, err := utils.KubectlWithOutput(
			"delete", "deployment", "-n", deploymentNS, deploymentName, "--kubeconfig="+kubeconfigHub,
		)
		Expect(err).To(BeNil())

		_, err = utils.KubectlWithOutput(
			"delete", "-f", policyURL, "-n", userNamespace, "--kubeconfig="+kubeconfigHub,
		)
		Expect(err).To(BeNil())

	})

})
