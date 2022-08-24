// Copyright (c) 2022 Red Hat, Inc.
// Copyright Contributors to the Open Cluster Management project

package integration

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8stypes "k8s.io/apimachinery/pkg/types"
	policiesv1 "open-cluster-management.io/governance-policy-propagator/api/v1"
	"open-cluster-management.io/governance-policy-propagator/test/utils"

	"github.com/stolostron/governance-policy-framework/test/common"
)

var _ = Describe(
	"GRC: [P1][Sev1][policy-grc] Test the policy-certificate policy",
	Ordered,
	Label("policy-collection", "stable", "BVT"),
	func() {
		const (
			policyCertificateName   = "policy-certificate"
			policyCertificateURL    = policyCollectSCURL + policyCertificateName + ".yaml"
			expiredCertSecretName   = "expired-cert"
			policyCertificateNSName = "policy-certificate"
		)

		It("stable/"+policyCertificateName+" should be created on the Hub", func() {
			By("Creating the policy on the Hub")
			_, err := utils.KubectlWithOutput(
				"apply",
				"-f",
				policyCertificateURL,
				"-n",
				userNamespace,
				"--kubeconfig="+kubeconfigHub,
			)
			Expect(err).To(BeNil())

			By("Creating the " + policyCertificateNSName + " namespace on the managed cluster")
			namespace := &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name:   policyCertificateNSName,
					Labels: map[string]string{"e2e": "true"},
				},
			}

			_, err = clientManaged.CoreV1().Namespaces().Create(context.TODO(), namespace, metav1.CreateOptions{})
			Expect(err).To(BeNil())

			By("Patching the namespaceSelector to use the " + policyCertificateNSName + " namespace")
			_, err = clientHubDynamic.Resource(common.GvrPolicy).Namespace(userNamespace).Patch(
				context.TODO(),
				policyCertificateName,
				k8stypes.JSONPatchType,
				[]byte(`[{"op": "replace", "path": "/spec/policy-templates/0/objectDefinition`+
					`/spec/namespaceSelector/include", "value": ["`+policyCertificateNSName+`"]}]`),
				metav1.PatchOptions{},
			)
			Expect(err).To(BeNil())

			By("Patching placement rule")
			err = common.PatchPlacementRule(
				userNamespace, "placement-"+policyCertificateName, clusterNamespace, kubeconfigHub,
			)
			Expect(err).To(BeNil())

			By("Checking that " + policyCertificateName + " exists on the Hub cluster")
			rootPolicy := utils.GetWithTimeout(
				clientHubDynamic, common.GvrPolicy, policyCertificateName, userNamespace, true, defaultTimeoutSeconds,
			)
			Expect(rootPolicy).NotTo(BeNil())
		})

		It("stable/"+policyCertificateName+" should be created on managed cluster", func() {
			By("Checking the policy on the managed cluster in the namespace " + clusterNamespace)
			managedPolicy := utils.GetWithTimeout(
				clientManagedDynamic,
				common.GvrPolicy,
				userNamespace+"."+policyCertificateName,
				clusterNamespace,
				true,
				defaultTimeoutSeconds,
			)
			Expect(managedPolicy).NotTo(BeNil())
		})

		It("stable/"+policyCertificateName+" should be Compliant", func() {
			By("Checking if the status of the root policy is Compliant")
			Eventually(
				common.GetComplianceState(clientHubDynamic, userNamespace, policyCertificateName, clusterNamespace),
				defaultTimeoutSeconds*2,
				1,
			).Should(Equal(policiesv1.Compliant))
		})

		It("Make the policy NonCompliant", func() {
			By("Creating a secret with an expired certificate")
			key, err := rsa.GenerateKey(rand.Reader, 2048)
			Expect(err).To(BeNil())

			template := x509.Certificate{
				SerialNumber: big.NewInt(1),
				Subject: pkix.Name{
					Organization: []string{"Red Hat"},
				},
				NotBefore:             time.Now().Add(time.Hour * -25),
				NotAfter:              time.Now().Add(time.Hour * -1),
				DNSNames:              []string{"www.example.com"},
				KeyUsage:              x509.KeyUsageDataEncipherment | x509.KeyUsageDigitalSignature,
				ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
				BasicConstraintsValid: true,
			}
			derBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, &key.PublicKey, key)
			Expect(err).To(BeNil())

			pemBytes := &bytes.Buffer{}
			err = pem.Encode(pemBytes, &pem.Block{Type: "CERTIFICATE", Bytes: derBytes})
			Expect(err).To(BeNil())

			secret := corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{Name: expiredCertSecretName},
				Data:       map[string][]byte{"tls.crt": pemBytes.Bytes()},
			}
			_, err = clientManaged.CoreV1().Secrets(policyCertificateNSName).Create(
				context.TODO(), &secret, metav1.CreateOptions{},
			)
			Expect(err).To(BeNil())
		})

		It("stable/"+policyCertificateName+" should be NonCompliant", func() {
			By("Checking if the status of the root policy is NonCompliant")
			Eventually(
				common.GetComplianceState(clientHubDynamic, userNamespace, policyCertificateName, clusterNamespace),
				defaultTimeoutSeconds*2,
				1,
			).Should(Equal(policiesv1.NonCompliant))
		})

		AfterAll(func() {
			_, err := utils.KubectlWithOutput(
				"delete", "-f", policyCertificateURL, "-n",
				userNamespace, "--kubeconfig="+kubeconfigHub,
				"--ignore-not-found",
			)
			Expect(err).To(BeNil())

			err = clientManaged.CoreV1().Namespaces().Delete(
				context.TODO(), policyCertificateNSName, metav1.DeleteOptions{},
			)
			Expect(err).To(BeNil())
		})
	})
