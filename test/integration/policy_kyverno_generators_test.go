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

var _ = Describe("GRC: [P1][Sev1][policy-grc] Test the kyverno generator "+
	"policies", Ordered, Label("policy-collection", "stable"), func() {
	const policyNetworkURL = policyCollectCMURL + "policy-kyverno-add-network-policy.yaml"
	const policyQuotaURL = policyCollectCMURL + "policy-kyverno-add-quota.yaml"
	const policySecretsURL = policyCollectCMURL + "policy-kyverno-sync-secrets.yaml"
	const policyNetworkName = "policy-kyverno-add-network-policy"
	const policyQuotaName = "policy-kyverno-add-quota"
	const policySecretsName = "policy-kyverno-sync-secrets"
	policyNameMap := map[string]string{
		policyNetworkName: policyNetworkURL,
		policyQuotaName:   policyQuotaURL,
		policySecretsName: policySecretsURL,
	}
	const kyvernoNamespace = "kyverno"
	const kyvernoDeployment = "kyverno"
	const testNamespace = "e2e-kyverno"
	const kyvernoInstallURL = "https://raw.githubusercontent.com/stolostron/policy-collection" +
		"/main/community/CM-Configuration-Management/policy-install-kyverno.yaml"
	const kyvernoInstallPolicy = "policy-install-kyverno"
	const policyReportCRDURL = "https://raw.githubusercontent.com/kubernetes-sigs/wg-policy-prototypes" +
		"/master/policy-report/crd/v1alpha2/wgpolicyk8s.io_policyreports.yaml"
	const localClusterName = "local-cluster"

	It("Install Kyverno on the managed cluster", func() {
		By("Creating kyverno resources by deploying the community policy")
		_, err := utils.KubectlWithOutput(
			"apply", "-f", kyvernoInstallURL, "-n",
			userNamespace, "--kubeconfig="+kubeconfigHub,
		)
		Expect(err).To(BeNil())

		By("Checking if the status of the root policy is NonCompliant")
		Eventually(
			common.GetClusterComplianceState(kyvernoInstallPolicy, localClusterName),
			defaultTimeoutSeconds*2,
			1,
		).Should(Equal(policiesv1.NonCompliant))

		By("Patching the kyverno subscription's placement rule in the policy")
		_, err = utils.KubectlWithOutput(
			"patch",
			"-n",
			userNamespace,
			"policy.policy.open-cluster-management.io",
			kyvernoInstallPolicy,
			"--type=json",
			`-p=[{"op": "replace", "path": "/spec/policy-templates/1/objectDefinition/spec`+
				`/object-templates/3/objectDefinition/spec/clusterSelector", `+
				`"value":{"matchExpressions":[{"key": "name", "operator": "In", "values": ["`+
				clusterNamespace+`"]}]}}]`,
			"--kubeconfig="+kubeconfigHub,
		)
		Expect(err).To(BeNil())

		By("Patching remediationAction = enforce on the root policy")
		_, err = clientHubDynamic.Resource(common.GvrPolicy).Namespace(userNamespace).Patch(
			context.TODO(),
			kyvernoInstallPolicy,
			k8stypes.JSONPatchType,
			[]byte(`[{"op": "replace", "path": "/spec/remediationAction", "value": "enforce"}]`),
			metav1.PatchOptions{},
		)
		Expect(err).To(BeNil())

		By("Checking if the status of the root policy is Compliant")
		Eventually(
			common.GetClusterComplianceState("policy-install-kyverno", localClusterName),
			defaultTimeoutSeconds*10,
			1,
		).Should(Equal(policiesv1.Compliant))

		By("Checking that kyverno deployment exists on the managed cluster")
		Eventually(
			func() int64 {
				pod := utils.GetWithTimeout(
					clientManagedDynamic,
					common.GvrDeployment,
					kyvernoDeployment,
					kyvernoNamespace,
					true,
					defaultTimeoutSeconds*6,
				).Object
				if status, ok := pod["status"]; ok {
					if ready, ok := status.(map[string]interface{})["readyReplicas"]; ok {
						return ready.(int64)
					}
				}

				return int64(0)
			},
			common.MaxTimeoutSeconds,
			1,
		).Should(BeNumerically("==", int64(1)))
	})

	It("Create stable kyverno policies on the Hub", func() {
		for name, url := range policyNameMap {
			By("Creating the " + name + " policy on the Hub")
			_, err := utils.KubectlWithOutput(
				"apply", "-f", url, "-n", userNamespace, "--kubeconfig="+kubeconfigHub,
			)
			Expect(err).To(BeNil())

			By("Patching " + name + " placement rule")
			err = common.PatchPlacementRule(userNamespace, "placement-"+name)
			Expect(err).To(BeNil())
		}
	})

	It("Create resources used by Kyverno policies", func() {
		By("Creating secret used by the policy " + policySecretsName)
		_, err := utils.KubectlWithOutput(
			"apply", "-f",
			"../resources/kyverno-generate/sync-secret.yaml",
			"--kubeconfig="+kubeconfigManaged,
		)
		Expect(err).To(BeNil())
	})

	It("Validate policy propagation to managed cluster", func() {
		for name := range policyNameMap {
			By("Checking the " + name + " policy on managed cluster in ns " + clusterNamespace)
			managedPolicy := utils.GetWithTimeout(
				clientManagedDynamic,
				common.GvrPolicy,
				userNamespace+"."+name,
				clusterNamespace,
				true,
				defaultTimeoutSeconds,
			)
			Expect(managedPolicy).NotTo(BeNil())
		}
	})

	It("github deployed policy should be NonCompliant initially", func() {
		for name := range policyNameMap {
			By("Checking if the status of root policy " + name + " is NonCompliant")
			Eventually(
				common.GetComplianceState(name),
				defaultTimeoutSeconds*2,
				1,
			).Should(Equal(policiesv1.NonCompliant))
		}
	})

	It("Enforcing the default policy to make it compliant and activate the kyverno policy", func() {
		for name := range policyNameMap {
			By("Patching remediationAction = enforce on the root policy " + name)
			_, err := clientHubDynamic.Resource(common.GvrPolicy).Namespace(userNamespace).Patch(
				context.TODO(),
				name,
				k8stypes.JSONPatchType,
				[]byte(`[{"op": "replace", "path": "/spec/remediationAction", "value": "enforce"}]`),
				metav1.PatchOptions{},
			)
			Expect(err).To(BeNil())

			By("Checking if the status of root policy " + name + " is now Compliant")
			Eventually(
				common.GetComplianceState(name),
				defaultTimeoutSeconds*2,
				1,
			).Should(Equal(policiesv1.Compliant))
		}
	})

	It("Initiate resource generation for Kyverno policies", func() {
		By("Create a new namespace that kyverno will react to")
		_, err := utils.KubectlWithOutput(
			"apply", "-f",
			"../resources/kyverno-generate/namespace.yaml",
			"--kubeconfig="+kubeconfigManaged,
		)
		Expect(err).To(BeNil())
	})

	It("Kyverno generate policies should create resources in the new namespace", func() {
		By("Checking if the NetworkPolicy resource got created")
		Eventually(
			func() error {
				_, err := utils.KubectlWithOutput(
					"get", "NetworkPolicy", "-n", testNamespace,
					"default-deny", "--kubeconfig="+kubeconfigManaged,
				)

				return err
			},
			defaultTimeoutSeconds*2,
			1,
		).Should(BeNil())

		By("Checking if the LimitRange resource got created")
		Eventually(
			func() error {
				_, err := utils.KubectlWithOutput(
					"get", "LimitRange", "-n",
					testNamespace,
					"default-limitrange",
					"--kubeconfig="+kubeconfigManaged,
				)

				return err
			},
			defaultTimeoutSeconds*2,
			1,
		).Should(BeNil())

		By("Checking if the ResourceQuota resource got created")
		Eventually(
			func() error {
				_, err := utils.KubectlWithOutput(
					"get", "ResourceQuota", "-n",
					testNamespace,
					"default-resourcequota",
					"--kubeconfig="+kubeconfigManaged,
				)

				return err
			},
			defaultTimeoutSeconds*2,
			1,
		).Should(BeNil())

		By("Checking if the Secret resource got created")
		Eventually(
			func() error {
				_, err := utils.KubectlWithOutput(
					"get", "Secret", "-n",
					testNamespace, "regcred",
					"--kubeconfig="+kubeconfigManaged,
				)

				return err
			},
			defaultTimeoutSeconds*2,
			1,
		).Should(BeNil())
	})

	AfterAll(func() {
		if CurrentSpecReport().Failed() {
			common.OutputDebugInfo(
				"Kyverno generator",
				kubeconfigHub,
				"clusterpolicies.kyverno.io",
				"policies.kyverno.io",
			)
		}

		Eventually(func(g Gomega) {
			// delete the policies
			for _, url := range policyNameMap {
				_, err := utils.KubectlWithOutput(
					"delete", "-f", url, "-n",
					userNamespace, "--kubeconfig="+kubeconfigHub,
					"--ignore-not-found",
				)
				g.Expect(err).To(BeNil())
			}

			// remove the kyverno install policy
			_, err := utils.KubectlWithOutput(
				"delete", "-f", kyvernoInstallURL,
				"-n", userNamespace, "--kubeconfig="+kubeconfigHub,
				"--ignore-not-found",
			)
			g.Expect(err).To(BeNil())

			// delete the subscription
			_, err = utils.KubectlWithOutput(
				"delete", "subscription.apps.open-cluster-management.io",
				"-n", kyvernoNamespace, "--all",
				"--kubeconfig="+kubeconfigManaged,
				"--ignore-not-found",
			)
			g.Expect(err).To(BeNil())

			// make sure kyverno mutating webhooks are removed
			_, err = utils.KubectlWithOutput(
				"delete", "mutatingwebhookconfigurations",
				"kyverno-policy-mutating-webhook-cfg",
				"kyverno-resource-mutating-webhook-cfg",
				"kyverno-verify-mutating-webhook-cfg",
				"--kubeconfig="+kubeconfigManaged,
				"--ignore-not-found",
			)
			g.Expect(err).To(BeNil())

			// make sure kyverno validating webhooks are removed
			_, err = utils.KubectlWithOutput(
				"delete",
				"validatingwebhookconfigurations",
				"kyverno-policy-validating-webhook-cfg",
				"kyverno-resource-validating-webhook-cfg",
				"--kubeconfig="+kubeconfigManaged,
				"--ignore-not-found",
			)
			g.Expect(err).To(BeNil())

			// delete the namespace created to test the generators
			_, err = utils.KubectlWithOutput(
				"delete", "ns", testNamespace,
				"--kubeconfig="+kubeconfigManaged,
				"--ignore-not-found",
			)
			g.Expect(err).To(BeNil())

			// delete the channel namespace
			_, err = utils.KubectlWithOutput(
				"delete", "ns",
				"kyverno-channel",
				"--kubeconfig="+kubeconfigManaged,
				"--ignore-not-found",
			)
			g.Expect(err).To(BeNil())

			// delete the kyverno namespace
			_, err = utils.KubectlWithOutput(
				"delete", "ns",
				"kyverno",
				"--kubeconfig="+kubeconfigManaged,
				"--ignore-not-found",
			)
			g.Expect(err).To(BeNil())

			// ensure the PolicyReport CRD remains on the cluster
			_, _ = utils.KubectlWithOutput(
				"apply", "-f", policyReportCRDURL, "--kubeconfig="+kubeconfigManaged,
			)

			// delete secret that is synced by the generator
			_, err = utils.KubectlWithOutput(
				"delete", "secret", "-n", "default", "regcred",
				"--kubeconfig="+kubeconfigManaged, "--ignore-not-found",
			)
			g.Expect(err).To(BeNil())
		}, defaultTimeoutSeconds*2)
	})
})
