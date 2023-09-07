// Copyright (c) 2022 Red Hat, Inc.
// Copyright Contributors to the Open Cluster Management project

package e2e

import (
	"context"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"os/exec"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	policiesv1 "open-cluster-management.io/governance-policy-propagator/api/v1"
	"open-cluster-management.io/governance-policy-propagator/test/utils"

	"github.com/stolostron/governance-policy-framework/test/common"
)

var _ = Describe("Test Hub Template Encryption", Ordered, func() {
	Describe("Test that a secret can be securely copied to managed clusters", func() {
		ctx := context.TODO()
		const policyName = "test-hub-encryption"
		const policyYAML = "../resources/hub_templates_encryption/test-hub-encryption.yaml"

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
			_, err := common.OcHub("apply", "-f", secretYAML, "-n", userNamespace)
			Expect(err).ToNot(HaveOccurred())

			By("Creating the " + configMapName + " ConfigMap")
			_, err = common.OcHub("apply", "-f", configMapYAML, "-n", userNamespace)
			Expect(err).ToNot(HaveOccurred())

			common.DoCreatePolicyTest(policyYAML, common.GvrConfigurationPolicy)
		})

		It("Should be compliant after enforcing it", func() {
			common.EnforcePolicy(policyName, common.GvrConfigurationPolicy)
			common.DoRootComplianceTest(policyName, policiesv1.Compliant)
		})

		It("Should use encryption in the replicated policy", func() {
			By("Verifying the replicated policy")
			managedplc := utils.GetWithTimeout(
				clientHostingDynamic,
				common.GvrPolicy,
				userNamespace+"."+policyName,
				clusterNamespace,
				true,
				defaultTimeoutSeconds,
			)
			Expect(managedplc).NotTo(BeNil())

			plcTemplates, ok, err := unstructured.NestedSlice(managedplc.Object, "spec", "policy-templates")
			Expect(ok).To(BeTrue())
			Expect(err).ToNot(HaveOccurred())
			Expect(plcTemplates).To(HaveLen(1))

			plcTemplate, ok := plcTemplates[0].(map[string]interface{})
			Expect(ok).To(BeTrue())

			objectTemplates, ok, err := unstructured.NestedSlice(
				plcTemplate, "objectDefinition", "spec", "object-templates",
			)
			Expect(ok).To(BeTrue())
			Expect(err).ToNot(HaveOccurred())
			Expect(objectTemplates).To(HaveLen(2))

			secretTemplate, ok := objectTemplates[0].(map[string]interface{})
			Expect(ok).To(BeTrue())

			city, ok, err := unstructured.NestedString(secretTemplate, "objectDefinition", "data", "city")
			Expect(ok).To(BeTrue())
			Expect(err).ToNot(HaveOccurred())

			// Verify that the value is encrypted
			Expect(strings.Contains(city, "$ocm_encrypted:")).To(BeTrue())
			// Verify that the plaintext or the base64 of the plaintext is not included
			Expect(strings.Contains(city, "Raleigh")).ToNot(BeTrue())
			Expect(strings.Contains(city, "UmFsZWlnaA==")).ToNot(BeTrue())

			state, ok, err := unstructured.NestedString(secretTemplate, "objectDefinition", "data", "state")
			Expect(ok).To(BeTrue())
			Expect(err).ToNot(HaveOccurred())

			// Verify that the value is encrypted
			Expect(strings.Contains(state, "$ocm_encrypted:")).To(BeTrue())
			// Verify that the plaintext or the base64 of the plaintext is not included
			Expect(strings.Contains(state, "North Carolina")).ToNot(BeTrue())
			Expect(strings.Contains(state, "Tm9ydGggQ2Fyb2xpbmE=")).ToNot(BeTrue())

			configMapTemplate, ok := objectTemplates[1].(map[string]interface{})
			Expect(ok).To(BeTrue())

			cert, ok, err := unstructured.NestedString(configMapTemplate, "objectDefinition", "data", "cert")
			Expect(ok).To(BeTrue())
			Expect(err).ToNot(HaveOccurred())

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
			Expect(err).ToNot(HaveOccurred())
			Expect(string(secret.Data["city"])).To(Equal("Raleigh"))
			Expect(string(secret.Data["state"])).To(Equal("North Carolina"))

			By("Verifying the copied ConfigMap")
			configMap, err := clientManaged.CoreV1().ConfigMaps("default").Get(
				ctx, configMapCopyName, metav1.GetOptions{},
			)
			Expect(err).ToNot(HaveOccurred())

			// Verify that the certificate can be parsed
			block, _ := pem.Decode([]byte(configMap.Data["cert"]))
			Expect(block).ToNot(BeNil())

			_, err = x509.ParseCertificate(block.Bytes)
			Expect(err).ToNot(HaveOccurred())
		})

		It("Verifies that the key can be rotated", func() {
			encryptionSecret, err := clientHub.CoreV1().Secrets(clusterNamespaceOnHub).Get(
				ctx, "policy-encryption-key", metav1.GetOptions{},
			)
			Expect(err).ToNot(HaveOccurred())
			originalKey := encryptionSecret.Data["key"]

			// Trigger a key rotation
			encryptionSecret.Annotations[lastRotatedAnnotation] = ""
			_, err = clientHub.CoreV1().Secrets(clusterNamespaceOnHub).
				Update(ctx, encryptionSecret, metav1.UpdateOptions{})
			Expect(err).ToNot(HaveOccurred())

			Eventually(
				func(g Gomega) {
					encryptionSecret, err = clientHub.CoreV1().Secrets(clusterNamespaceOnHub).Get(
						ctx, "policy-encryption-key", metav1.GetOptions{},
					)
					g.Expect(err).ToNot(HaveOccurred())

					// Wait until the "last-rotated" annotation is set to indicate the key has been rotated
					g.Expect(encryptionSecret.Annotations[lastRotatedAnnotation]).ToNot(Equal(""))
					// Verify the rotation
					g.Expect(originalKey).ToNot(Equal(encryptionSecret.Data["key"]))
					g.Expect(originalKey).To(Equal(encryptionSecret.Data["previousKey"]))
				},
				defaultTimeoutSeconds,
				1,
			).Should(Succeed())

			// Wait until the "trigger-update" annotation has been set on the policy
			expectedTriggerUpdate := fmt.Sprintf(
				"rotate-key-%s-%s",
				clusterNamespaceOnHub,
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

		AfterAll(func() {
			err := clientHubDynamic.Resource(common.GvrPolicy).Namespace(userNamespace).Delete(
				ctx, policyName, metav1.DeleteOptions{},
			)
			if !k8serrors.IsNotFound(err) {
				var exitError *exec.ExitError
				ok := errors.As(err, &exitError)
				if ok {
					Expect(exitError.Stderr).To(BeNil())
				} else {
					Expect(err).ToNot(HaveOccurred())
				}
			}

			err = clientHub.CoreV1().Secrets(userNamespace).Delete(ctx, secretName, metav1.DeleteOptions{})
			if !k8serrors.IsNotFound(err) {
				var exitError *exec.ExitError
				ok := errors.As(err, &exitError)
				if ok {
					Expect(exitError.Stderr).To(BeNil())
				} else {
					Expect(err).ToNot(HaveOccurred())
				}
			}

			err = clientHub.CoreV1().ConfigMaps(userNamespace).Delete(ctx, configMapName, metav1.DeleteOptions{})
			if !k8serrors.IsNotFound(err) {
				var exitError *exec.ExitError
				ok := errors.As(err, &exitError)
				if ok {
					Expect(exitError.Stderr).To(BeNil())
				} else {
					Expect(err).ToNot(HaveOccurred())
				}
			}

			err = clientManaged.CoreV1().Secrets(userNamespace).Delete(ctx, secretCopyName, metav1.DeleteOptions{})
			if !k8serrors.IsNotFound(err) {
				var exitError *exec.ExitError
				ok := errors.As(err, &exitError)
				if ok {
					Expect(exitError.Stderr).To(BeNil())
				} else {
					Expect(err).ToNot(HaveOccurred())
				}
			}

			err = clientHub.CoreV1().ConfigMaps(userNamespace).Delete(ctx, configMapCopyName, metav1.DeleteOptions{})
			if !k8serrors.IsNotFound(err) {
				var exitError *exec.ExitError
				ok := errors.As(err, &exitError)
				if ok {
					Expect(exitError.Stderr).To(BeNil())
				} else {
					Expect(err).ToNot(HaveOccurred())
				}
			}
		})
	})
})
