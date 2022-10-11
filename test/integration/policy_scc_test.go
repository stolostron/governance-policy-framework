// Copyright (c) 2022 Red Hat, Inc.
// Copyright Contributors to the Open Cluster Management project

package integration

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8stypes "k8s.io/apimachinery/pkg/types"
	policiesv1 "open-cluster-management.io/governance-policy-propagator/api/v1"
	"open-cluster-management.io/governance-policy-propagator/test/utils"

	"github.com/stolostron/governance-policy-framework/test/common"
)

var _ = Describe("GRC: [P1][Sev1][policy-grc] Test the policy-scc policy",
	Ordered, Label("policy-collection", "stable"), func() {
		const (
			rootPolicyName = "policy-securitycontextconstraints"
			rootPolicyURL  = policyCollectSCURL + "policy-scc.yaml"
			targetName     = "restricted"
			targetKind     = "scc"
		)

		targetGVR := common.GvrSCC

		It("stable/"+rootPolicyName+" should be created on the Hub", func() {
			By("Creating the policy on the Hub")
			_, err := utils.KubectlWithOutput(
				"apply", "-f", rootPolicyURL, "-n", userNamespace, "--kubeconfig="+kubeconfigHub,
			)
			Expect(err).To(BeNil())

			By("Patching placement rule")
			err = common.PatchPlacementRule(userNamespace, "placement-"+rootPolicyName)
			Expect(err).To(BeNil())

			By("Checking that " + rootPolicyName + " exists on the Hub cluster")
			rootPolicy := utils.GetWithTimeout(
				clientHubDynamic, common.GvrPolicy, rootPolicyName, userNamespace, true, defaultTimeoutSeconds,
			)
			Expect(rootPolicy).NotTo(BeNil())
		})

		It("stable/"+rootPolicyName+" should be created on managed cluster", func() {
			By("Checking the policy on managed cluster in ns " + clusterNamespace)
			managedPolicy := utils.GetWithTimeout(
				clientManagedDynamic,
				common.GvrPolicy,
				userNamespace+"."+rootPolicyName,
				clusterNamespace,
				true,
				defaultTimeoutSeconds,
			)
			Expect(managedPolicy).NotTo(BeNil())
		})

		// This is a special case because it specifies a manifest that by default is on an Openshift cluster
		It("stable/"+rootPolicyName+" should be Compliant", func() {
			By("Checking if the status of the root policy is Compliant")
			Eventually(
				common.GetComplianceState(rootPolicyName),
				defaultTimeoutSeconds*2,
				1,
			).Should(Equal(policiesv1.Compliant))
		})

		It("Enforcing stable/"+rootPolicyName, func() {
			By("Patching remediationAction = enforce on the root policy")
			_, err := clientHubDynamic.Resource(common.GvrPolicy).Namespace(userNamespace).Patch(
				context.TODO(),
				rootPolicyName,
				k8stypes.JSONPatchType,
				[]byte(`[{"op": "replace", "path": "/spec/remediationAction", "value": "enforce"}]`),
				metav1.PatchOptions{},
			)
			Expect(err).To(BeNil())
		})

		It("stable/"+rootPolicyName+" should be Compliant", func() {
			By("Checking if the status of the root policy is Compliant")
			Eventually(
				common.GetComplianceState(rootPolicyName),
				defaultTimeoutSeconds*2,
				1,
			).Should(Equal(policiesv1.Compliant))
		})

		It("The "+targetKind+" should exist", func() {
			By("Checking the " + targetKind)
			Eventually(
				func() error {
					_, err := clientManagedDynamic.Resource(targetGVR).Get(
						context.TODO(), targetName, metav1.GetOptions{},
					)

					return err
				},
				defaultTimeoutSeconds*2,
				1,
			).Should(BeNil())
		})

		AfterAll(func() {
			_, err := utils.KubectlWithOutput(
				"delete", "-f", rootPolicyURL, "-n",
				userNamespace, "--kubeconfig="+kubeconfigHub,
				"--ignore-not-found",
			)
			Expect(err).To(BeNil())
		})
	})
