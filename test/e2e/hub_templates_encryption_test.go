// Copyright (c) 2022 Red Hat, Inc.
// Copyright Contributors to the Open Cluster Management project

package e2e

import (
	"bytes"
	"context"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"open-cluster-management.io/governance-policy-propagator/test/utils"

	"github.com/stolostron/governance-policy-framework/test/common"
)

var _ = Describe("Test Hub Template Encryption", func() {
	Describe("Test that a secret can be securely copied to managed clusters", func() {
		ctx := context.TODO()
		const policyName = "test-hub-encryption"
		const policyYAML = "../resources/hub_templates_encryption/policy.yaml"

		const secretName = "test-hub-encryption"
		const secretYAML = "../resources/hub_templates_encryption/secret.yaml"
		const secretCopyName = "test-hub-encryption-copy"

		const configMapName = "test-hub-encryption"
		const configMapYAML = "../resources/hub_templates_encryption/configmap.yaml"
		const configMapCopyName = "test-hub-encryption-copy"

		const lastRotatedAnnotation = "policy.open-cluster-management.io/last-rotated"
		const triggerUpdateAnnotation = "policy.open-cluster-management.io/trigger-update"

		It("Should be created on the managed cluster", func() {
			By("Creating the " + secretName + " Secret")
			_, err := utils.KubectlWithOutput(
				"apply", "-f", secretYAML, "-n", userNamespace, "--kubeconfig=../../kubeconfig_hub",
			)
			Expect(err).To(BeNil())

			By("Creating the " + configMapName + " ConfigMap")
			_, err = utils.KubectlWithOutput(
				"apply", "-f", configMapYAML, "-n", userNamespace, "--kubeconfig=../../kubeconfig_hub",
			)
			Expect(err).To(BeNil())

			By("Creating " + policyName)
			_, err = utils.KubectlWithOutput(
				"apply", "-f", policyYAML, "-n", userNamespace, "--kubeconfig=../../kubeconfig_hub",
			)
			Expect(err).To(BeNil())

			hubPlc := utils.GetWithTimeout(
				clientHubDynamic, common.GvrPolicy, policyName, userNamespace, true, defaultTimeoutSeconds,
			)
			Expect(hubPlc).NotTo(BeNil())

			By("Patching " + policyName + "-plr with the decision of the cluster managed")
			plr := utils.GetWithTimeout(
				clientHubDynamic,
				common.GvrPlacementRule,
				policyName+"-plr",
				userNamespace,
				true,
				defaultTimeoutSeconds,
			)
			plr.Object["status"] = utils.GeneratePlrStatus("managed")
			_, err = clientHubDynamic.Resource(common.GvrPlacementRule).Namespace(userNamespace).UpdateStatus(
				ctx, plr, metav1.UpdateOptions{},
			)
			Expect(err).To(BeNil())

			By("Checking " + policyName + " on the managed cluster in ns " + clusterNamespace)
			managedplc := utils.GetWithTimeout(
				clientManagedDynamic,
				common.GvrPolicy,
				userNamespace+"."+policyName,
				clusterNamespace,
				true,
				defaultTimeoutSeconds,
			)
			Expect(managedplc).NotTo(BeNil())
		})

		It("Should be compliant after enforcing it", func() {
			By("Patching remediationAction=enforce on the root policy")
			rootPlc := utils.GetWithTimeout(
				clientHubDynamic, common.GvrPolicy, policyName, userNamespace, true, defaultTimeoutSeconds,
			)
			rootPlc.Object["spec"].(map[string]interface{})["remediationAction"] = "enforce"
			_, err := clientHubDynamic.Resource(common.GvrPolicy).Namespace(userNamespace).Update(
				ctx, rootPlc, metav1.UpdateOptions{},
			)
			Expect(err).To(BeNil())

			By("Waiting for the policy to be compliant")
			Eventually(func() interface{} {
				plc := utils.GetWithTimeout(
					clientHubDynamic,
					common.GvrPolicy,
					userNamespace+"."+policyName,
					clusterNamespace,
					true,
					defaultTimeoutSeconds,
				)

				status, ok := plc.Object["status"].(map[string]interface{})
				if !ok {
					return ""
				}

				return status["compliant"]
			}, defaultTimeoutSeconds, 1).Should(Equal("Compliant"))
		})

		It("Should use encryption in the replicated policy", func() {
			By("Verifying the replicated policy")
			managedplc := utils.GetWithTimeout(
				clientManagedDynamic,
				common.GvrPolicy,
				userNamespace+"."+policyName,
				clusterNamespace,
				true,
				defaultTimeoutSeconds,
			)
			Expect(managedplc).NotTo(BeNil())

			plcTemplates, ok, err := unstructured.NestedSlice(managedplc.Object, "spec", "policy-templates")
			Expect(ok).To(BeTrue())
			Expect(err).To(BeNil())
			Expect(len(plcTemplates)).To(Equal(1))

			plcTemplate, ok := plcTemplates[0].(map[string]interface{})
			Expect(ok).To(BeTrue())

			objectTemplates, ok, err := unstructured.NestedSlice(
				plcTemplate, "objectDefinition", "spec", "object-templates",
			)
			Expect(ok).To(BeTrue())
			Expect(err).To(BeNil())
			Expect(len(objectTemplates)).To(Equal(2))

			secretTemplate, ok := objectTemplates[0].(map[string]interface{})
			Expect(ok).To(BeTrue())

			city, ok, err := unstructured.NestedString(secretTemplate, "objectDefinition", "data", "city")
			Expect(ok).To(BeTrue())
			Expect(err).To(BeNil())

			// Verify that the value is encrypted
			Expect(strings.Contains(city, "$ocm_encrypted:")).To(BeTrue())
			// Verify that the plaintext or the base64 of the plaintext is not included
			Expect(strings.Contains(city, "Raleigh")).ToNot(BeTrue())
			Expect(strings.Contains(city, "UmFsZWlnaA==")).ToNot(BeTrue())

			state, ok, err := unstructured.NestedString(secretTemplate, "objectDefinition", "data", "state")
			Expect(ok).To(BeTrue())
			Expect(err).To(BeNil())

			// Verify that the value is encrypted
			Expect(strings.Contains(state, "$ocm_encrypted:")).To(BeTrue())
			// Verify that the plaintext or the base64 of the plaintext is not included
			Expect(strings.Contains(state, "North Carolina")).ToNot(BeTrue())
			Expect(strings.Contains(state, "Tm9ydGggQ2Fyb2xpbmE=")).ToNot(BeTrue())

			configMapTemplate, ok := objectTemplates[1].(map[string]interface{})
			Expect(ok).To(BeTrue())

			cert, ok, err := unstructured.NestedString(configMapTemplate, "objectDefinition", "data", "cert")
			Expect(ok).To(BeTrue())
			Expect(err).To(BeNil())

			// Verify that the value is encrypted
			Expect(strings.Contains(cert, "$ocm_encrypted:")).To(BeTrue())
			// Verify that the plaintext is not included
			Expect(strings.Contains(state, "-----BEGIN CERTIFICATE-----")).ToNot(BeTrue())
		})

		It("Verifies that the objects are created by the policy", func() {
			By("Verifying the copied Secret")
			secret, err := clientManaged.CoreV1().Secrets("default").Get(
				ctx, secretCopyName, metav1.GetOptions{},
			)
			Expect(err).To(BeNil())
			Expect(string(secret.Data["city"])).To(Equal("Raleigh"))
			Expect(string(secret.Data["state"])).To(Equal("North Carolina"))

			By("Verifying the copied ConfigMap")
			configMap, err := clientManaged.CoreV1().ConfigMaps("default").Get(
				ctx, configMapCopyName, metav1.GetOptions{},
			)
			Expect(err).To(BeNil())

			// Verify that the certificate can be parsed
			block, _ := pem.Decode([]byte(configMap.Data["cert"]))
			Expect(block).ToNot(BeNil())

			_, err = x509.ParseCertificate(block.Bytes)
			Expect(err).To(BeNil())
		})

		It("Verifies that the key can be rotated", func() {
			encryptionSecret, err := clientHub.CoreV1().Secrets(clusterNamespace).Get(
				ctx, "policy-encryption-key", metav1.GetOptions{},
			)
			Expect(err).To(BeNil())
			originalKey := encryptionSecret.Data["key"]

			// Trigger a key rotation
			encryptionSecret.Annotations[lastRotatedAnnotation] = ""
			_, err = clientHub.CoreV1().Secrets(clusterNamespace).Update(ctx, encryptionSecret, metav1.UpdateOptions{})
			Expect(err).To(BeNil())

			// Wait until the "last-rotated" annotation is set to indicate the key has been rotated
			Eventually(
				func() interface{} {
					encryptionSecret, err = clientHub.CoreV1().Secrets(clusterNamespace).Get(
						ctx, "policy-encryption-key", metav1.GetOptions{},
					)
					Expect(err).To(BeNil())

					return encryptionSecret.Annotations[lastRotatedAnnotation]
				},
				defaultTimeoutSeconds,
				1,
			).ShouldNot(Equal(""))

			// Verify the rotation
			Expect(bytes.Equal(originalKey, encryptionSecret.Data["key"])).To(BeFalse())
			Expect(bytes.Equal(originalKey, encryptionSecret.Data["previousKey"])).To(BeTrue())

			// Wait until the "trigger-update" annotation has been set on the policy
			expectedTriggerUpdate := fmt.Sprintf(
				"rotate-key-%s-%s",
				clusterNamespace,
				encryptionSecret.Annotations[lastRotatedAnnotation],
			)

			Eventually(
				func() interface{} {
					rootPolicy := utils.GetWithTimeout(
						clientHubDynamic,
						common.GvrPolicy,
						policyName,
						userNamespace,
						true,
						defaultTimeoutSeconds,
					)

					return rootPolicy.GetAnnotations()[triggerUpdateAnnotation]
				},
				defaultTimeoutSeconds,
				1,
			).Should(Equal(expectedTriggerUpdate))
		})

		It("Cleans up", func() {
			err := clientHubDynamic.Resource(common.GvrPolicy).Namespace(userNamespace).Delete(
				ctx, policyName, metav1.DeleteOptions{},
			)
			if !errors.IsNotFound(err) {
				Expect(err).To(BeNil())
			}

			err = clientHub.CoreV1().Secrets(userNamespace).Delete(ctx, secretName, metav1.DeleteOptions{})
			if !errors.IsNotFound(err) {
				Expect(err).To(BeNil())
			}

			err = clientHub.CoreV1().ConfigMaps(userNamespace).Delete(ctx, configMapName, metav1.DeleteOptions{})
			if !errors.IsNotFound(err) {
				Expect(err).To(BeNil())
			}

			err = clientManaged.CoreV1().Secrets(userNamespace).Delete(ctx, secretCopyName, metav1.DeleteOptions{})
			if !errors.IsNotFound(err) {
				Expect(err).To(BeNil())
			}

			err = clientHub.CoreV1().ConfigMaps(userNamespace).Delete(ctx, configMapCopyName, metav1.DeleteOptions{})
			if !errors.IsNotFound(err) {
				Expect(err).To(BeNil())
			}
		})
	})
})
