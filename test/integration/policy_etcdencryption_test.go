// Copyright (c) 2022 Red Hat, Inc.
// Copyright Contributors to the Open Cluster Management project

package integration

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	k8stypes "k8s.io/apimachinery/pkg/types"
	policiesv1 "open-cluster-management.io/governance-policy-propagator/api/v1"
	"open-cluster-management.io/governance-policy-propagator/test/utils"

	"github.com/stolostron/governance-policy-framework/test/common"
)

var _ = Describe(
	"GRC: [P1][Sev1][policy-grc] Test the policy-etcdencryption policy",
	Ordered,
	Label("policy-collection", "stable", "etcd"),
	func() {
		const (
			policyEtcdEncryptionName = "policy-etcdencryption"
			policyEtcdEncryptionURL  = policyCollectSCURL + policyEtcdEncryptionName + ".yaml"
			apiSeverName             = "cluster"
			configPolicyName         = "enable-etcd-encryption"
		)

		It("stable/"+policyEtcdEncryptionName+" should be created on the Hub", func() {
			By("Creating the policy on the Hub")
			_, err := utils.KubectlWithOutput(
				"apply", "-f", policyEtcdEncryptionURL, "-n", userNamespace, "--kubeconfig="+kubeconfigHub,
			)
			Expect(err).To(BeNil())

			By("Patching placement rule")
			err = common.PatchPlacementRule(
				userNamespace, "placement-"+policyEtcdEncryptionName, clusterNamespace, kubeconfigHub,
			)
			Expect(err).To(BeNil())

			By("Checking that " + policyEtcdEncryptionName + " exists on the Hub cluster")
			rootPolicy := utils.GetWithTimeout(
				clientHubDynamic,
				common.GvrPolicy,
				policyEtcdEncryptionName,
				userNamespace,
				true,
				defaultTimeoutSeconds,
			)
			Expect(rootPolicy).NotTo(BeNil())
		})

		It("stable/"+policyEtcdEncryptionName+" should be created on managed cluster", func() {
			By("Checking the policy on the managed cluster in the namespace " + clusterNamespace)
			managedPolicy := utils.GetWithTimeout(
				clientManagedDynamic,
				common.GvrPolicy,
				userNamespace+"."+policyEtcdEncryptionName,
				clusterNamespace,
				true,
				defaultTimeoutSeconds,
			)
			Expect(managedPolicy).NotTo(BeNil())
		})

		It("stable/"+policyEtcdEncryptionName+" should be NonCompliant", func() {
			By("Checking if the status of the root policy is NonCompliant")
			Eventually(
				common.GetComplianceState(clientHubDynamic, userNamespace, policyEtcdEncryptionName, clusterNamespace),
				defaultTimeoutSeconds*2,
				1,
			).Should(Equal(policiesv1.NonCompliant))
		})

		It("Enforcing stable/"+policyEtcdEncryptionName, func() {
			By("Patching remediationAction = enforce on the first policy-template")
			// The second policy-templates entry is to inform on the encryption status
			_, err := clientHubDynamic.Resource(common.GvrPolicy).Namespace(userNamespace).Patch(
				context.TODO(),
				policyEtcdEncryptionName,
				k8stypes.JSONPatchType,
				[]byte(
					`[{"op": "remove", "path": "/spec/remediationAction"},`+
						`{"op": "replace", "path": "/spec/policy-templates/0`+
						`/objectDefinition/spec/remediationAction", "value": "enforce"}]`,
				),
				metav1.PatchOptions{},
			)
			Expect(err).To(BeNil())
		})

		It("Etcd encryption should be enabled", func() {
			// Only check the first ConfigurationPolicy status
			// since the second one is to inform on the encryption process
			// completing, which takes too long for the test to wait on.
			By("Checking if the status of the configuration policy " + configPolicyName + " is Compliant")
			Eventually(
				func() string {
					policy, err := clientManagedDynamic.Resource(
						common.GvrConfigurationPolicy,
					).Namespace(clusterNamespace).Get(
						context.TODO(),
						configPolicyName,
						metav1.GetOptions{},
					)
					if err != nil {
						return ""
					}

					compliant, _, _ := unstructured.NestedString(policy.Object, "status", "compliant")

					return compliant
				},
				defaultTimeoutSeconds*2,
				1,
			).Should(Equal(string(policiesv1.Compliant)))
		})

		It("The APIServer object should be configured for encryption", func() {
			By("Checking the APIServer object")
			Eventually(
				func() string {
					apiServer, err := clientManagedDynamic.Resource(common.GvrAPIServer).Get(
						context.TODO(), apiSeverName, metav1.GetOptions{},
					)
					if err != nil {
						return ""
					}

					encryptionType, _, _ := unstructured.NestedString(apiServer.Object, "spec", "encryption", "type")

					return encryptionType
				},
				defaultTimeoutSeconds*2,
				1,
			).Should(Equal("aescbc"))
		})

		AfterAll(func() {
			_, err := utils.KubectlWithOutput(
				"delete", "-f",
				policyEtcdEncryptionURL, "-n",
				userNamespace, "--kubeconfig="+kubeconfigHub,
				"--ignore-not-found",
			)
			Expect(err).To(BeNil())

			_, err = clientManagedDynamic.Resource(common.GvrAPIServer).Patch(
				context.TODO(),
				apiSeverName,
				k8stypes.JSONPatchType,
				[]byte(`[{"op": "remove", "path": "/spec/encryption"}]`),
				metav1.PatchOptions{},
			)
			Expect(err).To(BeNil())
		})
	})
