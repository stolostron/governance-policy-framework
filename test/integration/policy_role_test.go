// Copyright (c) 2022 Red Hat, Inc.
// Copyright Contributors to the Open Cluster Management project

package integration

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	policiesv1 "github.com/stolostron/governance-policy-propagator/api/v1"
	"github.com/stolostron/governance-policy-propagator/test/utils"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8stypes "k8s.io/apimachinery/pkg/types"

	"github.com/stolostron/governance-policy-framework/test/common"
)

var _ = Describe("GRC: [P1][Sev1][policy-grc] Test the policy-role policy", Ordered, Label("policy-collection", "stable"), func() {
	const (
		policyRoleName   = "policy-role"
		policyRoleURL    = policyCollectACURL + policyRoleName + ".yaml"
		policyRoleNSName = policyRoleName + "ns"
		roleName         = "sample-role"
	)
	It("stable/"+policyRoleName+" should be created on the Hub", func() {
		By("Creating policy on hub")
		_, err := utils.KubectlWithOutput(
			"apply", "-f", policyRoleURL, "-n", userNamespace, "--kubeconfig="+kubeconfigHub,
		)
		Expect(err).To(BeNil())

		By("Creating the " + policyRoleNSName + " namespace on the managed cluster")
		namespace := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: policyRoleNSName, Labels: map[string]string{"e2e": "true"}}}
		_, err = clientManaged.CoreV1().Namespaces().Create(context.TODO(), namespace, metav1.CreateOptions{})
		Expect(err).To(BeNil())

		By("Patching the namespaceSelector to use the " + policyRoleNSName + " namespace")
		_, err = clientHubDynamic.Resource(common.GvrPolicy).Namespace(userNamespace).Patch(
			context.TODO(),
			policyRoleName,
			k8stypes.JSONPatchType,
			[]byte(`[{"op": "replace", "path": "/spec/policy-templates/0/objectDefinition/spec/namespaceSelector/include", "value": ["`+policyRoleNSName+`"]}]`),
			metav1.PatchOptions{},
		)
		Expect(err).To(BeNil())

		By("Patching placement rule")
		err = common.PatchPlacementRule(
			userNamespace, "placement-"+policyRoleName, clusterNamespace, kubeconfigHub,
		)
		Expect(err).To(BeNil())

		By("Checking that " + policyRoleName + " exists on the Hub cluster")
		rootPolicy := utils.GetWithTimeout(
			clientHubDynamic, common.GvrPolicy, policyRoleName, userNamespace, true, defaultTimeoutSeconds,
		)
		Expect(rootPolicy).NotTo(BeNil())
	})

	It("stable/"+policyRoleName+" should be created on managed cluster", func() {
		By("Checking the policy on managed cluster in ns " + clusterNamespace)
		managedPolicy := utils.GetWithTimeout(
			clientManagedDynamic,
			common.GvrPolicy,
			userNamespace+"."+policyRoleName,
			clusterNamespace,
			true,
			defaultTimeoutSeconds,
		)
		Expect(managedPolicy).NotTo(BeNil())
	})

	It("stable/"+policyRoleName+" should be NonCompliant", func() {
		By("Checking if the status of the root policy is NonCompliant")
		Eventually(
			common.GetComplianceState(clientHubDynamic, userNamespace, policyRoleName, clusterNamespace),
			defaultTimeoutSeconds*2,
			1,
		).Should(Equal(policiesv1.NonCompliant))
	})

	It("Enforcing stable/"+policyRoleName, func() {
		By("Patching remediationAction = enforce on the root policy")
		_, err := clientHubDynamic.Resource(common.GvrPolicy).Namespace(userNamespace).Patch(
			context.TODO(),
			policyRoleName,
			k8stypes.JSONPatchType,
			[]byte(`[{"op": "replace", "path": "/spec/remediationAction", "value": "enforce"}]`),
			metav1.PatchOptions{},
		)
		Expect(err).To(BeNil())
	})

	It("stable/"+policyRoleName+" should be Compliant", func() {
		By("Checking if the status of the root policy is Compliant")
		Eventually(
			common.GetComplianceState(clientHubDynamic, userNamespace, policyRoleName, clusterNamespace),
			defaultTimeoutSeconds*2,
			1,
		).Should(Equal(policiesv1.Compliant))
	})

	It("The Role should exist", func() {
		By("Checking the Role in the " + policyRoleNSName + " namespace")
		Eventually(
			func() error {
				_, err := clientManaged.RbacV1().Roles(policyRoleNSName).Get(
					context.TODO(), roleName, metav1.GetOptions{},
				)

				return err
			},
			defaultTimeoutSeconds*2,
			1,
		).Should(BeNil())
	})

	AfterAll(func() {
		_, err := utils.KubectlWithOutput(
			"delete", "-f", policyRoleURL, "-n", userNamespace, "--kubeconfig="+kubeconfigHub,
		)
		Expect(err).To(BeNil())

		err = clientManaged.CoreV1().Namespaces().Delete(context.TODO(), policyRoleNSName, metav1.DeleteOptions{})
		Expect(err).To(BeNil())
	})
})
