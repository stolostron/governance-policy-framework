// Copyright (c) 2020 Red Hat, Inc.
// Copyright Contributors to the Open Cluster Management project

package e2e

import (
	"context"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/stolostron/governance-policy-propagator/test/utils"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/stolostron/governance-policy-framework/test/common"
)

/*
 * NOTE: With the current Compliant/NonCompliant validation checks it is important each test alternates the expected
 * result.  In other words, do not run 2 tests in a row that return NonCompliant, the second test will immediately pass
 * using the results of the first test.
 */
var _ = Describe("Test cert policy", func() {
	Describe("Test cert policy inform", func() {
		const certPolicyName string = "cert-policy"
		const certPolicyYaml string = "../resources/cert_policy/cert-policy.yaml"
		It("should be created on managed cluster", func() {
			By("Creating " + certPolicyYaml)
			utils.Kubectl("apply", "-f", certPolicyYaml, "-n", userNamespace, "--kubeconfig=../../kubeconfig_hub")
			hubPlc := utils.GetWithTimeout(clientHubDynamic, common.GvrPolicy, certPolicyName, userNamespace, true, defaultTimeoutSeconds)
			Expect(hubPlc).NotTo(BeNil())
			By("Patching " + certPolicyName + "-plr with decision of cluster managed")
			plr := utils.GetWithTimeout(clientHubDynamic, common.GvrPlacementRule, certPolicyName+"-plr", userNamespace, true, defaultTimeoutSeconds)
			plr.Object["status"] = utils.GeneratePlrStatus("managed")
			_, err := clientHubDynamic.Resource(common.GvrPlacementRule).Namespace(userNamespace).UpdateStatus(context.TODO(), plr, metav1.UpdateOptions{})
			Expect(err).To(BeNil())
			By("Checking " + certPolicyName + " on managed cluster in ns " + clusterNamespace)
			managedplc := utils.GetWithTimeout(clientManagedDynamic, common.GvrPolicy, userNamespace+"."+certPolicyName, clusterNamespace, true, defaultTimeoutSeconds)
			Expect(managedplc).NotTo(BeNil())
		})
		It("the policy should be compliant as there is no certificate", func() {
			By("Checking if the status of root policy is compliant")
			yamlPlc := utils.ParseYaml("../resources/cert_policy/" + certPolicyName + "-compliant.yaml")
			Eventually(func() interface{} {
				rootPlc := utils.GetWithTimeout(clientHubDynamic, common.GvrPolicy, certPolicyName, userNamespace, true, defaultTimeoutSeconds)
				return rootPlc.Object["status"]
			}, defaultTimeoutSeconds, 1).Should(utils.SemanticEqual(yamlPlc.Object["status"]))
		})
		It("the policy should be noncompliant after creating a certficate that expires", func() {
			By("Creating ../resources/cert_policy/issuer.yaml in ns default")
			utils.Kubectl("apply", "-f", "../resources/cert_policy/issuer.yaml", "-n", "default", "--kubeconfig=../../kubeconfig_managed")
			By("Creating ../resources/cert_policy/certificate.yaml in ns default")
			utils.Kubectl("apply", "-f", "../resources/cert_policy/certificate.yaml", "-n", "default", "--kubeconfig=../../kubeconfig_managed")
			By("Checking if the status of root policy is noncompliant")
			yamlPlc := utils.ParseYaml("../resources/cert_policy/" + certPolicyName + "-noncompliant.yaml")
			Eventually(func() interface{} {
				rootPlc := utils.GetWithTimeout(clientHubDynamic, common.GvrPolicy, certPolicyName, userNamespace, true, defaultTimeoutSeconds)
				return rootPlc.Object["status"]
			}, defaultTimeoutSeconds, 1).Should(utils.SemanticEqual(yamlPlc.Object["status"]))
		})
		It("the policy should be compliant after creating a certficate that doesn't expire", func() {
			By("Creating ../resources/cert_policy/certificate_compliant.yaml in ns default")
			utils.Kubectl("apply", "-f", "../resources/cert_policy/certificate_compliant.yaml", "-n", "default", "--kubeconfig=../../kubeconfig_managed")
			By("Checking if the status of root policy is compliant")
			yamlPlc := utils.ParseYaml("../resources/cert_policy/" + certPolicyName + "-compliant.yaml")
			Eventually(func() interface{} {
				rootPlc := utils.GetWithTimeout(clientHubDynamic, common.GvrPolicy, certPolicyName, userNamespace, true, defaultTimeoutSeconds)
				return rootPlc.Object["status"]
			}, defaultTimeoutSeconds, 1).Should(utils.SemanticEqual(yamlPlc.Object["status"]))
		})
		It("the policy should be noncompliant after creating a certficate that expires and then is compliant after a fix", func() {
			By("Creating ../resources/cert_policy/issuer.yaml in ns default")
			utils.Kubectl("apply", "-f", "../resources/cert_policy/issuer.yaml", "-n", "default", "--kubeconfig=../../kubeconfig_managed")
			By("Creating ../resources/cert_policy/certificate.yaml in ns default")
			utils.Kubectl("apply", "-f", "../resources/cert_policy/certificate.yaml", "-n", "default", "--kubeconfig=../../kubeconfig_managed")
			By("Checking if the status of root policy is noncompliant")
			yamlPlc := utils.ParseYaml("../resources/cert_policy/" + certPolicyName + "-noncompliant.yaml")
			Eventually(func() interface{} {
				rootPlc := utils.GetWithTimeout(clientHubDynamic, common.GvrPolicy, certPolicyName, userNamespace, true, defaultTimeoutSeconds)
				return rootPlc.Object["status"]
			}, defaultTimeoutSeconds, 1).Should(utils.SemanticEqual(yamlPlc.Object["status"]))
			By("Creating ../resources/cert_policy/certificate_compliant.yaml in ns default")
			utils.Kubectl("apply", "-f", "../resources/cert_policy/certificate_compliant.yaml", "-n", "default", "--kubeconfig=../../kubeconfig_managed")
			By("Checking if the status of root policy is compliant")
			yamlPlc = utils.ParseYaml("../resources/cert_policy/" + certPolicyName + "-compliant.yaml")
			Eventually(func() interface{} {
				rootPlc := utils.GetWithTimeout(clientHubDynamic, common.GvrPolicy, certPolicyName, userNamespace, true, defaultTimeoutSeconds)
				return rootPlc.Object["status"]
			}, defaultTimeoutSeconds, 1).Should(utils.SemanticEqual(yamlPlc.Object["status"]))
		})
		It("the policy should be noncompliant after creating a CA certficate that expires", func() {
			By("Creating ../resources/cert_policy/issuer.yaml in ns default")
			utils.Kubectl("apply", "-f", "../resources/cert_policy/issuer.yaml", "-n", "default", "--kubeconfig=../../kubeconfig_managed")
			By("Creating ../resources/cert_policy/certificate_expired-ca.yaml in ns default")
			utils.Kubectl("apply", "-f", "../resources/cert_policy/certificate_expired-ca.yaml", "-n", "default", "--kubeconfig=../../kubeconfig_managed")
			By("Checking if the status of root policy is noncompliant")
			yamlPlc := utils.ParseYaml("../resources/cert_policy/" + certPolicyName + "-noncompliant.yaml")
			Eventually(func() interface{} {
				rootPlc := utils.GetWithTimeout(clientHubDynamic, common.GvrPolicy, certPolicyName, userNamespace, true, defaultTimeoutSeconds)
				return rootPlc.Object["status"]
			}, defaultTimeoutSeconds, 1).Should(utils.SemanticEqual(yamlPlc.Object["status"]))
		})
		It("the policy should be compliant after creating a certficate that doesn't expire after CA expired", func() {
			By("Creating ../resources/cert_policy/certificate_compliant.yaml in ns default")
			utils.Kubectl("apply", "-f", "../resources/cert_policy/certificate_compliant.yaml", "-n", "default", "--kubeconfig=../../kubeconfig_managed")
			By("Checking if the status of root policy is compliant")
			yamlPlc := utils.ParseYaml("../resources/cert_policy/" + certPolicyName + "-compliant.yaml")
			Eventually(func() interface{} {
				rootPlc := utils.GetWithTimeout(clientHubDynamic, common.GvrPolicy, certPolicyName, userNamespace, true, defaultTimeoutSeconds)
				return rootPlc.Object["status"]
			}, defaultTimeoutSeconds, 1).Should(utils.SemanticEqual(yamlPlc.Object["status"]))
		})
		It("the policy should be noncompliant after creating a certficate that has too long duration", func() {
			By("Creating ../resources/cert_policy/issuer.yaml in ns default")
			utils.Kubectl("apply", "-f", "../resources/cert_policy/issuer.yaml", "-n", "default", "--kubeconfig=../../kubeconfig_managed")
			By("Creating ../resources/cert_policy/certificate_long.yaml in ns default")
			utils.Kubectl("apply", "-f", "../resources/cert_policy/certificate_long.yaml", "-n", "default", "--kubeconfig=../../kubeconfig_managed")
			By("Checking if the status of root policy is noncompliant")
			yamlPlc := utils.ParseYaml("../resources/cert_policy/" + certPolicyName + "-noncompliant.yaml")
			Eventually(func() interface{} {
				rootPlc := utils.GetWithTimeout(clientHubDynamic, common.GvrPolicy, certPolicyName, userNamespace, true, defaultTimeoutSeconds)
				return rootPlc.Object["status"]
			}, defaultTimeoutSeconds, 1).Should(utils.SemanticEqual(yamlPlc.Object["status"]))
		})
		It("the policy should be compliant after creating a certficate with an expected duration", func() {
			By("Creating ../resources/cert_policy/certificate_compliant.yaml in ns default")
			utils.Kubectl("apply", "-f", "../resources/cert_policy/certificate_compliant.yaml", "-n", "default", "--kubeconfig=../../kubeconfig_managed")
			By("Checking if the status of root policy is compliant")
			yamlPlc := utils.ParseYaml("../resources/cert_policy/" + certPolicyName + "-compliant.yaml")
			Eventually(func() interface{} {
				rootPlc := utils.GetWithTimeout(clientHubDynamic, common.GvrPolicy, certPolicyName, userNamespace, true, defaultTimeoutSeconds)
				return rootPlc.Object["status"]
			}, defaultTimeoutSeconds, 1).Should(utils.SemanticEqual(yamlPlc.Object["status"]))
		})
		It("the policy should be noncompliant after creating a CA certficate that has too long duration", func() {
			By("Creating ../resources/cert_policy/issuer.yaml in ns default")
			utils.Kubectl("apply", "-f", "../resources/cert_policy/issuer.yaml", "-n", "default", "--kubeconfig=../../kubeconfig_managed")
			By("Creating ../resources/cert_policy/certificate_long-ca.yaml in ns default")
			utils.Kubectl("apply", "-f", "../resources/cert_policy/certificate_long-ca.yaml", "-n", "default", "--kubeconfig=../../kubeconfig_managed")
			By("Checking if the status of root policy is noncompliant")
			yamlPlc := utils.ParseYaml("../resources/cert_policy/" + certPolicyName + "-noncompliant.yaml")
			Eventually(func() interface{} {
				rootPlc := utils.GetWithTimeout(clientHubDynamic, common.GvrPolicy, certPolicyName, userNamespace, true, defaultTimeoutSeconds)
				return rootPlc.Object["status"]
			}, defaultTimeoutSeconds, 1).Should(utils.SemanticEqual(yamlPlc.Object["status"]))
		})
		It("the policy should be compliant after creating a certficate with an expected duration after CA", func() {
			By("Creating ../resources/cert_policy/certificate_compliant.yaml in ns default")
			utils.Kubectl("apply", "-f", "../resources/cert_policy/certificate_compliant.yaml", "-n", "default", "--kubeconfig=../../kubeconfig_managed")
			By("Checking if the status of root policy is compliant")
			yamlPlc := utils.ParseYaml("../resources/cert_policy/" + certPolicyName + "-compliant.yaml")
			Eventually(func() interface{} {
				rootPlc := utils.GetWithTimeout(clientHubDynamic, common.GvrPolicy, certPolicyName, userNamespace, true, defaultTimeoutSeconds)
				return rootPlc.Object["status"]
			}, defaultTimeoutSeconds, 1).Should(utils.SemanticEqual(yamlPlc.Object["status"]))
		})
		It("the policy should be noncompliant after creating a certficate that has a DNS entry that is not allowed", func() {
			By("Creating ../resources/cert_policy/issuer.yaml in ns default")
			utils.Kubectl("apply", "-f", "../resources/cert_policy/issuer.yaml", "-n", "default", "--kubeconfig=../../kubeconfig_managed")
			By("Creating ../resources/cert_policy/certificate_allow-noncompliant.yaml in ns default")
			utils.Kubectl("apply", "-f", "../resources/cert_policy/certificate_allow-noncompliant.yaml", "-n", "default", "--kubeconfig=../../kubeconfig_managed")
			By("Checking if the status of root policy is noncompliant")
			yamlPlc := utils.ParseYaml("../resources/cert_policy/" + certPolicyName + "-noncompliant.yaml")
			Eventually(func() interface{} {
				rootPlc := utils.GetWithTimeout(clientHubDynamic, common.GvrPolicy, certPolicyName, userNamespace, true, defaultTimeoutSeconds)
				return rootPlc.Object["status"]
			}, defaultTimeoutSeconds, 1).Should(utils.SemanticEqual(yamlPlc.Object["status"]))
		})
		It("the policy should be compliant after creating a certficate with allowed dns names", func() {
			By("Creating ../resources/cert_policy/certificate_compliant.yaml in ns default")
			utils.Kubectl("apply", "-f", "../resources/cert_policy/certificate_compliant.yaml", "-n", "default", "--kubeconfig=../../kubeconfig_managed")
			By("Checking if the status of root policy is compliant")
			yamlPlc := utils.ParseYaml("../resources/cert_policy/" + certPolicyName + "-compliant.yaml")
			Eventually(func() interface{} {
				rootPlc := utils.GetWithTimeout(clientHubDynamic, common.GvrPolicy, certPolicyName, userNamespace, true, defaultTimeoutSeconds)
				return rootPlc.Object["status"]
			}, defaultTimeoutSeconds, 1).Should(utils.SemanticEqual(yamlPlc.Object["status"]))
		})
		It("the policy should be noncompliant after creating a certficate with a disallowed wildcard", func() {
			By("Creating ../resources/cert_policy/issuer.yaml in ns default")
			utils.Kubectl("apply", "-f", "../resources/cert_policy/issuer.yaml", "-n", "default", "--kubeconfig=../../kubeconfig_managed")
			By("Creating ../resources/cert_policy/certificate_disallow-noncompliant.yaml in ns default")
			utils.Kubectl("apply", "-f", "../resources/cert_policy/certificate_disallow-noncompliant.yaml", "-n", "default", "--kubeconfig=../../kubeconfig_managed")
			By("Checking if the status of root policy is noncompliant")
			yamlPlc := utils.ParseYaml("../resources/cert_policy/" + certPolicyName + "-noncompliant.yaml")
			Eventually(func() interface{} {
				rootPlc := utils.GetWithTimeout(clientHubDynamic, common.GvrPolicy, certPolicyName, userNamespace, true, defaultTimeoutSeconds)
				return rootPlc.Object["status"]
			}, defaultTimeoutSeconds, 1).Should(utils.SemanticEqual(yamlPlc.Object["status"]))
		})
		It("the policy should be compliant after creating a certficate with no dns names that are not allowed", func() {
			By("Creating ../resources/cert_policy/certificate_compliant.yaml in ns default")
			utils.Kubectl("apply", "-f", "../resources/cert_policy/certificate_compliant.yaml", "-n", "default", "--kubeconfig=../../kubeconfig_managed")
			By("Checking if the status of root policy is compliant")
			yamlPlc := utils.ParseYaml("../resources/cert_policy/" + certPolicyName + "-compliant.yaml")
			Eventually(func() interface{} {
				rootPlc := utils.GetWithTimeout(clientHubDynamic, common.GvrPolicy, certPolicyName, userNamespace, true, defaultTimeoutSeconds)
				return rootPlc.Object["status"]
			}, defaultTimeoutSeconds, 1).Should(utils.SemanticEqual(yamlPlc.Object["status"]))
		})
		It("should clean up", func() {
			By("Deleting " + certPolicyYaml)
			utils.Kubectl("delete", "-f", certPolicyYaml, "-n", userNamespace, "--kubeconfig=../../kubeconfig_hub")
			By("Checking if there is any policy left")
			utils.ListWithTimeout(clientHubDynamic, common.GvrPolicy, metav1.ListOptions{}, 0, true, defaultTimeoutSeconds)
			utils.ListWithTimeout(clientManagedDynamic, common.GvrPolicy, metav1.ListOptions{}, 0, true, defaultTimeoutSeconds)
			By("Checking if there is any cert policy left")
			utils.ListWithTimeout(clientManagedDynamic, common.GvrCertPolicy, metav1.ListOptions{}, 0, true, defaultTimeoutSeconds)
			By("Deleting ../resources/cert_policy/issuer.yaml in ns default")
			utils.Kubectl("delete", "-f", "../resources/cert_policy/issuer.yaml", "-n", "default", "--kubeconfig=../../kubeconfig_managed")
			By("Deleting ../resources/cert_policy/certificate.yaml in ns default")
			utils.Kubectl("delete", "-f", "../resources/cert_policy/certificate.yaml", "-n", "default", "--kubeconfig=../../kubeconfig_managed")
			By("Deleting cert-policy-secret")
			utils.Kubectl("delete", "secret", "cert-policy-secret", "-n", "default", "--kubeconfig=../../kubeconfig_managed")
		})
	})
})
